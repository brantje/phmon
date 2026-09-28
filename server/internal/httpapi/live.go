package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"net/url"
	"strings"
	"sync"
	"time"

	"github.com/coder/websocket"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/characters"
	"phmon/server/internal/commands"
	"phmon/server/internal/events"
	"phmon/server/internal/resources"
)

const (
	liveProtocolVersion       = 1
	liveMaxClientMessageBytes = 16 * 1024
	liveMaxServerMessageBytes = 512 * 1024
	liveMaxSubscriptions      = 32
	liveOutgoingQueueSize     = 64
	liveSnapshotTimeout       = 3 * time.Second
	liveWriteTimeout          = 3 * time.Second
	liveHeartbeatInterval     = 10 * time.Second
	liveHeartbeatTimeout      = 35 * time.Second
	liveSnapshotCoalesce      = 500 * time.Millisecond
	liveMaxConcurrentBuilds   = 2
)

type liveFilter struct {
	Query        string   `json:"q,omitempty"`
	GroupID      string   `json:"group_id,omitempty"`
	CharacterID  string   `json:"character_id,omitempty"`
	CommandName  string   `json:"command_name,omitempty"`
	CommandState string   `json:"command_state,omitempty"`
	Limit        int      `json:"limit,omitempty"`
	ResourceKeys []string `json:"resource_keys,omitempty"`
	Server       string   `json:"server,omitempty"`
	Kind         string   `json:"kind,omitempty"`
	From         string   `json:"from,omitempty"`
	To           string   `json:"to,omitempty"`
	Cursor       string   `json:"cursor,omitempty"`
}

type liveClientMessage struct {
	Type            string     `json:"type"`
	ProtocolVersion int        `json:"protocol_version"`
	SubscriptionID  string     `json:"subscription_id,omitempty"`
	Revision        uint64     `json:"revision,omitempty"`
	Stream          string     `json:"stream,omitempty"`
	Filter          liveFilter `json:"filter,omitempty"`
}

type liveSubscription struct {
	ID       string
	Revision uint64
	Stream   string
	Filter   liveFilter
}

type liveServerMessage struct {
	Type            string `json:"type"`
	ProtocolVersion int    `json:"protocol_version"`
	SubscriptionID  string `json:"subscription_id,omitempty"`
	Revision        uint64 `json:"revision,omitempty"`
	Stream          string `json:"stream,omitempty"`
	Reason          string `json:"reason,omitempty"`
	SentAt          string `json:"sent_at,omitempty"`
	Data            any    `json:"data,omitempty"`
}

type LiveHub struct {
	agents     AgentStore
	registry   *agentdomain.Registry
	characters *characters.Store
	commands   *commands.Service
	resources  *resources.Store
	events     *events.Store

	mu         sync.RWMutex
	clients    map[*liveClient]struct{}
	buildSlots chan struct{}
}

func (h *LiveHub) SetCommands(service *commands.Service) { h.commands = service }
func (h *LiveHub) SetResources(store *resources.Store)   { h.resources = store }
func (h *LiveHub) SetEvents(store *events.Store)         { h.events = store }

func NewLiveHub(agents AgentStore, registry *agentdomain.Registry, characterStore *characters.Store) *LiveHub {
	return &LiveHub{
		agents:     agents,
		registry:   registry,
		characters: characterStore,
		clients:    make(map[*liveClient]struct{}),
		buildSlots: make(chan struct{}, liveMaxConcurrentBuilds),
	}
}

// Invalidate schedules replacement snapshots for every active browser subscription.
// Each client owns a capacity-one trigger, so bursts coalesce while a snapshot is
// being built. If another committed change arrives during snapshot creation, the
// trigger remains queued and a second pass follows; the latest commit is never lost.
func (h *LiveHub) Invalidate() {
	if h == nil {
		return
	}
	h.mu.RLock()
	defer h.mu.RUnlock()
	for client := range h.clients {
		client.notify()
	}
}

func (h *LiveHub) register(client *liveClient) {
	h.mu.Lock()
	h.clients[client] = struct{}{}
	h.mu.Unlock()
}

func (h *LiveHub) unregister(client *liveClient) {
	h.mu.Lock()
	delete(h.clients, client)
	h.mu.Unlock()
}

