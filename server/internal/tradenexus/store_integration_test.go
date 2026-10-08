package tradenexus

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
	"phmon/server/internal/players"
)

func TestThiefSightingStoreRecentListAndRetention(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run thief sighting integration tests")
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
	store := NewStore(pool)
	server := "TradeNexus-" + time.Now().UTC().Format("150405.000000")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM thief_sightings WHERE server_name=$1`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, strings.ToLower(server))
	})
	base := time.Now().UTC().Truncate(time.Microsecond)
	first := sampleSighting(server, "Bandit", base.Add(-time.Minute), 10)
	latest := sampleSighting(server, "bandit", base, 20)
	other := sampleSighting(server, "Other", base.Add(-2*time.Minute), 30)
	old := sampleSighting(server, "Old", base.Add(-48*time.Hour), 40)
	for _, sighting := range []Sighting{first, latest, other, old} {
		if err := store.Insert(ctx, sighting); err != nil {
			t.Fatal(err)
		}
	}
	missing := latest
	missing.ID = "00000000-0000-4000-8000-000000000099"
	missing.EventID = "00000000-0000-4000-8000-000000000098"
	if err := store.Insert(ctx, missing); err == nil {
		t.Fatal("sighting accepted a missing activity event")
	}
	recent, truncated, err := store.Recent(ctx, strings.ToUpper(server), base.Add(-5*time.Minute), 256)
	if err != nil || truncated || len(recent) != 2 || recent[0].ThiefName != "bandit" || recent[0].Position.X != 20 || recent[1].ThiefName != "Other" {
		t.Fatalf("recent = %+v truncated=%v err=%v", recent, truncated, err)
	}
	page, err := store.List(ctx, Filter{Server: server, Query: "band", Limit: 1})
	if err != nil || page.Total != 2 || len(page.Sightings) != 1 || page.NextCursor == "" {
		t.Fatalf("first page = %+v err=%v", page, err)
	}
	next, err := store.List(ctx, Filter{Server: server, Query: "band", Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(next.Sightings) != 1 || next.NextCursor != "" || next.Sightings[0].ID == page.Sightings[0].ID {
		t.Fatalf("second page = %+v err=%v", next, err)
	}
	// Original evidence must survive while the registry importer is behind.
	_, err = store.DeleteOlderThan(ctx, 1)
	var pending int
	if err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM thief_sightings WHERE sighting_id=$1::uuid`, old.ID).Scan(&pending); err != nil || pending != 1 {
		t.Fatal("source retention raced evidence preservation", err)
	}
	if _, err = players.NewStore(pool).ImportThiefSightings(ctx); err != nil {
		t.Fatal(err)
	}
	removed, err := store.DeleteOlderThan(ctx, 1)
	if err != nil || removed < 1 {
		t.Fatalf("removed = %d err=%v", removed, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM thief_sightings WHERE server_name=$1`, server).Scan(&remaining); err != nil || remaining != 3 {
		t.Fatalf("remaining = %d err=%v", remaining, err)
	}
}

func sampleSighting(server, name string, received time.Time, x float64) Sighting {
	return Sighting{
		ID: newTestID(received, name), Server: server, ThiefName: name,
		Position: &Position{Region: 25735, X: x, Y: 5}, PositionSource: SourceThief,
		Reporter: Reporter{Name: "trader", App: "AdvancedAutoTrade", Version: "1"},
		Origin:   OriginExternal, ObservedAt: received, ReceivedAt: received,
	}
}

func TestThiefRegistryRejectsInvalidRowsWithoutBlockingValidEvidence(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run thief sighting integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	registry := players.NewStore(pool)
	server := "TradeNexus-Reject-" + time.Now().UTC().Format("150405.000000")
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM thief_sightings WHERE server_name=$1`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, strings.ToLower(server))
	}()
	old := time.Now().UTC().Add(-48 * time.Hour).Truncate(time.Microsecond)
	valid := sampleSighting(server, "Valid", old, 10)
	invalid := sampleSighting(server, " Invalid ", old.Add(-time.Second), 20)
	future := sampleSighting(server, "Deferred", old, 30)
	future.ObservedAt = time.Now().Add(time.Hour)
	for _, sighting := range []Sighting{invalid, valid, future} {
		if err = store.Insert(ctx, sighting); err != nil {
			t.Fatal(err)
		}
	}
	if _, err = registry.ImportThiefSightings(ctx); err != nil {
		t.Fatal("invalid source blocked valid evidence", err)
	}
	var pinned, rejected int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM player_observations WHERE server_key=$1 AND source='thief_sighting' AND pinned`, strings.ToLower(server)).Scan(&pinned); err != nil || pinned != 1 {
		t.Fatalf("valid evidence not preserved: %d %v", pinned, err)
	}
	var rejectedAt time.Time
	if err = pool.QueryRow(ctx, `SELECT registry_rejected_at FROM thief_sightings WHERE sighting_id=$1::uuid AND registry_rejection_reason='invalid_player_observation'`, invalid.ID).Scan(&rejectedAt); err != nil {
		t.Fatal("invalid source not recorded", err)
	}
	if _, err = registry.ImportThiefSightings(ctx); err != nil {
		t.Fatal(err)
	}
	var retryRejectedAt time.Time
	if err = pool.QueryRow(ctx, `SELECT registry_rejected_at FROM thief_sightings WHERE sighting_id=$1::uuid`, invalid.ID).Scan(&retryRejectedAt); err != nil || !retryRejectedAt.Equal(rejectedAt) {
		t.Fatal("replay retried a rejected source", err)
	}
	if removed, err := store.DeleteOlderThan(ctx, 1); err != nil || removed != 1 {
		t.Fatalf("grace/pending protection: removed %d, %v", removed, err)
	}
	if _, err = pool.Exec(ctx, `UPDATE thief_sightings SET registry_rejected_at=now()-interval '25 hours' WHERE sighting_id=$1::uuid`, invalid.ID); err != nil {
		t.Fatal(err)
	}
	if removed, err := store.DeleteOlderThan(ctx, 1); err != nil || removed != 1 {
		t.Fatalf("expired rejection not pruned: %d, %v", removed, err)
	}
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM thief_sightings WHERE server_name=$1 AND registry_rejected_at IS NOT NULL`, server).Scan(&rejected); err != nil || rejected != 0 {
		t.Fatal("expired rejection remained", err)
	}
	var pending int
	if err = pool.QueryRow(ctx, `SELECT count(*) FROM thief_sightings WHERE sighting_id=$1::uuid AND registry_rejected_at IS NULL`, future.ID).Scan(&pending); err != nil || pending != 1 {
		t.Fatal("future evidence was lost or permanently rejected", err)
	}
	page, err := registry.List(ctx, players.Filter{Server: server})
	if err != nil || len(page.Players) != 1 || page.Players[0].ObservedName != "Valid" {
		t.Fatal("invalid or future evidence fabricated a profile", err)
	}
	if _, err = pool.Exec(ctx, `UPDATE thief_sightings SET observed_at=$2 WHERE sighting_id=$1::uuid`, future.ID, old); err != nil {
		t.Fatal(err)
	}
	if _, err = registry.ImportThiefSightings(ctx); err != nil {
		t.Fatal("deferred evidence could not recover", err)
	}
	if removed, err := store.DeleteOlderThan(ctx, 1); err != nil || removed != 1 {
		t.Fatalf("recovered evidence retention: %d, %v", removed, err)
	}
}

func newTestID(received time.Time, name string) string {
	id, err := newSightingID()
	if err != nil {
		panic(err)
	}
	_ = received
	_ = name
	return id
}
