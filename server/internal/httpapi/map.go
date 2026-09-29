package httpapi

import (
	"context"
	"net/http"
	"strconv"
	"strings"
	"time"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/resources"
)

type mapHandler struct {
	resources *resources.Store
	mobs      *mobs.Store
}

func (h *mapHandler) profile(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if !validServerFilter(server) || server == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "server is required"})
		return
	}
	dataset, ok := h.resources.DatasetIDForServer(server)
	if !ok {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "map profile unavailable"})
		return
	}
	profile, err := mapprofile.ForServer(server, dataset)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "map profile unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, profile)
}

func (h *mapHandler) density(w http.ResponseWriter, r *http.Request) {
	if h.mobs == nil || h.resources == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "map observations unavailable"})
		return
	}
	query := r.URL.Query()
	server := strings.TrimSpace(query.Get("server"))
	dataset, knownServer := h.resources.DatasetIDForServer(server)
	if !validServerFilter(server) || server == "" || !knownServer || dataset != mapprofile.GreatestDatasetID {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "unsupported map dataset"})
		return
	}
	areaID := strings.TrimSpace(query.Get("area"))
	floorID := strings.TrimSpace(query.Get("floor"))
	if floorID == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "floor is required"})
		return
	}
	from, fromErr := parseEventBound(strings.TrimSpace(query.Get("from")), false)
	to, toErr := parseEventBound(strings.TrimSpace(query.Get("to")), false)
	if fromErr != nil || toErr != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "from and to RFC3339 timestamps are required"})
		return
	}
	filter := mobs.DensityFilter{Server: server, AreaID: areaID, FloorID: floorID, From: from, To: to, Limit: 200,
		MonsterType: strings.TrimSpace(query.Get("monster_type"))}
	if raw := query.Get("model_id"); raw != "" {
		model, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid model_id"})
			return
		}
		filter.ModelID = &model
	}
	if raw := query.Get("limit"); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil || limit < 1 || limit > mobs.MaxQueryCells {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid limit"})
			return
		}
		filter.Limit = limit
	}
	if err := mobs.ValidateDensityFilter(filter); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid observer-metric scope"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	result, err := h.mobs.Density(ctx, filter)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "observer-local observation query unavailable"})
		return
	}
	result.DatasetVersion = dataset
	respondJSON(w, http.StatusOK, result)
}
