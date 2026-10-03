ALTER TABLE activity_events
    ADD COLUMN item_drop_class TEXT,
    ADD COLUMN item_drop_class_version TEXT,
    ADD CONSTRAINT activity_events_item_drop_class_check
        CHECK (item_drop_class IS NULL OR item_drop_class IN ('normal', 'rare')),
    ADD CONSTRAINT activity_events_item_drop_class_version_check
        CHECK (item_drop_class_version IS NULL OR char_length(item_drop_class_version) BETWEEN 1 AND 64),
    ADD CONSTRAINT activity_events_item_drop_class_pair_check
        CHECK ((item_drop_class IS NULL) = (item_drop_class_version IS NULL));

CREATE INDEX activity_events_drop_class_time_idx
    ON activity_events (item_drop_class, occurred_at DESC, event_id DESC)
    WHERE item_drop_class IS NOT NULL;
