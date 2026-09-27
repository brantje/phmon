package commands

import (
	"strings"
	"testing"
	"time"
)

func TestCommandExpiryTimestampIsWholeSecondUTC(t *testing.T) {
	expires := time.Date(2026, time.September, 27, 12, 57, 10, 987654321, time.FixedZone("offset", 2*60*60))
	got := commandExpiryTimestamp(expires)
	want := "2026-09-27T10:57:10Z"
	if got != want {
		t.Fatalf("command expiry = %q, want %q", got, want)
	}
	if strings.Contains(got, ".") {
		t.Fatalf("command expiry must not contain fractional seconds: %q", got)
	}
	if _, err := time.Parse(time.RFC3339, got); err != nil {
		t.Fatalf("command expiry is not RFC3339: %v", err)
	}
}
