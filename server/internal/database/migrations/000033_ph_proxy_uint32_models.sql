-- V2 model references are positive uint32. V1 validation remains unchanged.
ALTER TABLE player_enrichments
    ALTER COLUMN model_ref_id TYPE BIGINT,
    DROP CONSTRAINT player_enrichments_model_ref_id_check,
    ADD CONSTRAINT player_enrichments_model_ref_id_check CHECK (model_ref_id IS NULL OR model_ref_id BETWEEN 1 AND 4294967295);
ALTER TABLE player_enrichment_pending
    ALTER COLUMN model_ref_id TYPE BIGINT,
    DROP CONSTRAINT player_enrichment_pending_model_ref_id_check,
    ADD CONSTRAINT player_enrichment_pending_model_ref_id_check CHECK (model_ref_id IS NULL OR model_ref_id BETWEEN 1 AND 4294967295);

-- PhBot tracking remains bounded to its protocol's 64-byte names. V2 may
-- stage longer verified names until compatible ordinary tracking exists.
ALTER TABLE player_enrichment_pending
    DROP CONSTRAINT player_enrichment_pending_name_key_check,
    DROP CONSTRAINT player_enrichment_pending_observed_name_check,
    ADD CONSTRAINT player_enrichment_pending_name_key_check CHECK (octet_length(name_key) BETWEEN 1 AND 128),
    ADD CONSTRAINT player_enrichment_pending_observed_name_check CHECK (octet_length(observed_name) BETWEEN 1 AND 128);

CREATE INDEX player_observations_thief_identity_idx ON player_observations (player_id)
    WHERE source='thief_sighting';
