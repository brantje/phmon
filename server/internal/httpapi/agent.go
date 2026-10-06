package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"log/slog"
	"net/http"
	"strconv"
	"strings"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/commands"
	"phmon/server/internal/events"
	"phmon/server/internal/mapanalytics"
	"phmon/server/internal/mobs"
	"phmon/server/internal/navigation"
	"phmon/server/internal/npcs"
	"phmon/server/internal/players"
	"phmon/server/internal/positions"
	"phmon/server/internal/resources"
)

const (
	agentProtocolVersion    = 17
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
	commands   *commands.Service
	resources  *resources.Store
	events     *events.Store
	mobs       *mobs.Store
	mobLive    *mobs.LiveStore
	npcLive    *npcs.LiveStore
	playerLive *players.LiveStore
	positions  *positions.Store
	analytics  *mapanalytics.Store
	navigation *navigation.Store
}

type agentMonsterSnapshot struct {
	Status      string           `json:"status"`
	CharacterID string           `json:"character_id"`
	SessionID   string           `json:"session_id"`
	ObservedAt  time.Time        `json:"observed_at"`
	Region      int              `json:"region"`
	ObserverZ   *float64         `json:"observer_z,omitempty"`
	Truncated   bool             `json:"truncated,omitempty"`
	Monsters    []mobs.Monster   `json:"monsters"`
	NPCs        []npcs.NPC       `json:"npcs"`
	Players     []players.Player `json:"players"`
}

type agentPosition struct {
	Region int      `json:"region"`
	X      float64  `json:"x"`
	Y      float64  `json:"y"`
	Z      *float64 `json:"z,omitempty"`
}

type agentCapability struct {
	Name      string   `json:"name"`
	Supported bool     `json:"supported"`
	Reason    string   `json:"reason,omitempty"`
	Modes     []string `json:"modes,omitempty"`
}

type agentMessage struct {
	Type             string                     `json:"type"`
	ProtocolVersion  int                        `json:"protocol_version"`
	AgentID          string                     `json:"agent_id,omitempty"`
	PluginVersion    string                     `json:"plugin_version,omitempty"`
	PhBotVersion     string                     `json:"phbot_version,omitempty"`
	SentAt           string                     `json:"sent_at,omitempty"`
	CharacterID      string                     `json:"character_id,omitempty"`
	Server           string                     `json:"server,omitempty"`
	Name             string                     `json:"name,omitempty"`
	Guild            *string                    `json:"guild,omitempty"`
	State            characters.State           `json:"state,omitempty"`
	SessionID        string                     `json:"session_id,omitempty"`
	Sequence         uint64                     `json:"sequence,omitempty"`
	Position         *agentPosition             `json:"position,omitempty"`
	SchemaVersion    int                        `json:"schema_version,omitempty"`
	Commands         []agentCapability          `json:"commands,omitempty"`
	CommandID        string                     `json:"command_id,omitempty"`
	Status           string                     `json:"status,omitempty"`
	Reason           string                     `json:"reason,omitempty"`
	Verification     string                     `json:"verification,omitempty"`
	APIReturn        json.RawMessage            `json:"api_return,omitempty"`
	EffectiveArgs    json.RawMessage            `json:"effective_args,omitempty"`
	ObservedAfter    json.RawMessage            `json:"observed_after,omitempty"`
	ControlState     json.RawMessage            `json:"control_state,omitempty"`
	ResourceRevision uint64                     `json:"revision,omitempty"`
	BaseRevision     uint64                     `json:"base_revision,omitempty"`
	ChunkIndex       uint64                     `json:"chunk_index,omitempty"`
	ChunkCount       uint64                     `json:"chunk_count,omitempty"`
	Full             bool                       `json:"full,omitempty"`
	ResourceData     map[string]json.RawMessage `json:"resources,omitempty"`
	DeathEvent       *events.AgentDeath         `json:"event,omitempty"`
	Events           []events.AgentEvent        `json:"events,omitempty"`
	EventResults     []events.AppendResult      `json:"results,omitempty"`
	MapSnapshot      *agentMonsterSnapshot      `json:"map_snapshot,omitempty"`
	NavigationRoute  *navigation.Input          `json:"route,omitempty"`
	ObservedRoute    *navigation.ObservedInput  `json:"observed,omitempty"`
	MobSample        *mobs.Sample               `json:"sample,omitempty"`
}

