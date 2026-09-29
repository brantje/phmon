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
	"phmon/server/internal/chat"
)

var ErrUnauthorizedSession = errors.New("event session is not owned by this agent and character")
var ErrEventConflict = errors.New("event id was already used for a different occurrence")
var ErrInvalidEvent = errors.New("invalid death event")

const (
	DeathKind     = "character.died"
	DeathCategory = "character"
	MaxPayload    = 32768
	MaxPageSize   = 100
	MaxBatchSize  = 16
	MaxBatchBytes = 256 * 1024
)

var eventKinds = map[string]string{
	"session.connected": "session", "session.disconnected": "session",
	"session.joined_game": "session", "session.teleported": "session",
	"character.died": "character", "character.level_up": "character",
	"character.attacked": "character", "drop.item": "drop", "drop.rare": "drop",
	"world.unique_spawned": "world", "world.gm_spawned": "world",
	"job.hunter_trader_seen": "job", "job.thief_seen": "job",
	"pet.transport_died": "pet", "alchemy.finished": "alchemy",
	"alchemy.attempt": "alchemy", "chat.message_received": "chat",
	"party.member_joined": "party", "party.member_left": "party",
	"academy.member_joined": "academy", "academy.member_left": "academy",
	"academy.member_graduated": "academy", "pet.summoned": "pet", "pet.dismissed": "pet",
	"item.acquired": "item", "item.transferred": "item",
	"item.quantity_increased": "item", "item.quantity_decreased": "item",
}

var validEventSources = map[string]map[string]bool{
	"phbot.callback": {"EVENT_DIED": true, "EVENT_LEVEL_UP": true, "EVENT_PLAYER_ATTACKING": true,
		"EVENT_ITEM_DROP": true, "EVENT_RARE_DROP": true, "EVENT_UNIQUE_SPAWN": true,
		"EVENT_GM_SPAWNED": true, "EVENT_HUNTER_SPAWN": true, "EVENT_THIEF_SPAWN": true,
		"EVENT_TRANSPORT_DIED": true, "EVENT_ALCHEMY_FINISHED": true},
	"phbot.lifecycle_callback": {"connected": true, "disconnected": true, "joined_game": true, "teleported": true},
	"phbot.chat_callback":      {"handle_chat": true},
	"phbot.alchemy_callback":   {"alchemy_update": true},
	"phbot.state_diff":         {"party": true, "academy": true, "pet": true, "item_container": true},
}

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
	ID           string          `json:"event_id"`
	Schema       int             `json:"schema_version"`
	Kind         string          `json:"kind"`
	Category     string          `json:"category"`
	AgentID      string          `json:"agent_id"`
	CharacterID  string          `json:"character_id"`
	SessionID    string          `json:"session_id"`
	Server       string          `json:"server"`
	Character    string          `json:"character"`
	OccurredAt   time.Time       `json:"occurred_at"`
	ReceivedAt   time.Time       `json:"received_at"`
	Source       string          `json:"source"`
	SourceRef    string          `json:"source_ref"`
	Region       *int            `json:"region,omitempty"`
	X            *float64        `json:"x,omitempty"`
	Y            *float64        `json:"y,omitempty"`
	Z            *float64        `json:"z,omitempty"`
	ModelID      *int64          `json:"model_id,omitempty"`
	PortraitURL  string          `json:"portrait_url,omitempty"`
	Payload      json.RawMessage `json:"payload"`
	Sequence     *int64          `json:"sequence,omitempty"`
	DedupeKey    string          `json:"dedupe_key,omitempty"`
	ItemModel    *int64          `json:"item_model,omitempty"`
	ItemCode     string          `json:"item_code,omitempty"`
	ItemMetadata map[string]any  `json:"item_metadata,omitempty"`
}

