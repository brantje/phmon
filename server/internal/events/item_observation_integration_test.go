package events

import (
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

func TestRelatedItemObservationsFailClosed(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL to run event integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
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
	server := "item-details-" + credential.AgentID
	characterID, err := characters.NewStore(pool).Resolve(ctx, characters.Identity{Server: server, Name: "Observed"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characters.NewStore(pool).ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	}()
	store := NewStore(pool)
	var now time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	for index, tc := range []struct {
		name   string
		mutate func(*AgentEvent)
		extra  bool
		want   bool
	}{
		{name: "bag", want: true},
		{name: "pet", want: true, mutate: func(g *AgentEvent) {
			g.Payload = json.RawMessage(`{"item":{"model":777,"plus":4},"quantity_delta":1,"destination_container":{"type":"pets","id":"pet-7","slot":1}}`)
		}},
		{name: "ambiguous copies", mutate: func(g *AgentEvent) {
			g.Payload = json.RawMessage(`{"item":{"model":777},"quantity_delta":1,"item_instance_unobserved":true,"destination_container":{"type":"inventory","slot":14}}`)
		}},
		{name: "multiple gains", extra: true},
		{name: "too late", mutate: func(g *AgentEvent) { g.OccurredAt = g.OccurredAt.Add(5 * time.Second) }},
		{name: "different location", mutate: func(g *AgentEvent) { g.X = floatPointer(1000) }},
		{name: "missing location", mutate: func(g *AgentEvent) { g.Region = nil }},
		{name: "different model", mutate: func(g *AgentEvent) { g.ItemModel = int64Pointer64(888) }},
		{name: "another stored link", mutate: func(g *AgentEvent) {
			g.Payload = json.RawMessage(`{"item":{"model":777},"quantity_delta":1,"drop_event_id":"00000000-0000-4000-8000-000000000199","destination_container":{"type":"inventory","slot":14}}`)
		}},
	} {
		t.Run(tc.name, func(t *testing.T) {
			drop := AgentEvent{ID: newTestEventID(t), Schema: 1, Kind: "drop.item", Category: "drop", CharacterID: characterID, SessionID: sessionID,
				Sequence: int64Pointer64(int64(index*3 + 1)),
				Server:   server, Character: "Observed", OccurredAt: now.Add(time.Duration(index*20) * time.Second), Source: "phbot.callback", SourceRef: "EVENT_ITEM_DROP",
				ItemModel: int64Pointer64(777), Region: intPointer(25273), X: floatPointer(10), Y: floatPointer(20), Z: floatPointer(30), Payload: json.RawMessage(`{"model":777}`)}
			gain := drop
			gain.ID = newTestEventID(t)
			gain.Sequence = int64Pointer64(int64(index*3 + 2))
			gain.Kind = "item.acquired"
			gain.Category = "item"
			gain.Source = "phbot.state_diff"
			gain.SourceRef = "item_container"
			gain.OccurredAt = drop.OccurredAt.Add(time.Second)
			gain.Payload = json.RawMessage(`{"item":{"model":777,"plus":4},"quantity_delta":1,"destination_container":{"type":"inventory","slot":14}}`)
			if tc.mutate != nil {
				tc.mutate(&gain)
			}
			var payload map[string]any
			if err := json.Unmarshal(gain.Payload, &payload); err != nil {
				t.Fatal(err)
			}
			payload["acquisition_method"] = "unknown"
			payload["item"].(map[string]any)["model"] = *gain.ItemModel
			gain.Payload, _ = json.Marshal(payload)
			batch := []AgentEvent{drop, gain}
			if tc.extra {
				duplicate := gain
				duplicate.ID = newTestEventID(t)
				duplicate.Sequence = int64Pointer64(int64(index*3 + 3))
				batch = append(batch, duplicate)
			}
			for _, event := range batch {
				if err := validateAgentEvent(event); err != nil {
					t.Fatal(err)
				}
			}
			results, _, err := store.AppendBatch(ctx, credential.AgentID, batch)
			if err != nil {
				t.Fatal(err)
			}
			for _, result := range results {
				if result.Status != "persisted" {
					t.Fatalf("fixture rejected: %+v", result)
				}
			}
			page, err := store.List(ctx, Filter{EventID: drop.ID, Limit: 1})
			if err != nil || len(page.Events) != 1 {
				t.Fatalf("lookup: %+v %v", page, err)
			}
			event := page.Events[0]
			if got := event.RelatedItemObservation != nil; got != tc.want {
				t.Fatalf("related observation present=%v want=%v: %+v", got, tc.want, event.RelatedItemObservation)
			}
			var canonical map[string]any
			if err := json.Unmarshal(event.Payload, &canonical); err != nil || len(canonical) != 1 || canonical["model"] != float64(777) {
				t.Fatalf("canonical drop mutated: %s", event.Payload)
			}
		})
	}
}
