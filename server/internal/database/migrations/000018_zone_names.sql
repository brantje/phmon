ALTER TABLE activity_events
    ADD COLUMN zone_name TEXT
        CHECK (zone_name IS NULL OR char_length(zone_name) BETWEEN 1 AND 100);

ALTER TABLE character_control_state
    ADD COLUMN training_zone TEXT
        CHECK (training_zone IS NULL OR char_length(training_zone) BETWEEN 1 AND 100);
