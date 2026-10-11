-- Personal storage gold observations for per-character balance history.
-- Samples are written when an accepted storage resource observation includes gold.
CREATE TABLE character_storage_gold_samples (
    sample_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    character_id UUID NOT NULL REFERENCES characters(character_id) ON DELETE CASCADE,
    session_id UUID NOT NULL REFERENCES character_sessions(session_id) ON DELETE CASCADE,
    resource_revision BIGINT NOT NULL CHECK (resource_revision > 0),
    sampled_at TIMESTAMPTZ NOT NULL,
    gold BIGINT NOT NULL CHECK (gold >= 0),
    created_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
CREATE INDEX character_storage_gold_character_time_idx
    ON character_storage_gold_samples (character_id, sampled_at DESC);
CREATE INDEX character_storage_gold_session_time_idx
    ON character_storage_gold_samples (session_id, sampled_at DESC);
