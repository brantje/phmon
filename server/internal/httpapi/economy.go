package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"phmon/server/internal/chat"
)

type economyHandler struct {
	store *chat.Store
}

func (h *economyHandler) offers(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	filter := chat.OfferFilter{
		Server: strings.TrimSpace(query.Get("server")),
		Sender: strings.TrimSpace(query.Get("sender")),
		Query:  strings.TrimSpace(query.Get("q")),
		Kind:   strings.TrimSpace(query.Get("offer_type")),
		Cursor: strings.TrimSpace(query.Get("cursor")),
		Limit:  30,
	}
	if !validServerFilter(filter.Server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server filter"})
		return
	}
	if value := query.Get("limit"); value != "" {
		n, err := strconv.Atoi(value)
		if err != nil || n < 1 || n > chat.MaxPageSize {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid page size"})
			return
		}
		filter.Limit = n
	}
	for _, bound := range []struct {
		raw string
		target **time.Time
	}{
		{query.Get("from"), &filter.From},
		{query.Get("to"), &filter.To},
	} {
		if bound.raw == "" {
			continue
		}
		parsed, err := parseEventBound(bound.raw, false)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid date range"})
			return
		}
		*bound.target = &parsed
	}
	if len(filter.Sender) > 64 || len(filter.Query) > 128 || len(filter.Cursor) > 256 ||
		(filter.From != nil && filter.To != nil && !filter.To.After(*filter.From)) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid economy filter"})
		return
	}
	switch filter.Kind {
	case "", "WTB", "WTS", "WTT", "unclassified":
	default:
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid offer type"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Offers(ctx, filter)
	if err != nil {
		code := http.StatusServiceUnavailable
		if errors.Is(err, chat.ErrInvalid) {
			code = http.StatusBadRequest
		}
		respondJSON(w, code, map[string]string{"error": "global offers unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, page)
}
