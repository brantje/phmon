package agents

import (
	"context"
	"testing"
)

func TestRegistryGenerationFencesStaleCleanup(t *testing.T) {
	registry := NewRegistry()
	_, cancelOne := context.WithCancel(context.Background())
	genOne, _, previous := registry.Register("agent", cancelOne)
	if previous != nil {
		t.Fatal("first registration unexpectedly replaced a session")
	}

	_, cancelTwo := context.WithCancel(context.Background())
	genTwo, _, previous := registry.Register("agent", cancelTwo)
	if previous == nil {
		t.Fatal("second registration did not return the previous cancellation")
	}
	if registry.Unregister("agent", genOne) {
		t.Fatal("stale session removed the replacement")
	}
	if _, ok := registry.ConnectedAt("agent"); !ok {
		t.Fatal("replacement session disappeared")
	}
	if !registry.Unregister("agent", genTwo) {
		t.Fatal("current session did not unregister")
	}
	if _, ok := registry.ConnectedAt("agent"); ok {
		t.Fatal("session remained registered")
	}
}
