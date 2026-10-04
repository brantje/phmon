package events

import (
	"encoding/json"
	"strings"
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
	notice := validTestEvent("world.unique_spawned", "world", "joymax.unique_notice", "0x300C", `{"model":1954,"notice":"spawn"}`)
	if err := validateAgentEvent(notice); err != nil {
		t.Fatalf("unique notice rejected: %v", err)
	}
	notice.Payload = json.RawMessage(`{"model":1954,"notice":"other"}`)
	if err := validateAgentEvent(notice); err == nil {
		t.Fatal("unique notice accepted an unknown notice type")
	}
	for _, test := range tests {
		t.Run(test.sourceRef, func(t *testing.T) {
			event := validTestEvent(test.kind, test.category, "phbot.callback", test.sourceRef, test.payload)
			event.ItemModel = test.model
			event.Zone = "Jangan"
			if err := validateAgentEvent(event); err != nil {
				t.Fatalf("validateAgentEvent() error = %v", err)
			}
		})
	}
}

func TestValidateAgentEventBoundsAndTrimsZoneName(t *testing.T) {
	event := validTestEvent("character.died", "character", "phbot.callback", "EVENT_DIED", `{"cause":"unknown"}`)
	event.Zone = "Jangan"
	if err := validateAgentEvent(event); err != nil {
		t.Fatalf("valid zone name rejected: %v", err)
	}
	for _, zone := range []string{" Jangan", "Jangan ", strings.Repeat("Z", 101)} {
		event.Zone = zone
		if err := validateAgentEvent(event); err == nil {
			t.Errorf("invalid zone name %q was accepted", zone)
		}
	}
}

func TestValidateAgentEventAcceptsInferredDeathReason(t *testing.T) {
	for _, payload := range []string{
		`{"cause":"Rival","reason_type":"attacker","reason_value":"Rival"}`,
		`{"cause":"monster_environment","reason_type":"monster_or_environment","reason_value":"Monster / environment"}`,
	} {
		event := validTestEvent("character.died", "character", "phbot.callback", "EVENT_DIED", payload)
		if err := validateAgentEvent(event); err != nil {
			t.Fatalf("inferred death reason %s rejected: %v", payload, err)
		}
	}
}