func (h *LiveHub) connect(w http.ResponseWriter, r *http.Request) {
	_, authenticatedOperator := operatorSessionFromContext(r.Context())
	if !authenticatedOperator && !liveOriginAllowed(r) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "cross-origin websocket rejected"})
		return
	}

	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		// The operator middleware has already validated the exact configured
		// browser Origin and authenticated its session. The request reaches Go
		// through Nuxt, so the backend Host differs from the browser Origin.
		InsecureSkipVerify: true,
		CompressionMode:    websocket.CompressionDisabled,
	})
	if err != nil {
		return
	}
	conn.SetReadLimit(liveMaxClientMessageBytes)

	ctx, cancel := context.WithCancel(r.Context())
	if operatorSession, ok := operatorSessionFromContext(r.Context()); ok {
		go func() {
			delay := time.Until(operatorSession.watch.ExpiresAt)
			if delay < 0 {
				delay = 0
			}
			timer := time.NewTimer(delay)
			defer timer.Stop()
			select {
			case <-operatorSession.watch.Done:
			case <-timer.C:
			case <-ctx.Done():
				return
			}
			cancel()
			_ = conn.Close(websocket.StatusPolicyViolation, "operator session expired")
		}()
	}
	client := &liveClient{
		hub:           h,
		conn:          conn,
		ctx:           ctx,
		cancel:        cancel,
		outgoing:      make(chan []byte, liveOutgoingQueueSize),
		snapshotWake:  make(chan struct{}, 1),
		subscriptions: make(map[string]liveSubscription),
		revisions:     make(map[string]uint64),
	}
	h.register(client)
	defer func() {
		h.unregister(client)
		cancel()
		_ = conn.Close(websocket.StatusNormalClosure, "")
	}()

	writerDone := make(chan struct{})
	go func() {
		defer close(writerDone)
		client.writeLoop()
	}()
	snapshotDone := make(chan struct{})
	go func() {
		defer close(snapshotDone)
		client.snapshotLoop()
	}()

	client.readLoop()
	cancel()
	select {
	case <-writerDone:
	case <-time.After(liveWriteTimeout):
	}
	select {
	case <-snapshotDone:
	case <-time.After(liveWriteTimeout):
	}
}

type liveClient struct {
	hub    *LiveHub
	conn   *websocket.Conn
	ctx    context.Context
	cancel context.CancelFunc

	outgoing     chan []byte
	snapshotWake chan struct{}

	mu            sync.RWMutex
	subscriptions map[string]liveSubscription
	revisions     map[string]uint64
}

func (c *liveClient) notify() {
	select {
	case c.snapshotWake <- struct{}{}:
	default:
	}
}

func (c *liveClient) fail(status websocket.StatusCode, reason string) {
	c.cancel()
	if c.conn == nil {
		return
	}
	// Send protocol failures synchronously so the handler's normal-close defer
	// cannot race and overwrite the close code observed by the peer.
	_ = c.conn.Close(status, reason)
}

func (c *liveClient) enqueue(value liveServerMessage) bool {
	payload, err := json.Marshal(value)
	if err != nil {
		c.fail(websocket.StatusInternalError, "cannot encode live data")
		return false
	}
	if len(payload) > liveMaxServerMessageBytes {
		c.fail(websocket.StatusMessageTooBig, "live snapshot too large")
		return false
	}
	select {
	case <-c.ctx.Done():
		return false
	case c.outgoing <- payload:
		return true
	default:
		c.fail(websocket.StatusTryAgainLater, "slow live consumer")
		return false
	}
}

func (c *liveClient) readLoop() {
	for {
		readCtx, cancel := context.WithTimeout(c.ctx, liveHeartbeatTimeout)
		messageType, payload, err := c.conn.Read(readCtx)
		cancel()
		if err != nil {
			return
		}
		if messageType != websocket.MessageText || len(payload) > liveMaxClientMessageBytes {
			c.fail(websocket.StatusUnsupportedData, "text JSON required")
			return
		}
		var message liveClientMessage
		decoder := json.NewDecoder(strings.NewReader(string(payload)))
		decoder.DisallowUnknownFields()
		if err := decoder.Decode(&message); err != nil {
			c.fail(websocket.StatusPolicyViolation, "malformed live frame")
			return
		}
		var trailing any
		if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
			c.fail(websocket.StatusPolicyViolation, "malformed live frame")
			return
		}
		if message.ProtocolVersion != liveProtocolVersion {
			c.fail(websocket.StatusUnsupportedData, "unsupported live protocol")
			return
		}

		switch message.Type {
		case "heartbeat":
			continue
		case "subscribe":
			if !c.subscribe(message) {
				return
			}
		case "unsubscribe":
			c.unsubscribe(message)
		case "refresh":
			c.refresh(message)
		default:
			c.fail(websocket.StatusPolicyViolation, "unknown live frame")
			return
		}
	}
}

