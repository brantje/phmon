package httpapi

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/characters"
)

const (
	agentProtocolVersion    = 3
	agentMinProtocolVersion = 2
)

type AgentOptions struct {
	HelloTimeout      time.Duration
	HeartbeatInterval time.Duration
	HeartbeatTimeout  time.Duration
}

func (o AgentOptions) withDefaults() AgentOptions {
	if o.HelloTimeout <= 0 {
		o.HelloTimeout = 5 * time.Second
	}
	if o.HeartbeatInterval <= 0 {
		o.HeartbeatInterval = 10 * time.Second
	}
	if o.HeartbeatTimeout <= 0 {
		o.HeartbeatTimeout = 30 * time.Second
	}
	return o
}

type agentHandler struct {
	store      AgentStore
	registry   *agentdomain.Registry
	options    AgentOptions
	characters *characters.Store
	live       *LiveHub
}

type agentCapability struct {
	Name      string `json:"name"`
	Supported bool   `json:"supported"`
	Reason    string `json:"reason,omitempty"`
}

type agentMessage struct {
	Type            string           `json:"type"`
	ProtocolVersion int              `json:"protocol_version"`
	AgentID         string           `json:"agent_id,omitempty"`
	PluginVersion   string           `json:"plugin_version,omitempty"`
	PhBotVersion    string           `json:"phbot_version,omitempty"`
	SentAt          string           `json:"sent_at,omitempty"`
	CharacterID     string           `json:"character_id,omitempty"`
	Server          string           `json:"server,omitempty"`
	Name            string           `json:"name,omitempty"`
	Guild           *string          `json:"guild,omitempty"`
	State           characters.State `json:"state,omitempty"`
	SessionID       string           `json:"session_id,omitempty"`
	SchemaVersion   int              `json:"schema_version,omitempty"`
	Commands        []agentCapability `json:"commands,omitempty"`
}

type helloAck struct {
	Type                     string `json:"type"`
	ProtocolVersion          int    `json:"protocol_version"`
	HeartbeatIntervalSeconds int    `json:"heartbeat_interval_seconds"`
	HeartbeatTimeoutSeconds  int    `json:"heartbeat_timeout_seconds"`
}

type AgentCredentialView struct {
	AgentID    string `json:"agent_id"`
	AgentToken string `json:"agent_token"`
}

type AgentView struct {
	AgentID            string     `json:"agent_id"`
	Connected          bool       `json:"connected"`
	ActiveConnections  int        `json:"active_connections"`
	ConnectedAt        *time.Time `json:"connected_at,omitempty"`
	FirstSeenAt        *time.Time `json:"first_seen_at,omitempty"`
	LastSeenAt         *time.Time `json:"last_seen_at,omitempty"`
	LastConnectedAt    *time.Time `json:"last_connected_at,omitempty"`
	LastDisconnectedAt *time.Time `json:"last_disconnected_at,omitempty"`
	ProtocolVersion    *int       `json:"protocol_version,omitempty"`
	PluginVersion      *string    `json:"plugin_version,omitempty"`
	PhBotVersion       *string    `json:"phbot_version,omitempty"`
}

