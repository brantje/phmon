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
	"phmon/server/internal/characters"
	"phmon/server/internal/commands"
	"phmon/server/internal/navigation"
)

const testAgentID = "11111111-2222-4333-8444-555555555555"

func TestValidCommandResultStatus(t *testing.T) {
	for _, status := range []string{"completed", "failed", "unknown"} {
		if !validCommandResultStatus(status) {
			t.Errorf("valid command result status %q was rejected", status)
		}
	}
	for _, status := range []string{"queued", "dispatching", "sent", "acknowledged", "expired", "bogus"} {
		if validCommandResultStatus(status) {
			t.Errorf("non-result status %q was accepted", status)
		}
	}
}

func TestNavigationRouteRequiresExactCompletedCommandAndCurrentConnectionOwner(t *testing.T) {
	sessionID := "22222222-3333-4444-8555-666666666666"
	agentID := testAgentID
	character := characters.Character{
		ID: "33333333-4444-4555-8666-777777777777", Online: true,
		SessionID: &sessionID, AgentID: &agentID,
	}
	route := navigation.Input{CharacterID: character.ID, SessionID: sessionID}
	command := commands.Command{
		CharacterID: character.ID, SessionID: sessionID, AgentID: testAgentID,
		ConnectionGeneration: 7, Name: "character.navigate", State: commands.StateCompleted,
	}
	if !navigationRouteOwnerMatches(route, command, character, testAgentID, 7) {
		t.Fatal("valid route ownership did not match")
	}
	cases := map[string]func(*navigation.Input, *commands.Command, *characters.Character){
		"wrong character": func(route *navigation.Input, _ *commands.Command, _ *characters.Character) {
			route.CharacterID = "44444444-5555-4666-8777-888888888888"
		},
		"wrong session": func(route *navigation.Input, _ *commands.Command, _ *characters.Character) {
			route.SessionID = "55555555-6666-4777-8888-999999999999"
		},
		"wrong agent": func(_ *navigation.Input, _ *commands.Command, character *characters.Character) {
			*character.AgentID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
		},
		"wrong generation": func(_ *navigation.Input, command *commands.Command, _ *characters.Character) {
			command.ConnectionGeneration++
		},
		"wrong command": func(_ *navigation.Input, command *commands.Command, _ *characters.Character) {
			command.Name = "character.walk"
		},
		"incomplete command": func(_ *navigation.Input, command *commands.Command, _ *characters.Character) {
			command.State = commands.StateAcknowledged
		},
		"offline": func(_ *navigation.Input, _ *commands.Command, character *characters.Character) {
			character.Online = false
		},
	}
	for name, mutate := range cases {
		t.Run(name, func(t *testing.T) {
			currentRoute, currentCommand, currentCharacter := route, command, character
			currentAgentID := testAgentID
			currentCharacter.AgentID = &currentAgentID
			mutate(&currentRoute, &currentCommand, &currentCharacter)
			if navigationRouteOwnerMatches(currentRoute, currentCommand, currentCharacter, testAgentID, 7) {
				t.Fatal("mismatched route ownership was accepted")
			}
		})
	}
}

type fakeAgentStore struct {
	mu                sync.Mutex
	token             string
	agentID           string
	record            agentdomain.Record
	seenCount         int
	disconnectedCount int
	created           []agentdomain.Credential
	revoked           []string
}

func newFakeAgentStore() *fakeAgentStore {
	return &fakeAgentStore{token: "phm_test_token", agentID: testAgentID}
}

func (s *fakeAgentStore) CreateCredential(_ context.Context, credential agentdomain.Credential) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.created = append(s.created, credential)
	return nil
}

func (s *fakeAgentStore) RevokeCredential(_ context.Context, agentID string) (bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if agentID != s.agentID {
		found := false
		for _, credential := range s.created {
			if credential.AgentID == agentID {
				found = true
				break
			}
		}
		if !found {
			return false, nil
		}
	}
	s.revoked = append(s.revoked, agentID)
	return true, nil
}

func (s *fakeAgentStore) AuthenticateToken(_ context.Context, token string) (string, error) {
	if token != s.token {
		return "", agentdomain.ErrInvalidToken
	}
	return s.agentID, nil
}