func (c *liveClient) subscribe(message liveClientMessage) bool {
	subscription, ok := validateLiveSubscription(message)
	if !ok {
		c.fail(websocket.StatusPolicyViolation, "invalid subscription")
		return false
	}

	c.mu.Lock()
	_, exists := c.subscriptions[subscription.ID]
	highestRevision := c.revisions[subscription.ID]
	if subscription.Revision <= highestRevision {
		c.mu.Unlock()
		c.enqueue(liveServerMessage{
			Type:            "subscription.rejected",
			ProtocolVersion: liveProtocolVersion,
			SubscriptionID:  subscription.ID,
			Revision:        subscription.Revision,
			Stream:          subscription.Stream,
			Reason:          "obsolete_revision",
		})
		return true
	}
	if !exists && len(c.subscriptions) >= liveMaxSubscriptions {
		c.mu.Unlock()
		c.fail(websocket.StatusPolicyViolation, "too many subscriptions")
		return false
	}
	c.subscriptions[subscription.ID] = subscription
	c.revisions[subscription.ID] = subscription.Revision
	c.mu.Unlock()
	c.notify()
	return true
}

func (c *liveClient) unsubscribe(message liveClientMessage) {
	if !validSubscriptionID(message.SubscriptionID) || message.Revision == 0 {
		c.fail(websocket.StatusPolicyViolation, "invalid unsubscribe")
		return
	}
	c.mu.Lock()
	current, exists := c.subscriptions[message.SubscriptionID]
	if exists && message.Revision >= current.Revision {
		delete(c.subscriptions, message.SubscriptionID)
	}
	c.mu.Unlock()
}

func (c *liveClient) refresh(message liveClientMessage) {
	if !validSubscriptionID(message.SubscriptionID) || message.Revision == 0 {
		c.fail(websocket.StatusPolicyViolation, "invalid refresh")
		return
	}
	c.mu.RLock()
	current, exists := c.subscriptions[message.SubscriptionID]
	c.mu.RUnlock()
	if !exists || current.Revision != message.Revision {
		c.enqueue(liveServerMessage{
			Type:            "subscription.rejected",
			ProtocolVersion: liveProtocolVersion,
			SubscriptionID:  message.SubscriptionID,
			Revision:        message.Revision,
			Reason:          "obsolete_revision",
		})
		return
	}
	c.notify()
}

func (c *liveClient) writeLoop() {
	ticker := time.NewTicker(liveHeartbeatInterval)
	defer ticker.Stop()
	for {
		select {
		case <-c.ctx.Done():
			return
		case payload := <-c.outgoing:
			writeCtx, cancel := context.WithTimeout(c.ctx, liveWriteTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				c.cancel()
				return
			}
		case <-ticker.C:
			payload, _ := json.Marshal(liveServerMessage{
				Type:            "heartbeat",
				ProtocolVersion: liveProtocolVersion,
				SentAt:          time.Now().UTC().Format(time.RFC3339),
			})
			writeCtx, cancel := context.WithTimeout(c.ctx, liveWriteTimeout)
			err := c.conn.Write(writeCtx, websocket.MessageText, payload)
			cancel()
			if err != nil {
				c.cancel()
				return
			}
		}
	}
}

