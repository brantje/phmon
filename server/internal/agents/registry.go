package agents

import (
	"context"
	"errors"
	"sync"
	"time"
)

var ErrConnectionUnavailable = errors.New("agent connection unavailable")

type CommandCapability struct {
	Name      string
	Supported bool
	Reason    string
}

type SendFunc func(context.Context, any) error

type activeSession struct {
	agentID      string
	generation   uint64
	connectedAt  time.Time
	protocol     int
	sender       SendFunc
	capabilities map[string]CommandCapability
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
		agentID:      agentID,
		generation:   r.next,
		connectedAt:  connectedAt,
		capabilities: make(map[string]CommandCapability),
	}
	r.latest[agentID] = connectedAt
	return r.next, connectedAt
}

func (r *Registry) Configure(agentID string, generation uint64, protocol int, sender SendFunc) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		return false
	}
	current.protocol = protocol
	current.sender = sender
	r.sessions[generation] = current
	return true
}

func (r *Registry) SetCapabilities(agentID string, generation uint64, capabilities []CommandCapability) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID || current.protocol < 3 {
		return false
	}
	next := make(map[string]CommandCapability, len(capabilities))
	for _, capability := range capabilities {
		next[capability.Name] = capability
	}
	current.capabilities = next
	r.sessions[generation] = current
	return true
}

func (r *Registry) CommandSupport(agentID string, generation uint64, commandName string) (bool, string) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		return false, "connection_unavailable"
	}
	if current.protocol < 3 {
		return false, "plugin_upgrade_required"
	}
	capability, ok := current.capabilities[commandName]
	if !ok {
		return false, "capabilities_pending"
	}
	if !capability.Supported {
		if capability.Reason == "" {
			return false, "unsupported_runtime_primitive"
		}
		return false, capability.Reason
	}
	return true, ""
}

func (r *Registry) Send(ctx context.Context, agentID string, generation uint64, payload any) error {
	r.mu.RLock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID || current.sender == nil {
		r.mu.RUnlock()
		return ErrConnectionUnavailable
	}
	sender := current.sender
	r.mu.RUnlock()
	return sender(ctx, payload)
}

func (r *Registry) Protocol(agentID string, generation uint64) (int, bool) {
	r.mu.RLock()
	defer r.mu.RUnlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		return 0, false
	}
	return current.protocol, true
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

func (r *Registry) HasGeneration(agentID string, generation uint64) bool {
	return r.IsCurrent(agentID, generation)
}
