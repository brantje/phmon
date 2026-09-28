package httpapi

import (
	"testing"
	"time"

	"phmon/server/internal/characters"
)

func TestCharacterWireStateValidation(t *testing.T) {
	level := 120
	negative := int64(-1)
	region := 70000
	coordinate := 1000001.0
	validModel := int64(1907)
	tooLargeModel := int64(4294967296)
	zeroModel := int64(0)
	valid := characters.State{Level: &level}
	if !validWireState(valid) {
		t.Fatal("valid level was rejected")
	}
	if !validWireState(characters.State{Model: &validModel}) {
		t.Fatal("valid model-only state was rejected")
	}
	for _, state := range []characters.State{{}, {HP: &negative}, {Region: &region}, {X: &coordinate}, {Model: &zeroModel}, {Model: &negative}, {Model: &tooLargeModel}} {
		if validWireState(state) {
			t.Fatalf("invalid state accepted: %+v", state)
		}
	}
}

func TestDiagnosticTimestampValidation(t *testing.T) {
	if !validMessageTime(time.Now().UTC().Format(time.RFC3339)) {
		t.Fatal("valid RFC3339 timestamp rejected")
	}
	for _, value := range []string{"", "yesterday", "2026-09-26T12:00:00", "2026-09-26T12:00:00Z" + "123456789012345678901234567890"} {
		if validMessageTime(value) {
			t.Fatalf("invalid timestamp accepted: %q", value)
		}
	}
}
