-- Invalid historical source rows must not block the bounded registry importer.
-- Keep their originals for a grace period without creating fictional players.
ALTER TABLE thief_sightings
    ADD COLUMN registry_rejected_at TIMESTAMPTZ,
    ADD COLUMN registry_rejection_reason TEXT,
    ADD CONSTRAINT thief_registry_rejection_check CHECK (
        (registry_rejected_at IS NULL AND registry_rejection_reason IS NULL)
        OR (registry_rejected_at IS NOT NULL AND registry_rejection_reason IS NOT NULL
            AND registry_rejection_reason = 'invalid_player_observation')
    );

CREATE INDEX thief_sightings_registry_pending_idx
    ON thief_sightings (received_at, sighting_id)
    WHERE registry_rejected_at IS NULL;
