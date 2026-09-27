CREATE TABLE commands (
    command_id TEXT PRIMARY KEY DEFAULT ('cmd_' || gen_random_uuid()::text),
    character_id UUID NOT NULL REFERENCES characters(character_id),
    session_id UUID NOT NULL REFERENCES character_sessions(session_id),
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    connection_generation BIGINT NOT NULL CHECK (connection_generation > 0),
    operator_identity TEXT NOT NULL CHECK (char_length(operator_identity) BETWEEN 1 AND 64),
    idempotency_key TEXT NOT NULL CHECK (char_length(idempotency_key) BETWEEN 1 AND 128),
    request_hash BYTEA NOT NULL CHECK (octet_length(request_hash) = 32),
    command_name TEXT NOT NULL CHECK (char_length(command_name) BETWEEN 1 AND 64),
    schema_version INTEGER NOT NULL DEFAULT 1 CHECK (schema_version = 1),
    args JSONB NOT NULL,
    state TEXT NOT NULL CHECK (state IN ('queued','dispatching','sent','acknowledged','completed','failed','expired','unknown')),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    expires_at TIMESTAMPTZ NOT NULL,
    dispatch_started_at TIMESTAMPTZ,
    sent_at TIMESTAMPTZ,
    acknowledged_at TIMESTAMPTZ,
    finished_at TIMESTAMPTZ,
    result_code TEXT CHECK (result_code IS NULL OR char_length(result_code) <= 64),
    result_message TEXT CHECK (result_message IS NULL OR char_length(result_message) <= 512),
    verification TEXT CHECK (verification IS NULL OR verification IN ('api_confirmed','observed','unverified')),
    api_return JSONB,
    effective_args JSONB,
    observed_after JSONB,
    CONSTRAINT commands_idempotency_unique UNIQUE (operator_identity, idempotency_key),
    CHECK (expires_at > created_at)
);

CREATE UNIQUE INDEX commands_one_inflight_per_character_idx
    ON commands (character_id)
    WHERE state IN ('queued','dispatching','sent','acknowledged');

CREATE INDEX commands_character_history_idx
    ON commands (character_id, created_at DESC, command_id DESC);
CREATE INDEX commands_pending_deadline_idx
    ON commands (expires_at)
    WHERE state IN ('queued','dispatching','sent','acknowledged');

CREATE TABLE command_events (
    event_id BIGINT GENERATED ALWAYS AS IDENTITY PRIMARY KEY,
    command_id TEXT NOT NULL REFERENCES commands(command_id),
    happened_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    kind TEXT NOT NULL CHECK (char_length(kind) BETWEEN 1 AND 64),
    evidence JSONB NOT NULL DEFAULT '{}'::jsonb
);
CREATE INDEX command_events_command_idx ON command_events (command_id, event_id);

CREATE TABLE character_control_state (
    session_id UUID PRIMARY KEY REFERENCES character_sessions(session_id) ON DELETE CASCADE,
    character_id UUID NOT NULL REFERENCES characters(character_id),
    training_available BOOLEAN NOT NULL DEFAULT false,
    training_region INTEGER,
    training_x DOUBLE PRECISION,
    training_y DOUBLE PRECISION,
    training_z DOUBLE PRECISION,
    training_radius DOUBLE PRECISION,
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