// AgentEvent is the bounded occurrence envelope submitted over an authenticated
// agent connection. Agent identity is always taken from the connection itself.
type AgentEvent struct {
	ID          string          `json:"event_id"`
	Schema      int             `json:"schema_version"`
	Kind        string          `json:"kind"`
	Category    string          `json:"category"`
	CharacterID string          `json:"character_id,omitempty"`
	SessionID   string          `json:"session_id,omitempty"`
	Server      string          `json:"server,omitempty"`
	Character   string          `json:"character,omitempty"`
	OccurredAt  time.Time       `json:"occurred_at"`
	Sequence    *int64          `json:"sequence,omitempty"`
	Source      string          `json:"source"`
	SourceRef   string          `json:"source_ref"`
	DedupeKey   string          `json:"dedupe_key,omitempty"`
	Region      *int            `json:"region,omitempty"`
	X           *float64        `json:"x,omitempty"`
	Y           *float64        `json:"y,omitempty"`
	Z           *float64        `json:"z,omitempty"`
	ItemModel   *int64          `json:"item_model,omitempty"`
	ItemCode    string          `json:"item_code,omitempty"`
	Payload     json.RawMessage `json:"payload"`
}

type AppendResult struct {
	EventID string `json:"event_id"`
	Status  string `json:"status"`
	Reason  string `json:"reason,omitempty"`
}

type Filter struct {
	Server         string
	CharacterID    string
	CharacterQuery string
	Kind           string
	Category       string
	ItemQuery      string
	From           *time.Time
	To             *time.Time
	Cursor         string
	Limit          int
}

type Page struct {
	Events     []Event         `json:"events"`
	Total      int64           `json:"total"`
	NextCursor string          `json:"next_cursor,omitempty"`
	Alchemy    *AlchemySummary `json:"alchemy_summary,omitempty"`
}

type AlchemySummary struct {
	Attempts    int64 `json:"attempts"`
	Successes   int64 `json:"successes"`
	Failures    int64 `json:"failures"`
	HighestPlus *int  `json:"highest_plus,omitempty"`
}

