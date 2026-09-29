package commands

import (
	"strings"
	"testing"
)

func TestControlStateZoneNameBounds(t *testing.T) {
	if !(ControlState{}).ZoneNameValid() {
		t.Fatal("missing zone name should be valid")
	}
	for _, zone := range []string{"Jangan", "Donwhang Cave"} {
		if !(ControlState{TrainingZone: &zone}).ZoneNameValid() {
			t.Errorf("valid training zone %q was rejected", zone)
		}
	}
	for _, zone := range []string{"", " Jangan", "Jangan ", strings.Repeat("Z", 101)} {
		if (ControlState{TrainingZone: &zone}).ZoneNameValid() {
			t.Errorf("invalid training zone %q was accepted", zone)
		}
	}
}
