ALTER TABLE character_position_samples
    DROP CONSTRAINT IF EXISTS character_position_samples_region_check;
ALTER TABLE character_position_samples
    ADD CONSTRAINT character_position_samples_region_check
    CHECK (region BETWEEN -32768 AND 65535 AND region <> 0);

ALTER TABLE map_heatmap_resets
    DROP CONSTRAINT IF EXISTS map_heatmap_resets_region_check;
ALTER TABLE map_heatmap_resets
    ADD CONSTRAINT map_heatmap_resets_region_check
    CHECK (region IS NULL OR (region BETWEEN -32768 AND 65535 AND region <> 0));
