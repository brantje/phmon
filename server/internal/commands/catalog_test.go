package commands

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateCatalog(t *testing.T) {
	valid := []struct {
		name, args string
		confirm    bool
	}{
		{"bot.start", "{}", false},
		{"bot.stop", "{}", false},
		{"trace.start", `{"name":"Target"}`, false},
		{"trace.stop", "{}", false},
		{"training.area.set", `{"mode":"current_position"}`, false},
		{"training.area.set", `{"mode":"position","region":25000,"x":1,"y":2,"z":3}`, false},
		{"training.area.set", `{"mode":"named","name":"Jangan"}`, false},
		{"training.radius.set", `{"radius":50}`, false},
		{"character.walk", `{"region":25000,"x":1,"y":2,"z":3}`, false},
		{"character.navigate", `{"region":-32767,"x":-24272.5,"y":-93.5,"z":0}`, true},
		{"character.navigate.stop", `{"command_id":"cmd_00000000-0000-4000-8000-000000000001","route_sequence":1}`, false},
		{"training.area.set", `{"mode":"position","region":-32767,"x":-24272.5,"y":-93.5,"z":0}`, true},
		{"character.return", "{}", true},
		{"character.disconnect", "{}", true},
		{"chat.send", `{"channel":"general","text":"hello"}`, false},
		{"chat.send", `{"channel":"private","recipient":"Beta","text":"hello"}`, false},
		{"chat.send", `{"channel":"global","text":"hello"}`, true},
	}
	for _, tc := range valid {
		t.Run(tc.name+"/"+tc.args, func(t *testing.T) {
			if _, err := Validate(tc.name, json.RawMessage(tc.args), tc.confirm); err != nil {
				t.Fatalf("valid command rejected: %v", err)
			}
		})
	}
}

func TestValidateCatalogRejectsUnsafeInputs(t *testing.T) {
	cases := []struct {
		name, args string
		confirm    bool
	}{
		{"bot.start", `{"extra":true}`, false},
		{"trace.start", `{"name":""}`, false},
		{"training.radius.set", `{"radius":true}`, false},
		{"training.radius.set", `{"radius":10001}`, false},
		{"character.walk", `{"region":0,"x":1,"y":2,"z":3}`, false},
		{"character.navigate", `{"region":0,"x":1,"y":2,"z":3}`, false},
		{"character.navigate", `{"region":70000,"x":1,"y":2,"z":3}`, false},
		{"character.navigate.stop", `{"command_id":"bad","route_sequence":0}`, false},
		{"character.navigate.stop", `{"command_id":"cmd_00000000-0000-4000-8000-000000000001","route_sequence":true}`, false},
		{"training.area.set", `{"mode":"current_position","x":0}`, false},
		{"training.area.set", `{"mode":"named","name":" "}`, false},
		{"character.return", "{}", false},
		{"character.disconnect", "{}", false},
		{"chat.send", `{"channel":"global","text":"hello"}`, false},
		{"chat.send", `{"channel":"private","text":"hello"}`, false},
		{"chat.send", `{"channel":"party","recipient":"Beta","text":"hello"}`, false},
		{"chat.send", `{"channel":"general","text":"hello","extra":true}`, false},
	}
	for _, tc := range cases {
		t.Run(tc.name+"/"+tc.args, func(t *testing.T) {
			if _, err := Validate(tc.name, json.RawMessage(tc.args), tc.confirm); err == nil {
				t.Fatal("unsafe command accepted")
			}
		})
	}
	if _, err := Validate("arbitrary.python", json.RawMessage(`{}`), false); !errors.Is(err, ErrUnsupported) {
		t.Fatalf("unknown command error = %v", err)
	}
}
