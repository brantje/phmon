package analytics

import (
	"context"
	"errors"
	"fmt"
	"sync"
	"sync/atomic"
	"testing"
	"time"
)

func TestEconomyCachePreservesAsOfAndExpiresFromObservationTime(t *testing.T) {
	now := time.Unix(100, 0)
	cache := economySnapshotCache{now: func() time.Time { return now }}
	calls := 0
	build := func() (Snapshot, error) {
		calls++
		return Snapshot{AsOf: now.Add(-2 * time.Second), Total: fmt.Sprint(calls)}, nil
	}
	first, _ := cache.query(context.Background(), "one", build)
	now = now.Add(2 * time.Second)
	second, _ := cache.query(context.Background(), "one", build)
	if calls != 1 || first.AsOf != second.AsOf || second.Total != "1" {
		t.Fatalf("cached balance was rebuilt or relabelled: calls=%d first=%+v second=%+v", calls, first, second)
	}
	now = now.Add(time.Second)
	third, _ := cache.query(context.Background(), "one", build)
	if calls != 2 || third.Total != "2" {
		t.Fatal("snapshot older than five seconds was reused")
	}
}

func TestEconomyCacheScopesFailuresAndMemoryBound(t *testing.T) {
	cache := economySnapshotCache{}
	failure := errors.New("database unavailable")
	_, err := cache.query(context.Background(), "one", func() (Snapshot, error) { return Snapshot{}, failure })
	if !errors.Is(err, failure) {
		t.Fatal(err)
	}
	for index := 0; index < economyCacheEntries+3; index++ {
		key := fmt.Sprint(index)
		got, err := cache.query(context.Background(), key, func() (Snapshot, error) {
			return Snapshot{AsOf: time.Now(), Total: key}, nil
		})
		if err != nil || got.Total != key {
			t.Fatalf("different balance scope was reused: got=%+v err=%v", got, err)
		}
	}
	if len(cache.entries) != economyCacheEntries {
		t.Fatalf("cache exceeded its bound: %d", len(cache.entries))
	}
	got, err := cache.query(context.Background(), "one", func() (Snapshot, error) {
		return Snapshot{AsOf: time.Now(), Total: "recovered"}, nil
	})
	if err != nil || got.Total != "recovered" {
		t.Fatalf("failed read was cached: %+v %v", got, err)
	}
}

func TestEconomyCacheSharesConcurrentReadAndWaiterCanCancel(t *testing.T) {
	cache := economySnapshotCache{}
	started, release := make(chan struct{}), make(chan struct{})
	var calls atomic.Int32
	build := func() (Snapshot, error) {
		if calls.Add(1) == 1 {
			close(started)
		}
		<-release
		return Snapshot{AsOf: time.Now(), Total: "12"}, nil
	}
	var workers sync.WaitGroup
	workers.Add(1)
	go func() {
		defer workers.Done()
		_, _ = cache.query(context.Background(), "fleet", build)
	}()
	<-started
	ctx, cancel := context.WithCancel(context.Background())
	go cancel()
	if _, err := cache.query(ctx, "fleet", build); !errors.Is(err, context.Canceled) {
		t.Fatalf("waiting client did not cancel independently: %v", err)
	}
	for index := 0; index < 8; index++ {
		workers.Add(1)
		go func() {
			defer workers.Done()
			got, err := cache.query(context.Background(), "fleet", build)
			if err != nil || got.Total != "12" {
				t.Errorf("shared balance read failed: %+v %v", got, err)
			}
		}()
	}
	close(release)
	workers.Wait()
	if calls.Load() != 1 {
		t.Fatalf("same fleet history rebuilt %d times", calls.Load())
	}
}
