package mapanalytics

import (
	"context"
	"os"
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
