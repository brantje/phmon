CREATE TABLE agents (
    agent_id UUID PRIMARY KEY,
    token_hash BYTEA NOT NULL UNIQUE CHECK (octet_length(token_hash) = 32),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    first_seen_at TIMESTAMPTZ,
    last_seen_at TIMESTAMPTZ,
    last_connected_at TIMESTAMPTZ,
    last_disconnected_at TIMESTAMPTZ,
    protocol_version INTEGER,
    plugin_version TEXT CHECK (plugin_version IS NULL OR char_length(plugin_version) <= 64),
    phbot_version TEXT CHECK (phbot_version IS NULL OR char_length(phbot_version) <= 64)
);

CREATE INDEX agents_last_seen_idx ON agents (last_seen_at DESC NULLS LAST);
