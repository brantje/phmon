-- Complete Slice 2 fields for databases where the first Slice 2 migration was
-- exercised during development before connection/server identity fencing landed.
ALTER TABLE characters ADD COLUMN IF NOT EXISTS server_key TEXT;
UPDATE characters SET server_key = lower(server_name) WHERE server_key IS NULL;
ALTER TABLE characters ALTER COLUMN server_key SET NOT NULL;

ALTER TABLE character_sessions ADD COLUMN IF NOT EXISTS connection_generation BIGINT;
UPDATE character_sessions SET connection_generation = 1 WHERE connection_generation IS NULL;
ALTER TABLE character_sessions ALTER COLUMN connection_generation SET NOT NULL;
ALTER TABLE character_sessions ALTER COLUMN connection_generation SET DEFAULT 1;
ALTER TABLE character_sessions DROP CONSTRAINT IF EXISTS character_sessions_connection_generation_check;
ALTER TABLE character_sessions ADD CONSTRAINT character_sessions_connection_generation_check CHECK (connection_generation > 0);

ALTER TABLE characters DROP CONSTRAINT IF EXISTS characters_server_name_identity_key_key;
ALTER TABLE characters DROP CONSTRAINT IF EXISTS characters_server_key_identity_key_key;
ALTER TABLE characters ADD CONSTRAINT characters_server_scope_identity_key UNIQUE (server_key, identity_key);
