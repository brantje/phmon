package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"

	agentdomain "phmon/server/internal/agents"
	authdomain "phmon/server/internal/auth"
	"phmon/server/internal/events"
)

type liveEnvelope struct {
	Type            string          `json:"type"`
	ProtocolVersion int             `json:"protocol_version"`
	SubscriptionID  string          `json:"subscription_id"`
	Revision        uint64          `json:"revision"`
	Stream          string          `json:"stream"`
	Reason          string          `json:"reason"`
	Data            json.RawMessage `json:"data"`
}

func TestLiveAgentSubscriptionInitialAndReplacementSnapshots(t *testing.T) {
	store := newFakeAgentStore()
	now := time.Now().UTC()
	protocol := agentProtocolVersion
	plugin, phbot := "1.2.3", "fixture"
	store.record = agentdomain.Record{
		AgentID:         testAgentID,
		FirstSeenAt:     &now,
		LastSeenAt:      &now,
		ProtocolVersion: &protocol,
		PluginVersion:   &plugin,
		PhBotVersion:    &phbot,
	}
	registry := agentdomain.NewRegistry()
	firstGeneration, _ := registry.Register(testAgentID)
	defer registry.Unregister(testAgentID, firstGeneration)
	live := NewLiveHub(store, registry, nil)
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		Live:     live,
	}))
	defer server.Close()

	conn := dialLive(t, server.URL, server.URL)
	defer conn.CloseNow()
	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "agents",
		Revision:        1,
		Stream:          "agents",
	})

	first := readLiveType(t, conn, "snapshot")
	if first.ProtocolVersion != liveProtocolVersion || first.SubscriptionID != "agents" || first.Revision != 1 || first.Stream != "agents" {
		t.Fatalf("unexpected first snapshot envelope: %+v", first)
	}
	var firstData struct {
		Agents []AgentView `json:"agents"`
	}
	if err := json.Unmarshal(first.Data, &firstData); err != nil {
		t.Fatal(err)
	}
	if len(firstData.Agents) != 1 || firstData.Agents[0].AgentID != testAgentID || firstData.Agents[0].ActiveConnections != 1 {
		t.Fatalf("unexpected first agent snapshot: %+v", firstData)
	}

	secondGeneration, _ := registry.Register(testAgentID)
	defer registry.Unregister(testAgentID, secondGeneration)
	live.Invalidate()
	second := readLiveType(t, conn, "snapshot")
	var secondData struct {
		Agents []AgentView `json:"agents"`
	}
	if err := json.Unmarshal(second.Data, &secondData); err != nil {
		t.Fatal(err)
	}
	if len(secondData.Agents) != 1 || secondData.Agents[0].ActiveConnections != 2 {
		t.Fatalf("live replacement did not observe registry change: %+v", secondData)
	}
}

