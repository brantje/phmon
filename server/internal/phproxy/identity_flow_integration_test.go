package phproxy

import (
	"context"
	"encoding/json"
	"os"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/database"
	"phmon/server/internal/players"
)

func identityEnvironment(t *testing.T) (context.Context, *pgxpool.Pool, *players.Registry, *Store, string, string) {
	t.Helper()
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for isolated PostgreSQL tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 40*time.Second)
	t.Cleanup(cancel)
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(pool.Close)
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	if err = agents.NewStore(pool).CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	scope := "Identity-" + credential.AgentID
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=$1`, strings.ToLower(scope))
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1 OR server_key=$2`, strings.ToLower(scope), strings.ToLower(scope+"-other"))
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	registry := players.NewRegistry(pool)
	store := NewStore(pool, nil)
	registry.SetEnrichmentPromoter(store)
	return ctx, pool, registry, store, scope, credential.AgentID
}

func submitIdentity(t *testing.T, ctx context.Context, store *Store, scope, agent, name string, aliases []string, at time.Time, gearCount int) SubmitResponse {
	t.Helper()
	body := identityBody(t, scope, agent, name, aliases, at, gearCount)
	out, err := store.Submit(ctx, agent, body)
	if err != nil {
		t.Fatal(err)
	}
	return out
}
func identityBody(t *testing.T, scope, agent, name string, aliases []string, at time.Time, gearCount int) []byte {
	t.Helper()
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	links := []IdentityLink{}
	for _, alias := range aliases {
		links = append(links, IdentityLink{Alias: alias, Status: "inferred", Reason: "job_item_transition", FirstMatched: at, LastMatched: at, Matches: 1, Profile: ProfileName, SourceOpcode: OpcodeSpawnEquipment})
	}
	gear := []GearEntry{}
	for i := 0; i < gearCount; i++ {
		gear = append(gear, GearEntry{RefItemID: 200 + i, OptLevel: intPtr(7)})
	}
	evidence := map[string]json.RawMessage{}
	for _, field := range []string{"name", "gear", "identity_links"} {
		reason := EvidenceReasonVerified
		if field == "identity_links" {
			reason = EvidenceReasonInferredJobTransition
		}
		evidence[field], _ = json.Marshal(FieldEvidence{ObservedAt: at, Profile: ProfileName, SourceOpcode: OpcodeSpawnEquipment, Reason: reason, SourceIdentityKind: IdentityKindCharacterName, SourceName: name})
	}
	request := wireRequestV2{SchemaVersion: 2, RequestID: credential.AgentID, AgentID: agent, Server: scope, Players: []wirePlayerV2{{Name: name, IdentityKind: IdentityKindCharacterName, Gear: &gear, IdentityLinks: &links, FieldEvidence: evidence}}}
	body, err := json.Marshal(request)
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestMagikAliasGroupingPreservesTracking(t *testing.T) {
	ctx, pool, registry, store, scope, agent := identityEnvironment(t)
	at := time.Date(2026, 10, 12, 0, 0, 0, 0, time.UTC)
	for _, o := range []players.Observation{
		{Server: scope, ObservedName: "Magik", Level: intPtr(110), Source: players.SourceMapPlayers, ObservedAt: at},
		{Server: scope, ObservedName: "xDont", Source: players.SourceMapPlayers, ObservedAt: at.Add(time.Hour)},
		{Server: scope + "-other", ObservedName: "xDont", Source: players.SourceThiefSighting, ObservedAt: at},
	} {
		if err := registry.Apply(ctx, []players.Observation{o}); err != nil {
			t.Fatal(err)
		}
	}
	before, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || before.Total != 2 {
		t.Fatalf("initial list: %+v %v", before, err)
	}
	submitIdentity(t, ctx, store, scope, agent, "Magik", []string{"xDont"}, at.Add(time.Minute), 7)
	page, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "XDONT", Limit: 1})
	if err != nil || page.Total != 1 || len(page.Players) != 1 {
		t.Fatalf("grouped list: %+v %v", page, err)
	}
	player := page.Players[0]
	identity := player.DisplayIdentity
	if identity == nil || identity.PlayerName == nil || *identity.PlayerName != "Magik" || identity.JobAlias == nil || *identity.JobAlias != "xDont" || identity.Status != "inferred" || len(identity.RelatedRecordIDs) != 2 {
		t.Fatalf("identity: %+v", identity)
	}
	if !identity.LastSeenAt.Equal(at.Add(time.Hour)) || !player.LastSeenAt.Equal(at) {
		t.Fatal("native/display sighting times confused")
	}
	for _, id := range identity.RelatedRecordIDs {
		detail, err := registry.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if detail.DisplayIdentity.CanonicalID != player.ID {
			t.Fatal("list/detail identity disagreement")
		}
		history, err := registry.Observations(ctx, id, 25, 0)
		if err != nil || history.Total != 1 {
			t.Fatalf("history changed: %+v %v", history, err)
		}
	}
	enrichment, err := store.LoadEnrichmentWithPromotion(ctx, player.ID, scope, strings.ToLower(scope), "magik")
	if err != nil || enrichment == nil || len(enrichment.Gear) != 7 || len(enrichment.IdentityLinks) != 1 {
		t.Fatalf("gear/link: %+v %v", enrichment, err)
	}
	other, err := registry.List(ctx, players.ListFilter{Server: scope + "-other"})
	if err != nil || other.Total != 1 || other.Players[0].DisplayIdentity.PlayerName != nil || *other.Players[0].DisplayIdentity.JobAlias != "xDont" {
		t.Fatalf("cross-server alias: %+v %v", other, err)
	}
	var rows, levels int
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM players WHERE server_key=$1`, strings.ToLower(scope)).Scan(&rows)
	_ = pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_level_snapshots s JOIN players p ON p.id=s.player_id WHERE p.server_key=$1`, strings.ToLower(scope)).Scan(&levels)
	if rows != 2 || levels != 1 {
		t.Fatalf("tracking changed rows=%d levels=%d", rows, levels)
	}
	if _, err = pool.Exec(ctx, `INSERT INTO characters(server_name,server_key,character_name,identity_key) VALUES($1,$2,'Magik','magik')`, scope, strings.ToLower(scope)); err != nil {
		t.Fatal(err)
	}
	hidden, err := registry.List(ctx, players.ListFilter{Server: scope, HideOwn: true})
	if err != nil || hidden.Total != 0 {
		t.Fatalf("own grouped identity leaked: %+v %v", hidden, err)
	}
	submitIdentity(t, ctx, store, scope, agent, "Magik", []string{}, at.Add(2*time.Hour), 7)
	ungrouped, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || ungrouped.Total != 2 {
		t.Fatalf("clear links: %+v %v", ungrouped, err)
	}
}

