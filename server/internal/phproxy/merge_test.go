package phproxy

import (
	"testing"
	"time"
)

func TestMergeModelRefRejectsOlderTimestamp(t *testing.T) {
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	current := &storedFields{
		ModelRefID:         intPtr(5),
		ModelRefObservedAt: &at,
		FieldEvidence:      map[string]FieldEvidence{},
	}
	vp := validatedPlayer{
		HasModel:   true,
		ModelRefID: intPtr(9),
		Evidence: map[string]FieldEvidence{
			"model_ref_id": {
				ObservedAt: at.Add(-time.Minute), Profile: ProfileName,
				SourceOpcode: OpcodeSpawnEquipment, Reason: EvidenceReasonVerified,
			},
		},
	}
	out := mergeModelRef(current, vp)
	if out.Changed || !out.Stale {
		t.Fatalf("expected stale without change, got %+v", out)
	}
	if *current.ModelRefID != 5 {
		t.Fatal("model should remain")
	}
}

func TestMergeGearEmptyListReplaces(t *testing.T) {
	at := time.Date(2026, 10, 11, 12, 0, 0, 0, time.UTC)
	current := &storedFields{
		Gear:           []GearEntry{{RefItemID: 1}},
		GearObservedAt: timePtr(at.Add(-time.Hour)),
		FieldEvidence:  map[string]FieldEvidence{},
	}
	vp := validatedPlayer{
		HasGear: true,
		Gear:    []GearEntry{},
		Evidence: map[string]FieldEvidence{
			"gear": {
				ObservedAt: at, Profile: ProfileName,
				SourceOpcode: OpcodeSpawnEquipment, Reason: EvidenceReasonVerified,
			},
		},
	}
	out := mergeGear(current, vp)
	if !out.Changed || len(current.Gear) != 0 {
		t.Fatalf("expected cleared gear, got %+v len=%d", out, len(current.Gear))
	}
}

func intPtr(v int) *int              { return &v }
func timePtr(t time.Time) *time.Time { return &t }