func TestLiveProxyOriginAcceptedOnlyAfterOperatorOriginValidation(t *testing.T) {
	store := newFakeAgentStore()
	now := time.Now().UTC()
	store.record = agentdomain.Record{AgentID: testAgentID, FirstSeenAt: &now, LastSeenAt: &now}
	registry := agentdomain.NewRegistry()
	live := NewLiveHub(store, registry, nil)
	manager, err := authdomain.New("0123456789abcdef0123456789abcdef", "phmon_operator", []string{"http://dashboard.example.test:3005"}, true)
	if err != nil {
		t.Fatal(err)
	}
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Auth:     manager,
		Agents:   store,
		Registry: registry,
		Live:     live,
	}))
	defer server.Close()
	token, _, err := manager.CreateSession("0123456789abcdef0123456789abcdef", "127.0.0.1:1234", now)
	if err != nil {
		t.Fatal(err)
	}

	dial := func(origin string) (*websocket.Conn, *http.Response, error) {
		headers := http.Header{
			"Origin": {origin},
			"Cookie": {"phmon_operator=" + token},
		}
		ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
		defer cancel()
		return websocket.Dial(ctx, liveWSURL(server.URL), &websocket.DialOptions{HTTPHeader: headers})
	}
	conn, response, err := dial("http://dashboard.example.test:3005")
	if err != nil {
		if response != nil {
			t.Fatalf("authorized proxied origin rejected: HTTP %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	defer conn.CloseNow()
	writeLive(t, conn, liveClientMessage{Type: "subscribe", ProtocolVersion: liveProtocolVersion, SubscriptionID: "agents", Revision: 1, Stream: "agents"})
	first := readLiveType(t, conn, "snapshot")
	var data struct {
		Agents []AgentView `json:"agents"`
	}
	if err := json.Unmarshal(first.Data, &data); err != nil || len(data.Agents) != 1 {
		t.Fatalf("authorized proxy received invalid agent snapshot: count=%d err=%v", len(data.Agents), err)
	}

	_, response, err = dial("http://evil.example.test")
	if err == nil || response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("unauthorized origin response=%v err=%v", response, err)
	}
}

type blockingListAgentStore struct {
	*fakeAgentStore
	mu           sync.Mutex
	calls        int
	first        []agentdomain.Record
	second       []agentdomain.Record
	firstStarted chan struct{}
	releaseFirst chan struct{}
}

type concurrentListAgentStore struct {
	*fakeAgentStore
	mu      sync.Mutex
	active  int
	max     int
	started chan struct{}
	release chan struct{}
}

func (s *concurrentListAgentStore) ListSeen(ctx context.Context) ([]agentdomain.Record, error) {
	s.mu.Lock()
	s.active++
	if s.active > s.max {
		s.max = s.active
	}
	s.mu.Unlock()
	defer func() {
		s.mu.Lock()
		s.active--
		s.mu.Unlock()
	}()

	select {
	case s.started <- struct{}{}:
	case <-ctx.Done():
		return nil, ctx.Err()
	}
	select {
	case <-s.release:
		return nil, nil
	case <-ctx.Done():
		return nil, ctx.Err()
	}
}

func (s *concurrentListAgentStore) maxActive() int {
	s.mu.Lock()
	defer s.mu.Unlock()
	return s.max
}

func (s *blockingListAgentStore) ListSeen(ctx context.Context) ([]agentdomain.Record, error) {
	s.mu.Lock()
	s.calls++
	call := s.calls
	s.mu.Unlock()
	if call == 1 {
		close(s.firstStarted)
		select {
		case <-ctx.Done():
			return nil, ctx.Err()
		case <-s.releaseFirst:
		}
		return s.first, nil
	}
	return s.second, nil
}

func TestLiveSnapshotBuildConcurrencyIsBounded(t *testing.T) {
	store := &concurrentListAgentStore{
		fakeAgentStore: newFakeAgentStore(),
		started:        make(chan struct{}, 3),
		release:        make(chan struct{}),
	}
	registry := agentdomain.NewRegistry()
	live := NewLiveHub(store, registry, nil)
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		Live:     live,
	}))
	defer server.Close()

	connections := make([]*websocket.Conn, 0, 3)
	defer func() {
		for _, conn := range connections {
			conn.CloseNow()
		}
	}()
	for i := 0; i < 3; i++ {
		conn := dialLive(t, server.URL, server.URL)
		connections = append(connections, conn)
		writeLive(t, conn, liveClientMessage{
			Type:            "subscribe",
			ProtocolVersion: liveProtocolVersion,
			SubscriptionID:  "agents",
			Revision:        1,
			Stream:          "agents",
		})
	}

	for i := 0; i < liveMaxConcurrentBuilds; i++ {
		select {
		case <-store.started:
		case <-time.After(2 * time.Second):
			t.Fatal("snapshot build did not start")
		}
	}
	select {
	case <-store.started:
		t.Fatal("snapshot concurrency exceeded liveMaxConcurrentBuilds")
	case <-time.After(150 * time.Millisecond):
	}
	if got := store.maxActive(); got != liveMaxConcurrentBuilds {
		t.Fatalf("max concurrent snapshot builds = %d, want %d", got, liveMaxConcurrentBuilds)
	}

	close(store.release)
	for _, conn := range connections {
		readLiveType(t, conn, "snapshot")
	}
}

