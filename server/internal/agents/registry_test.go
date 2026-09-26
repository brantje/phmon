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
}
