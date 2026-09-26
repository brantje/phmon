package agents

import (
	"sync"
	"time"
)

type activeSession struct {
	agentID     string
	generation  uint64
	connectedAt time.Time
}

type Registry struct {
	mu       sync.RWMutex
	next     uint64
	sessions map[uint64]activeSession
	latest   map[string]time.Time
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[uint64]activeSession), latest: make(map[string]time.Time)}
}

func (r *Registry) Register(agentID string) (generation uint64, connectedAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	connectedAt = time.Now().UTC()
	r.sessions[r.next] = activeSession{
		agentID:     agentID,
		generation:  r.next,
		connectedAt: connectedAt,
	}
	r.latest[agentID] = connectedAt
	return r.next, connectedAt
}

// LatestConnectedAt is retained after disconnect so persistence can fence an
// older socket's close using the newest generation for this logical agent.
func (r *Registry) LatestConnectedAt(agentID string) (time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	at, ok := r.latest[agentID]
	return at, ok
}

// Unregister removes one connection generation and reports whether any
// connections for the agent remain active.
func (r *Registry) Unregister(agentID string, generation uint64) (removed bool, stillConnected bool) {
	removed, stillConnected, _ = r.UnregisterWithFence(agentID, generation)
	return removed, stillConnected
}

// UnregisterWithFence atomically removes one generation and captures the most
// recent connect time in its logical-agent cohort when it was the last socket.
// Persisting that value lets storage reject a disconnect if a newer connection
// was durably recorded before cleanup reaches PostgreSQL.
func (r *Registry) UnregisterWithFence(agentID string, generation uint64) (removed bool, stillConnected bool, disconnectFence time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		stillConnected = r.hasConnectionsLocked(agentID)
		return false, stillConnected, time.Time{}
	}
	delete(r.sessions, generation)
	stillConnected = r.hasConnectionsLocked(agentID)
	if !stillConnected {
		disconnectFence = r.latest[agentID]
	}
	return true, stillConnected, disconnectFence
}

func (r *Registry) ConnectedAt(agentID string) (time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	var connectedAt time.Time
	for _, current := range r.sessions {
		if current.agentID == agentID && (connectedAt.IsZero() || current.connectedAt.Before(connectedAt)) {
			connectedAt = current.connectedAt
		}
	}
	return connectedAt, !connectedAt.IsZero()
}

func (r *Registry) ConnectionCount(agentID string) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	count := 0
	for _, current := range r.sessions {
		if current.agentID == agentID {
			count++
		}
	}
	return count
}

func (r *Registry) hasConnectionsLocked(agentID string) bool {
	for _, current := range r.sessions {
		if current.agentID == agentID {
			return true
		}
	}
	return false
}

func (r *Registry) IsCurrent(agentID string, generation uint64) bool {
	r.mu.RLock()
	defer r.mu.RUnlock()
	current, ok := r.sessions[generation]
	return ok && current.agentID == agentID && current.generation == generation
}

// HasGeneration reports whether this exact authenticated socket is still live.
// Character session reconciliation uses it to close only durable sessions whose
// owning connection has disappeared, even when sibling sockets share the agent.
func (r *Registry) HasGeneration(agentID string, generation uint64) bool {
	return r.IsCurrent(agentID, generation)
}
