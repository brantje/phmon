package resources

import (
	"testing"
	"time"

	"phmon/server/internal/commands"
)

func TestReverseReturnResourceFreshnessAndInventoryAdvisory(t *testing.T) {
	now := time.Now().UTC()
	for _, age := range []time.Duration{0, 35 * time.Second, -5 * time.Second, 36 * time.Second, -6 * time.Second} {
		value := commands.ReverseReturnContext{SessionID: "session", PartyStatus: "unavailable", PartyNames: []string{}}
		applyReverseReturnResource(&value, "party", "observed", []byte(`{"members":[{"name":"Member"},{"name":"member"},{"name":"Bad\nName"}]}`), now.Add(-age), now)
		fresh := age <= 35*time.Second && age >= -5*time.Second
		if fresh && (value.PartyStatus != "observed" || len(value.PartyNames) != 1) {
			t.Fatalf("fresh %s: %#v", age, value)
		}
		if !fresh && (value.PartyStatus != "stale" || len(value.PartyNames) != 0) {
			t.Fatalf("stale %s: %#v", age, value)
		}
	}
	value := commands.ReverseReturnContext{}
	payload := []byte(`{"slots":[null,{"item":{"servername":"ITEM_MALL_REVERSE_RETURN_SCROLL","quantity":1}}]}`)
	applyReverseReturnResource(&value, "inventory", "observed", payload, now, now)
	if value.ScrollObserved == nil || !*value.ScrollObserved {
		t.Fatal("scroll not identified")
	}
	value = commands.ReverseReturnContext{}
	applyReverseReturnResource(&value, "storage", "observed", payload, now, now)
	if value.ScrollObserved != nil {
		t.Fatal("storage considered usable inventory")
	}
	applyReverseReturnResource(&value, "inventory", "unavailable", payload, now, now)
	if value.ScrollObserved != nil {
		t.Fatal("unavailable inventory considered fresh")
	}
	applyReverseReturnResource(&value, "inventory", "observed", payload, now.Add(-36*time.Second), now)
	if value.ScrollObserved != nil {
		t.Fatal("stale inventory considered fresh")
	}
}
