package chat

import (
	"strings"
	"testing"
	"time"
)

func TestValidateOutbound(t *testing.T) {
	valid := []struct{ channel, text, recipient string }{
		{"general", "hello", ""},
		{"private", "hello", "Beta"},
		{"global", "hello", ""},
	}
	for _, item := range valid {
		if err := ValidateOutbound(item.channel, item.text, item.recipient); err != nil {
			t.Fatalf("valid chat send rejected: %#v: %v", item, err)
		}
	}
	invalid := []struct{ channel, text, recipient string }{
		{"unknown", "hello", ""},
		{"general", " ", ""},
		{"private", "hello", ""},
		{"party", "hello", "Beta"},
		{"guild", strings.Repeat("a", MaxTextBytes+1), ""},
		{"general", "hello\x00world", ""},
	}
	for _, item := range invalid {
		if err := ValidateOutbound(item.channel, item.text, item.recipient); err == nil {
			t.Fatalf("invalid chat send accepted: %#v", item)
		}
	}
}

func TestCursorRoundTripPreservesTimestampAndMessageID(t *testing.T) {
	at := time.Date(2026, 9, 28, 12, 34, 56, 123456789, time.FixedZone("CEST", 2*60*60))
	id := "12345678-1234-4234-8234-123456789abc"
	decoded, err := DecodeCursor(EncodeCursor(at, id))
	if err != nil {
		t.Fatal(err)
	}
	if !decoded.At.Equal(at) || decoded.ID != id {
		t.Fatalf("cursor round trip = %#v", decoded)
	}
	if _, err := DecodeCursor("not-a-cursor"); err == nil {
		t.Fatal("invalid cursor accepted")
	}
}

func TestCountUnreadObservationsDeduplicatesOnlyServerwideObserverCopies(t *testing.T) {
	base := time.Date(2026, 9, 28, 16, 25, 49, 0, time.UTC)
	observations := []unreadObservation{
		{channel: "general", server: "Greatest", characterID: "beta", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(600 * time.Millisecond)},
		{channel: "general", server: "greatest", characterID: "alpha", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(500 * time.Millisecond)},
		{channel: "general", server: "Greatest", characterID: "beta", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(100 * time.Millisecond)},
		{channel: "general", server: "Greatest", characterID: "alpha", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base},
		{channel: "party", server: "Greatest", characterID: "beta", rawType: "3", sender: "Veyra", message: "party", occurredAt: base.Add(time.Second)},
		{channel: "party", server: "Greatest", characterID: "alpha", rawType: "3", sender: "Veyra", message: "party", occurredAt: base.Add(900 * time.Millisecond)},
		{channel: "guild", server: "Greatest", characterID: "beta", rawType: "4", sender: "Veyra", message: "guild", occurredAt: base.Add(time.Second)},
		{channel: "guild", server: "Greatest", characterID: "alpha", rawType: "4", sender: "Veyra", message: "guild", occurredAt: base.Add(900 * time.Millisecond)},
		{channel: "private", server: "Greatest", characterID: "beta", rawType: "2", sender: "Veyra", message: "private", occurredAt: base.Add(time.Second)},
		{channel: "private", server: "Greatest", characterID: "alpha", rawType: "2", sender: "Veyra", message: "private", occurredAt: base.Add(900 * time.Millisecond)},
	}

	counts := countUnreadObservations(observations)
	if counts["general"] != 2 || counts["party"] != 1 || counts["guild"] != 2 || counts["private"] != 2 {
		t.Fatalf("deduplicated unread counts = %#v", counts)
	}
}

func TestCountUnreadObservationsChecksGroupsAfterStaleGroup(t *testing.T) {
	base := time.Date(2026, 9, 28, 16, 25, 49, 0, time.UTC)
	observations := []unreadObservation{
		{channel: "general", server: "Greatest", characterID: "alpha", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(3 * time.Second)},
		{channel: "general", server: "Greatest", characterID: "beta", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(3 * time.Second)},
		{channel: "general", server: "Greatest", characterID: "alpha", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(time.Second)},
		{channel: "general", server: "Greatest", characterID: "beta", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base.Add(time.Second)},
		{channel: "general", server: "Greatest", characterID: "alpha", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base},
		{channel: "general", server: "Greatest", characterID: "beta", rawType: "1", sender: "Veyra", message: "repeat", occurredAt: base},
	}

	counts := countUnreadObservations(observations)
	if counts["general"] != 3 {
		t.Fatalf("unread count with an intervening stale group = %#v, want 3", counts)
	}
}
