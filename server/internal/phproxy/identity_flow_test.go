package phproxy

import (
	"encoding/json"
	"errors"
	"os"
	"testing"
	"time"
)

func upstreamRequests(t *testing.T) []json.RawMessage {
	t.Helper()
	raw, err := os.ReadFile("testdata/upstream-v2-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Requests []json.RawMessage `json:"requests"`
	}
	if err = json.Unmarshal(raw, &fixture); err != nil {
		t.Fatal(err)
	}
	return fixture.Requests
}

func TestUpstreamV2RequestsAndAliasValidation(t *testing.T) {
	requests := upstreamRequests(t)
	for i, body := range requests {
		req, err := ValidateSubmitBody(body, "22222222-2222-4222-8222-222222222222")
		if err != nil {
			t.Fatalf("upstream request %d: %v", i, err)
		}
		if len(req.Players) != 1 {
			t.Fatalf("request %d missing player", i)
		}
	}
	var request wireRequestV2
	if err := json.Unmarshal(requests[2], &request); err != nil {
		t.Fatal(err)
	}
	tests := []struct {
		name  string
		alter func(*FieldEvidence)
	}{
		{"missing association", func(e *FieldEvidence) { e.Association = nil }},
		{"wrong alias", func(e *FieldEvidence) { e.Association.Alias = "OtherAlias" }},
		{"unsupported status", func(e *FieldEvidence) { e.Association.Status = "verified" }},
		{"unsupported reason", func(e *FieldEvidence) { e.Association.Reason = "same_gear" }},
		{"missing time", func(e *FieldEvidence) { e.Association.ObservedAt = time.Time{} }},
	}
	for _, tc := range tests {
		t.Run(tc.name, func(t *testing.T) {
			var ev FieldEvidence
			if err := json.Unmarshal(request.Players[0].FieldEvidence["gear"], &ev); err != nil {
				t.Fatal(err)
			}
			tc.alter(&ev)
			bad := request
			bad.Players = append([]wirePlayerV2(nil), request.Players...)
			bad.Players[0].FieldEvidence = map[string]json.RawMessage{}
			for k, v := range request.Players[0].FieldEvidence {
				bad.Players[0].FieldEvidence[k] = v
			}
			bad.Players[0].FieldEvidence["gear"], _ = json.Marshal(ev)
			body, _ := json.Marshal(bad)
			_, err := ValidateSubmitBody(body, request.AgentID)
			var detail *ValidationError
			if !errors.As(err, &detail) || detail.Field != "gear" || detail.PlayerIndex == nil || *detail.PlayerIndex != 0 {
				t.Fatalf("expected bounded field rejection, got %v", err)
			}
		})
	}
	t.Run("alias level rejected", func(t *testing.T) {
		var ev FieldEvidence
		_ = json.Unmarshal(request.Players[0].FieldEvidence["gear"], &ev)
		ev.SourceOpcode = OpcodeMetadataB007
		if validateEvidenceV2("level", ev, request.Players[0].Name) == nil {
			t.Fatal("alias level accepted")
		}
	})
	t.Run("v1 remains strict", func(t *testing.T) {
		var ev FieldEvidence
		_ = json.Unmarshal(request.Players[0].FieldEvidence["gear"], &ev)
		if validateEvidenceV1(ev) == nil {
			t.Fatal("v1 accepted alias evidence")
		}
	})
	t.Run("evidence without value", func(t *testing.T) {
		bad := request
		bad.Players = append([]wirePlayerV2(nil), request.Players...)
		bad.Players[0].Gear = nil
		body, _ := json.Marshal(bad)
		_, err := ValidateSubmitBody(body, request.AgentID)
		var detail *ValidationError
		if !errors.As(err, &detail) || detail.Code != "evidence_without_value" || detail.Field != "gear" {
			t.Fatalf("unexpected rejection: %v", err)
		}
	})
}

func TestEqualTimeEvidenceIsPartOfFieldContent(t *testing.T) {
	var req wireRequestV2
	_ = json.Unmarshal(upstreamRequests(t)[2], &req)
	parsed, err := ValidateSubmitBody(upstreamRequests(t)[2], req.AgentID)
	if err != nil {
		t.Fatal(err)
	}
	original := parsed.Players[0]
	current := &storedFields{}
	if out, _ := mergeValidatedPlayer(current, original); !out.Changed {
		t.Fatal("missing initial merge")
	}
	if out, _ := mergeValidatedPlayer(current, original); out.Changed || out.Stale {
		t.Fatalf("identical retry: %+v", out)
	}
	changed := original
	changed.Evidence = map[string]FieldEvidence{}
	for k, v := range original.Evidence {
		changed.Evidence[k] = v
	}
	ev := changed.Evidence["gear"]
	association := *ev.Association
	association.Alias = "OtherAlias"
	ev.Association = &association
	changed.Evidence["gear"] = ev
	if out, _ := mergeValidatedPlayer(current, changed); out.Changed || !out.EqualConflict {
		t.Fatalf("changed association at equal time: %+v", out)
	}
	if current.FieldEvidence["gear"].Association.Alias != original.Evidence["gear"].Association.Alias {
		t.Fatal("association overwritten")
	}
	if current.ProxyJob != nil || current.ProxyJobLevel != nil {
		t.Fatal("alias fields became direct fallbacks")
	}
}

func TestKnownEmptyListsAndEvidenceValidation(t *testing.T) {
	at := time.Now().UTC()
	vp := validatedPlayer{HasGear: true, HasJobItemIDs: true, HasIdentityLinks: true, Gear: []GearEntry{}, JobItemIDs: []int{}, IdentityLinks: []IdentityLink{}, Evidence: map[string]FieldEvidence{}}
	for _, key := range []string{"gear", "job_item_ids", "identity_links"} {
		vp.Evidence[key] = FieldEvidence{ObservedAt: at, Profile: ProfileName, SourceOpcode: OpcodeSpawnEquipment, Reason: EvidenceReasonVerified}
	}
	current := &storedFields{}
	if out, _ := mergeValidatedPlayer(current, vp); !out.Changed {
		t.Fatal("complete empty lists not accepted")
	}
	if out, _ := mergeValidatedPlayer(current, vp); out.Changed || out.Stale {
		t.Fatalf("empty-list exact retry: %+v", out)
	}
	projection := NewStore(nil, nil).projectEnrichment("Fixture", current)
	if projection == nil {
		t.Fatal("empty known enrichment disappeared")
	}
	raw, err := json.Marshal(projection)
	if err != nil {
		t.Fatal(err)
	}
	var read map[string]json.RawMessage
	_ = json.Unmarshal(raw, &read)
	for _, key := range []string{"gear", "job_item_ids", "identity_links"} {
		if string(read[key]) != "[]" {
			t.Fatalf("%s not a known empty list: %s", key, raw)
		}
	}
	var request wireRequestV2
	_ = json.Unmarshal(upstreamRequests(t)[0], &request)
	var value map[string]any
	_ = json.Unmarshal(request.Players[0].FieldEvidence["gear"], &value)
	value["unsupported_private_field"] = "must-not-leak"
	request.Players[0].FieldEvidence["gear"], _ = json.Marshal(value)
	body, _ := json.Marshal(request)
	_, err = ValidateSubmitBody(body, request.AgentID)
	if !errors.Is(err, ErrInvalid) {
		t.Fatal("unsupported evidence field accepted")
	}
}