func TestLiveInvalidationDuringSnapshotIsNotLost(t *testing.T) {
	firstPlugin, secondPlugin := "before", "after"
	store := &blockingListAgentStore{
		fakeAgentStore: newFakeAgentStore(),
		first:          []agentdomain.Record{{AgentID: testAgentID, PluginVersion: &firstPlugin}},
		second:         []agentdomain.Record{{AgentID: testAgentID, PluginVersion: &secondPlugin}},
		firstStarted:   make(chan struct{}),
		releaseFirst:   make(chan struct{}),
	}
	registry := agentdomain.NewRegistry()
	live := NewLiveHub(store, registry, nil)
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		Live:     live,
	}))
	defer server.Close()

	conn := dialLive(t, server.URL, server.URL)
	defer conn.CloseNow()
	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "agents",
		Revision:        1,
		Stream:          "agents",
	})

	select {
	case <-store.firstStarted:
	case <-time.After(time.Second):
		t.Fatal("first snapshot did not start")
	}
	live.Invalidate()
	close(store.releaseFirst)

	first := readLiveType(t, conn, "snapshot")
	second := readLiveType(t, conn, "snapshot")
	var firstData, secondData struct {
		Agents []AgentView `json:"agents"`
	}
	if err := json.Unmarshal(first.Data, &firstData); err != nil {
		t.Fatal(err)
	}
	if err := json.Unmarshal(second.Data, &secondData); err != nil {
		t.Fatal(err)
	}
	if len(firstData.Agents) != 1 || firstData.Agents[0].PluginVersion == nil || *firstData.Agents[0].PluginVersion != firstPlugin {
		t.Fatalf("unexpected first snapshot: %+v", firstData)
	}
	if len(secondData.Agents) != 1 || secondData.Agents[0].PluginVersion == nil || *secondData.Agents[0].PluginVersion != secondPlugin {
		t.Fatalf("invalidation during snapshot was lost: %+v", secondData)
	}
}

func TestLiveRejectsObsoleteSubscriptionRevision(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
	}))
	defer server.Close()

	conn := dialLive(t, server.URL, server.URL)
	defer conn.CloseNow()
	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "agent-list",
		Revision:        2,
		Stream:          "agents",
	})
	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "agent-list",
		Revision:        1,
		Stream:          "agents",
	})

	rejected := readLiveType(t, conn, "subscription.rejected")
	if rejected.SubscriptionID != "agent-list" || rejected.Revision != 1 || rejected.Reason != "obsolete_revision" {
		t.Fatalf("unexpected rejection: %+v", rejected)
	}
}

func TestLiveRevisionRemainsMonotonicAcrossUnsubscribe(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	live := NewLiveHub(store, registry, nil)
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		Live:     live,
	}))
	defer server.Close()

	conn := dialLive(t, server.URL, server.URL)
	defer conn.CloseNow()
	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "detail",
		Revision:        1,
		Stream:          "agents",
	})
	writeLive(t, conn, liveClientMessage{
		Type:            "unsubscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "detail",
		Revision:        1,
	})
	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "detail",
		Revision:        1,
		Stream:          "agents",
	})

	rejected := readLiveType(t, conn, "subscription.rejected")
	if rejected.SubscriptionID != "detail" || rejected.Revision != 1 || rejected.Reason != "obsolete_revision" {
		t.Fatalf("unexpected recycled revision response: %+v", rejected)
	}

	writeLive(t, conn, liveClientMessage{
		Type:            "subscribe",
		ProtocolVersion: liveProtocolVersion,
		SubscriptionID:  "detail",
		Revision:        2,
		Stream:          "agents",
	})
	snapshot := readLiveType(t, conn, "snapshot")
	if snapshot.SubscriptionID != "detail" || snapshot.Revision != 2 {
		t.Fatalf("newer revision was not accepted: %+v", snapshot)
	}
}

