CREATE TABLE mob_observation_samples (
    sample_id UUID PRIMARY KEY,
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    character_id UUID NOT NULL REFERENCES characters(character_id),
    session_id UUID NOT NULL REFERENCES character_sessions(session_id),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    dataset_id TEXT NOT NULL CHECK (dataset_id ~ '^gamedata-[a-z0-9]{1,64}$'),
    area_id TEXT NOT NULL CHECK (char_length(area_id) BETWEEN 1 AND 96),
    floor_id TEXT NOT NULL CHECK (char_length(floor_id) BETWEEN 1 AND 32),
    region INTEGER NOT NULL CHECK (region BETWEEN 1 AND 65535),
    sampled_at TIMESTAMPTZ NOT NULL,
    sample_hash BYTEA NOT NULL CHECK (octet_length(sample_hash) = 32),
    sample_minute TIMESTAMPTZ NOT NULL,
    observer_x DOUBLE PRECISION NOT NULL CHECK (observer_x BETWEEN -1000000 AND 1000000),
    observer_y DOUBLE PRECISION NOT NULL CHECK (observer_y BETWEEN -1000000 AND 1000000),
    observer_z DOUBLE PRECISION CHECK (observer_z IS NULL OR observer_z BETWEEN -1000000 AND 1000000),
    observer_cell_x INTEGER NOT NULL,
    observer_cell_y INTEGER NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (sample_minute = date_trunc('minute', sampled_at))
);

-- A replay uses sample_id. This additional key prevents a restarted agent from
-- creating a second denominator observation in the same minute and observer cell.
CREATE UNIQUE INDEX mob_observation_sampling_rate_idx
    ON mob_observation_samples (session_id, area_id, floor_id, observer_cell_x, observer_cell_y, sample_minute);
CREATE INDEX mob_observation_scope_time_idx
    ON mob_observation_samples (lower(server_name), area_id, floor_id, sampled_at DESC);
CREATE INDEX mob_observation_session_idx
    ON mob_observation_samples (session_id, sampled_at DESC);

CREATE TABLE mob_observations (
    sample_id UUID NOT NULL REFERENCES mob_observation_samples(sample_id) ON DELETE CASCADE,
    ordinal SMALLINT NOT NULL CHECK (ordinal BETWEEN 0 AND 127),
    monster_id TEXT NOT NULL CHECK (char_length(monster_id) BETWEEN 1 AND 64),
    model_id BIGINT CHECK (model_id IS NULL OR model_id BETWEEN 0 AND 4294967295),
    monster_type TEXT NOT NULL DEFAULT '' CHECK (char_length(monster_type) <= 64),
    region INTEGER NOT NULL CHECK (region BETWEEN 1 AND 65535),
    x DOUBLE PRECISION NOT NULL CHECK (x BETWEEN -1000000 AND 1000000),
    y DOUBLE PRECISION NOT NULL CHECK (y BETWEEN -1000000 AND 1000000),
    z DOUBLE PRECISION CHECK (z IS NULL OR z BETWEEN -1000000 AND 1000000),
    PRIMARY KEY (sample_id, ordinal)
);
CREATE INDEX mob_observations_model_sample_idx ON mob_observations (model_id, sample_id) WHERE model_id IS NOT NULL;
CREATE INDEX mob_observations_type_sample_idx ON mob_observations (monster_type, sample_id) WHERE monster_type <> '';
