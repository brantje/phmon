package httpapi

import (
	"encoding/json"
	"testing"
	"time"

	"phmon/server/internal/events"
)

func TestNotificationLocationForJobSightings(t *testing.T) {
	region, x, y, z := 25735, 98.5, 1551.25, 3.0
	reporter := events.Event{
		Kind: "job.hunter_trader_seen", Server: "Greatest", Zone: "Jangan",
		Region: &region, X: &x, Y: &y, Z: &z, OccurredAt: time.Now().UTC(),
		Payload: json.RawMessage(`{"value":"TraderOne"}`),
	}
	got := notificationLocation(nil, reporter)
	if got["source"] != "reporter" || got["zone"] != "Jangan" || got["x"] != x || got["y"] != y || got["z"] != z {
		t.Fatalf("reporter location = %#v", got)
	}

	thief := reporter
	thief.Kind = "job.thief_seen"
	thief.Payload = json.RawMessage(`{"value":"Pickpocket","position_source":"thief","thief":{"name":"Pickpocket","region":23687,"x":10.5,"y":20.5}}`)
	got = notificationLocation(nil, thief)
	if got["source"] != "thief" || got["x"] != 10.5 || got["y"] != 20.5 {
		t.Fatalf("thief location = %#v", got)
	}
	if _, ok := got["zone"]; ok {
		t.Fatalf("the reporter's zone must not label the thief's own position: %#v", got)
	}

	observer := thief
	observer.Payload = json.RawMessage(`{"value":"Pickpocket","position_source":"observer"}`)
	got = notificationLocation(nil, observer)
	if got["source"] != "reporter" || got["zone"] != "Jangan" || got["x"] != x {
		t.Fatalf("observer-sourced thief location = %#v", got)
	}

	if got := notificationLocation(nil, events.Event{Kind: "job.thief_seen", Server: "Greatest", Payload: json.RawMessage(`{"value":"Pickpocket"}`)}); got != nil {
		t.Fatalf("a sighting without any position has no location: %#v", got)
	}
	if got := notificationLocation(nil, events.Event{Kind: "character.died", Server: "Greatest", Zone: "Jangan"}); got != nil {
		t.Fatalf("only job sightings carry a location: %#v", got)
	}
}
