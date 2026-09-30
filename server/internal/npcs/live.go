package npcs

import (
	"math"
	"regexp"
	"strings"
	"sync"
	"time"

	"phmon/server/internal/mobs"
)

const (
	LiveTTL = 35 * time.Second
	MaxNPCs = 128
	maxText = 64
)

var gateRole = regexp.MustCompile(`^GATE_[A-Za-z0-9_]+$`)

type NPC struct {
	ID         string  `json:"id"`
	Name       string  `json:"name,omitempty"`
	ServerName string  `json:"servername,omitempty"`
	Model      *int64  `json:"model_id,omitempty"`
	Role       string  `json:"role"`
	Region     int     `json:"region"`
	X          float64 `json:"x"`
	Y          float64 `json:"y"`
}

type LiveSnapshot struct {
	Server      string    `json:"server"`
	AgentID     string    `json:"agent_id"`
	CharacterID string    `json:"character_id"`
	SessionID   string    `json:"session_id"`
	Character   string    `json:"character"`
	Status      string    `json:"status"`
	Region      int       `json:"region,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
	ObserverZ   *float64  `json:"observer_z,omitempty"`
	Truncated   bool      `json:"truncated,omitempty"`
	NPCs        []NPC     `json:"npcs"`
}

// LiveStore keeps the current NPC observation for each character session.
// It has no database and cannot be rebuilt from history.
type LiveStore struct {
	mu    sync.RWMutex
	items map[string]LiveSnapshot
}

func NewLiveStore() *LiveStore { return &LiveStore{items: make(map[string]LiveSnapshot)} }

func ExpectedRole(serverName string) string {
	if gateRole.MatchString(serverName) {
		return "teleporter"
	}
	return "npc"
}

func ValidateLiveSnapshot(status string, region int, rows []NPC, now, observedAt time.Time) error {
	if status != "observed" && status != "unavailable" && status != "truncated" || !mobs.ValidRegion(region) ||
		observedAt.IsZero() || observedAt.After(now.Add(2*time.Minute)) || observedAt.Before(now.Add(-2*time.Minute)) ||
		len(rows) > MaxNPCs {
		return mobs.ErrInvalidSample
	}
	if status == "unavailable" && len(rows) != 0 {
		return mobs.ErrInvalidSample
	}
	seen := make(map[string]struct{}, len(rows))
	for _, npc := range rows {
		if !validNPC(region, npc) {
			return mobs.ErrInvalidSample
		}
		if _, ok := seen[npc.ID]; ok {
			return mobs.ErrInvalidSample
		}
		seen[npc.ID] = struct{}{}
	}
	return nil
}

func validNPC(observerRegion int, npc NPC) bool {
	if npc.ID == "" || len(npc.ID) > maxText || strings.ContainsRune(npc.ID, 0) ||
		len(npc.Name) > maxText || strings.ContainsRune(npc.Name, 0) ||
		len(npc.ServerName) > maxText || strings.ContainsRune(npc.ServerName, 0) ||
		npc.Role != ExpectedRole(npc.ServerName) || !mobs.ValidRegion(npc.Region) ||
		!mobs.RegionsMatch(observerRegion, npc.Region) || !validCoordinate(npc.X) || !validCoordinate(npc.Y) {
		return false
	}
	return npc.Model == nil || *npc.Model >= 0 && *npc.Model <= 4294967295
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
		snapshot.NPCs = []NPC{}
	}
	if snapshot.NPCs == nil {
		snapshot.NPCs = []NPC{}
	}
	s.mu.Lock()
	s.items[snapshot.SessionID] = snapshot
	s.mu.Unlock()
}

func (s *LiveStore) Snapshot(server string, now time.Time) []LiveSnapshot {
	if s == nil {
		return []LiveSnapshot{}
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	items := make([]LiveSnapshot, 0)
	for key, item := range s.items {
		if now.Sub(item.ObservedAt) > LiveTTL || item.ObservedAt.After(now.Add(2*time.Minute)) {
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

func (s *LiveStore) RemoveAgent(agentID string) {
	if s == nil {
		return
	}
	s.mu.Lock()
	for key, item := range s.items {
		if item.AgentID == agentID {
			delete(s.items, key)
		}
	}
	s.mu.Unlock()
}