func (h *agentHandler) connect(w http.ResponseWriter, r *http.Request) {
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}

	authCtx, authCancel := context.WithTimeout(r.Context(), 2*time.Second)
	authenticatedID, err := h.store.AuthenticateToken(authCtx, token)
	authCancel()
	if errors.Is(err, agentdomain.ErrInvalidToken) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode: websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(8192)
	defer conn.Close(websocket.StatusNormalClosure, "")

	helloCtx, helloCancel := context.WithTimeout(r.Context(), h.options.HelloTimeout)
	var hello agentMessage
	err = wsjson.Read(helloCtx, conn, &hello)
	helloCancel()
	if err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "hello required")
		return
	}
	if hello.Type != "hello" {
		_ = conn.Close(websocket.StatusPolicyViolation, "hello required")
		return
	}
	if hello.ProtocolVersion < agentMinProtocolVersion || hello.ProtocolVersion > agentProtocolVersion {
		_ = conn.Close(websocket.StatusUnsupportedData, "unsupported protocol version")
		return
	}
	if hello.AgentID != authenticatedID || !agentdomain.ValidAgentID(hello.AgentID) {
		_ = conn.Close(websocket.StatusPolicyViolation, "agent identity mismatch")
		return
	}
	if len(hello.PluginVersion) == 0 || len(hello.PluginVersion) > 64 || len(hello.PhBotVersion) == 0 || len(hello.PhBotVersion) > 64 {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid version metadata")
		return
	}
	if _, err := time.Parse(time.RFC3339, hello.SentAt); err != nil {
		_ = conn.Close(websocket.StatusPolicyViolation, "invalid timestamp")
		return
	}

	connectedAt := time.Now().UTC()
	updateCtx, updateCancel := context.WithTimeout(r.Context(), 2*time.Second)
	err = h.store.MarkConnected(updateCtx, hello.AgentID, connectedAt, hello.ProtocolVersion, hello.PluginVersion, hello.PhBotVersion)
	updateCancel()
	if err != nil {
		_ = conn.Close(websocket.StatusInternalError, "state unavailable")
		return
	}

	sessionCtx, sessionCancel := context.WithCancel(r.Context())
	generation, _ := h.registry.Register(hello.AgentID)
	writer := newAgentWriter(sessionCtx, conn, sessionCancel)
	if !h.registry.Configure(hello.AgentID, generation, hello.ProtocolVersion, writer.Send) {
		sessionCancel()
		return
	}
	h.live.Invalidate()
	defer func() {
		sessionCancel()
		removed, stillConnected, disconnectFence := h.registry.UnregisterWithFence(hello.AgentID, generation)
		if !removed {
			return
		}
		h.live.Invalidate()
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		if h.characters != nil {
			if err := h.characters.EndAgent(ctx, hello.AgentID, generation); err != nil {
				slog.Warn("failed to close character sessions", "agent_id", hello.AgentID)
			}
		}
		if !stillConnected {
			if disconnectFence.IsZero() {
				disconnectFence = connectedAt
			}
			if err := h.store.MarkDisconnected(ctx, hello.AgentID, disconnectFence); err != nil {
				slog.Warn("failed to persist agent disconnect", "agent_id", hello.AgentID)
			}
		}
		// Publish again after durable session/disconnect cleanup. The earlier
		// invalidation exposes the in-memory socket transition promptly; this one
		// guarantees the replacement snapshot observes committed cleanup.
		h.live.Invalidate()
	}()

	ack := helloAck{
		Type:                     "hello.ack",
		ProtocolVersion:          hello.ProtocolVersion,
		HeartbeatIntervalSeconds: durationSeconds(h.options.HeartbeatInterval),
		HeartbeatTimeoutSeconds:  durationSeconds(h.options.HeartbeatTimeout),
	}
	writeCtx, writeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
	err = writer.Send(writeCtx, ack)
	writeCancel()
	if err != nil {
		return
	}

	for {
		readCtx, readCancel := context.WithTimeout(sessionCtx, h.options.HeartbeatTimeout)
		var message agentMessage
		err := wsjson.Read(readCtx, conn, &message)
		readCancel()
		if err != nil {
			return
		}
		if !h.registry.IsCurrent(hello.AgentID, generation) {
			return
		}
		if message.ProtocolVersion != hello.ProtocolVersion {
			_ = conn.Close(websocket.StatusUnsupportedData, "unsupported protocol version")
			return
		}
		switch message.Type {
		case "heartbeat":
			if _, err := time.Parse(time.RFC3339, message.SentAt); err != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid timestamp")
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			err = h.store.MarkSeen(ctx, hello.AgentID)
			cancel()
			if err != nil {
				slog.Warn("failed to persist agent heartbeat", "agent_id", hello.AgentID)
			} else {
				h.live.Invalidate()
			}
		case "agent.capabilities":
			if hello.ProtocolVersion < 3 || message.SchemaVersion != 1 || len(message.Commands) == 0 || len(message.Commands) > 32 {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid capability report")
				return
			}
			capabilities := make([]agentdomain.CommandCapability, 0, len(message.Commands))
			seen := make(map[string]struct{}, len(message.Commands))
			for _, capability := range message.Commands {
				if !validReportedCommandName(capability.Name) || len(capability.Reason) > 64 {
					_ = conn.Close(websocket.StatusPolicyViolation, "invalid capability report")
					return
				}
				if _, exists := seen[capability.Name]; exists {
					_ = conn.Close(websocket.StatusPolicyViolation, "duplicate capability")
					return
				}
				seen[capability.Name] = struct{}{}
				capabilities = append(capabilities, agentdomain.CommandCapability{
					Name: capability.Name, Supported: capability.Supported, Reason: capability.Reason,
				})
			}
			if !h.registry.SetCapabilities(hello.AgentID, generation, capabilities) {
				return
			}
			h.live.Invalidate()
		case "character.identify":
			if h.characters == nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "character state unavailable")
				return
			}
			if !validMessageTime(message.SentAt) {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid timestamp")
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			id, e := h.characters.Resolve(ctx, characters.Identity{Server: message.Server, Name: message.Name, Guild: message.Guild})
			var sessionID string
			if e == nil {
				sessionID, e = h.characters.ClaimSessionID(ctx, hello.AgentID, id, generation)
			}
			cancel()
			if e != nil {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid character identity")
				return
			}
			h.live.Invalidate()
			reply := map[string]any{"type": "character.registered", "protocol_version": hello.ProtocolVersion, "character_id": id}
			if hello.ProtocolVersion >= 3 {
				reply["session_id"] = sessionID
			}
			if e = writer.Send(sessionCtx, reply); e != nil {
				return
			}
		case "character.snapshot", "character.state":
			if !agentdomain.ValidAgentID(message.CharacterID) || !validWireState(message.State) || h.characters == nil ||
				(hello.ProtocolVersion >= 3 && !agentdomain.ValidAgentID(message.SessionID)) {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid character state")
				return
			}
			if !validMessageTime(message.SentAt) {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid timestamp")
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			var e error
			if message.Type == "character.snapshot" {
				if hello.ProtocolVersion >= 3 {
					e = h.characters.SnapshotSession(ctx, hello.AgentID, message.CharacterID, generation, message.SessionID, message.State)
				} else {
					e = h.characters.Snapshot(ctx, hello.AgentID, message.CharacterID, generation, message.State)
				}
			} else {
				if hello.ProtocolVersion >= 3 {
					e = h.characters.UpdateSession(ctx, hello.AgentID, message.CharacterID, generation, message.SessionID, message.State)
				} else {
					e = h.characters.Update(ctx, hello.AgentID, message.CharacterID, generation, message.State)
				}
			}
			cancel()
			if e != nil {
				slog.Warn("character state update rejected", "agent_id", hello.AgentID, "message_type", message.Type, "reason", e.Error())
				if errors.Is(e, characters.ErrNotFound) {
					if !writeCharacterRejected(sessionCtx, writer, hello.ProtocolVersion, message.CharacterID, message.SessionID) {
						return
					}
					continue
				}
				_ = conn.Close(websocket.StatusInternalError, "character state unavailable")
				return
			}
			h.live.Invalidate()
		case "character.left":
			if !agentdomain.ValidAgentID(message.CharacterID) || h.characters == nil ||
				(hello.ProtocolVersion >= 3 && !agentdomain.ValidAgentID(message.SessionID)) {
				_ = conn.Close(websocket.StatusPolicyViolation, "character_id required")
				return
			}
			if !validMessageTime(message.SentAt) {
				_ = conn.Close(websocket.StatusPolicyViolation, "invalid timestamp")
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			var e error
			if hello.ProtocolVersion >= 3 {
				e = h.characters.EndSession(ctx, hello.AgentID, message.CharacterID, generation, message.SessionID, "left")
			} else {
				e = h.characters.End(ctx, hello.AgentID, message.CharacterID, generation, "left")
			}
			cancel()
			if e != nil {
				if errors.Is(e, characters.ErrNotFound) {
					if !writeCharacterRejected(sessionCtx, writer, hello.ProtocolVersion, message.CharacterID, message.SessionID) {
						return
					}
					continue
				}
				_ = conn.Close(websocket.StatusInternalError, "character state unavailable")
				return
			}
			h.live.Invalidate()
		default:
			_ = conn.Close(websocket.StatusPolicyViolation, "unexpected agent message")
			return
		}
	}
}

