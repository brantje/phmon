-- Bank history begins with paired observations from protocol 19; do not backfill.
ALTER TABLE character_metric_samples ADD COLUMN storage_gold BIGINT
    CHECK (storage_gold IS NULL OR storage_gold >= 0);
