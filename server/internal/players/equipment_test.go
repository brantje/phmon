package players

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

func intRef(v int) *int        { return &v }
func modelRef(v int64) *int64  { return &v }
func textRef(v string) *string { return &v }
func fixtureEquipment(at time.Time) *Equipment {
	e := &Equipment{Availability: "observed_partial", Source: "synthetic.test", Profile: "synthetic-fixture", ObservedAt: at, IdentityVerified: true, CharacterModel: modelRef(123)}
	for i, name := range EquipmentSlots[:6] {
		e.Slots = append(e.Slots, EquipmentSlot{Slot: name, State: "occupied", ModelID: modelRef(int64(100 + i)), Plus: intRef(7), ObservedAt: at})
	}
	return e
}
func TestEquipmentDeterminismCoverageAndPrecision(t *testing.T) {
	now := time.Now().UTC()
	a := fixtureEquipment(now)
	a.Slots[0].Variance = textRef("18446744073709551615")
	a.Slots[0].MagicOptions = map[string]string{"1": "18446744073709551615", "2": "9007199254740993"}
	if err := a.Validate(); err != nil {
		t.Fatal(err)
	}
	raw, _ := json.Marshal(a)
	var b Equipment
	if err := decodeJSON(raw, &b); err != nil {
		t.Fatal(err)
	}
	for i, j := 0, len(b.Slots)-1; i < j; i, j = i+1, j-1 {
		b.Slots[i], b.Slots[j] = b.Slots[j], b.Slots[i]
	}
	if gearHash(a) != gearHash(&b) || identityHash(a) != identityHash(&b) {
		t.Fatal("slot ordering changed fingerprint")
	}
	if *b.Slots[len(b.Slots)-1].Variance != "18446744073709551615" {
		t.Fatal("64 bit precision lost")
	}
	d := uint32(99)
	b.Slots[0].Durability = &d
	if gearHash(a) != gearHash(&b) {
		t.Fatal("durability created a configuration change")
	}
	b.Slots[0].Item = map[string]any{"metadata": map[string]any{"name": "Reference only"}, "reference_level": 110, "icon_url": "/fixture.png", "durability": 2}
	if gearHash(a) != gearHash(&b) {
		t.Fatal("presentation metadata entered the observed configuration hash")
	}
	b.Profile = "different-comparison-profile"
	if identityHash(a) == identityHash(&b) {
		t.Fatal("identity hash omitted its comparison profile")
	}
	_, count, _ := ComparableEquipment(a, &b)
	if count != 0 {
		t.Fatal("different comparison profiles were treated as comparable")
	}
	b.Profile = a.Profile
	b.Slots = append(b.Slots, EquipmentSlot{Slot: "job", State: "occupied", ModelID: modelRef(999), Plus: intRef(0)})
	if identityHash(a) != identityHash(&b) {
		t.Fatal("job outfit entered identity fingerprint")
	}
	b.IdentityVerified = false
	if identityHash(&b) != "" {
		t.Fatal("unverified identity profile fingerprint enabled")
	}
	b.Availability = "observed_complete"
	if b.Validate() == nil {
		t.Fatal("partial set declared complete")
	}
	bad := *a
	bad.Slots = append(append([]EquipmentSlot{}, a.Slots...), a.Slots[0])
	if bad.Validate() == nil {
		t.Fatal("duplicate slot accepted")
	}
}
func TestEquipmentMergePreservesUnknownAndInvalidatesReplacement(t *testing.T) {
	now := time.Now().UTC()
	old := fixtureEquipment(now)
	old.Slots[0].Variance = textRef("9223372036854775808")
	partial := fixtureEquipment(now.Add(time.Second))
	partial.Slots = partial.Slots[:1]
	merged := mergeEquipment(old, partial)
	var head EquipmentSlot
	for _, slot := range merged.Slots {
		if slot.Slot == "head" {
			head = slot
		}
	}
	if len(merged.Slots) != 6 || head.Variance == nil {
		t.Fatal("partial observation erased known slots/statistics")
	}
	if !head.FieldTimes["variance"].Equal(now) {
		t.Fatal("retained variance was attributed to a newer observation")
	}
	unavailable := &Equipment{Availability: "unavailable", ObservedAt: now.Add(2 * time.Second)}
	missing := mergeEquipment(merged, unavailable)
	if missing.LastAvailability != "unavailable" || gearHash(missing) != gearHash(merged) || len(missing.Slots) != 6 {
		t.Fatal("unavailable observation lost gear knowledge or freshness state")
	}
	replacement := fixtureEquipment(now.Add(2 * time.Second))
	replacement.Slots = replacement.Slots[:1]
	replacement.Slots[0].ModelID = modelRef(800)
	merged = mergeEquipment(merged, replacement)
	for _, slot := range merged.Slots {
		if slot.Slot == "head" && slot.Variance != nil {
			t.Fatal("old stats transferred to replacement")
		}
	}
	late := fixtureEquipment(now.Add(-time.Second))
	late.Slots[0].ModelID = modelRef(777)
	merged = mergeEquipment(merged, late)
	for _, slot := range merged.Slots {
		if slot.Slot == "head" && *slot.ModelID != 800 {
			t.Fatal("late slot overwrote newer item")
		}
	}
	if gearHash(mergeEquipment(merged, &Equipment{Availability: "unavailable"})) != gearHash(merged) {
		t.Fatal("unavailable erased gear")
	}
	matching, comparable, conflict := ComparableEquipment(old, partial)
	if matching != 1 || comparable != 1 || conflict {
		t.Fatal("partial overlap comparison")
	}
}
func TestEquipmentLateAttributesUseFieldTimesAndConfigurationBoundary(t *testing.T) {
	base := time.Now().Add(-time.Minute).UTC()
	known := mergeEquipment(nil, fixtureEquipment(base))
	known = mergeEquipment(known, fixtureEquipment(base.Add(30*time.Second)))
	late := fixtureEquipment(base.Add(10 * time.Second))
	late.Slots = late.Slots[:1]
	late.Slots[0].Variance = textRef("18446744073709551615")
	known = mergeEquipment(known, late)
	head := func(e *Equipment) EquipmentSlot {
		for _, s := range e.Slots {
			if s.Slot == "head" {
				return s
			}
		}
		t.Fatal("missing head")
		return EquipmentSlot{}
	}
	slot := head(known)
	if slot.Variance == nil || *slot.Variance != "18446744073709551615" || !slot.FieldTimes["variance"].Equal(late.ObservedAt) || !slot.ObservedAt.Equal(base.Add(30*time.Second)) {
		t.Fatal("late attribute failed to fill an earlier unknown field")
	}
	older := fixtureEquipment(base.Add(5 * time.Second))
	older.Slots[0].Variance = textRef("1")
	if *head(mergeEquipment(known, older)).Variance != "18446744073709551615" {
		t.Fatal("older variance overwrote a newer field")
	}
	replacement := fixtureEquipment(base.Add(40 * time.Second))
	replacement.Slots[0].ModelID = modelRef(999)
	known = mergeEquipment(known, replacement)
	known = mergeEquipment(known, fixtureEquipment(base.Add(50*time.Second)))
	if head(mergeEquipment(known, late)).Variance != nil {
		t.Fatal("late instance statistics crossed a known replacement/reversal")
	}
}

