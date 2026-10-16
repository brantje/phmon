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

func TestTraderHunterHistorySearchHideOwnAndPagination(t *testing.T) {
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
	server := "hunter-" + credential.AgentID
	otherServer := "hunter-other-" + credential.AgentID
	charactersStore := characters.NewStore(pool)
	reporter, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "Scout"})
	if err != nil {
		t.Fatal(err)
	}
	if _, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "SelfHunter"}); err != nil {
		t.Fatal(err)
	}
	if _, err := charactersStore.Resolve(ctx, characters.Identity{Server: otherServer, Name: "OtherRealmHero"}); err != nil {
		t.Fatal(err)
	}
	sessionID, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, reporter, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		cleanupCtx := context.Background()
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM activity_events WHERE agent_id=$1::uuid`, credential.AgentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM character_sessions WHERE agent_id=$1::uuid`, credential.AgentID)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM players WHERE server_key IN (lower($1),lower($2))`, server, otherServer)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM characters WHERE server_key IN (lower($1),lower($2))`, server, otherServer)
		_, _ = pool.Exec(cleanupCtx, `DELETE FROM agents WHERE agent_id=$1::uuid`, credential.AgentID)
	})
	now := time.Now().UTC().Truncate(time.Microsecond)
	for index, spotted := range []string{"SelfHunter", "Rival", "Rival", "OtherRealmHero"} {
		_, err := pool.Exec(ctx, `INSERT INTO activity_events (
			event_id,schema_version,kind,category,agent_id,character_id,session_id,
			server_name,occurred_at,source,source_ref,payload
		) VALUES (
			gen_random_uuid(),1,'job.hunter_trader_seen','job',$1::uuid,$2::uuid,$3::uuid,
			$4,$5,'phbot.callback','EVENT_HUNTER_SPAWN',jsonb_build_object('value',$6::text)
		)`, credential.AgentID, reporter, sessionID, server, now.Add(time.Duration(index-4)*time.Minute), spotted)
		if err != nil {
			t.Fatal(err)
		}
	}

	store := NewStore(pool)
	filter := Filter{Server: server, Kind: "job.hunter_trader_seen", HideOwn: true, Limit: 10}
	page, err := store.List(ctx, filter)
	if err != nil {
		t.Fatal(err)
	}
	if page.Total != 3 || len(page.Events) != 3 {
		t.Fatalf("hide own must exclude spotted SelfHunter, not reporter Scout: total=%d rows=%d", page.Total, len(page.Events))
	}
	if page.Events[0].Character != "Scout" {
		t.Fatalf("reporter identity lost: %+v", page.Events[0])
	}
	for _, event := range page.Events {
		if string(event.Payload) == "" {
			t.Fatal("empty sighting payload")
		}
	}
	filter.HideOwn = false
	all, err := store.List(ctx, filter)
	if err != nil || all.Total != 4 {
		t.Fatalf("unchecked hide own must restore the owned sighting: total=%d error=%v", all.Total, err)
	}
	filter.HideOwn = true
	filter.CharacterQuery = "rIvAl"
	rivals, err := store.List(ctx, filter)
	if err != nil || rivals.Total != 2 {
		t.Fatalf("spotted-name search must preserve both occurrences: total=%d error=%v", rivals.Total, err)
	}
	filter.CharacterQuery = "Scout"
	reported, err := store.List(ctx, filter)
	if err != nil || reported.Total != 3 {
		t.Fatalf("reporter search must match all non-owned sightings: total=%d error=%v", reported.Total, err)
	}
	filter.CharacterQuery = ""
	filter.Limit = 1
	first, err := store.List(ctx, filter)
	if err != nil || first.Total != 3 || len(first.Events) != 1 || first.NextCursor == "" {
		t.Fatalf("first cursor page = %+v, error=%v", first, err)
	}
	seen := map[string]bool{first.Events[0].ID: true}
	cursor := first.NextCursor
	for cursor != "" {
		filter.Cursor = cursor
		next, err := store.List(ctx, filter)
		if err != nil || len(next.Events) != 1 || next.Total != 3 || seen[next.Events[0].ID] {
			t.Fatalf("incorrect keyset pagination: %+v error=%v", next, err)
		}
		seen[next.Events[0].ID] = true
		cursor = next.NextCursor
	}
	if len(seen) != 3 {
		t.Fatalf("pagination lost sightings: %d", len(seen))
	}
	filter = Filter{Server: server, CharacterQuery: "Rival", Limit: 10}
	general, err := store.List(ctx, filter)
	if err != nil || general.Total != 0 {
		t.Fatalf("normal event search unexpectedly matches spotted payload: total=%d error=%v", general.Total, err)
	}
	// A directly observed normal-name/job-alias pair is an additional
	// owned identity, but an unrelated server's own name is not.
	_, err = pool.Exec(ctx, `INSERT INTO players (
		server_name,server_key,observed_name,observed_name_key,player_name,job_name,
		first_seen_at,last_seen_at,state_observed_at,last_history_at
	) VALUES ($1,lower($1),'SelfJobAlias','selfjobalias','SelfHunter','SelfJobAlias',$2,$2,$2,$2)`,
		server, now)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO activity_events (
		event_id,schema_version,kind,category,agent_id,character_id,session_id,
		server_name,occurred_at,source,source_ref,payload
	) VALUES (
		gen_random_uuid(),1,'job.hunter_trader_seen','job',$1::uuid,$2::uuid,$3::uuid,
		$4,$5,'phbot.callback','EVENT_HUNTER_SPAWN',jsonb_build_object('value','SelfJobAlias')
	)`, credential.AgentID, reporter, sessionID, server, now)
	if err != nil {
		t.Fatal(err)
	}
	hiddenAlias, err := store.List(ctx, Filter{Server: server, Kind: "job.hunter_trader_seen", HideOwn: true, Limit: 10})
	if err != nil || hiddenAlias.Total != 3 {
		t.Fatalf("verified owned job alias leaked: total=%d error=%v", hiddenAlias.Total, err)
	}
	unhiddenAlias, err := store.List(ctx, Filter{Server: server, Kind: "job.hunter_trader_seen", Limit: 10})
	if err != nil || unhiddenAlias.Total != 5 {
		t.Fatalf("unfiltered aliases not restored: total=%d error=%v", unhiddenAlias.Total, err)
	}
	if _, err := store.List(ctx, Filter{Server: server, Kind: "character.died", HideOwn: true, Limit: 10}); err == nil {
		t.Fatal("hide own must not apply to an unrelated event kind")
	}
}
