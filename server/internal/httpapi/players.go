package httpapi

import (
	"context"
	"errors"
	"net/http"
	"net/url"
	"strconv"
	"strings"
	"time"

	"phmon/server/internal/phproxy"
	"phmon/server/internal/players"
)

type playerHandler struct {
	store      *players.Registry
	enrichment *phproxy.Store
}

type playerDetailResponse struct {
	players.PlayerRecord
	Enrichment *phproxy.EnrichmentProjection `json:"enrichment,omitempty"`
}

func (h *playerHandler) list(w http.ResponseWriter, r *http.Request) {
	filter, err := playerFilter(r.URL.Query(), time.Now().UTC())
	if err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid player filter"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.List(ctx, filter)
	if errors.Is(err, players.ErrInvalidFilter) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid player filter"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, page)
}

func (h *playerHandler) get(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	player, err := h.store.Get(ctx, r.PathValue("id"))
	if errors.Is(err, players.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "player not found"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	detail := playerDetailResponse{PlayerRecord: player}
	// Alias detail URLs retain their native history, while equipment comes from
	// the displayed normal-name anchor and keeps its original field evidence.
	enrichmentID, enrichmentName := player.ID, player.ObservedName
	if identity := player.DisplayIdentity; identity != nil && identity.CanonicalID != player.ID && identity.PlayerName != nil {
		enrichmentID, enrichmentName = identity.CanonicalID, *identity.PlayerName
	}
	if h.enrichment != nil {
		sk := strings.ToLower(strings.TrimSpace(player.Server))
		nk := strings.ToLower(strings.TrimSpace(enrichmentName))
		enrichment, enrichErr := h.enrichment.LoadEnrichmentWithPromotion(ctx, enrichmentID, player.Server, sk, nk)
		if enrichErr == nil && enrichment != nil {
			// Promotion can attach a newly staged mapping during this read.
			if refreshed, refreshErr := h.store.Get(ctx, player.ID); refreshErr == nil {
				detail.PlayerRecord = refreshed
			}
			// A concurrent mapping change must not attach the previous owner's gear.
			if detail.DisplayIdentity == nil || detail.DisplayIdentity.CanonicalID == enrichmentID {
				detail.Enrichment = enrichment
			}
		}
	}
	respondJSON(w, http.StatusOK, detail)
}

func (h *playerHandler) observations(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := playerPage(r.URL.Query())
	if !ok {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid player page"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Observations(ctx, r.PathValue("id"), limit, offset)
	if errors.Is(err, players.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "player not found"})
		return
	}
	if errors.Is(err, players.ErrInvalidFilter) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid player page"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, page)
}

func (h *playerHandler) levels(w http.ResponseWriter, r *http.Request) {
	limit, offset, ok := playerPage(r.URL.Query())
	if !ok {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid player page"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Levels(ctx, r.PathValue("id"), limit, offset)
	if errors.Is(err, players.ErrNotFound) {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "player not found"})
		return
	}
	if errors.Is(err, players.ErrInvalidFilter) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid player page"})
		return
	}
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "service unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, page)
}

func playerPage(query url.Values) (int, int, bool) {
	limit := 25
	offset := 0
	if raw := query.Get("limit"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 1 || parsed > 100 {
			return 0, 0, false
		}
		limit = parsed
	}
	if raw := query.Get("offset"); raw != "" {
		parsed, err := strconv.Atoi(raw)
		if err != nil || parsed < 0 || parsed > 100000 {
			return 0, 0, false
		}
		offset = parsed
	}
	return limit, offset, true
}

func playerFilter(query url.Values, now time.Time) (players.ListFilter, error) {
	filter := players.ListFilter{
		Server:     strings.TrimSpace(query.Get("server")),
		Name:       strings.TrimSpace(query.Get("q")),
		Guild:      strings.TrimSpace(query.Get("guild")),
		Job:        strings.TrimSpace(query.Get("job")),
		Jobbing:    strings.TrimSpace(query.Get("jobbing")),
		Model:      strings.TrimSpace(query.Get("model")),
		Sort:       strings.TrimSpace(query.Get("sort")),
		Descending: query.Get("dir") != "asc",
		HideOwn:    true,
	}
	switch strings.ToLower(strings.TrimSpace(query.Get("hide_own"))) {
	case "", "1", "true", "yes":
	case "0", "false", "no":
		filter.HideOwn = false
	default:
		return filter, players.ErrInvalidFilter
	}
	if filter.Server == "all" {
		filter.Server = ""
	}
	if filter.Job == "all" {
		filter.Job = ""
	}
	if filter.Jobbing == "all" {
		filter.Jobbing = ""
	}
	if filter.Sort == "" {
		filter.Descending = true
	}
	var err error
	if filter.MinLevel, err = optionalBound(query.Get("min_level"), 1, 255); err != nil {
		return filter, err
	}
	if filter.MaxLevel, err = optionalBound(query.Get("max_level"), 1, 255); err != nil {
		return filter, err
	}
	if filter.MinJobLevel, err = optionalBound(query.Get("min_job_level"), 0, 255); err != nil {
		return filter, err
	}
	if filter.MaxJobLevel, err = optionalBound(query.Get("max_job_level"), 0, 255); err != nil {
		return filter, err
	}
	switch query.Get("seen") {
	case "", "all":
	case "1h":
		since := now.Add(-time.Hour)
		filter.SeenSince = &since
	case "24h":
		since := now.Add(-24 * time.Hour)
		filter.SeenSince = &since
	case "7d":
		since := now.Add(-7 * 24 * time.Hour)
		filter.SeenSince = &since
	case "30d":
		since := now.Add(-30 * 24 * time.Hour)
		filter.SeenSince = &since
	default:
		return filter, players.ErrInvalidFilter
	}
	limit, offset, ok := playerPage(query)
	if !ok {
		return filter, players.ErrInvalidFilter
	}
	filter.Limit = limit
	filter.Offset = offset
	return filter, nil
}

func optionalBound(raw string, minimum, maximum int) (*int, error) {
	if strings.TrimSpace(raw) == "" {
		return nil, nil
	}
	parsed, err := strconv.Atoi(raw)
	if err != nil || parsed < minimum || parsed > maximum {
		return nil, players.ErrInvalidFilter
	}
	return &parsed, nil
}
