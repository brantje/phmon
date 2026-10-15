package httpapi

import (
	"context"
	"errors"
	"net/http"
	"strconv"
	"strings"
	"time"

	"phmon/server/internal/resources"
)

type itemSearchHandler struct {
	store *resources.Store
}

func (h *itemSearchHandler) list(w http.ResponseWriter, r *http.Request) {
	q := r.URL.Query()
	filter := resources.ItemSearchFilter{
		Server:           strings.TrimSpace(q.Get("server")),
		Query:            strings.TrimSpace(q.Get("q")),
		Container:        strings.TrimSpace(q.Get("container")),
		Owner:            strings.TrimSpace(q.Get("owner")),
		Type:             strings.TrimSpace(q.Get("item_type")),
		Subcategory:      strings.TrimSpace(q.Get("subcategory")),
		Degree:           strings.TrimSpace(q.Get("degree")),
		IncludeEquipment: true,
		Cursor:           q.Get("cursor"),
		Limit:            30,
	}
	if !validServerFilter(filter.Server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server"})
		return
	}
	if raw := q.Get("include_sets"); raw != "" {
		if raw != "true" && raw != "false" {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid sets filter"})
			return
		}
		filter.IncludeEquipment = raw == "true"
	}
	if raw := q.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > 100 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		filter.Limit = limit
	}
	if len(filter.Cursor) > 512 || len(filter.Query) > 128 || len(filter.Owner) > 100 ||
		len(filter.Type) > 64 || len(filter.Subcategory) > 64 || len(filter.Degree) > 2 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid item search filter"})
		return
	}
	if filter.Server == "" && (filter.Type != "" || filter.Subcategory != "" || filter.Degree != "") {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "select a server for taxonomy filters"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.ItemSearch(ctx, filter)
	if err != nil {
		code := http.StatusServiceUnavailable
		if errors.Is(err, resources.ErrInvalid) {
			code = http.StatusBadRequest
		}
		respondJSON(w, code, map[string]string{"error": "item search unavailable or invalid"})
		return
	}
	respondJSON(w, http.StatusOK, page)
}
