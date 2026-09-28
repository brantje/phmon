CREATE TABLE character_resource_state (
    character_id UUID PRIMARY KEY REFERENCES characters(character_id) ON DELETE CASCADE,
    agent_id UUID NOT NULL REFERENCES agents(agent_id),
    connection_generation BIGINT NOT NULL CHECK (connection_generation > 0),
    session_id UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

CREATE TABLE character_resource_observations (
    observation_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    observer_character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    server_key TEXT NOT NULL,
    guild_key TEXT NOT NULL DEFAULT '',
    resource_key TEXT NOT NULL,
    session_id UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    availability TEXT NOT NULL CHECK (availability IN ('observed', 'unavailable', 'not_observed')),
    payload JSONB NOT NULL,
    content_hash BYTEA NOT NULL CHECK (octet_length(content_hash) = 32),
    observed_at TIMESTAMPTZ,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    UNIQUE (observer_character_id, resource_key),
    CHECK (length(resource_key) BETWEEN 1 AND 64)
);

CREATE INDEX character_resource_observations_guild_latest
    ON character_resource_observations (server_key, guild_key, observed_at DESC)
    WHERE resource_key = 'guild_storage' AND guild_key <> '';

-- Slot rows make owner/container/source-slot queries possible without losing
-- the complete typed phBot observation retained in the parent JSON snapshot.
CREATE TABLE character_resource_items (
    observer_character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    owner_character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    server_key TEXT NOT NULL,
    guild_key TEXT NOT NULL DEFAULT '',
    container_key TEXT NOT NULL,
    pet_id TEXT NOT NULL DEFAULT '',
    source_slot INTEGER NOT NULL CHECK (source_slot >= 0),
    displayed_slot INTEGER CHECK (displayed_slot IS NULL OR displayed_slot >= 0),
    session_id UUID NOT NULL,
    revision BIGINT NOT NULL CHECK (revision > 0),
    observed_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    model BIGINT,
    name TEXT,
    server_name TEXT,
    quantity NUMERIC,
    plus NUMERIC,
    durability NUMERIC,
    raw_item JSONB NOT NULL,
    PRIMARY KEY (observer_character_id, container_key, pet_id, source_slot),
    CHECK (length(container_key) BETWEEN 1 AND 64),
    CHECK (length(pet_id) <= 64)
);

CREATE INDEX character_resource_items_owner_container_slot
    ON character_resource_items (owner_character_id, container_key, source_slot);
CREATE INDEX character_resource_items_guild_scope
    ON character_resource_items (server_key, guild_key, container_key, source_slot)
    WHERE guild_key <> '';
