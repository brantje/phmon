CREATE TABLE character_position_samples (
    sample_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    character_id UUID NOT NULL REFERENCES characters(character_id),
    session_id UUID NOT NULL REFERENCES character_sessions(session_id),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    dataset_id TEXT NOT NULL CHECK (dataset_id ~ '^gamedata-[a-z0-9]{1,64}$'),
    sampled_at TIMESTAMPTZ NOT NULL,
    region INTEGER NOT NULL CHECK (region BETWEEN 1 AND 65535),
    x DOUBLE PRECISION NOT NULL CHECK (x BETWEEN -1000000 AND 1000000),
    y DOUBLE PRECISION NOT NULL CHECK (y BETWEEN -1000000 AND 1000000),
    z DOUBLE PRECISION CHECK (z IS NULL OR z BETWEEN -1000000 AND 1000000),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (session_id, sampled_at)
);
CREATE INDEX character_position_server_time_idx
    ON character_position_samples (lower(server_name), sampled_at DESC);
CREATE INDEX character_position_character_time_idx
    ON character_position_samples (character_id, sampled_at DESC);
CREATE INDEX character_position_session_time_idx
    ON character_position_samples (session_id, sampled_at DESC);
CREATE INDEX character_position_server_region_time_idx
    ON character_position_samples (lower(server_name), region, sampled_at DESC);

CREATE TABLE map_heatmap_resets (
    reset_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    layer TEXT NOT NULL CHECK (char_length(layer) BETWEEN 1 AND 64),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    dataset_id TEXT NOT NULL CHECK (dataset_id ~ '^gamedata-[a-z0-9]{1,64}$'),
    area_id TEXT NOT NULL CHECK (char_length(area_id) BETWEEN 1 AND 96),
    floor_id TEXT NOT NULL CHECK (char_length(floor_id) BETWEEN 1 AND 32),
    region INTEGER CHECK (region IS NULL OR region BETWEEN 1 AND 65535),
    character_id UUID REFERENCES characters(character_id),
    monster_type TEXT CHECK (monster_type IS NULL OR char_length(monster_type) <= 64),
    model_id BIGINT CHECK (model_id IS NULL OR model_id BETWEEN 0 AND 4294967295),
    from_time TIMESTAMPTZ NOT NULL,
    to_time TIMESTAMPTZ NOT NULL,
    broad_scope BOOLEAN NOT NULL DEFAULT FALSE,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (to_time > from_time)
);
CREATE INDEX map_heatmap_resets_scope_idx
    ON map_heatmap_resets (lower(server_name), layer, created_at DESC);

CREATE INDEX activity_events_map_heatmap_idx
    ON activity_events (lower(server_name), kind, occurred_at DESC, region)
    WHERE region IS NOT NULL AND x IS NOT NULL AND y IS NOT NULL;
