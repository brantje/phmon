package positions

import (
	"math"
	"sort"
	"strings"
	"sync"
	"time"

	agentdomain "phmon/server/internal/agents"
)

const (
	MaxPastAge  = 2 * time.Minute
	MaxFutureSkew = 30 * time.Second
)

type Position struct {
	Server      string  `json:"-"`
	AgentID     string  `json:"-"`
	Generation  uint64  `json:"-"`
	CharacterID string  `json:"character_id"`
	SessionID   string  `json:"session_id"`
	Sequence    uint64  `json:"sequence"`
	Region      int     `json:"region"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Z           *float64 `json:"z,omitempty"`
	ObservedAt  time.Time `json:"observed_at"`
}

type Removal struct {
	Server      string `json:"-"`
	CharacterID string `json:"character_id"`
	SessionID   string `json:"session_id"`
}

type claim struct {
	Server      string
	AgentID     string
	Generation  uint64
	CharacterID string
	SessionID   string
}

type Store struct {
	mu             sync.RWMutex
	claims         map[string]claim
	items          map[string]Position
	lastCheckpoint map[string]time.Time
}

func NewStore() *Store {
	return &Store{
		claims:         make(map[string]claim),
		items:          make(map[string]Position),
		lastCheckpoint: make(map[string]time.Time),
	}
}

func ValidCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1_000_000 && value <= 1_000_000
}

func ValidRegion(region int) bool {
	return region != 0 && region >= -32768 && region <= 65535
}

func Validate(position Position, now time.Time) bool {
	server := strings.TrimSpace(position.Server)
	return server != "" && len(server) <= 100 &&
		agentdomain.ValidAgentID(position.AgentID) &&
		agentdomain.ValidAgentID(position.CharacterID) &&
		agentdomain.ValidAgentID(position.SessionID) &&
		position.Generation != 0 && position.Sequence != 0 &&
		ValidRegion(position.Region) && ValidCoordinate(position.X) && ValidCoordinate(position.Y) &&
		(position.Z == nil || ValidCoordinate(*position.Z)) &&
		!position.ObservedAt.IsZero() &&
		!position.ObservedAt.After(now.Add(MaxFutureSkew)) &&
		!position.ObservedAt.Before(now.Add(-MaxPastAge))
}

func (s *Store) Claim(server, agentID string, generation uint64, characterID, sessionID string) []Removal {
	if s == nil {
		return nil
	}
	server = strings.TrimSpace(server)
	if server == "" || len(server) > 100 || generation == 0 ||
		!agentdomain.ValidAgentID(agentID) || !agentdomain.ValidAgentID(characterID) || !agentdomain.ValidAgentID(sessionID) {
		return nil
	}
	next := claim{Server: server, AgentID: agentID, Generation: generation, CharacterID: characterID, SessionID: sessionID}
	s.mu.Lock()
	defer s.mu.Unlock()
	current, exists := s.claims[characterID]
	if exists && current == next {
		return nil
	}
	var removed []Removal
	if item, ok := s.items[characterID]; ok {
		removed = append(removed, Removal{Server: item.Server, CharacterID: item.CharacterID, SessionID: item.SessionID})
		delete(s.items, characterID)
	}
	if exists {
		delete(s.lastCheckpoint, current.SessionID)
	}
	s.claims[characterID] = next
	return removed
}

func (s *Store) Apply(position Position, now time.Time) (Position, bool) {
	if s == nil {
		return Position{}, false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, ok := s.claims[position.CharacterID]
	if !ok || owner.AgentID != position.AgentID || owner.Generation != position.Generation || owner.SessionID != position.SessionID {
		return Position{}, false
	}
	position.Server = owner.Server
	if !Validate(position, now) {
		return Position{}, false
	}
	if current, exists := s.items[position.CharacterID]; exists &&
		current.SessionID == position.SessionID && position.Sequence <= current.Sequence {
		return Position{}, false
	}
	position.ObservedAt = position.ObservedAt.UTC()
	s.items[position.CharacterID] = position
	return position, true
}

func (s *Store) Snapshot(server string) []Position {
	if s == nil {
		return []Position{}
	}
	server = strings.TrimSpace(server)
	s.mu.RLock()
	items := make([]Position, 0, len(s.items))
	for _, item := range s.items {
		if server == "" || strings.EqualFold(item.Server, server) {
			items = append(items, item)
		}
	}
	s.mu.RUnlock()
	sort.Slice(items, func(i, j int) bool {
		if items[i].CharacterID == items[j].CharacterID {
			return items[i].SessionID < items[j].SessionID
		}
		return items[i].CharacterID < items[j].CharacterID
	})
	return items
}

func (s *Store) RemoveSession(sessionID string) []Removal {
	if s == nil || sessionID == "" {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []Removal
	for characterID, owner := range s.claims {
		if owner.SessionID != sessionID {
			continue
		}
		if item, ok := s.items[characterID]; ok && item.SessionID == sessionID {
			removed = append(removed, Removal{Server: item.Server, CharacterID: item.CharacterID, SessionID: item.SessionID})
			delete(s.items, characterID)
		}
		delete(s.claims, characterID)
		delete(s.lastCheckpoint, sessionID)
	}
	return removed
}

func (s *Store) RemoveAgentGeneration(agentID string, generation uint64) []Removal {
	if s == nil || agentID == "" || generation == 0 {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	var removed []Removal
	for characterID, owner := range s.claims {
		if owner.AgentID != agentID || owner.Generation != generation {
			continue
		}
		if item, ok := s.items[characterID]; ok && item.SessionID == owner.SessionID {
			removed = append(removed, Removal{Server: item.Server, CharacterID: item.CharacterID, SessionID: item.SessionID})
			delete(s.items, characterID)
		}
		delete(s.claims, characterID)
		delete(s.lastCheckpoint, owner.SessionID)
	}
	return removed
}

func (s *Store) CheckpointDue(position Position, now time.Time, interval time.Duration) bool {
	if s == nil || interval <= 0 {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	owner, ok := s.claims[position.CharacterID]
	current, currentOK := s.items[position.CharacterID]
	if !ok || !currentOK || owner.SessionID != position.SessionID || owner.AgentID != position.AgentID ||
		owner.Generation != position.Generation || current.SessionID != position.SessionID || current.Sequence != position.Sequence {
		return false
	}
	if previous := s.lastCheckpoint[position.SessionID]; !previous.IsZero() && now.Sub(previous) < interval {
		return false
	}
	// Mark before persistence so a failing database does not cause a retry storm.
	s.lastCheckpoint[position.SessionID] = now
	return true
}