func TestTransitionEvidenceSafetyForEveryJob(t *testing.T) {
	now := time.Now().Add(-time.Minute).UTC()
	for _, job := range []string{"hunter", "trader", "thief"} {
		t.Run(job, func(t *testing.T) {
			normal := Observation{Server: "Test", Name: "Normal", NameType: "normal", SessionID: "session", Epoch: "1", ObservedAt: now, Model: modelRef(123), Level: intRef(110), Equipment: fixtureEquipment(now), Location: &Location{Region: 1, X: 1, Y: 2, Scope: "synthetic-world-profile"}}
			alias := normal
			alias.Name = "Alias"
			alias.NameType = "job"
			alias.Job = &job
			alias.ObservedAt = now.Add(2 * time.Second)
			e := TransitionEvidence{Normal: normal, Job: alias, DespawnAt: now.Add(time.Second), LifecycleVerified: true}
			a := AssessTransition(e)
			if !a.Eligible || a.Automatic {
				t.Fatalf("expected reviewable, nonautomatic transition %+v", a)
			}
			e.LifecycleVerified = false
			if AssessTransition(e).Eligible {
				t.Fatal("gear-only match linked")
			}
			e.LifecycleVerified = true
			e.Competing = 2
			if AssessTransition(e).Eligible {
				t.Fatal("ambiguous match eligible")
			}
			e.Competing = 0
			e.Simultaneous = true
			if AssessTransition(e).Eligible {
				t.Fatal("simultaneous players eligible")
			}
			e.Simultaneous = false
			e.Job.Server = "Other"
			if AssessTransition(e).Eligible {
				t.Fatal("cross server match eligible")
			}
		})
	}
}
func TestFilterAndCursorAreBoundToContext(t *testing.T) {
	f := Filter{Name: "abc", Sort: "level", Direction: "asc", Limit: 25}
	if f.Validate() != nil {
		t.Fatal("valid filter")
	}
	f.Cursor = encodeCursor(cursor{contextHash(f), "110", "00000000-0000-4000-8000-000000000001", time.Now().UTC(), false})
	if f.Validate() != nil {
		t.Fatal("valid cursor")
	}
	f.Name = "other"
	if f.Validate() == nil {
		t.Fatal("cursor reused with changed filter")
	}
	for _, f := range []Filter{{Sort: "name;DROP TABLE players"}, {Job: "warrior"}, {MinLevel: intRef(200), MaxLevel: intRef(100)}, {Equipment: "fake"}, {Limit: 101}} {
		if f.Validate() == nil {
			t.Fatalf("invalid filter accepted %+v", f)
		}
	}
	if likePattern("a%_\\") != "%a\\%\\_\\\\%" {
		t.Fatal("wildcard escaping")
	}
	if ValidID(strings.Repeat("-", 36)) {
		t.Fatal("invalid UUID accepted")
	}
}
