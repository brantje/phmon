package agents

import (
	"context"
	"errors"
	"strconv"
	"strings"
	"sync"
	"time"
)

var (
	ErrConnectionUnavailable = errors.New("agent connection unavailable")
	ErrNotSent               = errors.New("agent command was not sent")
)

type CommandCapability struct {
	Name      string
	Supported bool
	Reason    string
	Modes     []string
}

func (r *Registry) CommandModeSupport(agentID string, generation uint64, name, mode string) (bool, string) {
	supported, reason := r.CommandSupport(agentID, generation, name)
	if !supported {
		return false, reason
	}
	r.mu.RLock()
	defer r.mu.RUnlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		return false, "connection_unavailable"
	}
	capability, ok := current.capabilities[name]
	if !ok {
		return false, "capabilities_pending"
	}
	if len(capability.Modes) == 0 {
		if name == "training.area.set" || name == "chat.send" || name == "character.reverse_return" {
			return false, "capability_modes_missing"
		}
		return true, ""
	}
	for _, allowed := range capability.Modes {
		if allowed == mode {
			return true, ""
		}
	}
	return false, "unsupported_argument_mode"
}

type SendFunc func(context.Context, any) error

type activeSession struct {
	agentID       string
	generation    uint64
	connectedAt   time.Time
	protocol      int
	pluginVersion string
	sender        SendFunc
	capabilities  map[string]CommandCapability
}

type Registry struct {
	mu       sync.RWMutex
	next     uint64
	sessions map[uint64]activeSession
	latest   map[string]time.Time
	revoking map[string]struct{}
}

func NewRegistry() *Registry {
	return &Registry{
		sessions: make(map[uint64]activeSession),
		latest:   make(map[string]time.Time),
		revoking: make(map[string]struct{}),
	}
}

func (r *Registry) Register(agentID string) (generation uint64, connectedAt time.Time) {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, blocked := r.revoking[agentID]; blocked {
		return 0, time.Time{}
	}
	r.next++
	connectedAt = time.Now().UTC()
	if previous, ok := r.latest[agentID]; ok && !connectedAt.After(previous) {
		connectedAt = previous.Add(time.Nanosecond)
	}
	r.sessions[r.next] = activeSession{
		agentID:      agentID,
		generation:   r.next,
		connectedAt:  connectedAt,
		capabilities: make(map[string]CommandCapability),
	}
	r.latest[agentID] = connectedAt
	return r.next, connectedAt
}

func (r *Registry) Configure(agentID string, generation uint64, protocol int, pluginVersion string, sender SendFunc) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		return false
	}
	current.protocol = protocol
	current.pluginVersion = pluginVersion
	current.sender = sender
	r.sessions[generation] = current
	return true
}

func (r *Registry) ProtocolVersion(agentID string, generation uint64) int {
	r.mu.RLock()
	defer r.mu.RUnlock()
	current, ok := r.sessions[generation]
	if !ok || current.agentID != agentID {
		return 0
	}
	return current.protocol
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
	if commandName == "character.walk" && !pluginVersionAtLeast(current.pluginVersion, 1, 1, 2) {
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

func pluginVersionAtLeast(version string, major, minor, patch int) bool {
	version = strings.TrimSpace(version)
	if version == "" || strings.Contains(version, "-") {
		return false
	}
	version = strings.SplitN(version, "+", 2)[0]
	parts := strings.Split(version, ".")
	if len(parts) != 3 {
		return false
	}
	got := [3]int{}
	for i, part := range parts {
		value, err := strconv.Atoi(part)
		if err != nil || value < 0 {
			return false
		}
		got[i] = value
	}
	want := [3]int{major, minor, patch}
	for i := range got {
		if got[i] != want[i] {
			return got[i] > want[i]
		}
	}
	return true
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

// BeginCredentialRevocation reserves an offline logical agent against new live
// registrations while its credential is being revoked. The caller must always pair
// a successful reservation with EndCredentialRevocation.
func (r *Registry) BeginCredentialRevocation(agentID string) bool {
	r.mu.Lock()
	defer r.mu.Unlock()
	if _, exists := r.revoking[agentID]; exists || r.hasConnectionsLocked(agentID) {
		return false
	}
	r.revoking[agentID] = struct{}{}
	return true
}

func (r *Registry) EndCredentialRevocation(agentID string) {
	r.mu.Lock()
	defer r.mu.Unlock()
	delete(r.revoking, agentID)
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
