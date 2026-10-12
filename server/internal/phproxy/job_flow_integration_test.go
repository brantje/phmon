package phproxy

import (
	"encoding/json"
	"os"
	"strings"
	"testing"
	"time"

	"phmon/server/internal/agents"
	"phmon/server/internal/players"
)

func TestNormalJobEnrichmentProjection(t *testing.T) {
	ctx, pool, registry, store, scope, agent := identityEnvironment(t)
	at := time.Now().UTC().Add(-time.Hour).Truncate(time.Microsecond)
	noSuit := false
	none, hunter := "none", "hunter"
	cases := []struct {
		name        string
		nativeJob   *string
		nativeLevel *int
		code        int
		alias       bool
		want        string
	}{
		{name: "NormalTrader", code: 1, want: "trader"},
		{name: "NormalThief", code: 2, want: "thief"},
		{name: "NormalHunter", code: 3, want: "hunter"},
		{name: "ProxyZero", code: 0, want: "none"},
		{name: "UnknownCode", code: 255, want: "255"},
		{name: "NativeZero", nativeJob: &none, nativeLevel: intPtr(0), code: 1, want: "none"},
		{name: "NativeHunter", nativeJob: &hunter, nativeLevel: intPtr(4), code: 2, want: "hunter"},
		{name: "AliasAttributed", code: 1, alias: true},
	}
	for _, c := range cases {
		if err := registry.Apply(ctx, []players.Observation{{Server: scope, ObservedName: c.name, Job: c.nativeJob, JobLevel: c.nativeLevel, IsJobbing: &noSuit, Level: intPtr(110), Source: players.SourceMapPlayers, ObservedAt: at}}); err != nil {
			t.Fatal(err)
		}
	}
	var before string
	// Native rows and history must remain byte-for-byte unchanged by enrichment.
	tracking := `SELECT jsonb_build_object('players',(SELECT jsonb_agg(to_jsonb(p) ORDER BY p.id) FROM players p WHERE server_key=$1), 'observations',(SELECT jsonb_agg(to_jsonb(o) ORDER BY o.id) FROM player_observations o WHERE server_key=$1), 'levels',(SELECT jsonb_agg(to_jsonb(l) ORDER BY l.id) FROM player_level_snapshots l JOIN players p ON p.id=l.player_id WHERE p.server_key=$1))::text`
	if err := pool.QueryRow(ctx, tracking, strings.ToLower(scope)).Scan(&before); err != nil {
		t.Fatal(err)
	}
	for _, c := range cases {
		body := jobBody(t, scope, agent, c.name, c.code, 7, c.alias, at.Add(time.Minute))
		result, err := store.Submit(ctx, agent, body)
		if err != nil || result.Results[0].Status != "applied" {
			t.Fatalf("%s: %v %+v", c.name, err, result)
		}
		var id string
		if err = pool.QueryRow(ctx, `SELECT id::text FROM players WHERE server_key=$1 AND observed_name=$2`, strings.ToLower(scope), c.name).Scan(&id); err != nil {
			t.Fatal(err)
		}
		detail, err := registry.Get(ctx, id)
		if err != nil {
			t.Fatal(err)
		}
		if detail.IsJobbing == nil || *detail.IsJobbing {
			t.Fatal("affiliation changed jobbing")
		}
		projected := ""
		if detail.Job != nil {
			projected = *detail.Job
		} else if detail.DisplayFallback != nil {
			projected = detail.DisplayFallback.Job
		}
		if projected != c.want {
			t.Fatalf("%s displayed job=%q want=%q", c.name, projected, c.want)
		}
		if c.alias && detail.DisplayFallback != nil && (detail.DisplayFallback.Job != "" || detail.DisplayFallback.JobLevel != nil) {
			t.Fatal("alias promoted to unqualified fallback")
		}
		if c.nativeJob != nil && detail.DisplayFallback != nil && detail.DisplayFallback.Job != "" {
			t.Fatal("native job overwritten")
		}
		if c.nativeLevel != nil && (detail.JobLevel == nil || *detail.JobLevel != *c.nativeLevel) {
			t.Fatal("known native job level overwritten")
		}
		if c.nativeJob == nil && !c.alias {
			if detail.DisplayFallback == nil || detail.DisplayFallback.JobLevel == nil || *detail.DisplayFallback.JobLevel != 7 {
				t.Fatal("missing normal job level fallback")
			}
			var evidence FieldEvidence
			if err = json.Unmarshal(detail.DisplayFallback.JobEvidence, &evidence); err != nil || evidence.Reason != EvidenceReasonUnverifiedReference || evidence.SourceName != c.name {
				t.Fatalf("fallback confidence lost: %+v %v", evidence, err)
			}
		}
		enriched, err := store.LoadEnrichmentWithPromotion(ctx, id, scope, strings.ToLower(scope), strings.ToLower(c.name))
		if err != nil || enriched.Job == nil || *enriched.Job != c.code || enriched.FieldEvidence["job"].Reason != EvidenceReasonUnverifiedReference {
			t.Fatalf("lost original value/evidence: %+v %v", enriched, err)
		}
		if enriched.JobItemIDs == nil && enriched.JobItemIDsObservedAt == nil {
			t.Fatal("missing known empty job-item observation")
		}
		if c.alias && enriched.FieldEvidence["job"].Association == nil {
			t.Fatal("alias attribution lost")
		}
	}
	var after string
	if err := pool.QueryRow(ctx, tracking, strings.ToLower(scope)).Scan(&after); err != nil || before != after {
		t.Fatal("job enrichment changed tracking/history", err)
	}
	// Counts, ordering and filters run on the displayed values before pagination.
	for _, f := range []struct {
		job   string
		total int
	}{{"trader", 1}, {"hunter", 2}, {"none", 2}, {"255", 1}, {"unknown", 1}} {
		page, err := registry.List(ctx, players.ListFilter{Server: scope, Job: f.job, Sort: "job", Limit: 1})
		if err != nil || page.Total != f.total || len(page.Players) != 1 {
			t.Fatalf("filter %s: %+v %v", f.job, page, err)
		}
	}
	page, err := registry.List(ctx, players.ListFilter{Server: scope, MinJobLevel: intPtr(7), MaxJobLevel: intPtr(7), Sort: "job_level", Limit: 1, Offset: 4})
	if err != nil || page.Total != 5 || len(page.Players) != 1 {
		t.Fatalf("job level count/page: %+v %v", page, err)
	}
	page, err = registry.List(ctx, players.ListFilter{Server: scope, Sort: "job", Descending: false, Limit: 1})
	if err != nil || page.Total != 8 || page.Players[0].ObservedName != "UnknownCode" {
		t.Fatalf("job sort before pagination: %+v %v", page, err)
	}
	page, err = registry.List(ctx, players.ListFilter{Server: scope, Sort: "job_level", Descending: false, Limit: 1})
	if err != nil || page.Players[0].ObservedName != "NativeZero" {
		t.Fatalf("known zero sort: %+v %v", page, err)
	}
}

