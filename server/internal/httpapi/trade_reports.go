package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"phmon/server/internal/tradenexus"
)

type tradeReportHandler struct {
	store *tradenexus.Store
}

func (h *tradeReportHandler) list(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := tradenexus.Filter{
		Server: strings.TrimSpace(query.Get("server")),
		Query:  strings.TrimSpace(query.Get("q")),
		Cursor: strings.TrimSpace(query.Get("cursor")),
		Limit:  10,
	}
	if !validServerFilter(filter.Server) || len(filter.Query) > 64 || len(filter.Cursor) > 256 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid trade report filter"})
		return
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > tradenexus.MaxPageSize {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		filter.Limit = limit
	}
	if raw := query.Get("from"); raw != "" {
		parsed, err := parseEventBound(raw, false)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid from date"})
			return
		}
		filter.From = &parsed
	}
	if raw := query.Get("to"); raw != "" {
		parsed, err := parseEventBound(raw, true)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid to date"})
			return
		}
		filter.To = &parsed
	}
	if filter.From != nil && filter.To != nil && !filter.To.After(*filter.From) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "to date precedes from date"})
		return
	}
	if _, err := tradenexus.DecodeCursor(filter.Cursor); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid cursor"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.ListTrades(ctx, filter)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, page)
}
