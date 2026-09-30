package npcs

import (
	"strings"
	"testing"
	"time"
)

func TestNPCLiveStoreReplacesExpiresAndRemoves(t *testing.T) {
	store := NewLiveStore()
	now := time.Now().UTC()
	model := int64(2094)
	npc := NPC{ID: "10", Name: "Jangan", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 1, Y: 2}
	store.Apply(LiveSnapshot{Server: "Greatest", AgentID: "agent-a", CharacterID: "char-a", SessionID: "session-a",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now, NPCs: []NPC{npc}})
	if got := store.Snapshot("greatest", now); len(got) != 1 || len(got[0].NPCs) != 1 || got[0].NPCs[0].Name != "Jangan" {
		t.Fatalf("npc snapshot was not stored: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "Greatest", AgentID: "agent-a", CharacterID: "char-a", SessionID: "session-a",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now.Add(time.Second), NPCs: []NPC{}})
	if got := store.Snapshot("greatest", now.Add(time.Second)); len(got) != 1 || len(got[0].NPCs) != 0 || got[0].Status != "observed" {
		t.Fatalf("replacement snapshot did not clear npcs: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "Greatest", AgentID: "agent-a", CharacterID: "char-a", SessionID: "session-a",
		Character: "Alpha", Status: "unavailable", Region: 25000, ObservedAt: now.Add(2 * time.Second), NPCs: []NPC{npc}})
	if got := store.Snapshot("greatest", now.Add(2*time.Second)); len(got) != 1 || got[0].Status != "unavailable" || len(got[0].NPCs) != 0 {
		t.Fatalf("unavailable snapshot kept npc rows: %+v", got)
	}
	store.RemoveSession("session-a")
	if got := store.Snapshot("greatest", now.Add(3*time.Second)); len(got) != 0 {
		t.Fatalf("removed session remained: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "Greatest", AgentID: "agent-a", CharacterID: "char-a", SessionID: "session-b",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now, NPCs: []NPC{npc}})
	store.RemoveAgent("agent-a")
	if got := store.Snapshot("greatest", now); len(got) != 0 {
		t.Fatalf("removed agent remained: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "Greatest", AgentID: "agent-a", CharacterID: "char-a", SessionID: "session-c",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now, NPCs: []NPC{npc}})
	if got := store.Snapshot("greatest", now.Add(LiveTTL+time.Second)); len(got) != 0 {
		t.Fatalf("expired npc snapshot remained: %+v", got)
	}
}

func TestValidateNPCSnapshotRejectsBadRolesAndDuplicates(t *testing.T) {
	now := time.Now().UTC()
	model := int64(2094)
	gate := NPC{ID: "10", Name: "Jangan", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 1, Y: 2}
	if err := ValidateLiveSnapshot("observed", 25000, []NPC{gate}, now, now); err != nil {
		t.Fatal(err)
	}
	gate.Role = "npc"
	if err := ValidateLiveSnapshot("observed", 25000, []NPC{gate}, now, now); err == nil {
		t.Fatal("gate classified as npc was accepted")
	}
	shop := NPC{ID: "273", Name: "Herbalist", ServerName: "NPC_CH_POTION", Role: "npc", Region: 25000, X: 3, Y: 4}
	if err := ValidateLiveSnapshot("observed", 25000, []NPC{shop, shop}, now, now); err == nil {
		t.Fatal("duplicate npc id was accepted")
	}
	shop.Name = strings.Repeat("n", 65)
	if err := ValidateLiveSnapshot("observed", 25000, []NPC{shop}, now, now); err == nil {
		t.Fatal("oversized npc name was accepted")
	}
	if err := ValidateLiveSnapshot("unavailable", 25000, []NPC{{ID: "1", Role: "npc", Region: 25000, X: 1, Y: 2}}, now, now); err == nil {
		t.Fatal("unavailable snapshot with rows was accepted")
	}
}