func (c *liveClient) snapshotLoop() {
	for {
		select {
		case <-c.ctx.Done():
			return
		case <-c.snapshotWake:
		}

		timer := time.NewTimer(liveSnapshotCoalesce)
		select {
		case <-c.ctx.Done():
			timer.Stop()
			return
		case <-timer.C:
		}
		// Coalesce everything that arrived before snapshot creation. Notifications
		// that arrive after this drain stay queued and force a second pass.
		for {
			select {
			case <-c.snapshotWake:
				continue
			default:
			}
			break
		}

		c.mu.RLock()
		subscriptions := make([]liveSubscription, 0, len(c.subscriptions))
		for _, subscription := range c.subscriptions {
			subscriptions = append(subscriptions, subscription)
		}
		c.mu.RUnlock()

		select {
		case c.hub.buildSlots <- struct{}{}:
		case <-c.ctx.Done():
			return
		}
		keepGoing := true
		for _, subscription := range subscriptions {
			if !c.snapshot(subscription) {
				keepGoing = false
				break
			}
		}
		<-c.hub.buildSlots
		if !keepGoing {
			return
		}
	}
}

func (c *liveClient) snapshot(subscription liveSubscription) bool {
	ctx, cancel := context.WithTimeout(c.ctx, liveSnapshotTimeout)
	defer cancel()

	data, err := c.hub.snapshot(ctx, subscription)
	if err != nil {
		if errors.Is(err, context.Canceled) && c.ctx.Err() != nil {
			return false
		}
		if !c.subscriptionCurrent(subscription) {
			return true
		}
		return c.enqueue(liveServerMessage{
			Type:            "subscription.unavailable",
			ProtocolVersion: liveProtocolVersion,
			SubscriptionID:  subscription.ID,
			Revision:        subscription.Revision,
			Stream:          subscription.Stream,
			Reason:          "temporarily_unavailable",
		})
	}
	if !c.subscriptionCurrent(subscription) {
		return true
	}
	return c.enqueue(liveServerMessage{
		Type:            "snapshot",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  subscription.ID,
		Revision:        subscription.Revision,
		Stream:          subscription.Stream,
		Data:            data,
	})
}

func (c *liveClient) subscriptionCurrent(subscription liveSubscription) bool {
	c.mu.RLock()
	defer c.mu.RUnlock()
	current, ok := c.subscriptions[subscription.ID]
	return ok && current.Revision == subscription.Revision && current.Stream == subscription.Stream
}

func (h *LiveHub) snapshot(ctx context.Context, subscription liveSubscription) (any, error) {
	switch subscription.Stream {
	case "agents":
		agents, err := loadAgentViews(ctx, h.agents, h.registry)
		if err != nil {
			return nil, err
		}
		return map[string]any{"agents": agents}, nil
	case "characters":
		if h.characters == nil {
			return nil, errors.New("character store unavailable")
		}
		items, err := h.characters.ListScoped(ctx, subscription.Filter.Query, subscription.Filter.GroupID, subscription.Filter.Server)
		if err != nil {
			return nil, err
		}
		return map[string]any{"characters": items}, nil
	case "character":
		if h.characters == nil {
			return nil, errors.New("character store unavailable")
		}
		item, err := h.characters.GetScoped(ctx, subscription.Filter.CharacterID, subscription.Filter.Server)
		if errors.Is(err, characters.ErrNotFound) {
			return map[string]any{"character": nil}, nil
		}
		if err != nil {
			return nil, err
		}
		return map[string]any{"character": item}, nil
	case "groups":
		if h.characters == nil {
			return nil, errors.New("character store unavailable")
		}
		groups, err := h.characters.GroupsScoped(ctx, subscription.Filter.Server)
		if err != nil {
			return nil, err
		}
		return map[string]any{"groups": groups}, nil
	case "commands":
		if h.commands == nil {
			return nil, errors.New("command history unavailable")
		}
		limit := subscription.Filter.Limit
		if limit == 0 {
			limit = 25
		}
		items, err := h.commands.History(ctx, subscription.Filter.CharacterID, subscription.Filter.CommandName, subscription.Filter.CommandState, limit)
		if err != nil {
			return nil, err
		}
		return map[string]any{"commands": items, "character_id": subscription.Filter.CharacterID}, nil
	case "controls":
		if h.commands == nil {
			return nil, errors.New("character controls unavailable")
		}
		return h.commands.Controls(ctx, subscription.Filter.CharacterID)
	case "resources":
		if h.resources == nil {
			return nil, errors.New("character resources unavailable")
		}
		return h.resources.Character(ctx, subscription.Filter.CharacterID, subscription.Filter.ResourceKeys...)
	case "events":
		if h.events == nil {
			return nil, errors.New("event history unavailable")
		}
		filter := events.Filter{Server: subscription.Filter.Server, CharacterID: subscription.Filter.CharacterID, CharacterQuery: subscription.Filter.Query,
			Kind: subscription.Filter.Kind, Cursor: subscription.Filter.Cursor, Limit: subscription.Filter.Limit}
		var err error
		if subscription.Filter.From != "" {
			value, parseErr := parseEventBound(subscription.Filter.From, false)
			if parseErr != nil {
				return nil, errors.New("invalid event date")
			}
			filter.From = &value
		}
		if subscription.Filter.To != "" {
			value, parseErr := parseEventBound(subscription.Filter.To, true)
			if parseErr != nil {
				return nil, errors.New("invalid event date")
			}
			filter.To = &value
		}
		page, err := h.events.List(ctx, filter)
		return page, err
	default:
		return nil, errors.New("unsupported live stream")
	}
}