type helloAck struct {
	Type                     string `json:"type"`
	ProtocolVersion          int    `json:"protocol_version"`
	HeartbeatIntervalSeconds int    `json:"heartbeat_interval_seconds"`
	HeartbeatTimeoutSeconds  int    `json:"heartbeat_timeout_seconds"`
	ServerTime               string `json:"server_time"`
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
	frameLimit := resources.MaxFrameBytes
	if events.MaxBatchBytes > frameLimit {
		frameLimit = events.MaxBatchBytes
	}
	if navigation.MaxObservedFrameBytes > frameLimit {
		frameLimit = navigation.MaxObservedFrameBytes
	}
	conn.SetReadLimit(int64(frameLimit))
	defer conn.Close(websocket.StatusNormalClosure, "")

	helloCtx, helloCancel := context.WithTimeout(r.Context(), h.options.HelloTimeout)
	var hello agentMessage
	err = wsjson.Read(helloCtx, conn, &hello)
	helloCancel()
	if err != nil {
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "hello required", hello.AgentID, hello.ProtocolVersion)
		return
	}
	if hello.Type != "hello" {
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "hello required", hello.AgentID, hello.ProtocolVersion)
		return
	}
	if hello.ProtocolVersion < agentMinProtocolVersion || hello.ProtocolVersion > agentProtocolVersion {
		rejectAgentFrame(conn, websocket.StatusUnsupportedData, "unsupported protocol version", hello.AgentID, hello.ProtocolVersion)
		return
	}
	if hello.AgentID != authenticatedID || !agentdomain.ValidAgentID(hello.AgentID) {
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "agent identity mismatch", hello.AgentID, hello.ProtocolVersion)
		return
	}
	if len(hello.PluginVersion) == 0 || len(hello.PluginVersion) > 64 || len(hello.PhBotVersion) == 0 || len(hello.PhBotVersion) > 64 {
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid version metadata", hello.AgentID, hello.ProtocolVersion)
		return
	}
	if _, err := time.Parse(time.RFC3339, hello.SentAt); err != nil {
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid timestamp", hello.AgentID, hello.ProtocolVersion)
		return
	}

	sessionCtx, sessionCancel := context.WithCancel(r.Context())
	generation, connectedAt := h.registry.Register(hello.AgentID)
	if generation == 0 {
		sessionCancel()
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "credential revocation in progress", hello.AgentID, hello.ProtocolVersion)
		return
	}
	unregisterPendingSession := func() {
		sessionCancel()
		h.registry.Unregister(hello.AgentID, generation)
	}

	updateCtx, updateCancel := context.WithTimeout(r.Context(), 2*time.Second)
	err = h.store.MarkConnected(updateCtx, hello.AgentID, connectedAt, hello.ProtocolVersion, hello.PluginVersion, hello.PhBotVersion)
	updateCancel()
	if errors.Is(err, agentdomain.ErrInvalidToken) {
		unregisterPendingSession()
		rejectAgentFrame(conn, websocket.StatusPolicyViolation, "credential revoked", hello.AgentID, hello.ProtocolVersion)
		return
	}
	if err != nil {
		unregisterPendingSession()
		rejectAgentFrame(conn, websocket.StatusInternalError, "state unavailable", hello.AgentID, hello.ProtocolVersion)
		return
	}

	writer := newAgentWriter(sessionCtx, conn, sessionCancel)
	if !h.registry.Configure(hello.AgentID, generation, hello.ProtocolVersion, hello.PluginVersion, writer.Send) {
		unregisterPendingSession()
		return
	}
	h.live.Invalidate()
	defer func() {
		// Log only the transport close status. Plugin tokens and frame contents are
		// intentionally excluded; this makes short post-handshake disconnects
		// diagnosable without exposing credentials or character data.
		if closeStatus := websocket.CloseStatus(r.Context().Err()); closeStatus != websocket.StatusNormalClosure {
			slog.Warn("agent connection ended", "agent_id", hello.AgentID, "protocol", hello.ProtocolVersion, "close_status", closeStatus)
		}
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
		if h.navigation != nil {
			h.navigation.RemoveAgentGeneration(hello.AgentID, generation)
		}
		if h.positions != nil {
			for _, removal := range h.positions.RemoveAgentGeneration(hello.AgentID, generation) {
				h.live.PublishPositionRemoval(removal)
			}
		}
		if h.playerLive != nil {
			h.playerLive.RemoveAgentGeneration(hello.AgentID, generation)
		}
		if !stillConnected {
			if h.mobLive != nil {
				h.mobLive.RemoveAgent(hello.AgentID)
			}
			if h.npcLive != nil {
				h.npcLive.RemoveAgent(hello.AgentID)
			}
			if h.navigation != nil {
				h.navigation.RemoveAgent(hello.AgentID)
			}
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
		// Protocol v3 workers require the canonical whole-second RFC3339 UTC
		// form so their clock offset parser can validate the Z suffix.
		ServerTime: time.Now().UTC().Format(time.RFC3339),
	}
	writeCtx, writeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
	err = writer.Send(writeCtx, ack)
	writeCancel()
	if err != nil {
		return
	}

	resourceAssemblies := make(map[string]*resources.Assembly)
	for {
		readCtx, readCancel := context.WithTimeout(sessionCtx, h.options.HeartbeatTimeout)
		var message agentMessage
		err := wsjson.Read(readCtx, conn, &message)
		readCancel()
		if err != nil {
			slog.Warn("agent websocket read failed", "agent_id", hello.AgentID, "protocol", hello.ProtocolVersion, "close_status", websocket.CloseStatus(err), "error", err.Error())
			return
		}
		if !h.registry.IsCurrent(hello.AgentID, generation) {
			return
		}
		if message.ProtocolVersion != hello.ProtocolVersion {
			rejectAgentFrame(conn, websocket.StatusUnsupportedData, "unsupported protocol version", hello.AgentID, hello.ProtocolVersion)
			return
		}
		if hello.ProtocolVersion < 4 {
			encoded, encodeErr := json.Marshal(message)
			if encodeErr != nil || len(encoded) > 8192 {
				rejectAgentFrame(conn, websocket.StatusMessageTooBig, "legacy frame too large", hello.AgentID, hello.ProtocolVersion)
				return
			}
		}
		switch message.Type {
		case "map.monsters":
			frame := message.MapSnapshot
			if hello.ProtocolVersion < 7 || frame == nil || h.characters == nil || h.mobLive == nil ||
				!agentdomain.ValidAgentID(frame.CharacterID) || !agentdomain.ValidAgentID(frame.SessionID) ||
				frame.ObserverZ != nil && !mobs.ValidCoordinate(*frame.ObserverZ) ||
				mobs.ValidateLiveSnapshot(frame.Status, frame.Region, frame.Monsters, time.Now().UTC(), frame.ObservedAt) != nil ||
				(frame.Status == "truncated") != frame.Truncated {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid map monster snapshot", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			character, characterErr := h.characters.GetScoped(ctx, frame.CharacterID, "")
			cancel()
			if characterErr != nil || !character.Online || character.SessionID == nil || *character.SessionID != frame.SessionID ||
				character.AgentID == nil || *character.AgentID != hello.AgentID {
				if !writeCharacterRejected(sessionCtx, writer, hello.ProtocolVersion, frame.CharacterID, frame.SessionID) {
					return
				}
				continue
			}
			if character.Region == nil || *character.Region != frame.Region {
				// State and snapshot can arrive out of order at a region seam; drop only this snapshot.
				continue
			}
			var dataset string
			var lookup mobs.LevelLookup
			if h.resources != nil {
				dataset, _ = h.resources.DatasetIDForServer(character.Server)
				lookup = h.resources.MonsterLevel
			}
			resolvedMonsters := make([]mobs.Monster, len(frame.Monsters))
			for i, monster := range frame.Monsters {
				resolvedMonsters[i] = mobs.ResolveLevel(monster, dataset, lookup)
			}
			h.mobLive.Apply(mobs.LiveSnapshot{Server: character.Server, AgentID: hello.AgentID, CharacterID: frame.CharacterID, SessionID: frame.SessionID,
				Character: character.Name, Status: frame.Status, Region: frame.Region, ObservedAt: frame.ObservedAt.UTC(),
				ObserverZ: frame.ObserverZ, Truncated: frame.Truncated, Monsters: resolvedMonsters})
			h.live.Invalidate()
		case "map.npcs":
			frame := message.MapSnapshot
			if hello.ProtocolVersion < 9 || frame == nil || h.characters == nil || h.npcLive == nil ||
				!agentdomain.ValidAgentID(frame.CharacterID) || !agentdomain.ValidAgentID(frame.SessionID) ||
				frame.ObserverZ != nil && !mobs.ValidCoordinate(*frame.ObserverZ) ||
				npcs.ValidateLiveSnapshot(frame.Status, frame.Region, frame.NPCs, time.Now().UTC(), frame.ObservedAt) != nil ||
				(frame.Status == "truncated") != frame.Truncated {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid map npc snapshot", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			character, characterErr := h.characters.GetScoped(ctx, frame.CharacterID, "")
			cancel()
			if characterErr != nil || !character.Online || character.SessionID == nil || *character.SessionID != frame.SessionID ||
				character.AgentID == nil || *character.AgentID != hello.AgentID {
				if !writeCharacterRejected(sessionCtx, writer, hello.ProtocolVersion, frame.CharacterID, frame.SessionID) {
					return
				}
				continue
			}
			if character.Region == nil || *character.Region != frame.Region {
				continue
			}
			h.npcLive.Apply(npcs.LiveSnapshot{Server: character.Server, AgentID: hello.AgentID, CharacterID: frame.CharacterID, SessionID: frame.SessionID,
				Character: character.Name, Status: frame.Status, Region: frame.Region, ObservedAt: frame.ObservedAt.UTC(),
				ObserverZ: frame.ObserverZ, Truncated: frame.Truncated, NPCs: frame.NPCs})
			h.live.Invalidate()
		case "map.players":
			frame := message.MapSnapshot
			if hello.ProtocolVersion < 10 || frame == nil || h.characters == nil || h.playerLive == nil ||
				!agentdomain.ValidAgentID(frame.CharacterID) || !agentdomain.ValidAgentID(frame.SessionID) ||
				frame.ObserverZ == nil || !mobs.ValidCoordinate(*frame.ObserverZ) ||
				players.ValidateLiveSnapshot(frame.Status, frame.Region, frame.Players, time.Now().UTC(), frame.ObservedAt) != nil ||
				(frame.Status == "truncated") != frame.Truncated {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid map player snapshot", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			character, characterErr := h.characters.GetScoped(ctx, frame.CharacterID, "")
			active, hasSession, sessionErr := h.characters.ActiveSession(ctx, frame.CharacterID)
			cancel()
			if characterErr != nil || sessionErr != nil || !hasSession || !character.Online ||
				character.SessionID == nil || *character.SessionID != frame.SessionID ||
				character.AgentID == nil || *character.AgentID != hello.AgentID ||
				active.AgentID != hello.AgentID || active.SessionID != frame.SessionID || active.Generation != generation {
				if !writeCharacterRejected(sessionCtx, writer, hello.ProtocolVersion, frame.CharacterID, frame.SessionID) {
					return
				}
				continue
			}
			if character.Region == nil || *character.Region != frame.Region {
				continue
			}
			ctxApply, cancelApply := context.WithTimeout(sessionCtx, 2*time.Second)
			activeNow, hasNow, activeNowErr := h.characters.ActiveSession(ctxApply, frame.CharacterID)
			cancelApply()
			if activeNowErr != nil || !hasNow || activeNow.SessionID != frame.SessionID ||
				activeNow.AgentID != hello.AgentID || activeNow.Generation != generation {
				continue
			}
			h.playerLive.Apply(players.LiveSnapshot{
				Server: character.Server, AgentID: hello.AgentID, Generation: generation,
				CharacterID: frame.CharacterID, SessionID: frame.SessionID, Character: character.Name,
				Status: frame.Status, Region: frame.Region, ObservedAt: frame.ObservedAt.UTC(),
				ObserverZ: frame.ObserverZ, Truncated: frame.Truncated, Players: frame.Players,
			})
			h.live.Invalidate()
		case "navigation.route":
			frame := message.NavigationRoute
			encoded, marshalErr := json.Marshal(message)
			if hello.ProtocolVersion < 8 || frame == nil || marshalErr != nil || len(encoded) > navigation.MaxFrameBytes ||
				h.navigation == nil || h.commands == nil || h.characters == nil || h.resources == nil || navigation.Valid(*frame) != nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid navigation route", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if h.navigation.AlreadyApplied(*frame, hello.AgentID, generation) {
				continue
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 3*time.Second)
			character, characterErr := h.characters.GetScoped(ctx, frame.CharacterID, "")
			command, commandErr := h.commands.GetByID(ctx, frame.CommandID)
			cancel()
			if characterErr != nil || commandErr != nil || !navigationRouteOwnerMatches(*frame, command, character, hello.AgentID, generation) {
				continue
			}
			var destination struct {
				Region int     `json:"region"`
				X      float64 `json:"x"`
				Y      float64 `json:"y"`
				Z      float64 `json:"z"`
			}
			if json.Unmarshal(command.Args, &destination) != nil || destination.Region == 0 {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid navigation command evidence", hello.AgentID, hello.ProtocolVersion)
				return
			}
			dataset, known := h.resources.DatasetIDForServer(character.Server)
			if !known {
				continue
			}
			if h.navigation.ReplaceIf(*frame, hello.AgentID, generation, character.Server, dataset, navigation.Point{
				Region: destination.Region, X: destination.X, Y: destination.Y, Z: destination.Z,
			}, time.Now().UTC(), func() bool {
				// Revalidate both the agent generation and durable character session
				// after the command lookup. ReplaceIf serializes this check with
				// session/generation cleanup for the route being written.
				if !h.registry.IsCurrent(hello.AgentID, generation) {
					return false
				}
				ownerCtx, ownerCancel := context.WithTimeout(sessionCtx, 2*time.Second)
				defer ownerCancel()
				current, ownerErr := h.characters.GetScoped(ownerCtx, frame.CharacterID, "")
				return ownerErr == nil && navigationCharacterOwnerMatches(current, frame.CharacterID, frame.SessionID, hello.AgentID) &&
					h.registry.IsCurrent(hello.AgentID, generation)
			}) {
				h.live.Invalidate()
			}
		case "navigation.observed":
			frame := message.ObservedRoute
			encoded, marshalErr := json.Marshal(message)
			if hello.ProtocolVersion < 17 || frame == nil || marshalErr != nil || len(encoded) > navigation.MaxObservedFrameBytes ||
				h.navigation == nil || h.characters == nil || h.resources == nil || navigation.ValidObserved(*frame) != nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid observed navigation", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ownerCurrent := func() bool {
				if !h.registry.IsCurrent(hello.AgentID, generation) {
					return false
				}
				ownerCtx, ownerCancel := context.WithTimeout(sessionCtx, 2*time.Second)
				defer ownerCancel()
				current, ownerErr := h.characters.GetScoped(ownerCtx, frame.CharacterID, "")
				return ownerErr == nil && navigationCharacterOwnerMatches(current, frame.CharacterID, frame.SessionID, hello.AgentID) &&
					h.registry.IsCurrent(hello.AgentID, generation)
			}
			if !frame.Active {
				if h.navigation.ClearObserved(*frame, hello.AgentID, generation, ownerCurrent) {
					h.live.Invalidate()
				}
				continue
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 3*time.Second)
			character, characterErr := h.characters.GetScoped(ctx, frame.CharacterID, "")
			cancel()
			if characterErr != nil || !navigationCharacterOwnerMatches(character, frame.CharacterID, frame.SessionID, hello.AgentID) {
				continue
			}
			dataset, known := h.resources.DatasetIDForServer(character.Server)
			if !known {
				continue
			}
			if h.navigation.ReplaceObserved(*frame, hello.AgentID, generation, character.Server, dataset, time.Now().UTC(), ownerCurrent) {
				h.live.Invalidate()
			}
		case "mob.sample":
			if hello.ProtocolVersion < 7 || h.mobs == nil || h.characters == nil || h.resources == nil || message.MobSample == nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid mob sample", hello.AgentID, hello.ProtocolVersion)
				return
			}
			sample := *message.MobSample
			if !agentdomain.ValidAgentID(sample.ID) || !agentdomain.ValidAgentID(sample.CharacterID) || !agentdomain.ValidAgentID(sample.SessionID) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid mob sample identity", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if mobs.ValidateSample(sample, time.Now().UTC()) != nil {
				if !ackMobSample(sessionCtx, writer, hello.ProtocolVersion, sample.ID, "rejected", "invalid_sample") {
					return
				}
				continue
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			character, characterErr := h.characters.GetScoped(ctx, sample.CharacterID, "")
			cancel()
			if characterErr != nil {
				status, reason := "retry", "temporarily_unavailable"
				if errors.Is(characterErr, characters.ErrNotFound) {
					status, reason = "rejected", "unknown_character"
				}
				if !ackMobSample(sessionCtx, writer, hello.ProtocolVersion, sample.ID, status, reason) {
					return
				}
				continue
			}
			dataset, knownServer := h.resources.DatasetIDForServer(character.Server)
			status, reason := "retry", "temporarily_unavailable"
			if !knownServer {
				status, reason = "rejected", "unsupported_server_profile"
			} else {
				ctx, cancel = context.WithTimeout(sessionCtx, 3*time.Second)
				inserted, appendErr := h.mobs.Append(ctx, hello.AgentID, dataset, sample, time.Now().UTC())
				cancel()
				switch {
				case appendErr == nil:
					status, reason = "persisted", ""
					if inserted {
						h.live.Invalidate()
					}
				case errors.Is(appendErr, mobs.ErrSamplingFrequency):
					status, reason = "rejected", "sample_frequency_limit"
				case errors.Is(appendErr, mobs.ErrUnauthorized):
					status, reason = "rejected", "session_or_region_rejected"
				case errors.Is(appendErr, mobs.ErrInvalidSample), errors.Is(appendErr, mobs.ErrConflict):
					status, reason = "rejected", "invalid_or_conflicting_sample"
				default:
					slog.Warn("mob observation persistence failed", "agent_id", hello.AgentID, "reason", appendErr.Error())
				}
			}
			if !ackMobSample(sessionCtx, writer, hello.ProtocolVersion, sample.ID, status, reason) {
				return
			}
		case "event.batch":
			if hello.ProtocolVersion < 6 || message.ProtocolVersion != hello.ProtocolVersion || h.events == nil || len(message.Events) < 1 || len(message.Events) > events.MaxBatchSize || !validMessageTime(message.SentAt) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid event batch", hello.AgentID, hello.ProtocolVersion)
				return
			}
			for _, event := range message.Events {
				if !agentdomain.ValidAgentID(event.ID) {
					rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid event identifier", hello.AgentID, hello.ProtocolVersion)
					return
				}
				if event.Source == "joymax.pet_inventory" && hello.ProtocolVersion < 14 {
					rejectAgentFrame(conn, websocket.StatusPolicyViolation, "pet inventory events require protocol 14", hello.AgentID, hello.ProtocolVersion)
					return
				}
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 3*time.Second)
			results, changed, eventErr := h.events.AppendBatch(ctx, hello.AgentID, message.Events)
			cancel()
			if eventErr != nil {
				slog.Warn("event batch persistence failed", "agent_id", hello.AgentID, "reason", eventErr.Error())
			}
			if changed {
				h.live.Invalidate()
			}
			ack := map[string]any{"type": "event.batch.ack", "protocol_version": hello.ProtocolVersion, "results": results}
			writeCtx, writeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
			writeErr := writer.Send(writeCtx, ack)
			writeCancel()
			if writeErr != nil {
				return
			}
		case "heartbeat":
			if _, err := time.Parse(time.RFC3339, message.SentAt); err != nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid timestamp", hello.AgentID, hello.ProtocolVersion)
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
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid capability report", hello.AgentID, hello.ProtocolVersion)
				return
			}
			capabilities := make([]agentdomain.CommandCapability, 0, len(message.Commands))
			seen := make(map[string]struct{}, len(message.Commands))
			for _, capability := range message.Commands {
				if !validCapabilityCommandName(capability.Name) {
					rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid capability report", hello.AgentID, hello.ProtocolVersion)
					return
				}
				// Capability catalogs are extensible within protocol v3. An older
				// server ignores commands it does not implement, allowing newer
				// plugins to keep reporting their full catalog without disconnecting.
				if !validReportedCommandName(capability.Name) {
					continue
				}
				if len(capability.Reason) > 64 {
					rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid capability report", hello.AgentID, hello.ProtocolVersion)
					return
				}
				for _, mode := range capability.Modes {
					validTrainingMode := capability.Name == "training.area.set" && (mode == "current_position" || mode == "position" || mode == "named")
					validChatMode := capability.Name == "chat.send" && (mode == "general" || mode == "private" || mode == "party" || mode == "guild" || mode == "union" || mode == "global")
					validReverseMode := capability.Name == "character.reverse_return" && (mode == "last_return" || mode == "last_death" || mode == "party_member" || mode == "named_location")
					if !validTrainingMode && !validChatMode && !validReverseMode {
						rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid capability mode", hello.AgentID, hello.ProtocolVersion)
						return
					}
				}
				if _, exists := seen[capability.Name]; exists {
					rejectAgentFrame(conn, websocket.StatusPolicyViolation, "duplicate capability", hello.AgentID, hello.ProtocolVersion)
					return
				}
				seen[capability.Name] = struct{}{}
				capabilities = append(capabilities, agentdomain.CommandCapability{
					Name: capability.Name, Supported: capability.Supported, Reason: capability.Reason,
					Modes: capability.Modes,
				})
			}
			if !h.registry.SetCapabilities(hello.AgentID, generation, capabilities) {
				return
			}
			h.live.Invalidate()
		case "command.ack":
			if hello.ProtocolVersion < 3 || h.commands == nil || !validCommandFrame(message) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid command acknowledgement", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			changed, e := h.commands.Acknowledge(ctx, message.CommandID, hello.AgentID, message.SessionID, generation, time.Now().UTC())
			cancel()
			if e != nil && !errors.Is(e, commands.ErrNotFound) {
				rejectAgentFrame(conn, websocket.StatusInternalError, "command acknowledgement unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if changed {
				h.live.Invalidate()
			}
		case "command.result":
			if hello.ProtocolVersion < 3 || h.commands == nil || !validCommandFrame(message) || !validCommandResultStatus(message.Status) || (message.Verification != "api_confirmed" && message.Verification != "observed" && message.Verification != "unverified") {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid command result", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			changed, e := h.commands.Result(ctx, message.CommandID, hello.AgentID, message.SessionID, generation, time.Now().UTC(), commands.ResultInput{Status: commands.State(message.Status), Code: message.Reason, Message: message.Reason, Verification: message.Verification, APIReturn: message.APIReturn, EffectiveArgs: message.EffectiveArgs, ObservedAfter: message.ObservedAfter})
			cancel()
			if e != nil && !errors.Is(e, commands.ErrNotFound) {
				rejectAgentFrame(conn, websocket.StatusInternalError, "command result unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if changed && h.navigation != nil && h.commands != nil {
				resultCtx, resultCancel := context.WithTimeout(sessionCtx, 2*time.Second)
				command, commandErr := h.commands.GetByID(resultCtx, message.CommandID)
				resultCancel()
				if commandErr == nil && command.Name == "character.navigate.stop" &&
					(message.Status == "completed" || message.Status == "failed") &&
					message.Verification == "api_confirmed" {
					var args struct {
						CommandID     string `json:"command_id"`
						RouteSequence uint64 `json:"route_sequence"`
					}
					if json.Unmarshal(command.Args, &args) == nil {
						success := message.Status == "completed"
						if h.navigation.MarkNavigationStopped(command.SessionID, args.CommandID, args.RouteSequence, success, time.Now().UTC()) {
							h.live.Invalidate()
						}
					}
				}
			}
			if changed {
				h.live.Invalidate()
			}
		case "character.control_state":
			if hello.ProtocolVersion < 3 || h.commands == nil || !validCommandFrame(agentMessage{CommandID: "cmd_00000000-0000-4000-8000-000000000000", CharacterID: message.CharacterID, SessionID: message.SessionID}) || len(message.ControlState) == 0 || len(message.ControlState) > 2048 {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid control state", hello.AgentID, hello.ProtocolVersion)
				return
			}
			var control commands.ControlState
			if err := json.Unmarshal(message.ControlState, &control); err != nil || !control.ZoneNameValid() || !control.ActivityValid() || !control.TraceRequestedValid() {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid control state", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			err := h.commands.SaveControlState(ctx, message.CharacterID, message.SessionID, hello.AgentID, generation, control)
			cancel()
			if errors.Is(err, commands.ErrStaleSession) {
				if !writeCharacterRejected(sessionCtx, writer, hello.ProtocolVersion, message.CharacterID, message.SessionID) {
					return
				}
				continue
			}
			if err != nil {
				rejectAgentFrame(conn, websocket.StatusInternalError, "control state unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			h.live.Invalidate()
		case "character.identify":
			if h.characters == nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "character state unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if !validMessageTime(message.SentAt) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid timestamp", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			id, e := h.characters.Resolve(ctx, characters.Identity{Server: message.Server, Name: message.Name, Guild: message.Guild})
			var sessionID string
			var previous commands.Target
			if e == nil && h.commands != nil {
				previous, _ = h.commands.ResolveTarget(ctx, id)
			}
			if e == nil {
				sessionID, e = h.characters.ClaimSessionID(ctx, hello.AgentID, id, generation)
			}
			cancel()
			if e != nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid character identity", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if h.navigation != nil && previous.SessionID != "" {
				// A worker can re-identify within the same claimed session after a
				// profile change and reset its route sequence to one. Clear the old
				// snapshot in either case so that the new route can be admitted.
				h.navigation.RemoveSession(previous.SessionID)
			}
			if h.npcLive != nil && previous.SessionID != "" && previous.SessionID != sessionID {
				h.npcLive.RemoveSession(previous.SessionID)
			}
			if h.playerLive != nil && previous.SessionID != "" && previous.SessionID != sessionID {
				h.playerLive.RemoveSession(previous.SessionID)
			}
			if h.positions != nil {
				for _, removal := range h.positions.Claim(message.Server, hello.AgentID, generation, id, sessionID) {
					h.live.PublishPositionRemoval(removal)
				}
			}
			if previous.SessionID != "" && (previous.AgentID != hello.AgentID || previous.Generation != generation) {
				revokeCtx, revokeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
				previousProtocol := h.registry.ProtocolVersion(previous.AgentID, previous.Generation)
				if previousProtocol >= 3 {
					_ = h.registry.Send(revokeCtx, previous.AgentID, previous.Generation, map[string]any{"type": "command.revoke", "protocol_version": previousProtocol, "character_id": id, "session_id": previous.SessionID, "reason": "session_superseded"})
				}
				revokeCancel()
			}
			h.live.Invalidate()
			reply := map[string]any{"type": "character.registered", "protocol_version": hello.ProtocolVersion, "character_id": id}
			if hello.ProtocolVersion >= 3 {
				reply["session_id"] = sessionID
			}
			if e = writer.Send(sessionCtx, reply); e != nil {
				return
			}
		case "character.position":
			if hello.ProtocolVersion < 16 || h.positions == nil || message.Position == nil ||
				!agentdomain.ValidAgentID(message.CharacterID) || !agentdomain.ValidAgentID(message.SessionID) ||
				message.Sequence == 0 || !validMessageTime(message.SentAt) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid character position", hello.AgentID, hello.ProtocolVersion)
				return
			}
			observedAt, parseErr := time.Parse(time.RFC3339, message.SentAt)
			if parseErr != nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid character position timestamp", hello.AgentID, hello.ProtocolVersion)
				return
			}
			input := positions.Position{
				AgentID: hello.AgentID, Generation: generation, CharacterID: message.CharacterID,
				SessionID: message.SessionID, Sequence: message.Sequence,
				Region: message.Position.Region, X: message.Position.X, Y: message.Position.Y, Z: message.Position.Z,
				ObservedAt: observedAt.UTC(),
			}
			accepted, ok := h.applyRealtimePosition(input, time.Now().UTC())
			if !ok {
				continue
			}
			if h.characters != nil && h.positions.CheckpointDue(accepted, time.Now().UTC(), time.Second) {
				checkpointCtx, checkpointCancel := context.WithTimeout(sessionCtx, 2*time.Second)
				checkpointErr := h.characters.UpdatePositionSession(
					checkpointCtx, hello.AgentID, accepted.CharacterID, generation, accepted.SessionID,
					accepted.Region, accepted.X, accepted.Y, accepted.Z,
				)
				checkpointCancel()
				if checkpointErr != nil {
					slog.Warn("realtime position checkpoint failed", "agent_id", hello.AgentID, "character_id", accepted.CharacterID, "reason", checkpointErr.Error())
				}
				if h.analytics != nil && h.resources != nil {
					if datasetID, known := h.resources.DatasetIDForServer(accepted.Server); known {
						analyticsCtx, analyticsCancel := context.WithTimeout(sessionCtx, 2*time.Second)
						_, analyticsErr := h.analytics.RecordPosition(analyticsCtx, mapanalytics.PositionSample{
							AgentID: hello.AgentID, CharacterID: accepted.CharacterID, SessionID: accepted.SessionID,
							DatasetID: datasetID, SampledAt: accepted.ObservedAt, Region: accepted.Region,
							X: accepted.X, Y: accepted.Y, Z: accepted.Z,
						}, time.Now().UTC())
						analyticsCancel()
						if analyticsErr != nil && !errors.Is(analyticsErr, mapanalytics.ErrStaleSession) {
							slog.Warn("movement analytics persistence failed", "agent_id", hello.AgentID, "character_id", accepted.CharacterID, "reason", analyticsErr.Error())
						}
					}
				}
			}
		case "character.snapshot", "character.state":
			if !agentdomain.ValidAgentID(message.CharacterID) || !validWireState(message.State) || h.characters == nil ||
				(hello.ProtocolVersion >= 3 && !agentdomain.ValidAgentID(message.SessionID)) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid character state", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if !validMessageTime(message.SentAt) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid timestamp", hello.AgentID, hello.ProtocolVersion)
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
				rejectAgentFrame(conn, websocket.StatusInternalError, "character state unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if h.navigation != nil && message.State.Region != nil && message.State.X != nil && message.State.Y != nil {
				if observedAt, parseErr := time.Parse(time.RFC3339, message.SentAt); parseErr == nil {
					h.navigation.Observe(message.CharacterID, message.SessionID, navigation.Position{
						Region: *message.State.Region, X: *message.State.X, Y: *message.State.Y, Z: message.State.Z, At: observedAt.UTC(),
					})
				}
			}
			// Protocol 16+ moves movement analytics to the throttled realtime
			// position checkpoint path. Preserve the established state-frame
			// behavior for older compatible agents that do not know that frame.
			if hello.ProtocolVersion >= 3 && hello.ProtocolVersion < 16 &&
				h.analytics != nil && h.resources != nil &&
				message.State.Region != nil && message.State.X != nil && message.State.Y != nil {
				if sampledAt, parseErr := time.Parse(time.RFC3339, message.SentAt); parseErr == nil {
					analyticsCtx, analyticsCancel := context.WithTimeout(sessionCtx, 2*time.Second)
					if character, characterErr := h.characters.GetScoped(analyticsCtx, message.CharacterID, ""); characterErr == nil {
						if datasetID, ok := h.resources.DatasetIDForServer(character.Server); ok {
							_, analyticsErr := h.analytics.RecordPosition(analyticsCtx, mapanalytics.PositionSample{
								AgentID: hello.AgentID, CharacterID: message.CharacterID, SessionID: message.SessionID,
								DatasetID: datasetID, SampledAt: sampledAt.UTC(), Region: *message.State.Region,
								X: *message.State.X, Y: *message.State.Y, Z: message.State.Z,
							}, time.Now().UTC())
							if analyticsErr != nil && !errors.Is(analyticsErr, mapanalytics.ErrStaleSession) {
								slog.Warn("legacy movement analytics persistence failed", "agent_id", hello.AgentID, "character_id", message.CharacterID, "reason", analyticsErr.Error())
							}
						}
					}
					analyticsCancel()
				}
			}
			h.live.Invalidate()
		case "character.died":
			if hello.ProtocolVersion < 5 || h.events == nil ||
				!agentdomain.ValidAgentID(message.CharacterID) || !agentdomain.ValidAgentID(message.SessionID) ||
				message.DeathEvent == nil || !validMessageTime(message.SentAt) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid death event", hello.AgentID, hello.ProtocolVersion)
				return
			}
			ctx, cancel := context.WithTimeout(sessionCtx, 2*time.Second)
			persisted, eventErr := h.events.AppendDeath(ctx, hello.AgentID, message.CharacterID, message.SessionID, *message.DeathEvent)
			cancel()
			ack := map[string]any{"type": "event.ack", "protocol_version": hello.ProtocolVersion, "event_id": message.DeathEvent.ID}
			if eventErr == nil {
				ack["status"] = "persisted"
				if persisted {
					h.live.Invalidate()
				}
			} else if errors.Is(eventErr, events.ErrInvalidEvent) {
				ack["status"] = "rejected"
				ack["reason"] = "invalid_event"
				slog.Warn("death event rejected", "agent_id", hello.AgentID, "reason", eventErr.Error())
			} else if errors.Is(eventErr, events.ErrUnauthorizedSession) || errors.Is(eventErr, events.ErrEventConflict) {
				ack["status"] = "rejected"
				ack["reason"] = "session_or_event_rejected"
				slog.Warn("death event rejected", "agent_id", hello.AgentID, "character_id", message.CharacterID, "reason", eventErr.Error())
			} else {
				ack["status"] = "retry"
				ack["reason"] = "temporarily_unavailable"
				slog.Warn("death event persistence failed", "agent_id", hello.AgentID, "reason", eventErr.Error())
			}
			writeCtx, writeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
			writeErr := writer.Send(writeCtx, ack)
			writeCancel()
			if writeErr != nil {
				return
			}
		case "character.left":
			if !agentdomain.ValidAgentID(message.CharacterID) || h.characters == nil ||
				(hello.ProtocolVersion >= 3 && !agentdomain.ValidAgentID(message.SessionID)) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "character_id required", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if !validMessageTime(message.SentAt) {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid timestamp", hello.AgentID, hello.ProtocolVersion)
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
				rejectAgentFrame(conn, websocket.StatusInternalError, "character state unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if h.mobLive != nil && message.SessionID != "" {
				h.mobLive.RemoveSession(message.SessionID)
			}
			if h.npcLive != nil && message.SessionID != "" {
				h.npcLive.RemoveSession(message.SessionID)
			}
			if h.playerLive != nil && message.SessionID != "" {
				h.playerLive.RemoveSession(message.SessionID)
			}
			if h.navigation != nil && message.SessionID != "" {
				h.navigation.RemoveSession(message.SessionID)
			}
			if h.positions != nil && message.SessionID != "" {
				for _, removal := range h.positions.RemoveSession(message.SessionID) {
					h.live.PublishPositionRemoval(removal)
				}
			}
			h.live.Invalidate()
		case "resource.snapshot", "resource.delta":
			if hello.ProtocolVersion < 4 || h.resources == nil || h.characters == nil ||
				!agentdomain.ValidAgentID(message.CharacterID) || !agentdomain.ValidAgentID(message.SessionID) ||
				message.ResourceRevision == 0 || !validMessageTime(message.SentAt) || len(message.ResourceData) == 0 ||
				message.ChunkCount == 0 || message.ChunkCount > resources.MaxResources || message.ChunkIndex >= message.ChunkCount {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid resource snapshot", hello.AgentID, hello.ProtocolVersion)
				return
			}
			full := message.Type == "resource.snapshot"
			if message.Full != full {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid resource snapshot kind", hello.AgentID, hello.ProtocolVersion)
				return
			}
			snapshot := resources.Snapshot{Revision: message.ResourceRevision, BaseRevision: message.BaseRevision, Full: full, Resources: message.ResourceData}
			if _, _, err := resources.Validate(snapshot); err != nil {
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid resource snapshot", hello.AgentID, hello.ProtocolVersion)
				return
			}
			assemblyKey := message.CharacterID + ":" + message.SessionID + ":" + strconv.FormatUint(message.ResourceRevision, 10)
			assembly := resourceAssemblies[assemblyKey]
			if assembly == nil {
				prefix := message.CharacterID + ":" + message.SessionID + ":"
				for key := range resourceAssemblies {
					if strings.HasPrefix(key, prefix) {
						delete(resourceAssemblies, key)
					}
				}
				if len(resourceAssemblies) >= 4 {
					rejectAgentFrame(conn, websocket.StatusPolicyViolation, "too many incomplete resource baselines", hello.AgentID, hello.ProtocolVersion)
					return
				}
				assembly = &resources.Assembly{}
				resourceAssemblies[assemblyKey] = assembly
			}
			assembled, complete, err := assembly.Add(snapshot, message.ChunkIndex, message.ChunkCount)
			if err != nil {
				delete(resourceAssemblies, assemblyKey)
				rejectAgentFrame(conn, websocket.StatusPolicyViolation, "invalid resource snapshot chunks", hello.AgentID, hello.ProtocolVersion)
				return
			}
			if !complete {
				continue
			}
			delete(resourceAssemblies, assemblyKey)
			ctx, cancel := context.WithTimeout(sessionCtx, 4*time.Second)
			err = h.resources.Apply(ctx, hello.AgentID, message.CharacterID, generation, message.ResourceRevision, message.SessionID, assembled)
			cancel()
			if errors.Is(err, resources.ErrSequence) || errors.Is(err, resources.ErrStale) {
				writeCtx, writeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
				writeErr := writer.Send(writeCtx, map[string]any{"type": "resource.resync", "protocol_version": hello.ProtocolVersion, "character_id": message.CharacterID, "session_id": message.SessionID, "reason": "baseline_required"})
				writeCancel()
				if writeErr != nil {
					return
				}
				continue
			}
			if err != nil {
				rejectAgentFrame(conn, websocket.StatusInternalError, "resource snapshot unavailable", hello.AgentID, hello.ProtocolVersion)
				return
			}
			writeCtx, writeCancel := context.WithTimeout(sessionCtx, 2*time.Second)
			err = writer.Send(writeCtx, map[string]any{"type": "resource.ack", "protocol_version": hello.ProtocolVersion, "character_id": message.CharacterID, "session_id": message.SessionID, "revision": message.ResourceRevision})
			writeCancel()
			if err != nil {
				return
			}
			h.live.Invalidate()
		default:
			rejectAgentFrame(conn, websocket.StatusPolicyViolation, "unexpected agent message", hello.AgentID, hello.ProtocolVersion)
			return
		}
	}
}

