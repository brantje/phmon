package agents

import (
	"context"
	"testing"
)

func TestRegistryTracksConcurrentConnectionsPerAgent(t *testing.T) {
	registry := NewRegistry()
	genOne, atOne := registry.Register("agent")
	genTwo, atTwo := registry.Register("agent")

	if genOne == genTwo {
		t.Fatal("connections shared a generation")
	}
	if !registry.IsCurrent("agent", genOne) || !registry.IsCurrent("agent", genTwo) {
		t.Fatal("one agent's connection invalidated another connection")
	}
	if registry.ConnectionCount("agent") != 2 {
		t.Fatalf("connection count = %d, want 2", registry.ConnectionCount("agent"))
	}
	if connectedAt, ok := registry.ConnectedAt("agent"); !ok || !connectedAt.Equal(atOne) || atTwo.Before(atOne) {
		t.Fatalf("unexpected connected-at value %v (atOne %v, atTwo %v)", connectedAt, atOne, atTwo)
	}
	if connectedAt, ok := registry.LatestConnectedAt("agent"); !ok || !connectedAt.Equal(atTwo) {
		t.Fatalf("latest connected-at value = %v, want %v", connectedAt, atTwo)
	}

	removed, stillConnected := registry.Unregister("agent", genOne)
	if !removed || !stillConnected {
		t.Fatalf("first unregister = (%v, %v), want (true, true)", removed, stillConnected)
	}
	if !registry.IsCurrent("agent", genTwo) || registry.IsCurrent("agent", genOne) {
		t.Fatal("unregister did not fence only the removed generation")
	}

	removed, stillConnected = registry.Unregister("agent", genTwo)
	if !removed || stillConnected {
		t.Fatalf("last unregister = (%v, %v), want (true, false)", removed, stillConnected)
	}
	if _, ok := registry.ConnectedAt("agent"); ok {
		t.Fatal("agent remained connected after its last socket closed")
	}
	if connectedAt, ok := registry.LatestConnectedAt("agent"); !ok || !connectedAt.Equal(atTwo) {
		t.Fatalf("disconnect cutoff was not retained: %v, %v", connectedAt, ok)
	}
}

func TestRegistryKeepsCapabilitiesAndWritersPerSocket(t *testing.T) {
	registry := NewRegistry()
	first, _ := registry.Register("agent")
	second, _ := registry.Register("agent")
	sent := make(chan string, 1)
	if !registry.Configure("agent", first, 3, "1.1.2", func(_ context.Context, value any) error {
		sent <- value.(string)
		return nil
	}) || !registry.Configure("agent", second, 2, "1.1.0", nil) {
		t.Fatal("could not configure registered connections")
	}
	if !registry.SetCapabilities("agent", first, []CommandCapability{{Name: "bot.stop", Supported: true}}) {
		t.Fatal("v3 capability update rejected")
	}
	if ok, reason := registry.CommandSupport("agent", first, "bot.stop"); !ok || reason != "" {
		t.Fatalf("v3 support = %v %q", ok, reason)
	}
	if ok, reason := registry.CommandSupport("agent", second, "bot.stop"); ok || reason != "plugin_upgrade_required" {
		t.Fatalf("v2 support = %v %q", ok, reason)
	}
	if err := registry.Send(context.Background(), "agent", first, "command"); err != nil {
		t.Fatal(err)
	}
	if got := <-sent; got != "command" {
		t.Fatalf("sent %q", got)
	}
}

func TestRegistryGatesTrainingAreaModesPerSocket(t *testing.T) {
	registry := NewRegistry()
	generation, _ := registry.Register("agent")
	if !registry.Configure("agent", generation, 3, "1.1.2", nil) {
		t.Fatal("configure failed")
	}
	if !registry.SetCapabilities("agent", generation, []CommandCapability{{Name: "training.area.set", Supported: true, Modes: []string{"position"}}}) {
		t.Fatal("capability update failed")
	}
	if supported, reason := registry.CommandModeSupport("agent", generation, "training.area.set", "position"); !supported || reason != "" {
		t.Fatalf("position supported=%v reason=%q", supported, reason)
	}
	if supported, reason := registry.CommandModeSupport("agent", generation, "training.area.set", "current_position"); supported || reason != "unsupported_argument_mode" {
		t.Fatalf("current position supported=%v reason=%q", supported, reason)
	}
}

