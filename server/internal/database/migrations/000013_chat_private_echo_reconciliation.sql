-- Reconcile private inbound echoes after migration 000012 classified raw type 2.
CREATE TEMP TABLE chat_private_echo_reconciliation ON COMMIT DROP AS
WITH candidates AS (
    SELECT inbound.message_id AS inbound_id,
           inbound.event_id,
           outbound.command_id
    FROM chat_messages AS inbound
    JOIN chat_messages AS outbound
      ON outbound.character_id = inbound.character_id
     AND lower(outbound.server_name) = lower(inbound.server_name)
     AND outbound.session_id = inbound.session_id
     AND outbound.channel = inbound.channel
     AND outbound.message = inbound.message
     AND (inbound.peer_key = '' OR outbound.peer_key = inbound.peer_key)
     AND abs(extract(epoch FROM (inbound.occurred_at - outbound.occurred_at))) <= 10
    JOIN commands AS command ON command.command_id = outbound.command_id
    WHERE inbound.direction = 'inbound'
      AND inbound.raw_type = '2'
      AND inbound.channel = 'private'
      AND inbound.echo_of_command_id IS NULL
      AND outbound.direction = 'outbound'
      AND outbound.echo_event_id IS NULL
      AND command.state IN ('sent', 'acknowledged', 'completed', 'unknown')
), ranked AS (
    SELECT *,
           count(*) OVER (PARTITION BY inbound_id) AS inbound_matches,
           count(*) OVER (PARTITION BY command_id) AS command_matches
    FROM candidates
)
SELECT inbound_id, event_id, command_id
FROM ranked
WHERE inbound_matches = 1
  AND command_matches = 1;

UPDATE chat_messages AS inbound
SET echo_of_command_id = match.command_id
FROM chat_private_echo_reconciliation AS match
WHERE inbound.message_id = match.inbound_id
  AND inbound.echo_of_command_id IS NULL;

UPDATE chat_messages AS outbound
SET echo_event_id = match.event_id
FROM chat_private_echo_reconciliation AS match
WHERE outbound.command_id = match.command_id
  AND outbound.echo_event_id IS NULL;
