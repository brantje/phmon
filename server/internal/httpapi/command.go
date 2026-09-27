package httpapi

import (
	"context"
	"encoding/json"
	"errors"
	"io"
	"net/http"
	"time"

	"phmon/server/internal/commands"
)

const commandRequestMaxBytes = 16 * 1024

type commandHandler struct {
	service *commands.Service
}

type commandRequest struct {
	CharacterID       string          `json:"character_id"`
	ExpectedSessionID string          `json:"expected_session_id"`
	Name              string          `json:"name"`
	Args              json.RawMessage `json:"args"`
	IdempotencyKey    string          `json:"idempotency_key"`
	Confirmation      bool            `json:"confirmation"`
}

func (h *commandHandler) submit(w http.ResponseWriter, r *http.Request) {
	r.Body = http.MaxBytesReader(w, r.Body, commandRequestMaxBytes)
	var request commandRequest
	decoder := json.NewDecoder(r.Body)
	decoder.DisallowUnknownFields()
	if err := decoder.Decode(&request); err != nil {
		respondCommandError(w, http.StatusBadRequest, "invalid_payload", "invalid command payload")
		return
	}
	var trailing any
	if err := decoder.Decode(&trailing); !errors.Is(err, io.EOF) {
		respondCommandError(w, http.StatusBadRequest, "invalid_payload", "invalid command payload")
		return
	}

	ctx, cancel := context.WithTimeout(r.Context(), 3*time.Second)
	defer cancel()
	command, duplicate, unsupportedReason, err := h.service.Submit(ctx, "operator", commands.SubmitInput{
		CharacterID:       request.CharacterID,
		ExpectedSessionID: request.ExpectedSessionID,
		Name:              request.Name,
		Args:              request.Args,
		IdempotencyKey:    request.IdempotencyKey,
		Confirmation:      request.Confirmation,
	})
	if err != nil {
		switch {
		case errors.Is(err, commands.ErrInvalid):
			respondCommandError(w, http.StatusBadRequest, "invalid_command", "command arguments are invalid")
		case errors.Is(err, commands.ErrNotFound):
			respondCommandError(w, http.StatusNotFound, "unknown_target", "character was not found")
		case errors.Is(err, commands.ErrStaleSession):
			respondCommandError(w, http.StatusConflict, "stale_session", "character session changed or is offline")
		case errors.Is(err, commands.ErrIdempotencyConflict):
			respondCommandError(w, http.StatusConflict, "idempotency_conflict", "idempotency key is already used by a different request")
		case errors.Is(err, commands.ErrInFlight):
			respondCommandError(w, http.StatusConflict, "command_in_flight", "character already has an in-flight command")
		case errors.Is(err, commands.ErrUnsupported):
			if unsupportedReason == "" {
				unsupportedReason = "unsupported"
			}
			respondCommandError(w, http.StatusUnprocessableEntity, unsupportedReason, "command is not supported by the current character session")
		case errors.Is(err, commands.ErrRateLimited):
			respondCommandError(w, http.StatusTooManyRequests, "rate_limited", "command admission rate limit exceeded")
		default:
			respondCommandError(w, http.StatusServiceUnavailable, "command_service_unavailable", "command service unavailable")
		}
		return
	}

	respondJSON(w, http.StatusAccepted, map[string]any{
		"command_id": command.ID,
		"state":      command.State,
		"duplicate":  duplicate,
	})
}

func respondCommandError(w http.ResponseWriter, status int, code, message string) {
	respondJSON(w, status, map[string]string{"error": code, "message": message})
}
