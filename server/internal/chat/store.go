package chat

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

const (
	MaxPageSize  = 100
	MaxTextBytes = 2048
	EchoWindow   = 10 * time.Second
)

var ErrInvalid = errors.New("invalid chat request")

var Channels = map[string]bool{
	"general": true, "private": true, "party": true, "guild": true,
	"union": true, "global": true, "unknown": true,
}

func serverScopedChannel(channel string) bool {
	return channel != "private" && channel != "guild" && channel != "union"
}

type Store struct{ pool *pgxpool.Pool }

type EventInput struct {
	EventID, CharacterID, SessionID string
	Server, Character               string
	OccurredAt                      time.Time
	Payload                         json.RawMessage
}

type Message struct {
	MessageID   string    `json:"message_id"`
	EventID     string    `json:"event_id,omitempty"`
	CommandID   string    `json:"command_id,omitempty"`
	EchoEventID string    `json:"echo_event_id,omitempty"`
	CharacterID string    `json:"character_id"`
	SessionID   string    `json:"session_id,omitempty"`
	Server      string    `json:"server"`
	Character   string    `json:"character"`
	Channel     string    `json:"channel"`
	Direction   string    `json:"direction"`
	RawType     string    `json:"raw_type,omitempty"`
	Sender      string    `json:"sender,omitempty"`
	PeerName    string    `json:"peer_name,omitempty"`
	PeerKey     string    `json:"peer_key,omitempty"`
	Text        string    `json:"message"`
	State       string    `json:"state"`
	OccurredAt  time.Time `json:"occurred_at"`
}

type Contact struct {
	Server        string    `json:"server"`
	CharacterID   string    `json:"character_id"`
	Character     string    `json:"character"`
	PeerName      string    `json:"peer_name"`
	PeerKey       string    `json:"peer_key"`
	LastMessageID string    `json:"last_message_id"`
	LastMessage   string    `json:"last_message"`
	LastMessageAt time.Time `json:"last_message_at"`
	LastDirection string    `json:"last_direction"`
	Unread        int       `json:"unread"`
}

type Preferences struct {
	BrowserNotifications bool `json:"browser_notifications"`
	MessageSound         bool `json:"message_sound"`
}

type Filter struct {
	Server, CharacterID, Channel, Peer string
	Before, After                      string
	Limit                              int
}

type MessagePage struct {
	Messages    []Message `json:"messages"`
	OlderCursor string    `json:"older_cursor,omitempty"`
	NewerCursor string    `json:"newer_cursor,omitempty"`
	HasOlder    bool      `json:"has_older"`
}

type Snapshot struct {
	Channel         string         `json:"channel"`
	Contacts        []Contact      `json:"contacts"`
	Page            MessagePage    `json:"page"`
	UnreadByChannel map[string]int `json:"unread_by_channel"`
}

type ReadState struct {
	Contacts        []Contact      `json:"contacts"`
	UnreadByChannel map[string]int `json:"unread_by_channel"`
}

type Cursor struct {
	At time.Time
	ID string
}

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func NormalizePeer(value string) string { return strings.ToLower(strings.TrimSpace(value)) }

func ValidChannel(value string) bool { return Channels[value] }

