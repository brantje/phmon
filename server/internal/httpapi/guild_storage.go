package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"phmon/server/internal/resources"
)

type guildStorageHandler struct{ store *resources.Store }

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
