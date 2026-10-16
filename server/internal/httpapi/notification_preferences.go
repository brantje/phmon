package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"phmon/server/internal/notifications"
)

type notificationPreferencesHandler struct{ store *notifications.Store }

func (h *notificationPreferencesHandler) catalog(w http.ResponseWriter, _ *http.Request) {
	respondJSON(w, http.StatusOK, map[string]any{"categories": notifications.Catalog()})
}

func (h *notificationPreferencesHandler) preferences(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if r.Method == http.MethodGet {
		value, err := h.store.Get(ctx)
		if err != nil {
			respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification preferences unavailable"})
			return
		}
		respondJSON(w, http.StatusOK, value)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, 8*1024)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var value notifications.Preferences
	if decoder.Decode(&value) != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification preferences"})
		return
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) || value.Validate() != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid notification preferences"})
		return
	}
	if err := h.store.Save(ctx, value); err != nil {
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "notification preferences unavailable"})
		return
	}
	respondJSON(w, http.StatusOK, value)
}
