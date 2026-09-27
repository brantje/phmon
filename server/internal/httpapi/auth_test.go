package httpapi

import (
	"bytes"
	"net/http"
	"net/http/httptest"
	"strings"
	"testing"

	authdomain "phmon/server/internal/auth"
)

func testOperatorManager(t *testing.T) *authdomain.Manager {
	t.Helper()
	manager, err := authdomain.New(
		"0123456789abcdef0123456789abcdef",
		"phmon_operator",
		[]string{"http://127.0.0.1:3005"},
		true,
	)
	if err != nil {
		t.Fatal(err)
	}
	return manager
}

func TestOperatorLoginAndProtectedRoute(t *testing.T) {
	manager := testOperatorManager(t)
	mux := http.NewServeMux()
	authHandler := &operatorAuthHandler{manager: manager}
	mux.HandleFunc("POST /api/auth/login", authHandler.login)
	mux.HandleFunc("GET /protected", requireOperator(manager, false, func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(http.StatusNoContent)
	}))

	anonymous := httptest.NewRequest(http.MethodGet, "/protected", nil)
	anonymousResult := httptest.NewRecorder()
	mux.ServeHTTP(anonymousResult, anonymous)
	if anonymousResult.Code != http.StatusUnauthorized {
		t.Fatalf("anonymous status = %d", anonymousResult.Code)
	}

	login := httptest.NewRequest(http.MethodPost, "/api/auth/login", bytes.NewBufferString(`{"secret":"0123456789abcdef0123456789abcdef"}`))
	login.RemoteAddr = "127.0.0.1:12345"
	login.Header.Set("Origin", "http://127.0.0.1:3005")
	loginResult := httptest.NewRecorder()
	mux.ServeHTTP(loginResult, login)
	if loginResult.Code != http.StatusOK {
		t.Fatalf("login status = %d body=%s", loginResult.Code, loginResult.Body.String())
	}
	cookies := loginResult.Result().Cookies()
	if len(cookies) != 1 || cookies[0].Name != "phmon_operator" || !cookies[0].HttpOnly || cookies[0].SameSite != http.SameSiteStrictMode {
		t.Fatalf("unexpected session cookie: %#v", cookies)
	}
	if cookies[0].Secure {
		t.Fatal("loopback HTTP development cookie unexpectedly secure")
	}

	protected := httptest.NewRequest(http.MethodGet, "/protected", nil)
	protected.AddCookie(cookies[0])
	protectedResult := httptest.NewRecorder()
	mux.ServeHTTP(protectedResult, protected)
	if protectedResult.Code != http.StatusNoContent {
		t.Fatalf("protected status = %d", protectedResult.Code)
	}
}

func TestOperatorLoginRejectsCrossOrigin(t *testing.T) {
	manager := testOperatorManager(t)
	handler := &operatorAuthHandler{manager: manager}
	request := httptest.NewRequest(http.MethodPost, "/api/auth/login", strings.NewReader(`{"secret":"0123456789abcdef0123456789abcdef"}`))
	request.Header.Set("Origin", "https://evil.example")
	result := httptest.NewRecorder()
	handler.login(result, request)
	if result.Code != http.StatusForbidden {
		t.Fatalf("cross-origin status = %d", result.Code)
	}
}