func (h *agentHandler) applyRealtimePosition(input positions.Position, now time.Time) (positions.Position, bool) {
	if h == nil || h.positions == nil {
		return positions.Position{}, false
	}
	accepted, ok := h.positions.Apply(input, now)
	if !ok {
		return positions.Position{}, false
	}
	if h.live != nil {
		h.live.PublishPosition(accepted)
	}
	if h.navigation != nil {
		h.navigation.Observe(accepted.CharacterID, accepted.SessionID, navigation.Position{
			Region: accepted.Region,
			X:      accepted.X,
			Y:      accepted.Y,
			Z:      accepted.Z,
			At:     accepted.ObservedAt,
		})
	}
	return accepted, true
}

func navigationRouteOwnerMatches(route navigation.Input, command commands.Command, character characters.Character, agentID string, generation uint64) bool {
	return navigationCharacterOwnerMatches(character, route.CharacterID, route.SessionID, agentID) &&
		command.CharacterID == route.CharacterID && command.SessionID == route.SessionID && command.AgentID == agentID &&
		command.ConnectionGeneration == generation && command.Name == "character.navigate" && command.State == commands.StateCompleted
}

func navigationCharacterOwnerMatches(character characters.Character, characterID, sessionID, agentID string) bool {
	return character.ID == characterID && character.Online && character.SessionID != nil &&
		*character.SessionID == sessionID && character.AgentID != nil && *character.AgentID == agentID
}

