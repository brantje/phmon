package mobs

import (
	"encoding/json"
	"strings"
	"testing"
	"time"
)

const (
	testAgentID     = "11111111-2222-4333-8444-555555555555"
	testCharacterID = "aaaaaaaa-bbbb-4ccc-8ddd-eeeeeeeeeeee"
	testSessionID   = "ffffffff-1111-4222-8333-444444444444"
	testSampleID    = "00000000-0000-4000-8000-000000000001"
)

func TestEmptySampleIsEligibleAndCoordinatesAreBounded(t *testing.T) {
	now := time.Now().UTC()
	sample := Sample{ID: testSampleID, CharacterID: testCharacterID, SessionID: testSessionID,
		AreaID: "region:25273", FloorID: "unmapped", Region: 25273, SampledAt: now,
		Observer: Position{X: -1, Y: 191.9}, Monsters: []Monster{}}
	if err := ValidateSample(sample, now); err != nil {
		t.Fatalf("valid empty sample rejected: %v", err)
	}
	sample.AreaID = "region:1"
	if err := ValidateSample(sample, now); err == nil {
		t.Fatal("area/region mismatch accepted")
	}
	sample.AreaID = "region:25273"
	sample.Observer.X = 1_000_001
	if err := ValidateSample(sample, now); err == nil {
		t.Fatal("out-of-range observer coordinate accepted")
	}
}

func TestSignedDonwhangRegionAcceptsMonsterRegionAlias(t *testing.T) {
	now := time.Now().UTC()
	sample := Sample{ID: testSampleID, CharacterID: testCharacterID, SessionID: testSessionID,
		AreaID: "region:-32767", FloorID: "unmapped", Region: -32767, SampledAt: now,
		Observer: Position{X: -24300, Y: 20, Z: floatPointer(-9)},
		Monsters: []Monster{{ID: "7", Region: 32767, X: -24300, Y: 20, Z: floatPointer(-9)}}}
	if err := ValidateSample(sample, now); err != nil {
		t.Fatalf("valid signed cave sample rejected: %v", err)
	}
	if err := ValidateLiveSnapshot("observed", -32767, sample.Monsters, now, now); err != nil {
		t.Fatalf("valid signed cave live snapshot rejected: %v", err)
	}
	if err := ValidateDensityFilter(DensityFilter{Server: "greatest", AreaID: "region:-32767", FloorID: "unmapped",
		From: now.Add(-time.Minute), To: now, Limit: 10}); err != nil {
		t.Fatalf("valid signed cave density scope rejected: %v", err)
	}
	if RegionsMatch(-32767, 32766) {
		t.Fatal("unobserved cave region alias accepted")
	}
}

func floatPointer(value float64) *float64 { return &value }

func TestDensityFilterHasRequiredScopeAndBoundedWindow(t *testing.T) {
	now := time.Now().UTC()
	filter := DensityFilter{Server: "greatest", AreaID: "region:25273", FloorID: "unmapped", From: now.Add(-24 * time.Hour), To: now, Limit: 200}
	if err := ValidateDensityFilter(filter); err != nil {
		t.Fatalf("valid filter rejected: %v", err)
	}
	filter.AreaID = "world"
	if err := ValidateDensityFilter(filter); err == nil {
		t.Fatal("non-region aggregation scope accepted")
	}
	filter.AreaID = "region:25273"
	filter.To = filter.From.Add(MaxQueryWindow + time.Second)
	if err := ValidateDensityFilter(filter); err == nil {
		t.Fatal("unbounded time window accepted")
	}
}

func TestObserverLocalMetricJSONDoesNotClaimSpatialDensity(t *testing.T) {
	payload, err := json.Marshal(DensityResult{
		Metric:         "observer_local_average_count",
		Interpretation: "Observed rows divided by eligible observer samples; coverage is unverified and this is not spatial mob density.",
		Cells:          []DensityCell{{ObserverCellX: 2, ObserverCellY: 0, MonsterRows: 1, EligibleSamples: 2, AverageObserved: 0.5}},
	})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	if err := json.Unmarshal(payload, &result); err != nil {
		t.Fatal(err)
	}
	if result["metric"] != "observer_local_average_count" || !strings.Contains(result["interpretation"].(string), "not spatial mob density") {
		t.Fatalf("observer-local limitation is missing from response: %s", payload)
	}
	cell := result["cells"].([]any)[0].(map[string]any)
	if cell["observer_cell_x"] != float64(2) || cell["average_observed_per_sample"] != 0.5 || cell["density"] != nil {
		t.Fatalf("cell response does not distinguish observer-local average from density: %s", payload)
	}
}

