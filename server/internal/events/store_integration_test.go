package events

import (
	"context"
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestDeathEventsAreDurableIdempotentAndScoped(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run event integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pool.Close() })
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
	server := "events-" + credential.AgentID
	characterID, err := characters.NewStore(pool).Resolve(ctx, characters.Identity{Server: server, Name: "Alpha"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characters.NewStore(pool).ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	var databaseNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&databaseNow); err != nil {
		t.Fatal(err)
	}
	baseTime := databaseNow.UTC().Add(-15 * time.Minute).Truncate(time.Microsecond)
	if _, err := pool.Exec(ctx, `UPDATE character_sessions SET started_at=$2::timestamptz-interval '30 minutes',ended_at=$2::timestamptz-interval '10 minutes',end_reason='left' WHERE session_id=$1`, sessionID, databaseNow); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})

	store := NewStore(pool)
	first := AgentDeath{
		ID: credential.AgentID, OccurredAt: baseTime,
		Source: "phbot.callback", SourceRef: "EVENT_DIED",
		Region: intPointer(25273), X: floatPointer(10), Y: floatPointer(20), Z: floatPointer(30),
		Payload: json.RawMessage(`{"cause":"unknown"}`),
	}
	inserted, err := store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, first)
	if err != nil || !inserted {
		t.Fatalf("first event insert = %v, %v", inserted, err)
	}
	inserted, err = store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, first)
	if err != nil || inserted {
		t.Fatalf("idempotent retry = %v, %v", inserted, err)
	}
	newSessionID, err := characters.NewStore(pool).ClaimSessionID(ctx, credential.AgentID, characterID, 2)
	if err != nil {
		t.Fatal(err)
	}
	if inserted, err = store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, first); err != nil || inserted {
		t.Fatalf("delayed replay from prior owned session = %v, %v", inserted, err)
	}
	conflicting := first
	conflicting.Payload = json.RawMessage(`{"cause":"different"}`)
	if _, err := store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, conflicting); !errors.Is(err, ErrEventConflict) {
		t.Fatalf("same event id with different content: %v", err)
	}
	unowned := first
	unowned.ID = newTestEventID(t)
	unowned.OccurredAt = databaseNow.UTC()
	if _, err := store.AppendDeath(ctx, credential.AgentID, characterID, "00000000-0000-4000-8000-000000000199", unowned); !errors.Is(err, ErrUnauthorizedSession) {
		t.Fatalf("unowned session accepted: %v", err)
	}
	stale := first
	stale.ID = newTestEventID(t)
	stale.OccurredAt = databaseNow.UTC().Add(-4 * time.Minute)
	if _, err := store.AppendDeath(ctx, credential.AgentID, characterID, sessionID, stale); !errors.Is(err, ErrUnauthorizedSession) {
		t.Fatalf("old session reported a post-session occurrence: %v", err)
	}
	otherCharacterID, err := characters.NewStore(pool).Resolve(ctx, characters.Identity{Server: server, Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	wrongCharacter := first
	wrongCharacter.ID = newTestEventID(t)
	wrongCharacter.OccurredAt = databaseNow.UTC()
	if _, err := store.AppendDeath(ctx, credential.AgentID, otherCharacterID, sessionID, wrongCharacter); !errors.Is(err, ErrUnauthorizedSession) {
		t.Fatalf("session was accepted for another character: %v", err)
	}

	second := first
	second.ID = newTestEventID(t)
	second.OccurredAt = databaseNow.UTC().Truncate(time.Microsecond)
	if inserted, err = store.AppendDeath(ctx, credential.AgentID, characterID, newSessionID, second); err != nil || !inserted {
		t.Fatalf("second event insert = %v, %v", inserted, err)
	}
	from := baseTime.Add(-time.Second)
	to := second.OccurredAt.Add(time.Minute)
	page, err := store.List(ctx, Filter{Server: server, CharacterID: characterID, From: &from, To: &to, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Events) != 1 || page.Events[0].ID != second.ID || page.NextCursor == "" {
		t.Fatalf("first filtered page: %+v", page)
	}
	page, err = store.List(ctx, Filter{Server: server, CharacterID: characterID, From: &from, To: &to, Cursor: page.NextCursor, Limit: 1})
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 2 || len(page.Events) != 1 || page.Events[0].ID != first.ID || page.NextCursor != "" {
		t.Fatalf("second filtered page: %+v", page)
	}
	page, err = store.List(ctx, Filter{Server: "different-server", Limit: 10})
	if err != nil || page.Total != 0 {
		t.Fatalf("server filter leaked events: page=%+v err=%v", page, err)
	}
}

func intPointer(value int) *int           { return &value }
func floatPointer(value float64) *float64 { return &value }

func newTestEventID(t *testing.T) string {
	t.Helper()
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	return credential.AgentID
}
