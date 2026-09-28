package events

import (
	"context"
	"encoding/base64"
	"encoding/json"
	"errors"
	"fmt"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	agentdomain "phmon/server/internal/agents"
)

var ErrUnauthorizedSession = errors.New("event session is not owned by this agent and character")
var ErrEventConflict = errors.New("event id was already used for a different occurrence")
var ErrInvalidEvent = errors.New("invalid death event")

const (
	DeathKind     = "character.died"
	DeathCategory = "character"
	MaxPayload    = 4096
	MaxPageSize   = 100
)

type AgentDeath struct {
	ID         string          `json:"event_id"`
	OccurredAt time.Time       `json:"occurred_at"`
	Source     string          `json:"source"`
	SourceRef  string          `json:"source_ref"`
	Region     *int            `json:"region,omitempty"`
	X          *float64        `json:"x,omitempty"`
	Y          *float64        `json:"y,omitempty"`
	Z          *float64        `json:"z,omitempty"`
	Payload    json.RawMessage `json:"payload"`
}

type Event struct {
	ID          string          `json:"event_id"`
	Schema      int             `json:"schema_version"`
	Kind        string          `json:"kind"`
	Category    string          `json:"category"`
	AgentID     string          `json:"agent_id"`
	CharacterID string          `json:"character_id"`
	SessionID   string          `json:"session_id"`
	Server      string          `json:"server"`
	Character   string          `json:"character"`
	OccurredAt  time.Time       `json:"occurred_at"`
	ReceivedAt  time.Time       `json:"received_at"`
	Source      string          `json:"source"`
	SourceRef   string          `json:"source_ref"`
	Region      *int            `json:"region,omitempty"`
	X           *float64        `json:"x,omitempty"`
	Y           *float64        `json:"y,omitempty"`
	Z           *float64        `json:"z,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

type Filter struct {
	Server         string
	CharacterID    string
	CharacterQuery string
	Kind           string
	From           *time.Time
	To             *time.Time
	Cursor         string
	Limit          int
}

type Page struct {
	Events     []Event `json:"events"`
	Total      int64   `json:"total"`
	NextCursor string  `json:"next_cursor,omitempty"`
}

type Cursor struct {
	OccurredAt time.Time
	EventID    string
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// AppendDeath accepts a retried event only when the session belongs to the same
// authenticated agent and character. This lets the bounded plugin spool replay
// after reconnect while preserving the original session provenance.
func (s *Store) AppendDeath(ctx context.Context, agentID, characterID, sessionID string, incoming AgentDeath) (bool, error) {
	if !agentdomain.ValidAgentID(agentID) || !agentdomain.ValidAgentID(characterID) ||
		!agentdomain.ValidAgentID(sessionID) || !agentdomain.ValidAgentID(incoming.ID) ||
		incoming.Source != "phbot.callback" || incoming.SourceRef != "EVENT_DIED" ||
		incoming.OccurredAt.IsZero() || incoming.OccurredAt.After(time.Now().UTC().Add(5*time.Minute)) ||
		incoming.OccurredAt.Before(time.Now().UTC().Add(-365*24*time.Hour)) ||
		!validPosition(incoming.Region, incoming.X, incoming.Y, incoming.Z) {
		return false, fmt.Errorf("%w: invalid fields", ErrInvalidEvent)
	}
	payload := incoming.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{"cause":"unknown"}`)
	}
	if len(payload) > MaxPayload || !json.Valid(payload) {
		return false, fmt.Errorf("%w: invalid payload", ErrInvalidEvent)
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(payload, &object) != nil || object == nil {
		return false, fmt.Errorf("%w: payload must be an object", ErrInvalidEvent)
	}
	var payloadFields map[string]any
	if err := json.Unmarshal(payload, &payloadFields); err != nil {
		return false, fmt.Errorf("%w: invalid payload", ErrInvalidEvent)
	}
	deferredSessionBinding, _ := payloadFields["session_binding"].(string)
	var rows int64
	err := s.pool.QueryRow(ctx, `
INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,region,x,y,z,payload)
SELECT $1::uuid,1,$2,$3,$4::uuid,$5::uuid,$6::uuid,c.server_name,$7,$8,$9,$10,$11,$12,$13,$14::jsonb
FROM character_sessions cs JOIN characters c ON c.character_id=cs.character_id
WHERE cs.session_id=$6::uuid AND cs.agent_id=$4::uuid AND cs.character_id=$5::uuid
AND (
    ($7 >= cs.started_at - interval '5 minutes' AND (cs.ended_at IS NULL OR $7 <= cs.ended_at + interval '5 minutes'))
 OR (cs.ended_at IS NOT NULL AND $7 > cs.ended_at + interval '5 minutes'
     AND NOT EXISTS (SELECT 1 FROM character_sessions later
       WHERE later.agent_id=cs.agent_id AND later.character_id=cs.character_id
       AND later.started_at > cs.started_at AND later.started_at <= $7))
 OR ($15='deferred' AND $7 < cs.started_at - interval '5 minutes'
     AND NOT EXISTS (SELECT 1 FROM character_sessions other
       WHERE other.agent_id=cs.agent_id AND other.character_id=cs.character_id
       AND other.session_id <> cs.session_id
       AND $7 >= other.started_at - interval '5 minutes'
       AND (other.ended_at IS NULL OR $7 <= other.ended_at + interval '5 minutes')))
)
ON CONFLICT(event_id) DO NOTHING
RETURNING 1`, incoming.ID, DeathKind, DeathCategory, agentID, characterID, sessionID,
		incoming.OccurredAt.UTC(), incoming.Source, incoming.SourceRef, incoming.Region, incoming.X,
		incoming.Y, incoming.Z, string(payload), deferredSessionBinding).Scan(&rows)
	if err == nil {
		return true, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return false, fmt.Errorf("store death event: %w", err)
	}
	var same bool
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(
SELECT 1 FROM activity_events WHERE event_id=$1::uuid AND agent_id=$2::uuid AND character_id=$3::uuid
AND session_id=$4::uuid AND kind=$5 AND occurred_at=$6 AND source=$7 AND source_ref=$8
AND region IS NOT DISTINCT FROM $9 AND x IS NOT DISTINCT FROM $10 AND y IS NOT DISTINCT FROM $11
AND z IS NOT DISTINCT FROM $12 AND payload=$13::jsonb)`, incoming.ID, agentID, characterID,
		sessionID, DeathKind, incoming.OccurredAt.UTC(), incoming.Source, incoming.SourceRef,
		incoming.Region, incoming.X, incoming.Y, incoming.Z, string(payload)).Scan(&same)
	if err != nil {
		return false, fmt.Errorf("check death event replay: %w", err)
	}
	if same {
		return false, nil
	}
	var conflict bool
	err = s.pool.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM activity_events WHERE event_id=$1::uuid
AND NOT (agent_id=$2::uuid AND character_id=$3::uuid AND session_id=$4::uuid AND kind=$5
AND occurred_at=$6 AND source=$7 AND source_ref=$8 AND region IS NOT DISTINCT FROM $9
AND x IS NOT DISTINCT FROM $10 AND y IS NOT DISTINCT FROM $11 AND z IS NOT DISTINCT FROM $12
AND payload=$13::jsonb))`, incoming.ID, agentID, characterID, sessionID, DeathKind,
		incoming.OccurredAt.UTC(), incoming.Source, incoming.SourceRef, incoming.Region,
		incoming.X, incoming.Y, incoming.Z, string(payload)).Scan(&conflict)
	if err != nil {
		return false, fmt.Errorf("check conflicting event id: %w", err)
	}
	if conflict {
		return false, ErrEventConflict
	}
	return false, ErrUnauthorizedSession
}

