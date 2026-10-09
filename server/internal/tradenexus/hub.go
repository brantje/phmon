package tradenexus

import (
	"context"
	"encoding/json"
	"log/slog"
	"net/http"
	"strings"
	"sync"
	"time"

	"net"

	"github.com/coder/websocket"
)

type Recorder interface {
	Insert(context.Context, Sighting) error
	Latest(context.Context, string, string) (Sighting, bool, error)
}

type TradeRecorder interface {
	InsertTrade(context.Context, TradeReport) (TradeReport, error)
}

type Options struct {
	Store            Recorder
	Trades           TradeRecorder
	Invalidate       func()
	Now              func() time.Time
	MaxConnections   int
	MaxPerIP         int
	MaxServers       int
	ReportsPerWindow int
	SendQueue        int
	MaxInvalidFrames int
}

type outbound struct {
	payload []byte
	final   bool
}

type Hub struct {
	store            Recorder
	trades           TradeRecorder
	invalidate       func()
	now              func() time.Time
	maxConnections   int
	maxPerIP         int
	maxServers       int
	reportsPerWindow int
	sendQueue        int
	maxInvalidFrames int

	mu       sync.Mutex
	clients  map[*client]struct{}
	perIP    map[string]int
	admitted int

	invalidateMu    sync.Mutex
	lastInvalidate  time.Time
	invalidateTimer *time.Timer

	active *activeCache
}

type client struct {
	hub     *Hub
	conn    *websocket.Conn
	ip      string
	send    chan outbound
	closed  chan struct{}
	once    sync.Once
	mu      sync.Mutex
	closing bool
	servers map[string]struct{}
	reports []time.Time
}

func NewHub(options Options) *Hub {
	if options.MaxConnections <= 0 {
		options.MaxConnections = MaxConnections
	}
	if options.MaxPerIP <= 0 {
		options.MaxPerIP = MaxConnectionsIP
	}
	if options.MaxServers <= 0 {
		options.MaxServers = MaxServers
	}
	if options.ReportsPerWindow <= 0 {
		options.ReportsPerWindow = ReportsPerWindow
	}
	if options.SendQueue <= 0 {
		options.SendQueue = SendQueue
	}
	if options.MaxInvalidFrames <= 0 {
		options.MaxInvalidFrames = MaxInvalidFrames
	}
	return &Hub{
		store: options.Store, trades: options.Trades, invalidate: options.Invalidate, now: options.Now,
		maxConnections: options.MaxConnections, maxPerIP: options.MaxPerIP,
		maxServers: options.MaxServers, reportsPerWindow: options.ReportsPerWindow,
		sendQueue: options.SendQueue, maxInvalidFrames: options.MaxInvalidFrames,
		clients: map[*client]struct{}{}, perIP: map[string]int{},
		active: newActiveCache(),
	}
}

func (h *Hub) clock() time.Time {
	if h.now != nil {
		return h.now().UTC()
	}
	return time.Now().UTC()
}

// Record stores a new sighting. A repeat of the same server and thief within
// EncounterGap of the latest stored row is not inserted, including when the
// region changes. The returned sighting is the stored row, and fresh is false
// so the caller does not broadcast. The in-memory pin still moves to the
// repeat's coordinates.
func (h *Hub) Record(ctx context.Context, sighting Sighting) (stored Sighting, fresh bool, err error) {
	prepared, err := h.prepare(sighting)
	if err != nil {
		return Sighting{}, false, err
	}
	if h.store != nil {
		previous, found, lookupErr := h.store.Latest(ctx, prepared.Server, prepared.ThiefName)
		if lookupErr != nil {
			slog.Warn("tradenexus repeat check failed", "server", prepared.Server, "reason", lookupErr.Error())
		} else if found && sameOpenEncounter(previous, prepared) {
			moved := previous
			moved.Position = clonePosition(prepared.Position)
			if prepared.Position != nil {
				moved.PositionSource = prepared.PositionSource
			}
			h.active.remember(h.clock(), moved)
			return previous, false, nil
		}
		if err := h.store.Insert(ctx, prepared); err != nil {
			slog.Warn("tradenexus sighting was not stored", "server", prepared.Server, "reason", err.Error())
		}
	}
	// Remember before any subscriber ack so subscribe snapshots cannot race the report handler.
	h.active.remember(h.clock(), prepared)
	return prepared, true, nil
}

// sameOpenEncounter reports whether next continues the stored sighting.
// A gap of exactly EncounterGap still continues it. Region is ignored.
func sameOpenEncounter(previous, next Sighting) bool {
	if previous.ID == "" || previous.ReceivedAt.IsZero() || next.ReceivedAt.IsZero() {
		return false
	}
	gap := next.ReceivedAt.Sub(previous.ReceivedAt)
	return gap >= 0 && gap <= EncounterGap
}

