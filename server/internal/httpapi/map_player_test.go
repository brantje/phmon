package httpapi

import (
	"testing"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/players"
)

func TestProjectPlayersDedupesByPlayerID(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	z := 0.0
	snapshots := []players.LiveSnapshot{
		{
			Server: "Greatest", CharacterID: "char-b", SessionID: "session-b", Character: "Beta",
			Status: "observed", Region: 25000, ObservedAt: now, ObserverZ: &z,
			Players: []players.Player{{PlayerID: "7", Name: "Nearby", X: 10, Y: 20}},
		},
		{
			Server: "Greatest", CharacterID: "char-a", SessionID: "session-a", Character: "Alpha",
			Status: "observed", Region: 25000, ObservedAt: now.Add(time.Second), ObserverZ: &z,
			Players: []players.Player{{PlayerID: "7", Name: "Nearby", X: 11, Y: 21}},
		},
	}
	profile := mapprofile.Profile{
		Areas: []mapprofile.Area{{ID: "world", Kind: "outdoor", Floors: []mapprofile.Floor{{ID: "world"}}}},
		CoordinateTransforms: []mapprofile.CoordinateTransform{{
			AreaID: "world", FloorID: "world", Region: 25000, Status: "validated",
			WorldOriginX: 6400, WorldOriginY: 1000, TileOriginX: 30, TileOriginY: 40,
			UnitsPerTileX: 192, UnitsPerTileY: 192, AxisX: 1, AxisY: 1,
		}},
	}
	result := projectPlayers(profile, snapshots, "world", "world", 0, now.Add(2*time.Second))
	if result.Status != "observed" || len(result.Players) != 1 {
		t.Fatalf("unexpected projection: %+v", result)
	}
	if result.Players[0].X != 11 || len(result.Players[0].Observers) != 2 {
		t.Fatalf("expected freshest coordinates with merged observers: %+v", result.Players[0])
	}
}

func TestProjectPlayersWithholdsContradictoryRegion(t *testing.T) {
	now := time.Date(2026, 1, 1, 0, 0, 0, 0, time.UTC)
	z := 0.0
	region := 25001
	snapshots := []players.LiveSnapshot{{
		Server: "Greatest", CharacterID: "char-a", SessionID: "session-a", Character: "Alpha",
		Status: "observed", Region: 25000, ObservedAt: now, ObserverZ: &z,
		Players: []players.Player{{PlayerID: "7", Name: "Nearby", Region: &region, X: 10, Y: 20}},
	}}
	profile := mapprofile.Profile{
		Areas: []mapprofile.Area{{ID: "world", Kind: "outdoor", Floors: []mapprofile.Floor{{ID: "world"}}}},
		CoordinateTransforms: []mapprofile.CoordinateTransform{{
			AreaID: "world", FloorID: "world", Region: 25000, Status: "validated",
			WorldOriginX: 6400, WorldOriginY: 1000, TileOriginX: 30, TileOriginY: 40,
			UnitsPerTileX: 192, UnitsPerTileY: 192, AxisX: 1, AxisY: 1,
		}},
	}
	result := projectPlayers(profile, snapshots, "world", "world", 0, now)
	if len(result.Players) != 0 {
		t.Fatalf("expected contradictory region to be withheld")
	}
}
