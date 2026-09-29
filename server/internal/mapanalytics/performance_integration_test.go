package mapanalytics

import (
	"context"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
	"phmon/server/internal/mapprofile"
)

func TestLargeMovementHistoryRemainsBounded(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run map analytics performance integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
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
	server := "heat-perf-" + credential.AgentID
	characterStore := characters.NewStore(pool)
	characterID, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "PerfWalker"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_position_samples WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var dbNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&dbNow); err != nil {
		t.Fatal(err)
	}
	from := dbNow.UTC().Add(-30 * 24 * time.Hour)
	_, err = pool.Exec(ctx, `INSERT INTO character_position_samples
		(agent_id,character_id,session_id,server_name,dataset_id,sampled_at,region,x,y)
		SELECT $1,$2,$3,$4,$5,$6::timestamptz + (n * interval '20 seconds'),25273,
			((n % 10000)::double precision),((n / 10000) * 220)::double precision
		FROM generate_series(0,99999) AS n`,
		credential.AgentID, characterID, sessionID, server, mapprofile.GreatestDatasetID, from)
	if err != nil {
		t.Fatal(err)
	}
	started := time.Now()
	result, err := NewStore(pool).Heatmap(ctx, Filter{
		Layer: LayerPlayerMovement, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: from.Add(-time.Minute), To: dbNow.UTC().Add(time.Minute),
		Resolution: 96, Limit: 500,
	})
	elapsed := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if len(result.Points) > 500 || !result.Truncated {
		t.Fatalf("large history was not bounded: points=%d truncated=%v", len(result.Points), result.Truncated)
	}
	if elapsed > 5*time.Second {
		t.Fatalf("100k-row movement heatmap query took %s", elapsed)
	}
	t.Logf("100k movement samples aggregated to %d points in %s", len(result.Points), elapsed)
}

