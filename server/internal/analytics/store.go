package analytics

import (
	"context"
	"errors"
	"log/slog"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const MaxRetentionDeleteBatch = 10000

const (
	analyticsRetentionBatchSize = 5000
	analyticsRetentionRunBudget = 2 * time.Minute
)

var ErrRateResetNameMismatch = errors.New("character name confirmation does not match")
var ErrRateResetNotFound = errors.New("character not found")

type Store struct {
	pool            *pgxpool.Pool
	itemTaxonomy    func(server string, model int64, code string) (itemType, degree string, known bool)
	itemModels      func(server, itemType, degree string) ([]int64, bool)
	taxonomyOptions func(server string) []TaxonomyOption
	itemDetails     func(server string, model *int64, code string, payloadItem map[string]any) (metadata, details map[string]any)
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func (s *Store) SetItemTaxonomy(
	resolve func(server string, model int64, code string) (itemType, degree string, known bool),
	models func(server, itemType, degree string) ([]int64, bool),
	options func(server string) []TaxonomyOption,
) {
	if s == nil {
		return
	}
	s.itemTaxonomy, s.itemModels, s.taxonomyOptions = resolve, models, options
}

func (s *Store) SetItemDetailsResolver(resolver func(server string, model *int64, code string, payloadItem map[string]any) (metadata, details map[string]any)) {
	if s != nil {
		s.itemDetails = resolver
	}
}

func (s *Store) ResetRateWindow(ctx context.Context, characterID, typedName, actor, idempotencyKey string) (time.Time, error) {
	if s == nil || s.pool == nil || len(characterID) != 36 || len(typedName) == 0 || len(typedName) > 64 || len(actor) == 0 || len(actor) > 100 || len(idempotencyKey) < 16 || len(idempotencyKey) > 128 {
		return time.Time{}, ErrInvalidFilter
	}
	var canonicalName string
	err := s.pool.QueryRow(ctx, `SELECT character_name FROM characters WHERE character_id=$1::uuid`, characterID).Scan(&canonicalName)
	if errors.Is(err, pgx.ErrNoRows) {
		return time.Time{}, ErrRateResetNotFound
	}
	if err != nil {
		return time.Time{}, err
	}
	if canonicalName != typedName {
		return time.Time{}, ErrRateResetNameMismatch
	}
	var resetAt time.Time
	err = s.pool.QueryRow(ctx, `INSERT INTO character_rate_resets(character_id,idempotency_key,requested_by)
VALUES($1::uuid,$2,$3)
ON CONFLICT(character_id,idempotency_key) DO NOTHING
RETURNING reset_at`, characterID, idempotencyKey, actor).Scan(&resetAt)
	if errors.Is(err, pgx.ErrNoRows) {
		err = s.pool.QueryRow(ctx, `SELECT reset_at FROM character_rate_resets WHERE character_id=$1::uuid AND idempotency_key=$2`, characterID, idempotencyKey).Scan(&resetAt)
	}
	return resetAt.UTC(), err
}

// PruneOldSamples performs bounded cleanup. Callers repeat it to make progress
// without holding a long lock or monopolizing the shared database pool.
func (s *Store) PruneOldSamples(ctx context.Context, before time.Time, batchSize int) (characterRows, guildRows int64, err error) {
	if s == nil || s.pool == nil || before.IsZero() {
		return 0, 0, errors.New("invalid analytics retention request")
	}
	if batchSize < 1 || batchSize > MaxRetentionDeleteBatch {
		batchSize = 1000
	}
	if err := s.pool.QueryRow(ctx, `
WITH expired AS (
    SELECT sample_id FROM character_metric_samples
    WHERE sampled_at<$1
    ORDER BY sampled_at
    LIMIT $2
), removed AS (
    DELETE FROM character_metric_samples sample
    USING expired
    WHERE sample.sample_id=expired.sample_id
    RETURNING 1
)
SELECT count(*) FROM removed`, before.UTC(), batchSize).Scan(&characterRows); err != nil {
		return 0, 0, err
	}
	if err := s.pool.QueryRow(ctx, `
WITH expired AS (
    SELECT sample_id FROM guild_gold_samples
    WHERE sampled_at<$1
    ORDER BY sampled_at
    LIMIT $2
), removed AS (
    DELETE FROM guild_gold_samples sample
    USING expired
    WHERE sample.sample_id=expired.sample_id
    RETURNING 1
)
SELECT count(*) FROM removed`, before.UTC(), batchSize).Scan(&guildRows); err != nil {
		return characterRows, 0, err
	}
	return characterRows, guildRows, nil
}

func (s *Store) OldestSample(ctx context.Context, server string) (*time.Time, error) {
	if s == nil || s.pool == nil {
		return nil, errors.New("analytics store unavailable")
	}
	var oldest *time.Time
	err := s.pool.QueryRow(ctx, `
SELECT min(sampled_at)
FROM (
    SELECT sampled_at FROM character_metric_samples
    WHERE ($1='' OR server_key=lower($1))
    UNION ALL
    SELECT sample.sampled_at FROM guild_gold_samples sample
    WHERE ($1='' OR sample.server_key=lower($1))
) samples`, server).Scan(&oldest)
	if err != nil {
		return nil, err
	}
	if oldest != nil {
		value := oldest.UTC()
		return &value, nil
	}
	return nil, nil
}

func (s *Store) RunRetention(ctx context.Context, days int, interval time.Duration) {
	if days < 1 || days > 3650 {
		slog.Error("analytics retention disabled: invalid day count")
		return
	}
	if interval < time.Minute {
		interval = 15 * time.Minute
	}
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		cutoff := time.Now().UTC().Add(-time.Duration(days) * 24 * time.Hour)
		total := int64(0)
		runDeadline := time.Now().Add(analyticsRetentionRunBudget)
		deleted, err := pruneOldSamplesUntil(ctx, cutoff, analyticsRetentionBatchSize, runDeadline, s.PruneOldSamples)
		if err != nil && ctx.Err() == nil {
			slog.Warn("analytics retention batch failed", "error", err)
		}
		total += deleted
		if total > 0 {
			slog.Info("expired analytics samples pruned", "rows", total, "retention_days", days)
		}
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
	}
}

func pruneOldSamplesUntil(
	ctx context.Context,
	cutoff time.Time,
	batchSize int,
	deadline time.Time,
	prune func(context.Context, time.Time, int) (int64, int64, error),
) (int64, error) {
	var total int64
	for ctx.Err() == nil && time.Now().Before(deadline) {
		batchCtx, cancel := context.WithTimeout(ctx, 5*time.Second)
		characterRows, guildRows, err := prune(batchCtx, cutoff, batchSize)
		cancel()
		if err != nil {
			return total, err
		}
		total += characterRows + guildRows
		if characterRows < int64(batchSize) && guildRows < int64(batchSize) {
			break
		}
	}
	return total, ctx.Err()
}