func TestCurrentMonsterSnapshotsClearAndExpireSeparately(t *testing.T) {
	store := NewLiveStore()
	now := time.Now().UTC()
	monster := Monster{ID: "42", Region: 25273, X: 3, Y: 4}
	store.Apply(LiveSnapshot{Server: "greatest", AgentID: testAgentID, CharacterID: testCharacterID,
		SessionID: testSessionID, Character: "Alpha", Status: "observed", Region: 25273,
		ObservedAt: now, Monsters: []Monster{monster}})
	if got := store.Snapshot("greatest", now); len(got) != 1 || len(got[0].Monsters) != 1 {
		t.Fatalf("current snapshot missing monster: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "greatest", AgentID: testAgentID, CharacterID: testCharacterID,
		SessionID: testSessionID, Character: "Alpha", Status: "observed", Region: 25273,
		ObservedAt: now.Add(time.Second), Monsters: []Monster{}})
	if got := store.Snapshot("greatest", now.Add(time.Second)); len(got) != 1 || len(got[0].Monsters) != 0 {
		t.Fatalf("empty snapshot did not clear monsters: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "greatest", AgentID: testAgentID, CharacterID: testCharacterID,
		SessionID: testSessionID, Character: "Alpha", Status: "unavailable", Region: 25273,
		ObservedAt: now.Add(2 * time.Second), Monsters: []Monster{}})
	if got := store.Snapshot("greatest", now.Add(2*time.Second)); len(got) != 1 || got[0].Status != "unavailable" || len(got[0].Monsters) != 0 {
		t.Fatalf("unavailable snapshot did not clear rows and preserve status: %+v", got)
	}
	store.Apply(LiveSnapshot{Server: "greatest", AgentID: testAgentID, CharacterID: testCharacterID,
		SessionID: testSessionID, Character: "Alpha", Status: "truncated", Region: 25273,
		ObservedAt: now.Add(3 * time.Second), Truncated: true, Monsters: []Monster{monster}})
	if got := store.Snapshot("greatest", now.Add(3*time.Second)); len(got) != 1 || got[0].Status != "truncated" || !got[0].Truncated || len(got[0].Monsters) != 1 {
		t.Fatalf("truncated current snapshot status was lost: %+v", got)
	}
	if got := store.Snapshot("greatest", now.Add(LiveTTL+4*time.Second)); len(got) != 0 {
		t.Fatalf("expired current snapshot remained: %+v", got)
	}
}

func TestLiveMonsterPopupFieldsAreOptionalAndBounded(t *testing.T) {
	now := time.Now().UTC()
	typeCode, level, hp, maxHP := 20, 72, int64(7515), int64(9000)
	monster := Monster{ID: "46296", Region: 25735, X: 48.8, Y: 1550.7,
		Type: "20", TypeCode: &typeCode, Name: "Eldimmu", ServerName: "MOB_EU_ELDIMMU",
		Level: &level,
		HP:    &hp, MaxHP: &maxHP, Attacking: true}
	if err := ValidateLiveSnapshot("observed", 25735, []Monster{monster}, now, now); err != nil {
		t.Fatalf("documented monster fields rejected: %v", err)
	}
	monster.Name = strings.Repeat("x", 129)
	if err := ValidateLiveSnapshot("observed", 25735, []Monster{monster}, now, now); err == nil {
		t.Fatal("oversized monster name accepted")
	}
	monster.Name = "Eldimmu"
	level = 256
	if err := ValidateLiveSnapshot("observed", 25735, []Monster{monster}, now, now); err == nil {
		t.Fatal("invalid monster level accepted")
	}
	level = 72
	maxHP = 9007199254740992
	if err := ValidateLiveSnapshot("observed", 25735, []Monster{monster}, now, now); err == nil {
		t.Fatal("unsafe JSON HP accepted")
	}
}