func validPosition(region *int, axes ...*float64) bool {
	if region != nil && (*region < 0 || *region > 65535) {
		return false
	}
	for _, axis := range axes {
		if axis != nil && (math.IsNaN(*axis) || math.IsInf(*axis, 0) || *axis < -1000000 || *axis > 1000000) {
			return false
		}
	}
	return true
}

func EncodeCursor(cursor Cursor) string {
	value := cursor.OccurredAt.UTC().Format(time.RFC3339Nano) + "|" + cursor.EventID
	return base64.RawURLEncoding.EncodeToString([]byte(value))
}

func DecodeCursor(value string) (Cursor, error) {
	var cursor Cursor
	if len(value) == 0 {
		return cursor, nil
	}
	if len(value) > 256 {
		return cursor, errors.New("cursor too long")
	}
	decoded, err := base64.RawURLEncoding.DecodeString(value)
	if err != nil {
		return cursor, errors.New("invalid cursor")
	}
	parts := strings.SplitN(string(decoded), "|", 2)
	if len(parts) != 2 || !agentdomain.ValidAgentID(parts[1]) {
		return cursor, errors.New("invalid cursor")
	}
	cursor.OccurredAt, err = time.Parse(time.RFC3339Nano, parts[0])
	if err != nil {
		return Cursor{}, errors.New("invalid cursor")
	}
	cursor.EventID = parts[1]
	return cursor, nil
}

