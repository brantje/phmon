package analytics

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
)

const (
	goldRollupName = "character_gold"
	// goldRollupQuarter divides every real-world UTC offset, so each quarter
	// lies inside one local hour, day and week bucket.
	goldRollupQuarter = 15 * time.Minute
	// Samples are timestamped at insert but may commit seconds later while
	// WAL writes are slow. Only roll quarters that ended before this margin.
	goldRollupSettle = 5 * time.Minute
	// Each run rebuilds this much already-rolled history to absorb late commits.
	goldRollupRecheck = time.Hour
	goldRollupChunk   = 6 * time.Hour
)

func floorQuarter(t time.Time) time.Time {
	return t.UTC().Truncate(goldRollupQuarter)
}

// goldRollupUsable reports whether rollup quarters align with the filter's
// start and local buckets. Callers otherwise read raw samples only.
func goldRollupUsable(filter Filter) bool {
	if !filter.From.Equal(floorQuarter(filter.From)) {
		return false
	}
	location, err := time.LoadLocation(filter.Timezone)
	if err != nil {
		return false
	}
	for _, at := range []time.Time{filter.From, filter.To.Add(-time.Nanosecond)} {
		if _, offset := at.In(location).Zone(); offset%int(goldRollupQuarter/time.Second) != 0 {
			return false
		}
	}
	return true
}

// goldRollupSplit returns the instant before which Economy reads rollups and
// after which it reads raw samples. It equals filter.From when rollups cannot
// serve any part of the window.
func goldRollupSplit(ctx context.Context, tx pgx.Tx, filter Filter) (time.Time, error) {
	if !goldRollupUsable(filter) {
		return filter.From, nil
	}
	var watermark time.Time
	err := tx.QueryRow(ctx, `SELECT watermark FROM analytics_rollup_state WHERE name=$1`, goldRollupName).Scan(&watermark)
	if errors.Is(err, pgx.ErrNoRows) {
		return filter.From, nil
	}
	if err != nil {
		return time.Time{}, err
	}
	split := watermark
	if filter.To.Before(split) {
		split = filter.To
	}
	split = floorQuarter(split)
	if split.Before(filter.From) {
		return filter.From, nil
	}
	return split, nil
}

// RebuildGoldRollups replaces rollups for quarters starting in [from, to).
// Both bounds must be quarter aligned.
func (s *Store) RebuildGoldRollups(ctx context.Context, from, to time.Time) error {
	if s == nil || s.pool == nil {
		return errors.New("analytics store unavailable")
	}
	if !to.After(from) || !from.Equal(floorQuarter(from)) || !to.Equal(floorQuarter(to)) {
		return errors.New("invalid gold rollup range")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `DELETE FROM character_gold_rollups WHERE bucket_start >= $1 AND bucket_start < $2`, from, to); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO character_gold_rollups (
    character_id,bucket_start,server_key,last_sampled_at,last_sample_id,last_gold,
    net_delta,interval_count,carry_delta,carry_count
)
WITH sequenced AS (
    SELECT s.character_id,s.server_key,s.sample_id,s.sampled_at,s.gold,
           date_bin('15 minutes',s.sampled_at,'2000-01-01 00:00:00+00'::timestamptz) AS bucket_start,
           lag(s.sampled_at) OVER w AS before_at,
           lag(s.gold) OVER w AS before_gold
    FROM character_metric_samples s
    WHERE s.sampled_at >= ($1::timestamptz-interval '30 seconds') AND s.sampled_at < $2
    WINDOW w AS (PARTITION BY s.session_id ORDER BY s.sampled_at,s.sample_id)
), inside AS MATERIALIZED (
    SELECT *, (gold IS NOT NULL AND before_gold IS NOT NULL
               AND sampled_at-before_at <= interval '30 seconds') AS comparable
    FROM sequenced WHERE sampled_at >= $1
), totals AS (
    SELECT character_id,bucket_start,min(server_key) AS server_key,
           COALESCE(sum(gold-before_gold) FILTER (WHERE comparable),0) AS net_delta,
           count(*) FILTER (WHERE comparable) AS interval_count,
           COALESCE(sum(gold-before_gold) FILTER (WHERE comparable AND before_at < bucket_start),0) AS carry_delta,
           count(*) FILTER (WHERE comparable AND before_at < bucket_start) AS carry_count
    FROM inside GROUP BY character_id,bucket_start
), last_gold AS (
    SELECT DISTINCT ON(character_id,bucket_start) character_id,bucket_start,sampled_at,sample_id,gold
    FROM inside WHERE gold IS NOT NULL
    ORDER BY character_id,bucket_start,sampled_at DESC,sample_id DESC
)
SELECT t.character_id,t.bucket_start,t.server_key,l.sampled_at,l.sample_id,l.gold,
       t.net_delta,t.interval_count,t.carry_delta,t.carry_count
FROM totals t LEFT JOIN last_gold l ON l.character_id=t.character_id AND l.bucket_start=t.bucket_start`, from, to); err != nil {
		return err
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO analytics_rollup_state(name,watermark) VALUES ($1,$2)
ON CONFLICT (name) DO UPDATE SET watermark=GREATEST(analytics_rollup_state.watermark,EXCLUDED.watermark)`, goldRollupName, to); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

// advanceGoldRollups rolls every settled quarter up to now-goldRollupSettle,
// rebuilding the most recent hour already rolled. It returns the watermark.
func (s *Store) advanceGoldRollups(ctx context.Context, now time.Time) (time.Time, error) {
	target := floorQuarter(now.Add(-goldRollupSettle))
	var start time.Time
	var watermark *time.Time
	if err := s.pool.QueryRow(ctx, `SELECT watermark FROM analytics_rollup_state WHERE name=$1`, goldRollupName).Scan(&watermark); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, err
	}
	if watermark != nil {
		start = floorQuarter(watermark.Add(-goldRollupRecheck))
	} else {
		var oldest *time.Time
		if err := s.pool.QueryRow(ctx, `SELECT min(sampled_at) FROM character_metric_samples`).Scan(&oldest); err != nil {
			return time.Time{}, err
		}
		if oldest == nil {
			start = target.Add(-goldRollupQuarter)
		} else {
			start = floorQuarter(*oldest)
		}
	}
	for start.Before(target) {
		end := start.Add(goldRollupChunk)
		if end.After(target) {
			end = target
		}
		chunkCtx, cancel := context.WithTimeout(ctx, time.Minute)
		err := s.RebuildGoldRollups(chunkCtx, start, end)
		cancel()
		if err != nil {
			return start, err
		}
		start = end
	}
	return target, nil
}

// RunGoldRollups keeps Economy rollups current until ctx ends.
func (s *Store) RunGoldRollups(ctx context.Context, interval time.Duration) {
	if interval < 10*time.Second {
		interval = time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		if _, err := s.advanceGoldRollups(ctx, time.Now()); err != nil && ctx.Err() == nil {
			slog.Warn("character gold rollup failed", "error", err)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}