func ackMobSample(ctx context.Context, writer *agentWriter, protocol int, sampleID, status, reason string) bool {
	ack := map[string]any{"type": "mob.sample.ack", "protocol_version": protocol, "sample_id": sampleID, "status": status}
	if reason != "" {
		ack["reason"] = reason
	}
	writeCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
	defer cancel()
	return writer.Send(writeCtx, ack) == nil
}

func validCommandResultStatus(status string) bool {
	return status == "completed" || status == "failed" || status == "unknown"
}

func validCommandFrame(message agentMessage) bool {
	return len(message.CommandID) == 40 && strings.HasPrefix(message.CommandID, "cmd_") && agentdomain.ValidAgentID(message.CharacterID) && agentdomain.ValidAgentID(message.SessionID)
}

func rejectAgentFrame(conn *websocket.Conn, status websocket.StatusCode, reason, agentID string, protocol int) {
	slog.Warn("agent frame rejected", "agent_id", agentID, "protocol", protocol, "close_status", status, "reason", reason)
	_ = conn.Close(status, reason)
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
		"training.area.set", "training.radius.set", "character.walk", "character.move_to", "character.navigate",
		"character.navigate.stop", "character.teleport", "character.recall_point.designate",
		"character.return", "character.reverse_return", "character.disconnect", "client.clientless", "chat.send":
		return true
	default:
		return false
	}
}

