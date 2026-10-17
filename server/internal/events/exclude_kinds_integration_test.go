package events

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

func TestListExcludeKindsCountsAndPagesOnlyRemainingKinds(t *testing.T) {
	dsn := os.Getenv("TEST_DATABASE_URL")
	if dsn == "" {
		t.Skip("set TEST_DATABASE_URL to run event integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, dsn)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
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
	server := "exclude-" + credential.AgentID
	charactersStore := characters.NewStore(pool)
	observer, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "Observer"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, observer, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM activity_events WHERE agent_id=$1::uuid`, credential.AgentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM character_sessions WHERE agent_id=$1::uuid`, credential.AgentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agents WHERE agent_id=$1::uuid`, credential.AgentID)
	})

	// Newest first: stack changes and chat surround the three kept events.
	rows := []struct{ kind, source, sourceRef string }{
		{"item.quantity_increased", "phbot.state_diff", "item_container"},
		{"drop.item", "phbot.callback", "EVENT_ITEM_DROP"},
		{"chat.message_received", "phbot.chat_callback", "handle_chat"},
		{"item.quantity_increased", "phbot.state_diff", "item_container"},
		{"world.unique_spawned", "joymax.unique_notice", "0x300C"},
		{"item.quantity_increased", "phbot.state_diff", "item_container"},
		{"character.died", "phbot.callback", "EVENT_DIED"},
	}
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index, row := range rows {
		_, err := pool.Exec(ctx, `INSERT INTO activity_events (
			event_id,schema_version,kind,category,agent_id,character_id,session_id,
			server_name,occurred_at,source,source_ref,payload
		) VALUES (
			gen_random_uuid(),1,$1,$2,$3::uuid,$4::uuid,$5::uuid,$6,$7,$8,$9,'{}'::jsonb
		)`, row.kind, eventKinds[row.kind], credential.AgentID, observer, sessionID, server,
			now.Add(-time.Duration(index)*time.Minute), row.source, row.sourceRef)
		if err != nil {
			t.Fatal(err)
		}
	}

	store := NewStore(pool)
	filter := Filter{Server: server, Limit: 2, ExcludeKinds: []string{"item.quantity_increased", "chat.message_received"}}
	first, err := store.List(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if first.Total != 3 || len(first.Events) != 2 || first.NextCursor == "" ||
		first.Events[0].Kind != "drop.item" || first.Events[1].Kind != "world.unique_spawned" {
		t.Fatalf("first excluded page = total %d next %q events %+v", first.Total, first.NextCursor, first.Events)
	}
	filter.Cursor = first.NextCursor
	second, err := store.List(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if len(second.Events) != 1 || second.Events[0].Kind != "character.died" || second.NextCursor != "" {
		t.Fatalf("second excluded page = next %q events %+v", second.NextCursor, second.Events)
	}

	all, err := store.List(ctx, Filter{Server: server, Limit: 10})
	if err != nil {
		t.Fatal(err)
	}
	if all.Total != int64(len(rows)) {
		t.Fatalf("unexcluded total = %d, want %d", all.Total, len(rows))
	}
	// An exclusion narrows a category filter; it never widens it.
	items, err := store.List(ctx, Filter{Server: server, Category: "item", Limit: 10, ExcludeKinds: []string{"item.quantity_increased"}})
	if err != nil {
		t.Fatal(err)
	}
	if items.Total != 0 || len(items.Events) != 0 {
		t.Fatalf("category with its only kind excluded = total %d events %+v", items.Total, items.Events)
	}
}