func TestAccumulatedMobHistoryUsesTimeScopeIndexAndStaysBounded(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run map analytics performance integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 45*time.Second)
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
	server := "mob-heat-perf-" + credential.AgentID
	characterStore := characters.NewStore(pool)
	characterID, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "MobPerf"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM map_heatmap_resets WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM mob_observation_samples WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var dbNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&dbNow); err != nil {
		t.Fatal(err)
	}
	const sampleCount = 100000
	start := dbNow.UTC().Add(-time.Duration(sampleCount) * 20 * time.Second)
	_, err = pool.Exec(ctx, `INSERT INTO mob_observation_samples
		(sample_id,agent_id,character_id,session_id,server_name,dataset_id,area_id,floor_id,region,
		 sampled_at,sample_hash,sample_minute,observer_x,observer_y,observer_cell_x,observer_cell_y)
		SELECT gen_random_uuid(),$1,$2,$3,$4,$5,'region:25273','unmapped',25273,
			$6::timestamptz + (n * interval '20 seconds'),
			decode(md5(n::text)||md5('phmon-'||n::text),'hex'),
			date_trunc('minute',$6::timestamptz + (n * interval '20 seconds')),
			(n % 1000)::double precision,((n / 1000) % 1000)::double precision,n::integer,0
		FROM generate_series(0,$7-1) AS n`,
		credential.AgentID, characterID, sessionID, server, mapprofile.GreatestDatasetID, start, sampleCount)
	if err != nil {
		t.Fatal(err)
	}
	_, err = pool.Exec(ctx, `INSERT INTO mob_observations
		(sample_id,ordinal,monster_id,model_id,monster_type,region,x,y)
		SELECT sample_id,0,sample_id::text,
			CASE WHEN observer_cell_x % 2 = 0 THEN 500 ELSE 501 END,
			CASE WHEN observer_cell_x % 2 = 0 THEN 'General' ELSE 'Champion' END,
			region,observer_x + 10,observer_y + 10
		FROM mob_observation_samples WHERE lower(server_name)=lower($1) AND dataset_id=$2`,
		server, mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE mob_observation_samples`); err != nil {
		t.Fatal(err)
	}
	if _, err := pool.Exec(ctx, `ANALYZE mob_observations`); err != nil {
		t.Fatal(err)
	}

	from := dbNow.UTC().Add(-24 * time.Hour)
	to := dbNow.UTC().Add(time.Minute)
	store := NewStore(pool)
	filter := Filter{
		Server: server, DatasetID: mapprofile.GreatestDatasetID, AreaID: "world", FloorID: "world",
		From: from, To: to, Resolution: 96, Limit: 100,
	}

	filter.Layer = LayerMobTypes
	started := time.Now()
	sightings, err := store.Heatmap(ctx, filter)
	sightingsDuration := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if !sightings.Truncated || len(sightings.Points) > 100 || sightings.SourceRows < 4000 || sightings.SourceRows > 5000 {
		t.Fatalf("100k mob sightings were not time-scoped/bounded: points=%d source_rows=%d truncated=%v",
			len(sightings.Points), sightings.SourceRows, sightings.Truncated)
	}
	if sightingsDuration > 5*time.Second {
		t.Fatalf("time-scoped mob sightings query took %s", sightingsDuration)
	}

	filter.Layer = LayerMobObserverAvg
	started = time.Now()
	average, err := store.Heatmap(ctx, filter)
	averageDuration := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	if !average.Truncated || len(average.Points) > 100 || average.SourceRows != sightings.SourceRows {
		t.Fatalf("observer average was not bounded to the same source window: points=%d source_rows=%d sightings=%d truncated=%v",
			len(average.Points), average.SourceRows, sightings.SourceRows, average.Truncated)
	}
	if averageDuration > 5*time.Second {
		t.Fatalf("time-scoped observer average query took %s", averageDuration)
	}

	started = time.Now()
	facets, err := store.MobFacets(ctx, Filter{
		Layer: LayerMobTypes, Server: server, DatasetID: mapprofile.GreatestDatasetID,
		AreaID: "world", FloorID: "world", From: from, To: to, Limit: 100,
	}, 100)
	facetDuration := time.Since(started)
	if err != nil {
		t.Fatal(err)
	}
	var facetRows int64
	for _, facet := range facets {
		facetRows += facet.Count
	}
	if len(facets) != 2 || facetRows != sightings.SourceRows {
		t.Fatalf("facets did not respect the selected time window: facets=%+v source_rows=%d", facets, sightings.SourceRows)
	}
	if facetDuration > 5*time.Second {
		t.Fatalf("time-scoped mob facets query took %s", facetDuration)
	}

	explain := func(name, sql string, args ...any) string {
		t.Helper()
		rows, queryErr := pool.Query(ctx, "EXPLAIN (ANALYZE, BUFFERS) "+sql, args...)
		if queryErr != nil {
			t.Fatal(queryErr)
		}
		defer rows.Close()
		lines := make([]string, 0)
		for rows.Next() {
			var line string
			if scanErr := rows.Scan(&line); scanErr != nil {
				t.Fatal(scanErr)
			}
			lines = append(lines, line)
		}
		if rows.Err() != nil {
			t.Fatal(rows.Err())
		}
		plan := strings.Join(lines, "\n")
		if !strings.Contains(plan, "mob_observation_samples_heatmap_time_idx") {
			t.Fatalf("%s plan did not use mob heatmap time index:\n%s", name, plan)
		}
		t.Logf("%s plan:\n%s", name, plan)
		return plan
	}

	explain("mob sightings", `SELECT o.region,floor(o.x/96)::bigint,floor(o.y/96)::bigint,count(*)
		FROM mob_observation_samples s JOIN mob_observations o ON o.sample_id=s.sample_id
		WHERE lower(s.server_name)=lower($1) AND s.dataset_id=$2 AND s.sampled_at >= $3 AND s.sampled_at < $4
		GROUP BY o.region,floor(o.x/96)::bigint,floor(o.y/96)::bigint`,
		server, mapprofile.GreatestDatasetID, from, to)
	explain("observer average", `SELECT s.region,s.observer_cell_x,s.observer_cell_y,count(DISTINCT s.sample_id),count(o.ordinal)
		FROM mob_observation_samples s LEFT JOIN mob_observations o ON o.sample_id=s.sample_id
		WHERE lower(s.server_name)=lower($1) AND s.dataset_id=$2 AND s.sampled_at >= $3 AND s.sampled_at < $4
		GROUP BY s.region,s.observer_cell_x,s.observer_cell_y`,
		server, mapprofile.GreatestDatasetID, from, to)
	explain("mob facets", `SELECT o.monster_type,o.model_id,count(*)
		FROM mob_observation_samples s JOIN mob_observations o ON o.sample_id=s.sample_id
		WHERE lower(s.server_name)=lower($1) AND s.dataset_id=$2 AND s.sampled_at >= $3 AND s.sampled_at < $4
		GROUP BY o.monster_type,o.model_id`,
		server, mapprofile.GreatestDatasetID, from, to)

	t.Logf("100k mob samples: sightings=%s observer_average=%s facets=%s source_rows=%d",
		sightingsDuration, averageDuration, facetDuration, sightings.SourceRows)
}
