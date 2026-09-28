package commands

import (
	"context"
	"strings"
	"testing"
	"time"
)

type protocolTestSender struct{ version int }

func (*protocolTestSender) Send(_ context.Context, _ string, _ uint64, _ any) error { return nil }
func (s *protocolTestSender) ProtocolVersion(_ string, _ uint64) int                { return s.version }

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

func TestCommandDispatchUsesNegotiatedAgentProtocolVersion(t *testing.T) {
	for _, test := range []struct{ negotiated, want int }{{3, 3}, {4, 4}, {0, 3}, {2, 3}} {
		sender := &protocolTestSender{version: test.negotiated}
		if got := commandProtocolVersion(sender, "agent", 7); got != test.want {
			t.Fatalf("negotiated protocol %d produced command protocol %d, want %d", test.negotiated, got, test.want)
		}
	}
}
