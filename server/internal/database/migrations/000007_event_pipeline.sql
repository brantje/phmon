-- Generalize the death occurrence table into the canonical activity stream.
ALTER TABLE activity_events
    ALTER COLUMN character_id DROP NOT NULL,
    ALTER COLUMN session_id DROP NOT NULL,
    ALTER COLUMN server_name DROP NOT NULL,
    DROP CONSTRAINT activity_events_payload_check,
    ADD CONSTRAINT activity_events_payload_size_check
        CHECK (octet_length(payload::text) <= 32768);

ALTER TABLE activity_events
    ADD COLUMN sequence BIGINT CHECK (sequence IS NULL OR sequence > 0),
    ADD COLUMN dedupe_key TEXT CHECK (dedupe_key IS NULL OR char_length(dedupe_key) BETWEEN 1 AND 160),
    ADD COLUMN item_model BIGINT CHECK (item_model IS NULL OR item_model >= 0),
    ADD COLUMN item_code TEXT CHECK (item_code IS NULL OR char_length(item_code) BETWEEN 1 AND 128);

CREATE UNIQUE INDEX activity_events_session_sequence_idx
    ON activity_events (session_id, sequence)
    WHERE session_id IS NOT NULL AND sequence IS NOT NULL;

CREATE UNIQUE INDEX activity_events_dedupe_idx
    ON activity_events (agent_id, source, dedupe_key)
    WHERE dedupe_key IS NOT NULL;

CREATE INDEX activity_events_category_time_idx
    ON activity_events (category, occurred_at DESC, event_id DESC);

CREATE INDEX activity_events_item_time_idx
    ON activity_events (item_model, occurred_at DESC, event_id DESC)
    WHERE item_model IS NOT NULL;

CREATE INDEX activity_events_item_code_time_idx
    ON activity_events (lower(item_code), occurred_at DESC, event_id DESC)
    WHERE item_code IS NOT NULL;
