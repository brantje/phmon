package players

import (
	"testing"
	"time"
)

func TestPlayerLiveStoreReplacesExpiresAndRemovesGeneration(t *testing.T) {
	store := NewLiveStore()
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	store.Apply(LiveSnapshot{
		Server: "Greatest", AgentID: "agent-a", Generation: 1,
		CharacterID: "char-a", SessionID: "session-a", Character: "Alpha",
		Status: "observed", Region: 25000, ObservedAt: now,
		ObserverZ: floatPtr(0), Players: []Player{{PlayerID: "7", Name: "Nearby", X: 1, Y: 2}},
	})
	store.Apply(LiveSnapshot{
		Server: "Greatest", AgentID: "agent-a", Generation: 2,
		CharacterID: "char-b", SessionID: "session-b", Character: "Beta",
		Status: "observed", Region: 25000, ObservedAt: now,
		ObserverZ: floatPtr(0), Players: []Player{{PlayerID: "8", Name: "Other", X: 3, Y: 4}},
	})
	if len(store.Snapshot("Greatest", now)) != 2 {
		t.Fatalf("expected two live snapshots")
	}
	store.RemoveAgentGeneration("agent-a", 1)
	if len(store.Snapshot("Greatest", now)) != 1 {
		t.Fatalf("expected generation-scoped removal")
	}
	if got := store.Snapshot("Greatest", now.Add(LiveTTL+time.Second)); len(got) != 0 {
		t.Fatalf("expected ttl expiry")
	}
}

func TestVisibleNamesBecomePlayerNamesAndJobAliases(t *testing.T) {
	now := time.Date(2026, 10, 8, 7, 12, 12, 0, time.UTC)
	got := LiveObservations(LiveSnapshot{
		Server: "Greatest", Status: "observed", Region: 24223, ObservedAt: now, Generation: 4,
		Players: []Player{
			{PlayerID: "1", Name: "NoobTrader", X: 1, Y: 2},
			{PlayerID: "2", Name: "Pito_Pequna", X: 3, Y: 4},
			{PlayerID: "3", Name: "MrTrade", X: 5, Y: 6},
			{PlayerID: "4", Name: "SwiftHunter", X: 7, Y: 8},
			{PlayerID: "5", Name: "DarkThief", X: 9, Y: 10},
			{PlayerID: "6", Name: "Trader", X: 11, Y: 12},
		},
	})
	if len(got) != 6 {
		t.Fatalf("observations: %d", len(got))
	}
	assertJob := func(index int, name, job string) {
		t.Helper()
		if got[index].Name != name || got[index].NameType != "job" || got[index].Job == nil || *got[index].Job != job {
			t.Fatalf("%s classified as %s/%v", name, got[index].NameType, got[index].Job)
		}
	}
	assertJob(0, "NoobTrader", "trader")
	assertJob(3, "SwiftHunter", "hunter")
	assertJob(4, "DarkThief", "thief")
	for _, index := range []int{1, 2, 5} {
		if got[index].NameType != "normal" || got[index].Job != nil {
			t.Fatalf("%s should stay a player name", got[index].Name)
		}
	}
}

func TestValidateLiveSnapshotAcceptsZoneAndRejectsUntrimmedZone(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	valid := ValidateLiveSnapshot("observed", 25000, []Player{{
		PlayerID: "7", Name: "Nearby", X: 1, Y: 2, Zone: "Taklamakan",
	}}, now, now)
	if valid != nil {
		t.Fatal(valid)
	}
	if err := ValidateLiveSnapshot("observed", 25000, []Player{{
		PlayerID: "7", Name: "Nearby", X: 1, Y: 2, Zone: " Taklamakan",
	}}, now, now); err == nil {
		t.Fatal("expected untrimmed zone to be rejected")
	}
}

func floatPtr(value float64) *float64 { return &value }
