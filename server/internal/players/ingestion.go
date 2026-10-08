package players

import (
	"context"
	"encoding/json"
	"log/slog"
	"sort"
	"sync"
	"time"
)

const (
	maxPendingObservations = 4096
	maxPendingBytes        = 8 * 1024 * 1024
	maxAdmissionCache      = 8192
)

type IngestionStatus struct {
	Pending    int        `json:"pending"`
	Overflow   uint64     `json:"overflow"`
	LastError  string     `json:"last_error,omitempty"`
	LastCommit *time.Time `json:"last_commit_at,omitempty"`
}
type admitted struct {
	signature string
	at        time.Time
}
type queuedObservation struct {
	observation Observation
	size        int
	key         string
	pendingKey  string
}
type pendingObservations struct {
	mu         sync.Mutex
	items      map[string]queuedObservation
	recent     map[string]admitted
	latest     map[string]string
	bytes      int
	overflow   uint64
	lastError  string
	lastCommit *time.Time
}

func newPending() *pendingObservations {
	return &pendingObservations{items: map[string]queuedObservation{}, recent: map[string]admitted{}, latest: map[string]string{}}
}
func admissionKey(o Observation) string {
	return ServerKey(o.Server) + "\x00" + o.SessionID + "\x00" + o.Epoch + "\x00" + o.RuntimeID + "\x00" + o.Source + "\x00" + o.Name
}
func (s *Store) Status() IngestionStatus {
	if s == nil || s.pending == nil {
		return IngestionStatus{LastError: "unavailable"}
	}
	p := s.pending
	p.mu.Lock()
	defer p.mu.Unlock()
	var stamp *time.Time
	if p.lastCommit != nil {
		copy := *p.lastCommit
		stamp = &copy
	}
	return IngestionStatus{Pending: len(p.items), Overflow: p.overflow, LastError: p.lastError, LastCommit: stamp}
}
func (s *Store) SubmitLive(snapshot LiveSnapshot) bool {
	accepted := true
	for _, o := range LiveObservations(snapshot) {
		if !s.submit(o) {
			accepted = false
		}
	}
	return accepted
}
func (s *Store) submit(o Observation) bool {
	if s == nil || s.pending == nil || o.normalize() != nil {
		return false
	}
	raw, err := json.Marshal(o)
	if err != nil {
		return false
	}
	p := s.pending
	key := admissionKey(o)
	signature := o.signature()
	pendingKey := key + "\x00" + o.ID
	p.mu.Lock()
	defer p.mu.Unlock()
	if previous, ok := p.items[p.latest[key]]; ok && previous.observation.signature() == signature {
		pendingKey = previous.pendingKey
		if !o.LastSeen.After(previous.observation.LastSeen) {
			return true
		}
		o.ID = previous.observation.ID
		o.SourceRef = previous.observation.SourceRef
		o.ObservedAt = previous.observation.ObservedAt
		o.ReceivedAt = previous.observation.ReceivedAt
		raw, _ = json.Marshal(o)
		size := len(raw)
		if p.bytes-previous.size+size > maxPendingBytes {
			p.overflow++
			return false
		}
		p.bytes += size - previous.size
		p.items[pendingKey] = queuedObservation{o, size, key, pendingKey}
		return true
	}
	if len(p.items) >= maxPendingObservations || p.bytes+len(raw) > maxPendingBytes {
		p.overflow++
		p.lastError = "pending queue full; transient sightings may be lost"
		return false
	}
	p.items[pendingKey] = queuedObservation{o, len(raw), key, pendingKey}
	p.latest[key] = pendingKey
	p.bytes += len(raw)
	return true
}

func (s *Store) flush(ctx context.Context) error {
	p := s.pending
	p.mu.Lock()
	ordered := make([]queuedObservation, 0, len(p.items))
	for _, v := range p.items {
		ordered = append(ordered, v)
	}
	sort.Slice(ordered, func(i, j int) bool {
		if ordered[i].observation.ObservedAt.Equal(ordered[j].observation.ObservedAt) {
			return ordered[i].observation.ID < ordered[j].observation.ID
		}
		return ordered[i].observation.ObservedAt.Before(ordered[j].observation.ObservedAt)
	})
	// Compare against the preceding pending configuration too. A committed A,
	// then queued B→A within 30 seconds is a reversal, not an A checkpoint.
	preceding := map[string]admitted{}
	batch := []queuedObservation{}
	for _, v := range ordered {
		last, ok := preceding[v.key]
		if !ok {
			last, ok = p.recent[v.key]
		}
		if ok && last.signature == v.observation.signature() && v.observation.LastSeen.Before(last.at.Add(CheckpointInterval)) {
			continue
		}
		batch = append(batch, v)
		preceding[v.key] = admitted{v.observation.signature(), v.observation.LastSeen}
		if len(batch) == MaxPlayers {
			break
		}
	}
	p.mu.Unlock()
	if len(batch) == 0 {
		return nil
	}
	observations := make([]Observation, len(batch))
	for i, v := range batch {
		observations[i] = v.observation
	}
	err := s.ApplyObservations(ctx, observations)
	p.mu.Lock()
	defer p.mu.Unlock()
	if err != nil {
		p.lastError = "database persistence retrying"
		return err
	}
	now := time.Now().UTC()
	p.lastCommit = &now
	p.lastError = ""
	for _, v := range batch {
		signature := v.observation.signature()
		key := v.pendingKey
		current, ok := p.items[key]
		if ok && !current.observation.LastSeen.After(v.observation.LastSeen) {
			delete(p.items, key)
			p.bytes -= current.size
			if p.latest[v.key] == key {
				delete(p.latest, v.key)
			}
		} else if ok {
			// Newer unchanged data arrived during commit. Give its next checkpoint
			// a new evidence ID; replay of the committed row remains immutable.
			delete(p.items, key)
			current.observation.ObservedAt = current.observation.LastSeen
			current.observation.ID = ""
			current.observation.SourceRef = ""
			_ = current.observation.normalize()
			current.pendingKey = current.key + "\x00" + current.observation.ID
			p.items[current.pendingKey] = current
			if p.latest[v.key] == key {
				p.latest[v.key] = current.pendingKey
			}
		}
		p.recent[v.key] = admitted{signature, v.observation.LastSeen}
	}
	if len(p.recent) > maxAdmissionCache {
		keys := []string{}
		for key := range p.recent {
			keys = append(keys, key)
		}
		sort.Slice(keys, func(i, j int) bool { return p.recent[keys[i]].at.Before(p.recent[keys[j]].at) })
		for _, key := range keys[:len(keys)/2] {
			delete(p.recent, key)
		}
	}
	// Checkpoint-only pending rows cannot occupy the queue forever after a player
	// leaves. Preserve their final observation by flushing when its cache ages out.
	return nil
}
func (s *Store) Run(ctx context.Context) {
	ticker := time.NewTicker(time.Second)
	defer ticker.Stop()
	lastLog := time.Time{}
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			p := s.pending
			p.mu.Lock()
			now := time.Now()
			for key, last := range p.recent {
				if now.Sub(last.at) > CheckpointInterval {
					delete(p.recent, key)
				}
			}
			p.mu.Unlock()
			run, cancel := context.WithTimeout(ctx, 3*time.Second)
			err := s.flush(run)
			cancel()
			if err != nil && time.Since(lastLog) > 30*time.Second {
				slog.Warn("player registry persistence retrying")
				lastLog = time.Now()
			}
		}
	}
}
