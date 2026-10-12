package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/phproxy"
)

type phProxyHandler struct {
	agents AgentStore
	store  *phproxy.Store
}

func (h *phProxyHandler) submitEnrichment(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodPost {
		respondJSON(w, http.StatusMethodNotAllowed, map[string]string{"error": "method not allowed"})
		return
	}
	token, ok := bearerToken(r.Header.Get("Authorization"))
	if !ok {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	body, err := io.ReadAll(io.LimitReader(r.Body, phproxy.MaxBodyBytes+1))
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	if len(body) > phproxy.MaxBodyBytes {
		respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "request too large"})
		return
	}
	authCtx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	authenticatedID, err := h.agents.AuthenticateToken(authCtx, token)
	cancel()
	if errors.Is(err, agentdomain.ErrInvalidToken) {
		respondJSON(w, http.StatusUnauthorized, map[string]string{"error": "unauthorized"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	var envelope struct {
		SchemaVersion int `json:"schema_version"`
	}
	if json.Unmarshal(body, &envelope) != nil ||
		(strings.Contains(r.URL.Path, "/v1/") && envelope.SchemaVersion != phproxy.SchemaVersionV1) ||
		(strings.Contains(r.URL.Path, "/v2/") && envelope.SchemaVersion != phproxy.SchemaVersionV2) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request", "code": "invalid_schema"})
		return
	}
	submitCtx, submitCancel := context.WithTimeout(r.Context(), 8*time.Second)
	defer submitCancel()
	response, err := h.store.Submit(submitCtx, authenticatedID, body)
	if err != nil {
		switch {
		case errors.Is(err, phproxy.AgentIDMismatch{}):
			respondJSON(w, http.StatusForbidden, map[string]string{"error": "forbidden"})
		case errors.Is(err, phproxy.ErrInvalid):
			log.Printf("WARN ph-proxy enrichment rejected status=400")
			payload := map[string]any{"error": "invalid request", "code": "invalid_request"}
			var detail *phproxy.ValidationError
			if errors.As(err, &detail) {
				payload["code"] = detail.Code
				if detail.Field != "" {
					payload["field"] = detail.Field
				}
				if detail.PlayerIndex != nil {
					payload["player_index"] = *detail.PlayerIndex
				}
			}
			respondJSON(w, http.StatusBadRequest, payload)
		case errors.Is(err, phproxy.ErrIdempotencyConflict):
			respondJSON(w, http.StatusConflict, map[string]string{"error": "idempotency conflict"})
		default:
			var cap phproxy.PendingCapacityError
			if errors.As(err, &cap) {
				w.Header().Set("Retry-After", strconv.Itoa(cap.RetryAfter))
				respondJSON(w, http.StatusTooManyRequests, map[string]string{"error": "pending capacity reached"})
				return
			}
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		}
		return
	}
	respondJSON(w, http.StatusOK, response)
}

func (h *phProxyHandler) integrationStatus(w http.ResponseWriter, r *http.Request) {
	server := r.URL.Query().Get("server")
	if server == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	status, err := h.store.ServerStatus(ctx, server)
	if errors.Is(err, phproxy.ErrInvalid) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, status)
}
