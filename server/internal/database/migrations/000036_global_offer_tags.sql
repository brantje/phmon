-- Heuristic item tags parsed from Global advertisement text. These are derived
-- from chat_messages and can be rebuilt by bumping the parser version.
CREATE TABLE global_offer_parse (
	message_id     uuid PRIMARY KEY REFERENCES chat_messages(message_id) ON DELETE CASCADE,
	parser_version smallint NOT NULL
);

CREATE TABLE global_offer_items (
	message_id uuid NOT NULL REFERENCES chat_messages(message_id) ON DELETE CASCADE,
	ordinal    smallint NOT NULL,
	item_type  text NOT NULL,
	subtype    text NOT NULL DEFAULT '',
	part       text NOT NULL DEFAULT '',
	degree     smallint NOT NULL DEFAULT 0,
	race       text NOT NULL DEFAULT '',
	seal       text NOT NULL DEFAULT '',
	plus       smallint NOT NULL DEFAULT 0,
	price      text NOT NULL DEFAULT '',
	currency   text NOT NULL DEFAULT '',
	notes      text[] NOT NULL DEFAULT '{}',
	PRIMARY KEY (message_id, ordinal)
);
CREATE INDEX global_offer_items_filter_idx ON global_offer_items(item_type, subtype, degree);
