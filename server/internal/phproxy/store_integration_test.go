package phproxy

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/database"
	"phmon/server/internal/players"
)

func TestEnrichmentDoesNotMutatePlayerTracking(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run ph-proxy enrichment integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
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
	agentStore := agents.NewStore(pool)
	if err := agentStore.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM agents WHERE agent_id = $1", credential.AgentID)
	})

	serverName := "PhProxy-" + time.Now().UTC().Format("150405.000")
	registry := players.NewRegistry(pool)
	store := NewStore(pool, nil)
	registry.SetEnrichmentPromoter(store)
	level := 55
	start := time.Date(2026, 10, 11, 10, 0, 0, 0, time.UTC)
	if err := registry.Apply(ctx, []players.Observation{{
		Server: serverName, ObservedName: "GearTarget", Level: &level,
		Source: players.SourceMapPlayers, ObservedAt: start,
	}}); err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), "DELETE FROM players WHERE server_key=$1", strings.ToLower(serverName))
	})

	var playerID string
	var firstSeen, lastSeen, stateObserved time.Time
	err = pool.QueryRow(ctx,
		`SELECT id::text, first_seen_at, last_seen_at, state_observed_at FROM players WHERE server_key=$1 AND observed_name_key=$2`,
		strings.ToLower(serverName), strings.ToLower("GearTarget"),
	).Scan(&playerID, &firstSeen, &lastSeen, &stateObserved)
	if err != nil {
		t.Fatal(err)
	}
	var obsCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM player_observations WHERE player_id=$1::uuid`, playerID).Scan(&obsCount); err != nil {
		t.Fatal(err)
	}

	body := strings.Replace(validFixtureBody(), "FixturePlayer", "GearTarget", 1)
	body = strings.Replace(body, `"server": "Greatest"`, `"server": "`+serverName+`"`, 1)
	body = strings.Replace(body, "22222222-2222-4222-8222-222222222222", credential.AgentID, 1)
	resp, err := store.Submit(ctx, credential.AgentID, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if len(resp.Results) != 1 || resp.Results[0].Status != "applied" {
		t.Fatalf("unexpected response: %+v", resp)
	}

	var firstSeenAfter, lastSeenAfter, stateObservedAfter time.Time
	if err := pool.QueryRow(ctx,
		`SELECT first_seen_at, last_seen_at, state_observed_at FROM players WHERE id=$1::uuid`, playerID,
	).Scan(&firstSeenAfter, &lastSeenAfter, &stateObservedAfter); err != nil {
		t.Fatal(err)
	}
	if !firstSeen.Equal(firstSeenAfter) || !lastSeen.Equal(lastSeenAfter) || !stateObserved.Equal(stateObservedAfter) {
		t.Fatal("tracking timestamps changed after enrichment")
	}
	var obsCountAfter int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM player_observations WHERE player_id=$1::uuid`, playerID).Scan(&obsCountAfter); err != nil {
		t.Fatal(err)
	}
	if obsCountAfter != obsCount {
		t.Fatalf("observation count changed: %d -> %d", obsCount, obsCountAfter)
	}

	enrichment, err := store.LoadEnrichmentWithPromotion(ctx, playerID, serverName, strings.ToLower(serverName), strings.ToLower("GearTarget"))
	if err != nil || enrichment == nil || len(enrichment.Gear) != 1 {
		t.Fatalf("enrichment=%+v err=%v", enrichment, err)
	}
}

func TestPendingPromotionAfterTrackingCreatesPlayer(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run ph-proxy enrichment integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
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
	agentStore := agents.NewStore(pool)
	if err := agentStore.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}

	serverName := "PhProxyPending-" + time.Now().UTC().Format("150405.000")
	registry := players.NewRegistry(pool)
	store := NewStore(pool, nil)
	registry.SetEnrichmentPromoter(store)

	body := strings.Replace(validFixtureBody(), "FixturePlayer", "PendingHero", 1)
	body = strings.Replace(body, `"server": "Greatest"`, `"server": "`+serverName+`"`, 1)
	body = strings.Replace(body, "22222222-2222-4222-8222-222222222222", credential.AgentID, 1)
	resp, err := store.Submit(ctx, credential.AgentID, []byte(body))
	if err != nil {
		t.Fatal(err)
	}
	if resp.Results[0].Status != "pending" {
		t.Fatalf("expected pending, got %+v", resp.Results[0])
	}
	var playerCount int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*)::int FROM players WHERE server_key=$1`, strings.ToLower(serverName)).Scan(&playerCount); err != nil {
		t.Fatal(err)
	}
	if playerCount != 0 {
		t.Fatal("pending enrichment created a player row")
	}

	level := 12
	if err := registry.Apply(ctx, []players.Observation{{
		Server: serverName, ObservedName: "PendingHero", Level: &level,
		Source: players.SourceMapPlayers, ObservedAt: time.Now().UTC(),
	}}); err != nil {
		t.Fatal(err)
	}
	var playerID string
	if err := pool.QueryRow(ctx,
		`SELECT id::text FROM players WHERE server_key=$1 AND observed_name_key=$2`,
		strings.ToLower(serverName), strings.ToLower("PendingHero"),
	).Scan(&playerID); err != nil {
		t.Fatal(err)
	}
	enrichment, err := store.LoadEnrichmentWithPromotion(ctx, playerID, serverName, strings.ToLower(serverName), strings.ToLower("PendingHero"))
	if err != nil || enrichment == nil || len(enrichment.Gear) != 1 {
		t.Fatalf("expected promoted gear, got %+v err=%v", enrichment, err)
	}
}
