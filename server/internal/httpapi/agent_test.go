package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"net/http"
	"net/http/httptest"
	"strings"
	"sync"
	"testing"
	"time"

	"github.com/coder/websocket"
	"github.com/coder/websocket/wsjson"
	agentdomain "phmon/server/internal/agents"
)

const testAgentID = "11111111-2222-4333-8444-555555555555"

type fakeAgentStore struct {
	mu               sync.Mutex
	token            string
	agentID          string
	record           agentdomain.Record
	seenCount        int
	disconnectedCount int
}

func newFakeAgentStore() *fakeAgentStore {
	return &fakeAgentStore{token: "phm_test_token", agentID: testAgentID}
}

func (s *fakeAgentStore) AuthenticateToken(_ context.Context, token string) (string, error) {
	if token != s.token {
		return "", agentdomain.ErrInvalidToken
	}
	return s.agentID, nil
}

func (s *fakeAgentStore) MarkConnected(_ context.Context, agentID string, protocol int, plugin, phbot string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	now := time.Now().UTC()
	s.record.AgentID = agentID
	s.record.FirstSeenAt = &now
	s.record.LastSeenAt = &now
	s.record.LastConnectedAt = &now
	s.record.ProtocolVersion = &protocol
	s.record.PluginVersion = &plugin
	s.record.PhBotVersion = &phbot
	return nil
}

func (s *fakeAgentStore) MarkSeen(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.seenCount++
	now := time.Now().UTC()
	s.record.LastSeenAt = &now
	return nil
}

func (s *fakeAgentStore) MarkDisconnected(_ context.Context, _ string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.disconnectedCount++
	now := time.Now().UTC()
	s.record.LastDisconnectedAt = &now
	return nil
}

func (s *fakeAgentStore) ListSeen(context.Context) ([]agentdomain.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.FirstSeenAt == nil {
		return []agentdomain.Record{}, nil
	}
	return []agentdomain.Record{s.record}, nil
}

func TestAgentAuthenticationIsEnforced(t *testing.T) {
	store := newFakeAgentStore()
	server := newAgentTestServer(t, store, AgentOptions{})
	defer server.Close()

	headers := http.Header{}
	headers.Set("Authorization", "Bearer wrong")
	_, response, err := websocket.Dial(context.Background(), wsURL(server.URL), &websocket.DialOptions{HTTPHeader: headers})
	if err == nil {
		t.Fatal("invalid token unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusUnauthorized {
		t.Fatalf("unexpected response: %#v err=%v", response, err)
	}
}

func TestAgentHelloHeartbeatAndList(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		AgentOptions: AgentOptions{
			HelloTimeout:      time.Second,
			HeartbeatInterval: 50 * time.Millisecond,
			HeartbeatTimeout:  300 * time.Millisecond,
		},
	}))
	defer server.Close()

	conn := dialAgent(t, server.URL, store.token)
	writeHello(t, conn, testAgentID)
	var ack helloAck
	if err := wsjson.Read(context.Background(), conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.Type != "hello.ack" || ack.ProtocolVersion != 1 {
		t.Fatalf("unexpected ack: %+v", ack)
	}
	if _, ok := registry.ConnectedAt(testAgentID); !ok {
		t.Fatal("agent was not registered")
	}

	if err := wsjson.Write(context.Background(), conn, agentMessage{
		Type:            "heartbeat",
		ProtocolVersion: 1,
		SentAt:          time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}

	response, err := http.Get(server.URL + "/api/agents")
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var agents []AgentView
	if err := json.NewDecoder(response.Body).Decode(&agents); err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents[0].AgentID != testAgentID || !agents[0].Connected || agents[0].ConnectedAt == nil {
		t.Fatalf("unexpected agent list: %+v", agents)
	}

	if err := conn.Close(websocket.StatusNormalClosure, "test complete"); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool {
		_, connected := registry.ConnectedAt(testAgentID)
		store.mu.Lock()
		defer store.mu.Unlock()
		return !connected && store.disconnectedCount == 1 && store.seenCount >= 1
	})
}

func TestNewSessionSupersedesOldWithoutStaleDisconnect(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		AgentOptions: AgentOptions{
			HelloTimeout:      time.Second,
			HeartbeatInterval: 50 * time.Millisecond,
			HeartbeatTimeout:  time.Second,
		},
	}))
	defer server.Close()

	first := dialAgent(t, server.URL, store.token)
	writeHello(t, first, testAgentID)
	var ack helloAck
	if err := wsjson.Read(context.Background(), first, &ack); err != nil {
		t.Fatal(err)
	}

	second := dialAgent(t, server.URL, store.token)
	writeHello(t, second, testAgentID)
	if err := wsjson.Read(context.Background(), second, &ack); err != nil {
		t.Fatal(err)
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var ignored agentMessage
	if err := wsjson.Read(ctx, first, &ignored); err == nil {
		t.Fatal("superseded connection remained readable")
	}
	time.Sleep(25 * time.Millisecond)
	store.mu.Lock()
	disconnected := store.disconnectedCount
	store.mu.Unlock()
	if disconnected != 0 {
		t.Fatalf("stale session recorded disconnect count=%d", disconnected)
	}
	if _, ok := registry.ConnectedAt(testAgentID); !ok {
		t.Fatal("replacement session was removed by stale cleanup")
	}

	_ = second.Close(websocket.StatusNormalClosure, "done")
	waitFor(t, time.Second, func() bool {
		store.mu.Lock()
		defer store.mu.Unlock()
		return store.disconnectedCount == 1
	})
}

