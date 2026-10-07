package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strings"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/analytics"
)

type analyticsRateResetHandler struct {
	store *analytics.Store
	live  *LiveHub
}

func (h *analyticsRateResetHandler) reset(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body struct {
		CharacterID    string `json:"character_id"`
		CharacterName  string `json:"character_name"`
		IdempotencyKey string `json:"idempotency_key"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_payload"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_payload"})
		return
	}
	if !agentdomain.ValidAgentID(body.CharacterID) || body.CharacterName == "" || strings.TrimSpace(body.CharacterName) != body.CharacterName || body.IdempotencyKey == "" || strings.TrimSpace(body.IdempotencyKey) != body.IdempotencyKey || strings.ContainsRune(body.IdempotencyKey, 0) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_reset_request"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	resetAt, err := h.store.ResetRateWindow(ctx, body.CharacterID, body.CharacterName, "operator", body.IdempotencyKey)
	if err != nil {
		switch {
		case errors.Is(err, analytics.ErrRateResetNameMismatch):
			respondJSON(w, http.StatusConflict, map[string]string{"error": "character_name_mismatch"})
		case errors.Is(err, analytics.ErrRateResetNotFound):
			respondJSON(w, http.StatusNotFound, map[string]string{"error": "character_not_found"})
		case errors.Is(err, analytics.ErrInvalidFilter):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid_reset_request"})
		default:
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "rate_reset_unavailable"})
		}
		return
	}
	if h.live != nil {
		h.live.InvalidateAnalytics()
	}
	respondJSON(w, http.StatusOK, map[string]any{"reset_at": resetAt, "rates_only": true})
}
