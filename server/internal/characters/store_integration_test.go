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
	guild := "Founders"
	a, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Alpha", Guild: &guild})
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
	zone := "Jangan"
	fullState := State{Level: &level, HP: &hp, HPMax: &hpmax, MP: &mp, CurrentEXP: &exp, SP: &sp, Gold: &gold, Region: &region, Zone: &zone, X: &x, Y: &y}
	if err := store.ClaimSession(ctx, credential.AgentID, a, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, a, 1, fullState); err != nil {
		t.Fatal(err)
	}
	b, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Beta"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimSession(ctx, credential.AgentID, b, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, b, 2, fullState); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, credential.AgentID, a, 2, State{Level: &level}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("cross-generation character update accepted: %v", err)
	}
	alpha, err := store.Get(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if !alpha.Online || alpha.Level == nil || *alpha.Level != 110 {
		t.Fatalf("alpha presence/state incorrect: %+v", alpha)
	}
	beta, err := store.Get(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !beta.Online {
		t.Fatalf("beta should be online: %+v", beta)
	}
	// Character authority is independent of logical-agent/socket validity. A
	// second generation may take over A while the first still observes B.
	if err := store.ClaimSession(ctx, credential.AgentID, a, 2); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, credential.AgentID, a, 1, State{Level: &level}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale generation state accepted after takeover: %v", err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, a, 1, State{Level: &level}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale generation snapshot reclaimed character: %v", err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, a, 2, State{HP: &hp}); err != nil {
		t.Fatal(err)
	}
	// The same old socket can still explicitly claim an unrelated character.
	c, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Gamma"})
	if err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimSession(ctx, credential.AgentID, c, 1); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, c, 1, fullState); err != nil {
		t.Fatal(err)
	}
	if err := store.End(ctx, credential.AgentID, c, 1, "left"); err != nil {
		t.Fatal(err)
	}
	// A full new-session snapshot clears fields unavailable in the new session.
	if err := store.End(ctx, credential.AgentID, a, 2, "left"); err != nil {
		t.Fatal(err)
	}
	if err := store.ClaimSession(ctx, credential.AgentID, a, 3); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, a, 3, State{HP: &hp}); err != nil {
		t.Fatal(err)
	}
	alpha, err = store.Get(ctx, a)
	if err != nil {
		t.Fatal(err)
	}
	if alpha.Gold != nil || alpha.Zone != nil || alpha.HP == nil || *alpha.HP != hp {
		t.Fatalf("new full snapshot retained stale state: %+v", alpha)
	}
	// A later explicit reconnect of the same durable identity reuses its ID.
	aReconnect, err := store.Resolve(ctx, Identity{Server: "slice2-server", Name: "ALPHA"})
	if err != nil || aReconnect != a {
		t.Fatalf("identity changed across reconnect: %s, %v", aReconnect, err)
	}
	if _, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Beta"}); err != nil {
		t.Fatal(err)
	}
	if err := store.EndAgent(ctx, credential.AgentID, 1); err != nil {
		t.Fatal(err)
	}
	alpha, err = store.Get(ctx, a)
	if err != nil || !alpha.Online {
		t.Fatalf("closing generation 1 ended the character owned by generation 3: %+v err=%v", alpha, err)
	}
	if err := store.ClaimSession(ctx, credential.AgentID, b, 3); err != nil {
		t.Fatal(err)
	}
	if err := store.Snapshot(ctx, credential.AgentID, b, 3, fullState); err != nil {
		t.Fatal(err)
	}
	if err := store.Update(ctx, credential.AgentID, b, 2, State{Level: &level}); !errors.Is(err, ErrNotFound) {
		t.Fatalf("stale generation update accepted: %v", err)
	}
	if err := store.EndAgent(ctx, credential.AgentID, 2); err != nil {
		t.Fatal(err)
	}
	beta, err = store.Get(ctx, b)
	if err != nil {
		t.Fatal(err)
	}
	if !beta.Online {
		t.Fatal("stale agent cleanup ended replacement generation")
	}
	if err := store.EndAgent(ctx, credential.AgentID, 3); err != nil {
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
	noGuild := ""
	if _, err := store.Resolve(ctx, Identity{Server: "Slice2-Server", Name: "Alpha", Guild: &noGuild}); err != nil {
		t.Fatal(err)
	}
	results, err = store.List(ctx, "founders", "")
	if err != nil {
		t.Fatal(err)
	}
	for _, result := range results {
		if result.ID == a {
			t.Fatal("character still matched its previous guild after observed leave")
		}
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
