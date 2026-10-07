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
	"phmon/server/internal/characters"
	"phmon/server/internal/resources"
)

type characterHandler struct {
	store     *characters.Store
	live      *LiveHub
	resources *resources.Store
}

func (h *characterHandler) list(w http.ResponseWriter, r *http.Request) {
	groupID := r.URL.Query().Get("group_id")
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if len(query) > 100 || !validServerFilter(server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid character filter"})
		return
	}
	if groupID != "" && !agentdomain.ValidAgentID(groupID) {
		respondJSON(w, 400, map[string]string{"error": "invalid group_id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	items, err := h.store.ListScoped(ctx, query, groupID, server)
	if err != nil {
		respondJSON(w, 503, map[string]string{"error": "service unavailable"})
		return
	}
	for i := range items {
		items[i] = characterWithPortrait(items[i], h.resources)
	}
	respondJSON(w, 200, map[string]any{"characters": items})
}
func (h *characterHandler) get(w http.ResponseWriter, r *http.Request) {
	if !agentdomain.ValidAgentID(r.PathValue("id")) {
		respondJSON(w, 400, map[string]string{"error": "invalid character_id"})
		return
	}
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if !validServerFilter(server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server filter"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	item, err := h.store.GetScoped(ctx, r.PathValue("id"), server)
	if errors.Is(err, characters.ErrNotFound) {
		respondJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		respondJSON(w, 503, map[string]string{"error": "service unavailable"})
		return
	}
	item = characterWithPortrait(item, h.resources)
	respondJSON(w, 200, item)
}
func (h *characterHandler) groups(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if !validServerFilter(server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server filter"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := h.store.GroupsScoped(ctx, server)
	if err != nil {
		respondJSON(w, 503, map[string]string{"error": "service unavailable"})
		return
	}
	items = groupsWithPortraits(items, h.resources)
	respondJSON(w, 200, map[string]any{"groups": items})
}
func decodeGroupName(r *http.Request) (string, error) {
	var body struct {
		Name string `json:"name"`
	}
	d := json.NewDecoder(io.LimitReader(r.Body, 4097))
	d.DisallowUnknownFields()
	if err := d.Decode(&body); err != nil {
		return "", err
	}
	if len(body.Name) > 80 {
		return "", errors.New("group name is too long")
	}
	var extra any
	if err := d.Decode(&extra); !errors.Is(err, io.EOF) {
		return "", errors.New("unexpected trailing data")
	}
	return body.Name, nil
}
func (h *characterHandler) createGroup(w http.ResponseWriter, r *http.Request) {
	name, err := decodeGroupName(r)
	if err != nil {
		respondJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	g, err := h.store.CreateGroup(ctx, name)
	if err != nil {
		respondJSON(w, 400, map[string]string{"error": "invalid group"})
		return
	}
	h.live.Invalidate()
	h.live.InvalidateAnalytics()
	respondJSON(w, 201, g)
}
func (h *characterHandler) renameGroup(w http.ResponseWriter, r *http.Request) {
	if !agentdomain.ValidAgentID(r.PathValue("id")) {
		respondJSON(w, 400, map[string]string{"error": "invalid group_id"})
		return
	}
	name, err := decodeGroupName(r)
	if err != nil {
		respondJSON(w, 400, map[string]string{"error": "invalid request"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	err = h.store.RenameGroup(ctx, r.PathValue("id"), name)
	if errors.Is(err, characters.ErrNotFound) {
		respondJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		respondJSON(w, 400, map[string]string{"error": "invalid group"})
		return
	}
	h.live.Invalidate()
	h.live.InvalidateAnalytics()
	w.WriteHeader(http.StatusNoContent)
}
func (h *characterHandler) deleteGroup(w http.ResponseWriter, r *http.Request) {
	if !agentdomain.ValidAgentID(r.PathValue("id")) {
		respondJSON(w, 400, map[string]string{"error": "invalid group_id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	err := h.store.DeleteGroup(ctx, r.PathValue("id"))
	if errors.Is(err, characters.ErrNotFound) {
		respondJSON(w, 404, map[string]string{"error": "not found"})
		return
	}
	if err != nil {
		respondJSON(w, 503, map[string]string{"error": "service unavailable"})
		return
	}
	h.live.Invalidate()
	h.live.InvalidateAnalytics()
	w.WriteHeader(http.StatusNoContent)
}
func (h *characterHandler) member(w http.ResponseWriter, r *http.Request, add bool) {
	if !agentdomain.ValidAgentID(r.PathValue("id")) || !agentdomain.ValidAgentID(r.PathValue("characterID")) {
		respondJSON(w, 400, map[string]string{"error": "invalid identifier"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	err := h.store.SetMember(ctx, r.PathValue("id"), r.PathValue("characterID"), add)
	if err != nil {
		code := 400
		if errors.Is(err, characters.ErrNotFound) {
			code = 404
		}
		respondJSON(w, code, map[string]string{"error": "membership update failed"})
		return
	}
	h.live.Invalidate()
	h.live.InvalidateAnalytics()
	w.WriteHeader(http.StatusNoContent)
}
func (h *characterHandler) addMember(w http.ResponseWriter, r *http.Request) { h.member(w, r, true) }
func (h *characterHandler) removeMember(w http.ResponseWriter, r *http.Request) {
	h.member(w, r, false)
}
func validWireState(s characters.State) bool {
	if s.Model == nil && s.Level == nil && s.HP == nil && s.HPMax == nil && s.MP == nil && s.MPMax == nil && s.CurrentEXP == nil && s.MaxEXP == nil && s.SP == nil && s.Gold == nil && s.Region == nil && s.Zone == nil && s.X == nil && s.Y == nil && s.Z == nil && s.Botting == nil && s.Dead == nil {
		return false
	}
	if s.Model != nil && (*s.Model < 1 || *s.Model > 4294967295) {
		return false
	}
	if s.Level != nil && (*s.Level < 0 || *s.Level > 255) {
		return false
	}
	for _, v := range []*int64{s.HP, s.HPMax, s.MP, s.MPMax, s.CurrentEXP, s.MaxEXP, s.SP, s.Gold} {
		if v != nil && *v < 0 {
			return false
		}
	}
	for _, v := range []*float64{s.X, s.Y, s.Z} {
		if v != nil && (*v < -1000000 || *v > 1000000) {
			return false
		}
	}
	if s.Zone != nil && (len(*s.Zone) > 100 || strings.TrimSpace(*s.Zone) == "") {
		return false
	}
	if s.Region != nil && (*s.Region < -32768 || *s.Region > 65535 || *s.Region == 0) {
		return false
	}
	return true
}

func validMessageTime(value string) bool {
	if len(value) < 20 || len(value) > 40 {
		return false
	}
	_, err := time.Parse(time.RFC3339, value)
	return err == nil
}
