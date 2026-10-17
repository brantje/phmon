package analytics_test

import (
	"context"
	"errors"
	"math/rand"
	"reflect"
	"testing"
	"time"

	"github.com/jackc/pgx/v5"
	"phmon/server/internal/analytics"
	"phmon/server/internal/characters"
)

// Rollups must not change any Economy result. Build an irregular history in
// 2001 (so the shared watermark never covers other tests' recent windows) and
// compare raw snapshots with rollup-backed snapshots at several watermarks.
func TestEconomyGoldRollupsMatchRawSamples(t *testing.T) {
	f := newReviewFixture(t, 3)
	ctx := context.Background()
	var previous *time.Time
	if err := f.pool.QueryRow(ctx, `SELECT watermark FROM analytics_rollup_state WHERE name='character_gold'`).Scan(&previous); err != nil && !errors.Is(err, pgx.ErrNoRows) {
		t.Fatal(err)
	}
	if previous != nil && previous.Year() > 2001 {
		t.Skip("shared database already has rollups past 2001; use a disposable database")
	}
	t.Cleanup(func() {
		_, _ = f.pool.Exec(ctx, `DELETE FROM character_gold_rollups WHERE bucket_start < '2002-01-01'`)
		_, _ = f.pool.Exec(ctx, `DELETE FROM analytics_rollup_state WHERE name='character_gold'`)
	})

	amsterdam, _ := time.LoadLocation("Europe/Amsterdam")
	// Europe switches to summer time on 2001-03-25 at 01:00 UTC.
	from := time.Date(2001, 3, 22, 0, 0, 0, 0, amsterdam).UTC()
	to := time.Date(2001, 3, 28, 0, 0, 0, 0, amsterdam).UTC()
	group, err := characters.NewStore(f.pool).CreateGroup(ctx, "rollup-"+f.agent[:8])
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { _, _ = f.pool.Exec(ctx, `DELETE FROM character_groups WHERE group_id=$1::uuid`, group.ID) })
	if err := characters.NewStore(f.pool).SetMember(ctx, group.ID, f.chars[1], true); err != nil {
		t.Fatal(err)
	}

	random := rand.New(rand.NewSource(7))
	gaps := []time.Duration{3 * time.Second, 10 * time.Second, 29 * time.Second, 30 * time.Second, 31 * time.Second, 2 * time.Minute, 47 * time.Minute}
	for index, characterID := range f.chars {
		session := f.sessions[index]
		// Start just before From so the first quarter has carried intervals.
		at := from.Add(-time.Duration(20+index) * time.Second)
		gold := int64(1_000_000 * (index + 1))
		for count := 0; at.Before(to.Add(time.Hour)); count++ {
			if count == 600 {
				newSession, err := characters.NewStore(f.pool).ClaimSessionID(ctx, f.agent, characterID, 2)
				if err != nil {
					t.Fatal(err)
				}
				session = newSession
			}
			gold += int64(random.Intn(2001) - 900)
			var value any = gold
			if random.Intn(23) == 0 {
				value = nil
			}
			if _, err := f.pool.Exec(ctx, `INSERT INTO character_metric_samples(character_id,session_id,agent_id,server_key,sampled_at,gold)
VALUES($1::uuid,$2::uuid,$3::uuid,$4,$5,$6)`, characterID, session, f.agent, f.server, at, value); err != nil {
				t.Fatal(err)
			}
			if count%500 == 0 {
				if _, err := f.pool.Exec(ctx, `INSERT INTO character_storage_gold_samples(character_id,session_id,resource_revision,sampled_at,gold)
VALUES($1::uuid,$2::uuid,$3,$4,$5)`, characterID, session, count+1, at.Add(time.Second), int64(count)*7); err != nil {
					t.Fatal(err)
				}
			}
			at = at.Add(gaps[random.Intn(len(gaps))] + time.Duration(random.Intn(1000))*time.Millisecond)
		}
	}

	store := analytics.NewStore(f.pool)
	filters := []analytics.Filter{
		{Server: f.server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "Europe/Amsterdam"},
		{Server: f.server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "Europe/Amsterdam", Bucket: "hour"},
		{Server: f.server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "UTC", Bucket: "week"},
		{Server: f.server, View: analytics.ViewEconomy, From: from, To: to.Add(-7 * time.Minute), Timezone: "Asia/Kathmandu"},
		{Server: f.server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "Europe/Amsterdam", CharacterID: f.chars[2]},
		{Server: f.server, View: analytics.ViewEconomy, From: from, To: to, Timezone: "Europe/Amsterdam", GroupID: group.ID},
		// Unaligned windows must keep reading raw samples.
		{Server: f.server, View: analytics.ViewEconomy, From: from.Add(17 * time.Second), To: to, Timezone: "Europe/Amsterdam"},
	}
	snapshot := func(filter analytics.Filter) analytics.Snapshot {
		t.Helper()
		result, err := store.Query(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		result.AsOf = time.Time{}
		return result
	}
	raw := make([]analytics.Snapshot, len(filters))
	for index, filter := range filters {
		raw[index] = snapshot(filter)
		if len(raw[index].TimeSeries) == 0 {
			t.Fatalf("fixture produced no economy series for filter %d", index)
		}
	}

	rollupFrom := from.Add(-time.Hour).Truncate(15 * time.Minute)
	// A watermark inside the window (mixed rollup/raw), then past its end.
	for _, watermark := range []time.Time{
		time.Date(2001, 3, 25, 0, 45, 0, 0, time.UTC),
		to.Add(2 * time.Hour),
	} {
		if err := store.RebuildGoldRollups(ctx, rollupFrom, watermark); err != nil {
			t.Fatal(err)
		}
		// Prove aligned filters read rollups: a tampered rollup must change
		// the result. Rebuild before the equality checks.
		if tag, err := f.pool.Exec(ctx, `UPDATE character_gold_rollups SET net_delta=net_delta+1
WHERE (character_id,bucket_start)=(SELECT character_id,bucket_start FROM character_gold_rollups
    WHERE character_id=$1::uuid AND bucket_start>$2 AND bucket_start<$3 AND interval_count>0
    ORDER BY bucket_start LIMIT 1)`, f.chars[0], from, watermark); err != nil || tag.RowsAffected() != 1 {
			t.Fatalf("tamper rollup: %v (%d rows)", err, tag.RowsAffected())
		}
		probe, err := analytics.NewStore(f.pool).Query(ctx, filters[0])
		if err != nil {
			t.Fatal(err)
		}
		probe.AsOf = time.Time{}
		if reflect.DeepEqual(probe, raw[0]) {
			t.Fatalf("watermark %s: aligned filter did not read rollups", watermark)
		}
		if err := store.RebuildGoldRollups(ctx, rollupFrom, watermark); err != nil {
			t.Fatal(err)
		}
		for index, filter := range filters {
			// Each Query call must bypass the five-second snapshot cache.
			fresh := analytics.NewStore(f.pool)
			result, err := fresh.Query(ctx, filter)
			if err != nil {
				t.Fatal(err)
			}
			result.AsOf = time.Time{}
			if !reflect.DeepEqual(result, raw[index]) {
				t.Fatalf("watermark %s filter %d: rollup result differs from raw samples\nraw:    %+v\nrollup: %+v", watermark, index, raw[index].Summary, result.Summary)
			}
		}
	}

	// Retention with a cutoff inside a quarter keeps that quarter's rollup,
	// rebuilt from the samples it kept, so rollup reads still match raw ones.
	cutoff := from.Add(7*time.Minute + 13*time.Second)
	if _, _, _, err := store.PruneOldSamples(ctx, cutoff, analytics.MaxRetentionDeleteBatch); err != nil {
		t.Fatal(err)
	}
	var older, boundary int
	if err := f.pool.QueryRow(ctx, `SELECT count(*) FILTER (WHERE bucket_start<$2),count(*) FILTER (WHERE bucket_start=$2)
FROM character_gold_rollups WHERE character_id=ANY($1::uuid[])`, f.chars, from).Scan(&older, &boundary); err != nil {
		t.Fatal(err)
	}
	if older != 0 || boundary == 0 {
		t.Fatalf("retention kept %d expired rollups and %d boundary rollups", older, boundary)
	}
	var watermark time.Time
	if err := f.pool.QueryRow(ctx, `DELETE FROM analytics_rollup_state WHERE name='character_gold' RETURNING watermark`).Scan(&watermark); err != nil {
		t.Fatal(err)
	}
	// Fresh stores bypass the five-second Economy snapshot cache.
	uncached := func(filter analytics.Filter) analytics.Snapshot {
		t.Helper()
		result, err := analytics.NewStore(f.pool).Query(ctx, filter)
		if err != nil {
			t.Fatal(err)
		}
		result.AsOf = time.Time{}
		return result
	}
	pruned := uncached(filters[0])
	if _, err := f.pool.Exec(ctx, `INSERT INTO analytics_rollup_state(name,watermark) VALUES('character_gold',$1)`, watermark); err != nil {
		t.Fatal(err)
	}
	if result := uncached(filters[0]); !reflect.DeepEqual(result, pruned) {
		t.Fatalf("after retention: rollup result differs from raw samples\nraw:    %+v\nrollup: %+v", pruned.Summary, result.Summary)
	}

	var rollups, carried int
	if err := f.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE carry_count>0) FROM character_gold_rollups
WHERE character_id=ANY($1::uuid[])`, f.chars).Scan(&rollups, &carried); err != nil {
		t.Fatal(err)
	}
	if rollups == 0 || carried == 0 {
		t.Fatalf("fixture did not exercise rollups with carried intervals: %d rows, %d carried", rollups, carried)
	}
}
