package analytics

import (
	"context"
	"errors"
	"testing"
	"time"
)

func TestPruneOldSamplesUntilContinuesWhileEitherTableHasFullBatch(t *testing.T) {
	tests := []struct {
		name     string
		batches  [][2]int64
		wantRows int64
	}{
		{
			name:     "character table full while guild table is sparse",
			batches:  [][2]int64{{5000, 0}, {120, 0}},
			wantRows: 5120,
		},
		{
			name:     "guild table full while character table is sparse",
			batches:  [][2]int64{{0, 5000}, {0, 120}},
			wantRows: 5120,
		},
		{
			name:     "both tables full then drained",
			batches:  [][2]int64{{5000, 5000}, {0, 0}},
			wantRows: 10000,
		},
	}

	for _, test := range tests {
		t.Run(test.name, func(t *testing.T) {
			calls := 0
			gotRows, err := pruneOldSamplesUntil(
				context.Background(), time.Unix(1, 0), 5000, time.Now().Add(time.Minute),
				func(_ context.Context, cutoff time.Time, batchSize int) (int64, int64, error) {
					if !cutoff.Equal(time.Unix(1, 0)) {
						t.Fatalf("unexpected cutoff: %s", cutoff)
					}
					if batchSize != 5000 {
						t.Fatalf("unexpected batch size: %d", batchSize)
					}
					if calls >= len(test.batches) {
						t.Fatalf("prune called too many times: %d", calls+1)
					}
					batch := test.batches[calls]
					calls++
					return batch[0], batch[1], nil
				},
			)
			if err != nil {
				t.Fatalf("pruneOldSamplesUntil returned an error: %v", err)
			}
			if gotRows != test.wantRows {
				t.Fatalf("deleted rows = %d, want %d", gotRows, test.wantRows)
			}
			if calls != len(test.batches) {
				t.Fatalf("prune calls = %d, want %d", calls, len(test.batches))
			}
		})
	}
}

func TestPruneOldSamplesUntilStopsOnCancellation(t *testing.T) {
	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()
	calls := 0
	gotRows, err := pruneOldSamplesUntil(
		ctx, time.Unix(1, 0), 5000, time.Now().Add(time.Minute),
		func(_ context.Context, _ time.Time, _ int) (int64, int64, error) {
			calls++
			cancel()
			return 5000, 0, nil
		},
	)
	if !errors.Is(err, context.Canceled) {
		t.Fatalf("error = %v, want context.Canceled", err)
	}
	if gotRows != 5000 || calls != 1 {
		t.Fatalf("deleted rows = %d after %d calls, want 5000 after one call", gotRows, calls)
	}
}

func TestPruneOldSamplesUntilSkipsExpiredBudget(t *testing.T) {
	calls := 0
	gotRows, err := pruneOldSamplesUntil(
		context.Background(), time.Unix(1, 0), 5000, time.Now().Add(-time.Second),
		func(context.Context, time.Time, int) (int64, int64, error) {
			calls++
			return 0, 0, nil
		},
	)
	if err != nil || gotRows != 0 || calls != 0 {
		t.Fatalf("expired budget: rows=%d calls=%d err=%v, want 0, 0, nil", gotRows, calls, err)
	}
}
