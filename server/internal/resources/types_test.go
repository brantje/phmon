package resources

import (
	"encoding/json"
	"errors"
	"testing"
)

func TestValidateFullResourceSnapshot(t *testing.T) {
	snapshot := Snapshot{
		Revision: 1,
		Full:     true,
		Resources: map[string]json.RawMessage{
			"inventory":     json.RawMessage(`{"availability":"observed","capacity":2,"slots":[null,{"source_slot":14,"displayed_slot":1,"item":{"model":7,"quantity":200}}]}`),
			"guild_storage": json.RawMessage(`{"availability":"not_observed","reason":"getter_unavailable_or_not_open"}`),
		},
	}
	observations, hashes, err := Validate(snapshot)
	if err != nil {
		t.Fatal(err)
	}
	if observations["inventory"].Availability != "observed" || observations["guild_storage"].Availability != "not_observed" {
		t.Fatalf("availability was not preserved: %+v", observations)
	}
	if len(hashes) != 2 || hashes["inventory"] == ([32]byte{}) {
		t.Fatalf("canonical content hashes missing: %+v", hashes)
	}
}

func TestValidateRejectsInvalidSequenceAndObservation(t *testing.T) {
	valid := json.RawMessage(`{"availability":"observed","slots":[]}`)
	cases := []Snapshot{
		{Revision: 1, Full: false, BaseRevision: 0, Resources: map[string]json.RawMessage{"inventory": valid}},
		{Revision: 0, Full: true, Resources: map[string]json.RawMessage{"inventory": valid}},
		{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"inventory/unsafe": valid}},
		{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"inventory": json.RawMessage(`{"availability":"empty"}`)}},
		{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"inventory": json.RawMessage(`[]`)}},
	}
	for i, snapshot := range cases {
		if _, _, err := Validate(snapshot); !errors.Is(err, ErrInvalid) {
			t.Errorf("case %d: expected ErrInvalid, got %v", i, err)
		}
	}
}

func TestDecodeRejectsUnknownFieldsAndOversizedFrames(t *testing.T) {
	if _, err := Decode(json.RawMessage(`{"revision":1,"full":true,"resources":{"inventory":{"availability":"observed"}},"extra":true}`)); err == nil {
		t.Fatal("unknown fields must not be accepted")
	}
	if _, err := Decode(make([]byte, MaxFrameBytes+1)); !errors.Is(err, ErrInvalid) {
		t.Fatalf("oversized frame error = %v", err)
	}
}

func TestAssemblyAcceptsOutOfOrderChunksAndRejectsDuplicateResources(t *testing.T) {
	assembly := &Assembly{}
	_, done, err := assembly.Add(Snapshot{
		Revision: 1, Full: true,
		Resources: map[string]json.RawMessage{"pets": json.RawMessage(`{"availability":"observed","pets":[]}`)},
	}, 1, 2)
	if err != nil || done {
		t.Fatalf("first out-of-order chunk: done=%v err=%v", done, err)
	}
	full, done, err := assembly.Add(Snapshot{
		Revision: 1, Full: true,
		Resources: map[string]json.RawMessage{"inventory": json.RawMessage(`{"availability":"observed","slots":[]}`)},
	}, 0, 2)
	if err != nil || !done || len(full.Resources) != 2 {
		t.Fatalf("completed snapshot=%+v done=%v err=%v", full, done, err)
	}

	duplicate := &Assembly{}
	_, _, err = duplicate.Add(Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"inventory": json.RawMessage(`{"availability":"observed"}`)}}, 0, 2)
	if err != nil {
		t.Fatal(err)
	}
	if _, _, err := duplicate.Add(Snapshot{Revision: 1, Full: true, Resources: map[string]json.RawMessage{"inventory": json.RawMessage(`{"availability":"observed"}`)}}, 1, 2); !errors.Is(err, ErrInvalid) {
		t.Fatalf("duplicate resource key error = %v", err)
	}
}