func TestAgentIdentityMismatchIsClosed(t *testing.T) {
	store := newFakeAgentStore()
	server := newAgentTestServer(t, store, AgentOptions{HelloTimeout: time.Second, HeartbeatTimeout: time.Second})
	defer server.Close()
	conn := dialAgent(t, server.URL, store.token)
	defer conn.CloseNow()
	writeHello(t, conn, "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	var ack helloAck
	err := wsjson.Read(ctx, conn, &ack)
	if websocket.CloseStatus(err) != websocket.StatusPolicyViolation {
		t.Fatalf("expected policy close, got %v status=%v", err, websocket.CloseStatus(err))
	}
}

func TestHeartbeatTimeoutDisconnects(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
		AgentOptions: AgentOptions{
			HelloTimeout:      time.Second,
			HeartbeatInterval: 20 * time.Millisecond,
			HeartbeatTimeout:  80 * time.Millisecond,
		},
	}))
	defer server.Close()

	conn := dialAgent(t, server.URL, store.token)
	defer conn.CloseNow()
	writeHello(t, conn, testAgentID)
	var ack helloAck
	if err := wsjson.Read(context.Background(), conn, &ack); err != nil {
		t.Fatal(err)
	}
	waitFor(t, time.Second, func() bool {
		_, connected := registry.ConnectedAt(testAgentID)
		store.mu.Lock()
		defer store.mu.Unlock()
		return !connected && store.disconnectedCount == 1
	})
}

func newAgentTestServer(t *testing.T, store AgentStore, options AgentOptions) *httptest.Server {
	t.Helper()
	return httptest.NewServer(New(Dependencies{
		Database:     pingFunc(func(context.Context) error { return nil }),
		Agents:       store,
		Registry:     agentdomain.NewRegistry(),
		AgentOptions: options,
	}))
}

func dialAgent(t *testing.T, baseURL, token string) *websocket.Conn {
	t.Helper()
	headers := http.Header{}
	headers.Set("Authorization", "Bearer "+token)
	conn, response, err := websocket.Dial(context.Background(), wsURL(baseURL), &websocket.DialOptions{HTTPHeader: headers})
	if err != nil {
		if response != nil {
			t.Fatalf("dial failed: HTTP %d: %v", response.StatusCode, err)
		}
		t.Fatal(err)
	}
	return conn
}

func writeHello(t *testing.T, conn *websocket.Conn, agentID string) {
	t.Helper()
	err := wsjson.Write(context.Background(), conn, agentMessage{
		Type:            "hello",
		ProtocolVersion: 1,
		AgentID:         agentID,
		PluginVersion:   "1.0.0",
		PhBotVersion:    "fixture-phbot",
		SentAt:          time.Now().UTC().Format(time.RFC3339),
	})
	if err != nil {
		t.Fatal(err)
	}
}

func wsURL(baseURL string) string {
	return "ws" + strings.TrimPrefix(baseURL, "http") + "/agent"
}

func waitFor(t *testing.T, timeout time.Duration, condition func() bool) {
	t.Helper()
	deadline := time.Now().Add(timeout)
	for time.Now().Before(deadline) {
		if condition() {
			return
		}
		time.Sleep(10 * time.Millisecond)
	}
	t.Fatal("condition was not satisfied before timeout")
}

func TestBearerToken(t *testing.T) {
	for _, tc := range []struct {
		header string
		ok     bool
	}{
		{"Bearer phm_token", true},
		{"", false},
		{"Basic abc", false},
		{"Bearer ", false},
		{"Bearer token with spaces", false},
	} {
		_, ok := bearerToken(tc.header)
		if ok != tc.ok {
			t.Fatalf("header=%q ok=%v", tc.header, ok)
		}
	}
}

func TestAgentStoreFailureIsSanitized(t *testing.T) {
	failing := failingAgentStore{}
	server := newAgentTestServer(t, failing, AgentOptions{})
	defer server.Close()
	headers := http.Header{}
	headers.Set("Authorization", "Bearer secret-token")
	_, response, err := websocket.Dial(context.Background(), wsURL(server.URL), &websocket.DialOptions{HTTPHeader: headers})
	if err == nil {
		t.Fatal("store failure unexpectedly connected")
	}
	if response == nil || response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected response: %#v err=%v", response, err)
	}
}

type failingAgentStore struct{}

func (failingAgentStore) AuthenticateToken(context.Context, string) (string, error) {
	return "", errors.New("password=super-secret")
}
func (failingAgentStore) MarkConnected(context.Context, string, int, string, string) error {
	return errors.New("unused")
}
func (failingAgentStore) MarkSeen(context.Context, string) error { return errors.New("unused") }
func (failingAgentStore) MarkDisconnected(context.Context, string) error {
	return errors.New("unused")
}
func (failingAgentStore) ListSeen(context.Context) ([]agentdomain.Record, error) {
	return nil, errors.New("unused")
}
