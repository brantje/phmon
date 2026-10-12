package phproxy

import (
	"strings"
	"testing"
)

func TestValidateSubmitBodyRejectsUppercaseUUID(t *testing.T) {
	body := strings.Replace(
		validFixtureBody(),
		"11111111-1111-4111-8111-111111111111",
		"11111111-1111-4111-8111-11111111111A",
		1,
	)
	_, err := ValidateSubmitBody([]byte(body), "22222222-2222-4222-8222-222222222222")
	if err == nil {
		t.Fatal("expected invalid request")
	}
}

func TestValidateSubmitBodyRejectsAgentMismatch(t *testing.T) {
	_, err := ValidateSubmitBody([]byte(validFixtureBody()), "33333333-3333-4333-8333-333333333333")
	if _, ok := err.(AgentIDMismatch); !ok {
		t.Fatalf("expected agent mismatch, got %v", err)
	}
}

func TestValidateSubmitBodyAcceptsNameOnlyAndNullGatewayCounts(t *testing.T) {
	body := `{
	  "schema_version": 1,
	  "request_id": "11111111-1111-4111-8111-111111111111",
	  "agent_id": "22222222-2222-4222-8222-222222222222",
	  "server": "Greatest",
	  "players": [{
	    "name": "NameOnly",
	    "identity_kind": "character_name",
	    "field_evidence": {
	      "name": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified"}
	    }
	  }],
	  "server_metadata": {
	    "gateway_name": "GreatestSRO",
	    "online": null,
	    "max": null,
	    "observed_at": "2026-10-11T12:00:00Z"
	  },
	  "coverage": {"profile": "silkroad-equipment-v1", "dropped_observations": 0, "capacity_reached": false}
	}`
	req, err := ValidateSubmitBody([]byte(body), "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Players) != 1 || req.Players[0].HasModel || req.Players[0].HasGear {
		t.Fatalf("expected name-only player, got %+v", req.Players)
	}
	if req.ServerMetadata == nil || req.ServerMetadata.Online != nil || req.ServerMetadata.Max != nil {
		t.Fatalf("expected null gateway counts, got %+v", req.ServerMetadata)
	}
}

func TestValidateSubmitBodyAcceptsMetadataOnlyBatch(t *testing.T) {
	body := `{
	  "schema_version": 1,
	  "request_id": "11111111-1111-4111-8111-111111111111",
	  "agent_id": "22222222-2222-4222-8222-222222222222",
	  "server": "Greatest",
	  "players": [],
	  "server_metadata": {
	    "gateway_name": "GreatestSRO",
	    "online": 10,
	    "max": 100,
	    "observed_at": "2026-10-11T12:00:00Z"
	  }
	}`
	req, err := ValidateSubmitBody([]byte(body), "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Players) != 0 || req.ServerMetadata == nil || req.ServerMetadata.Online == nil || *req.ServerMetadata.Online != 10 {
		t.Fatalf("unexpected metadata-only request: %+v", req)
	}
}

func TestValidateSubmitBodyAcceptsFixture(t *testing.T) {
	req, err := ValidateSubmitBody([]byte(validFixtureBody()), "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatal(err)
	}
	if len(req.Players) != 1 || !req.Players[0].HasGear {
		t.Fatalf("unexpected players: %+v", req.Players)
	}
}

func validFixtureBody() string {
	return `{
  "schema_version": 1,
  "request_id": "11111111-1111-4111-8111-111111111111",
  "agent_id": "22222222-2222-4222-8222-222222222222",
  "server": "Greatest",
  "players": [{
    "name": "FixturePlayer",
    "identity_kind": "character_name",
    "model_ref_id": 100,
    "gear": [{"ref_item_id": 200, "opt_level": 7, "slot": null}],
    "field_evidence": {
      "name": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified"},
      "model_ref_id": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified"},
      "gear": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified"}
    }
  }]
}`
}