func clonePosition(position *Position) *Position {
	if position == nil {
		return nil
	}
	copied := *position
	if position.Z != nil {
		z := *position.Z
		copied.Z = &z
	}
	return &copied
}

func (h *Hub) prepare(sighting Sighting) (Sighting, error) {
	if sighting.ID == "" {
		id, err := newSightingID()
		if err != nil {
			return Sighting{}, err
		}
		sighting.ID = id
	}
	now := h.clock()
	if sighting.ReceivedAt.IsZero() {
		sighting.ReceivedAt = now
	}
	if sighting.ObservedAt.IsZero() || sighting.ObservedAt.After(now.Add(ObservedAtSkew)) || sighting.ObservedAt.Before(now.Add(-ObservedAtSkew)) {
		sighting.ObservedAt = sighting.ReceivedAt
	}
	server, ok := validServer(sighting.Server)
	if !ok {
		return Sighting{}, errInvalidSighting
	}
	name, ok := requiredText(sighting.ThiefName, MaxNameBytes)
	if !ok {
		return Sighting{}, errInvalidSighting
	}
	sighting.Server = server
	sighting.ThiefName = name
	if sighting.Origin == "" {
		sighting.Origin = OriginExternal
	}
	if sighting.Position == nil {
		if sighting.PositionSource == "" || sighting.PositionSource == SourceThief {
			sighting.PositionSource = SourceUnknown
		}
	} else if sighting.PositionSource == "" {
		sighting.PositionSource = SourceUnknown
	}
	if sighting.Origin != OriginPhMon && sighting.Origin != OriginExternal {
		return Sighting{}, errInvalidSighting
	}
	if sighting.PositionSource != SourceThief && sighting.PositionSource != SourceObserver && sighting.PositionSource != SourceUnknown {
		return Sighting{}, errInvalidSighting
	}
	if sighting.Position != nil {
		if !validRegion(sighting.Position.Region) || !finiteCoord(sighting.Position.X) || !finiteCoord(sighting.Position.Y) {
			return Sighting{}, errInvalidSighting
		}
		if sighting.Position.Z != nil && !finiteCoord(*sighting.Position.Z) {
			return Sighting{}, errInvalidSighting
		}
	}
	return sighting, nil
}

func (h *Hub) RecordTrade(ctx context.Context, report TradeReport) (TradeReport, error) {
	if err := report.valid(); err != nil {
		return TradeReport{}, err
	}
	if report.ID == "" {
		id, err := newSightingID()
		if err != nil {
			return TradeReport{}, err
		}
		report.ID = id
	}
	now := h.clock()
	if report.ReceivedAt.IsZero() {
		report.ReceivedAt = now
	}
	if report.FinishedAt.IsZero() || report.FinishedAt.After(now.Add(ObservedAtSkew)) || report.FinishedAt.Before(now.Add(-ObservedAtSkew)) {
		report.FinishedAt = report.ReceivedAt
	}
	if h.trades == nil {
		return report, nil
	}
	stored, err := h.trades.InsertTrade(ctx, report)
	if err != nil {
		slog.Warn("tradenexus trade report was not stored", "server", report.Server, "reason", err.Error())
		return TradeReport{}, err
	}
	return stored, nil
}

func (h *Hub) Broadcast(sighting Sighting) {
	h.active.remember(h.clock(), sighting)
	payload, err := marshalSighting(sighting)
	if err != nil {
		slog.Warn("tradenexus sighting could not be encoded", "reason", err.Error())
		return
	}
	h.scheduleInvalidate()
	for _, subscriber := range h.subscribers(sighting.Server) {
		h.enqueue(subscriber, outbound{payload: payload})
	}
}

func (h *Hub) pushActiveThieves(subscriber *client, servers []string) {
	sightings, truncated := h.active.snapshot(h.clock(), servers)
	payload, err := marshalThievesSnapshot(sightings, truncated)
	if err != nil {
		slog.Warn("tradenexus thieves snapshot could not be encoded", "reason", err.Error())
		return
	}
	h.enqueue(subscriber, outbound{payload: payload})
}

// scheduleInvalidate rebuilds operator snapshots at most once per second.
// A burst keeps one trailing refresh so the latest sighting still appears.
func (h *Hub) scheduleInvalidate() {
	if h == nil || h.invalidate == nil {
		return
	}
	h.invalidateMu.Lock()
	now := time.Now()
	if h.invalidateTimer != nil {
		h.invalidateMu.Unlock()
		return
	}
	if h.lastInvalidate.IsZero() || now.Sub(h.lastInvalidate) >= time.Second {
		h.lastInvalidate = now
		refresh := h.invalidate
		h.invalidateMu.Unlock()
		refresh()
		return
	}
	delay := time.Second - now.Sub(h.lastInvalidate)
	h.invalidateTimer = time.AfterFunc(delay, func() {
		h.invalidateMu.Lock()
		h.lastInvalidate = time.Now()
		h.invalidateTimer = nil
		refresh := h.invalidate
		h.invalidateMu.Unlock()
		if refresh != nil {
			refresh()
		}
	})
	h.invalidateMu.Unlock()
}

