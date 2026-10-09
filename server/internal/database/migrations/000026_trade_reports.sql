CREATE TABLE trade_reports (
    trade_id UUID PRIMARY KEY,
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    server_key TEXT GENERATED ALWAYS AS (lower(server_name)) STORED,
    client_ref TEXT NOT NULL CHECK (octet_length(client_ref) BETWEEN 1 AND 64),
    outcome TEXT NOT NULL CHECK (outcome IN ('success', 'failed', 'cancelled')),
    reason TEXT NOT NULL,
    route_from TEXT NOT NULL CHECK (route_from IN (
        'Jangan', 'Donwhang', 'Hotan', 'Samarkand', 'Constantinople', 'Alexandria'
    )),
    route_to TEXT NOT NULL CHECK (route_to IN (
        'Jangan', 'Donwhang', 'Hotan', 'Samarkand', 'Constantinople', 'Alexandria'
    )),
    waypoints JSONB NOT NULL,
    goods JSONB,
    gold INTEGER,
    duration_s INTEGER,
    stars TEXT,
    detail TEXT,
    thief_name TEXT,
    transport TEXT,
    reporter_name TEXT NOT NULL CHECK (octet_length(reporter_name) BETWEEN 1 AND 64),
    reporter_app TEXT NOT NULL DEFAULT '' CHECK (octet_length(reporter_app) <= 64),
    reporter_version TEXT NOT NULL DEFAULT '' CHECK (octet_length(reporter_version) <= 64),
    finished_at TIMESTAMPTZ NOT NULL,
    received_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    CONSTRAINT trade_reports_server_ref_key UNIQUE (server_key, client_ref),
    CHECK (route_from <> route_to),
    CHECK (
        (outcome = 'success' AND reason IN ('sold', 'already_empty'))
        OR (outcome = 'failed' AND reason IN ('thief', 'navigation', 'error'))
        OR (outcome = 'cancelled' AND reason = 'cancelled')
    ),
    CHECK (
        (reason = 'thief' AND thief_name IS NOT NULL AND octet_length(thief_name) BETWEEN 1 AND 64)
        OR (reason <> 'thief' AND thief_name IS NULL)
    ),
    CHECK (
        detail IS NULL
        OR (
            reason IN ('navigation', 'error')
            AND octet_length(detail) BETWEEN 1 AND 256
        )
    ),
    CHECK (duration_s IS NULL OR duration_s BETWEEN 0 AND 86400),
    CHECK (stars IS NULL OR stars IN ('1', '2', '3', '4', '5', 'Max')),
    CHECK (transport IS NULL OR octet_length(transport) BETWEEN 1 AND 64),
    CHECK (
        jsonb_typeof(waypoints) = 'array'
        AND jsonb_array_length(waypoints) <= 200
    ),
    CHECK (
        goods IS NULL
        OR (jsonb_typeof(goods) = 'array' AND jsonb_array_length(goods) <= 16)
    ),
    CHECK (
        reason <> 'already_empty'
        OR goods IS NULL
        OR goods = '[]'::jsonb
    )
);

CREATE INDEX trade_reports_server_finished_idx
    ON trade_reports (lower(server_name), finished_at DESC, trade_id DESC);
