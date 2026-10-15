CREATE TABLE player_enrichments (
    player_id UUID PRIMARY KEY REFERENCES players (id) ON DELETE CASCADE,
    model_ref_id INTEGER NULL CHECK (model_ref_id IS NULL OR model_ref_id BETWEEN 1 AND 2147483647),
    model_ref_observed_at TIMESTAMPTZ NULL,
    gear JSONB NOT NULL DEFAULT '[]'::jsonb,
    gear_observed_at TIMESTAMPTZ NULL,
    field_evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE player_enrichment_pending (
    agent_id UUID NOT NULL,
    server_key TEXT NOT NULL CHECK (char_length(server_key) BETWEEN 1 AND 100),
    name_key TEXT NOT NULL CHECK (octet_length(name_key) BETWEEN 1 AND 64),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    observed_name TEXT NOT NULL CHECK (octet_length(observed_name) BETWEEN 1 AND 64),
    model_ref_id INTEGER NULL CHECK (model_ref_id IS NULL OR model_ref_id BETWEEN 1 AND 2147483647),
    model_ref_observed_at TIMESTAMPTZ NULL,
    gear JSONB NOT NULL DEFAULT '[]'::jsonb,
    gear_observed_at TIMESTAMPTZ NULL,
    field_evidence JSONB NOT NULL DEFAULT '{}'::jsonb,
    expires_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, server_key, name_key)
);

CREATE INDEX player_enrichment_pending_expires_idx ON player_enrichment_pending (expires_at);

CREATE TABLE ph_proxy_enrichment_requests (
    agent_id UUID NOT NULL,
    request_id UUID NOT NULL,
    body_hash BYTEA NOT NULL,
    response_json JSONB NOT NULL,
    created_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (agent_id, request_id)
);

CREATE INDEX ph_proxy_enrichment_requests_created_idx
    ON ph_proxy_enrichment_requests (created_at);

CREATE TABLE ph_proxy_server_status (
    server_key TEXT PRIMARY KEY CHECK (char_length(server_key) BETWEEN 1 AND 100),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    gateway_name TEXT NULL CHECK (gateway_name IS NULL OR char_length(gateway_name) BETWEEN 1 AND 128),
    gateway_online INTEGER NULL CHECK (gateway_online IS NULL OR gateway_online BETWEEN 0 AND 1000000),
    gateway_max INTEGER NULL CHECK (gateway_max IS NULL OR gateway_max BETWEEN 0 AND 1000000),
    gateway_observed_at TIMESTAMPTZ NULL,
    coverage_profile TEXT NULL CHECK (coverage_profile IS NULL OR char_length(coverage_profile) BETWEEN 1 AND 128),
    coverage_dropped INTEGER NULL CHECK (coverage_dropped IS NULL OR coverage_dropped BETWEEN 0 AND 1000000000),
    coverage_capacity_reached BOOLEAN NULL,
    last_receipt_at TIMESTAMPTZ NOT NULL,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
