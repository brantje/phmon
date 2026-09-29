package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"strconv"
	"strings"
	"time"

	"phmon/server/internal/mapanalytics"
	"phmon/server/internal/mapprofile"
	"phmon/server/internal/mobs"
	"phmon/server/internal/resources"
)

type mapHandler struct {
	resources *resources.Store
	mobs      *mobs.Store
	analytics *mapanalytics.Store
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

func validateAreaFloor(profile mapprofile.Profile, areaID, floorID string) bool {
	for _, area := range profile.Areas {
		if area.ID != areaID {
			continue
		}
		for _, floor := range area.Floors {
			if floor.ID == floorID {
				return true
			}
		}
	}
	return false
}

func (h *mapHandler) heatmapFilter(r *http.Request, forcedLayer string) (mapanalytics.Filter, error) {
	if h.resources == nil || h.analytics == nil {
		return mapanalytics.Filter{}, errors.New("map analytics unavailable")
	}
	query := r.URL.Query()
	server := strings.TrimSpace(query.Get("server"))
	if server == "" || !validServerFilter(server) {
		return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
	}
	dataset, ok := h.resources.DatasetIDForServer(server)
	if !ok {
		return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
	}
	profile, err := mapprofile.ForServer(server, dataset)
	if err != nil {
		return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
	}
	areaID, floorID := strings.TrimSpace(query.Get("area")), strings.TrimSpace(query.Get("floor"))
	if !validateAreaFloor(profile, areaID, floorID) {
		return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
	}
	from, fromErr := parseEventBound(strings.TrimSpace(query.Get("from")), false)
	to, toErr := parseEventBound(strings.TrimSpace(query.Get("to")), false)
	if fromErr != nil || toErr != nil {
		return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
	}
	layer := forcedLayer
	if layer == "" {
		layer = strings.TrimSpace(query.Get("layer"))
	}
	filter := mapanalytics.Filter{
		Layer: layer, Server: server, DatasetID: dataset, AreaID: areaID, FloorID: floorID,
		CharacterID: strings.TrimSpace(query.Get("character_id")), MonsterType: strings.TrimSpace(query.Get("monster_type")),
		From: from, To: to,
	}
	if raw := strings.TrimSpace(query.Get("region")); raw != "" {
		region, err := strconv.Atoi(raw)
		if err != nil {
			return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
		}
		filter.Region = &region
	}
	if raw := strings.TrimSpace(query.Get("model_id")); raw != "" {
		model, err := strconv.ParseInt(raw, 10, 64)
		if err != nil {
			return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
		}
		filter.ModelID = &model
	}
	if raw := strings.TrimSpace(query.Get("resolution")); raw != "" {
		resolution, err := strconv.ParseFloat(raw, 64)
		if err != nil {
			return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
		}
		filter.Resolution = resolution
	}
	if raw := strings.TrimSpace(query.Get("limit")); raw != "" {
		limit, err := strconv.Atoi(raw)
		if err != nil {
			return mapanalytics.Filter{}, mapanalytics.ErrInvalidFilter
		}
		filter.Limit = limit
	}
	return mapanalytics.NormalizeFilter(filter)
}

func (h *mapHandler) heatmap(w http.ResponseWriter, r *http.Request) {
	filter, err := h.heatmapFilter(r, "")
	if err != nil {
		code := http.StatusBadRequest
		if h.analytics == nil || h.resources == nil {
			code = http.StatusServiceUnavailable
		}
		respondJSON(w, code, map[string]string{"error": "invalid heatmap scope"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 5*time.Second)
	defer cancel()
	result, err := h.analytics.Heatmap(ctx, filter)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "heatmap query unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, result)
}

func (h *mapHandler) heatmapFacets(w http.ResponseWriter, r *http.Request) {
	filter, err := h.heatmapFilter(r, mapanalytics.LayerMobTypes)
	if err != nil {
		code := http.StatusBadRequest
		if h.analytics == nil || h.resources == nil {
			code = http.StatusServiceUnavailable
		}
		respondJSON(w, code, map[string]string{"error": "invalid heatmap scope"})
		return
	}
	limit := 100
	if raw := strings.TrimSpace(r.URL.Query().Get("facet_limit")); raw != "" {
		parsed, parseErr := strconv.Atoi(raw)
		if parseErr != nil || parsed < 1 || parsed > 200 {
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid facet limit"})
			return
		}
		limit = parsed
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	facets, err := h.analytics.MobFacets(ctx, filter, limit)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "heatmap facets unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"facets": facets})
}

func (h *mapHandler) heatmapReset(w http.ResponseWriter, r *http.Request) {
	if h.resources == nil || h.analytics == nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "map analytics unavailable"})
		return
	}
	var body struct {
		Layer        string `json:"layer"`
		Server       string `json:"server"`
		AreaID       string `json:"area_id"`
		FloorID      string `json:"floor_id"`
		Region       *int   `json:"region"`
		CharacterID  string `json:"character_id"`
		MonsterType  string `json:"monster_type"`
		ModelID      *int64 `json:"model_id"`
		From         string `json:"from"`
		To           string `json:"to"`
		ConfirmBroad bool   `json:"confirm_broad"`
	}
	decoder := json.NewDecoder(io.LimitReader(r.Body, 8193))
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reset request"})
		return
	}
	var extra any
	if err := decoder.Decode(&extra); !errors.Is(err, io.EOF) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reset request"})
		return
	}
	body.Server, body.AreaID, body.FloorID = strings.TrimSpace(body.Server), strings.TrimSpace(body.AreaID), strings.TrimSpace(body.FloorID)
	dataset, ok := h.resources.DatasetIDForServer(body.Server)
	if !ok || !validServerFilter(body.Server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reset scope"})
		return
	}
	profile, err := mapprofile.ForServer(body.Server, dataset)
	if err != nil || !validateAreaFloor(profile, body.AreaID, body.FloorID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reset scope"})
		return
	}
	from, fromErr := parseEventBound(strings.TrimSpace(body.From), false)
	to, toErr := parseEventBound(strings.TrimSpace(body.To), false)
	if fromErr != nil || toErr != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reset time range"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	reset, err := h.analytics.Reset(ctx, mapanalytics.ResetScope{
		Layer: body.Layer, Server: body.Server, DatasetID: dataset, AreaID: body.AreaID, FloorID: body.FloorID,
		Region: body.Region, CharacterID: strings.TrimSpace(body.CharacterID), MonsterType: strings.TrimSpace(body.MonsterType),
		ModelID: body.ModelID, From: from, To: to, ConfirmBroad: body.ConfirmBroad,
	})
	if err != nil {
		switch {
		case errors.Is(err, mapanalytics.ErrInvalidFilter):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid reset scope"})
		case errors.Is(err, mapanalytics.ErrBroadResetUnconfirmed):
			respondJSON(w, http.StatusBadRequest, map[string]string{"error": "broad heatmap reset requires explicit confirmation"})
		default:
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "heatmap reset unavailable"})
		}
		return
	}
	respondJSON(w, http.StatusCreated, reset)
}