// ProjectInbound runs in the same transaction as canonical event persistence.
func ProjectInbound(ctx context.Context, tx pgx.Tx, event EventInput) error {
	var payload struct {
		Channel   string `json:"channel"`
		RawType   string `json:"raw_type"`
		Sender    string `json:"sender"`
		Recipient string `json:"recipient"`
		Direction string `json:"direction"`
		Message   string `json:"message"`
	}
	if json.Unmarshal(event.Payload, &payload) != nil || payload.Direction != "inbound" || !ValidChannel(payload.Channel) ||
		len(payload.Message) == 0 || len([]byte(payload.Message)) > MaxTextBytes || len(payload.RawType) > 64 ||
		len(payload.Sender) > 64 || len(payload.Recipient) > 64 || event.CharacterID == "" || event.Server == "" || event.Character == "" {
		return ErrInvalid
	}
	peer := ""
	if payload.Channel == "private" {
		peer = strings.TrimSpace(payload.Sender)
	}
	peerKey := NormalizePeer(peer)
	var echoCommand any
	if payload.Channel != "unknown" && payload.Message != "" {
		rows, err := tx.Query(ctx, `
SELECT m.command_id
FROM chat_messages m JOIN commands c ON c.command_id=m.command_id
WHERE m.direction='outbound' AND m.echo_event_id IS NULL
  AND m.character_id=$1::uuid AND lower(m.server_name)=lower($2)
  AND m.session_id=$8::uuid AND m.channel=$3 AND m.message=$4
  AND c.state IN ('sent','acknowledged','completed','unknown')
  AND abs(extract(epoch FROM ($5::timestamptz-m.occurred_at))) <= $6
			AND ($3 <> 'private' OR $7='' OR m.peer_key=$7)
ORDER BY m.occurred_at DESC, m.message_id DESC
LIMIT 2 FOR UPDATE OF m SKIP LOCKED`, event.CharacterID, event.Server, payload.Channel, payload.Message, event.OccurredAt.UTC(), EchoWindow.Seconds(), peerKey, event.SessionID)
		if err != nil {
			return err
		}
		var candidates []string
		for rows.Next() {
			var id string
			if err := rows.Scan(&id); err != nil {
				rows.Close()
				return err
			}
			candidates = append(candidates, id)
		}
		if err := rows.Err(); err != nil {
			rows.Close()
			return err
		}
		rows.Close()
		if len(candidates) == 1 {
			echoCommand = candidates[0]
		}
	}
	if echoCommand != nil {
		if _, err := tx.Exec(ctx, `UPDATE chat_messages SET echo_event_id=$2::uuid WHERE command_id=$1 AND echo_event_id IS NULL`, echoCommand, event.EventID); err != nil {
			return err
		}
	}
	_, err := tx.Exec(ctx, `
INSERT INTO chat_messages(message_id,event_id,character_id,session_id,server_name,character_name,channel,direction,raw_type,sender,peer_name,peer_key,message,occurred_at,echo_of_command_id)
VALUES($1::uuid,$1::uuid,$2::uuid,$3::uuid,$4,$5,$6,'inbound',$7,$8,$9,$10,$11,$12,$13)
ON CONFLICT(event_id) DO NOTHING`, event.EventID, event.CharacterID, event.SessionID, event.Server, event.Character,
		payload.Channel, payload.RawType, payload.Sender, peer, peerKey, payload.Message, event.OccurredAt.UTC(), echoCommand)
	return err
}

// ProjectCommandTx creates a traceable outgoing row during command admission.
func ProjectCommandTx(ctx context.Context, tx pgx.Tx, commandID, characterID, sessionID string, args json.RawMessage, at time.Time) error {
	var payload struct {
		Channel   string `json:"channel"`
		Text      string `json:"text"`
		Recipient string `json:"recipient"`
	}
	if json.Unmarshal(args, &payload) != nil || !ValidChannel(payload.Channel) || payload.Channel == "unknown" ||
		len(payload.Text) == 0 || len([]byte(payload.Text)) > MaxTextBytes || len(payload.Recipient) > 64 ||
		(payload.Channel == "private") != (strings.TrimSpace(payload.Recipient) != "") {
		return ErrInvalid
	}
	tag, err := tx.Exec(ctx, `
INSERT INTO chat_messages(message_id,command_id,character_id,session_id,server_name,character_name,channel,direction,sender,peer_name,peer_key,message,occurred_at)
SELECT gen_random_uuid(),$1,c.character_id,$3::uuid,c.server_name,c.character_name,$4,'outbound',c.character_name,$5,lower($5),$6,$7
FROM characters c WHERE c.character_id=$2::uuid`, commandID, characterID, sessionID, payload.Channel,
		strings.TrimSpace(payload.Recipient), payload.Text, at.UTC())
	if err != nil {
		return err
	}
	if tag.RowsAffected() != 1 {
		return ErrInvalid
	}
	return nil
}

