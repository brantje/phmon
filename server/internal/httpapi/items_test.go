package httpapi

import (
	"net/http"
	"net/http/httptest"
	"testing"
)

func TestItemSearchLimitBounds(t *testing.T) {
	handler := &itemSearchHandler{}
	for _, limit := range []string{"0", "5001", "x"} {
		recorder := httptest.NewRecorder()
		handler.list(recorder, httptest.NewRequest(http.MethodGet, "/api/items?limit="+limit, nil))
		if recorder.Code != http.StatusBadRequest {
			t.Fatalf("limit=%s status=%d, want 400", limit, recorder.Code)
		}
	}
}