func TestLiveRejectsCrossOriginBrowserUpgrade(t *testing.T) {
	store := newFakeAgentStore()
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: agentdomain.NewRegistry(),
	}))
	defer server.Close()

	headers := http.Header{}
	headers.Set("Origin", "https://evil.example")
	_, response, err := websocket.Dial(context.Background(), liveWSURL(server.URL), &websocket.DialOptions{HTTPHeader: headers})
	if err == nil {
		t.Fatal("cross-origin live WebSocket unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusForbidden {
		t.Fatalf("unexpected cross-origin response: %#v err=%v", response, err)
	}
}

func TestLiveMalformedFrameClosesConnection(t *testing.T) {
	store := newFakeAgentStore()
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: agentdomain.NewRegistry(),
	}))
	defer server.Close()

	conn := dialLive(t, server.URL, server.URL)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, []byte(`{"type":"subscribe"} trailing`)); err != nil {
		t.Fatal(err)
	}
	_, _, err := conn.Read(ctx)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("malformed frame should close with policy violation: err=%v status=%v", err, websocket.CloseStatus(err))
	}
}

func TestLiveSlowConsumerQueueIsBounded(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	client := &liveClient{
		ctx:      ctx,
		cancel:   cancel,
		outgoing: make(chan []byte, 1),
	}
	defer cancel()
	if !client.enqueue(liveServerMessage{Type: "snapshot", ProtocolVersion: liveProtocolVersion}) {
		t.Fatal("first frame should fit bounded queue")
	}
	if client.enqueue(liveServerMessage{Type: "snapshot", ProtocolVersion: liveProtocolVersion}) {
		t.Fatal("second frame unexpectedly fit full queue")
	}
	select {
	case <-ctx.Done():
	default:
		t.Fatal("slow consumer did not cancel its connection")
	}
}