func TestIdentityConflictsNamesAndPagination(t *testing.T) {
	ctx, _, registry, store, scope, agent := identityEnvironment(t)
	at := time.Now().UTC().Add(-time.Hour)
	knownFalse, knownTrue := false, true
	for _, o := range []players.Observation{
		{Server: scope, ObservedName: "NamedOnly", IsJobbing: &knownFalse, Source: players.SourceMapPlayers, ObservedAt: at},
		{Server: scope, ObservedName: "AliasOnly", IsJobbing: &knownTrue, Source: players.SourceMapPlayers, ObservedAt: at},
		{Server: scope, ObservedName: "Unclassified", Source: players.SourceMapPlayers, ObservedAt: at},
		{Server: scope, ObservedName: "Anchor", PlayerName: stringPtr("Anchor"), Source: players.SourceMapPlayers, ObservedAt: at},
		{Server: scope, ObservedName: "SecondAnchor", PlayerName: stringPtr("SecondAnchor"), Source: players.SourceMapPlayers, ObservedAt: at},
	} {
		if err := registry.Apply(ctx, []players.Observation{o}); err != nil {
			t.Fatal(err)
		}
	}
	submitIdentity(t, ctx, store, scope, agent, "Anchor", []string{"AliasOnly", "HistoricalAlias"}, at, 1)
	matched, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "HistoricalAlias"})
	if err != nil || matched.Total != 1 {
		t.Fatalf("all alias search: %+v %v", matched, err)
	}
	if *matched.Players[0].DisplayIdentity.JobAlias != "AliasOnly" {
		t.Fatal("equal-time alias tiebreak changed")
	}
	submitIdentity(t, ctx, store, scope, agent, "SecondAnchor", []string{"AliasOnly", "NamedOnly"}, at, 1)
	page, err := registry.List(ctx, players.ListFilter{Server: scope, Sort: "player_name", Limit: 2})
	if err != nil || page.Total != 5 || len(page.Players) != 2 {
		t.Fatalf("conflicts/pagination: %+v %v", page, err)
	}
	second, err := registry.List(ctx, players.ListFilter{Server: scope, Sort: "player_name", Limit: 2, Offset: 2})
	if err != nil || second.Total != 5 || second.Players[0].ID == page.Players[0].ID {
		t.Fatalf("second page: %+v %v", second, err)
	}
	all, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil {
		t.Fatal(err)
	}
	for _, p := range all.Players {
		switch p.ObservedName {
		case "NamedOnly":
			if p.DisplayIdentity.PlayerName == nil || p.DisplayIdentity.JobAlias != nil || p.DisplayIdentity.Status != "conflict" {
				t.Fatalf("normal conflict: %+v", p.DisplayIdentity)
			}
		case "AliasOnly":
			if p.DisplayIdentity.PlayerName != nil || p.DisplayIdentity.JobAlias == nil || p.DisplayIdentity.Status != "conflict" {
				t.Fatalf("alias conflict: %+v", p.DisplayIdentity)
			}
		case "Unclassified":
			if p.DisplayIdentity.PlayerName != nil || p.DisplayIdentity.JobAlias != nil {
				t.Fatal("unclassified name guessed")
			}
		}
	}
}