func validateLiveSubscription(message liveClientMessage) (liveSubscription, bool) {
	subscription := liveSubscription{
		ID:       message.SubscriptionID,
		Revision: message.Revision,
		Stream:   message.Stream,
		Filter: liveFilter{
			Query:        strings.TrimSpace(message.Filter.Query),
			GroupID:      message.Filter.GroupID,
			CharacterID:  message.Filter.CharacterID,
			CommandName:  message.Filter.CommandName,
			CommandState: message.Filter.CommandState,
			Limit:        message.Filter.Limit,
			ResourceKeys: append([]string(nil), message.Filter.ResourceKeys...),
			Server:       strings.TrimSpace(message.Filter.Server), Kind: message.Filter.Kind,
			From: message.Filter.From, To: message.Filter.To, Cursor: message.Filter.Cursor,
		},
	}
	if !validSubscriptionID(subscription.ID) || subscription.Revision == 0 {
		return liveSubscription{}, false
	}
	switch subscription.Stream {
	case "agents":
		if subscription.Filter.Query != "" || subscription.Filter.GroupID != "" || subscription.Filter.CharacterID != "" || subscription.Filter.CommandName != "" || subscription.Filter.CommandState != "" || subscription.Filter.Limit != 0 || len(subscription.Filter.ResourceKeys) != 0 || hasEventFilters(subscription.Filter) {
			return liveSubscription{}, false
		}
	case "groups":
		if !validServerFilter(subscription.Filter.Server) || subscription.Filter.Query != "" || subscription.Filter.GroupID != "" || subscription.Filter.CharacterID != "" || subscription.Filter.CommandName != "" || subscription.Filter.CommandState != "" || subscription.Filter.Limit != 0 || len(subscription.Filter.ResourceKeys) != 0 || hasEventSpecificFilters(subscription.Filter) {
			return liveSubscription{}, false
		}
	case "characters":
		if len(subscription.Filter.Query) > 100 || !validServerFilter(subscription.Filter.Server) || subscription.Filter.CommandName != "" || subscription.Filter.CommandState != "" || subscription.Filter.Limit != 0 || len(subscription.Filter.ResourceKeys) != 0 {
			return liveSubscription{}, false
		}
		if subscription.Filter.GroupID != "" && !agentdomain.ValidAgentID(subscription.Filter.GroupID) {
			return liveSubscription{}, false
		}
		if subscription.Filter.CharacterID != "" || hasEventSpecificFilters(subscription.Filter) {
			return liveSubscription{}, false
		}
	case "character":
		if !agentdomain.ValidAgentID(subscription.Filter.CharacterID) ||
			!validServerFilter(subscription.Filter.Server) || subscription.Filter.Query != "" || subscription.Filter.GroupID != "" || subscription.Filter.CommandName != "" || subscription.Filter.CommandState != "" || subscription.Filter.Limit != 0 || len(subscription.Filter.ResourceKeys) != 0 || hasEventSpecificFilters(subscription.Filter) {
			return liveSubscription{}, false
		}
	case "commands", "controls", "resources":
		if !agentdomain.ValidAgentID(subscription.Filter.CharacterID) || subscription.Filter.Query != "" || subscription.Filter.GroupID != "" || hasEventFilters(subscription.Filter) {
			return liveSubscription{}, false
		}
		if subscription.Stream != "resources" && len(subscription.Filter.ResourceKeys) != 0 {
			return liveSubscription{}, false
		}
		if subscription.Stream == "resources" {
			if len(subscription.Filter.ResourceKeys) == 0 || len(subscription.Filter.ResourceKeys) > resources.MaxResources {
				return liveSubscription{}, false
			}
			seen := make(map[string]struct{}, len(subscription.Filter.ResourceKeys))
			for _, key := range subscription.Filter.ResourceKeys {
				if !resources.ValidResourceKey(key) {
					return liveSubscription{}, false
				}
				if _, exists := seen[key]; exists {
					return liveSubscription{}, false
				}
				seen[key] = struct{}{}
			}
		} else if subscription.Stream == "controls" && subscription.Filter.Limit != 0 {
			return liveSubscription{}, false
		}
		if subscription.Stream != "commands" && (subscription.Filter.CommandName != "" || subscription.Filter.CommandState != "") {
			return liveSubscription{}, false
		}
		if subscription.Stream == "commands" {
			if len(subscription.Filter.CommandName) > 64 || (subscription.Filter.Limit != 0 && (subscription.Filter.Limit < 1 || subscription.Filter.Limit > 100)) || len(subscription.Filter.ResourceKeys) != 0 {
				return liveSubscription{}, false
			}
			switch subscription.Filter.CommandState {
			case "", "queued", "dispatching", "sent", "acknowledged", "completed", "failed", "expired", "unknown":
			default:
				return liveSubscription{}, false
			}
		}
	case "events":
		if len(subscription.Filter.Query) > 64 || subscription.Filter.GroupID != "" || subscription.Filter.CommandName != "" || subscription.Filter.CommandState != "" || len(subscription.Filter.ResourceKeys) != 0 ||
			!validServerFilter(subscription.Filter.Server) || subscription.Filter.CharacterID != "" && !agentdomain.ValidAgentID(subscription.Filter.CharacterID) ||
			subscription.Filter.Kind != "" && subscription.Filter.Kind != events.DeathKind || len(subscription.Filter.Cursor) > 256 ||
			subscription.Filter.Limit != 0 && (subscription.Filter.Limit < 1 || subscription.Filter.Limit > events.MaxPageSize) {
			return liveSubscription{}, false
		}
		var from, to *time.Time
		for index, raw := range []string{subscription.Filter.From, subscription.Filter.To} {
			if raw != "" {
				parsed, err := parseEventBound(raw, index == 1)
				if err != nil {
					return liveSubscription{}, false
				}
				if index == 0 {
					from = &parsed
				} else {
					to = &parsed
				}
			}
		}
		if from != nil && to != nil && !to.After(*from) {
			return liveSubscription{}, false
		}
		if subscription.Filter.Cursor != "" {
			if _, err := events.DecodeCursor(subscription.Filter.Cursor); err != nil {
				return liveSubscription{}, false
			}
		}
	default:
		return liveSubscription{}, false
	}
	return subscription, true
}

