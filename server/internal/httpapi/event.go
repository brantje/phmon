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

func parseEventBound(value string, endOfDate bool) (time.Time, error) {
	if parsed, err := time.Parse(time.RFC3339Nano, value); err == nil {
		return parsed.UTC(), nil
	}
	parsed, err := time.Parse("2006-01-02", value)
	if err != nil {
		return time.Time{}, err
	}
	if endOfDate {
		parsed = parsed.AddDate(0, 0, 1)
	}
	return parsed.UTC(), nil
}

type eventHandler struct{ store *events.Store }

func (h *eventHandler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := events.Filter{
		Server:         strings.TrimSpace(query.Get("server")),
		CharacterID:    strings.TrimSpace(query.Get("character_id")),
		CharacterQuery: strings.TrimSpace(query.Get("q")),
		Kind:           strings.TrimSpace(query.Get("kind")),
		Category:       strings.TrimSpace(query.Get("category")),
		ItemQuery:      strings.TrimSpace(query.Get("item")),
		Cursor:         strings.TrimSpace(query.Get("cursor")),
		Limit:          10,
	}
	if !validServerFilter(filter.Server) || len(filter.CharacterQuery) > 64 || len(filter.CharacterID) > 0 && !agentdomain.ValidAgentID(filter.CharacterID) ||
		!events.ValidKind(filter.Kind) || !events.ValidCategory(filter.Category) || len(filter.ItemQuery) > 128 || len(filter.Cursor) > 256 {
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
		parsed, parseErr := parseEventBound(raw, false)
		if parseErr != nil {
			respondJSON(w, 400, map[string]string{"error": "invalid from date"})
			return
		}
		filter.From = &parsed
	}
	if raw := query.Get("to"); raw != "" {
		parsed, parseErr := parseEventBound(raw, true)
		if parseErr != nil {
			respondJSON(w, 400, map[string]string{"error": "invalid to date"})
			return
		}
		filter.To = &parsed
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
