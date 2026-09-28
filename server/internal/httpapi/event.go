package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/events"
)

type eventHandler struct{ store *events.Store }

func (h *eventHandler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := events.Filter{
		Server:         strings.TrimSpace(query.Get("server")),
		CharacterID:    strings.TrimSpace(query.Get("character_id")),
		CharacterQuery: strings.TrimSpace(query.Get("q")),
		Kind:           strings.TrimSpace(query.Get("kind")),
		Cursor:         strings.TrimSpace(query.Get("cursor")),
		Limit:          10,
	}
	if !validServerFilter(filter.Server) || len(filter.CharacterQuery) > 64 || len(filter.CharacterID) > 0 && !agentdomain.ValidAgentID(filter.CharacterID) ||
		filter.Kind != "" && filter.Kind != events.DeathKind || len(filter.Cursor) > 256 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid event filter"})
		return
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > events.MaxPageSize {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		filter.Limit = limit
	}
	var err error
	if raw := query.Get("from"); raw != "" {
		parsed, parseErr := time.Parse("2006-01-02", raw)
		if parseErr != nil {
			respondJSON(w, 400, map[string]string{"error": "invalid from date"})
			return
		}
		filter.From = &parsed
	}
	if raw := query.Get("to"); raw != "" {
		parsed, parseErr := time.Parse("2006-01-02", raw)
		if parseErr != nil {
			respondJSON(w, 400, map[string]string{"error": "invalid to date"})
			return
		}
		endExclusive := parsed.AddDate(0, 0, 1)
		filter.To = &endExclusive
	}
	if filter.From != nil && filter.To != nil && !filter.To.After(*filter.From) {
		respondJSON(w, 400, map[string]string{"error": "to date precedes from date"})
		return
	}
	if _, err = events.DecodeCursor(filter.Cursor); err != nil {
		respondJSON(w, 400, map[string]string{"error": "invalid cursor"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.List(ctx, filter)
	if err != nil {
		respondJSON(w, 503, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, 200, page)
}
