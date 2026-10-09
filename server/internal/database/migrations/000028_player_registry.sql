CREATE TABLE players (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    server_key TEXT NOT NULL CHECK (char_length(server_key) BETWEEN 1 AND 100),
    observed_name TEXT NOT NULL CHECK (octet_length(observed_name) BETWEEN 1 AND 64),
    observed_name_key TEXT NOT NULL CHECK (octet_length(observed_name_key) BETWEEN 1 AND 64),
    player_name TEXT NULL CHECK (player_name IS NULL OR octet_length(player_name) BETWEEN 1 AND 64),
    job_name TEXT NULL CHECK (job_name IS NULL OR octet_length(job_name) BETWEEN 1 AND 64),
    level INTEGER NULL CHECK (level IS NULL OR level BETWEEN 1 AND 255),
    guild_name TEXT NULL CHECK (guild_name IS NULL OR octet_length(guild_name) BETWEEN 1 AND 64),
    job TEXT NULL CHECK (job IS NULL OR job IN ('none', 'trader', 'thief', 'hunter', 'unknown')),
    job_level INTEGER NULL CHECK (job_level IS NULL OR job_level BETWEEN 0 AND 255),
    is_jobbing BOOLEAN NULL,
    model_id BIGINT NULL CHECK (model_id IS NULL OR model_id BETWEEN 1 AND 4294967295),
    model_name TEXT NULL CHECK (model_name IS NULL OR (char_length(model_name) BETWEEN 1 AND 128 AND model_name = btrim(model_name))),
    last_region INTEGER NULL CHECK (last_region IS NULL OR (last_region <> 0 AND last_region BETWEEN -32768 AND 65535)),
    last_zone TEXT NULL CHECK (last_zone IS NULL OR char_length(last_zone) BETWEEN 1 AND 100),
    last_x DOUBLE PRECISION NULL,
    last_y DOUBLE PRECISION NULL,
    last_z DOUBLE PRECISION NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    state_observed_at TIMESTAMPTZ NOT NULL,
    last_history_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (server_key, observed_name_key),
    CHECK (first_seen_at <= last_seen_at),
    CHECK (
        (last_x IS NULL AND last_y IS NULL AND last_z IS NULL)
        OR (
            last_x IS NOT NULL AND last_y IS NOT NULL
            AND last_x BETWEEN -1000000 AND 1000000
            AND last_y BETWEEN -1000000 AND 1000000
            AND (last_z IS NULL OR last_z BETWEEN -1000000 AND 1000000)
        )
    )
);

CREATE INDEX players_last_seen_idx ON players (last_seen_at DESC, id DESC);
CREATE INDEX players_server_last_seen_idx ON players (server_key, last_seen_at DESC, id DESC);

CREATE TABLE player_observations (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    server_key TEXT NOT NULL,
    observed_name TEXT NOT NULL,
    player_name TEXT NULL,
    job_name TEXT NULL,
    observer_agent_id UUID NULL,
    observer_character_id UUID NULL,
    observer_session_id UUID NULL,
    runtime_entity_id TEXT NULL CHECK (runtime_entity_id IS NULL OR octet_length(runtime_entity_id) BETWEEN 1 AND 64),
    level INTEGER NULL CHECK (level IS NULL OR level BETWEEN 1 AND 255),
    guild_name TEXT NULL,
    job TEXT NULL CHECK (job IS NULL OR job IN ('none', 'trader', 'thief', 'hunter', 'unknown')),
    job_level INTEGER NULL CHECK (job_level IS NULL OR job_level BETWEEN 0 AND 255),
    is_jobbing BOOLEAN NULL,
    model_id BIGINT NULL CHECK (model_id IS NULL OR model_id BETWEEN 1 AND 4294967295),
    region INTEGER NULL CHECK (region IS NULL OR (region <> 0 AND region BETWEEN -32768 AND 65535)),
    zone TEXT NULL,
    x DOUBLE PRECISION NULL,
    y DOUBLE PRECISION NULL,
    z DOUBLE PRECISION NULL,
    source TEXT NOT NULL CHECK (source IN ('map.players', 'spawn', 'thief_sighting')),
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    dedupe_key TEXT NOT NULL UNIQUE,
    CHECK (
        (x IS NULL AND y IS NULL AND z IS NULL)
        OR (
            x IS NOT NULL AND y IS NOT NULL
            AND x BETWEEN -1000000 AND 1000000
            AND y BETWEEN -1000000 AND 1000000
            AND (z IS NULL OR z BETWEEN -1000000 AND 1000000)
        )
    )
);

CREATE INDEX player_observations_player_observed_idx
    ON player_observations (player_id, observed_at DESC, id DESC);

CREATE TABLE player_level_snapshots (
    id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    player_id UUID NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    level INTEGER NOT NULL CHECK (level BETWEEN 1 AND 255),
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    observed_name TEXT NOT NULL,
    player_name TEXT NULL,
    job_name TEXT NULL,
    guild_name TEXT NULL,
    job TEXT NULL CHECK (job IS NULL OR job IN ('none', 'trader', 'thief', 'hunter', 'unknown')),
    job_level INTEGER NULL CHECK (job_level IS NULL OR job_level BETWEEN 0 AND 255),
    is_jobbing BOOLEAN NULL,
    model_id BIGINT NULL CHECK (model_id IS NULL OR model_id BETWEEN 1 AND 4294967295),
    region INTEGER NULL CHECK (region IS NULL OR (region <> 0 AND region BETWEEN -32768 AND 65535)),
    zone TEXT NULL,
    x DOUBLE PRECISION NULL,
    y DOUBLE PRECISION NULL,
    z DOUBLE PRECISION NULL,
    source TEXT NOT NULL CHECK (source IN ('map.players', 'spawn', 'thief_sighting')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (player_id, level),
    CHECK (first_seen_at <= last_seen_at)
);

CREATE INDEX player_level_snapshots_player_first_idx
    ON player_level_snapshots (player_id, first_seen_at DESC, level DESC);
