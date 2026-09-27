package httpapi

import (
	"context"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/coder/websocket/wsjson"
	agentdomain "phmon/server/internal/agents"
)

func TestAgentProtocolV3CapabilitiesAreConnectionScoped(t *testing.T) {
	store := newFakeAgentStore()
	registry := agentdomain.NewRegistry()
	server := newAgentTestServerWithRegistry(t, store, registry, AgentOptions{
		HelloTimeout: time.Second, HeartbeatTimeout: time.Second,
	})
	defer server.Close()

	conn := dialAgent(t, server.URL, store.token)
	defer conn.CloseNow()
	if err := wsjson.Write(context.Background(), conn, agentMessage{
		Type:            "hello",
		ProtocolVersion: 3,
		AgentID:         testAgentID,
		PluginVersion:   "1.2.0",
		PhBotVersion:    "fixture-phbot",
		SentAt:          time.Now().UTC().Format(time.RFC3339),
	}); err != nil {
		t.Fatal(err)
	}
	var ack helloAck
	if err := wsjson.Read(context.Background(), conn, &ack); err != nil {
		t.Fatal(err)
	}
	if ack.ProtocolVersion != 3 {
		t.Fatalf("v3 hello negotiated protocol %d", ack.ProtocolVersion)
	}
	if err := wsjson.Write(context.Background(), conn, agentMessage{
		Type:            "agent.capabilities",
		ProtocolVersion: 3,
		SchemaVersion:   1,
		Commands: []agentCapability{
			{Name: "bot.stop", Supported: true},
			{Name: "client.clientless", Supported: false, Reason: "unsupported_runtime_primitive"},
		},
	}); err != nil {
		t.Fatal(err)
	}

	waitFor(t, time.Second, func() bool {
		for generation := uint64(1); generation < 8; generation++ {
			if supported, _ := registry.CommandSupport(testAgentID, generation, "bot.stop"); supported {
				if clientless, reason := registry.CommandSupport(testAgentID, generation, "client.clientless"); clientless || reason != "unsupported_runtime_primitive" {
					t.Fatalf("unexpected clientless capability %v %q", clientless, reason)
				}
				return true
			}
		}
		return false
	})
}

func newAgentTestServerWithRegistry(t *testing.T, store AgentStore, registry *agentdomain.Registry, options AgentOptions) *httptest.Server {
	t.Helper()
	return httptest.NewServer(New(Dependencies{
		Database:     pingFunc(func(context.Context) error { return nil }),
		Agents:       store,
		Registry:     registry,
		AgentOptions: options,
	}))
}
