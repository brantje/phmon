package resources

import (
	"context"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"
)

const MaxPartyObservers = 64

type PartyMember struct {
	PartyID   string
	PlayerID  uint64
	Name      string
	Guild     string
	Level     *int
	HPPercent *int
	MPPercent *int
	X         *float64
	Y         *float64
}

type PartyObservation struct {
	ObserverCharacterID string
	ObserverName        string
	SessionID           string
	Availability        string
	ObserverRegion      *int
	ObserverZ           *float64
	StateUpdatedAt      *time.Time
	CheckedAt           time.Time
	Members             []PartyMember
}

func (s *Store) CurrentPartyObservations(ctx context.Context, server string) ([]PartyObservation, bool, error) {
	if s == nil || s.pool == nil {
		return nil, false, errors.New("resource store unavailable")
	}
	server = strings.TrimSpace(server)
	if server == "" || len(server) > 100 {
		return nil, false, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, `SELECT o.observer_character_id::text,c.character_name,o.session_id::text,o.availability,o.payload,o.updated_at,c.region,c.z,c.state_updated_at
FROM character_resource_observations o
JOIN character_resource_state rs ON rs.character_id=o.observer_character_id AND rs.session_id=o.session_id
JOIN character_sessions cs ON cs.character_id=o.observer_character_id AND cs.session_id=o.session_id AND cs.ended_at IS NULL
JOIN characters c ON c.character_id=o.observer_character_id
WHERE o.resource_key='party' AND lower(o.server_key)=lower($1)
ORDER BY o.updated_at DESC,o.observer_character_id
LIMIT $2`, server, MaxPartyObservers+1)
	if err != nil {
		return nil, false, err
	}
	defer rows.Close()
	result := make([]PartyObservation, 0, MaxPartyObservers)
	truncated := false
	for rows.Next() {
		if len(result) >= MaxPartyObservers {
			truncated = true
			break
		}
		var observation PartyObservation
		var payload []byte
		if err := rows.Scan(
			&observation.ObserverCharacterID,
			&observation.ObserverName,
			&observation.SessionID,
			&observation.Availability,
			&payload,
			&observation.CheckedAt,
			&observation.ObserverRegion,
			&observation.ObserverZ,
			&observation.StateUpdatedAt,
		); err != nil {
			return nil, false, err
		}
		observation.Members = []PartyMember{}
		if observation.Availability == "observed" {
			observation.Members = decodePartyMembers(payload)
		}
		result = append(result, observation)
	}
	if err := rows.Err(); err != nil {
		return nil, false, err
	}
	return result, truncated, nil
}

func decodePartyMembers(payload []byte) []PartyMember {
	var object struct {
		Members []json.RawMessage `json:"members"`
	}
	if len(payload) == 0 || json.Unmarshal(payload, &object) != nil {
		return []PartyMember{}
	}
	members := make([]PartyMember, 0, min(len(object.Members), 32))
	for _, raw := range object.Members {
		if len(members) >= 32 {
			break
		}
		var fields map[string]json.RawMessage
		if json.Unmarshal(raw, &fields) != nil || fields == nil {
			continue
		}
		member := PartyMember{}
		if value, ok := decodeBoundedString(fields["party_id"], 64); ok {
			member.PartyID = value
		}
		if value, ok := decodeUint(fields["player_id"]); ok {
			member.PlayerID = value
		}
		if value, ok := decodeBoundedString(fields["name"], 100); ok {
			member.Name = value
		}
		if value, ok := decodeBoundedString(fields["guild"], 100); ok {
			member.Guild = value
		}
		if value, ok := decodeBoundedInt(fields["level"], 0, 255); ok {
			member.Level = &value
		}
		if value, ok := decodeBoundedInt(fields["hp_percent"], 0, 100); ok {
			member.HPPercent = &value
		}
		if value, ok := decodeBoundedInt(fields["mp_percent"], 0, 100); ok {
			member.MPPercent = &value
		}
		if value, ok := decodePartyCoordinate(fields["x"]); ok {
			member.X = &value
		}
		if value, ok := decodePartyCoordinate(fields["y"]); ok {
			member.Y = &value
		}
		members = append(members, member)
	}
	return members
}

func decodeBoundedString(raw json.RawMessage, limit int) (string, bool) {
	if len(raw) == 0 || string(raw) == "null" {
		return "", false
	}
	var value string
	if json.Unmarshal(raw, &value) != nil {
		return "", false
	}
	value = strings.TrimSpace(value)
	if value == "" || len(value) > limit || strings.ContainsRune(value, 0) {
		return "", false
	}
	return value, true
}

func decodeUint(raw json.RawMessage) (uint64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	value, err := strconv.ParseUint(string(raw), 10, 64)
	return value, err == nil
}

func decodeBoundedInt(raw json.RawMessage, minimum, maximum int) (int, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	value64, err := strconv.ParseInt(string(raw), 10, 64)
	if err != nil || value64 < int64(minimum) || value64 > int64(maximum) {
		return 0, false
	}
	return int(value64), true
}

func decodePartyCoordinate(raw json.RawMessage) (float64, bool) {
	if len(raw) == 0 {
		return 0, false
	}
	value, err := strconv.ParseFloat(string(raw), 64)
	if err != nil || math.IsNaN(value) || math.IsInf(value, 0) || math.Abs(value) > 1000000 {
		return 0, false
	}
	return value, true
}
