package httpapi

import (
	"phmon/server/internal/events"
	"testing"
	"time"
)

func TestParseEventBoundAcceptsLocalRFC3339Boundaries(t *testing.T) {
	parsed, err := parseEventBound("2026-09-28T22:00:00+02:00", false)
	if err != nil {
		t.Fatal(err)
	}
	want := time.Date(2026, 9, 28, 20, 0, 0, 0, time.UTC)
	if !parsed.Equal(want) {
		t.Fatalf("parsed local midnight = %s, want %s", parsed, want)
	}

	end, err := parseEventBound("2026-09-29T22:00:00+02:00", true)
	if err != nil {
		t.Fatal(err)
	}
	wantEnd := time.Date(2026, 9, 29, 20, 0, 0, 0, time.UTC)
	if !end.Equal(wantEnd) {
		t.Fatalf("parsed exclusive local end = %s, want %s", end, wantEnd)
	}
}

func TestRESTAdditionalItemFeedsOnlyApplyToNormalAndRareDrops(t *testing.T) {
	for _, filter := range []events.Filter{
		{Kind: "drop.item", IncludePetPickups: true},
		{Kind: "drop.rare", IncludePetPickups: true},
		{Kind: "drop.item", IncludeOwnedGains: true},
		{Kind: "drop.rare", IncludeOwnedGains: true},
	} {
		if !validEventDropFeedFilter(filter) {
			t.Fatalf("valid feed filter rejected: %+v", filter)
		}
	}
	for _, filter := range []events.Filter{
		{IncludePetPickups: true},
		{Kind: "drop.rare", Category: "drop", IncludePetPickups: true},
		{Kind: "item.acquired", IncludePetPickups: true},
		{IncludeOwnedGains: true},
		{Kind: "item.acquired", IncludeOwnedGains: true},
	} {
		if validEventDropFeedFilter(filter) {
			t.Fatalf("invalid feed filter accepted: %+v", filter)
		}
	}
}

func TestParseEventBoundKeepsDateOnlyCompatibility(t *testing.T) {
	from, err := parseEventBound("2026-09-28", false)
	if err != nil {
		t.Fatal(err)
	}
	to, err := parseEventBound("2026-09-28", true)
	if err != nil {
		t.Fatal(err)
	}
	if got, want := from.Format(time.RFC3339), "2026-09-28T00:00:00Z"; got != want {
		t.Fatalf("date-only from = %s, want %s", got, want)
	}
	if got, want := to.Format(time.RFC3339), "2026-09-29T00:00:00Z"; got != want {
		t.Fatalf("date-only exclusive to = %s, want %s", got, want)
	}
}
