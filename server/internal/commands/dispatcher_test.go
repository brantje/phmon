package commands

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"sync"
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

type schedulerTestStore struct {
	mu         sync.Mutex
	states     map[string]string
	claims     map[string]int
	reconciled chan struct{}
}

func (s *schedulerTestStore) Queued(_ context.Context, limit int) ([]string, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	var ids []string
	for i := 0; i < 8 && len(ids) < limit; i++ {
		id := fmt.Sprint(i)
		if s.states[id] == "queued" {
			ids = append(ids, id)
		}
	}
	return ids, nil
}
func (s *schedulerTestStore) Reconcile(context.Context, time.Time, time.Duration, time.Duration) (bool, error) {
	select {
	case s.reconciled <- struct{}{}:
	default:
	}
	return false, nil
}
func (s *schedulerTestStore) ClaimDispatch(_ context.Context, id string, now time.Time) (Command, bool, error) {
	s.mu.Lock()
	defer s.mu.Unlock()
	if s.states[id] != "queued" {
		return Command{}, false, nil
	}
	s.claims[id]++
	s.states[id] = "dispatching"
	return Command{ID: id, AgentID: id, ExpiresAt: now.Add(time.Minute)}, true, nil
}
func (*schedulerTestStore) CurrentTargetMatches(context.Context, Command) (bool, error) {
	return true, nil
}
func (s *schedulerTestStore) finish(id, state string) error {
	s.mu.Lock()
	defer s.mu.Unlock()
	s.states[id] = state
	return nil
}
func (s *schedulerTestStore) FailBeforeSend(_ context.Context, id, _ string, _ time.Time) error {
	return s.finish(id, "failed")
}
func (s *schedulerTestStore) MarkUnknown(_ context.Context, id, _ string, _ time.Time) error {
	return s.finish(id, "unknown")
}
func (s *schedulerTestStore) MarkSent(_ context.Context, id string, _ time.Time) error {
	return s.finish(id, "sent")
}

type schedulerTestSender struct {
	mu           sync.Mutex
	active, peak int
	started      chan string
	release      chan struct{}
	blocked      chan struct{}
}

func (s *schedulerTestSender) Send(ctx context.Context, id string, _ uint64, _ any) error {
	s.mu.Lock()
	s.active++
	if s.active > s.peak {
		s.peak = s.active
	}
	s.mu.Unlock()
	defer func() { s.mu.Lock(); s.active--; s.mu.Unlock() }()
	s.started <- id
	// Keep the first connection blocked while siblings can finish independently.
	if id == "0" {
		close(s.blocked)
		select {
		case <-s.release:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if id != "0" {
		select {
		case <-s.blocked:
		case <-ctx.Done():
			return ctx.Err()
		}
	}
	if id == "1" {
		return errors.New("ambiguous fixture write")
	}
	return nil
}

func TestDispatcherParallelDeliveryKeepsReconciliationAndShutdownResponsive(t *testing.T) {
	store := &schedulerTestStore{states: map[string]string{}, claims: map[string]int{}, reconciled: make(chan struct{}, 1)}
	for i := 0; i < 8; i++ {
		store.states[fmt.Sprint(i)] = "queued"
	}
	sender := &schedulerTestSender{started: make(chan string, 8), release: make(chan struct{}), blocked: make(chan struct{})}
	dispatcher := &Dispatcher{store: store, sender: sender, wake: make(chan struct{}, 1)}
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	done := make(chan struct{})
	go func() { dispatcher.Run(ctx); close(done) }()
	dispatcher.Notify()
	seen := map[string]bool{}
	deadline := time.After(3 * time.Second)
	for len(seen) < 8 {
		select {
		case id := <-sender.started:
			if seen[id] {
				t.Fatalf("duplicate delivery of %s", id)
			}
			seen[id] = true
		case <-deadline:
			t.Fatal("blocked connection prevented sibling delivery")
		}
	}
	select {
	case <-store.reconciled:
	case <-time.After(2 * time.Second):
		t.Fatal("slow sender blocked reconciliation")
	}
	store.mu.Lock()
	for id, claims := range store.claims {
		if claims != 1 {
			t.Errorf("%s claimed %d times", id, claims)
		}
	}
	if store.states["0"] != "dispatching" || store.states["1"] != "unknown" || store.states["7"] != "sent" {
		t.Errorf("unexpected independent states: %v", store.states)
	}
	store.mu.Unlock()
	sender.mu.Lock()
	peak := sender.peak
	sender.mu.Unlock()
	if peak > dispatchWorkers || peak < 2 {
		t.Errorf("parallel deliveries peaked at %d", peak)
	}
	cancel()
	select {
	case <-done:
	case <-time.After(time.Second):
		t.Fatal("dispatcher workers leaked after cancellation")
	}
}