func TestRegistryGatesChatModesPerSocket(t *testing.T) {
	registry := NewRegistry()
	generation, _ := registry.Register("agent")
	if !registry.Configure("agent", generation, 6, "1.4.0", nil) {
		t.Fatal("configure failed")
	}
	if !registry.SetCapabilities("agent", generation, []CommandCapability{{Name: "chat.send", Supported: true, Modes: []string{"general", "private"}}}) {
		t.Fatal("capability update failed")
	}
	if supported, reason := registry.CommandModeSupport("agent", generation, "chat.send", "private"); !supported || reason != "" {
		t.Fatalf("private chat supported=%v reason=%q", supported, reason)
	}
	if supported, reason := registry.CommandModeSupport("agent", generation, "chat.send", "global"); supported || reason != "unsupported_argument_mode" {
		t.Fatalf("global chat supported=%v reason=%q", supported, reason)
	}
	if !registry.SetCapabilities("agent", generation, []CommandCapability{{Name: "chat.send", Supported: true}}) {
		t.Fatal("capability replacement failed")
	}
	if supported, reason := registry.CommandModeSupport("agent", generation, "chat.send", "general"); supported || reason != "capability_modes_missing" {
		t.Fatalf("mode-less chat capability supported=%v reason=%q", supported, reason)
	}
}

func TestWalkRequiresPathfindingPluginVersion(t *testing.T) {
	for _, tc := range []struct {
		version string
		want    bool
	}{{"1.1.0", false}, {"1.1.1", false}, {"1.1.2", true}, {"1.2.0", true}, {"1.1.2-beta", false}, {"unknown", false}} {
		t.Run(tc.version, func(t *testing.T) {
			registry := NewRegistry()
			generation, _ := registry.Register("agent")
			if !registry.Configure("agent", generation, 3, tc.version, nil) {
				t.Fatal("configure failed")
			}
			if !registry.SetCapabilities("agent", generation, []CommandCapability{{Name: "character.walk", Supported: true}}) {
				t.Fatal("capability update failed")
			}
			supported, reason := registry.CommandSupport("agent", generation, "character.walk")
			if supported != tc.want {
				t.Fatalf("Walk support for plugin %q = %v (%q), want %v", tc.version, supported, reason, tc.want)
			}
			if !tc.want && reason != "plugin_upgrade_required" {
				t.Fatalf("Walk gate reason = %q, want plugin_upgrade_required", reason)
			}
		})
	}
}

func TestEitherConcurrentSocketCloseOrderKeepsLogicalAgentOnlineUntilLastClose(t *testing.T) {
	for _, order := range []string{"first-then-second", "second-then-first"} {
		t.Run(order, func(t *testing.T) {
			registry := NewRegistry()
			first, _ := registry.Register("agent")
			second, _ := registry.Register("agent")
			closeOne, closeTwo := first, second
			if order == "second-then-first" {
				closeOne, closeTwo = second, first
			}
			removed, stillConnected := registry.Unregister("agent", closeOne)
			if !removed || !stillConnected || registry.ConnectionCount("agent") != 1 {
				t.Fatalf("first close removed logical agent: removed=%v connected=%v count=%d", removed, stillConnected, registry.ConnectionCount("agent"))
			}
			removed, stillConnected = registry.Unregister("agent", closeTwo)
			if !removed || stillConnected || registry.ConnectionCount("agent") != 0 {
				t.Fatalf("last close retained logical agent: removed=%v connected=%v count=%d", removed, stillConnected, registry.ConnectionCount("agent"))
			}
		})
	}
}

func TestDisconnectFenceDoesNotAdvanceWhenANewerConnectionRegisters(t *testing.T) {
	registry := NewRegistry()
	first, _ := registry.Register("agent")
	second, secondAt := registry.Register("agent")
	registry.Unregister("agent", first)
	removed, connected, fence := registry.UnregisterWithFence("agent", second)
	if !removed || connected || !fence.Equal(secondAt) {
		t.Fatalf("final close returned removed=%v connected=%v fence=%v, want %v", removed, connected, fence, secondAt)
	}
	third, thirdAt := registry.Register("agent")
	if !registry.HasGeneration("agent", third) || !thirdAt.After(fence) {
		t.Fatalf("new connection did not advance beyond final-close fence: fence=%v new=%v", fence, thirdAt)
	}
	if fence.Equal(thirdAt) {
		t.Fatal("captured disconnect fence changed to include a newer connection")
	}
}


func TestCredentialRevocationReservationFencesRegistration(t *testing.T) {
	registry := NewRegistry()
	if !registry.BeginCredentialRevocation("agent") {
		t.Fatal("offline agent revocation reservation was rejected")
	}
	generation, connectedAt := registry.Register("agent")
	if generation != 0 || !connectedAt.IsZero() {
		t.Fatalf("registration bypassed revocation reservation: generation=%d at=%v", generation, connectedAt)
	}
	if registry.BeginCredentialRevocation("agent") {
		t.Fatal("duplicate revocation reservation was accepted")
	}
	registry.EndCredentialRevocation("agent")

	generation, _ = registry.Register("agent")
	if generation == 0 {
		t.Fatal("registration remained blocked after revocation reservation ended")
	}
	if registry.BeginCredentialRevocation("agent") {
		t.Fatal("connected agent revocation reservation was accepted")
	}
	registry.Unregister("agent", generation)
	if !registry.BeginCredentialRevocation("agent") {
		t.Fatal("offline agent could not be reserved after disconnect")
	}
	registry.EndCredentialRevocation("agent")
}
