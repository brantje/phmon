package httpapi

import (
	"context"
	"net/http"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/resources"
)

type characterResourceHandler struct{ store *resources.Store }

func (h *characterResourceHandler) get(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	if !agentdomain.ValidAgentID(id) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid character id"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 2*time.Second)
	defer cancel()
	view, err := h.store.Character(ctx, id)
	if err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "resource state unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, view)
}
