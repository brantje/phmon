package httpapi

import (
	"math"
	"time"

	"phmon/server/internal/commands"
	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
)

const maxMapTrainingAreas = 256
const maxMapTrainingRadius = 10000

type mapTrainingArea struct {
	CharacterID string   `json:"character_id"`
	SessionID   string   `json:"session_id"`
	Name        string   `json:"name"`
	Region      int      `json:"region"`
	Zone        string   `json:"zone,omitempty"`
	X           float64  `json:"x"`
	Y           float64  `json:"y"`
	Z           *float64 `json:"z,omitempty"`
	Radius      float64  `json:"radius"`
	ObservedAt  string   `json:"observed_at,omitempty"`
}

type mapTrainingAreasSnapshot struct {
	Status    string            `json:"status"`
	Truncated bool              `json:"truncated,omitempty"`
	Areas     []mapTrainingArea `json:"areas"`
}

// projectTrainingAreas scopes observed training areas by their own region and
// Z, never by the character's current position. Rows are already ordered.
func projectTrainingAreas(profile mapprofile.Profile, observations []commands.TrainingAreaObservation, sourceTruncated bool, areaID, floorID string, regionFilter int) mapTrainingAreasSnapshot {
	areaKind := ""
	for _, area := range profile.Areas {
		if area.ID == areaID {
			areaKind = area.Kind
			break
		}
	}
	areas := make([]mapTrainingArea, 0, len(observations))
	for _, observation := range observations {
		state := observation.State
		if !state.TrainingAvailable || state.TrainingRegion == nil || state.TrainingX == nil || state.TrainingY == nil || state.TrainingRadius == nil {
			continue
		}
		region, x, y, radius := *state.TrainingRegion, *state.TrainingX, *state.TrainingY, *state.TrainingRadius
		if region == 0 || !finite(x) || !finite(y) || !finite(radius) || radius < 1 || radius > maxMapTrainingRadius {
			continue
		}
		z := state.TrainingZ
		if z != nil && !finite(*z) {
			z = nil
		}
		if areaKind == "cave" {
			area, floor, ok := mapprofile.ClassifyCave(profile, &region, z)
			if !ok || area != areaID || floor != floorID {
				continue
			}
		} else if caveRegion(profile, region) {
			continue
		}
		if regionFilter != 0 && !mobs.RegionsMatch(regionFilter, region) {
			continue
		}
		projected := mapTrainingArea{
			CharacterID: observation.CharacterID, SessionID: state.SessionID, Name: observation.CharacterName,
			Region: region, X: x, Y: y, Z: z, Radius: radius,
		}
		if state.TrainingZone != nil {
			projected.Zone = *state.TrainingZone
		}
		if state.ObservedAt != nil {
			projected.ObservedAt = state.ObservedAt.UTC().Format(time.RFC3339Nano)
		}
		areas = append(areas, projected)
	}
	truncated := sourceTruncated
	if len(areas) > maxMapTrainingAreas {
		areas = areas[:maxMapTrainingAreas]
		truncated = true
	}
	status := "observed"
	if truncated {
		status = "truncated"
	}
	return mapTrainingAreasSnapshot{Status: status, Truncated: truncated, Areas: areas}
}

// caveRegion reports whether any cave floor, including manually selected
// floors, claims the region. Such areas never render on the outdoor map.
func caveRegion(profile mapprofile.Profile, region int) bool {
	for _, entry := range profile.CaveFloors {
		for _, candidate := range entry.Floor.RegionIDs {
			if mobs.RegionsMatch(candidate, region) {
				return true
			}
		}
	}
	return false
}

func finite(value float64) bool { return !math.IsNaN(value) && !math.IsInf(value, 0) }
