package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	authdomain "phmon/server/internal/auth"
)

const operatorLoginMaxBytes = 4 * 1024

type operatorSessionContextKey struct{}

type operatorSessionContext struct {
	watch authdomain.Watch
}

type operatorAuthHandler struct {
	manager *authdomain.Manager
}

type operatorSessionView struct {
	Authenticated bool       `json:"authenticated"`
	Actor         string     `json:"actor,omitempty"`
	ExpiresAt     *time.Time `json:"expires_at,omitempty"`
}

func (h *operatorAuthHandler) login(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if !h.manager.OriginAllowed(origin) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "cross_origin"})
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, operatorLoginMaxBytes)
	var body struct {
		Secret string `json:"secret"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil || len(body.Secret) > 1024 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_request"})
		return
	}

	token, watch, err := h.manager.CreateSession(body.Secret, r.RemoteAddr, time.Now().UTC())
	switch {
	case errors.Is(err, authdomain.ErrInvalidSecret):
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "invalid_credentials"})
		return
	case errors.Is(err, authdomain.ErrRateLimited):
		respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate_limited"})
		return
	case err != nil:
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service_unavailable"})
		return
	}

	http.SetCookie(w, &http.Cookie{
		Name:     h.manager.CookieName(),
		Value:    token,
		Path:     "/",
		HttpOnly: true,
		Secure:   h.manager.CookieSecureForOrigin(origin),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   int(h.manager.TTL() / time.Second),
		Expires:  watch.ExpiresAt,
	})
	respondJSON(w, http.StatusOK, operatorSessionView{Authenticated: true, Actor: "operator", ExpiresAt: &watch.ExpiresAt})
}

func (h *operatorAuthHandler) session(w http.ResponseWriter, r *http.Request) {
	_, watch, ok := operatorSessionFromRequest(h.manager, r)
	if !ok {
		respondJSON(w, http.StatusUnauthorized, operatorSessionView{Authenticated: false})
		return
	}
	respondJSON(w, http.StatusOK, operatorSessionView{Authenticated: true, Actor: "operator", ExpiresAt: &watch.ExpiresAt})
}

func (h *operatorAuthHandler) logout(w http.ResponseWriter, r *http.Request) {
	origin := strings.TrimSpace(r.Header.Get("Origin"))
	if !h.manager.OriginAllowed(origin) {
		respondJSON(w, http.StatusForbidden, map[string]string{"error": "cross_origin"})
		return
	}
	if cookie, err := r.Cookie(h.manager.CookieName()); err == nil {
		h.manager.Revoke(cookie.Value)
	}
	http.SetCookie(w, &http.Cookie{
		Name:     h.manager.CookieName(),
		Value:    "",
		Path:     "/",
		HttpOnly: true,
		Secure:   h.manager.CookieSecureForOrigin(origin),
		SameSite: http.SameSiteStrictMode,
		MaxAge:   -1,
		Expires:  time.Unix(1, 0),
	})
	respondJSON(w, http.StatusOK, operatorSessionView{Authenticated: false})
}

func requireOperator(manager *authdomain.Manager, requireOrigin bool, next http.HandlerFunc) http.HandlerFunc {
	return func(w http.ResponseWriter, r *http.Request) {
		_, watch, ok := operatorSessionFromRequest(manager, r)
		if !ok {
			respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
			return
		}
		if requireOrigin && !manager.OriginAllowed(strings.TrimSpace(r.Header.Get("Origin"))) {
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "cross_origin"})
			return
		}
		ctx := context.WithValue(r.Context(), operatorSessionContextKey{}, operatorSessionContext{watch: watch})
		next(w, r.WithContext(ctx))
	}
}

func operatorSessionFromRequest(manager *authdomain.Manager, r *http.Request) (string, authdomain.Watch, bool) {
	cookie, err := r.Cookie(manager.CookieName())
	if err != nil || strings.TrimSpace(cookie.Value) == "" {
		return "", authdomain.Watch{}, false
	}
	watch, ok := manager.Validate(cookie.Value, time.Now().UTC())
	return cookie.Value, watch, ok
}

func operatorSessionFromContext(ctx context.Context) (operatorSessionContext, bool) {
	value, ok := ctx.Value(operatorSessionContextKey{}).(operatorSessionContext)
	return value, ok
}