func (s *fakeAgentStore) MarkConnected(_ context.Context, agentID string, connectedAt time.Time, protocol int, plugin, phbot string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.record.AgentID = agentID
	if s.record.FirstSeenAt == nil {
		s.record.FirstSeenAt = &connectedAt
	}
	s.record.LastSeenAt = &connectedAt
	s.record.LastConnectedAt = &connectedAt
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

func (s *fakeAgentStore) MarkDisconnected(_ context.Context, _ string, connectedAt time.Time) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.record.LastConnectedAt != nil && s.record.LastConnectedAt.After(connectedAt) {
		return nil
	}
	s.disconnectedCount++
	now := time.Now().UTC()
	s.record.LastDisconnectedAt = &now
	return nil
}

type blockingMarkConnectedStore struct {
	*fakeAgentStore
	started chan struct{}
	release chan struct{}
}

func (s *blockingMarkConnectedStore) MarkConnected(ctx context.Context, agentID string, connectedAt time.Time, protocol int, plugin, phbot string) error {
	close(s.started)
	select {
	case <-s.release:
		return s.fakeAgentStore.MarkConnected(ctx, agentID, connectedAt, protocol, plugin, phbot)
	case <-ctx.Done():
		return ctx.Err()
	}
}

func (s *fakeAgentStore) ListSeen(context.Context) ([]agentdomain.Record, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	records := make([]agentdomain.Record, 0, len(s.created)+1)
	if s.record.AgentID != "" {
		records = append(records, s.record)
	}
	for _, credential := range s.created {
		if credential.AgentID == s.record.AgentID {
			continue
		}
		revoked := false
		for _, agentID := range s.revoked {
			if agentID == credential.AgentID {
				revoked = true
				break
			}
		}
		if !revoked {
			records = append(records, agentdomain.Record{AgentID: credential.AgentID})
		}
	}
	return records, nil
}

func TestCreateAgentCredential(t *testing.T) {
	store := newFakeAgentStore()
	server := newAgentTestServer(t, store, AgentOptions{})
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/agents/credentials", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusCreated {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	if response.Header.Get("Cache-Control") != "no-store" {
		t.Fatalf("credential response must not be cacheable: %q", response.Header.Get("Cache-Control"))
	}

	var credential AgentCredentialView
	if err := json.NewDecoder(response.Body).Decode(&credential); err != nil {
		t.Fatal(err)
	}
	if !agentdomain.ValidAgentID(credential.AgentID) || !strings.HasPrefix(credential.AgentToken, "phm_") {
		t.Fatalf("unexpected credential shape: %+v", credential)
	}

	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.created) != 1 || store.created[0].AgentID != credential.AgentID || store.created[0].Token != credential.AgentToken {
		t.Fatalf("credential was not stored exactly once: %+v", store.created)
	}
}

func TestCreatedAgentIsListedBeforeFirstConnection(t *testing.T) {
	store := newFakeAgentStore()
	server := newAgentTestServer(t, store, AgentOptions{})
	defer server.Close()

	credentialCtx, credentialCancel := context.WithTimeout(context.Background(), time.Second)
	defer credentialCancel()
	credentialRequest, err := http.NewRequestWithContext(credentialCtx, http.MethodPost, server.URL+"/api/agents/credentials", strings.NewReader("{}"))
	if err != nil {
		t.Fatal(err)
	}
	credentialRequest.Header.Set("Content-Type", "application/json")
	response, err := http.DefaultClient.Do(credentialRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	var credential AgentCredentialView
	if err := json.NewDecoder(response.Body).Decode(&credential); err != nil {
		t.Fatal(err)
	}

	listCtx, listCancel := context.WithTimeout(context.Background(), time.Second)
	defer listCancel()
	listRequest, err := http.NewRequestWithContext(listCtx, http.MethodGet, server.URL+"/api/agents", nil)
	if err != nil {
		t.Fatal(err)
	}
	listResponse, err := http.DefaultClient.Do(listRequest)
	if err != nil {
		t.Fatal(err)
	}
	defer listResponse.Body.Close()
	var agents []AgentView
	if err := json.NewDecoder(listResponse.Body).Decode(&agents); err != nil {
		t.Fatal(err)
	}
	if len(agents) != 1 || agents[0].AgentID != credential.AgentID || agents[0].Connected || agents[0].FirstSeenAt != nil {
		t.Fatalf("unexpected never-connected agent list: %+v", agents)
	}
}

func TestRemoveAgentCredential(t *testing.T) {
	store := newFakeAgentStore()
	server := newAgentTestServer(t, store, AgentOptions{})
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/agents/"+testAgentID, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusOK {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.revoked) != 1 || store.revoked[0] != testAgentID {
		t.Fatalf("agent was not revoked exactly once: %+v", store.revoked)
	}
}