func validCapabilityCommandName(name string) bool {
	if len(name) == 0 || len(name) > 64 || name[0] < 'a' || name[0] > 'z' {
		return false
	}
	segmentStart := false
	for i := 0; i < len(name); i++ {
		char := name[i]
		switch {
		case char >= 'a' && char <= 'z', char >= '0' && char <= '9':
			segmentStart = false
		case (char == '.' || char == '_') && !segmentStart && i < len(name)-1:
			segmentStart = true
		default:
			return false
		}
	}
	return !segmentStart
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
	if h.live != nil {
		h.live.Invalidate()
	}

	respondJSON(w, http.StatusCreated, AgentCredentialView{
		AgentID:    credential.AgentID,
		AgentToken: credential.Token,
	})
}

func (h *agentHandler) remove(w http.ResponseWriter, r *http.Request) {
	agentID := r.PathValue("id")
	if !agentdomain.ValidAgentID(agentID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid agent id"})
		return
	}
	if !h.registry.BeginCredentialRevocation(agentID) {
		respondJSON(w, http.StatusConflict, map[string]string{"error": "agent connected or removal in progress"})
		return
	}
	defer h.registry.EndCredentialRevocation(agentID)

	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	removed, err := h.store.RevokeCredential(ctx, agentID)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	if !removed {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "agent not found"})
		return
	}
	if h.live != nil {
		h.live.Invalidate()
	}
	respondJSON(w, http.StatusOK, map[string]string{"status": "removed"})
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
