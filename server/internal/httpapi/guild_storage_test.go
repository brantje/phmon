package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
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
