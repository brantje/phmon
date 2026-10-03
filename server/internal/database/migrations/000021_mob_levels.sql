ALTER TABLE mob_observations
    ADD COLUMN resolved_level SMALLINT,
    ADD COLUMN level_source TEXT,
    ADD CONSTRAINT mob_observations_resolved_level_check
        CHECK (resolved_level IS NULL OR resolved_level BETWEEN 1 AND 255),
    ADD CONSTRAINT mob_observations_level_source_check
        CHECK ((resolved_level IS NULL AND level_source IS NULL)
            OR (resolved_level IS NOT NULL AND level_source IN ('runtime', 'catalog')));

-- The owning sample already carries dataset_id. For catalog levels that ID is
-- the exact reference catalog used at ingestion; legacy rows remain NULL.
