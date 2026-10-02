package httpapi

import (
	"testing"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/npcs"
)

func TestProjectNPCsDedupesAcrossSessionsAndKeepsSameSessionRows(t *testing.T) {
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	model := int64(2094)
	older := npcs.LiveSnapshot{
		Server: "Greatest", CharacterID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000011",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now.Add(-time.Second),
		NPCs: []npcs.NPC{{
			ID: "10", Name: "Old Jangan", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 100, Y: 200,
		}},
	}
	newer := npcs.LiveSnapshot{
		Server: "greatest", CharacterID: "00000000-0000-4000-8000-000000000002", SessionID: "00000000-0000-4000-8000-000000000012",
		Character: "Beta", Status: "observed", Region: 25000, ObservedAt: now,
		NPCs: []npcs.NPC{
			{ID: "99", Name: "Jangan", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 104, Y: 203},
			{ID: "100", Name: "Second gate", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 106, Y: 204},
		},
	}
	got := projectNPCs(profile, []npcs.LiveSnapshot{older, newer}, "world", "world", 0)
	if got.Status != "observed" || len(got.NPCs) != 2 {
		t.Fatalf("npc projection = %+v", got)
	}
	var merged mapNPC
	for _, npc := range got.NPCs {
		if npc.Name == "Jangan" || npc.Name == "Old Jangan" {
			merged = npc
		}
	}
	if merged.Name != "Jangan" || len(merged.Observers) != 2 || merged.X != 104 {
		t.Fatalf("freshest cross-session npc was not kept: %+v", got.NPCs)
	}
	if merged.ID != "npc:greatest:25000:GATE_CH:2094:10:100.0:200.0" {
		t.Fatalf("cross-session merge replaced the cluster id: %s", merged.ID)
	}
}

func TestProjectNPCsKeepsObserverTeleportRoutesSeparate(t *testing.T) {
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	model := int64(2094)
	jangan := int64(7)
	alpha := npcs.LiveSnapshot{
		Server: "Greatest", CharacterID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000011",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now.Add(-time.Second),
		NPCs: []npcs.NPC{{
			ID: "10", Name: "Hotan", ServerName: "GATE_KT", Model: &model, Role: "teleporter", Region: 25000, X: 100, Y: 200,
			TeleportRoutes: []npcs.TeleportRoute{{Destination: "Jangan", TeleportCode: &jangan}},
		}},
	}
	beta := npcs.LiveSnapshot{
		Server: "greatest", CharacterID: "00000000-0000-4000-8000-000000000002", SessionID: "00000000-0000-4000-8000-000000000012",
		Character: "Bravo", Status: "observed", Region: 25000, ObservedAt: now,
		NPCs: []npcs.NPC{{
			ID: "10", Name: "Hotan", ServerName: "GATE_KT", Model: &model, Role: "teleporter", Region: 25000, X: 101, Y: 201,
		}},
	}
	got := projectNPCs(profile, []npcs.LiveSnapshot{alpha, beta}, "world", "world", 0)
	if len(got.NPCs) != 1 || len(got.NPCs[0].Observers) != 2 {
		t.Fatalf("shared gate projection = %+v", got.NPCs)
	}
	if len(got.NPCs[0].TeleportRoutes) != 1 || got.NPCs[0].TeleportRoutes[0].Destination != "Jangan" {
		t.Fatalf("menu routes = %+v", got.NPCs[0].TeleportRoutes)
	}
	var alphaRoutes, bravoRoutes []mapTeleportRoute
	for _, observer := range got.NPCs[0].Observers {
		switch observer.Name {
		case "Alpha":
			alphaRoutes = observer.TeleportRoutes
		case "Bravo":
			bravoRoutes = observer.TeleportRoutes
		}
	}
	if len(alphaRoutes) != 1 || alphaRoutes[0].Destination != "Jangan" {
		t.Fatalf("alpha routes = %+v", alphaRoutes)
	}
	if len(bravoRoutes) != 0 {
		t.Fatalf("bravo inherited alpha routes: %+v", bravoRoutes)
	}
}

func TestProjectNPCsKeepsDistinctSameSessionMarkerIDs(t *testing.T) {
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	model := int64(2094)
	now := time.Now().UTC()
	snapshot := npcs.LiveSnapshot{
		Server: "Greatest", CharacterID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000011",
		Character: "Alpha", Status: "observed", Region: 25000, ObservedAt: now,
		NPCs: []npcs.NPC{
			{ID: "10", Name: "Gate A", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 100.04, Y: 200.04},
			{ID: "11", Name: "Gate B", ServerName: "GATE_CH", Model: &model, Role: "teleporter", Region: 25000, X: 100.01, Y: 200.01},
		},
	}
	got := projectNPCs(profile, []npcs.LiveSnapshot{snapshot}, "world", "world", 0)
	if len(got.NPCs) != 2 {
		t.Fatalf("same-session npcs = %+v", got.NPCs)
	}
	want := map[string]bool{
		"npc:greatest:25000:GATE_CH:2094:10:100.0:200.0": false,
		"npc:greatest:25000:GATE_CH:2094:11:100.0:200.0": false,
	}
	for _, npc := range got.NPCs {
		if _, ok := want[npc.ID]; !ok {
			t.Fatalf("unexpected marker id %s in %+v", npc.ID, got.NPCs)
		}
		want[npc.ID] = true
	}
	for id, seen := range want {
		if !seen {
			t.Fatalf("missing marker id %s in %+v", id, got.NPCs)
		}
	}
}

func TestProjectNPCsFailsClosedWithoutCaveZ(t *testing.T) {
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	now := time.Now().UTC()
	z := -9.0
	snapshots := []npcs.LiveSnapshot{
		{
			Server: "Greatest", CharacterID: "00000000-0000-4000-8000-000000000001", SessionID: "00000000-0000-4000-8000-000000000011",
			Character: "Valid", Status: "observed", Region: -32767, ObserverZ: &z, ObservedAt: now,
			NPCs: []npcs.NPC{{ID: "1", Name: "Floor NPC", ServerName: "NPC_CAVE", Role: "npc", Region: -32767, X: -24294, Y: -91}},
		},
		{
			Server: "Greatest", CharacterID: "00000000-0000-4000-8000-000000000002", SessionID: "00000000-0000-4000-8000-000000000012",
			Character: "NoZ", Status: "observed", Region: -32767, ObservedAt: now,
			NPCs: []npcs.NPC{{ID: "2", Name: "Unproven", ServerName: "NPC_CAVE", Role: "npc", Region: -32767, X: -24294, Y: -91}},
		},
	}
	got := projectNPCs(profile, snapshots, "donwhang-stone-cave", "1F", 0)
	if len(got.NPCs) != 1 || got.NPCs[0].Name != "Floor NPC" {
		t.Fatalf("unproven cave npc was projected: %+v", got)
	}
	outdoor := projectNPCs(profile, snapshots[:1], "world", "world", 0)
	if len(outdoor.NPCs) != 0 {
		t.Fatalf("cave npc leaked onto the outdoor map: %+v", outdoor)
	}
}
