package analytics

import (
	"context"
	"sync"
	"time"
)

const economyCacheLifetime = 5 * time.Second
const economyCacheEntries = 16

type economyCacheEntry struct {
	ready    chan struct{}
	snapshot Snapshot
	err      error
	expires  time.Time
}

// Preserve the original AsOf. A full normalized filter is the key, so scopes
// and date ranges cannot mix. Consumers treat the delivered snapshot as immutable.
type economySnapshotCache struct {
	mu      sync.Mutex
	entries map[string]*economyCacheEntry
	now     func() time.Time
}

func (c *economySnapshotCache) query(ctx context.Context, key string, build func() (Snapshot, error)) (Snapshot, error) {
	if err := ctx.Err(); err != nil {
		return Snapshot{}, err
	}
	now := time.Now
	if c.now != nil {
		now = c.now
	}
	c.mu.Lock()
	if entry := c.entries[key]; entry != nil {
		if entry.ready != nil {
			ready := entry.ready
			c.mu.Unlock()
			select {
			case <-ctx.Done():
				return Snapshot{}, ctx.Err()
			case <-ready:
				return entry.snapshot, entry.err
			}
		}
		if now().Before(entry.expires) {
			c.mu.Unlock()
			return entry.snapshot, nil
		}
		delete(c.entries, key)
	}
	if c.entries == nil {
		c.entries = make(map[string]*economyCacheEntry)
	}
	if len(c.entries) >= economyCacheEntries {
		oldestKey := ""
		var oldest *economyCacheEntry
		for candidateKey, candidate := range c.entries {
			if candidate.ready == nil && (oldest == nil || candidate.expires.Before(oldest.expires)) {
				oldestKey, oldest = candidateKey, candidate
			}
		}
		if oldest == nil {
			// Never evict an in-flight read or exceed the memory bound.
			c.mu.Unlock()
			return build()
		}
		delete(c.entries, oldestKey)
	}
	entry := &economyCacheEntry{ready: make(chan struct{})}
	c.entries[key] = entry
	c.mu.Unlock()

	snapshot, err := build()
	c.mu.Lock()
	entry.snapshot, entry.err = snapshot, err
	entry.expires = snapshot.AsOf.Add(economyCacheLifetime)
	if err != nil {
		delete(c.entries, key)
	}
	close(entry.ready)
	entry.ready = nil
	c.mu.Unlock()
	return snapshot, err
}
