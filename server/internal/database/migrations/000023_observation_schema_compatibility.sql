-- Some deployed instances applied an earlier migration 21 with level,
-- level_source and level_dataset_id instead of the final resolved_level schema.
-- Keep that historical provenance intact while providing the active columns.
DO $$
BEGIN
    IF EXISTS (SELECT 1 FROM information_schema.columns
               WHERE table_schema = current_schema() AND table_name = 'mob_observations'
                 AND column_name = 'level')
       AND NOT EXISTS (SELECT 1 FROM information_schema.columns
                       WHERE table_schema = current_schema() AND table_name = 'mob_observations'
                         AND column_name = 'resolved_level') THEN
        ALTER TABLE mob_observations RENAME COLUMN level_source TO legacy_level_source;
        ALTER TABLE mob_observations ADD COLUMN resolved_level SMALLINT;
        ALTER TABLE mob_observations ADD COLUMN level_source TEXT;
        UPDATE mob_observations SET resolved_level = level, level_source = legacy_level_source;
        ALTER TABLE mob_observations ADD CONSTRAINT mob_observations_resolved_level_check
            CHECK (resolved_level IS NULL OR resolved_level BETWEEN 1 AND 255);
        ALTER TABLE mob_observations ADD CONSTRAINT mob_observations_resolved_level_source_check
            CHECK ((resolved_level IS NULL AND level_source IS NULL)
                OR (resolved_level IS NOT NULL AND level_source IN ('runtime', 'catalog')));
    END IF;
END $$;

-- The plugin/event validator already supports signed cave regions. Previously
-- spooled cave events must be persistable without blocking the retry stream.
ALTER TABLE activity_events DROP CONSTRAINT IF EXISTS activity_events_region_check;
ALTER TABLE activity_events ADD CONSTRAINT activity_events_region_check
    CHECK (region IS NULL OR region BETWEEN -32768 AND 65535);