func TestV2PendingPromotionAndConcurrentFreshness(t *testing.T) {
	ctx, _, registry, store, scope, agent := identityEnvironment(t)
	at := time.Now().UTC().Add(-time.Hour)
	out := submitIdentity(t, ctx, store, scope, agent, "PendingPlayer", []string{"PendingAlias"}, at, 7)
	if out.Results[0].Status != "pending" {
		t.Fatalf("unexpected staging: %+v", out)
	}
	page, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || page.Total != 0 {
		t.Fatal("proxy created a player")
	}
	if err = registry.Apply(ctx, []players.Observation{{Server: scope, ObservedName: "PendingPlayer", Source: players.SourceMapPlayers, ObservedAt: at}, {Server: scope, ObservedName: "PendingAlias", Source: players.SourceMapPlayers, ObservedAt: at}}); err != nil {
		t.Fatal(err)
	}
	page, err = registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || page.Total != 1 {
		t.Fatalf("v2 pending link lost: %+v %v", page, err)
	}
	var wg sync.WaitGroup
	errs := make(chan error, 8)
	for i := 0; i < 8; i++ {
		body := identityBody(t, scope, agent, "PendingPlayer", []string{"PendingAlias"}, at.Add(time.Duration(i+1)*time.Minute), i+1)
		wg.Add(1)
		go func() { defer wg.Done(); _, err := store.Submit(ctx, agent, body); errs <- err }()
	}
	wg.Wait()
	close(errs)
	for err := range errs {
		if err != nil {
			t.Fatal(err)
		}
	}
	enrichment, err := store.LoadEnrichmentWithPromotion(ctx, page.Players[0].ID, scope, strings.ToLower(scope), "pendingplayer")
	if err != nil || len(enrichment.Gear) != 8 {
		t.Fatalf("concurrent stale update: %+v %v", enrichment, err)
	}
	submitIdentity(t, ctx, store, scope, agent, "PendingPlayer", []string{}, at.Add(time.Minute), 0)
	page, err = registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || page.Total != 1 {
		t.Fatal("stale empty links cleared mapping")
	}
}

func stringPtr(s string) *string { return &s }

func TestV2Uint32AndLongNameStaging(t *testing.T) {
	ctx, _, registry, store, scope, agent := identityEnvironment(t)
	at := time.Now().UTC().Add(-time.Hour)
	name := strings.Repeat("n", 128)
	var request wireRequestV2
	_ = json.Unmarshal(identityBody(t, scope, agent, name, []string{}, at, 1), &request)
	maximum := int(4294967295)
	request.Players[0].ModelRefID = &maximum
	evidence := FieldEvidence{ObservedAt: at, Profile: ProfileName, SourceOpcode: OpcodeSpawnEquipment, Reason: EvidenceReasonVerified, SourceIdentityKind: IdentityKindCharacterName, SourceName: name}
	request.Players[0].FieldEvidence["model_ref_id"], _ = json.Marshal(evidence)
	body, _ := json.Marshal(request)
	if _, validationErr := ValidateSubmitBody(body, agent); validationErr != nil {
		t.Fatalf("boundary validation: %#v", validationErr)
	}
	out, err := store.Submit(ctx, agent, body)
	if err != nil || out.Results[0].Status != "pending" {
		t.Fatalf("uint32/128-byte staging: %+v %v", out, err)
	}
	page, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || page.Total != 0 {
		t.Fatal("long proxy name created tracking")
	}
}
