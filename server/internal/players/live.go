package players

import (
	"math"
	"strings"
	"sync"
	"time"

	"phmon/server/internal/mobs"
)

const (
	LiveTTL        = 35 * time.Second
	LiveFutureSkew = 5 * time.Second
	MaxPlayers     = 128
	maxText        = 64
	maxLevel       = 255
)

type Player struct {
	PlayerID string  `json:"player_id"`
	Name     string  `json:"name"`
	Guild    string  `json:"guild,omitempty"`
	Grant    string  `json:"grant,omitempty"`
	Dead     *bool   `json:"dead,omitempty"`
	Level    *int    `json:"level,omitempty"`
	Region   *int    `json:"region,omitempty"`
	X        float64 `json:"x"`
	Y        float64 `json:"y"`
}

type LiveSnapshot struct {
	Server      string    `json:"server"`
	AgentID     string    `json:"agent_id"`
	Generation  uint64    `json:"generation"`
	CharacterID string    `json:"character_id"`
	SessionID   string    `json:"session_id"`
	Character   string    `json:"character"`
	Status      string    `json:"status"`
	Region      int       `json:"region,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
	ObserverZ   *float64  `json:"observer_z,omitempty"`
	Truncated   bool      `json:"truncated,omitempty"`
	Players     []Player  `json:"players"`
}

// LiveStore keeps the current nearby-player observation for each character session.
type LiveStore struct {
	mu    sync.RWMutex
	items map[string]LiveSnapshot
}

func NewLiveStore() *LiveStore { return &LiveStore{items: make(map[string]LiveSnapshot)} }

func ValidateLiveSnapshot(status string, region int, rows []Player, now, observedAt time.Time) error {
	if status != "observed" && status != "unavailable" && status != "truncated" || !mobs.ValidRegion(region) ||
		observedAt.IsZero() || observedAt.After(now.Add(LiveFutureSkew)) ||
		observedAt.Before(now.Add(-LiveTTL)) || len(rows) > MaxPlayers {
		return mobs.ErrInvalidSample
	}
	if status == "unavailable" && len(rows) != 0 {
		return mobs.ErrInvalidSample
	}
	seen := make(map[string]struct{}, len(rows))
	for _, player := range rows {
		if !validPlayer(region, player) {
			return mobs.ErrInvalidSample
		}
		if _, ok := seen[player.PlayerID]; ok {
			return mobs.ErrInvalidSample
		}
		seen[player.PlayerID] = struct{}{}
	}
	return nil
}

func validPlayer(observerRegion int, player Player) bool {
	if player.PlayerID == "" || len(player.PlayerID) > maxText || strings.ContainsRune(player.PlayerID, 0) ||
		player.Name == "" || len(player.Name) > maxText || strings.ContainsRune(player.Name, 0) ||
		len(player.Guild) > maxText || strings.ContainsRune(player.Guild, 0) ||
		len(player.Grant) > maxText || strings.ContainsRune(player.Grant, 0) ||
		!validCoordinate(player.X) || !validCoordinate(player.Y) {
		return false
	}
	if player.Region != nil {
		if !mobs.ValidRegion(*player.Region) || !mobs.RegionsMatch(observerRegion, *player.Region) {
			return false
		}
	}
	if player.Level != nil && (*player.Level < 1 || *player.Level > maxLevel) {
		return false
	}
	return true
}

func validCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1_000_000 && value <= 1_000_000
}

func (s *LiveStore) Apply(snapshot LiveSnapshot) {
	if s == nil {
		return
	}
	snapshot.Server = strings.TrimSpace(snapshot.Server)
	if snapshot.Status != "observed" && snapshot.Status != "unavailable" && snapshot.Status != "truncated" {
		return
	}
	if snapshot.Status == "unavailable" {
		snapshot.Players = []Player{}
	}
	if snapshot.Players == nil {
		snapshot.Players = []Player{}
	}
	s.mu.Lock()
	s.items[snapshot.SessionID] = snapshot
	s.mu.Unlock()
}

func SnapshotFresh(observedAt, now time.Time) bool {
	if observedAt.IsZero() || now.IsZero() {
		return false
	}
	age := now.Sub(observedAt.UTC())
	return age >= -LiveFutureSkew && age <= LiveTTL
}

func (s *LiveStore) Snapshot(server string, now time.Time) []LiveSnapshot {
	if s == nil {
		return []LiveSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]LiveSnapshot, 0)
	for key, item := range s.items {
		if !SnapshotFresh(item.ObservedAt, now) {
			delete(s.items, key)
			continue
		}
		if server != "" && !strings.EqualFold(item.Server, server) {
			continue
		}
		items = append(items, item)
	}
	return items
}

func (s *LiveStore) RemoveSession(sessionID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	delete(s.items, sessionID)
	s.mu.Unlock()
}

func (s *LiveStore) RemoveAgentGeneration(agentID string, generation uint64) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for key, item := range s.items {
		if item.AgentID == agentID && item.Generation == generation {
			delete(s.items, key)
		}
	}
	s.mu.Unlock()
}
