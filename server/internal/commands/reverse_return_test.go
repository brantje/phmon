package commands

import (
	"context"
	"encoding/json"
	"errors"
	"strings"
	"testing"
	"time"
)

func TestReverseReturnValidation(t *testing.T) {
	for _, raw := range []string{`{"type":0}`, `{"type":1,"name":""}`, `{"type":2,"name":" Member "}`, `{"type":3,"name":"Jangan"}`} {
		result, err := Validate("character.reverse_return", json.RawMessage(raw), true)
		if err != nil {
			t.Fatalf("%s: %v", raw, err)
		}
		if raw == `{"type":2,"name":" Member "}` && string(result.Args) != `{"type":2,"name":"Member"}` {
			t.Fatalf("normalization: %s", result.Args)
		}
	}
	for _, raw := range []string{`{"type":3}`, `{"type":3,"name":" "}`, `{"type":3,"name":"Jangan\n"}`} {
		if _, err := Validate("character.reverse_return", json.RawMessage(raw), true); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	for _, raw := range []string{`{}`, `{"type":null}`, `{"type":false}`, `{"type":0.5}`, `{"type":"0"}`, `{"type":-1}`, `{"type":4}`, `{"type":2}`, `{"type":2,"name":" "}`, `{"type":1,"name":"Member"}`, `{"type":0,"name":" "}`, `{"type":0,"extra":true}`, `{"type":0,"name":null}`, `{"type":2,"name":"Member\n"}`, `{"type":2,"name":"Member\u0085"}`} {
		if _, err := Validate("character.reverse_return", json.RawMessage(raw), true); err == nil {
			t.Fatalf("accepted %s", raw)
		}
	}
	raw, _ := json.Marshal(map[string]any{"type": 2, "name": strings.Repeat("é", 51)})
	if _, err := Validate("character.reverse_return", raw, true); err == nil {
		t.Fatal("accepted name above 100 UTF-8 bytes")
	}
	if _, err := Validate("character.reverse_return", json.RawMessage(`{"type":0}`), false); err == nil {
		t.Fatal("missing confirmation accepted")
	}
	for kind, wanted := range []string{"last_return", "last_death", "party_member", "named_location"} {
		raw, _ := json.Marshal(map[string]any{"type": kind, "name": "Member"})
		mode, ok := commandMode(Validated{Name: "character.reverse_return", Args: raw})
		if !ok || mode != wanted {
			t.Fatalf("mode %d: %q", kind, mode)
		}
	}
}

type namedLocationFixture map[string][]string

func (f namedLocationFixture) ReverseReturnLocations(server string) []string { return f[server] }

func TestNamedReverseReturnProfileAdmission(t *testing.T) {
	s := NewService(nil, nil)
	s.SetReverseReturnLocations(namedLocationFixture{"Greatest": {"Jangan", "Hotan"}, "Other": {"Different"}})
	for _, c := range []struct{ server, name, want string }{
		{"Greatest", "Jangan", ""}, {"Other", "Jangan", "named_location_not_found"},
		{"Unmapped", "Jangan", "named_location_names_unavailable"}, {"Greatest", "invented", "named_location_not_found"},
	} {
		if got := s.namedReverseReturnReason(c.server, c.name); got != c.want {
			t.Fatalf("%+v: %s", c, got)
		}
	}
	if got := s.controlSnapshot(Target{Server: "Greatest"}, nil)["reverse_return_named_locations"].([]string); len(got) != 2 {
		t.Fatal(got)
	}
}

type failingReverseReturnContextFixture struct{}

func (failingReverseReturnContextFixture) ReverseReturnContexts(
	context.Context,
	map[string]string,
	time.Time,
) (map[string]ReverseReturnContext, error) {
	return nil, errors.New("resource store unavailable")
}

func TestReverseReturnContextFailureDoesNotDiscardControls(t *testing.T) {
	s := NewService(nil, nil)
	s.SetReverseReturnContext(failingReverseReturnContextFixture{})
	snapshot := map[string]any{
		"character_id": "character-one",
		"session_id":   "session-one",
		"capabilities": map[string]Capability{},
	}
	s.addReverseReturnContexts(context.Background(), []map[string]any{snapshot})
	if snapshot["character_id"] != "character-one" || snapshot["session_id"] != "session-one" {
		t.Fatalf("core control fields were changed: %#v", snapshot)
	}
	if _, ok := snapshot["reverse_return"]; ok {
		t.Fatalf("failed optional context was attached: %#v", snapshot["reverse_return"])
	}
}
