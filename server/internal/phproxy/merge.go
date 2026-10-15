package phproxy

import (
	"encoding/json"
	"reflect"
	"time"
)

type storedFields struct {
	ModelRefID         *int
	ModelRefObservedAt *time.Time
	Gear               []GearEntry
	GearObservedAt     *time.Time
	FieldEvidence      map[string]FieldEvidence
	Extended           ExtendedPayload
	ProxyLevel         *int
	ProxyJob           *string
	ProxyJobLevel      *int
	ProxyGuild         *string
	ProxyRace          *string
}

type mergeOutcome struct {
	Changed       bool
	Stale         bool
	EqualConflict bool
}

func mergeFieldEvidence(current map[string]FieldEvidence, incoming map[string]FieldEvidence) map[string]FieldEvidence {
	if len(incoming) == 0 {
		return current
	}
	out := make(map[string]FieldEvidence, len(current)+len(incoming))
	for k, v := range current {
		out[k] = v
	}
	for k, v := range incoming {
		out[k] = v
	}
	return out
}

func mergeModelRef(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasModel {
		return mergeOutcome{}
	}
	at := vp.Evidence["model_ref_id"].ObservedAt.UTC()
	if current.ModelRefObservedAt == nil {
		current.ModelRefID = vp.ModelRefID
		current.ModelRefObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"model_ref_id": vp.Evidence["model_ref_id"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.ModelRefObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.ModelRefID = vp.ModelRefID
		current.ModelRefObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"model_ref_id": vp.Evidence["model_ref_id"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if intPtrEqual(current.ModelRefID, vp.ModelRefID) && evidenceEqual(current.FieldEvidence["model_ref_id"], vp.Evidence["model_ref_id"]) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeGear(current *storedFields, vp validatedPlayer) mergeOutcome {
	if !vp.HasGear {
		return mergeOutcome{}
	}
	at := vp.Evidence["gear"].ObservedAt.UTC()
	if current.GearObservedAt == nil {
		current.Gear = append([]GearEntry(nil), vp.Gear...)
		current.GearObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"gear": vp.Evidence["gear"],
		})
		return mergeOutcome{Changed: true}
	}
	curAt := current.GearObservedAt.UTC()
	switch {
	case at.After(curAt):
		current.Gear = append([]GearEntry(nil), vp.Gear...)
		current.GearObservedAt = &at
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{
			"gear": vp.Evidence["gear"],
		})
		return mergeOutcome{Changed: true}
	case at.Before(curAt):
		return mergeOutcome{Stale: true}
	default:
		if gearEqual(current.Gear, vp.Gear) && evidenceEqual(current.FieldEvidence["gear"], vp.Evidence["gear"]) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeValidatedPlayer(current *storedFields, vp validatedPlayer) (mergeOutcome, error) {
	if current.FieldEvidence == nil {
		current.FieldEvidence = map[string]FieldEvidence{}
	}
	name := mergeNameEvidence(current, vp)
	model := mergeModelRef(current, vp)
	gear := mergeGear(current, vp)
	v2 := mergeV2Fields(current, vp)
	out := mergeOutcome{
		Changed:       name.Changed || model.Changed || gear.Changed || v2.Changed,
		Stale:         name.Stale || model.Stale || gear.Stale || v2.Stale,
		EqualConflict: name.EqualConflict || model.EqualConflict || gear.EqualConflict || v2.EqualConflict,
	}
	if out.Changed {
		syncProxyFallbacks(current)
	}
	return out, nil
}

func playerResultStatus(out mergeOutcome, matched bool) string {
	if out.Changed {
		if matched {
			return "applied"
		}
		return "pending"
	}
	if out.Stale || out.EqualConflict {
		return "stale"
	}
	return "unchanged"
}

func intPtrEqual(a, b *int) bool {
	if a == nil && b == nil {
		return true
	}
	if a == nil || b == nil {
		return false
	}
	return *a == *b
}

func gearEqual(a, b []GearEntry) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if !reflect.DeepEqual(a[i], b[i]) {
			return false
		}
	}
	return true
}

func gearToJSON(gear []GearEntry) ([]byte, error) {
	return json.Marshal(gear)
}

func evidenceToJSON(ev map[string]FieldEvidence) ([]byte, error) {
	return json.Marshal(ev)
}

// Promotion uses the same field/evidence freshness rules as direct ingestion.
func mergeStoredInto(dst, src *storedFields) bool {
	vp := validatedPlayer{
		ModelRefID: src.ModelRefID, HasModel: src.ModelRefObservedAt != nil,
		Gear: src.Gear, HasGear: src.GearObservedAt != nil,
		AvatarGear: src.Extended.AvatarGear, HasAvatarGear: src.Extended.AvatarGearObservedAt != nil,
		Level: src.Extended.Level, HasLevel: src.Extended.LevelObservedAt != nil,
		Job: src.Extended.Job, HasJob: src.Extended.JobObservedAt != nil,
		JobLevel: src.Extended.JobLevel, HasJobLevel: src.Extended.JobLevelObservedAt != nil,
		Race: src.Extended.Race, HasRace: src.Extended.RaceObservedAt != nil,
		Guild: src.Extended.Guild, HasGuild: src.Extended.GuildObservedAt != nil,
		Position: src.Extended.Position, HasPosition: src.Extended.PositionObservedAt != nil,
		JobItemIDs: src.Extended.JobItemIDs, HasJobItemIDs: src.Extended.JobItemIDsObservedAt != nil,
		IdentityLinks: src.Extended.IdentityLinks, HasIdentityLinks: src.Extended.IdentityLinksObservedAt != nil,
		Evidence: src.FieldEvidence,
	}
	out, _ := mergeValidatedPlayer(dst, vp)
	return out.Changed
}

func evidenceEqual(a, b FieldEvidence) bool {
	a.ObservedAt, b.ObservedAt = a.ObservedAt.UTC(), b.ObservedAt.UTC()
	if a.Association != nil {
		copy := *a.Association
		copy.ObservedAt = copy.ObservedAt.UTC()
		a.Association = &copy
	}
	if b.Association != nil {
		copy := *b.Association
		copy.ObservedAt = copy.ObservedAt.UTC()
		b.Association = &copy
	}
	return reflect.DeepEqual(a, b)
}

func mergeNameEvidence(current *storedFields, vp validatedPlayer) mergeOutcome {
	ev, ok := vp.Evidence["name"]
	if !ok {
		return mergeOutcome{}
	}
	old, exists := current.FieldEvidence["name"]
	if !exists || ev.ObservedAt.After(old.ObservedAt) {
		current.FieldEvidence = mergeFieldEvidence(current.FieldEvidence, map[string]FieldEvidence{"name": ev})
		return mergeOutcome{Changed: true}
	}
	if ev.ObservedAt.Before(old.ObservedAt) {
		return mergeOutcome{Stale: true}
	}
	if !evidenceEqual(old, ev) {
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
	return mergeOutcome{}
}

func evidenceForField(st *storedFields, key string, at time.Time) FieldEvidence {
	if ev, ok := st.FieldEvidence[key]; ok {
		return ev
	}
	return FieldEvidence{
		ObservedAt:   at,
		Profile:      ProfileName,
		SourceOpcode: OpcodeSpawnEquipment,
		Reason:       EvidenceReasonVerified,
	}
}