func (h *Hub) subscribers(server string) []*client {
	h.mu.Lock()
	current := make([]*client, 0, len(h.clients))
	for subscriber := range h.clients {
		current = append(current, subscriber)
	}
	h.mu.Unlock()
	matched := make([]*client, 0, len(current))
	for _, subscriber := range current {
		if subscriber.wants(server) {
			matched = append(matched, subscriber)
		}
	}
	return matched
}

func (h *Hub) tryAdmit(ip string) bool {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.admitted >= h.maxConnections || h.perIP[ip] >= h.maxPerIP {
		return false
	}
	h.admitted++
	h.perIP[ip]++
	return true
}

func (h *Hub) release(ip string) {
	h.mu.Lock()
	defer h.mu.Unlock()
	if h.admitted > 0 {
		h.admitted--
	}
	if h.perIP[ip] <= 1 {
		delete(h.perIP, ip)
		return
	}
	h.perIP[ip]--
}

func (h *Hub) add(subscriber *client) {
	h.mu.Lock()
	h.clients[subscriber] = struct{}{}
	h.mu.Unlock()
}

func (h *Hub) remove(subscriber *client) {
	h.mu.Lock()
	delete(h.clients, subscriber)
	h.mu.Unlock()
}

func (h *Hub) enqueue(subscriber *client, message outbound) {
	subscriber.mu.Lock()
	if subscriber.closing {
		subscriber.mu.Unlock()
		return
	}
	select {
	case subscriber.send <- message:
		subscriber.mu.Unlock()
	default:
		subscriber.closing = true
		subscriber.mu.Unlock()
		subscriber.close()
	}
}

func (c *client) close() {
	c.once.Do(func() {
		c.mu.Lock()
		c.closing = true
		c.mu.Unlock()
		close(c.closed)
		c.hub.remove(c)
		if c.conn != nil {
			_ = c.conn.Close(websocket.StatusGoingAway, "closed")
		}
	})
}

func (c *client) wants(server string) bool {
	c.mu.Lock()
	defer c.mu.Unlock()
	for name := range c.servers {
		if strings.EqualFold(name, server) {
			return true
		}
	}
	return false
}

func (c *client) setServers(servers []string) {
	next := make(map[string]struct{}, len(servers))
	for _, server := range servers {
		next[activeServerKey(server)] = struct{}{}
	}
	c.mu.Lock()
	c.servers = next
	c.mu.Unlock()
}

func (c *client) allowReport(now time.Time) bool {
	cutoff := now.Add(-ReportWindow)
	kept := c.reports[:0]
	for _, stamp := range c.reports {
		if stamp.After(cutoff) {
			kept = append(kept, stamp)
		}
	}
	c.reports = kept
	if len(c.reports) >= c.hub.reportsPerWindow {
		return false
	}
	c.reports = append(c.reports, now)
	return true
}

func (h *Hub) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet {
		w.Header().Set("Allow", http.MethodGet)
		http.Error(w, "method not allowed", http.StatusMethodNotAllowed)
		return
	}
	ip := remoteIP(r)
	if !h.tryAdmit(ip) {
		w.Header().Set("Content-Type", "application/json")
		w.Header().Set("Cache-Control", "no-store")
		w.WriteHeader(http.StatusTooManyRequests)
		_ = json.NewEncoder(w).Encode(map[string]string{"error": "too many connections"})
		return
	}
	// TradeNexus has no cookies and accepts non-browser bots, so the browser
	// Origin check would reject the clients this route exists for.
	conn, err := websocket.Accept(w, r, &websocket.AcceptOptions{
		CompressionMode:    websocket.CompressionDisabled,
		InsecureSkipVerify: true,
	})
	if err != nil {
		h.release(ip)
		return
	}
	conn.SetReadLimit(ReadLimitBytes)
	subscriber := &client{
		hub: h, conn: conn, ip: ip,
		send: make(chan outbound, h.sendQueue), closed: make(chan struct{}),
		servers: map[string]struct{}{},
	}
	defer h.release(ip)
	defer subscriber.close()
	h.add(subscriber)
	go subscriber.write()
	go subscriber.ping()
	hello, _ := json.Marshal(map[string]any{
		"v": ProtocolVersion, "type": "hello", "server_time": h.clock().Format(time.RFC3339),
		"limits": map[string]int{
			"max_frame_bytes":       MaxFrameBytes,
			"max_trade_frame_bytes": MaxTradeFrameBytes,
			"max_servers":           h.maxServers,
			"reports_per_5s":        h.reportsPerWindow,
		},
	})
	h.enqueue(subscriber, outbound{payload: hello})
	h.read(r.Context(), subscriber)
}

