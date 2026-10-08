CREATE TABLE players (
    id UUID PRIMARY KEY,
    server_key TEXT NOT NULL CHECK (char_length(server_key) BETWEEN 1 AND 100),
    server_name TEXT NOT NULL,
    name TEXT NULL,
    level INTEGER NULL CHECK (level BETWEEN 1 AND 255),
    guild_name TEXT NULL,
    job TEXT NULL CHECK (job IN ('trader','thief','hunter','none','unknown')),
    job_name TEXT NULL,
    gear JSONB NULL,
    gear_hash TEXT NULL,
    identity_gear_hash TEXT NULL,
    field_times JSONB NOT NULL DEFAULT '{}',
    location JSONB NULL,
    character_model_id BIGINT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    revision BIGINT NOT NULL DEFAULT 1,
    UNIQUE (id, server_key),
    CHECK (first_seen_at <= last_seen_at)
);
CREATE UNIQUE INDEX players_confirmed_name_idx ON players (server_key, lower(name)) WHERE name IS NOT NULL;
CREATE INDEX players_last_seen_idx ON players (server_key, last_seen_at DESC, id DESC);

CREATE TABLE player_aliases (
    id UUID PRIMARY KEY,
    player_id UUID NOT NULL,
    server_key TEXT NOT NULL,
    alias_name TEXT NOT NULL CHECK (octet_length(alias_name) BETWEEN 1 AND 64),
    alias_type TEXT NOT NULL CHECK (alias_type IN ('normal','job','unknown')),
    job_type TEXT NULL CHECK (job_type IN ('trader','thief','hunter','none','unknown')),
    match_method TEXT NOT NULL,
    confidence_score INTEGER NULL,
    confirmation_status TEXT NOT NULL CHECK (confirmation_status IN ('provisional','confirmed','conflict')),
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    FOREIGN KEY (player_id, server_key) REFERENCES players (id, server_key) ON DELETE CASCADE,
    UNIQUE (player_id, alias_name, alias_type),
    CHECK (first_seen_at <= last_seen_at)
);
CREATE INDEX player_aliases_lookup_idx ON player_aliases (server_key, lower(alias_name));
CREATE UNIQUE INDEX player_aliases_confirmed_normal_idx ON player_aliases (server_key, lower(alias_name))
    WHERE alias_type='normal' AND confirmation_status='confirmed';

CREATE TABLE player_observations (
    id UUID PRIMARY KEY,
    player_id UUID NOT NULL,
    server_key TEXT NOT NULL,
    observed_name TEXT NOT NULL,
    observer_agent_id TEXT NULL,
    observer_character_id TEXT NULL,
    observer_session_id TEXT NULL,
    runtime_entity_id TEXT NULL,
    runtime_epoch TEXT NOT NULL DEFAULT '',
    character_model_id BIGINT NULL,
    level INTEGER NULL,
    guild_name TEXT NULL,
    job_type TEXT NULL,
    region INTEGER NULL,
    x DOUBLE PRECISION NULL,
    y DOUBLE PRECISION NULL,
    z DOUBLE PRECISION NULL,
    equipment_json JSONB NULL,
    gear_hash TEXT NULL,
    identity_gear_hash TEXT NULL,
    source TEXT NOT NULL,
    source_ref TEXT NOT NULL,
    signature TEXT NOT NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    evidence JSONB NOT NULL,
    pinned BOOLEAN NOT NULL DEFAULT false,
    FOREIGN KEY (player_id, server_key) REFERENCES players (id, server_key) ON DELETE CASCADE,
    UNIQUE (server_key, source, source_ref)
);
CREATE INDEX player_observations_history_idx ON player_observations (player_id, observed_at DESC, id DESC);
CREATE INDEX player_observations_runtime_idx ON player_observations (server_key, observer_session_id, runtime_epoch, runtime_entity_id, observed_at DESC);
CREATE INDEX player_observations_retention_idx ON player_observations (received_at) WHERE NOT pinned;

CREATE TABLE player_equipment_history (
    id UUID PRIMARY KEY,
    player_id UUID NOT NULL REFERENCES players (id) ON DELETE CASCADE,
    observation_id UUID NOT NULL REFERENCES player_observations (id),
    equipment_json JSONB NOT NULL,
    last_evidence JSONB NOT NULL,
    gear_hash TEXT NOT NULL,
    identity_gear_hash TEXT NULL,
    first_seen_at TIMESTAMPTZ NOT NULL,
    last_seen_at TIMESTAMPTZ NOT NULL,
    CHECK (first_seen_at <= last_seen_at)
);
CREATE INDEX player_equipment_history_idx ON player_equipment_history (player_id, first_seen_at DESC, id DESC);

