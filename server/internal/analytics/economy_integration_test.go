package analytics_test

import (
	"context"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/analytics"
)

func TestAnalyticsReadPoolDoesNotWaitForOccupiedIngestionPool(t *testing.T) {
	f := newReviewFixture(t, 1)
	f.sample(t, 0, f.now.Add(-time.Minute), 0, 1000, 100)
	config := f.pool.Config()
	config.MaxConns = 1
	ingestion, err := pgxpool.NewWithConfig(context.Background(), config)
	if err != nil {
		t.Fatal(err)
	}
	defer ingestion.Close()
	held, err := ingestion.Acquire(context.Background())
	if err != nil {
		t.Fatal(err)
	}
	defer held.Release()
	store := analytics.NewStore(ingestion)
	store.SetReadPool(f.pool)
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	snapshot, err := store.Query(ctx, analytics.Filter{
		Server: f.server, View: analytics.ViewEconomy, From: f.now.Add(-time.Hour), To: f.now, Timezone: "UTC",
	})
	if err != nil || snapshot.Total != "1" {
		t.Fatalf("chart waited behind the occupied ingestion pool: %+v %v", snapshot, err)
	}
}

func TestEconomyBalancesUseBankObservationAtSelectedInventorySample(t *testing.T) {
	f := newReviewFixture(t, 2)
	start := f.now.Truncate(24 * time.Hour).Add(-24 * time.Hour)
	f.sample(t, 0, start.Add(time.Hour), 0, 1000, 100)
	f.sample(t, 0, start.Add(2*time.Hour), 0, 1000, 200)
	f.sample(t, 0, start.Add(25*time.Hour), 0, 1000, 300)
	f.sample(t, 1, start.Add(25*time.Hour), 0, 1000, 800)
	for revision, value := range []struct {
		at   time.Time
		gold int64
	}{
		{start.Add(90 * time.Minute), 1000},
		{start.Add(24 * time.Hour), 2000},
		// A later bank observation must not change an earlier balance.
		{start.Add(26 * time.Hour), 9999},
	} {
		_, err := f.pool.Exec(context.Background(), `INSERT INTO character_storage_gold_samples
(character_id,session_id,resource_revision,sampled_at,gold) VALUES($1::uuid,$2::uuid,$3,$4,$5)`,
			f.chars[0], f.sessions[0], revision+1, value.at, value.gold)
		if err != nil {
			t.Fatal(err)
		}
	}
	snapshot, err := analytics.NewStore(f.pool).Query(context.Background(), analytics.Filter{
		Server: f.server, View: analytics.ViewEconomy, From: start, To: start.Add(27 * time.Hour), Timezone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if len(snapshot.TimeSeries) != 3 || snapshot.TimeSeries[0].Value != "1200" || snapshot.TimeSeries[1].Value != "2300" || snapshot.TimeSeries[2].Value != "800" {
		t.Fatalf("latest bucket balances lost bank time fencing or inventory-only fallback: %+v", snapshot.TimeSeries)
	}
	if len(snapshot.Breakdown) != 2 || snapshot.Breakdown[0].CharacterID != f.chars[0] || snapshot.Breakdown[0].Value != "2300" || snapshot.Breakdown[1].Value != "800" {
		t.Fatalf("rankings must sort combined balances after selecting the latest inventory sample: %+v", snapshot.Breakdown)
	}
}

func TestEconomyFleetLimitPreservesScopeTotalAndRichestCharacter(t *testing.T) {
	f := newReviewFixture(t, 22)
	for index := range f.chars {
		gold := int64(100)
		if index == 0 {
			gold = 10000
		}
		f.sample(t, index, f.now.Add(time.Duration(index-30)*time.Minute), 0, 1000, gold)
	}
	snapshot, err := analytics.NewStore(f.pool).Query(context.Background(), analytics.Filter{
		Server: f.server, View: analytics.ViewEconomy, From: f.now.Add(-time.Hour), To: f.now, Timezone: "UTC",
	})
	if err != nil {
		t.Fatal(err)
	}
	if snapshot.Total != "22" || !snapshot.Truncated || len(snapshot.TimeSeries) != 20 || len(snapshot.Breakdown) != 20 {
		t.Fatalf("fleet chart limit changed its scope count: %+v", snapshot)
	}
	if snapshot.Breakdown[0].CharacterID != f.chars[0] {
		t.Fatalf("richest character excluded by the recent-series limit: %+v", snapshot.Breakdown)
	}
	for _, point := range snapshot.TimeSeries {
		if point.CharacterID == f.chars[0] {
			t.Fatal("oldest character should be omitted from the 20 most recent series")
		}
	}
}

// Production had roughly 480,000 inventory samples; bank lookups on every row
// timed out before the fleet chart could be delivered. Use a disposable DB only.
func TestEconomyFleetHistoryQueryBudget(t *testing.T) {
	if os.Getenv("ANALYTICS_LOAD_TEST") != "1" {
		t.Skip("set ANALYTICS_LOAD_TEST=1 against a disposable PostgreSQL database")
	}
	f := newReviewFixture(t, 12)
	ctx := context.Background()
	for index, id := range f.chars {
		_, err := f.pool.Exec(ctx, `INSERT INTO character_metric_samples
(character_id,session_id,agent_id,server_key,sampled_at,gold)
SELECT $1::uuid,$2::uuid,$3::uuid,$4,$5::timestamptz-n*interval '10 seconds',$6::bigint+n
FROM generate_series(1,40000) n`, id, f.sessions[index], f.agent, f.server, f.now, int64(index+1)*10_000_000_000)
		if err != nil {
			t.Fatal(err)
		}
		_, err = f.pool.Exec(ctx, `INSERT INTO character_storage_gold_samples
(character_id,session_id,resource_revision,sampled_at,gold)
SELECT $1::uuid,$2::uuid,n,$3::timestamptz-(54-n)*interval '2 hours',1000000000
FROM generate_series(1,53) n`, id, f.sessions[index], f.now)
		if err != nil {
			t.Fatal(err)
		}
	}
	if _, err := f.pool.Exec(ctx, `ANALYZE character_metric_samples; ANALYZE character_storage_gold_samples`); err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	snapshot, err := analytics.NewStore(f.pool).Query(ctx, analytics.Filter{
		Server: f.server, View: analytics.ViewEconomy, From: f.now.Add(-7 * 24 * time.Hour), To: f.now.Add(time.Second), Timezone: "Europe/Amsterdam",
	})
	if err != nil {
		t.Fatalf("480,000-sample fleet chart failed after %s: %v", time.Since(started), err)
	}
	if snapshot.Total != "12" || len(snapshot.Breakdown) != 12 || len(snapshot.TimeSeries) < 60 {
		t.Fatalf("fleet chart omitted characters or days: %+v", snapshot)
	}
	t.Logf("480,000 inventory samples, 636 bank observations: fleet query %s", time.Since(started))
}
