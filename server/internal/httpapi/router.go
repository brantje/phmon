package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	agentdomain "phmon/server/internal/agents"
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
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerHealth(mux, deps.Database)
	if deps.Agents != nil && deps.Registry != nil {
		handler := &agentHandler{
			store:    deps.Agents,
			registry: deps.Registry,
			options:  deps.AgentOptions.withDefaults(),
		}
		mux.HandleFunc("GET /agent", handler.connect)
		mux.HandleFunc("GET /api/agents", handler.list)
		mux.HandleFunc("POST /api/agents/credentials", handler.createCredential)
	}
	return mux
}

func respondJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