CREATE TABLE player_identity_links (
    id UUID PRIMARY KEY,
    server_key TEXT NOT NULL,
    canonical_player_id UUID NOT NULL,
    linked_player_id UUID NOT NULL,
    alias_id UUID NULL REFERENCES player_aliases (id),
    decision TEXT NOT NULL CHECK (decision IN ('candidate','confirm','reject','unlink')),
    method TEXT NOT NULL,
    confidence_score INTEGER NULL,
    evidence_json JSONB NOT NULL,
    status TEXT NOT NULL CHECK (status IN ('pending','confirmed','rejected','revoked')),
    candidate_key TEXT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    decided_at TIMESTAMPTZ NULL,
    decided_by TEXT NULL,
    revision BIGINT NOT NULL DEFAULT 1,
    FOREIGN KEY (canonical_player_id, server_key) REFERENCES players (id, server_key),
    FOREIGN KEY (linked_player_id, server_key) REFERENCES players (id, server_key),
    CHECK (canonical_player_id <> linked_player_id)
);
CREATE UNIQUE INDEX player_identity_links_active_idx ON player_identity_links (linked_player_id) WHERE status='confirmed';
CREATE UNIQUE INDEX player_identity_candidates_idx ON player_identity_links (candidate_key) WHERE candidate_key IS NOT NULL;
CREATE INDEX player_identity_links_canonical_idx ON player_identity_links (canonical_player_id, status);

-- A read projection, not a destructive merge. Revocation restores source rows.
CREATE VIEW player_registry AS
WITH membership AS (
    SELECT p.id AS source_id, COALESCE(l.canonical_player_id,p.id) AS canonical_id
    FROM players p LEFT JOIN player_identity_links l ON l.linked_player_id=p.id AND l.status='confirmed'
), grouped AS (
    SELECT m.canonical_id AS id, array_agg(m.source_id) AS source_ids,
           min(p.first_seen_at) AS first_seen_at, max(p.last_seen_at) AS last_seen_at
    FROM membership m JOIN players p ON p.id=m.source_id GROUP BY m.canonical_id
)
SELECT g.id, c.server_key, c.server_name, g.source_ids, g.first_seen_at, g.last_seen_at,
    (SELECT p.name FROM players p WHERE p.id=ANY(g.source_ids) AND p.name IS NOT NULL ORDER BY (p.field_times->>'name')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS name,
    (SELECT p.level FROM players p WHERE p.id=ANY(g.source_ids) AND p.level IS NOT NULL ORDER BY (p.field_times->>'level')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS level,
    (SELECT p.guild_name FROM players p WHERE p.id=ANY(g.source_ids) AND p.guild_name IS NOT NULL ORDER BY (p.field_times->>'guild')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS guild_name,
    (SELECT p.job FROM players p WHERE p.id=ANY(g.source_ids) AND p.job IS NOT NULL ORDER BY (p.field_times->>'job')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS job,
    (SELECT p.job_name FROM players p WHERE p.id=ANY(g.source_ids) AND p.job_name IS NOT NULL ORDER BY (p.field_times->>'job_name')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS job_name,
    (SELECT p.gear FROM players p WHERE p.id=ANY(g.source_ids) AND p.gear IS NOT NULL ORDER BY (p.field_times->>'gear')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS gear,
    (SELECT p.gear_hash FROM players p WHERE p.id=ANY(g.source_ids) AND p.gear IS NOT NULL ORDER BY (p.field_times->>'gear')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS gear_hash,
    (SELECT p.identity_gear_hash FROM players p WHERE p.id=ANY(g.source_ids) AND p.gear IS NOT NULL ORDER BY (p.field_times->>'gear')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS identity_gear_hash,
    (SELECT p.location FROM players p WHERE p.id=ANY(g.source_ids) AND p.location IS NOT NULL ORDER BY (p.field_times->>'location')::timestamptz DESC NULLS LAST,p.id LIMIT 1) AS location,
    (SELECT a.alias_name FROM player_aliases a WHERE a.player_id=ANY(g.source_ids) ORDER BY a.last_seen_at DESC,a.id LIMIT 1) AS observed_name,
    (EXISTS (SELECT 1 FROM player_aliases a WHERE a.player_id=ANY(g.source_ids) AND a.confirmation_status='confirmed' AND a.alias_type='normal') OR cardinality(g.source_ids)>1) AS resolved,
    c.revision
FROM grouped g JOIN players c ON c.id=g.id;
