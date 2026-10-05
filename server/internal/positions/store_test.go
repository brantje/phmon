package positions

import (
	"testing"
	"time"
)

const (
	testAgentA = "11111111-2222-4333-8444-555555555555"
	testAgentB = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	testChar   = "99999999-8888-4777-8666-555555555555"
	testSessA  = "77777777-6666-4555-8444-333333333333"
	testSessB  = "22222222-3333-4444-8555-666666666666"
)

func positionFixture(session string, generation, sequence uint64, at time.Time) Position {
	z := 3.0
	return Position{
		AgentID: testAgentA, Generation: generation, CharacterID: testChar, SessionID: session,
		Sequence: sequence, Region: 25000, X: 10, Y: 20, Z: &z, ObservedAt: at,
	}
}

func TestStoreApplyFencesSessionGenerationAndSequence(t *testing.T) {
	now := time.Date(2026, 10, 5, 20, 0, 0, 0, time.UTC)
	store := NewStore()
	store.Claim("Greatest", testAgentA, 7, testChar, testSessA)

	first, ok := store.Apply(positionFixture(testSessA, 7, 1, now), now)
	if !ok || first.Server != "Greatest" {
		t.Fatal("valid position was not admitted with claimed server")
	}
	if _, ok := store.Apply(positionFixture(testSessA, 7, 1, now), now); ok {
		t.Fatal("equal sequence was admitted")
	}
	if _, ok := store.Apply(positionFixture(testSessA, 7, 0, now), now); ok {
		t.Fatal("zero sequence was admitted")
	}
	staleSession := positionFixture(testSessB, 7, 2, now)
	if _, ok := store.Apply(staleSession, now); ok {
		t.Fatal("stale session was admitted")
	}
	staleGeneration := positionFixture(testSessA, 6, 2, now)
	if _, ok := store.Apply(staleGeneration, now); ok {
		t.Fatal("stale generation was admitted")
	}
	wrongAgent := positionFixture(testSessA, 7, 2, now)
	wrongAgent.AgentID = testAgentB
	if _, ok := store.Apply(wrongAgent, now); ok {
		t.Fatal("wrong agent was admitted")
	}
	if _, ok := store.Apply(positionFixture(testSessA, 7, 2, now.Add(-MaxPastAge-time.Second)), now); ok {
		t.Fatal("stale timestamp was admitted")
	}
}

func TestStoreRejectsInvalidCoordinatesAndRegion(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore()
	store.Claim("Greatest", testAgentA, 1, testChar, testSessA)

	cases := []Position{
		positionFixture(testSessA, 1, 1, now),
		positionFixture(testSessA, 1, 2, now),
		positionFixture(testSessA, 1, 3, now),
	}
	cases[0].Region = 0
	cases[1].X = 1_000_001
	cases[2].Y = -1_000_001
	for i, item := range cases {
		if _, ok := store.Apply(item, now); ok {
			t.Fatalf("invalid position %d was admitted", i)
		}
	}
}

func TestNewClaimSupersedesOldAuthorityAndRemovalIsSessionAware(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore()
	store.Claim("Greatest", testAgentA, 1, testChar, testSessA)
	if _, ok := store.Apply(positionFixture(testSessA, 1, 5, now), now); !ok {
		t.Fatal("initial apply failed")
	}

	removed := store.Claim("Greatest", testAgentA, 2, testChar, testSessB)
	if len(removed) != 1 || removed[0].SessionID != testSessA {
		t.Fatalf("unexpected replacement removal: %#v", removed)
	}
	if _, ok := store.Apply(positionFixture(testSessA, 1, 6, now), now); ok {
		t.Fatal("old session moved newly claimed character")
	}
	next := positionFixture(testSessB, 2, 1, now)
	if _, ok := store.Apply(next, now); !ok {
		t.Fatal("new session position rejected")
	}
	if removed := store.RemoveSession(testSessA); len(removed) != 0 {
		t.Fatalf("old cleanup removed newer session: %#v", removed)
	}
	snapshot := store.Snapshot("greatest")
	if len(snapshot) != 1 || snapshot[0].SessionID != testSessB {
		t.Fatalf("newer snapshot lost after delayed cleanup: %#v", snapshot)
	}
}

func TestRemoveAgentGenerationOnlyRemovesOwnedGeneration(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore()
	store.Claim("Greatest", testAgentA, 3, testChar, testSessA)
	if _, ok := store.Apply(positionFixture(testSessA, 3, 1, now), now); !ok {
		t.Fatal("apply failed")
	}
	if removed := store.RemoveAgentGeneration(testAgentA, 2); len(removed) != 0 {
		t.Fatalf("wrong generation removed position: %#v", removed)
	}
	if len(store.Snapshot("Greatest")) != 1 {
		t.Fatal("wrong generation cleanup changed snapshot")
	}
	removed := store.RemoveAgentGeneration(testAgentA, 3)
	if len(removed) != 1 || removed[0].SessionID != testSessA {
		t.Fatalf("owned generation was not removed: %#v", removed)
	}
	if len(store.Snapshot("Greatest")) != 0 {
		t.Fatal("removed generation remains in snapshot")
	}
}

func TestCheckpointDueIsAtMostOncePerIntervalAndMarksBeforePersistence(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore()
	store.Claim("Greatest", testAgentA, 1, testChar, testSessA)
	first, ok := store.Apply(positionFixture(testSessA, 1, 1, now), now)
	if !ok {
		t.Fatal("apply failed")
	}
	if !store.CheckpointDue(first, now, time.Second) {
		t.Fatal("first checkpoint not admitted")
	}
	if store.CheckpointDue(first, now.Add(500*time.Millisecond), time.Second) {
		t.Fatal("checkpoint admitted before one second")
	}
	second, ok := store.Apply(positionFixture(testSessA, 1, 2, now.Add(900*time.Millisecond)), now.Add(900*time.Millisecond))
	if !ok {
		t.Fatal("second apply failed")
	}
	if store.CheckpointDue(second, now.Add(900*time.Millisecond), time.Second) {
		t.Fatal("newer coordinate bypassed checkpoint throttle")
	}
	if !store.CheckpointDue(second, now.Add(time.Second), time.Second) {
		t.Fatal("checkpoint not admitted after interval")
	}
}

func TestSnapshotIsServerFilteredAndLatestOnly(t *testing.T) {
	now := time.Now().UTC()
	store := NewStore()
	store.Claim("Greatest", testAgentA, 1, testChar, testSessA)
	for sequence := uint64(1); sequence <= 250; sequence++ {
		item := positionFixture(testSessA, 1, sequence, now)
		item.X = float64(sequence)
		if _, ok := store.Apply(item, now); !ok {
			t.Fatalf("sequence %d rejected", sequence)
		}
	}
	rows := store.Snapshot("GREATEST")
	if len(rows) != 1 || rows[0].Sequence != 250 || rows[0].X != 250 {
		t.Fatalf("store did not retain only latest position: %#v", rows)
	}
	if rows := store.Snapshot("Other"); len(rows) != 0 {
		t.Fatalf("server filter leaked rows: %#v", rows)
	}
}
