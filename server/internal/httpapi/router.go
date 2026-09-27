package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	agentdomain "phmon/server/internal/agents"
	authdomain "phmon/server/internal/auth"
	"phmon/server/internal/characters"
	"phmon/server/internal/commands"
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
	Auth         *authdomain.Manager
	Agents       AgentStore
	Registry     *agentdomain.Registry
	AgentOptions AgentOptions
	Characters   *characters.Store
	Commands     *commands.Service
	Dispatcher   *commands.Dispatcher
	Live         *LiveHub
}

func New(deps Dependencies) http.Handler {
	mux := http.NewServeMux()
	registerHealth(mux, deps.Database)

	register := func(pattern string, mutation bool, handler http.HandlerFunc) {
		if deps.Auth != nil {
			mux.HandleFunc(pattern, requireOperator(deps.Auth, mutation, handler))
			return
		}
		// Low-level unit tests may omit Auth deliberately. Production wiring cannot:
		// config.Load requires operator configuration and main always injects it.
		mux.HandleFunc(pattern, handler)
	}

	if deps.Auth != nil {
		authHandler := &operatorAuthHandler{manager: deps.Auth}
		mux.HandleFunc("POST /api/auth/login", authHandler.login)
		mux.HandleFunc("GET /api/auth/session", authHandler.session)
		mux.HandleFunc("POST /api/auth/logout", authHandler.logout)
	}
	if deps.Commands != nil {
		commandAPI := &commandHandler{service: deps.Commands, live: deps.Live}
		register("POST /api/commands", true, commandAPI.submit)
	}
	if deps.Agents != nil && deps.Registry != nil {
		live := deps.Live
		if live == nil {
			live = NewLiveHub(deps.Agents, deps.Registry, deps.Characters)
		}
		handler := &agentHandler{
			store:      deps.Agents,
			registry:   deps.Registry,
			options:    deps.AgentOptions.withDefaults(),
			characters: deps.Characters,
			live:       live,
			commands:   deps.Commands,
		}
		mux.HandleFunc("GET /agent", handler.connect)
		register("GET /api/live", true, live.connect)
		register("GET /api/agents", false, handler.list)
		register("POST /api/agents/credentials", true, handler.createCredential)
		if deps.Characters != nil {
			ch := &characterHandler{store: deps.Characters, live: live}
			register("GET /api/characters", false, ch.list)
			register("GET /api/characters/{id}", false, ch.get)
			register("GET /api/groups", false, ch.groups)
			register("POST /api/groups", true, ch.createGroup)
			register("PATCH /api/groups/{id}", true, ch.renameGroup)
			register("DELETE /api/groups/{id}", true, ch.deleteGroup)
			register("PUT /api/groups/{id}/members/{characterID}", true, ch.addMember)
			register("DELETE /api/groups/{id}/members/{characterID}", true, ch.removeMember)
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