func (s *Store) List(ctx context.Context, filter Filter) (Page, error) {
	if filter.Limit < 1 || filter.Limit > MaxPageSize {
		filter.Limit = 10
	}
	if filter.Kind == "" {
		filter.Kind = DeathKind
	}
	cursor, err := DecodeCursor(filter.Cursor)
	if err != nil {
		return Page{}, err
	}
	var cursorAt any
	var cursorID any
	if !cursor.OccurredAt.IsZero() {
		cursorAt, cursorID = cursor.OccurredAt, cursor.EventID
	}
	base := `FROM activity_events e JOIN characters c ON c.character_id=e.character_id
WHERE ($1='' OR lower(e.server_name)=lower($1)) AND ($2='' OR e.character_id=$2::uuid)
	AND ($3='' OR c.character_name ILIKE '%'||$3||'%') AND ($4='' OR e.kind=$4)
	AND ($5::timestamptz IS NULL OR e.occurred_at >= $5)
	AND ($6::timestamptz IS NULL OR e.occurred_at < $6)`
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+base, filter.Server, filter.CharacterID, filter.CharacterQuery, filter.Kind, filter.From, filter.To).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count events: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT e.event_id::text,e.schema_version,e.kind,e.category,e.agent_id::text,e.character_id::text,e.session_id::text,e.server_name,c.character_name,e.occurred_at,e.received_at,e.source,e.source_ref,e.region,e.x,e.y,e.z,e.payload
`+base+` AND ($7::timestamptz IS NULL OR (e.occurred_at,e.event_id)<($7::timestamptz,$8::uuid))
ORDER BY e.occurred_at DESC,e.event_id DESC LIMIT $9`, filter.Server, filter.CharacterID, filter.CharacterQuery,
		filter.Kind, filter.From, filter.To, cursorAt, cursorID, filter.Limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	page := Page{Events: make([]Event, 0, filter.Limit), Total: total}
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Schema, &item.Kind, &item.Category, &item.AgentID, &item.CharacterID, &item.SessionID, &item.Server, &item.Character, &item.OccurredAt, &item.ReceivedAt, &item.Source, &item.SourceRef, &item.Region, &item.X, &item.Y, &item.Z, &item.Payload); err != nil {
			return Page{}, err
		}
		page.Events = append(page.Events, item)
	}
	if err := rows.Err(); err != nil {
		return Page{}, err
	}
	if len(page.Events) > filter.Limit {
		page.Events = page.Events[:filter.Limit]
		last := page.Events[len(page.Events)-1]
		page.NextCursor = EncodeCursor(Cursor{OccurredAt: last.OccurredAt, EventID: last.ID})
	}
	return page, nil
}
