package agents

import "testing"

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
