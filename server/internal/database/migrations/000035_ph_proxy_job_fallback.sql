-- Recover job fallbacks from already accepted v2 evidence. Tracking rows,
-- observation history, presence and original evidence/timestamps are untouched.
UPDATE player_enrichments
SET proxy_job = CASE
    WHEN field_evidence->'job'->>'source_identity_kind' = 'character_name'
    THEN CASE extended->>'job'
        WHEN '0' THEN 'none' WHEN '1' THEN 'trader'
        WHEN '2' THEN 'thief' WHEN '3' THEN 'hunter'
        ELSE extended->>'job' END
    ELSE NULL END;

UPDATE player_enrichment_pending
SET proxy_job = CASE
    WHEN field_evidence->'job'->>'source_identity_kind' = 'character_name'
    THEN CASE extended->>'job'
        WHEN '0' THEN 'none' WHEN '1' THEN 'trader'
        WHEN '2' THEN 'thief' WHEN '3' THEN 'hunter'
        ELSE extended->>'job' END
    ELSE NULL END;
