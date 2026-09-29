ALTER TABLE mob_observation_samples
    DROP CONSTRAINT IF EXISTS mob_observation_samples_region_check;
ALTER TABLE mob_observation_samples
    ADD CONSTRAINT mob_observation_samples_region_check
    CHECK (region BETWEEN -32768 AND 65535 AND region <> 0);

ALTER TABLE mob_observations
    DROP CONSTRAINT IF EXISTS mob_observations_region_check;
ALTER TABLE mob_observations
    ADD CONSTRAINT mob_observations_region_check
    CHECK (region BETWEEN -32768 AND 65535 AND region <> 0);