func jobBody(t *testing.T, scope, agent, name string, code, level int, alias bool, at time.Time) []byte {
	t.Helper()
	requestID, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	evidence := map[string]json.RawMessage{}
	for _, field := range []string{"name", "job", "job_level", "job_item_ids"} {
		ev := FieldEvidence{ObservedAt: at, Profile: ProfileName, SourceOpcode: OpcodeSpawnEquipment, Reason: EvidenceReasonUnverifiedReference, SourceIdentityKind: IdentityKindCharacterName, SourceName: name}
		if field == "name" {
			ev.Reason = EvidenceReasonVerified
		} else if alias {
			ev.SourceIdentityKind = IdentityKindJobAlias
			ev.SourceName = "InferredAlias"
			ev.Association = &FieldAssociation{Alias: "InferredAlias", Status: IdentityLinkStatusInferred, Reason: IdentityLinkReasonJobTransition, ObservedAt: at}
		}
		evidence[field], _ = json.Marshal(ev)
	}
	empty := []int{}
	body, err := json.Marshal(wireRequestV2{SchemaVersion: 2, RequestID: requestID.AgentID, AgentID: agent, Server: scope, Players: []wirePlayerV2{{Name: name, IdentityKind: IdentityKindCharacterName, Job: &code, JobLevel: &level, JobItemIDs: &empty, FieldEvidence: evidence}}})
	if err != nil {
		t.Fatal(err)
	}
	return body
}

func TestJobFallbackBackfillAndPendingPromotion(t *testing.T) {
	ctx, pool, registry, store, scope, agent := identityEnvironment(t)
	at := time.Now().UTC().Add(-time.Minute)
	// Stage before native tracking, then simulate the former empty fallback column.
	for _, name := range []string{"StagedJob", "StagedAlias"} {
		if _, err := store.Submit(ctx, agent, jobBody(t, scope, agent, name, 3, 0, name == "StagedAlias", at)); err != nil {
			t.Fatal(err)
		}
	}
	if _, err := pool.Exec(ctx, `UPDATE player_enrichment_pending SET proxy_job=NULL WHERE server_key=$1`, strings.ToLower(scope)); err != nil {
		t.Fatal(err)
	}
	migration, err := os.ReadFile("../database/migrations/000035_ph_proxy_job_fallback.sql")
	if err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	var direct, alias *string
	if err = pool.QueryRow(ctx, `SELECT proxy_job FROM player_enrichment_pending WHERE server_key=$1 AND observed_name='StagedJob'`, strings.ToLower(scope)).Scan(&direct); err != nil {
		t.Fatal(err)
	}
	if err = pool.QueryRow(ctx, `SELECT proxy_job FROM player_enrichment_pending WHERE server_key=$1 AND observed_name='StagedAlias'`, strings.ToLower(scope)).Scan(&alias); err != nil {
		t.Fatal(err)
	}
	if direct == nil || *direct != "hunter" || alias != nil {
		t.Fatal("backfill attribution mismatch", direct, alias)
	}
	noSuit := false
	if err = registry.Apply(ctx, []players.Observation{{Server: scope, ObservedName: "StagedJob", IsJobbing: &noSuit, Source: players.SourceMapPlayers, ObservedAt: at}}); err != nil {
		t.Fatal(err)
	}
	// Recover already attached data too; no new sender upload is needed.
	if _, err = pool.Exec(ctx, `UPDATE player_enrichments SET proxy_job=NULL WHERE player_id IN (SELECT id FROM players WHERE server_key=$1)`, strings.ToLower(scope)); err != nil {
		t.Fatal(err)
	}
	if _, err = pool.Exec(ctx, string(migration)); err != nil {
		t.Fatal(err)
	}
	page, err := registry.List(ctx, players.ListFilter{Server: scope, Job: "hunter", Limit: 1})
	if err != nil || page.Total != 1 || page.Players[0].DisplayFallback.Job != "hunter" || *page.Players[0].DisplayFallback.JobLevel != 0 {
		t.Fatalf("promotion/backfill=%+v %v", page, err)
	}
}