func remoteIP(r *http.Request) string {
	host, _, err := net.SplitHostPort(r.RemoteAddr)
	if err == nil && host != "" {
		return host
	}
	if r.RemoteAddr == "" {
		return "unknown"
	}
	return r.RemoteAddr
}

func (c *client) write() {
	for {
		select {
		case <-c.closed:
			return
		case message := <-c.send:
			ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			err := c.conn.Write(ctx, websocket.MessageText, message.payload)
			cancel()
			if err != nil || message.final {
				c.close()
				return
			}
		}
	}
}

func (c *client) ping() {
	ticker := time.NewTicker(30 * time.Second)
	defer ticker.Stop()
	for {
		select {
		case <-c.closed:
			return
		case <-ticker.C:
			ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
			err := c.conn.Ping(ctx)
			cancel()
			if err != nil {
				c.close()
				return
			}
		}
	}
}

func (h *Hub) read(ctx context.Context, subscriber *client) {
	invalid := 0
	for {
		_, payload, err := subscriber.conn.Read(ctx)
		if err != nil {
			return
		}
		if len(payload) > MaxTradeFrameBytes {
			h.fail(subscriber, parseError{Code: "too_large", Message: "frame exceeds 16384 bytes"}, &invalid)
			continue
		}
		frame, parseErr := parseClientFrame(payload)
		if parseErr.Code != "" {
			h.fail(subscriber, parseErr, &invalid)
			continue
		}
		switch frame.Type {
		case "subscribe":
			servers, subscribeErr := subscribeServers(frame)
			if subscribeErr.Code != "" || len(servers) > h.maxServers {
				if subscribeErr.Code == "" {
					subscribeErr = parseError{Code: "invalid_message", Message: serverLimitMessage(), Ref: frame.Ref}
				}
				h.fail(subscriber, subscribeErr, &invalid)
				continue
			}
			invalid = 0
			subscriber.setServers(servers)
			body, _ := json.Marshal(map[string]any{"v": ProtocolVersion, "type": "subscribed", "servers": servers})
			h.enqueue(subscriber, outbound{payload: body})
			h.pushActiveThieves(subscriber, servers)
		case "thief.report":
			if !subscriber.allowReport(h.clock()) {
				invalid = 0
				body := errorFrame(frame.Ref, "rate_limited", "too many reports")
				h.enqueue(subscriber, outbound{payload: body})
				continue
			}
			sighting, reportErr := reportSighting(frame, h.clock())
			if reportErr.Code != "" {
				h.fail(subscriber, reportErr, &invalid)
				continue
			}
			recorded, fresh, err := h.Record(ctx, sighting)
			if err != nil {
				h.fail(subscriber, parseError{Code: "invalid_message", Message: "sighting could not be accepted", Ref: frame.Ref}, &invalid)
				continue
			}
			invalid = 0
			// Broadcast before the ack is queued. Otherwise the sender can
			// observe the ack and connect another subscriber before the
			// sighting is delivered, and that new subscriber receives it.
			if fresh {
				h.Broadcast(recorded)
			}
			h.enqueue(subscriber, outbound{payload: ackFrame(frame.Ref, recorded.ID)})
		case "trade.report":
			if !subscriber.allowReport(h.clock()) {
				invalid = 0
				body := errorFrame(frame.Ref, "rate_limited", "too many reports")
				h.enqueue(subscriber, outbound{payload: body})
				continue
			}
			trade, tradeErr := reportTrade(payload, h.clock())
			if tradeErr.Code != "" {
				h.fail(subscriber, tradeErr, &invalid)
				continue
			}
			storedTrade, tradeStoreErr := h.RecordTrade(ctx, trade)
			if tradeStoreErr != nil {
				h.fail(subscriber, parseError{Code: "invalid_message", Message: "trade could not be accepted", Ref: frame.Ref}, &invalid)
				continue
			}
			invalid = 0
			h.enqueue(subscriber, outbound{payload: tradeAckFrame(frame.Ref, storedTrade.ID)})
		}
	}
}

func (h *Hub) fail(subscriber *client, failure parseError, invalid *int) {
	*invalid++
	h.enqueue(subscriber, outbound{
		payload: errorFrame(failure.Ref, failure.Code, failure.Message),
		final:   *invalid >= h.maxInvalidFrames,
	})
}
