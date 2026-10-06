package httpapi

import (
	"context"
	"log/slog"
	"strings"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/tradenexus"
)

type mapThief struct {
	ID             string   `json:"id"`
	SightingID     string   `json:"sighting_id"`
	Name           string   `json:"name"`
	Region         int      `json:"region"`
	X              float64  `json:"x"`
	Y              float64  `json:"y"`
	Z              *float64 `json:"z,omitempty"`
	PositionSource string   `json:"position_source"`
	ReporterName   string   `json:"reporter_name,omitempty"`
	ReporterApp    string   `json:"reporter_app,omitempty"`
	Origin         string   `json:"origin"`
	ObservedAt     string   `json:"observed_at"`
}

type mapThiefSnapshot struct {
	Status    string     `json:"status"`
	Truncated bool       `json:"truncated,omitempty"`
	Thieves   []mapThief `json:"thieves"`
}

func (h *LiveHub) mapThiefSnapshot(ctx context.Context, profile mapprofile.Profile, server, areaID, floorID string, regionFilter int, areaKind string) mapThiefSnapshot {
	empty := mapThiefSnapshot{Status: "unavailable", Thieves: []mapThief{}}
	if h == nil || h.thiefSightings == nil {
		return empty
	}
	queryCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	rows, truncated, err := h.thiefSightings.Recent(queryCtx, server, time.Now().UTC().Add(-tradenexus.MarkerTTL), tradenexus.MaxRecentSightings)
	if err != nil {
		slog.Warn("map thief sightings unavailable", "server", server, "reason", err.Error())
		return empty
	}
	return projectThiefSightings(profile, areaKind, areaID, floorID, regionFilter, server, rows, truncated)
}

func projectThiefSightings(profile mapprofile.Profile, areaKind, areaID, floorID string, regionFilter int, server string, rows []tradenexus.Sighting, truncated bool) mapThiefSnapshot {
	thieves := make([]mapThief, 0, len(rows))
	for _, row := range rows {
		if row.Position == nil {
			continue
		}
		if !thiefInScope(profile, areaKind, areaID, floorID, row.Position.Region, row.Position.Z, regionFilter) {
			continue
		}
		thieves = append(thieves, mapThief{
			ID: thiefMarkerID(server, row.ThiefName), SightingID: row.ID, Name: row.ThiefName,
			Region: row.Position.Region, X: row.Position.X, Y: row.Position.Y, Z: row.Position.Z,
			PositionSource: row.PositionSource, ReporterName: row.Reporter.Name, ReporterApp: row.Reporter.App,
			Origin: row.Origin, ObservedAt: row.ObservedAt.UTC().Format(time.RFC3339Nano),
		})
	}
	status := "observed"
	if truncated {
		status = "truncated"
	}
	return mapThiefSnapshot{Status: status, Truncated: truncated, Thieves: thieves}
}

func thiefInScope(profile mapprofile.Profile, areaKind, areaID, floorID string, region int, z *float64, regionFilter int) bool {
	if regionFilter != 0 && !mobs.RegionsMatch(regionFilter, region) {
		return false
	}
	if areaKind == "cave" {
		if z == nil {
			return false
		}
		area, floor, ok := mapprofile.ClassifyCave(profile, &region, z)
		return ok && area == areaID && floor == floorID
	}
	if z != nil {
		area, _, ok := mapprofile.ClassifyCave(profile, &region, z)
		if ok && area != "" {
			return false
		}
	}
	return !thiefCaveRegion(profile, region)
}

func thiefCaveRegion(profile mapprofile.Profile, region int) bool {
	for _, entry := range profile.CaveFloors {
		for _, candidate := range entry.Floor.RegionIDs {
			if mobs.RegionsMatch(candidate, region) {
				return true
			}
		}
	}
	return false
}

func thiefMarkerID(server, name string) string {
	return "thief:" + strings.ToLower(strings.TrimSpace(server)) + ":" + strings.ToLower(strings.TrimSpace(name))
}
