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

func floatPtr(value float64) *float64 { return &value }
