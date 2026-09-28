package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	"phmon/server/internal/resources"
)

func TestGuildStorageRequiresExplicitServerAndGuildScope(t *testing.T) {
	handler := &guildStorageHandler{}
	for _, target := range []string{
		"/api/guild-storage",
		"/api/guild-storage?server=Greatest",
		"/api/guild-storage?guild=Guild",
		"/api/guild-storage?server=%20&guild=Guild",
	} {
		recorder := httptest.NewRecorder()
		request := httptest.NewRequest(http.MethodGet, target, nil)
		handler.get(recorder, request)
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("GET %s status = %d, want %d", target, recorder.Code, http.StatusBadRequest)
		}
	}
}

type guildStorageTestStore struct {
	deletedServer string
	deletedGuild  string
	deleteCalls   int
}

func (*guildStorageTestStore) GuildStorage(context.Context, string, string) ([]resources.Observation, error) {
	return nil, nil
}

func (s *guildStorageTestStore) DeleteGuildStorage(_ context.Context, server, guild string) (int64, int64, error) {
	s.deleteCalls++
	s.deletedServer, s.deletedGuild = server, guild
	return 2, 11, nil
}

func TestGuildStorageRemovalRequiresExactTypedGuildConfirmation(t *testing.T) {
	store := &guildStorageTestStore{}
	handler := &guildStorageHandler{store: store}
	for _, body := range []string{
		`{"server":"Greatest","guild":"iBot","confirmation":"ibot"}`,
		`{"server":"Greatest","guild":"iBot","confirmation":"iBot","extra":true}`,
		`{"server":"Greatest","guild":"iBot","confirmation":"iBot"} {}`,
	} {
		request := httptest.NewRequest(http.MethodDelete, "/api/guild-storage", strings.NewReader(body))
		response := httptest.NewRecorder()
		handler.delete(response, request)
		if response.Code != http.StatusBadRequest {
			t.Fatalf("DELETE body %s status=%d, want 400", body, response.Code)
		}
	}
	if store.deleteCalls != 0 {
		t.Fatalf("store delete called %d times for invalid confirmation", store.deleteCalls)
	}

	request := httptest.NewRequest(http.MethodDelete, "/api/guild-storage", strings.NewReader(`{"server":"Greatest","guild":"iBot","confirmation":"iBot"}`))
	response := httptest.NewRecorder()
	handler.delete(response, request)
	if response.Code != http.StatusOK || store.deleteCalls != 1 || store.deletedServer != "Greatest" || store.deletedGuild != "iBot" {
		t.Fatalf("valid confirmation: status=%d calls=%d scope=%q/%q body=%s", response.Code, store.deleteCalls, store.deletedServer, store.deletedGuild, response.Body.String())
	}
	var result map[string]any
	if err := json.Unmarshal(response.Body.Bytes(), &result); err != nil {
		t.Fatal(err)
	}
	if result["deleted_observations"] != float64(2) || result["deleted_items"] != float64(11) {
		t.Fatalf("deletion counts missing: %#v", result)
	}
}

func TestGuildStorageRemovalRouteRequiresOperatorAuthentication(t *testing.T) {
	handler := New(Dependencies{Resources: resources.NewStore(nil), Auth: testOperatorManager(t)})
	request := httptest.NewRequest(http.MethodDelete, "/api/guild-storage", strings.NewReader(`{"server":"Greatest","guild":"iBot","confirmation":"iBot"}`))
	request.Header.Set("Origin", "http://127.0.0.1:3005")
	response := httptest.NewRecorder()
	handler.ServeHTTP(response, request)
	if response.Code != http.StatusUnauthorized {
		t.Fatalf("unauthenticated guild-storage removal status=%d, want 401", response.Code)
	}
}
