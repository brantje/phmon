ALTER TABLE player_enrichments
    ADD COLUMN extended JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN proxy_level INTEGER NULL CHECK (proxy_level IS NULL OR proxy_level BETWEEN 1 AND 255),
    ADD COLUMN proxy_job TEXT NULL CHECK (proxy_job IS NULL OR char_length(proxy_job) BETWEEN 1 AND 64),
    ADD COLUMN proxy_job_level INTEGER NULL CHECK (proxy_job_level IS NULL OR proxy_job_level BETWEEN 0 AND 255),
    ADD COLUMN proxy_guild TEXT NULL CHECK (proxy_guild IS NULL OR octet_length(proxy_guild) BETWEEN 0 AND 1024),
    ADD COLUMN proxy_race VARCHAR(64) NULL;

ALTER TABLE player_enrichment_pending
    ADD COLUMN extended JSONB NOT NULL DEFAULT '{}'::jsonb,
    ADD COLUMN proxy_level INTEGER NULL CHECK (proxy_level IS NULL OR proxy_level BETWEEN 1 AND 255),
    ADD COLUMN proxy_job TEXT NULL CHECK (proxy_job IS NULL OR char_length(proxy_job) BETWEEN 1 AND 64),
    ADD COLUMN proxy_job_level INTEGER NULL CHECK (proxy_job_level IS NULL OR proxy_job_level BETWEEN 0 AND 255),
    ADD COLUMN proxy_guild TEXT NULL CHECK (proxy_guild IS NULL OR octet_length(proxy_guild) BETWEEN 0 AND 1024),
    ADD COLUMN proxy_race VARCHAR(64) NULL;