func TestNormalizeLevelUpEventPreservesRawCallbackAndIsIdempotent(t *testing.T) {
	event := validTestEvent("character.level_up", "character", "phbot.callback", "EVENT_LEVEL_UP", `{"level":71}`)
	if err := normalizeLevelUpEvent(&event, "20.1.2"); err != nil {
		t.Fatal(err)
	}
	if string(event.Payload) != `{"callback_level":71,"level":72}` {
		t.Fatalf("normalized payload = %s", event.Payload)
	}
	if err := validateAgentEvent(event); err != nil {
		t.Fatal(err)
	}
	if err := normalizeLevelUpEvent(&event, "20.1.2"); err != nil || string(event.Payload) != `{"callback_level":71,"level":72}` {
		t.Fatalf("replayed payload = %s, %v", event.Payload, err)
	}
	otherVersion := validTestEvent("character.level_up", "character", "phbot.callback", "EVENT_LEVEL_UP", `{"level":71}`)
	if err := normalizeLevelUpEvent(&otherVersion, "20.1.3"); err != nil || string(otherVersion.Payload) != `{"level":71}` {
		t.Fatalf("unverified version payload = %s, %v", otherVersion.Payload, err)
	}

	for _, payload := range []string{
		`{"level":255}`,
		`{"level":72,"callback_level":71.5}`,
		`{"level":71,"callback_level":71}`,
	} {
		invalid := validTestEvent("character.level_up", "character", "phbot.callback", "EVENT_LEVEL_UP", payload)
		if err := normalizeLevelUpEvent(&invalid, "20.1.2"); err == nil {
			t.Errorf("accepted inconsistent level-up payload %s", payload)
		}
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
	acquired.Payload = json.RawMessage(`{"item":{"model":7,"servername":"ITEM_TEST"},"quantity_delta":1,"destination_container":{"type":"inventory","slot":13},"acquisition_method":"unknown","drop_event_id":"8d16ab39-10b8-4b26-80aa-a4be01e10f18"}`)
	if err := validateAgentEvent(acquired); err != nil {
		t.Fatalf("bounded inventory observation link rejected: %v", err)
	}
	acquired.Kind = "item.quantity_increased"
	if err := validateAgentEvent(acquired); err != nil {
		t.Fatalf("existing-model inventory observation link rejected: %v", err)
	}
	acquired.Kind = "item.acquired"
	acquired.Payload = json.RawMessage(`{"item":{"model":7},"quantity_delta":2,"destination_container":{"type":"inventory","slot":13},"acquisition_method":"unknown","drop_event_id":"8d16ab39-10b8-4b26-80aa-a4be01e10f18"}`)
	if err := validateAgentEvent(acquired); err == nil {
		t.Fatal("ambiguous multi-item drop association was accepted")
	}
	acquired.Payload = json.RawMessage(`{"item":{"model":7},"quantity_delta":2,"destination_container":{"type":"inventory"},"acquisition_method":"ground_pickup"}`)
	if err := validateAgentEvent(acquired); err == nil {
		t.Fatal("unverified acquisition cause was accepted")
	}
}

func TestValidatedPetPickupRequiresBoundedPacketReceipt(t *testing.T) {
	model := int64(847)
	event := validTestEvent("item.acquired", "item", "joymax.pet_inventory", "0xB034", `{"item":{"model":847,"servername":"ITEM_TEST","plus":0,"quantity":4},"quantity_delta":2,"destination_container":{"type":"pets","id":"17","slot":3},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"session-a:seq-12","sequence":"12","decoder_version":"pet-v1"}}`)
	event.ItemModel = &model
	event.ItemCode = "ITEM_TEST"
	if err := validateAgentEvent(event); err != nil {
		t.Fatalf("verified pet pickup rejected: %v", err)
	}

	for name, replacement := range map[string]string{
		"missing evidence":   `{"item":{"model":847},"quantity_delta":2,"destination_container":{"type":"pets","id":"17","slot":3},"acquisition_method":"pet_pickup"}`,
		"wrong destination":  `{"item":{"model":847},"quantity_delta":2,"destination_container":{"type":"inventory","slot":3},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"seq-12","sequence":"12","decoder_version":"pet-v1"}}`,
		"zero sequence":      `{"item":{"model":847},"quantity_delta":2,"destination_container":{"type":"pets","id":"17","slot":3},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"seq-12","sequence":"0","decoder_version":"pet-v1"}}`,
		"wrong method":       `{"item":{"model":847},"quantity_delta":2,"destination_container":{"type":"pets","id":"17","slot":3},"acquisition_method":"unknown","packet_observation":{"observation_id":"seq-12","sequence":"12","decoder_version":"pet-v1"}}`,
		"unbounded quantity": `{"item":{"model":847},"quantity_delta":1000000001,"destination_container":{"type":"pets","id":"17","slot":3},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"seq-12","sequence":"12","decoder_version":"pet-v1"}}`,
		"control in pet ID":  `{"item":{"model":847},"quantity_delta":2,"destination_container":{"type":"pets","id":"17\n","slot":3},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"seq-12","sequence":"12","decoder_version":"pet-v1"}}`,
	} {
		t.Run(name, func(t *testing.T) {
			invalid := event
			invalid.Payload = json.RawMessage(replacement)
			if err := validateAgentEvent(invalid); err == nil {
				t.Fatal("unverified pet pickup accepted")
			}
		})
	}

	legacy := validTestEvent("item.acquired", "item", "phbot.state_diff", "item_container", string(event.Payload))
	if err := validateAgentEvent(legacy); err == nil {
		t.Fatal("state-diff event promoted itself to a verified pet pickup")
	}
}

func TestDropFeedPetPickupFilterIsLimitedToNormalAndRareTabs(t *testing.T) {
	for _, filter := range []Filter{
		{Kind: "drop.item", IncludePetPickups: true},
		{Kind: "drop.rare", IncludePetPickups: true},
		{Kind: "drop.item", IncludeOwnedGains: true},
		{Kind: "drop.rare", IncludeOwnedGains: true},
	} {
		if !validDropFeed(filter) {
			t.Fatalf("valid drop feed rejected: %+v", filter)
		}
	}
	for _, filter := range []Filter{
		{IncludePetPickups: true},
		{Kind: "drop.rare", Category: "drop", IncludePetPickups: true},
		{Kind: "item.acquired", IncludePetPickups: true},
		{IncludeOwnedGains: true},
		{Kind: "item.acquired", IncludeOwnedGains: true},
	} {
		if validDropFeed(filter) {
			t.Fatalf("invalid pet pickup feed accepted: %+v", filter)
		}
	}
}

func TestDropClassificationUsesCallbackClassOrVerifiedPetClassifier(t *testing.T) {
	store := NewStore(nil)
	store.SetDropClassifier(func(server string, model *int64, code string) (string, string) {
		if server == "Silkroad" && model != nil && *model == 847 {
			return "rare", "item-profile-rarity-v1"
		}
		return "", ""
	})
	model := int64(847)
	if class, version := eventDropClassification(store.classifyDrop, validTestEvent("drop.item", "drop", "phbot.callback", "EVENT_ITEM_DROP", `{"model":847}`)); class != "normal" || version != "phbot-callback-v1" {
		t.Fatalf("normal callback classification = %q %q", class, version)
	}
	pickup := validTestEvent("item.acquired", "item", "joymax.pet_inventory", "0xB034", `{}`)
	pickup.ItemModel = &model
	if class, version := eventDropClassification(store.classifyDrop, pickup); class != "rare" || version != "item-profile-rarity-v1" {
		t.Fatalf("pet pickup classification = %q %q", class, version)
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
