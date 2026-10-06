CREATE TABLE thief_sightings (
    sighting_id UUID PRIMARY KEY,
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    thief_name TEXT NOT NULL CHECK (octet_length(thief_name) BETWEEN 1 AND 64),
    region INTEGER,
    x DOUBLE PRECISION,
    y DOUBLE PRECISION,
    z DOUBLE PRECISION,
    position_source TEXT NOT NULL CHECK (position_source IN ('thief', 'observer', 'unknown')),
    reporter_name TEXT NOT NULL DEFAULT '' CHECK (octet_length(reporter_name) <= 64),
    reporter_app TEXT NOT NULL DEFAULT '' CHECK (octet_length(reporter_app) <= 64),
    reporter_version TEXT NOT NULL DEFAULT '' CHECK (octet_length(reporter_version) <= 64),
    origin TEXT NOT NULL CHECK (origin IN ('phmon', 'external')),
    event_id UUID NULL REFERENCES activity_events (event_id) ON DELETE SET NULL,
    observed_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CHECK (
        (region IS NULL AND x IS NULL AND y IS NULL AND z IS NULL)
        OR (
            region IS NOT NULL AND region <> 0 AND region BETWEEN -32768 AND 65535
            AND x IS NOT NULL AND y IS NOT NULL
            AND x BETWEEN -1000000 AND 1000000
            AND y BETWEEN -1000000 AND 1000000
            AND (z IS NULL OR z BETWEEN -1000000 AND 1000000)
        )
    )
);

CREATE INDEX thief_sightings_server_received_idx
    ON thief_sightings (lower(server_name), received_at DESC, sighting_id DESC);

CREATE INDEX thief_sightings_server_thief_received_idx
    ON thief_sightings (lower(server_name), lower(thief_name), received_at DESC);
