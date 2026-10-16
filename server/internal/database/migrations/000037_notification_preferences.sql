CREATE TABLE notification_preferences (
    operator_identity TEXT PRIMARY KEY CHECK (operator_identity = 'operator'),
    popup_kinds TEXT[] NOT NULL DEFAULT '{}',
    browser_kinds TEXT[] NOT NULL DEFAULT '{}',
    chat_channels TEXT[] NOT NULL DEFAULT ARRAY['general','private','party','guild','union','global','unknown'],
    chat_sound BOOLEAN NOT NULL DEFAULT false,
    updated_at TIMESTAMPTZ NOT NULL DEFAULT now()
);

-- Preserve the sound choice while the new event and channel selections start off.
INSERT INTO notification_preferences(operator_identity,chat_sound)
SELECT operator_identity,message_sound FROM chat_preferences WHERE operator_identity='operator';