func writeCharacterRejected(ctx context.Context, writer *agentWriter, protocol int, characterID, sessionID string) bool {
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	payload := map[string]any{
		"type":             "character.rejected",
		"protocol_version": protocol,
		"character_id":     characterID,
		"reason":           "not_current_session",
	}
	if protocol >= 3 && sessionID != "" {
		payload["session_id"] = sessionID
	}
	return writer.Send(writeCtx, payload) == nil
}

func validReportedCommandName(name string) bool {
	switch name {
	case "bot.start", "bot.stop", "trace.start", "trace.stop",
		"training.area.set", "training.radius.set", "character.walk",
		"character.return", "character.disconnect", "client.clientless":
		return true
	default:
		return false
	}
}

func (h *agentHandler) createCredential(w http.ResponseWriter, r *http.Request) {
	credential, err := agentdomain.NewCredential()
	if err != nil {
		respondJSON(w, http.StatusInternalServerError, map[string]string{"error": "cannot create credential"})
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	if err := h.store.CreateCredential(ctx, credential); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}

	respondJSON(w, http.StatusCreated, AgentCredentialView{
		AgentID:    credential.AgentID,
		AgentToken: credential.Token,
	})
}

func (h *agentHandler) list(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	views, err := loadAgentViews(ctx, h.store, h.registry)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, views)
}

