package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/characters"
)

type AgentStore interface {
	CreateCredential(context.Context, agentdomain.Credential) error
	AuthenticateToken(context.Context, string) (string, error)
	MarkConnected(context.Context, string, time.Time, int, string, string) error
	MarkSeen(context.Context, string) error
	MarkDisconnected(context.Context, string, time.Time) error
	ListSeen(context.Context) ([]agentdomain.Record, error)
}

type Dependencies struct {
	Database     Pinger
	Agents       AgentStore
	Registry     *agentdomain.Registry
	AgentOptions AgentOptions
	Characters   *characters.Store
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerHealth(mux, deps.Database)
	if deps.Agents != nil && deps.Registry != nil {
		handler := &agentHandler{
			store:      deps.Agents,
			registry:   deps.Registry,
			options:    deps.AgentOptions.withDefaults(),
			characters: deps.Characters,
		}
		mux.HandleFunc("GET /agent", handler.connect)
		mux.HandleFunc("GET /api/agents", handler.list)
		mux.HandleFunc("POST /api/agents/credentials", handler.createCredential)
		if deps.Characters != nil {
			ch := &characterHandler{store: deps.Characters}
			mux.HandleFunc("GET /api/characters", ch.list)
			mux.HandleFunc("GET /api/characters/{id}", ch.get)
			mux.HandleFunc("GET /api/groups", ch.groups)
			mux.HandleFunc("POST /api/groups", ch.createGroup)
			mux.HandleFunc("PATCH /api/groups/{id}", ch.renameGroup)
			mux.HandleFunc("DELETE /api/groups/{id}", ch.deleteGroup)
			mux.HandleFunc("PUT /api/groups/{id}/members/{characterID}", ch.addMember)
			mux.HandleFunc("DELETE /api/groups/{id}/members/{characterID}", ch.removeMember)
			handler.characters = deps.Characters
		}
	}
	return mux
}

func respondJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