type Cursor struct {
	OccurredAt time.Time
	EventID    string
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func ValidKind(value string) bool {
	if value == "" {
		return true
	}
	if strings.HasPrefix(value, "custom.") && len(value) <= 80 {
		return true
	}
	_, ok := eventKinds[value]
	return ok
}
func ValidCategory(value string) bool {
	if value == "" {
		return true
	}
	if value == "custom" {
		return true
	}
	for _, category := range eventKinds {
		if category == value {
			return true
		}
	}
	return false
}

func (s *Store) AppendBatch(ctx context.Context, agentID string, incoming []AgentEvent) ([]AppendResult, bool, error) {
	// The agent WebSocket bounds raw frames before decoding; re-marshalling here can
	// escape JSON differently and reject a valid batch based on a different byte count.
	if !agentdomain.ValidAgentID(agentID) || len(incoming) == 0 || len(incoming) > MaxBatchSize {
		return nil, false, ErrInvalidEvent
	}
	results := make([]AppendResult, len(incoming))
	for i := range incoming {
		results[i].EventID = incoming[i].ID
		if err := validateAgentEvent(incoming[i]); err != nil {
			results[i].Status = "rejected"
			results[i].Reason = "invalid_event"
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		for i := range results {
			if results[i].Status == "" {
				results[i].Status = "retry"
				results[i].Reason = "temporarily_unavailable"
			}
		}
		return results, false, fmt.Errorf("begin event batch: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()
	changed := false
	for i, event := range incoming {
		if results[i].Status == "rejected" {
			continue
		}
		inserted, duplicate, err := appendOne(ctx, tx, agentID, event)
		if errors.Is(err, ErrUnauthorizedSession) || errors.Is(err, ErrEventConflict) || errors.Is(err, ErrInvalidEvent) {
			results[i].Status = "rejected"
			results[i].Reason = "session_or_event_rejected"
			continue
		}
		if err != nil {
			for j := range results {
				if results[j].Status == "" || results[j].Status == "persisted" {
					results[j].Status = "retry"
					results[j].Reason = "temporarily_unavailable"
				}
			}
			return results, false, fmt.Errorf("append event batch: %w", err)
		}
		if inserted {
			changed = true
			results[i].Status = "persisted"
		} else if duplicate {
			results[i].Status = "persisted"
		} else {
			results[i].Status = "rejected"
			results[i].Reason = "session_or_event_rejected"
		}
	}
	if err := tx.Commit(ctx); err != nil {
		for i := range results {
			if results[i].Status == "persisted" {
				results[i].Status = "retry"
				results[i].Reason = "temporarily_unavailable"
			}
		}
		return results, false, fmt.Errorf("commit event batch: %w", err)
	}
	return results, changed, nil
}

// phBot 20.1.2 reports the level being left in EVENT_LEVEL_UP.
// Preserve that callback value while storing the reached level consistently.
// A plugin that already normalized the value can send both fields and is left alone.
func normalizeLevelUpEvent(event *AgentEvent, phbotVersion string) error {
	if event.Kind != "character.level_up" || event.Source != "phbot.callback" || event.SourceRef != "EVENT_LEVEL_UP" {
		return nil
	}
	var payload map[string]json.RawMessage
	if err := json.Unmarshal(event.Payload, &payload); err != nil || payload == nil {
		return ErrInvalidEvent
	}
	var level int
	if err := json.Unmarshal(payload["level"], &level); err != nil || level < 1 || level > 255 {
		return ErrInvalidEvent
	}
	if raw, present := payload["callback_level"]; present {
		var callbackLevel int
		if err := json.Unmarshal(raw, &callbackLevel); err != nil || callbackLevel < 1 || callbackLevel >= 255 || level != callbackLevel+1 {
			return ErrInvalidEvent
		}
		return nil
	}
	if phbotVersion != "20.1.2" {
		return nil
	}
	if level >= 255 {
		return ErrInvalidEvent
	}
	payload["callback_level"], _ = json.Marshal(level)
	payload["level"], _ = json.Marshal(level + 1)
	var err error
	event.Payload, err = json.Marshal(payload)
	return err
}

func validateAgentEvent(event AgentEvent) error {
	category, known := eventKinds[event.Kind]
	if !known || event.Category != category || event.Schema != 1 ||
		!agentdomain.ValidAgentID(event.ID) || event.OccurredAt.IsZero() ||
		event.OccurredAt.After(time.Now().UTC().Add(5*time.Minute)) ||
		event.OccurredAt.Before(time.Now().UTC().Add(-365*24*time.Hour)) ||
		!validPosition(event.Region, event.X, event.Y, event.Z) ||
		!validEventSources[event.Source][event.SourceRef] || len(event.SourceRef) > 120 ||
		len(event.DedupeKey) > 160 || len(event.Server) > 100 || len(event.Character) > 64 ||
		len(event.ItemCode) > 128 {
		return fmt.Errorf("%w: invalid event envelope", ErrInvalidEvent)
	}
	if !validKindSource(event) {
		return fmt.Errorf("%w: source does not support event kind", ErrInvalidEvent)
	}
	if (event.CharacterID == "") != (event.SessionID == "") ||
		(event.CharacterID != "" && (!agentdomain.ValidAgentID(event.CharacterID) || !agentdomain.ValidAgentID(event.SessionID))) {
		return fmt.Errorf("%w: invalid event session", ErrInvalidEvent)
	}
	if (event.Server == "") != (event.Character == "") ||
		(event.CharacterID == "" && !strings.HasPrefix(event.Kind, "session.")) {
		return fmt.Errorf("%w: invalid event identity context", ErrInvalidEvent)
	}
	if event.Sequence != nil && *event.Sequence <= 0 {
		return fmt.Errorf("%w: invalid event sequence", ErrInvalidEvent)
	}
	if event.SessionID != "" && event.Sequence == nil {
		return fmt.Errorf("%w: session event requires a sequence", ErrInvalidEvent)
	}
	if event.ItemModel != nil && *event.ItemModel < 0 {
		return fmt.Errorf("%w: invalid item model", ErrInvalidEvent)
	}
	if (event.Kind == "item.acquired" || event.Kind == "item.transferred" || event.Kind == "item.quantity_increased" || event.Kind == "item.quantity_decreased" || event.Kind == "drop.item" || event.Kind == "drop.rare") && event.ItemModel == nil && event.ItemCode == "" {
		return fmt.Errorf("%w: item event lacks canonical item identity", ErrInvalidEvent)
	}
	payload := event.Payload
	if len(payload) == 0 {
		payload = json.RawMessage(`{}`)
	}
	if len(payload) > MaxPayload || !json.Valid(payload) {
		return fmt.Errorf("%w: invalid payload", ErrInvalidEvent)
	}
	var object map[string]json.RawMessage
	if json.Unmarshal(payload, &object) != nil || object == nil {
		return fmt.Errorf("%w: payload must be an object", ErrInvalidEvent)
	}
	if event.Kind == "chat.message_received" {
		var fields struct {
			Channel   string `json:"channel"`
			RawType   string `json:"raw_type"`
			Message   string `json:"message"`
			Sender    string `json:"sender"`
			Recipient string `json:"recipient"`
		}
		if json.Unmarshal(payload, &fields) != nil || !chat.ValidChannel(fields.Channel) || len(fields.RawType) > 64 || len(fields.Message) == 0 || len(fields.Message) > chat.MaxTextBytes || len(fields.Sender) > 64 || len(fields.Recipient) > 64 {
			return fmt.Errorf("%w: chat message missing or too large", ErrInvalidEvent)
		}
		var direction struct {
			Value string `json:"direction"`
		}
		if json.Unmarshal(payload, &direction) != nil || direction.Value != "inbound" {
			return fmt.Errorf("%w: invalid chat direction", ErrInvalidEvent)
		}
	}
	if event.Kind == "character.level_up" {
		var level int
		if json.Unmarshal(object["level"], &level) != nil || level < 1 || level > 255 {
			return fmt.Errorf("%w: invalid level-up payload", ErrInvalidEvent)
		}
	}
	if event.Kind == "alchemy.attempt" {
		var fields struct {
			Slot    int   `json:"slot"`
			Success *bool `json:"success"`
			Plus    *int  `json:"plus"`
		}
		if _, exists := object["slot"]; !exists || json.Unmarshal(payload, &fields) != nil || fields.Slot < 0 || fields.Slot > 4096 || fields.Plus != nil && (*fields.Plus < 0 || *fields.Plus > 255) {
			return fmt.Errorf("%w: invalid alchemy payload", ErrInvalidEvent)
		}
	}
	if strings.HasPrefix(event.Kind, "session.") && len(object) != 0 {
		return fmt.Errorf("%w: lifecycle payload must be empty", ErrInvalidEvent)
	}
	if event.Source == "phbot.callback" {
		switch event.Kind {
		case "character.died":
			var fields struct {
				Cause string `json:"cause"`
			}
			if json.Unmarshal(payload, &fields) != nil || len(fields.Cause) > 128 {
				return fmt.Errorf("%w: invalid death callback payload", ErrInvalidEvent)
			}
		case "alchemy.finished":
			if len(object) != 0 {
				return fmt.Errorf("%w: alchemy completion payload must be empty", ErrInvalidEvent)
			}
		case "drop.rare", "drop.item":
			var model int64
			if json.Unmarshal(object["model"], &model) != nil || model < 0 || event.ItemModel == nil || *event.ItemModel != model {
				return fmt.Errorf("%w: drop callback requires its observed model ID", ErrInvalidEvent)
			}
		case "character.attacked", "world.unique_spawned", "world.gm_spawned", "job.hunter_trader_seen", "job.thief_seen", "pet.transport_died":
			var value string
			if json.Unmarshal(object["value"], &value) != nil || len(value) > 512 {
				return fmt.Errorf("%w: callback value is missing or too large", ErrInvalidEvent)
			}
		}
	}
	if strings.HasPrefix(event.Kind, "party.") || strings.HasPrefix(event.Kind, "academy.") {
		var fields struct {
			MemberID string         `json:"member_id"`
			Member   map[string]any `json:"member"`
		}
		if json.Unmarshal(payload, &fields) != nil || fields.MemberID == "" || len(fields.MemberID) > 100 || fields.Member == nil {
			return fmt.Errorf("%w: invalid membership transition", ErrInvalidEvent)
		}
	}
	if strings.HasPrefix(event.Kind, "pet.") && event.Kind != "pet.transport_died" {
		var fields struct {
			PetID string         `json:"pet_id"`
			Pet   map[string]any `json:"pet"`
		}
		if json.Unmarshal(payload, &fields) != nil || fields.PetID == "" || len(fields.PetID) > 64 || fields.Pet == nil {
			return fmt.Errorf("%w: invalid pet transition", ErrInvalidEvent)
		}
	}
	if strings.HasPrefix(event.Kind, "item.") {
		var fields struct {
			Item struct {
				Model      *int64 `json:"model"`
				ServerName string `json:"servername"`
			} `json:"item"`
			QuantityDelta  int64          `json:"quantity_delta"`
			Source         map[string]any `json:"source_container"`
			Destination    map[string]any `json:"destination_container"`
			Acquisition    string         `json:"acquisition_method"`
			TransferMethod string         `json:"method"`
		}
		if json.Unmarshal(payload, &fields) != nil || fields.QuantityDelta <= 0 || (fields.Item.Model == nil && fields.Item.ServerName == "") {
			return fmt.Errorf("%w: invalid item quantity event", ErrInvalidEvent)
		}
		if event.ItemModel != nil && fields.Item.Model != nil && *event.ItemModel != *fields.Item.Model {
			return fmt.Errorf("%w: item model identity mismatch", ErrInvalidEvent)
		}
		switch event.Kind {
		case "item.acquired":
			if !validContainerReference(fields.Destination) || fields.Acquisition != "unknown" {
				return fmt.Errorf("%w: acquisition lacks destination or uses unverified provenance", ErrInvalidEvent)
			}
		case "item.transferred":
			if !validContainerReference(fields.Source) || !validContainerReference(fields.Destination) || fields.TransferMethod != "observed_container_delta" {
				return fmt.Errorf("%w: transfer lacks matching container evidence", ErrInvalidEvent)
			}
		case "item.quantity_increased":
			if !validContainerReference(fields.Destination) || fields.Acquisition != "unknown" {
				return fmt.Errorf("%w: quantity increase requires destination and unknown cause", ErrInvalidEvent)
			}
		case "item.quantity_decreased":
			if !validContainerReference(fields.Source) {
				return fmt.Errorf("%w: quantity decrease lacks source", ErrInvalidEvent)
			}
		}
	}
	if err := validatePayloadBounds(object, 0, new(int)); err != nil {
		return fmt.Errorf("%w: payload exceeds bounds", ErrInvalidEvent)
	}
	return nil
}

func validContainerReference(value map[string]any) bool {
	if value == nil {
		return false
	}
	typeName, ok := value["type"].(string)
	if !ok || (typeName != "equipment" && typeName != "inventory" && typeName != "storage" && typeName != "job_pouch" && typeName != "pets") {
		return false
	}
	if id, exists := value["id"]; exists {
		text, ok := id.(string)
		if !ok || text == "" || len(text) > 64 {
			return false
		}
	}
	if slot, exists := value["slot"]; exists {
		number, ok := slot.(float64)
		if !ok || number < 0 || math.Trunc(number) != number || number > 65535 {
			return false
		}
	}
	return true
}

func validKindSource(event AgentEvent) bool {
	switch event.Source {
	case "phbot.callback":
		want := map[string]string{
			"EVENT_DIED": "character.died", "EVENT_LEVEL_UP": "character.level_up",
			"EVENT_PLAYER_ATTACKING": "character.attacked", "EVENT_ITEM_DROP": "drop.item",
			"EVENT_RARE_DROP": "drop.rare", "EVENT_UNIQUE_SPAWN": "world.unique_spawned",
			"EVENT_GM_SPAWNED": "world.gm_spawned", "EVENT_HUNTER_SPAWN": "job.hunter_trader_seen",
			"EVENT_THIEF_SPAWN": "job.thief_seen", "EVENT_TRANSPORT_DIED": "pet.transport_died",
			"EVENT_ALCHEMY_FINISHED": "alchemy.finished",
		}
		return want[event.SourceRef] == event.Kind
	case "phbot.lifecycle_callback":
		return map[string]string{"connected": "session.connected", "disconnected": "session.disconnected", "joined_game": "session.joined_game", "teleported": "session.teleported"}[event.SourceRef] == event.Kind
	case "phbot.chat_callback":
		return event.SourceRef == "handle_chat" && event.Kind == "chat.message_received"
	case "phbot.alchemy_callback":
		return event.SourceRef == "alchemy_update" && event.Kind == "alchemy.attempt"
	case "phbot.state_diff":
		want := map[string][]string{
			"party":          {"party.member_joined", "party.member_left"},
			"academy":        {"academy.member_joined", "academy.member_left", "academy.member_graduated"},
			"pet":            {"pet.summoned", "pet.dismissed"},
			"item_container": {"item.acquired", "item.transferred", "item.quantity_increased", "item.quantity_decreased"},
		}
		for _, kind := range want[event.SourceRef] {
			if kind == event.Kind {
				return true
			}
		}
		return false
	default:
		return false
	}
}

func validatePayloadBounds(value any, depth int, nodes *int) error {
	*nodes++
	if depth > 8 || *nodes > 512 {
		return ErrInvalidEvent
	}
	switch item := value.(type) {
	case map[string]json.RawMessage:
		for key, raw := range item {
			if len(key) > 80 {
				return ErrInvalidEvent
			}
			var child any
			if err := json.Unmarshal(raw, &child); err != nil {
				return err
			}
			if err := validatePayloadBounds(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case map[string]any:
		for key, child := range item {
			if len(key) > 80 {
				return ErrInvalidEvent
			}
			if err := validatePayloadBounds(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case []any:
		for _, child := range item {
			if err := validatePayloadBounds(child, depth+1, nodes); err != nil {
				return err
			}
		}
	case string:
		if len(item) > 8192 {
			return ErrInvalidEvent
		}
	}
	return nil
}

func appendOne(ctx context.Context, tx pgx.Tx, agentID string, event AgentEvent) (inserted, duplicate bool, err error) {
	if event.Payload == nil {
		event.Payload = json.RawMessage(`{}`)
	}
	if event.CharacterID != "" {
		var owned bool
		err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM character_sessions cs JOIN characters c ON c.character_id=cs.character_id WHERE cs.session_id=$1::uuid AND cs.agent_id=$2::uuid AND cs.character_id=$3::uuid AND ($6='' OR lower(c.server_name)=lower($6)) AND ($7='' OR c.character_name=$7) AND ($4 >= cs.started_at-interval '5 minutes' AND (cs.ended_at IS NULL OR $4 <= cs.ended_at+interval '5 minutes') OR cs.ended_at IS NOT NULL AND $4 > cs.ended_at+interval '5 minutes' AND NOT EXISTS (SELECT 1 FROM character_sessions later WHERE later.agent_id=$2::uuid AND later.character_id=$3::uuid AND later.started_at > cs.started_at AND later.started_at <= $4) OR $5='deferred' AND $4 < cs.started_at-interval '5 minutes' AND NOT EXISTS (SELECT 1 FROM character_sessions other WHERE other.agent_id=$2::uuid AND other.character_id=$3::uuid AND other.session_id<>cs.session_id AND $4 >= other.started_at-interval '5 minutes' AND (other.ended_at IS NULL OR $4 <= other.ended_at+interval '5 minutes'))))`, event.SessionID, agentID, event.CharacterID, event.OccurredAt.UTC(), deferredBinding(event.Payload), event.Server, event.Character).Scan(&owned)
		if err != nil {
			return false, false, err
		}
		if !owned {
			return false, false, ErrUnauthorizedSession
		}
	} else {
		var exists bool
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM agents WHERE agent_id=$1::uuid)`, agentID).Scan(&exists); err != nil {
			return false, false, err
		}
		if !exists || !strings.HasPrefix(event.Kind, "session.") {
			return false, false, ErrUnauthorizedSession
		}
	}
	if event.Kind == "character.level_up" && event.Source == "phbot.callback" && event.SourceRef == "EVENT_LEVEL_UP" {
		var phbotVersion string
		var previouslyNormalized bool
		// A queued retry can arrive after the agent upgrades phBot. Preserve the
		// interpretation used for the already stored occurrence in that case.
		if err := tx.QueryRow(ctx, `SELECT coalesce(a.phbot_version, ''), EXISTS (
			SELECT 1 FROM activity_events e WHERE e.event_id=$2::uuid AND e.agent_id=a.agent_id
			AND e.kind='character.level_up' AND e.payload ? 'callback_level'
		) FROM agents a WHERE a.agent_id=$1::uuid`, agentID, event.ID).Scan(&phbotVersion, &previouslyNormalized); err != nil {
			return false, false, err
		}
		if previouslyNormalized {
			phbotVersion = "20.1.2"
		}
		if err := normalizeLevelUpEvent(&event, phbotVersion); err != nil {
			return false, false, err
		}
	}
	var same bool
	err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM activity_events WHERE event_id=$1::uuid AND agent_id=$2::uuid AND kind=$3 AND category=$4 AND character_id IS NOT DISTINCT FROM NULLIF($5,'')::uuid AND session_id IS NOT DISTINCT FROM NULLIF($6,'')::uuid AND server_name IS NOT DISTINCT FROM NULLIF($7,'') AND occurred_at=$8 AND source=$9 AND source_ref=$10 AND region IS NOT DISTINCT FROM $11 AND x IS NOT DISTINCT FROM $12 AND y IS NOT DISTINCT FROM $13 AND z IS NOT DISTINCT FROM $14 AND payload=$15::jsonb AND sequence IS NOT DISTINCT FROM $16 AND dedupe_key IS NOT DISTINCT FROM NULLIF($17,'') AND item_model IS NOT DISTINCT FROM $18 AND item_code IS NOT DISTINCT FROM NULLIF($19,''))`, event.ID, agentID, event.Kind, event.Category, event.CharacterID, event.SessionID, event.Server, event.OccurredAt.UTC(), event.Source, event.SourceRef, event.Region, event.X, event.Y, event.Z, string(event.Payload), event.Sequence, event.DedupeKey, event.ItemModel, event.ItemCode).Scan(&same)
	if err != nil {
		return false, false, err
	}
	if same {
		return false, true, nil
	}
	var conflict bool
	if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM activity_events WHERE event_id=$1::uuid)`, event.ID).Scan(&conflict); err != nil {
		return false, false, err
	}
	if conflict {
		return false, false, ErrEventConflict
	}
	if event.DedupeKey != "" {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM activity_events WHERE agent_id=$1::uuid AND source=$2 AND dedupe_key=$3)`, agentID, event.Source, event.DedupeKey).Scan(&same); err != nil {
			return false, false, err
		}
		if same {
			return false, true, nil
		}
	}
	if event.Sequence != nil && event.SessionID != "" {
		if err = tx.QueryRow(ctx, `SELECT EXISTS(SELECT 1 FROM activity_events WHERE session_id=$1::uuid AND sequence=$2)`, event.SessionID, *event.Sequence).Scan(&same); err != nil {
			return false, false, err
		}
		if same {
			return false, false, ErrEventConflict
		}
	}
	isChatMessage := event.Kind == "chat.message_received"
	if isChatMessage {
		if _, err := tx.Exec(ctx, `SAVEPOINT chat_projection`); err != nil {
			return false, false, fmt.Errorf("create chat projection savepoint: %w", err)
		}
	}
	_, err = tx.Exec(ctx, `INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,region,x,y,z,payload,sequence,dedupe_key,item_model,item_code)
VALUES($1::uuid,1,$2,$3,$4::uuid,NULLIF($5,'')::uuid,NULLIF($6,'')::uuid,NULLIF($7,''),$8,$9,$10,$11,$12,$13,$14,$15::jsonb,$16,$17,$18,$19)`, event.ID, event.Kind, event.Category, agentID, event.CharacterID, event.SessionID, event.Server, event.OccurredAt.UTC(), event.Source, event.SourceRef, event.Region, event.X, event.Y, event.Z, string(event.Payload), event.Sequence, nullIfEmpty(event.DedupeKey), itemModel(event.ItemModel), nullIfEmpty(event.ItemCode))
	if err != nil {
		return false, false, err
	}
	if isChatMessage {
		projectionErr := chat.ProjectInbound(ctx, tx, chat.EventInput{
			EventID: event.ID, CharacterID: event.CharacterID, SessionID: event.SessionID,
			Server: event.Server, Character: event.Character, OccurredAt: event.OccurredAt, Payload: event.Payload,
		})
		if projectionErr != nil {
			if _, rollbackErr := tx.Exec(ctx, `ROLLBACK TO SAVEPOINT chat_projection`); rollbackErr != nil {
				return false, false, fmt.Errorf("chat projection failed (%v); rollback failed: %w", projectionErr, rollbackErr)
			}
			if _, releaseErr := tx.Exec(ctx, `RELEASE SAVEPOINT chat_projection`); releaseErr != nil {
				return false, false, fmt.Errorf("release chat projection savepoint after rollback: %w", releaseErr)
			}
			if errors.Is(projectionErr, chat.ErrInvalid) {
				return false, false, fmt.Errorf("%w: invalid chat projection", ErrInvalidEvent)
			}
			return false, false, fmt.Errorf("chat projection failed: %w", projectionErr)
		}
		if _, err := tx.Exec(ctx, `RELEASE SAVEPOINT chat_projection`); err != nil {
			return false, false, fmt.Errorf("release chat projection savepoint: %w", err)
		}
	}
	return true, false, nil
}

func deferredBinding(payload json.RawMessage) string {
	var value struct {
		Binding string `json:"session_binding"`
	}
	_ = json.Unmarshal(payload, &value)
	return value.Binding
}

func nullIfEmpty(value string) any {
	if value == "" {
		return nil
	}
	return value
}
func itemModel(value *int64) any {
	if value == nil {
		return nil
	}
	return *value
}

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
	cursor, err := DecodeCursor(filter.Cursor)
	if err != nil {
		return Page{}, err
	}
	var cursorAt any
	var cursorID any
	if !cursor.OccurredAt.IsZero() {
		cursorAt, cursorID = cursor.OccurredAt, cursor.EventID
	}
	base := `FROM activity_events e LEFT JOIN characters c ON c.character_id=e.character_id
WHERE ($1='' OR lower(e.server_name)=lower($1)) AND ($2='' OR e.character_id=$2::uuid)
	AND ($3='' OR c.character_name ILIKE '%'||$3||'%') AND ($4='' OR e.kind=$4)
	AND ($5='' OR e.category=$5) AND ($6='' OR COALESCE(e.item_code,'') ILIKE '%'||$6||'%' OR COALESCE(e.item_model::text,'') ILIKE '%'||$6||'%' OR e.payload->>'model' ILIKE '%'||$6||'%' OR e.payload->>'item_name' ILIKE '%'||$6||'%' OR e.payload->'item'->>'name' ILIKE '%'||$6||'%' OR e.payload->'item'->>'servername' ILIKE '%'||$6||'%')
	AND ($7::timestamptz IS NULL OR e.occurred_at >= $7)
	AND ($8::timestamptz IS NULL OR e.occurred_at < $8)`
	var total int64
	if err := s.pool.QueryRow(ctx, `SELECT count(*) `+base, filter.Server, filter.CharacterID, filter.CharacterQuery, filter.Kind, filter.Category, filter.ItemQuery, filter.From, filter.To).Scan(&total); err != nil {
		return Page{}, fmt.Errorf("count events: %w", err)
	}
	rows, err := s.pool.Query(ctx, `SELECT e.event_id::text,e.schema_version,e.kind,e.category,e.agent_id::text,COALESCE(e.character_id::text,''),COALESCE(e.session_id::text,''),COALESCE(e.server_name,''),COALESCE(c.character_name,''),e.occurred_at,e.received_at,e.source,e.source_ref,e.region,e.x,e.y,e.z,e.payload,e.sequence,COALESCE(e.dedupe_key,''),e.item_model,COALESCE(e.item_code,''),c.model_id
`+base+` AND ($9::timestamptz IS NULL OR (e.occurred_at,e.event_id)<($9::timestamptz,$10::uuid))
ORDER BY e.occurred_at DESC,e.event_id DESC LIMIT $11`, filter.Server, filter.CharacterID, filter.CharacterQuery,
		filter.Kind, filter.Category, filter.ItemQuery, filter.From, filter.To, cursorAt, cursorID, filter.Limit+1)
	if err != nil {
		return Page{}, fmt.Errorf("list events: %w", err)
	}
	defer rows.Close()
	page := Page{Events: make([]Event, 0, filter.Limit), Total: total}
	if filter.Kind == "alchemy.attempt" {
		var summary AlchemySummary
		var highest *int
		if err := s.pool.QueryRow(ctx, `SELECT count(*),count(*) FILTER (WHERE e.payload->>'success'='true'),count(*) FILTER (WHERE e.payload->>'success'='false'),max((e.payload->>'plus')::integer) `+base,
			filter.Server, filter.CharacterID, filter.CharacterQuery, filter.Kind, filter.Category, filter.ItemQuery, filter.From, filter.To).Scan(&summary.Attempts, &summary.Successes, &summary.Failures, &highest); err != nil {
			return Page{}, fmt.Errorf("summarize alchemy events: %w", err)
		}
		summary.HighestPlus = highest
		page.Alchemy = &summary
	}
	for rows.Next() {
		var item Event
		if err := rows.Scan(&item.ID, &item.Schema, &item.Kind, &item.Category, &item.AgentID, &item.CharacterID, &item.SessionID, &item.Server, &item.Character, &item.OccurredAt, &item.ReceivedAt, &item.Source, &item.SourceRef, &item.Region, &item.X, &item.Y, &item.Z, &item.Payload, &item.Sequence, &item.DedupeKey, &item.ItemModel, &item.ItemCode, &item.ModelID); err != nil {
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
