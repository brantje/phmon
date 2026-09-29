package mapanalytics

import (
	"context"
	"errors"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/database"
	"phmon/server/internal/mapprofile"
)

func TestRecordPositionFencesSessionsAndSuppressesStationarySamples(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run map analytics integration tests")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 20*time.Second)
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
	agentStore := agents.NewStore(pool)
	if err := agentStore.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	server := "movement-" + credential.AgentID
	characterStore := characters.NewStore(pool)
	characterID, err := characterStore.Resolve(ctx, characters.Identity{Server: server, Name: "Walker"})
	if err != nil {
		t.Fatal(err)
	}
	sessionID, err := characterStore.ClaimSessionID(ctx, credential.AgentID, characterID, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM map_heatmap_resets WHERE lower(server_name)=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_position_samples WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var now time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&now); err != nil {
		t.Fatal(err)
	}
	store := NewStore(pool)
	base := PositionSample{AgentID: credential.AgentID, CharacterID: characterID, SessionID: sessionID,
		DatasetID: mapprofile.GreatestDatasetID, SampledAt: now.UTC().Truncate(time.Microsecond), Region: 25273, X: 10, Y: 20}
	if inserted, err := store.RecordPosition(ctx, base, now); err != nil || !inserted {
		t.Fatalf("first sample inserted=%v err=%v", inserted, err)
	}
	tooSoon := base
	tooSoon.SampledAt = base.SampledAt.Add(time.Second)
	tooSoon.X = 30
	if inserted, err := store.RecordPosition(ctx, tooSoon, now.Add(time.Second)); err != nil || inserted {
		t.Fatalf("rate-limited sample inserted=%v err=%v", inserted, err)
	}
	stationary := base
	stationary.SampledAt = base.SampledAt.Add(3 * time.Second)
	stationary.X = 12
	if inserted, err := store.RecordPosition(ctx, stationary, now.Add(3*time.Second)); err != nil || inserted {
		t.Fatalf("stationary sample inserted=%v err=%v", inserted, err)
	}
	moved := stationary
	moved.SampledAt = base.SampledAt.Add(4 * time.Second)
	moved.X = 15
	if inserted, err := store.RecordPosition(ctx, moved, now.Add(4*time.Second)); err != nil || !inserted {
		t.Fatalf("moved sample inserted=%v err=%v", inserted, err)
	}
	transition := moved
	transition.SampledAt = base.SampledAt.Add(7 * time.Second)
	transition.Region = 25274
	if inserted, err := store.RecordPosition(ctx, transition, now.Add(7*time.Second)); err != nil || !inserted {
		t.Fatalf("region transition inserted=%v err=%v", inserted, err)
	}
	if err := characterStore.EndSession(ctx, credential.AgentID, characterID, 1, sessionID, "left"); err != nil {
		t.Fatal(err)
	}
	stale := transition
	stale.SampledAt = base.SampledAt.Add(10 * time.Second)
	stale.X = 100
	if _, err := store.RecordPosition(ctx, stale, now.Add(10*time.Second)); !errors.Is(err, ErrStaleSession) {
		t.Fatalf("stale session error=%v", err)
	}
	var count int
	if err := pool.QueryRow(ctx, `SELECT count(*) FROM character_position_samples WHERE character_id=$1`, characterID).Scan(&count); err != nil {
		t.Fatal(err)
	}
	if count != 3 {
		t.Fatalf("expected first, moved and region-transition samples; got %d", count)
	}
}