func TestLiveSubscriptionValidation(t *testing.T) {
	validCharacterID := "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	for name, tc := range map[string]struct {
		message liveClientMessage
		valid   bool
	}{
		"agent": {
			message: liveClientMessage{SubscriptionID: "agents", Revision: 1, Stream: "agents"},
			valid:   true,
		},
		"filtered characters": {
			message: liveClientMessage{SubscriptionID: "characters", Revision: 3, Stream: "characters", Filter: liveFilter{Query: "alpha", GroupID: validCharacterID}},
			valid:   true,
		},
		"server scoped characters": {
			message: liveClientMessage{SubscriptionID: "characters", Revision: 1, Stream: "characters", Filter: liveFilter{Server: "Servar"}},
			valid:   true,
		},
		"server scoped groups": {
			message: liveClientMessage{SubscriptionID: "groups", Revision: 1, Stream: "groups", Filter: liveFilter{Server: "Servar"}},
			valid:   true,
		},
		"detail": {
			message: liveClientMessage{SubscriptionID: "detail", Revision: 1, Stream: "character", Filter: liveFilter{CharacterID: validCharacterID}},
			valid:   true,
		},
		"server scoped detail": {
			message: liveClientMessage{SubscriptionID: "detail", Revision: 2, Stream: "character", Filter: liveFilter{CharacterID: validCharacterID, Server: "Servar"}},
			valid:   true,
		},
		"agents reject server filter": {
			message: liveClientMessage{SubscriptionID: "agents", Revision: 1, Stream: "agents", Filter: liveFilter{Server: "Servar"}},
			valid:   false,
		},
		"oversized character server filter": {
			message: liveClientMessage{SubscriptionID: "characters", Revision: 1, Stream: "characters", Filter: liveFilter{Server: strings.Repeat("x", 101)}},
			valid:   false,
		},
		"command history": {
			message: liveClientMessage{SubscriptionID: "commands", Revision: 1, Stream: "commands", Filter: liveFilter{CharacterID: validCharacterID, CommandName: "bot.stop", CommandState: "completed", Limit: 25}}, valid: true,
		},
		"character controls": {
			message: liveClientMessage{SubscriptionID: "controls", Revision: 1, Stream: "controls", Filter: liveFilter{CharacterID: validCharacterID}}, valid: true,
		},
		"invalid command limit": {
			message: liveClientMessage{SubscriptionID: "commands", Revision: 1, Stream: "commands", Filter: liveFilter{CharacterID: validCharacterID, Limit: 101}}, valid: false,
		},
		"groups": {
			message: liveClientMessage{SubscriptionID: "groups", Revision: 1, Stream: "groups"},
			valid:   true,
		},
		"bad stream": {
			message: liveClientMessage{SubscriptionID: "x", Revision: 1, Stream: "nonsense"},
			valid:   false,
		},
		"death events": {
			message: liveClientMessage{SubscriptionID: "events", Revision: 1, Stream: "events", Filter: liveFilter{Server: "Example", Kind: events.DeathKind, From: "2026-09-01", To: "2026-09-28", Limit: 25}},
			valid:   true,
		},
		"invalid event date": {
			message: liveClientMessage{SubscriptionID: "events", Revision: 1, Stream: "events", Filter: liveFilter{From: "yesterday"}},
			valid:   false,
		},
		"bad group": {
			message: liveClientMessage{SubscriptionID: "x", Revision: 1, Stream: "characters", Filter: liveFilter{GroupID: "not-a-uuid"}},
			valid:   false,
		},
		"detail without identity": {
			message: liveClientMessage{SubscriptionID: "x", Revision: 1, Stream: "character"},
			valid:   false,
		},
	} {
		t.Run(name, func(t *testing.T) {
			_, ok := validateLiveSubscription(tc.message)
			if ok != tc.valid {
				t.Fatalf("valid=%v want=%v message=%+v", ok, tc.valid, tc.message)
			}
		})
	}
}

func dialLive(t *testing.T, baseURL, origin string) *websocket.Conn {
	t.Helper()
	headers := http.Header{}
	if origin != "" {
		headers.Set("Origin", origin)
	}
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	conn, response, err := websocket.Dial(ctx, liveWSURL(baseURL), &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		if response != nil {
			t.Fatalf("live dial failed: HTTP %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	return conn
}

func writeLive(t *testing.T, conn *websocket.Conn, message liveClientMessage) {
	t.Helper()
	payload, err := json.Marshal(message)
	if err != nil {
		t.Fatal(err)
	}
	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	if err := conn.Write(ctx, websocket.MessageText, payload); err != nil {
		t.Fatal(err)
	}
}

func readLiveType(t *testing.T, conn *websocket.Conn, want string) liveEnvelope {
	t.Helper()
	deadline := time.Now().Add(2 * time.Second)
	for time.Now().Before(deadline) {
		ctx, cancel := context.WithTimeout(context.Background(), time.Until(deadline))
		messageType, payload, err := conn.Read(ctx)
		cancel()
		if err != nil {
			t.Fatal(err)
		}
		if messageType != websocket.MessageText {
			continue
		}
		var envelope liveEnvelope
		if err := json.Unmarshal(payload, &envelope); err != nil {
			t.Fatalf("decode live envelope: %v payload=%s", err, payload)
		}
		if envelope.Type == "heartbeat" {
			writeLive(t, conn, liveClientMessage{Type: "heartbeat", ProtocolVersion: liveProtocolVersion})
			continue
		}
		if envelope.Type == want {
			return envelope
		}
	}
	t.Fatalf("did not receive live message type %q", want)
	return liveEnvelope{}
}

func liveWSURL(baseURL string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + "/api/live"
}
