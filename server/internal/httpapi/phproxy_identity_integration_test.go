package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"os"
	"strings"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/database"
	"phmon/server/internal/phproxy"
	"phmon/server/internal/players"
)

func TestPhProxyIdentityHTTPFlow(t *testing.T) {
	url := os.Getenv("TEST_DATABASE_URL")
	if url == "" {
		t.Skip("set TEST_DATABASE_URL for PostgreSQL HTTP flow")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	credential, err := agents.NewCredential()
	if err != nil {
		t.Fatal(err)
	}
	agentStore := agents.NewStore(pool)
	if err = agentStore.CreateCredential(ctx, credential); err != nil {
		t.Fatal(err)
	}
	scope := "HTTP-Identity-" + credential.AgentID
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, strings.ToLower(scope))
		_, _ = pool.Exec(context.Background(), `DELETE FROM agents WHERE agent_id=$1`, credential.AgentID)
	}()
	registry := players.NewRegistry(pool)
	store := phproxy.NewStore(pool, nil)
	registry.SetEnrichmentPromoter(store)
	at := time.Now().UTC().Add(-time.Hour)
	if err = registry.Apply(ctx, []players.Observation{{Server: scope, ObservedName: "FixturePlayer", Source: players.SourceMapPlayers, ObservedAt: at}, {Server: scope, ObservedName: "FixtureTrader", Source: players.SourceMapPlayers, ObservedAt: at}}); err != nil {
		t.Fatal(err)
	}
	raw, err := os.ReadFile("../phproxy/testdata/upstream-v2-examples.json")
	if err != nil {
		t.Fatal(err)
	}
	var examples struct {
		Requests []map[string]any `json:"requests"`
	}
	if err = json.Unmarshal(raw, &examples); err != nil {
		t.Fatal(err)
	}
	submit := &phProxyHandler{agents: agentStore, store: store}
	send := func(body map[string]any, path, token string) *httptest.ResponseRecorder {
		data, _ := json.Marshal(body)
		r := httptest.NewRequest(http.MethodPost, path, strings.NewReader(string(data)))
		r.Header.Set("Authorization", "Bearer "+token)
		w := httptest.NewRecorder()
		submit.submitEnrichment(w, r)
		return w
	}
	first := examples.Requests[0]
	first["agent_id"], first["server"] = credential.AgentID, scope
	if w := send(first, "/api/integrations/ph-proxy/v2/enrichments", credential.Token); w.Code != 200 {
		t.Fatalf("direct request status %d: %s", w.Code, w.Body)
	}
	handler := &playerHandler{store: registry, enrichment: store}
	// FixturePlayer is a direct normal-name observation with job=0 and no
	// native affiliation. Unknown reference confidence is accepted and exposed.
	zeroRequest := httptest.NewRequest("GET", "/api/players?server="+scope+"&q=FixturePlayer&job=none", nil)
	zeroResponse := httptest.NewRecorder()
	handler.list(zeroResponse, zeroRequest)
	var zeroPage players.PlayerPage
	if err = json.Unmarshal(zeroResponse.Body.Bytes(), &zeroPage); err != nil || zeroResponse.Code != 200 || zeroPage.Total != 1 || zeroPage.Players[0].DisplayFallback == nil || zeroPage.Players[0].DisplayFallback.Job != "none" {
		t.Fatalf("normal-name zero fallback: %d %s %v", zeroResponse.Code, zeroResponse.Body, err)
	}
	zeroID := zeroPage.Players[0].ID
	zeroDetailRequest := httptest.NewRequest("GET", "/api/players/"+zeroID, nil)
	zeroDetailRequest.SetPathValue("id", zeroID)
	zeroDetailResponse := httptest.NewRecorder()
	handler.get(zeroDetailResponse, zeroDetailRequest)
	var zeroDetail playerDetailResponse
	if err = json.Unmarshal(zeroDetailResponse.Body.Bytes(), &zeroDetail); err != nil || zeroDetailResponse.Code != 200 || zeroDetail.DisplayFallback == nil || zeroDetail.DisplayFallback.Job != "none" || zeroDetail.Enrichment == nil || zeroDetail.Enrichment.FieldEvidence["job"].Reason != phproxy.EvidenceReasonUnverifiedReference {
		t.Fatalf("detail zero/confidence: %d %s %v", zeroDetailResponse.Code, zeroDetailResponse.Body, err)
	}
	third := examples.Requests[2]
	third["agent_id"], third["server"] = credential.AgentID, scope
	if w := send(third, "/api/integrations/ph-proxy/v2/enrichments", credential.Token); w.Code != 200 {
		t.Fatalf("alias request status %d: %s", w.Code, w.Body)
	}
	listRequest := httptest.NewRequest("GET", "/api/players?server="+scope+"&q=FixtureTrader", nil)
	list := httptest.NewRecorder()
	handler.list(list, listRequest)
	var page players.PlayerPage
	if err = json.Unmarshal(list.Body.Bytes(), &page); err != nil || list.Code != 200 || page.Total != 1 {
		t.Fatalf("resolved list: %d %s %v", list.Code, list.Body, err)
	}
	for _, id := range page.Players[0].DisplayIdentity.RelatedRecordIDs {
		r := httptest.NewRequest("GET", "/api/players/"+id, nil)
		r.SetPathValue("id", id)
		w := httptest.NewRecorder()
		handler.get(w, r)
		var detail playerDetailResponse
		if err = json.Unmarshal(w.Body.Bytes(), &detail); err != nil || w.Code != 200 || detail.Enrichment == nil || len(detail.Enrichment.Gear) != 2 {
			t.Fatalf("alias details: %d %s %v", w.Code, w.Body, err)
		}
		if detail.Enrichment.FieldEvidence["gear"].Association == nil {
			t.Fatal("alias gear attribution missing")
		}
		if detail.DisplayFallback != nil && (detail.DisplayFallback.Job != "" || detail.DisplayFallback.JobLevel != nil) {
			t.Fatal("HTTP projection used alias-attributed job as a normal fallback")
		}
	}
	if w := send(third, "/api/integrations/ph-proxy/v1/enrichments", credential.Token); w.Code != 400 {
		t.Fatal("v1 accepted v2 schema")
	}
	if w := send(third, "/api/integrations/ph-proxy/v2/enrichments", "invalid-fixture-token"); w.Code != 401 {
		t.Fatal("invalid credential accepted")
	}
	// A malformed association rejects the whole batch with safe, actionable details.
	player := third["players"].([]any)[0].(map[string]any)
	gearEvidence := player["field_evidence"].(map[string]any)["gear"].(map[string]any)
	gearEvidence["association"] = nil
	bad := send(third, "/api/integrations/ph-proxy/v2/enrichments", credential.Token)
	if bad.Code != 400 || !strings.Contains(bad.Body.String(), `"field":"gear"`) || !strings.Contains(bad.Body.String(), `"player_index":0`) || strings.Contains(bad.Body.String(), "FixtureTrader") || strings.Contains(bad.Body.String(), credential.Token) {
		t.Fatalf("unsafe/non-specific rejection: %d %s", bad.Code, bad.Body)
	}
}