func TestRemoveConnectedAgentCredentialIsRejected(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	generation, _ := registry.Register(testAgentID)
	defer registry.Unregister(testAgentID, generation)
	server := httptest.NewServer(New(Dependencies{
		Database: pingFunc(func(context.Context) error { return nil }),
		Agents:   store,
		Registry: registry,
	}))
	defer server.Close()

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/agents/"+testAgentID, nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusConflict {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if len(store.revoked) != 0 {
		t.Fatalf("connected agent was revoked: %+v", store.revoked)
	}
}

func TestAgentRegistrationFencesRevocationBeforeDurableConnectCheck(t *testing.T) {
	baseStore := newFakeAgentStore()
	store := &blockingMarkConnectedStore{
		fakeAgentStore: baseStore,
		started:        make(chan struct{}),
		release:        make(chan struct{}),
	}
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

	conn := dialAgent(t, server.URL, baseStore.token)
	defer conn.CloseNow()
	writeHello(t, conn, testAgentID)

	select {
	case <-store.started:
	case <-time.After(time.Second):
		t.Fatal("MarkConnected was not reached")
	}
	if registry.ConnectionCount(testAgentID) != 1 {
		close(store.release)
		t.Fatal("agent was not registered before the durable credential check")
	}

	ctx, cancel := context.WithTimeout(context.Background(), time.Second)
	defer cancel()
	request, err := http.NewRequestWithContext(ctx, http.MethodDelete, server.URL+"/api/agents/"+testAgentID, nil)
	if err != nil {
		close(store.release)
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		close(store.release)
		t.Fatal(err)
	}
	if response.StatusCode != http.StatusConflict {
		_ = response.Body.Close()
		close(store.release)
		t.Fatalf("revocation raced past pending registration: status=%d", response.StatusCode)
	}
	_ = response.Body.Close()

	close(store.release)
	readCtx, readCancel := context.WithTimeout(context.Background(), time.Second)
	defer readCancel()
	var ack helloAck
	if err := wsjson.Read(readCtx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.Type != "hello.ack" {
		t.Fatalf("unexpected ack after fenced revocation attempt: %+v", ack)
	}
}

func TestCreateAgentCredentialStoreFailureIsSanitized(t *testing.T) {
	server := newAgentTestServer(t, failingAgentStore{}, AgentOptions{})
	defer server.Close()

	request, err := http.NewRequest(http.MethodPost, server.URL+"/api/agents/credentials", nil)
	if err != nil {
		t.Fatal(err)
	}
	response, err := http.DefaultClient.Do(request)
	if err != nil {
		t.Fatal(err)
	}
	defer response.Body.Close()
	if response.StatusCode != http.StatusServiceUnavailable {
		t.Fatalf("unexpected status: %d", response.StatusCode)
	}

	var body map[string]string
	if err := json.NewDecoder(response.Body).Decode(&body); err != nil {
		t.Fatal(err)
	}
	if body["error"] != "service unavailable" {
		t.Fatalf("unexpected error response: %+v", body)
	}
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
	if ack.Type != "hello.ack" || ack.ProtocolVersion != 2 {
		t.Fatalf("unexpected ack: %+v", ack)
	}
	if len(ack.ServerTime) != len("2006-01-02T15:04:05Z") || ack.ServerTime[len(ack.ServerTime)-1] != 'Z' {
		t.Fatalf("hello ack must use whole-second RFC3339 UTC: %q", ack.ServerTime)
	}
	if _, ok := registry.ConnectedAt(testAgentID); !ok {
		t.Fatal("agent was not registered")
	}

	if err := wsjson.Write(context.Background(), conn, agentMessage{
		Type:            "heartbeat",
		ProtocolVersion: 2,
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
	if len(agents) != 1 || agents[0].AgentID != testAgentID || !agents[0].Connected || agents[0].ActiveConnections != 1 || agents[0].ConnectedAt == nil {
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

func TestMultipleSocketsForOneAgentRemainConnectedIndependently(t *testing.T) {
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

	for _, conn := range []*websocket.Conn{first, second} {
		if err := wsjson.Write(context.Background(), conn, agentMessage{
			Type:            "heartbeat",
			ProtocolVersion: 2,
			SentAt:          time.Now().UTC().Format(time.RFC3339),
		}); err != nil {
			t.Fatalf("agent socket stopped working after same-agent connect: %v", err)
		}
	}
	response, err := http.Get(server.URL + "/api/agents")
	if err != nil {
		t.Fatal(err)
	}
	var agents []AgentView
	if err := json.NewDecoder(response.Body).Decode(&agents); err != nil {
		t.Fatal(err)
	}
	_ = response.Body.Close()
	if len(agents) != 1 || agents[0].ActiveConnections != 2 {
		t.Fatalf("same agent should report two live sockets: %+v", agents)
	}
	_ = first.Close(websocket.StatusNormalClosure, "first socket done")
	waitFor(t, time.Second, func() bool { return registry.ConnectionCount(testAgentID) == 1 })
	store.mu.Lock()
	disconnected := store.disconnectedCount
	store.mu.Unlock()
	if disconnected != 0 {
		t.Fatalf("agent marked disconnected while another socket remains: count=%d", disconnected)
	}
	if _, ok := registry.ConnectedAt(testAgentID); !ok {
		t.Fatal("agent was marked offline while second socket remains")
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
		ProtocolVersion: 2,
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

func (failingAgentStore) CreateCredential(context.Context, agentdomain.Credential) error {
	return errors.New("password=super-secret")
}
func (failingAgentStore) RevokeCredential(context.Context, string) (bool, error) {
	return false, errors.New("password=super-secret")
}
func (failingAgentStore) AuthenticateToken(context.Context, string) (string, error) {
	return "", errors.New("password=super-secret")
}
func (failingAgentStore) MarkConnected(context.Context, string, time.Time, int, string, string) error {
	return errors.New("unused")
}
func (failingAgentStore) MarkSeen(context.Context, string) error { return errors.New("unused") }
func (failingAgentStore) MarkDisconnected(context.Context, string, time.Time) error {
	return errors.New("unused")
}
func (failingAgentStore) ListSeen(context.Context) ([]agentdomain.Record, error) {
	return nil, errors.New("unused")
}

func TestInvalidMapObservationsDoNotDisconnectAgent(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	server := httptest.NewServer(New(Dependencies{Agents: store, Registry: registry, Characters: characters.NewStore(nil)}))
	defer server.Close()
	conn := dialAgent(t, server.URL, store.token)
	defer conn.CloseNow()
	ctx, cancel := context.WithTimeout(context.Background(), 3*time.Second)
	defer cancel()
	if err := wsjson.Write(ctx, conn, agentMessage{Type: "hello", ProtocolVersion: agentProtocolVersion, AgentID: testAgentID, PluginVersion: "test", PhBotVersion: "fixture", SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
		t.Fatal(err)
	}
	var ack helloAck
	if err := wsjson.Read(ctx, conn, &ack); err != nil {
		t.Fatal(err)
	}
	z := 0.0
	frames := []*agentMonsterSnapshot{nil, {
		Status: "observed", CharacterID: testAgentID, SessionID: testAgentID,
		Region: 25000, ObserverZ: &z, ObservedAt: time.Now().UTC().Add(-time.Minute),
	}}
	count := 0
	for _, kind := range []string{"map.players", "map.monsters", "map.npcs"} {
		for _, frame := range frames {
			// Exercise both missing payloads and expired observations with collection enabled.
			if err := wsjson.Write(ctx, conn, agentMessage{Type: kind, ProtocolVersion: agentProtocolVersion, MapSnapshot: frame}); err != nil {
				t.Fatal(err)
			}
			if err := wsjson.Write(ctx, conn, agentMessage{Type: "heartbeat", ProtocolVersion: agentProtocolVersion, SentAt: time.Now().UTC().Format(time.RFC3339)}); err != nil {
				t.Fatal(err)
			}
			count++
			waitFor(t, time.Second, func() bool { store.mu.Lock(); defer store.mu.Unlock(); return store.seenCount == count })
			if registry.ConnectionCount(testAgentID) != 1 {
				t.Fatalf("%s disconnected the agent", kind)
			}
		}
	}
	store.mu.Lock()
	defer store.mu.Unlock()
	if store.disconnectedCount != 0 {
		t.Fatal("rejected observation ended the session")
	}
}
