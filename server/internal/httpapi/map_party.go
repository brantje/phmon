package httpapi

import (
	"fmt"
	"sort"
	"strings"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/resources"
)

const maxMapPartyMembers = 256
const partyObserverStateMaxAge = 35 * time.Second
const partyObserverStateFutureSkew = 5 * time.Second

type mapPartyMember struct {
	ID                  string   `json:"id"`
	PartyID             string   `json:"party_id,omitempty"`
	PlayerID            uint64   `json:"player_id"`
	Name                string   `json:"name,omitempty"`
	Guild               string   `json:"guild,omitempty"`
	Level               *int     `json:"level,omitempty"`
	HPPercent           *int     `json:"hp_percent,omitempty"`
	MPPercent           *int     `json:"mp_percent,omitempty"`
	X                   float64  `json:"x"`
	Y                   float64  `json:"y"`
	ObserverCharacterID string   `json:"observer_character_id"`
	ObserverName        string   `json:"observer_name"`
	ObserverSessionID   string   `json:"observer_session_id"`
	ObserverRegion      int      `json:"observer_region"`
	ObserverZ           *float64 `json:"observer_z,omitempty"`
	ObservedAt          string   `json:"observed_at"`
	checkedAt           time.Time
}

type mapPartySnapshot struct {
	Status    string           `json:"status"`
	Truncated bool             `json:"truncated,omitempty"`
	Members   []mapPartyMember `json:"members"`
}

func projectPartyMembers(profile mapprofile.Profile, observations []resources.PartyObservation, sourceTruncated bool, areaID, floorID string, regionFilter int, now time.Time) mapPartySnapshot {
	areaKind := ""
	for _, area := range profile.Areas {
		if area.ID == areaID {
			areaKind = area.Kind
			break
		}
	}
	anyObserved := false
	byIdentity := make(map[string]mapPartyMember)
	for _, observation := range observations {
		if observation.Availability != "observed" || !partyObserverStateFresh(observation.StateUpdatedAt, now) {
			continue
		}
		anyObserved = true
		if observation.ObserverRegion == nil {
			continue
		}
		if areaKind == "cave" {
			area, floor, ok := mapprofile.ClassifyCave(profile, observation.ObserverRegion, observation.ObserverZ)
			if !ok || area != areaID || floor != floorID {
				continue
			}
		} else if area, _, ok := mapprofile.ClassifyCave(profile, observation.ObserverRegion, observation.ObserverZ); ok && area != "" {
			continue
		}
		if regionFilter != 0 && !mobs.RegionsMatch(regionFilter, *observation.ObserverRegion) {
			continue
		}
		for _, member := range observation.Members {
			if member.PlayerID == 0 || member.X == nil || member.Y == nil {
				continue
			}
			identity := partyMemberIdentity(member)
			if identity == "" {
				continue
			}
			candidate := mapPartyMember{
				ID: identity, PartyID: member.PartyID, PlayerID: member.PlayerID, Name: member.Name, Guild: member.Guild,
				Level: member.Level, HPPercent: member.HPPercent, MPPercent: member.MPPercent,
				X: *member.X, Y: *member.Y,
				ObserverCharacterID: observation.ObserverCharacterID, ObserverName: observation.ObserverName,
				ObserverSessionID: observation.SessionID, ObserverRegion: *observation.ObserverRegion, ObserverZ: observation.ObserverZ,
				ObservedAt: observation.CheckedAt.UTC().Format(time.RFC3339Nano), checkedAt: observation.CheckedAt,
			}
			previous, exists := byIdentity[identity]
			if !exists || candidate.checkedAt.After(previous.checkedAt) ||
				candidate.checkedAt.Equal(previous.checkedAt) && candidate.ObserverCharacterID < previous.ObserverCharacterID {
				byIdentity[identity] = candidate
			}
		}
	}
	members := make([]mapPartyMember, 0, len(byIdentity))
	for _, member := range byIdentity {
		members = append(members, member)
	}
	sort.Slice(members, func(i, j int) bool {
		if !members[i].checkedAt.Equal(members[j].checkedAt) {
			return members[i].checkedAt.After(members[j].checkedAt)
		}
		return members[i].ID < members[j].ID
	})
	truncated := sourceTruncated
	if len(members) > maxMapPartyMembers {
		members = members[:maxMapPartyMembers]
		truncated = true
	}
	status := "unavailable"
	if anyObserved {
		status = "observed"
	}
	if truncated {
		status = "truncated"
	}
	return mapPartySnapshot{Status: status, Truncated: truncated, Members: members}
}

func partyMemberIdentity(member resources.PartyMember) string {
	name := strings.ToLower(strings.TrimSpace(member.Name))
	if member.PartyID != "" && name != "" {
		return "party:" + member.PartyID + ":" + name
	}
	if member.PlayerID > 0 {
		return fmt.Sprintf("player:%d", member.PlayerID)
	}
	if name != "" {
		return "name:" + name
	}
	return ""
}

func partyObserverStateFresh(updatedAt *time.Time, now time.Time) bool {
	if updatedAt == nil || now.IsZero() {
		return false
	}
	age := now.Sub(updatedAt.UTC())
	return age >= -partyObserverStateFutureSkew && age <= partyObserverStateMaxAge
}