func (s *Store) Messages(ctx context.Context, filter Filter) (MessagePage, error) {
	if !ValidChannel(filter.Channel) || filter.Limit < 1 || filter.Limit > MaxPageSize ||
		len(filter.Server) > 100 || len(filter.Peer) > 64 || filter.Before != "" && filter.After != "" {
		return MessagePage{}, ErrInvalid
	}
	if filter.CharacterID != "" && !validUUID(filter.CharacterID) {
		return MessagePage{}, ErrInvalid
	}
	if filter.Channel == "private" {
		if filter.Peer == "" {
			return MessagePage{Messages: []Message{}}, nil
		}
		filter.Peer = NormalizePeer(filter.Peer)
		if filter.Peer == "" {
			return MessagePage{}, ErrInvalid
		}
	} else if filter.Peer != "" {
		return MessagePage{}, ErrInvalid
	}
	var cursor *Cursor
	direction := "latest"
	var err error
	if filter.Before != "" {
		cursor, err = DecodeCursor(filter.Before)
		direction = "before"
	}
	if filter.After != "" {
		cursor, err = DecodeCursor(filter.After)
		direction = "after"
	}
	if err != nil {
		return MessagePage{}, ErrInvalid
	}
	query := `
SELECT m.message_id::text,COALESCE(m.event_id::text,''),COALESCE(m.command_id,''),COALESCE(m.echo_event_id::text,''),
 m.character_id::text,COALESCE(m.session_id::text,''),m.server_name,m.character_name,m.channel,m.direction,m.raw_type,m.sender,m.peer_name,m.peer_key,m.message,
 COALESCE(c.state,'received'),m.occurred_at
FROM chat_messages m LEFT JOIN commands c ON c.command_id=m.command_id
WHERE ($1='' OR lower(m.server_name)=lower($1)) AND
  ($2='' OR m.character_id=$2::uuid OR m.channel IN ('general','global'))
  AND m.channel=$3 AND ($3<>'private' OR m.peer_key=$4)
  AND m.echo_of_command_id IS NULL
  AND ($5::timestamptz IS NULL OR (m.occurred_at,m.message_id) < ($5::timestamptz,$6::uuid))
  AND ($7::timestamptz IS NULL OR (m.occurred_at,m.message_id) > ($7::timestamptz,$8::uuid))`
	var beforeAt, afterAt any
	var beforeID, afterID any
	if direction == "before" {
		beforeAt, beforeID = cursor.At, cursor.ID
	}
	if direction == "after" {
		afterAt, afterID = cursor.At, cursor.ID
	}
	order := "ASC"
	if direction != "after" {
		order = "DESC"
	}
	query += ` ORDER BY m.occurred_at ` + order + `,m.message_id ` + order + ` LIMIT $9`
	rows, err := s.pool.Query(ctx, query, filter.Server, filter.CharacterID, filter.Channel, filter.Peer, beforeAt, beforeID, afterAt, afterID, filter.Limit+1)
	if err != nil {
		return MessagePage{}, err
	}
	defer rows.Close()
	items := make([]Message, 0, filter.Limit+1)
	for rows.Next() {
		var item Message
		if err := rows.Scan(&item.MessageID, &item.EventID, &item.CommandID, &item.EchoEventID, &item.CharacterID, &item.SessionID,
			&item.Server, &item.Character, &item.Channel, &item.Direction, &item.RawType, &item.Sender, &item.PeerName, &item.PeerKey, &item.Text, &item.State, &item.OccurredAt); err != nil {
			return MessagePage{}, err
		}
		items = append(items, item)
	}
	if err := rows.Err(); err != nil {
		return MessagePage{}, err
	}
	hasMore := len(items) > filter.Limit
	if hasMore {
		items = items[:filter.Limit]
	}
	if direction != "after" {
		reverse(items)
	}
	page := MessagePage{Messages: items}
	if len(items) > 0 {
		first, last := items[0], items[len(items)-1]
		page.NewerCursor = EncodeCursor(last.OccurredAt, last.MessageID)
		if hasMore || direction == "latest" || direction == "before" {
			page.OlderCursor = EncodeCursor(first.OccurredAt, first.MessageID)
		}
		page.HasOlder = hasMore && direction != "after" || direction == "latest" && hasMore || direction == "before" && hasMore
	}
	return page, nil
}

func reverse(items []Message) {
	for i, j := 0, len(items)-1; i < j; i, j = i+1, j-1 {
		items[i], items[j] = items[j], items[i]
	}
}