func loadAgentViews(ctx context.Context, store AgentStore, registry *agentdomain.Registry) ([]AgentView, error) {
	records, err := store.ListSeen(ctx)
	if err != nil {
		return nil, err
	}
	views := make([]AgentView, 0, len(records))
	for _, record := range records {
		view := AgentView{
			AgentID:            record.AgentID,
			ActiveConnections:  registry.ConnectionCount(record.AgentID),
			FirstSeenAt:        record.FirstSeenAt,
			LastSeenAt:         record.LastSeenAt,
			LastConnectedAt:    record.LastConnectedAt,
			LastDisconnectedAt: record.LastDisconnectedAt,
			ProtocolVersion:    record.ProtocolVersion,
			PluginVersion:      record.PluginVersion,
			PhBotVersion:       record.PhBotVersion,
		}
		if connectedAt, ok := registry.ConnectedAt(record.AgentID); ok {
			view.Connected = true
			view.ConnectedAt = &connectedAt
		}
		views = append(views, view)
	}
	return views, nil
}

func bearerToken(header string) (string, bool) {
	if !strings.HasPrefix(header, "Bearer ") {
		return "", false
	}
	token := strings.TrimSpace(strings.TrimPrefix(header, "Bearer "))
	if token == "" || strings.ContainsAny(token, " \t\r\n") || len(token) > 256 {
		return "", false
	}
	return token, true
}

func durationSeconds(value time.Duration) int {
	seconds := int(value / time.Second)
	if seconds < 1 {
		return 1
	}
	return seconds
}
