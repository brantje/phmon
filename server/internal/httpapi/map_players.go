package httpapi

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/players"
)

const (
	maxMapPlayers      = 256
	maxPlayerObservers = 16
)

type mapPlayerObserver struct {
	CharacterID string `json:"character_id"`
	SessionID   string `json:"session_id"`
	Name        string `json:"name"`
}

type mapPlayer struct {
	server              string
	RegistryID          string              `json:"registry_id,omitempty"`
	ID                  string              `json:"id"`
	PlayerID            string              `json:"player_id"`
	Name                string              `json:"name"`
	Guild               string              `json:"guild,omitempty"`
	Grant               string              `json:"grant,omitempty"`
	Dead                *bool               `json:"dead,omitempty"`
	Level               *int                `json:"level,omitempty"`
	Region              int                 `json:"region"`
	Zone                string              `json:"zone,omitempty"`
	X                   float64             `json:"x"`
	Y                   float64             `json:"y"`
	ObserverRegion      int                 `json:"observer_region"`
	ObserverZ           *float64            `json:"observer_z,omitempty"`
	ObservedAt          string              `json:"observed_at"`
	Observers           []mapPlayerObserver `json:"observers"`
	observedAt          time.Time
	observerCharacterID string
}

type mapPlayerSnapshot struct {
	Status    string      `json:"status"`
	Truncated bool        `json:"truncated,omitempty"`
	Players   []mapPlayer `json:"players"`
}

func projectPlayers(profile mapprofile.Profile, snapshots []players.LiveSnapshot, areaID, floorID string, regionFilter int, now time.Time) mapPlayerSnapshot {
	areaKind := ""
	for _, area := range profile.Areas {
		if area.ID == areaID {
			areaKind = area.Kind
			break
		}
	}
	byID := make(map[string]mapPlayer)
	anyObserved := false
	anyTruncated := false
	inScope := 0
	for _, snapshot := range snapshots {
		if !playersSnapshotInScope(profile, snapshot, areaKind, areaID, floorID) {
			continue
		}
		inScope++
		if snapshot.Status == "truncated" || snapshot.Truncated {
			anyTruncated = true
		}
		if snapshot.Status == "unavailable" {
			continue
		}
		if !players.SnapshotFresh(snapshot.ObservedAt, now) {
			continue
		}
		anyObserved = true
		for _, player := range snapshot.Players {
			placementRegion, ok := playerPlacementRegion(snapshot, player)
			if !ok || !playerRowInScope(profile, snapshot, placementRegion, areaKind, areaID, floorID, regionFilter) {
				continue
			}
			candidate := mapPlayer{
				server: snapshot.Server,
				ID:     player.PlayerID, PlayerID: player.PlayerID, Name: player.Name,
				Guild: player.Guild, Grant: player.Grant, Dead: player.Dead, Level: player.Level,
				Region: placementRegion, Zone: player.Zone, X: player.X, Y: player.Y,
				ObserverRegion: snapshot.Region, ObserverZ: snapshot.ObserverZ,
				ObservedAt: snapshot.ObservedAt.UTC().Format(time.RFC3339Nano), observedAt: snapshot.ObservedAt,
				observerCharacterID: snapshot.CharacterID,
				Observers: []mapPlayerObserver{{
					CharacterID: snapshot.CharacterID, SessionID: snapshot.SessionID, Name: snapshot.Character,
				}},
			}
			previous, exists := byID[player.PlayerID]
			if !exists {
				byID[player.PlayerID] = candidate
				continue
			}
			if candidate.observedAt.After(previous.observedAt) ||
				candidate.observedAt.Equal(previous.observedAt) && candidate.observerCharacterID < previous.observerCharacterID {
				if candidate.Zone == "" {
					candidate.Zone = previous.Zone
				}
				candidate.Observers = append(candidate.Observers, previous.Observers...)
				byID[player.PlayerID] = candidate
				continue
			}
			if previous.Zone == "" {
				previous.Zone = candidate.Zone
			}
			previous.Observers = append(previous.Observers, candidate.Observers...)
			byID[player.PlayerID] = previous
		}
	}
	rows := make([]mapPlayer, 0, len(byID))
	for _, row := range byID {
		if len(row.Observers) > maxPlayerObservers {
			row.Observers = row.Observers[:maxPlayerObservers]
		}
		rows = append(rows, row)
	}
	sort.Slice(rows, func(i, j int) bool {
		if rows[i].Name != rows[j].Name {
			return rows[i].Name < rows[j].Name
		}
		return rows[i].PlayerID < rows[j].PlayerID
	})
	truncated := anyTruncated
	if len(rows) > maxMapPlayers {
		rows = rows[:maxMapPlayers]
		truncated = true
	}
	status := "unavailable"
	if inScope > 0 && anyObserved {
		status = "observed"
	}
	if truncated {
		status = "truncated"
	}
	return mapPlayerSnapshot{Status: status, Truncated: truncated, Players: rows}
}

func playersSnapshotInScope(profile mapprofile.Profile, snapshot players.LiveSnapshot, areaKind, areaID, floorID string) bool {
	if snapshot.ObserverZ == nil {
		return false
	}
	region := snapshot.Region
	if areaKind == "cave" {
		area, floor, ok := mapprofile.ClassifyCave(profile, &region, snapshot.ObserverZ)
		return ok && area == areaID && floor == floorID
	}
	area, _, ok := mapprofile.ClassifyCave(profile, &region, snapshot.ObserverZ)
	return !(ok && area != "")
}

func playerPlacementRegion(snapshot players.LiveSnapshot, player players.Player) (int, bool) {
	if player.Region != nil {
		if !mobs.RegionsMatch(snapshot.Region, *player.Region) {
			return 0, false
		}
		return *player.Region, true
	}
	return snapshot.Region, true
}

func playerRowInScope(profile mapprofile.Profile, snapshot players.LiveSnapshot, placementRegion int, areaKind, areaID, floorID string, regionFilter int) bool {
	if regionFilter != 0 && !mobs.RegionsMatch(regionFilter, placementRegion) {
		return false
	}
	if areaKind != "cave" {
		return true
	}
	area, floor, ok := mapprofile.ClassifyCave(profile, &placementRegion, snapshot.ObserverZ)
	return ok && area == areaID && floor == floorID
}

func playerMarkerID(server string, player mapPlayer) string {
	return fmt.Sprintf("player:%s:%s:%d:%.1f:%.1f",
		strings.ToLower(strings.TrimSpace(server)), player.PlayerID, player.Region, player.X, player.Y)
}
