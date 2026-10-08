package players

import (
	"testing"
	"time"
)

func TestLifecycleNeedsExplicitDespawnAndSupportsBothDirections(t *testing.T) {
	base := time.Now().Add(-time.Minute)
	normal := Observation{PlayerID: newID(), Server: "Fixture", NameType: "normal", Name: "Normal", SessionID: "observer", Epoch: "1", RuntimeID: "7", ObservedAt: base, Model: modelRef(123), Level: intRef(110), Equipment: fixtureEquipment(base), Location: &Location{Region: 1, X: 1, Y: 2, Scope: "synthetic-world-profile"}}
	job := normal
	job.PlayerID = newID()
	job.Name = "Hunter"
	job.NameType = "job"
	job.Job = textRef("hunter")
	job.RuntimeID = "8"
	job.ObservedAt = base.Add(2 * time.Second)
	store := NewLifecycleTracker()
	store.Spawn(normal, true)
	if matches := store.Spawn(job, true); len(matches) != 0 {
		t.Fatal("live coexistence became transition")
	}
	store = NewLifecycleTracker()
	store.Spawn(normal, true)
	if !store.Despawn(normal, base.Add(time.Second), true) {
		t.Fatal("despawn not tracked")
	}
	matches := store.Spawn(job, true)
	if len(matches) != 1 || !AssessTransition(matches[0]).Eligible {
		t.Fatal("explicit transition not reviewable")
	}
	if !store.Despawn(job, base.Add(3*time.Second), true) {
		t.Fatal("job despawn not tracked")
	}
	normal.RuntimeID = "9"
	normal.ObservedAt = base.Add(4 * time.Second)
	matches = store.Spawn(normal, true)
	if len(matches) != 1 || !AssessTransition(matches[0]).Eligible {
		t.Fatal("job back to normal not reviewable")
	}
	store.ResetSession("observer")
	if store.Despawn(normal, base.Add(5*time.Second), true) {
		t.Fatal("reset retained entity")
	}
}
func TestLifecycleRuntimeReuseAndUnverifiedEpochs(t *testing.T) {
	now := time.Now()
	o := Observation{Server: "Fixture", Name: "A", RuntimeID: "7", SessionID: "s", Epoch: "1", ObservedAt: now}
	store := NewLifecycleTracker()
	store.Spawn(o, false)
	if store.Despawn(o, now.Add(time.Second), true) {
		t.Fatal("unverified source became authoritative")
	}
	store.Spawn(o, true)
	o.Name = "B"
	o.ObservedAt = now.Add(time.Second)
	store.Spawn(o, true)
	old := o
	old.Epoch = "old"
	if store.Despawn(old, now.Add(2*time.Second), true) {
		t.Fatal("old epoch reused current runtime ID")
	}
}
