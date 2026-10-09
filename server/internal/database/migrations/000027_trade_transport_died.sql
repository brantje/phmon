-- A failed trade can end because the transport died. That reason is distinct
-- from thief, navigation, and generic error.
DO $$
DECLARE
    existing text;
BEGIN
    FOR existing IN
        SELECT conname
        FROM pg_constraint
        WHERE conrelid = 'trade_reports'::regclass
          AND contype = 'c'
          AND pg_get_constraintdef(oid) LIKE '%already_empty%'
          AND pg_get_constraintdef(oid) LIKE '%navigation%'
    LOOP
        EXECUTE format('ALTER TABLE trade_reports DROP CONSTRAINT %I', existing);
    END LOOP;
END $$;

ALTER TABLE trade_reports ADD CONSTRAINT trade_reports_outcome_reason_check CHECK (
    (outcome = 'success' AND reason IN ('sold', 'already_empty'))
    OR (outcome = 'failed' AND reason IN ('thief', 'navigation', 'error', 'transport_died'))
    OR (outcome = 'cancelled' AND reason = 'cancelled')
);
