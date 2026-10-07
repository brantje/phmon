package analytics_test

import (
	"context"
	"fmt"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/analytics"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
)

// TestAnalyticsMillionEventQueryBounds is an opt-in load gate. It is intended
// for a disposable PostgreSQL database, never the operator's application DB.
func TestAnalyticsMillionEventQueryBounds(t *testing.T) {
	if os.Getenv("ANALYTICS_LOAD_TEST") != "1" {
		t.Skip("set ANALYTICS_LOAD_TEST=1 against a disposable PostgreSQL database")
	}
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Fatal("TEST_DATABASE_URL is required for the disposable analytics load test")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Minute)
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
	server := "analytics-load-" + credential.AgentID[:8]
	const fleetSize = 50
	characterIDs := make([]string, 0, fleetSize)
	sessionIDs := make([]string, 0, fleetSize)
	characterStore := characters.NewStore(pool)
	for index := 0; index < fleetSize; index++ {
		characterID, err := characterStore.Resolve(ctx, characters.Identity{
			Server: server, Name: fmt.Sprintf("LoadFixture%02d", index),
		})
		if err != nil {
			t.Fatal(err)
		}
		sessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
		if err != nil {
			t.Fatal(err)
		}
		characterIDs = append(characterIDs, characterID)
		sessionIDs = append(sessionIDs, sessionID)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM activity_events WHERE agent_id=$1::uuid`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=$1`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1::uuid`, credential.AgentID)
	})
	base := time.Now().UTC().Truncate(time.Second)
	started := time.Now()
	commandTag, err := pool.Exec(ctx, `INSERT INTO activity_events(event_id,schema_version,kind,category,agent_id,character_id,session_id,server_name,occurred_at,source,source_ref,region,zone_name,payload)
SELECT gen_random_uuid(),1,'character.died','character',$1::uuid,
       ($2::text[])[((event_number-1)%cardinality($2::text[]))+1]::uuid,
       ($3::text[])[((event_number-1)%cardinality($3::text[]))+1]::uuid,$4,
       $5::timestamptz-((event_number::bigint*7776000)/1000000)*interval '1 second',
       'analytics.load','million-event-fixture',((event_number-1)%50)+1,
       'Fixture Zone '||(((event_number-1)%50)+1)::text,'{}'::jsonb
FROM generate_series(1,1000000) AS events(event_number)`, credential.AgentID, characterIDs, sessionIDs, server, base)
	if err != nil {
		t.Fatal(err)
	}
	t.Logf("seeded %d canonical events in %s", commandTag.RowsAffected(), time.Since(started).Round(time.Millisecond))
	if commandTag.RowsAffected() != 1_000_000 {
		t.Fatalf("seeded %d events, want 1000000", commandTag.RowsAffected())
	}
	// The bulk fixture intentionally bypasses the normal, trickle-fed agent
	// ingestion path; refresh planner statistics before measuring production
	// queries so the test reflects an analyzed million-event table.
	if _, err := pool.Exec(ctx, `ANALYZE activity_events`); err != nil {
		t.Fatal(err)
	}
	store := analytics.NewStore(pool)
	query := func(label string, span time.Duration, threshold time.Duration) {
		t.Helper()
		from, to := base.Add(-span), base.Add(time.Minute)
		started := time.Now()
		snapshot, err := store.Query(ctx, analytics.Filter{
			Server: server, View: analytics.ViewDeaths, From: from, To: to,
			Timezone: "UTC", Bucket: "day", GroupBy: "location", PageSize: 25,
		})
		elapsed := time.Since(started)
		if err != nil {
			t.Fatalf("%s analytics query failed after %s: %v", label, elapsed.Round(time.Millisecond), err)
		}
		expected := expectedLoadEvents(span)
		breakdownTotal := int64(0)
		for _, point := range snapshot.Breakdown {
			var amount int64
			if _, err := fmt.Sscan(point.Value, &amount); err != nil {
				t.Fatalf("invalid breakdown count %q: %v", point.Value, err)
			}
			breakdownTotal += amount
		}
		if snapshot.Total != fmt.Sprint(expected) || breakdownTotal != expected || len(snapshot.Breakdown) != 21 || len(snapshot.TimeSeries) == 0 || len(snapshot.Occurrences) != 25 {
			t.Fatalf("%s query lost exact totals or bounded output: total=%s series=%d rows=%d", label, snapshot.Total, len(snapshot.TimeSeries), len(snapshot.Occurrences))
		}
		t.Logf("%s analytics query completed in %s (target %s)", label, elapsed.Round(time.Millisecond), threshold)
		if elapsed > threshold {
			t.Errorf("%s analytics query took %s, target is %s", label, elapsed.Round(time.Millisecond), threshold)
		}
	}
	query("first 90-day", 90*24*time.Hour, 3*time.Second)
	query("warm 30-day", 30*24*time.Hour, time.Second)
	query("warm 7-day", 7*24*time.Hour, time.Second)
	query("warm 1-day", 24*time.Hour, time.Second)
	if t.Failed() {
		t.Log(fmt.Sprintf("host/database: %s", databaseURLHost(databaseURL)))
	}
}

func expectedLoadEvents(span time.Duration) int64 {
	maxAge := int64(span / time.Second)
	var count int64
	for eventNumber := int64(1); eventNumber <= 1_000_000; eventNumber++ {
		ageSeconds := eventNumber * 7_776_000 / 1_000_000
		if ageSeconds <= maxAge {
			count++
		}
	}
	return count
}

func databaseURLHost(value string) string {
	config, err := pgxpool.ParseConfig(value)
	if err != nil {
		return "unavailable"
	}
	return config.ConnConfig.Host
}
