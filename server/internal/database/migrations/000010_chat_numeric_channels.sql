WITH channel_map(raw_type, channel) AS (VALUES
    ('1', 'general'),
    ('4', 'party'),
    ('5', 'guild'),
    ('6', 'global')
)
UPDATE activity_events AS event
SET payload = jsonb_set(event.payload, '{channel}', to_jsonb(channel_map.channel), true)
FROM channel_map
WHERE event.kind = 'chat.message_received'
  AND event.payload->>'raw_type' = channel_map.raw_type
  AND event.payload->>'channel' = 'unknown';

UPDATE chat_messages
SET channel = CASE raw_type
    WHEN '1' THEN 'general'
    WHEN '4' THEN 'party'
    WHEN '5' THEN 'guild'
    WHEN '6' THEN 'global'
    ELSE channel
END
WHERE direction = 'inbound'
  AND channel = 'unknown'
  AND raw_type IN ('1', '4', '5', '6');
