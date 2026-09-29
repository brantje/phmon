package mobs

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

func TestSamplesReplayOnceEnforceStationaryRateAndAggregateObservers(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL to run mob observation integration tests")
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
	server := "mob-" + credential.AgentID
	charactersStore := characters.NewStore(pool)
	firstCharacter, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "ObserverA"})
	if err != nil {
		t.Fatal(err)
	}
	secondCharacter, err := charactersStore.Resolve(ctx, characters.Identity{Server: server, Name: "ObserverB"})
	if err != nil {
		t.Fatal(err)
	}
	firstSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, firstCharacter, 1)
	if err != nil {
		t.Fatal(err)
	}
	secondSession, err := charactersStore.ClaimSessionID(ctx, credential.AgentID, secondCharacter, 1)
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM mob_observation_samples WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM character_sessions WHERE agent_id=$1`, credential.AgentID)
		_, _ = pool.Exec(context.Background(), `DELETE FROM characters WHERE server_key=lower($1)`, server)
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	})
	var dbNow time.Time
	if err := pool.QueryRow(ctx, `SELECT now()`).Scan(&dbNow); err != nil {
		t.Fatal(err)
	}
	sampledAt := dbNow.UTC().Truncate(time.Microsecond)
	store := NewStore(pool)
	first := Sample{ID: "00000000-0000-4000-8000-000000000011", CharacterID: firstCharacter, SessionID: firstSession,
		AreaID: "region:25273", FloorID: "unmapped", Region: 25273, SampledAt: sampledAt,
		Observer: Position{X: 10, Y: 20}, Monsters: []Monster{}}
	inserted, err := store.Append(ctx, credential.AgentID, mapprofile.GreatestDatasetID, first, dbNow)
	if err != nil || !inserted {
		t.Fatalf("first empty sample inserted=%v err=%v", inserted, err)
	}
	second := Sample{ID: "00000000-0000-4000-8000-000000000012", CharacterID: secondCharacter, SessionID: secondSession,
		AreaID: "region:25273", FloorID: "unmapped", Region: 25273, SampledAt: sampledAt,
		Observer: Position{X: 12, Y: 18}, Monsters: []Monster{{ID: "900", Model: int64Pointer(500), Type: "1", Region: 25273, X: 400, Y: 21}}}
	inserted, err = store.Append(ctx, credential.AgentID, mapprofile.GreatestDatasetID, second, dbNow)
	if err != nil || !inserted {
		t.Fatalf("second observer sample inserted=%v err=%v", inserted, err)
	}
	inserted, err = store.Append(ctx, credential.AgentID, mapprofile.GreatestDatasetID, second, dbNow)
	if err != nil || inserted {
		t.Fatalf("replay inserted=%v err=%v", inserted, err)
	}
	conflictingReplay := second
	conflictingReplay.Monsters = []Monster{}
	if _, err := store.Append(ctx, credential.AgentID, mapprofile.GreatestDatasetID, conflictingReplay, dbNow); !errors.Is(err, ErrConflict) {
		t.Fatalf("sample ID with changed contents error=%v", err)
	}
	duplicateWindow := second
	duplicateWindow.ID = "00000000-0000-4000-8000-000000000013"
	if _, err := store.Append(ctx, credential.AgentID, mapprofile.GreatestDatasetID, duplicateWindow, dbNow); !errors.Is(err, ErrSamplingFrequency) {
		t.Fatalf("same-session stationary rate guard error=%v", err)
	}
	crossCellEmpty := Sample{ID: "00000000-0000-4000-8000-000000000014", CharacterID: secondCharacter, SessionID: secondSession,
		AreaID: "region:25273", FloorID: "unmapped", Region: 25273, SampledAt: sampledAt,
		Observer: Position{X: 400, Y: 20}, Monsters: []Monster{}}
	if inserted, err = store.Append(ctx, credential.AgentID, mapprofile.GreatestDatasetID, crossCellEmpty, dbNow); err != nil || !inserted {
		t.Fatalf("cross-cell empty sample inserted=%v err=%v", inserted, err)
	}
	result, err := store.Density(ctx, DensityFilter{Server: server, AreaID: "region:25273", FloorID: "unmapped",
		From: sampledAt.Add(-time.Minute), To: sampledAt.Add(time.Minute), Limit: 100})
	if err != nil {
		t.Fatal(err)
	}
	if result.Metric != "observer_local_average_count" || len(result.Cells) != 2 {
		t.Fatalf("observer-local metric or cross-cell groups missing: %+v", result)
	}
	observerCell := result.Cells[0]
	if observerCell.ObserverCellX != 0 || observerCell.ObserverCellY != 0 || observerCell.EligibleSamples != 2 || observerCell.MonsterRows != 1 || observerCell.AverageObserved != 0.5 {
		t.Fatalf("monster row should remain associated with the observer-local samples: %+v", observerCell)
	}
	monsterCell := result.Cells[1]
	if monsterCell.ObserverCellX != 2 || monsterCell.ObserverCellY != 0 || monsterCell.EligibleSamples != 1 || monsterCell.MonsterRows != 0 || monsterCell.AverageObserved != 0 {
		t.Fatalf("a monster coordinate alone must not imply coverage or density in its cell: %+v", monsterCell)
	}
}

func int64Pointer(value int64) *int64 { return &value }
