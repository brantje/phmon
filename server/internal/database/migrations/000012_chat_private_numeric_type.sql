UPDATE activity_events
SET payload = jsonb_set(payload, '{channel}', to_jsonb('private'::text), true)
WHERE kind = 'chat.message_received'
  AND payload->>'raw_type' = '2'
  AND payload->>'channel' = 'unknown';

UPDATE chat_messages
SET channel = 'private',
    peer_name = sender,
    peer_key = lower(btrim(sender))
WHERE direction = 'inbound'
  AND channel = 'unknown'
  AND raw_type = '2';
