package httpapi

import (
	"testing"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/tradenexus"
)

func TestProjectThiefSightingsScopesCavesAndKeepsStableIDs(t *testing.T) {
	profile, err := mapprofile.ForServer("Greatest", mapprofile.GreatestDatasetID)
	if err != nil {
		t.Fatal(err)
	}
	z := -9.0
	rows := []tradenexus.Sighting{
		{ID: "00000000-0000-4000-8000-000000000001", Server: "Greatest", ThiefName: "Bandit",
			Position: &tradenexus.Position{Region: 25735, X: 10, Y: 20}, PositionSource: "thief",
			Origin: "external", ObservedAt: time.Date(2026, 10, 6, 19, 0, 0, 0, time.UTC)},
		{ID: "00000000-0000-4000-8000-000000000002", Server: "Greatest", ThiefName: "Cave",
			Position: &tradenexus.Position{Region: -32767, X: 1, Y: 2}, PositionSource: "thief", Origin: "phmon"},
		{ID: "00000000-0000-4000-8000-000000000003", Server: "Greatest", ThiefName: "Donwhang",
			Position: &tradenexus.Position{Region: -32767, X: 3, Y: 4, Z: &z}, PositionSource: "observer", Origin: "phmon"},
	}
	outdoor := projectThiefSightings(profile, "outdoor", "world", "world", 0, "Greatest", rows, false)
	if outdoor.Status != "observed" || len(outdoor.Thieves) != 1 || outdoor.Thieves[0].Name != "Bandit" {
		t.Fatalf("outdoor thieves: %+v", outdoor.Thieves)
	}
	if outdoor.Thieves[0].ID != "thief:greatest:bandit" {
		t.Fatalf("marker id = %s", outdoor.Thieves[0].ID)
	}
	cave := projectThiefSightings(profile, "cave", "donwhang-stone-cave", "1F", 0, "Greatest", rows, false)
	if len(cave.Thieves) != 1 || cave.Thieves[0].Name != "Donwhang" {
		t.Fatalf("cave thieves: %+v", cave.Thieves)
	}
	filtered := projectThiefSightings(profile, "outdoor", "world", "world", 25000, "Greatest", rows, true)
	if filtered.Status != "truncated" || len(filtered.Thieves) != 0 {
		t.Fatalf("region filter: %+v", filtered)
	}
}
