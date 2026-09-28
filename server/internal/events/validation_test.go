package events

import (
	"encoding/json"
	"testing"
	"time"
)

const testEventID = "00000000-0000-4000-8000-000000000001"
const testSessionID = "00000000-0000-4000-8000-000000000002"
const testCharacterID = "00000000-0000-4000-8000-000000000003"

func TestValidateAgentEventAcceptsPublishedCallbackCatalog(t *testing.T) {
	tests := []struct {
		kind      string
		category  string
		sourceRef string
		payload   string
		model     *int64
	}{
		{"world.unique_spawned", "world", "EVENT_UNIQUE_SPAWN", `{"value":"Tiger Girl"}`, nil},
		{"job.hunter_trader_seen", "job", "EVENT_HUNTER_SPAWN", `{"value":"Hunter"}`, nil},
		{"job.thief_seen", "job", "EVENT_THIEF_SPAWN", `{"value":"Thief"}`, nil},
		{"pet.transport_died", "pet", "EVENT_TRANSPORT_DIED", `{"value":"123"}`, nil},
		{"character.attacked", "character", "EVENT_PLAYER_ATTACKING", `{"value":"Hunter"}`, nil},
		{"drop.rare", "drop", "EVENT_RARE_DROP", `{"model":7}`, int64Pointer(7)},
		{"drop.item", "drop", "EVENT_ITEM_DROP", `{"model":8}`, int64Pointer(8)},
		{"character.died", "character", "EVENT_DIED", `{"cause":"unknown"}`, nil},
		{"alchemy.finished", "alchemy", "EVENT_ALCHEMY_FINISHED", `{}`, nil},
		{"world.gm_spawned", "world", "EVENT_GM_SPAWNED", `{"value":"GM"}`, nil},
		{"character.level_up", "character", "EVENT_LEVEL_UP", `{"level":1}`, nil},
	}
	for _, test := range tests {
		t.Run(test.sourceRef, func(t *testing.T) {
			event := validTestEvent(test.kind, test.category, "phbot.callback", test.sourceRef, test.payload)
			event.ItemModel = test.model
			if err := validateAgentEvent(event); err != nil {
				t.Fatalf("validateAgentEvent() error = %v", err)
			}
		})
	}
}

func TestValidateAgentEventBoundsAndRequiresSessionSequence(t *testing.T) {
	event := validTestEvent("character.died", "character", "phbot.callback", "EVENT_DIED", `{}`)
	event.Sequence = nil
	if err := validateAgentEvent(event); err == nil {
		t.Fatal("session event without sequence was accepted")
	}
	event.Sequence = int64Pointer(1)
	if err := validateAgentEvent(event); err != nil {
		t.Fatalf("session event with sequence rejected: %v", err)
	}

	event = validTestEvent("chat.message_received", "chat", "phbot.chat_callback", "handle_chat", `{"channel":"unknown","raw_type":"3","message":"hello","sender":"A","recipient":"B","direction":"inbound"}`)
	if err := validateAgentEvent(event); err != nil {
		t.Fatalf("bounded chat message rejected: %v", err)
	}
	event.Payload = json.RawMessage(`{"channel":"unknown","raw_type":"3","message":"","sender":"A","recipient":"B","direction":"inbound"}`)
	if err := validateAgentEvent(event); err == nil {
		t.Fatal("empty chat message was accepted")
	}
	// Exercise the explicit character limit without relying on encoded JSON size.
	message, _ := json.Marshal(map[string]string{"message": string(make([]byte, 2049))})
	event.Payload = message
	if err := validateAgentEvent(event); err == nil {
		t.Fatal("oversized chat message was accepted")
	}
}

func TestValidateAgentEventRequiresObservedDropAndItemFacts(t *testing.T) {
	model := int64(7)
	drop := validTestEvent("drop.rare", "drop", "phbot.callback", "EVENT_RARE_DROP", `{"model":8}`)
	drop.ItemModel = &model
	if err := validateAgentEvent(drop); err == nil {
		t.Fatal("drop whose payload disagreed with its indexed model was accepted")
	}

	acquired := validTestEvent("item.acquired", "item", "phbot.state_diff", "item_container", `{"item":{"model":7,"servername":"ITEM_TEST"},"quantity_delta":2,"destination_container":{"type":"inventory","slot":3},"acquisition_method":"unknown"}`)
	acquired.ItemModel = &model
	if err := validateAgentEvent(acquired); err != nil {
		t.Fatalf("unknown-cause owned gain rejected: %v", err)
	}
	acquired.Payload = json.RawMessage(`{"item":{"model":7},"quantity_delta":2,"destination_container":{"type":"inventory"},"acquisition_method":"ground_pickup"}`)
	if err := validateAgentEvent(acquired); err == nil {
		t.Fatal("unverified acquisition cause was accepted")
	}
}

func validTestEvent(kind, category, source, sourceRef, payload string) AgentEvent {
	return AgentEvent{
		ID: testEventID, Schema: 1, Kind: kind, Category: category,
		CharacterID: testCharacterID, SessionID: testSessionID,
		Server: "Silkroad", Character: "Alpha", Sequence: int64Pointer(1),
		OccurredAt: time.Now().UTC(), Source: source, SourceRef: sourceRef,
		Payload: json.RawMessage(payload),
	}
}

func int64Pointer(value int64) *int64 { return &value }
