package tradenexus

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
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
	})
	base := time.Now().UTC().Truncate(time.Microsecond)
	earlier := sampleSighting(server, "Bandit", base.Add(-4*time.Minute), 7)
	first := sampleSighting(server, "Bandit", base.Add(-time.Minute), 10)
	latest := sampleSighting(server, "bandit", base, 20)
	other := sampleSighting(server, "Other", base.Add(-2*time.Minute), 30)
	old := sampleSighting(server, "Old", base.Add(-48*time.Hour), 40)
	for _, sighting := range []Sighting{earlier, first, latest, other, old} {
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
	if err != nil || page.Total != 2 || len(page.Sightings) != 1 || page.Sightings[0].ID != latest.ID || page.Sightings[0].Position.X != 20 || page.NextCursor == "" {
		t.Fatalf("first page = %+v err=%v", page, err)
	}
	next, err := store.List(ctx, Filter{Server: server, Query: "band", Limit: 1, Cursor: page.NextCursor})
	if err != nil || len(next.Sightings) != 1 || next.NextCursor != "" || next.Sightings[0].ID != earlier.ID {
		t.Fatalf("second page = %+v err=%v", next, err)
	}
	var storedX float64
	if err := pool.QueryRow(ctx, `SELECT x FROM thief_sightings WHERE sighting_id = $1::uuid`, first.ID).Scan(&storedX); err != nil || storedX != 10 {
		t.Fatalf("stored repeat x = %v err=%v", storedX, err)
	}
	removed, err := store.DeleteOlderThan(ctx, 1)
	if err != nil || removed < 1 {
		t.Fatalf("removed = %d err=%v", removed, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM thief_sightings WHERE server_name=$1`, server).Scan(&remaining); err != nil || remaining != 4 {
		t.Fatalf("remaining = %d err=%v", remaining, err)
	}
}

func TestTradeReportStoreInsertListAndRetention(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run trade report integration tests")
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
	serverName := "TradeReports-" + time.Now().UTC().Format("150405.000000")
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM trade_reports WHERE server_name=$1`, serverName)
	})
	base := time.Now().UTC().Truncate(time.Microsecond)
	first := sampleTrade(serverName, "aat-1", base.Add(-time.Minute), 45000)
	duplicate := first
	duplicate.ID = newTestID(base, "duplicate")
	duplicate.Gold = int32Ptr(-5)
	latest := sampleTrade(serverName, "aat-2", base, -1200)
	latest.Reason = "thief"
	latest.Outcome = "failed"
	latest.Thief = &tradeName{Name: "Bandit"}
	latest.Goods = &[]TradeGood{{Name: "Silk", Quantity: 40}}
	died := sampleTrade(serverName, "aat-died", base.Add(-2*time.Minute), 0)
	died.Outcome = "failed"
	died.Reason = "transport_died"
	died.Transport = "Horse"
	old := sampleTrade(serverName, "aat-old", base.Add(-48*time.Hour), 10)
	old.ReceivedAt = base.Add(-48 * time.Hour)
	stored, err := store.InsertTrade(ctx, first)
	if err != nil {
		t.Fatal(err)
	}
	again, err := store.InsertTrade(ctx, duplicate)
	if err != nil || again.ID != stored.ID {
		t.Fatalf("duplicate = %+v err=%v", again, err)
	}
	if _, err := store.InsertTrade(ctx, latest); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertTrade(ctx, died); err != nil {
		t.Fatal(err)
	}
	if _, err := store.InsertTrade(ctx, old); err != nil {
		t.Fatal(err)
	}
	var gold int32
	if err := pool.QueryRow(ctx, `SELECT gold FROM trade_reports WHERE trade_id=$1::uuid`, stored.ID).Scan(&gold); err != nil || gold != 45000 {
		t.Fatalf("first gold = %d err=%v", gold, err)
	}
	page, err := store.ListTrades(ctx, Filter{Server: strings.ToUpper(serverName), Query: "band", Limit: 1})
	if err != nil || page.Total != 1 || len(page.Reports) != 1 || page.Reports[0].Thief == nil || page.Reports[0].Thief.Name != "Bandit" {
		t.Fatalf("thief page = %+v err=%v", page, err)
	}
	routePage, err := store.ListTrades(ctx, Filter{Server: serverName, Query: "Donwhang", Limit: 10})
	if err != nil || routePage.Total != 4 {
		t.Fatalf("route page = %+v err=%v", routePage, err)
	}
	removed, err := store.DeleteTradesOlderThan(ctx, 1)
	if err != nil || removed < 1 {
		t.Fatalf("removed = %d err=%v", removed, err)
	}
	var remaining int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM trade_reports WHERE server_name=$1`, serverName).Scan(&remaining); err != nil || remaining != 3 {
		t.Fatalf("remaining = %d err=%v", remaining, err)
	}
}

func sampleTrade(server, ref string, finished time.Time, gold int32) TradeReport {
	goods := []TradeGood{{Name: "Silk", Quantity: 120}}
	return TradeReport{
		ID: newTestID(finished, ref), Server: server, Ref: ref, Outcome: "success", Reason: "sold",
		Route:     TradeRoute{From: "Jangan", To: "Donwhang"},
		Waypoints: []Waypoint{{Name: "chau_approach", X: 37643, Y: 7342}},
		Goods:     &goods, Gold: &gold, Stars: "Max",
		Reporter:   Reporter{Name: "CharName", App: "AdvancedAutoTrade", Version: "1.0.0"},
		FinishedAt: finished, ReceivedAt: finished,
	}
}

func int32Ptr(value int32) *int32 { return &value }

func sampleSighting(server, name string, received time.Time, x float64) Sighting {
	return Sighting{
		ID: newTestID(received, name), Server: server, ThiefName: name,
		Position: &Position{Region: 25735, X: x, Y: 5}, PositionSource: SourceThief,
		Reporter: Reporter{Name: "trader", App: "AdvancedAutoTrade", Version: "1"},
		Origin:   OriginExternal, ObservedAt: received, ReceivedAt: received,
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
