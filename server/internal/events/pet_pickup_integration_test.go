package events

import (
	"bytes"
	"context"
	"encoding/json"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestPetPickupsJoinOnlyTheirClassifiedDropFeedAndCountAsRecords(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run event integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err := database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err := agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	serverName := "pet-feed-" + credential.AgentID
	characterID, err := characters.NewStore(pool).Resolve(ctx, characters.Identity{Server: serverName, Name: "PetAlpha"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characters.NewStore(pool).ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, serverName)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var occurredAt time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&occurredAt); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	store.SetDropClassifier(func(_ string, model *int64, _ string) (string, string) {
		if model == nil {
			return "", ""
		}
		switch *model {
		case 847:
			return "rare", "fixture-rarity-v1"
		case 848:
			return "normal", "fixture-rarity-v1"
		default:
			return "", ""
		}
	})
	base := func(kind, source, sourceRef, payload string, model int64, sequence int64) AgentEvent {
		return AgentEvent{
			ID: newTestEventID(t), Schema: 1, Kind: kind, Category: eventKinds[kind],
			CharacterID: characterID, SessionID: sessionID, Server: serverName, Character: "PetAlpha",
			OccurredAt: occurredAt.UTC(), Source: source, SourceRef: sourceRef,
			Sequence: int64Pointer(sequence), ItemModel: int64Pointer(model),
			Payload: json.RawMessage(payload),
		}
	}
	items := []AgentEvent{
		base("drop.item", "phbot.callback", "EVENT_ITEM_DROP", `{"model":849}`, 849, 1),
		base("drop.rare", "phbot.callback", "EVENT_RARE_DROP", `{"model":850}`, 850, 2),
		base("item.acquired", "joymax.pet_inventory", "0xB034", `{"item":{"model":847,"servername":"ITEM_RARE_TEST","quantity":3},"quantity_delta":1,"destination_container":{"type":"pets","id":"17","slot":2},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"fixture-seq-3","sequence":"3","decoder_version":"simulator-pet-v1"}}`, 847, 3),
		base("item.acquired", "joymax.pet_inventory", "0xB034", `{"item":{"model":848,"servername":"ITEM_NORMAL_TEST","quantity":1},"quantity_delta":1,"destination_container":{"type":"pets","id":"17","slot":4},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"fixture-seq-4","sequence":"4","decoder_version":"simulator-pet-v1"}}`, 848, 4),
		base("item.acquired", "joymax.pet_inventory", "0xB034", `{"item":{"model":851,"servername":"ITEM_UNKNOWN_TEST","quantity":1},"quantity_delta":1,"destination_container":{"type":"pets","id":"17","slot":6},"acquisition_method":"pet_pickup","packet_observation":{"observation_id":"fixture-seq-5","sequence":"5","decoder_version":"simulator-pet-v1"}}`, 851, 5),
	}
	items[2].OccurredAt = occurredAt.Add(-time.Second)
	items[3].OccurredAt = occurredAt.Add(-time.Second)
	items[4].OccurredAt = occurredAt.Add(-2 * time.Second)
	results, _, err := store.AppendBatch(ctx, credential.AgentID, items)
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.Status != "persisted" {
			t.Fatalf("event was not persisted: %+v", result)
		}
	}

	rarePage, err := store.List(ctx, Filter{Server: serverName, Kind: "drop.rare", IncludePetPickups: true, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if rarePage.Total != 2 || len(rarePage.Events) != 1 || rarePage.NextCursor == "" {
		t.Fatalf("rare combined page/count = %+v", rarePage)
	}
	rareSecond, err := store.List(ctx, Filter{Server: serverName, Kind: "drop.rare", IncludePetPickups: true, Limit: 1, Cursor: rarePage.NextCursor})
	if err != nil {
		t.Fatal(err)
	}
	if rareSecond.Total != 2 || len(rareSecond.Events) != 1 || rareSecond.Events[0].Kind != "item.acquired" || rareSecond.Events[0].ItemDropClass != "rare" || rareSecond.Events[0].ItemDropClassVersion != "fixture-rarity-v1" {
		t.Fatalf("rare pet pickup page = %+v", rareSecond)
	}
	normalPage, err := store.List(ctx, Filter{Server: serverName, Kind: "drop.item", IncludePetPickups: true, Limit: 10})
	if err != nil || normalPage.Total != 2 {
		t.Fatalf("normal combined page/count = %+v, %v", normalPage, err)
	}
	allPage, err := store.List(ctx, Filter{Server: serverName, Limit: 10})
	if err != nil || allPage.Total != 5 {
		t.Fatalf("All feed must retain unknown pet pickups and both drop kinds: %+v, %v", allPage, err)
	}
	if _, err := store.List(ctx, Filter{Server: serverName, Kind: "item.acquired", IncludePetPickups: true}); err == nil {
		t.Fatal("pet pickup filter was accepted outside Normal/Rare Drops")
	}
	owned := base("item.quantity_increased", "phbot.state_diff", "item_container",
		`{"item":{"model":848,"servername":"ITEM_NORMAL_TEST","quantity":1,"api_fields":{"blues":{"9":5}}},"quantity_delta":1,"destination_container":{"type":"inventory","slot":29},"acquisition_method":"unknown"}`, 848, 6)
	owned.OccurredAt = occurredAt.Add(time.Second)
	results, _, err = store.AppendBatch(ctx, credential.AgentID, []AgentEvent{owned})
	if err != nil || results[0].Status != "persisted" {
		t.Fatalf("owned gain = %+v, %v", results, err)
	}
	combined, err := store.List(ctx, Filter{Server: serverName, Kind: "drop.item", IncludePetPickups: true, IncludeOwnedGains: true, Limit: 10})
	if err != nil || combined.Total != 3 || combined.Events[0].ID != owned.ID || combined.Events[0].ItemDropClass != "normal" {
		t.Fatalf("normal drop feed omitted recipient-owned gain: %+v, %v", combined, err)
	}
	if !json.Valid(combined.Events[0].Payload) || !bytes.Contains(combined.Events[0].Payload, []byte(`"9":5`)) {
		t.Fatalf("recipient item rolls were lost: %s", combined.Events[0].Payload)
	}
}
