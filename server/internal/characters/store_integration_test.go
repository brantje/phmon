package characters

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/database"
)

func TestCharacterIdentitySessionsSearchAndGroups(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run character integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 15*time.Second)
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
	groupName := "slice2-test-" + credential.AgentID
	t.Cleanup(func() {
		cleanCtx := context.Background()
		_, _ = pool.Exec(cleanCtx, `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM characters WHERE server_key=$1`, "slice2-server")
		_, _ = pool.Exec(cleanCtx, `DELETE FROM character_groups WHERE name=$1`, groupName)
		_, _ = pool.Exec(cleanCtx, `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	store := NewStore(pool)
	a, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Alpha", Guild: "Founders"})
	if err != nil {
		t.Fatal(err)
	}
	a2, err := store.Resolve(ctx, Identity{Server: "slice2-server", Name: "alpha"})
	if err != nil {
		t.Fatal(err)
	}
	if a != a2 {
		t.Fatalf("server/name identity changed: %s != %s", a, a2)
	}
	level, hp, hpmax, mp, exp, sp, gold, region, x, y := 110, int64(500), int64(1000), int64(250), int64(900), int64(42), int64(99), 25000, 12.5, 33.25
	fullState := State{Level: &level, HP: &hp, HPMax: &hpmax, MP: &mp, CurrentEXP: &exp, SP: &sp, Gold: &gold, Region: &region, X: &x, Y: &y}
	if err := store.Snapshot(ctx, credential.AgentID, a, 1, fullState); err != nil {
		t.Fatal(err)
	}
	if err := store.End(ctx, credential.AgentID, a, 1, "left"); err != nil {
		t.Fatal(err)
	}
	b, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, b, 1, fullState); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, credential.AgentID, a, 1, State{Level: &level}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale switched character update accepted: %v", err)
	}
	alpha, err := store.Get(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if alpha.Online || alpha.Level == nil || *alpha.Level != 110 {
		t.Fatalf("alpha presence/state incorrect: %+v", alpha)
	}
	beta, err := store.Get(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !beta.Online {
		t.Fatalf("beta should be online: %+v", beta)
	}
	if _, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Beta"}); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, b, 2, fullState); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, credential.AgentID, b, 1, State{Level: &level}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale generation update accepted: %v", err)
	}
	if err := store.EndAgent(ctx, credential.AgentID, 1); err != nil {
		t.Fatal(err)
	}
	beta, err = store.Get(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !beta.Online {
		t.Fatal("stale agent cleanup ended replacement generation")
	}
	if err := store.EndAgent(ctx, credential.AgentID, 2); err != nil {
		t.Fatal(err)
	}
	beta, err = store.Get(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if beta.Online {
		t.Fatal("agent disconnect left character online")
	}
	group, err := store.CreateGroup(ctx, groupName)
	if err != nil {
		t.Fatal(err)
	}
	if err := store.SetMember(ctx, group.ID, a, true); err != nil {
		t.Fatal(err)
	}
	results, err := store.List(ctx, "founders", "")
	if err != nil {
		t.Fatal(err)
	}
	if len(results) == 0 || results[0].ID != a {
		t.Fatalf("guild search failed: %+v", results)
	}
	groupResults, err := store.List(ctx, "", group.ID)
	if err != nil {
		t.Fatal(err)
	}
	if len(groupResults) != 1 || groupResults[0].ID != a {
		t.Fatalf("group membership failed: %+v", groupResults)
	}
	groups, err := store.Groups(ctx)
	if err != nil {
		t.Fatal(err)
	}
	found := false
	for _, g := range groups {
		if g.ID == group.ID && len(g.Members) == 1 && g.Members[0].ID == a {
			found = true
		}
	}
	if !found {
		t.Fatal("group did not persist membership")
	}
}
