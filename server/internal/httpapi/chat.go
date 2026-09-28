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

	"github.com/jackc/pgx/v5"
	"phmon/server/internal/chat"
)

const maxChatRequestBytes = 8 * 1024

type chatHandler struct {
	store *chat.Store
	live  *LiveHub
}

func (h *chatHandler) contacts(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, err := parseChatLimit(query.Get("limit"), 50)
	if err != nil || !validServerFilter(strings.TrimSpace(query.Get("server"))) || len(query.Get("character_id")) > 0 && len(query.Get("character_id")) != 36 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat filter"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	items, err := h.store.Contacts(ctx, strings.TrimSpace(query.Get("server")), strings.TrimSpace(query.Get("character_id")), limit)
	if err != nil {
		respondChatStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{"contacts": items})
}

func (h *chatHandler) messages(w http.ResponseWriter, r *http.Request) {
	query := r.URL.Query()
	limit, err := parseChatLimit(query.Get("limit"), 50)
	filter := chat.Filter{Server: strings.TrimSpace(query.Get("server")), CharacterID: strings.TrimSpace(query.Get("character_id")),
		Channel: strings.TrimSpace(query.Get("channel")), Peer: strings.TrimSpace(query.Get("peer")), Before: strings.TrimSpace(query.Get("before")), After: strings.TrimSpace(query.Get("after")), Limit: limit}
	if err != nil || !validServerFilter(filter.Server) || len(filter.Before) > 256 || len(filter.After) > 256 || chat.ValidateFilter(filter) != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat filter"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	page, err := h.store.Messages(ctx, filter)
	if err != nil {
		respondChatStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, page)
}

func (h *chatHandler) markRead(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, maxChatRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var body struct {
		Server      string `json:"server"`
		CharacterID string `json:"character_id"`
		Channel     string `json:"channel"`
		Peer        string `json:"peer"`
		MessageID   string `json:"message_id"`
	}
	if decoder.Decode(&body) != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid read cursor"})
		return
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid read cursor"})
		return
	}
	body.Server = strings.TrimSpace(body.Server)
	body.CharacterID = strings.TrimSpace(body.CharacterID)
	body.Channel = strings.TrimSpace(body.Channel)
	body.Peer = strings.TrimSpace(body.Peer)
	body.MessageID = strings.TrimSpace(body.MessageID)
	if !validServerFilter(body.Server) || body.Server == "" {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid read cursor"})
		return
	}
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	err := h.store.MarkRead(ctx, body.Server, body.CharacterID, body.Channel, body.Peer, body.MessageID)
	if err != nil {
		respondChatStoreError(w, err)
		return
	}
	readState, err := h.store.ReadState(ctx, body.Server, body.CharacterID)
	if err != nil {
		respondChatStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, map[string]any{
		"saved":             true,
		"contacts":          readState.Contacts,
		"unread_by_channel": readState.UnreadByChannel,
	})
	if h.live != nil {
		h.live.Invalidate()
	}
}

func (h *chatHandler) preferences(w http.ResponseWriter, r *http.Request) {
	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	if r.Method == http.MethodGet {
		p, err := h.store.Preferences(ctx)
		if err != nil {
			respondChatStoreError(w, err)
			return
		}
		respondJSON(w, http.StatusOK, p)
		return
	}
	r.Body = http.MaxBytesReader(w, r.Body, maxChatRequestBytes)
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	var p chat.Preferences
	if decoder.Decode(&p) != nil {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat preferences"})
		return
	}
	var trailing any
	if !errors.Is(decoder.Decode(&trailing), io.EOF) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat preferences"})
		return
	}
	if err := h.store.SavePreferences(ctx, p); err != nil {
		respondChatStoreError(w, err)
		return
	}
	respondJSON(w, http.StatusOK, p)
}

func parseChatLimit(raw string, fallback int) (int, error) {
	if raw == "" {
		return fallback, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil || value < 1 || value > chat.MaxPageSize {
		return 0, chat.ErrInvalid
	}
	return value, nil
}

func respondChatStoreError(w http.ResponseWriter, err error) {
	switch {
	case errors.Is(err, chat.ErrInvalid):
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid chat request"})
	case errors.Is(err, pgx.ErrNoRows):
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "chat message not found"})
	default:
		respondJSON(w, http.StatusServiceUnavailable, map[string]string{"error": "chat service unavailable"})
	}
}
