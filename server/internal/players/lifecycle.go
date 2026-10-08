package players

import (
	"sync"
	"time"
)

type lifecycleEntity struct {
	observation Observation
	incarnation uint64
}
type lifecycleSession struct {
	active    map[string]lifecycleEntity
	despawned []TransitionSource
	sequence  uint64
	last      time.Time
}
type TransitionSource struct {
	Observation Observation
	DespawnAt   time.Time
	Incarnation uint64
}

// LifecycleTracker is fed only by a verified server-to-client decoder. Ordinary
// map snapshots never call Despawn. It bounds retained observer epochs and clears
// transition continuity on overflow rather than guessing at an incomplete set.
type LifecycleTracker struct {
	mu       sync.Mutex
	sessions map[string]*lifecycleSession
}

func NewLifecycleTracker() *LifecycleTracker {
	return &LifecycleTracker{sessions: map[string]*lifecycleSession{}}
}
func lifecycleKey(o Observation) string {
	return ServerKey(o.Server) + "\x00" + o.SessionID + "\x00" + o.Epoch
}

func (s *LifecycleTracker) Spawn(o Observation, verified bool) []TransitionEvidence {
	if !verified || o.SessionID == "" || o.RuntimeID == "" || o.Epoch == "" || o.ObservedAt.IsZero() {
		return nil
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	now := o.ObservedAt
	for key, session := range s.sessions {
		if now.Sub(session.last) > 60*time.Second {
			delete(s.sessions, key)
		}
	}
	key := lifecycleKey(o)
	session := s.sessions[key]
	if session == nil {
		if len(s.sessions) >= 64 {
			return nil
		}
		session = &lifecycleSession{active: map[string]lifecycleEntity{}}
		s.sessions[key] = session
	}
	if now.Before(session.last) {
		return nil
	}
	session.last = now
	if previous, ok := session.active[o.RuntimeID]; ok {
		// A repeated spawn or an ID recycled without a despawn invalidates any
		// continuity for that incarnation. It can never be a normal/job transition.
		delete(session.active, o.RuntimeID)
		session.despawned = nil
		_ = previous
	}
	if len(session.active) >= 128 {
		session.active = map[string]lifecycleEntity{}
		session.despawned = nil
		return nil
	}
	session.sequence++
	session.active[o.RuntimeID] = lifecycleEntity{o, session.sequence}
	recent := []TransitionSource{}
	candidates := []TransitionEvidence{}
	for _, source := range session.despawned {
		if now.Sub(source.DespawnAt) > 15*time.Second {
			continue
		}
		recent = append(recent, source)
		before := source.Observation
		normal, job := before, o
		if before.NameType == "job" && o.NameType == "normal" {
			normal, job = o, before
		}
		evidence := TransitionEvidence{Normal: normal, Job: job, DespawnAt: source.DespawnAt, LifecycleVerified: true}
		for _, active := range session.active {
			if active.observation.PlayerID == before.PlayerID && active.observation.RuntimeID != o.RuntimeID {
				evidence.Simultaneous = true
			}
		}
		if AssessTransition(evidence).Eligible {
			candidates = append(candidates, evidence)
		}
	}
	session.despawned = recent
	for i := range candidates {
		candidates[i].Competing = len(candidates)
	}
	return candidates
}
func (s *LifecycleTracker) Despawn(o Observation, at time.Time, verified bool) bool {
	if !verified {
		return false
	}
	s.mu.Lock()
	defer s.mu.Unlock()
	session := s.sessions[lifecycleKey(o)]
	if session == nil {
		return false
	}
	entity, ok := session.active[o.RuntimeID]
	if !ok || at.Before(entity.observation.ObservedAt) || at.Before(session.last) {
		return false
	}
	delete(session.active, o.RuntimeID)
	session.last = at
	if len(session.despawned) >= 128 {
		session.despawned = nil
		return false
	}
	session.despawned = append(session.despawned, TransitionSource{entity.observation, at, entity.incarnation})
	return true
}
func (s *LifecycleTracker) ResetSession(sessionID string) {
	s.mu.Lock()
	defer s.mu.Unlock()
	for key, session := range s.sessions {
		for _, entity := range session.active {
			if entity.observation.SessionID == sessionID {
				delete(s.sessions, key)
				break
			}
		}
		for _, entity := range session.despawned {
			if entity.Observation.SessionID == sessionID {
				delete(s.sessions, key)
				break
			}
		}
	}
}
