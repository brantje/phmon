package phproxy

import (
	"encoding/json"
	"strings"
	"testing"
	"time"

	"phmon/server/internal/players"
)

func TestLiveAdmissionRequiresIdentityAndPreservesMap(t *testing.T) {
	ctx, pool, registry, _, scope, _ := identityEnvironment(t)
	at := time.Now().UTC().Truncate(time.Microsecond)
	no, yes := false, true
	rows := []players.Player{
		{PlayerID: "1", Name: "Unclassified", Level: intPtr(110), X: 10, Y: 20},
		{PlayerID: "2", Name: "Affiliated", Job: "hunter", JobLevel: intPtr(7), X: 10, Y: 20},
		{PlayerID: "3", Name: "Normal", IsJobbing: &no, X: 10, Y: 20},
		{PlayerID: "4", Name: "Alias", IsJobbing: &yes, X: 10, Y: 20},
		{PlayerID: "5", Name: "Own", X: 10, Y: 20},
	}
	if _, err := pool.Exec(ctx, `INSERT INTO characters(server_name,server_key,character_name,identity_key)
	 VALUES($1,$2,'Own','own')`, scope, strings.ToLower(scope)); err != nil {
		t.Fatal(err)
	}
	// This is the same independent map-store/registry ordering as agent ingestion.
	live := players.NewLiveStore()
	live.Apply(players.LiveSnapshot{Server: scope, SessionID: "fixture", Status: "observed", ObservedAt: at, Players: rows})
	for i := range 3 {
		if err := registry.ApplyLive(ctx, scope, "", "", "", at.Add(time.Duration(i)*time.Second), rows); err != nil {
			t.Fatal(err)
		}
	}
	page, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || page.Total != 3 {
		t.Fatalf("admitted rows: %+v %v", page, err)
	}
	var history int
	if err := pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_observations WHERE server_key=$1
	 AND observed_name IN ('Unclassified','Affiliated')`, strings.ToLower(scope)).Scan(&history); err != nil || history != 0 {
		t.Fatalf("unclassified history=%d err=%v", history, err)
	}
	if snapshots := live.Snapshot(scope, at); len(snapshots) != 1 || len(snapshots[0].Players) != len(rows) {
		t.Fatal("admission changed live map sightings")
	}
	// A known name in a different server must not authorize this observation.
	if err := registry.ApplyLive(ctx, scope+"-other", "", "", "", at, rows[4:]); err != nil {
		t.Fatal(err)
	}
	other, err := registry.List(ctx, players.ListFilter{Server: scope + "-other"})
	if err != nil || other.Total != 0 {
		t.Fatalf("cross-server admission: %+v %v", other, err)
	}
	// Later native classification starts history at that observation, not earlier.
	rows[0].IsJobbing = &no
	if err := registry.ApplyLive(ctx, scope, "", "", "", at.Add(time.Minute), rows[:1]); err != nil {
		t.Fatal(err)
	}
	classified, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "Unclassified"})
	if err != nil || classified.Total != 1 || !classified.Players[0].FirstSeenAt.Equal(at.Add(time.Minute)) {
		t.Fatalf("late native admission: %+v %v", classified, err)
	}
	rows[0].IsJobbing = nil
	if err := registry.ApplyLive(ctx, scope, "", "", "", at.Add(2*time.Minute), rows[:1]); err != nil {
		t.Fatal(err)
	}
	updated, err := registry.Get(ctx, classified.Players[0].ID)
	if err != nil || !updated.LastSeenAt.Equal(at.Add(2*time.Minute)) {
		t.Fatalf("known identity lost on omitted fields: %+v %v", updated, err)
	}
	// Even a row that would be skipped must validate before any writes occur.
	invalid := []players.Player{{PlayerID: "6", Name: "Atomic", IsJobbing: &no, X: 1, Y: 2},
		{PlayerID: "7", Name: "Invalid", Level: intPtr(0), X: 1, Y: 2}}
	if err := registry.ApplyLive(ctx, scope, "", "", "", at, invalid); err == nil {
		t.Fatal("invalid unclassified row was silently skipped")
	}
	atomic, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "Atomic"})
	if err != nil || atomic.Total != 0 {
		t.Fatalf("partial validation writes: %+v %v", atomic, err)
	}
	// Legacy unidentified records remain intact without accumulating new history.
	if err := registry.Apply(ctx, []players.Observation{{Server: scope, ObservedName: "LegacyUnknown", Level: intPtr(90),
		Source: players.SourceMapPlayers, ObservedAt: at}}); err != nil {
		t.Fatal(err)
	}
	if err := registry.ApplyLive(ctx, scope, "", "", "", at.Add(time.Hour),
		[]players.Player{{PlayerID: "8", Name: "LegacyUnknown", Level: intPtr(91), X: 1, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	legacyUnknown, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "LegacyUnknown"})
	if err != nil || legacyUnknown.Total != 1 || !legacyUnknown.Players[0].LastSeenAt.Equal(at) || *legacyUnknown.Players[0].Level != 90 {
		t.Fatalf("legacy unknown tracking rewritten: %+v %v", legacyUnknown, err)
	}
	levels, err := registry.Levels(ctx, legacyUnknown.Players[0].ID, 25, 0)
	if err != nil || levels.Total != 1 {
		t.Fatalf("unclassified level history grew: %+v %v", levels, err)
	}
}

func TestLiveAdmissionPromotesStagedIdentityAndMappedAlias(t *testing.T) {
	ctx, pool, registry, store, scope, agent := identityEnvironment(t)
	t.Cleanup(func() {
		_, _ = pool.Exec(ctx, `DELETE FROM player_enrichment_pending WHERE server_key=$1`, strings.ToLower(scope))
	})
	at := time.Now().UTC().Truncate(time.Microsecond)
	rows := []players.Player{{PlayerID: "1", Name: "Magik", Level: intPtr(110), X: 10, Y: 20}, {PlayerID: "2", Name: "xDont", X: 10, Y: 20}}
	if err := registry.ApplyLive(ctx, scope, "", "", "", at, rows); err != nil {
		t.Fatal(err)
	}
	initial, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || initial.Total != 0 {
		t.Fatalf("unclassified admission: %+v %v", initial, err)
	}
	out := submitIdentity(t, ctx, store, scope, agent, "Magik", []string{"xDont"}, at, 7)
	if out.Results[0].Status != "pending" {
		t.Fatalf("proxy-only identity became tracking: %+v", out)
	}
	// Normal tracking, rather than a proxy upload, triggers admission/promotion.
	for i := range 2 {
		if err := registry.ApplyLive(ctx, scope, "", "", "", at.Add(time.Duration(i+1)*time.Minute), rows); err != nil {
			t.Fatal(err)
		}
	}
	page, err := registry.List(ctx, players.ListFilter{Server: scope})
	if err != nil || page.Total != 1 || len(page.Players[0].DisplayIdentity.RelatedRecordIDs) != 2 {
		t.Fatalf("mapped admission: %+v %v", page, err)
	}
	if page.Players[0].DisplayIdentity.JobAlias == nil || *page.Players[0].DisplayIdentity.JobAlias != "xDont" {
		t.Fatal("alias identity lost")
	}
	gear, err := store.LoadEnrichmentWithPromotion(ctx, page.Players[0].ID, scope, strings.ToLower(scope), "magik")
	if err != nil || gear == nil || len(gear.Gear) != 7 {
		t.Fatalf("gear missing: %+v %v", gear, err)
	}
	var nativeName *string
	if err := pool.QueryRow(ctx, `SELECT player_name FROM players WHERE id=$1`, page.Players[0].ID).Scan(&nativeName); err != nil || nativeName != nil {
		t.Fatalf("proxy classification became native evidence: %v %v", nativeName, err)
	}
	// Staged evidence expires and cannot authorize a later unidentified sighting.
	submitIdentity(t, ctx, store, scope, agent, "Expired", nil, at, 0)
	if _, err := pool.Exec(ctx, `UPDATE player_enrichment_pending SET expires_at=now()-interval '1 second'
	 WHERE server_key=$1 AND name_key='expired'`, strings.ToLower(scope)); err != nil {
		t.Fatal(err)
	}
	if err := registry.ApplyLive(ctx, scope, "", "", "", at, []players.Player{{PlayerID: "3", Name: "Expired", X: 1, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	expired, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "Expired"})
	if err != nil || expired.Total != 0 {
		t.Fatalf("expired pending admitted: %+v %v", expired, err)
	}
	// A field association alone does not establish a global alias mapping.
	var body wireRequestV2
	if err := json.Unmarshal(identityBody(t, scope, agent, "AliasGearOnly", nil, at, 1), &body); err != nil {
		t.Fatal(err)
	}
	body.Players[0].IdentityLinks = nil
	delete(body.Players[0].FieldEvidence, "identity_links")
	ev := FieldEvidence{ObservedAt: at, Profile: ProfileName, SourceOpcode: OpcodeSpawnEquipment,
		Reason: EvidenceReasonVerified, SourceIdentityKind: IdentityKindJobAlias, SourceName: "UnlinkedAlias",
		Association: &FieldAssociation{Alias: "UnlinkedAlias", Status: "inferred", Reason: "job_item_transition", ObservedAt: at}}
	body.Players[0].FieldEvidence["gear"], _ = json.Marshal(ev)
	raw, _ := json.Marshal(body)
	if _, err := store.Submit(ctx, agent, raw); err != nil {
		t.Fatal(err)
	}
	if err := registry.ApplyLive(ctx, scope, "", "", "", at, []players.Player{{PlayerID: "4", Name: "UnlinkedAlias", X: 1, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	aliasOnly, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "UnlinkedAlias"})
	if err != nil || aliasOnly.Total != 0 {
		t.Fatalf("field association admitted an unlinked alias: %+v %v", aliasOnly, err)
	}
	// V1 verified evidence uses its legacy shape and remains eligible.
	v1 := strings.ReplaceAll(validFixtureBody(), "FixturePlayer", "LegacyName")
	v1 = strings.ReplaceAll(v1, "Greatest", scope)
	v1 = strings.ReplaceAll(v1, "22222222-2222-4222-8222-222222222222", agent)
	if _, err := store.Submit(ctx, agent, []byte(v1)); err != nil {
		t.Fatal(err)
	}
	if err := registry.ApplyLive(ctx, scope, "", "", "", at, []players.Player{{PlayerID: "5", Name: "LegacyName", X: 1, Y: 2}}); err != nil {
		t.Fatal(err)
	}
	legacy, err := registry.List(ctx, players.ListFilter{Server: scope, Name: "LegacyName"})
	if err != nil || legacy.Total != 1 || legacy.Players[0].DisplayIdentity.PlayerName == nil {
		t.Fatalf("legacy verified admission: %+v %v", legacy, err)
	}
}