func hasEventFilters(filter liveFilter) bool {
	return filter.Server != "" || hasEventSpecificFilters(filter)
}

func hasEventSpecificFilters(filter liveFilter) bool {
	return filter.Kind != "" || filter.From != "" || filter.To != "" || filter.Cursor != ""
}

func validServerFilter(server string) bool {
	return len(server) <= 100 && !strings.ContainsRune(server, 0)
}

func validSubscriptionID(value string) bool {
	if len(value) < 1 || len(value) > 64 {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '-' || r == '_' {
			continue
		}
		return false
	}
	return true
}

func liveOriginAllowed(r *http.Request) bool {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if origin == "" {
		// The same-origin Nitro relay connects server-to-server and has no browser
		// Origin header. The Go listener remains private to the deployment.
		return true
	}
	parsed, err := url.Parse(origin)
	if err != nil || parsed.User != nil || parsed.Host == "" {
		return false
	}
	if !strings.EqualFold(parsed.Host, r.Host) {
		return false
	}
	expectedScheme := "http"
	if r.TLS != nil {
		expectedScheme = "https"
	}
	if forwarded := strings.TrimSpace(strings.Split(r.Header.Get("X-Forwarded-Proto"), ",")[0]); forwarded == "http" || forwarded == "https" {
		expectedScheme = forwarded
	}
	return strings.EqualFold(parsed.Scheme, expectedScheme)
}
