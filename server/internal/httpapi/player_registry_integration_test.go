package httpapi

import (
	"bytes"
	"context"
	"encoding/json"
	"fmt"
	"net/http"
	"net/http/httptest"
	"net/url"
	"os"
	"testing"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/database"
	"phmon/server/internal/players"
)

func TestPlayerRegistryAuthenticatedAPIAndReview(t *testing.T) {
	databaseURL := os.Getenv("TEST_DATABASE_URL")
	if databaseURL == "" {
		t.Skip("set TEST_DATABASE_URL")
	}
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	pool, err := pgxpool.New(ctx, databaseURL)
	if err != nil {
		t.Fatal(err)
	}
	defer pool.Close()
	if err = database.Migrate(ctx, pool); err != nil {
		t.Fatal(err)
	}
	store := players.NewStore(pool)
	server := "Fixture HTTP Players " + time.Now().UTC().Format("150405.000000")
	defer func() {
		_, _ = pool.Exec(context.Background(), `DELETE FROM player_identity_links WHERE server_key=$1`, players.ServerKey(server))
		_, _ = pool.Exec(context.Background(), `DELETE FROM players WHERE server_key=$1`, players.ServerKey(server))
	}()
	now := time.Now().Add(-time.Minute).UTC()
	level := 110
	guild := "Guild"
	job := "hunter"
	observations := []players.Observation{{Server: server, Name: "FixtureNormal", NameType: "unknown", Level: &level, Guild: &guild, Source: "map.players", SessionID: "fixture-session", RuntimeID: "1", Epoch: "1", ObservedAt: now}, {Server: server, Name: "FixtureHunter", NameType: "job", Job: &job, Source: "synthetic.fixture", ObservedAt: now.Add(time.Second)}}
	if err = store.ApplyObservations(ctx, observations); err != nil {
		t.Fatal(err)
	}
	handler := New(Dependencies{Auth: testOperatorManager(t), PlayerRegistry: store})
	call := func(method, path string, body any, cookie *http.Cookie, origin string) *httptest.ResponseRecorder {
		var buffer bytes.Buffer
		if body != nil {
			_ = json.NewEncoder(&buffer).Encode(body)
		}
		request := httptest.NewRequest(method, path, &buffer)
		request.Header.Set("Origin", origin)
		if cookie != nil {
			request.AddCookie(cookie)
		}
		response := httptest.NewRecorder()
		handler.ServeHTTP(response, request)
		return response
	}
	if r := call("GET", "/api/players", nil, nil, ""); r.Code != 401 {
		t.Fatal("registry not authenticated")
	}
	login := call("POST", "/api/auth/login", map[string]string{"secret": "0123456789abcdef0123456789abcdef"}, nil, "http://127.0.0.1:3005")
	cookie := login.Result().Cookies()[0]
	r := call("GET", "/api/players?server="+url.QueryEscape(server)+"&q=Fixture&min_level=100&guild=Guild", nil, cookie, "")
	if r.Code != 200 {
		t.Fatalf("list %d %s", r.Code, r.Body.String())
	}
	var page players.Page
	_ = json.Unmarshal(r.Body.Bytes(), &page)
	if page.Total != 1 || page.Players[0].Name != nil {
		t.Fatal("filter or unknown name projection")
	}
	normal := page.Players[0]
	for _, path := range []string{"/api/players?job=invalid", "/api/players?min_level=200&max_level=100", "/api/players?sort=name%3BDROP", "/api/players?q=a&q=b", "/api/players?limit=1000", "/api/players?cursor=garbage", "/api/players/not-an-id"} {
		if r = call("GET", path, nil, cookie, ""); r.Code != 400 {
			t.Fatalf("invalid request %s -> %d", path, r.Code)
		}
	}
	for _, suffix := range []string{"", "/equipment", "/equipment/history", "/aliases", "/observations"} {
		if r = call("GET", "/api/players/"+normal.ID+suffix, nil, cookie, ""); r.Code != 200 {
			t.Fatalf("profile %s %d %s", suffix, r.Code, r.Body.String())
		}
	}
	classify := players.Classification{Name: "FixtureNormal", Type: "normal", Reason: "fixture verification", Confirmed: true, Revision: normal.Revision}
	if r = call("POST", "/api/players/"+normal.ID+"/aliases", classify, cookie, "https://evil.example"); r.Code != 403 {
		t.Fatal("classification lacked origin check")
	}
	if r = call("POST", "/api/players/"+normal.ID+"/aliases", classify, cookie, "http://127.0.0.1:3005"); r.Code != 204 {
		t.Fatalf("classification %d %s", r.Code, r.Body.String())
	}
	normal, _ = store.Get(ctx, normal.ID)
	p, _ := store.List(ctx, players.Filter{Server: server, Name: "Hunter"})
	target := p.Players[0]
	body := players.LinkRequest{CanonicalID: normal.ID, LinkedID: target.ID, Action: "confirm", Reason: "fixture verification", Confirmed: true, CanonicalRevision: normal.Revision, LinkedRevision: target.Revision}
	r = call("POST", "/api/players/links", body, cookie, "http://127.0.0.1:3005")
	if r.Code != 201 {
		t.Fatalf("association %d %s", r.Code, r.Body.String())
	}
	var linkResponse map[string]string
	_ = json.Unmarshal(r.Body.Bytes(), &linkResponse)
	if r = call("POST", "/api/players/links", body, cookie, "http://127.0.0.1:3005"); r.Code != 409 {
		t.Fatal("stale decision accepted")
	}
	r = call("GET", "/api/players/"+target.ID, nil, cookie, "")
	var canonical players.Record
	_ = json.Unmarshal(r.Body.Bytes(), &canonical)
	if canonical.ID != normal.ID {
		t.Fatal("linked route lacks canonical projection")
	}
	mapHub := &LiveHub{playerRegistry: store}
	markers := mapHub.registryPlayerSnapshot(ctx, "", mapPlayerSnapshot{Status: "observed", Players: []mapPlayer{{server: server, Name: "FixtureHunter", ID: "runtime-42", PlayerID: "42", X: 10, Y: 20}}})
	if markers.Players[0].RegistryID != normal.ID || markers.Players[0].ID != "runtime-42" || markers.Players[0].X != 10 {
		t.Fatal("canonical map link changed runtime marker identity or position")
	}
	r = call("GET", "/api/players/"+target.ID+"?source=1", nil, cookie, "")
	var source players.Record
	_ = json.Unmarshal(r.Body.Bytes(), &source)
	if source.ID != target.ID {
		t.Fatal("original record unavailable")
	}
	links, _ := store.Links(ctx, server, "", "confirmed", 25, "")
	revoke := map[string]any{"confirmed": true, "reason": "fixture correction", "revision": links.Items[0].Revision}
	if r = call("DELETE", "/api/players/links/"+linkResponse["id"], revoke, cookie, "http://127.0.0.1:3005"); r.Code != 204 {
		t.Fatalf("revoke %d %s", r.Code, r.Body.String())
	}
	if r = call("GET", "/api/players/00000000-0000-4000-8000-000000000001", nil, cookie, ""); r.Code != 404 {
		t.Fatal(fmt.Sprintf("missing profile %d", r.Code))
	}
}
