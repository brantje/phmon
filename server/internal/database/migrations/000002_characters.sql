CREATE TABLE characters (
    character_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    server_key TEXT NOT NULL,
    character_name TEXT NOT NULL CHECK (char_length(character_name) BETWEEN 1 AND 64),
    identity_key TEXT NOT NULL,
    guild_name TEXT,
    zone_name TEXT,
    region INTEGER,
    level INTEGER CHECK (level IS NULL OR level BETWEEN 0 AND 255),
    hp BIGINT CHECK (hp IS NULL OR hp >= 0),
    hp_max BIGINT CHECK (hp_max IS NULL OR hp_max >= 0),
    mp BIGINT CHECK (mp IS NULL OR mp >= 0),
    mp_max BIGINT CHECK (mp_max IS NULL OR mp_max >= 0),
    current_exp BIGINT CHECK (current_exp IS NULL OR current_exp >= 0),
    max_exp BIGINT CHECK (max_exp IS NULL OR max_exp >= 0),
    sp BIGINT CHECK (sp IS NULL OR sp >= 0),
    gold BIGINT CHECK (gold IS NULL OR gold >= 0),
    x DOUBLE PRECISION,
    y DOUBLE PRECISION,
    z DOUBLE PRECISION,
    botting BOOLEAN,
    state_updated_at TIMESTAMPTZ,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (server_key, identity_key)
);

CREATE INDEX characters_search_idx ON characters (lower(character_name), lower(server_name));
CREATE TABLE character_sessions (
    session_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(character_id),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    connection_generation BIGINT NOT NULL CHECK (connection_generation > 0),
    started_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    last_activity_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    ended_at TIMESTAMPTZ,
    end_reason TEXT CHECK (end_reason IS NULL OR end_reason IN ('left', 'switched', 'agent_disconnected', 'backend_restart'))
);
CREATE UNIQUE INDEX character_sessions_one_active_idx ON character_sessions (character_id) WHERE ended_at IS NULL;
CREATE INDEX character_sessions_agent_active_idx ON character_sessions (agent_id) WHERE ended_at IS NULL;

CREATE TABLE character_groups (
    group_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    name TEXT NOT NULL UNIQUE CHECK (char_length(name) BETWEEN 1 AND 80),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE TABLE character_group_members (
    group_id UUID NOT NULL REFERENCES character_groups(group_id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    added_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (group_id, character_id)
);

-- A process restart invalidates socket sessions; durable characters/state remain.
UPDATE character_sessions SET ended_at = now(), end_reason = 'backend_restart' WHERE ended_at IS NULL;
