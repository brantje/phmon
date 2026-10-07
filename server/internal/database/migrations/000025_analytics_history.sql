-- Post-install observations are the source for character performance rates and
-- guild-storage gold balance history. No current value is backfilled as history.
CREATE TABLE character_metric_samples (
    sample_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES character_sessions(session_id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(agent_id) ON DELETE CASCADE,
    sample_schema_version SMALLINT NOT NULL DEFAULT 1 CHECK (sample_schema_version > 0),
    server_key TEXT NOT NULL,
    sampled_at TIMESTAMPTZ NOT NULL,
    level INTEGER CHECK (level IS NULL OR level BETWEEN 0 AND 255),
    current_exp BIGINT CHECK (current_exp IS NULL OR current_exp >= 0),
    max_exp BIGINT CHECK (max_exp IS NULL OR max_exp >= 0),
    sp BIGINT CHECK (sp IS NULL OR sp >= 0),
    gold BIGINT CHECK (gold IS NULL OR gold >= 0),
    region INTEGER CHECK (region IS NULL OR (region BETWEEN -32768 AND 65535 AND region <> 0)),
    zone_name TEXT CHECK (zone_name IS NULL OR char_length(zone_name) <= 100),
    botting BOOLEAN,
    dead BOOLEAN,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX character_metric_character_time_idx
    ON character_metric_samples (character_id, sampled_at, sample_id);
CREATE INDEX character_metric_session_time_idx
    ON character_metric_samples (session_id, sampled_at, sample_id);
CREATE INDEX character_metric_server_time_idx
    ON character_metric_samples (server_key, sampled_at DESC);

CREATE TABLE guild_gold_samples (
    sample_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_key TEXT NOT NULL CHECK (char_length(server_key) BETWEEN 1 AND 100),
    guild_key TEXT NOT NULL CHECK (char_length(guild_key) BETWEEN 1 AND 100),
    observer_character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES character_sessions(session_id) ON DELETE CASCADE,
    resource_revision BIGINT NOT NULL CHECK (resource_revision > 0),
    sampled_at TIMESTAMPTZ NOT NULL,
    gold BIGINT NOT NULL CHECK (gold >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX guild_gold_scope_time_idx
    ON guild_gold_samples (server_key, guild_key, sampled_at DESC);
CREATE INDEX guild_gold_observer_time_idx
    ON guild_gold_samples (observer_character_id, sampled_at DESC);

CREATE TABLE character_rate_resets (
    reset_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 16 AND 128),
    requested_by TEXT NOT NULL CHECK (char_length(requested_by) BETWEEN 1 AND 100),
    reset_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (character_id, idempotency_key)
);
CREATE INDEX character_rate_reset_latest_idx
    ON character_rate_resets (character_id, reset_at DESC);
