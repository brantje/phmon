ALTER TABLE characters ADD COLUMN dead BOOLEAN;

CREATE TABLE activity_events (
    event_id UUID PRIMARY KEY,
    schema_version INTEGER NOT NULL CHECK (schema_version > 0),
    kind TEXT NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 80),
    category TEXT NOT NULL CHECK (char_length(category) BETWEEN 1 AND 40),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    character_id UUID NOT NULL REFERENCES characters(character_id),
    session_id UUID NOT NULL REFERENCES character_sessions(session_id),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    occurred_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    source TEXT NOT NULL CHECK (char_length(source) BETWEEN 1 AND 80),
    source_ref TEXT NOT NULL CHECK (char_length(source_ref) BETWEEN 1 AND 120),
    region INTEGER CHECK (region IS NULL OR region BETWEEN 0 AND 65535),
    x DOUBLE PRECISION CHECK (x IS NULL OR x BETWEEN -1000000 AND 1000000),
    y DOUBLE PRECISION CHECK (y IS NULL OR y BETWEEN -1000000 AND 1000000),
    z DOUBLE PRECISION CHECK (z IS NULL OR z BETWEEN -1000000 AND 1000000),
    payload JSONB NOT NULL DEFAULT '{}'::jsonb CHECK (octet_length(payload::text) <= 4096)
);

CREATE INDEX activity_events_server_time_idx
    ON activity_events (lower(server_name), occurred_at DESC, event_id DESC);
CREATE INDEX activity_events_character_time_idx
    ON activity_events (character_id, occurred_at DESC, event_id DESC);
CREATE INDEX activity_events_kind_time_idx
    ON activity_events (kind, occurred_at DESC, event_id DESC);