func (s *Store) Contacts(ctx context.Context, server, characterID string, limit int) ([]Contact, error) {
	if len(server) > 100 || limit < 1 || limit > MaxPageSize || characterID != "" && !validUUID(characterID) {
		return nil, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `
	SELECT (array_agg(m.server_name ORDER BY m.occurred_at DESC,m.message_id DESC))[1],
	 (array_agg(m.character_id::text ORDER BY m.occurred_at DESC,m.message_id DESC))[1],
	 (array_agg(m.character_name ORDER BY m.occurred_at DESC,m.message_id DESC))[1],
	 (array_agg(m.peer_name ORDER BY m.occurred_at DESC,m.message_id DESC))[1],m.peer_key,
 (array_agg(m.message_id::text ORDER BY m.occurred_at DESC,m.message_id DESC))[1],
 (array_agg(m.message ORDER BY m.occurred_at DESC,m.message_id DESC))[1],
 max(m.occurred_at),
 (array_agg(m.direction ORDER BY m.occurred_at DESC,m.message_id DESC))[1],
 count(*) FILTER (WHERE m.direction='inbound' AND m.echo_of_command_id IS NULL AND
   (r.last_read_at IS NULL OR (m.occurred_at,m.message_id)>(r.last_read_at,r.last_read_message_id)))::int
FROM chat_messages m
LEFT JOIN chat_read_cursors r ON r.operator_identity='operator' AND lower(r.server_name)=lower(m.server_name)
 AND r.character_id IS NOT DISTINCT FROM NULLIF($2,'')::uuid AND r.channel='private' AND r.peer_key=m.peer_key
WHERE m.channel='private' AND m.peer_key<>'' AND ($1='' OR lower(m.server_name)=lower($1))
 AND ($2='' OR m.character_id=$2::uuid) AND m.echo_of_command_id IS NULL
GROUP BY lower(m.server_name),m.character_id,m.peer_key,r.last_read_at,r.last_read_message_id
ORDER BY max(m.occurred_at) DESC,m.peer_key LIMIT $3`, server, characterID, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	contacts := make([]Contact, 0)
	for rows.Next() {
		var item Contact
		if err := rows.Scan(&item.Server, &item.CharacterID, &item.Character, &item.PeerName, &item.PeerKey, &item.LastMessageID, &item.LastMessage, &item.LastMessageAt, &item.LastDirection, &item.Unread); err != nil {
			return nil, err
		}
		contacts = append(contacts, item)
	}
	return contacts, rows.Err()
}

func (s *Store) UnreadByChannel(ctx context.Context, server, characterID string) (map[string]int, error) {
	if len(server) > 100 || characterID != "" && !validUUID(characterID) {
		return nil, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `
SELECT m.channel,m.server_name,m.character_id::text,m.raw_type,m.sender,m.message,m.occurred_at
FROM chat_messages m
LEFT JOIN chat_read_cursors r ON r.operator_identity='operator' AND lower(r.server_name)=lower(m.server_name)
	AND r.character_id IS NOT DISTINCT FROM CASE
		WHEN m.channel IN ('private','guild','union') THEN COALESCE(NULLIF($2,'')::uuid,m.character_id)
		ELSE NULL::uuid
	END
	AND r.channel=m.channel
	AND r.peer_key=CASE WHEN m.channel='private' THEN m.peer_key ELSE '' END
WHERE m.direction='inbound' AND m.echo_of_command_id IS NULL AND ($1='' OR lower(m.server_name)=lower($1))
	AND m.channel<>'global'
	AND ($2='' OR m.character_id=$2::uuid OR m.channel NOT IN ('private','guild','union'))
	AND (r.last_read_at IS NULL OR (m.occurred_at,m.message_id)>(r.last_read_at,r.last_read_message_id))
ORDER BY m.channel,m.occurred_at DESC,m.message_id DESC`, server, characterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	observations := make([]unreadObservation, 0)
	for rows.Next() {
		var item unreadObservation
		if err := rows.Scan(&item.channel, &item.server, &item.characterID, &item.rawType, &item.sender, &item.message, &item.occurredAt); err != nil {
			return nil, err
		}
		observations = append(observations, item)
	}
	if err := rows.Err(); err != nil {
		return nil, err
	}
	return countUnreadObservations(observations), nil
}

const chatObservationWindow = 2 * time.Second

type unreadObservation struct {
	channel, server, characterID, rawType, sender, message string
	occurredAt                                             time.Time
}

type chatObservationKey struct {
	channel, server, rawType, sender, message string
}

type unreadObservationGroup struct {
	occurredAt time.Time
	observers  map[string]struct{}
}

func countUnreadObservations(observations []unreadObservation) map[string]int {
	counts := map[string]int{}
	groupsByKey := map[chatObservationKey][]*unreadObservationGroup{}
	for _, item := range observations {
		if !serverScopedChannel(item.channel) || item.characterID == "" || item.sender == "" {
			counts[item.channel]++
			continue
		}
		key := chatObservationKey{
			channel: item.channel,
			server:  strings.ToLower(item.server),
			rawType: item.rawType,
			sender:  item.sender,
			message: item.message,
		}
		groups := groupsByKey[key]
		duplicate := false
		for _, group := range groups {
			if group.occurredAt.Sub(item.occurredAt) > chatObservationWindow {
				continue
			}
			if _, seen := group.observers[item.characterID]; seen {
				continue
			}
			group.observers[item.characterID] = struct{}{}
			duplicate = true
			break
		}
		if duplicate {
			continue
		}
		groupsByKey[key] = append(groups, &unreadObservationGroup{
			occurredAt: item.occurredAt,
			observers:  map[string]struct{}{item.characterID: {}},
		})
		counts[item.channel]++
	}
	return counts
}

func (s *Store) MarkRead(ctx context.Context, server, characterID, channel, peer, messageID string) error {
	if len(server) == 0 || len(server) > 100 || !ValidChannel(channel) || characterID != "" && !validUUID(characterID) ||
		messageID != "" && !validUUID(messageID) || channel == "private" && messageID == "" ||
		(channel == "private" || channel == "guild" || channel == "union") && characterID == "" ||
		(channel == "private") != (strings.TrimSpace(peer) != "") || len(peer) > 64 {
		return ErrInvalid
	}
	peerKey := NormalizePeer(peer)
	characterScope := characterID
	if serverScopedChannel(channel) {
		characterScope = ""
	}
	var at time.Time
	cursorMessageID := messageID
	if channel == "private" {
		err := s.pool.QueryRow(ctx, `SELECT occurred_at FROM chat_messages WHERE message_id=$1::uuid AND lower(server_name)=lower($2)
		 AND character_id=$3::uuid AND channel=$4 AND peer_key=$5 AND direction='inbound' AND echo_of_command_id IS NULL`, messageID, server, characterScope, channel, peerKey).Scan(&at)
		if err != nil {
			return err
		}
	} else if serverScopedChannel(channel) {
		if messageID != "" {
			err := s.pool.QueryRow(ctx, `SELECT occurred_at FROM chat_messages WHERE message_id=$1::uuid AND lower(server_name)=lower($2)
			 AND channel=$3 AND direction='inbound' AND echo_of_command_id IS NULL`, messageID, server, channel).Scan(&at)
			if err != nil {
				return err
			}
		}
		err := s.pool.QueryRow(ctx, `SELECT occurred_at,message_id::text FROM chat_messages WHERE lower(server_name)=lower($1)
		 AND channel=$2 AND direction='inbound' AND echo_of_command_id IS NULL
		 ORDER BY occurred_at DESC,message_id DESC LIMIT 1`, server, channel).Scan(&at, &cursorMessageID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
	} else {
		if messageID != "" {
			err := s.pool.QueryRow(ctx, `SELECT occurred_at FROM chat_messages WHERE message_id=$1::uuid AND lower(server_name)=lower($2)
			 AND character_id=$3::uuid AND channel=$4 AND direction='inbound' AND echo_of_command_id IS NULL`, messageID, server, characterScope, channel).Scan(&at)
			if err != nil {
				return err
			}
		}
		err := s.pool.QueryRow(ctx, `SELECT occurred_at,message_id::text FROM chat_messages WHERE lower(server_name)=lower($1)
		 AND character_id=$2::uuid AND channel=$3 AND direction='inbound' AND echo_of_command_id IS NULL
		 ORDER BY occurred_at DESC,message_id DESC LIMIT 1`, server, characterScope, channel).Scan(&at, &cursorMessageID)
		if errors.Is(err, pgx.ErrNoRows) {
			return nil
		}
		if err != nil {
			return err
		}
	}
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_read_cursors(operator_identity,server_name,character_id,channel,peer_key,last_read_at,last_read_message_id)
VALUES('operator',$1,NULLIF($2,'')::uuid,$3,$4,$5,$6::uuid)
ON CONFLICT(operator_identity,server_key,character_scope,channel,peer_key)
DO UPDATE SET last_read_at=CASE WHEN (EXCLUDED.last_read_at,EXCLUDED.last_read_message_id)>(chat_read_cursors.last_read_at,chat_read_cursors.last_read_message_id) THEN EXCLUDED.last_read_at ELSE chat_read_cursors.last_read_at END,
 last_read_message_id=CASE WHEN (EXCLUDED.last_read_at,EXCLUDED.last_read_message_id)>(chat_read_cursors.last_read_at,chat_read_cursors.last_read_message_id) THEN EXCLUDED.last_read_message_id ELSE chat_read_cursors.last_read_message_id END,
			updated_at=now()`, server, characterScope, channel, peerKey, at, cursorMessageID)
	return err
}

func (s *Store) Preferences(ctx context.Context) (Preferences, error) {
	var p Preferences
	err := s.pool.QueryRow(ctx, `SELECT browser_notifications,message_sound FROM chat_preferences WHERE operator_identity='operator'`).Scan(&p.BrowserNotifications, &p.MessageSound)
	if errors.Is(err, pgx.ErrNoRows) {
		return Preferences{}, nil
	}
	return p, err
}

func (s *Store) SavePreferences(ctx context.Context, p Preferences) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO chat_preferences(operator_identity,browser_notifications,message_sound)
VALUES('operator',$1,$2) ON CONFLICT(operator_identity) DO UPDATE SET browser_notifications=EXCLUDED.browser_notifications,message_sound=EXCLUDED.message_sound,updated_at=now()`, p.BrowserNotifications, p.MessageSound)
	return err
}

func (s *Store) Snapshot(ctx context.Context, filter Filter) (Snapshot, error) {
	if filter.Limit == 0 {
		filter.Limit = 50
	}
	page, err := s.Messages(ctx, filter)
	if err != nil {
		return Snapshot{}, err
	}
	readState, err := s.ReadState(ctx, filter.Server, filter.CharacterID)
	if err != nil {
		return Snapshot{}, err
	}
	return Snapshot{Channel: filter.Channel, Contacts: readState.Contacts, Page: page, UnreadByChannel: readState.UnreadByChannel}, nil
}

func (s *Store) ReadState(ctx context.Context, server, characterID string) (ReadState, error) {
	contacts, err := s.Contacts(ctx, server, characterID, 50)
	if err != nil {
		return ReadState{}, err
	}
	counts, err := s.UnreadByChannel(ctx, server, characterID)
	if err != nil {
		return ReadState{}, err
	}
	return ReadState{Contacts: contacts, UnreadByChannel: counts}, nil
}

func EncodeCursor(at time.Time, id string) string {
	return base64.RawURLEncoding.EncodeToString([]byte(at.UTC().Format(time.RFC3339Nano) + "|" + id))
}
func DecodeCursor(raw string) (*Cursor, error) {
	data, err := base64.RawURLEncoding.DecodeString(raw)
	if err != nil || len(data) > 128 {
		return nil, ErrInvalid
	}
	parts := strings.SplitN(string(data), "|", 2)
	if len(parts) != 2 || !validUUID(parts[1]) {
		return nil, ErrInvalid
	}
	at, err := time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return nil, ErrInvalid
	}
	return &Cursor{At: at, ID: parts[1]}, nil
}

func validUUID(value string) bool {
	if len(value) != 36 {
		return false
	}
	for i, c := range value {
		if i == 8 || i == 13 || i == 18 || i == 23 {
			if c != '-' {
				return false
			}
			continue
		}
		if !(c >= '0' && c <= '9' || c >= 'a' && c <= 'f' || c >= 'A' && c <= 'F') {
			return false
		}
	}
	return true
}

func ValidateOutbound(channel, text, recipient string) error {
	if !ValidChannel(channel) || channel == "unknown" || len([]byte(text)) == 0 || len([]byte(text)) > MaxTextBytes || strings.TrimSpace(text) == "" ||
		(channel == "private") != (strings.TrimSpace(recipient) != "") || len(recipient) > 64 || strings.ContainsRune(text, 0) || strings.ContainsRune(recipient, 0) {
		return ErrInvalid
	}
	return nil
}

func ValidateFilter(filter Filter) error {
	if !ValidChannel(filter.Channel) || len(filter.Server) > 100 || len(filter.Peer) > 64 || filter.Limit < 1 || filter.Limit > MaxPageSize ||
		filter.CharacterID != "" && !validUUID(filter.CharacterID) || filter.Before != "" && filter.After != "" {
		return ErrInvalid
	}
	return nil
}

func (m Message) String() string { return fmt.Sprintf("%s/%s %s", m.Server, m.Channel, m.MessageID) }
