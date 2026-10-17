-- Fifteen-minute summaries of settled character gold samples. Economy reads
-- these for history and scans raw samples only after the watermark, so query
-- cost stays bounded as the selected range and sample count grow.
CREATE TABLE character_gold_rollups (
    character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    bucket_start TIMESTAMPTZ NOT NULL,
    server_key TEXT NOT NULL,
    -- Latest sample in the quarter with observed gold (NULL when none had gold).
    last_sampled_at TIMESTAMPTZ,
    last_sample_id UUID,
    last_gold BIGINT,
    -- Comparable same-session intervals (<= 30 seconds, both golds observed)
    -- ending in this quarter, and the subset whose earlier sample precedes it.
    net_delta NUMERIC NOT NULL DEFAULT 0,
    interval_count INTEGER NOT NULL DEFAULT 0 CHECK (interval_count >= 0),
    carry_delta NUMERIC NOT NULL DEFAULT 0,
    carry_count INTEGER NOT NULL DEFAULT 0 CHECK (carry_count >= 0 AND carry_count <= interval_count),
    PRIMARY KEY (character_id, bucket_start)
);

CREATE INDEX character_gold_rollups_server_bucket_idx
    ON character_gold_rollups (server_key, bucket_start);
CREATE INDEX character_gold_rollups_bucket_idx
    ON character_gold_rollups (bucket_start);

-- Rollups are complete for every quarter before the watermark.
CREATE TABLE analytics_rollup_state (
    name TEXT PRIMARY KEY,
    watermark TIMESTAMPTZ NOT NULL
);
