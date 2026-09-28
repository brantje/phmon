package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"log/slog"
	"net/http"
	"strings"
	"time"

	"phmon/server/internal/resources"
)

type guildStorageStore interface {
	GuildStorage(context.Context, string, string) ([]resources.Observation, error)
	DeleteGuildStorage(context.Context, string, string) (int64, int64, error)
}

type guildStorageHandler struct{ store guildStorageStore }

func (h *guildStorageHandler) get(w http.ResponseWriter, r *http.Request) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	guild := strings.TrimSpace(r.URL.Query().Get("guild"))
	if len(server) == 0 || len(server) > 100 || len(guild) == 0 || len(guild) > 100 || strings.ContainsRune(server, 0) || strings.ContainsRune(guild, 0) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "server and guild are required"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	items, err := h.store.GuildStorage(ctx, server, guild)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "guild storage unavailable"})
		return
	}
	if items == nil {
		items = []resources.Observation{}
	}
	respondJSON(w, http.StatusOK, resources.GuildView{Server: server, Guild: guild, Items: items})
}

func (h *guildStorageHandler) delete(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, 4096)
	var body struct {
		Server       string `json:"server"`
		Guild        string `json:"guild"`
		Confirmation string `json:"confirmation"`
	}
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&body); err != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	if !validGuildScope(body.Server, body.Guild) || body.Confirmation != body.Guild {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "type the exact guild name to confirm removal"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 4*time.Second)
	defer cancel()
	observationCount, itemCount, err := h.store.DeleteGuildStorage(ctx, body.Server, body.Guild)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "guild storage removal failed"})
		return
	}
	slog.Info("guild storage records removed", "operator", "operator", "server", strings.ToLower(strings.TrimSpace(body.Server)), "guild", strings.ToLower(strings.TrimSpace(body.Guild)), "observations", observationCount, "items", itemCount)
	respondJSON(w, http.StatusOK, map[string]any{"server": body.Server, "guild": body.Guild, "deleted_observations": observationCount, "deleted_items": itemCount, "retention": "A later phBot observation may create a new saved snapshot; no in-game contents were changed."})
}

func validGuildScope(server, guild string) bool {
	return server == strings.TrimSpace(server) && guild == strings.TrimSpace(guild) &&
		len(server) > 0 && len(server) <= 100 && len(guild) > 0 && len(guild) <= 100 &&
		!strings.ContainsRune(server, 0) && !strings.ContainsRune(guild, 0)
}
