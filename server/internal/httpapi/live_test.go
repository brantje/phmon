package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"
	"time"

	"github.com/coder/websocket"

	agentdomain "phmon/server/internal/agents"
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
		"detail": {
			message: liveClientMessage{SubscriptionID: "detail", Revision: 1, Stream: "character", Filter: liveFilter{CharacterID: validCharacterID}},
			valid:   true,
		},
		"groups": {
			message: liveClientMessage{SubscriptionID: "groups", Revision: 1, Stream: "groups"},
			valid:   true,
		},
		"bad stream": {
			message: liveClientMessage{SubscriptionID: "x", Revision: 1, Stream: "events"},
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
