package mobs

import (
	"strings"
	"sync"
	"time"
)

const LiveTTL = 35 * time.Second

type LiveSnapshot struct {
	Server      string    `json:"server"`
	AgentID     string    `json:"agent_id"`
	CharacterID string    `json:"character_id"`
	SessionID   string    `json:"session_id"`
	Character   string    `json:"character"`
	Status      string    `json:"status"`
	Region      int       `json:"region,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
	Truncated   bool      `json:"truncated,omitempty"`
	Monsters    []Monster `json:"monsters"`
}

// LiveStore is intentionally ephemeral. Historical samples are committed to
// PostgreSQL separately and can never repopulate this current-presence layer.
type LiveStore struct {
	mu    sync.RWMutex
	items map[string]LiveSnapshot
}

func NewLiveStore() *LiveStore { return &LiveStore{items: make(map[string]LiveSnapshot)} }

func (s *LiveStore) Apply(snapshot LiveSnapshot) {
	if s == nil {
		return
	}
	snapshot.Server = strings.TrimSpace(snapshot.Server)
	if snapshot.Status != "observed" && snapshot.Status != "unavailable" && snapshot.Status != "truncated" {
		return
	}
	if snapshot.Status == "unavailable" {
		snapshot.Monsters = []Monster{}
	}
	if snapshot.Monsters == nil {
		snapshot.Monsters = []Monster{}
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
