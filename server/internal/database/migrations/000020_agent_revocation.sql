ALTER TABLE agents
    ADD COLUMN revoked_at TIMESTAMPTZ;

CREATE INDEX agents_active_created_idx
    ON agents (created_at DESC, agent_id)
    WHERE revoked_at IS NULL;
