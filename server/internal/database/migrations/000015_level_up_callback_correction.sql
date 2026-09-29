-- The installed phBot runtime sends the level being left in EVENT_LEVEL_UP.
-- Preserve the raw callback value and correct existing canonical occurrences.
UPDATE activity_events AS e
SET payload = e.payload || jsonb_build_object(
    'callback_level', (payload->>'level')::integer,
    'level', (payload->>'level')::integer + 1
)
FROM agents AS a
WHERE a.agent_id = e.agent_id
  AND a.phbot_version = '20.1.2'
  AND e.kind = 'character.level_up'
  AND e.source = 'phbot.callback'
  AND e.source_ref = 'EVENT_LEVEL_UP'
  AND NOT e.payload ? 'callback_level'
  AND e.payload->>'level' ~ '^[0-9]{1,3}$'
  AND (e.payload->>'level')::integer BETWEEN 1 AND 254;
