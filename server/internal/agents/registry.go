package agents

import (
	"context"
	"sync"
	"time"
)

type activeSession struct {
	generation  uint64
	connectedAt time.Time
	cancel      context.CancelFunc
}

type Registry struct {
	mu       sync.RWMutex
	next     uint64
	sessions map[string]activeSession
}

func NewRegistry() *Registry {
	return &Registry{sessions: make(map[string]activeSession)}
}

func (r *Registry) Register(agentID string, cancel context.CancelFunc) (generation uint64, connectedAt time.Time, previous context.CancelFunc) {
	r.mu.Lock()
	defer r.mu.Unlock()
	r.next++
	connectedAt = time.Now().UTC()
	if current, ok := r.sessions[agentID]; ok {
		previous = current.cancel
	}
	r.sessions[agentID] = activeSession{
		generation:  r.next,
		connectedAt: connectedAt,
		cancel:      cancel,
	}
	return r.next, connectedAt, previous
}

func (r *Registry) Unregister(agentID string, generation uint64) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[agentID]
	if !ok || current.generation != generation {
		return false
	}
	delete(r.sessions, agentID)
	return true
}

func (r *Registry) ConnectedAt(agentID string) (time.Time, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	current, ok := r.sessions[agentID]
	if !ok {
		return time.Time{}, false
	}
	return current.connectedAt, true
}
