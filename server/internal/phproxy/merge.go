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
		if intPtrEqual(current.ModelRefID, vp.ModelRefID) {
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
		if gearEqual(current.Gear, vp.Gear) {
			return mergeOutcome{}
		}
		return mergeOutcome{Stale: true, EqualConflict: true}
	}
}

func mergeValidatedPlayer(current *storedFields, vp validatedPlayer) (mergeOutcome, error) {
	if current.FieldEvidence == nil {
		current.FieldEvidence = map[string]FieldEvidence{}
	}
	model := mergeModelRef(current, vp)
	gear := mergeGear(current, vp)
	v2 := mergeV2Fields(current, vp)
	out := mergeOutcome{
		Changed:       model.Changed || gear.Changed || v2.Changed,
		Stale:         model.Stale || gear.Stale || v2.Stale,
		EqualConflict: model.EqualConflict || gear.EqualConflict || v2.EqualConflict,
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
	return reflect.DeepEqual(a, b)
}

func gearToJSON(gear []GearEntry) ([]byte, error) {
	return json.Marshal(gear)
}

func evidenceToJSON(ev map[string]FieldEvidence) ([]byte, error) {
	return json.Marshal(ev)
}

func mergeStoredInto(dst, src *storedFields) bool {
	if dst.FieldEvidence == nil {
		dst.FieldEvidence = map[string]FieldEvidence{}
	}
	changed := false
	if src.ModelRefID != nil && src.ModelRefObservedAt != nil {
		vp := validatedPlayer{
			HasModel:   true,
			ModelRefID: src.ModelRefID,
			Evidence: map[string]FieldEvidence{
				"model_ref_id": evidenceForField(src, "model_ref_id", *src.ModelRefObservedAt),
			},
		}
		if mergeModelRef(dst, vp).Changed {
			changed = true
		}
	}
	if src.GearObservedAt != nil {
		vp := validatedPlayer{
			HasGear: true,
			Gear:    append([]GearEntry(nil), src.Gear...),
			Evidence: map[string]FieldEvidence{
				"gear": evidenceForField(src, "gear", *src.GearObservedAt),
			},
		}
		if mergeGear(dst, vp).Changed {
			changed = true
		}
	}
	return changed
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
