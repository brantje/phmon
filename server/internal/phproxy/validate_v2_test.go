package phproxy

import (
	"strings"
	"testing"
)

func TestValidateSubmitV2AcceptsExtendedFixture(t *testing.T) {
	body := validV2FixtureBody()
	req, err := ValidateSubmitBody([]byte(body), "22222222-2222-4222-8222-222222222222")
	if err != nil {
		t.Fatal(err)
	}
	if req.SchemaVersion != SchemaVersionV2 || len(req.Players) != 1 {
		t.Fatalf("unexpected request: %+v", req)
	}
	if !req.Players[0].HasLevel || !req.Players[0].HasIdentityLinks {
		t.Fatalf("expected level and identity links: %+v", req.Players[0])
	}
}

func TestValidateSubmitV2PlayerNameMatchesPendingByteLimit(t *testing.T) {
	agentID := "22222222-2222-4222-8222-222222222222"
	accepted := strings.ReplaceAll(validV2FixtureBody(), "FixturePlayer", strings.Repeat("A", MaxPlayerNameBytesV2))
	if _, err := ValidateSubmitBody([]byte(accepted), agentID); err != nil {
		t.Fatal(err)
	}
	rejected := strings.ReplaceAll(validV2FixtureBody(), "FixturePlayer", strings.Repeat("A", MaxPlayerNameBytesV2+1))
	if _, err := ValidateSubmitBody([]byte(rejected), agentID); err == nil {
		t.Fatal("expected a player name above 128 bytes to be rejected")
	}
}

func TestValidateSubmitV2RejectsAliasEvidenceOnDirectLevel(t *testing.T) {
	body := strings.Replace(
		validV2FixtureBody(),
		`"source_identity_kind": "character_name"`,
		`"source_identity_kind": "job_alias"`,
		1,
	)
	_, err := ValidateSubmitBody([]byte(body), "22222222-2222-4222-8222-222222222222")
	if err == nil {
		t.Fatal("expected invalid request")
	}
}

func validV2FixtureBody() string {
	return `{
  "schema_version": 2,
  "request_id": "11111111-1111-4111-8111-111111111111",
  "agent_id": "22222222-2222-4222-8222-222222222222",
  "server": "Greatest",
  "players": [{
    "name": "FixturePlayer",
    "identity_kind": "character_name",
    "model_ref_id": 100,
    "gear": [{"ref_item_id": 200, "opt_level": 7, "slot": null}],
    "level": 110,
    "job_item_ids": [],
    "identity_links": [{
      "alias": "JobAlias",
      "status": "inferred",
      "reason": "job_item_transition",
      "first_matched": "2026-10-11T12:00:00Z",
      "last_matched": "2026-10-11T12:01:00Z",
      "matches": 2,
      "profile": "silkroad-equipment-v1",
      "source_opcode": 12309
    }],
    "field_evidence": {
      "name": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified", "source_identity_kind": "character_name", "source_name": "FixturePlayer"},
      "model_ref_id": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified", "source_identity_kind": "character_name", "source_name": "FixturePlayer"},
      "gear": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified", "source_identity_kind": "character_name", "source_name": "FixturePlayer"},
      "level": {"observed_at": "2026-10-11T12:00:01Z", "profile": "silkroad-equipment-v1", "source_opcode": 45063, "reason": "verified", "source_identity_kind": "character_name", "source_name": "FixturePlayer"},
      "job_item_ids": {"observed_at": "2026-10-11T12:00:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "verified", "source_identity_kind": "character_name", "source_name": "FixturePlayer"},
      "identity_links": {"observed_at": "2026-10-11T12:01:00Z", "profile": "silkroad-equipment-v1", "source_opcode": 12309, "reason": "inferred_job_item_transition", "source_identity_kind": "character_name", "source_name": "FixturePlayer"}
    }
  }]
}`
}
