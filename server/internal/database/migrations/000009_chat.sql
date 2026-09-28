CREATE TABLE chat_messages (
    message_id UUID PRIMARY KEY DEFAULT gen_random_uuid(),
    event_id UUID UNIQUE REFERENCES activity_events(event_id) ON DELETE CASCADE,
    command_id TEXT UNIQUE REFERENCES commands(command_id) ON DELETE CASCADE,
    echo_event_id UUID REFERENCES activity_events(event_id) ON DELETE SET NULL,
    echo_of_command_id TEXT REFERENCES commands(command_id) ON DELETE SET NULL,
    character_id UUID NOT NULL REFERENCES characters(character_id),
    session_id UUID REFERENCES character_sessions(session_id),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    character_name TEXT NOT NULL CHECK (char_length(character_name) BETWEEN 1 AND 64),
    channel TEXT NOT NULL CHECK (channel IN ('general','private','party','guild','union','global','unknown')),
    direction TEXT NOT NULL CHECK (direction IN ('inbound','outbound')),
    raw_type TEXT NOT NULL DEFAULT '' CHECK (char_length(raw_type) <= 64),
    sender TEXT NOT NULL DEFAULT '' CHECK (char_length(sender) <= 64),
    peer_name TEXT NOT NULL DEFAULT '' CHECK (char_length(peer_name) <= 64),
    peer_key TEXT NOT NULL DEFAULT '' CHECK (char_length(peer_key) <= 128),
    message TEXT NOT NULL CHECK (octet_length(message) BETWEEN 1 AND 2048),
    occurred_at TIMESTAMPTZ NOT NULL,
    CHECK ((event_id IS NULL) <> (command_id IS NULL)),
    CHECK ((direction = 'inbound' AND event_id IS NOT NULL) OR
           (direction = 'outbound' AND command_id IS NOT NULL)),
    CHECK (echo_event_id IS NULL OR event_id IS NOT NULL),
    CHECK (echo_of_command_id IS NULL OR event_id IS NOT NULL)
);

INSERT INTO chat_messages(message_id,event_id,character_id,session_id,server_name,character_name,channel,direction,raw_type,sender,peer_name,peer_key,message,occurred_at)
SELECT e.event_id,e.event_id,e.character_id,e.session_id,e.server_name,c.character_name,
       CASE WHEN e.payload->>'channel' IN ('general','private','party','guild','union','global') THEN e.payload->>'channel' ELSE 'unknown' END,
       'inbound',COALESCE(e.payload->>'raw_type',''),COALESCE(e.payload->>'sender',''),
       CASE WHEN e.payload->>'channel'='private' THEN COALESCE(e.payload->>'sender','') ELSE '' END,
       CASE WHEN e.payload->>'channel'='private' THEN lower(COALESCE(e.payload->>'sender','')) ELSE '' END,
       e.payload->>'message',e.occurred_at
FROM activity_events e JOIN characters c ON c.character_id=e.character_id
WHERE e.kind='chat.message_received' AND e.character_id IS NOT NULL AND e.session_id IS NOT NULL
  AND e.server_name IS NOT NULL AND COALESCE(e.payload->>'direction','inbound')='inbound'
  AND octet_length(COALESCE(e.payload->>'message','')) BETWEEN 1 AND 2048
ON CONFLICT(event_id) DO NOTHING;

CREATE INDEX chat_messages_scope_time_idx
    ON chat_messages (lower(server_name), character_id, channel, occurred_at DESC, message_id DESC);
CREATE INDEX chat_messages_conversation_time_idx
    ON chat_messages (lower(server_name), channel, peer_key, occurred_at DESC, message_id DESC)
    WHERE channel = 'private';
CREATE INDEX chat_messages_unread_idx
    ON chat_messages (lower(server_name), character_id, channel, occurred_at, message_id)
    WHERE direction = 'inbound' AND echo_of_command_id IS NULL;

CREATE TABLE chat_read_cursors (
    operator_identity TEXT NOT NULL CHECK (char_length(operator_identity) BETWEEN 1 AND 64),
    server_name TEXT NOT NULL CHECK (char_length(server_name) BETWEEN 1 AND 100),
    server_key TEXT GENERATED ALWAYS AS (lower(server_name)) STORED,
    character_id UUID REFERENCES characters(character_id) ON DELETE CASCADE,
    character_scope TEXT GENERATED ALWAYS AS (COALESCE(character_id::text, '*')) STORED,
    channel TEXT NOT NULL CHECK (channel IN ('general','private','party','guild','union','global','unknown')),
    peer_key TEXT NOT NULL DEFAULT '' CHECK (char_length(peer_key) <= 128),
    last_read_at TIMESTAMPTZ NOT NULL,
    last_read_message_id UUID NOT NULL REFERENCES chat_messages(message_id) ON DELETE CASCADE,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now(),
    PRIMARY KEY (operator_identity, server_key, character_scope, channel, peer_key)
);

CREATE TABLE chat_preferences (
    operator_identity TEXT PRIMARY KEY CHECK (char_length(operator_identity) BETWEEN 1 AND 64),
    browser_notifications BOOLEAN NOT NULL DEFAULT false,
    message_sound BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);
