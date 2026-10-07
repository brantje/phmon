package httpapi

import (
	"context"
	"encoding/json"
	"net/http"
	"time"

	agentdomain "phmon/server/internal/agents"
	"phmon/server/internal/analytics"
	authdomain "phmon/server/internal/auth"
	"phmon/server/internal/characters"
	"phmon/server/internal/chat"
	"phmon/server/internal/commands"
	"phmon/server/internal/events"
	"phmon/server/internal/mapanalytics"
	"phmon/server/internal/mobs"
	"phmon/server/internal/navigation"
	"phmon/server/internal/npcs"
	"phmon/server/internal/players"
	"phmon/server/internal/positions"
	"phmon/server/internal/resources"
	"phmon/server/internal/tradenexus"
)

type AgentStore interface {
	CreateCredential(context.Context, agentdomain.Credential) error
	RevokeCredential(context.Context, string) (bool, error)
	AuthenticateToken(context.Context, string) (string, error)
	MarkConnected(context.Context, string, time.Time, int, string, string) error
	MarkSeen(context.Context, string) error
	MarkDisconnected(context.Context, string, time.Time) error
	ListSeen(context.Context) ([]agentdomain.Record, error)
}

type Dependencies struct {
	Database       Pinger
	Auth           *authdomain.Manager
	Agents         AgentStore
	Registry       *agentdomain.Registry
	AgentOptions   AgentOptions
	Characters     *characters.Store
	Commands       *commands.Service
	Dispatcher     *commands.Dispatcher
	Live           *LiveHub
	Resources      *resources.Store
	Events         *events.Store
	Chat           *chat.Store
	Mobs           *mobs.Store
	MobLive        *mobs.LiveStore
	NPCLive        *npcs.LiveStore
	PlayerLive     *players.LiveStore
	Positions      *positions.Store
	MapAnalytics   *mapanalytics.Store
	Analytics      *analytics.Store
	TradeNexus     *tradenexus.Hub
	ThiefSightings *tradenexus.Store
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
	if deps.TradeNexus != nil {
		mux.Handle("GET /tradenexus", deps.TradeNexus)
	}
	if deps.ThiefSightings != nil {
		sightings := &thiefSightingHandler{store: deps.ThiefSightings}
		register("GET /api/thief-sightings", false, sightings.list)
	}
	analyticsLive := deps.Live
	if deps.Agents != nil && deps.Registry != nil {
		live := deps.Live
		if live == nil {
			live = NewLiveHub(deps.Agents, deps.Registry, deps.Characters)
		}
		analyticsLive = live
		live.SetResources(deps.Resources)
		live.SetEvents(deps.Events)
		live.SetAnalytics(deps.Analytics)
		live.SetChat(deps.Chat)
		mobLive := deps.MobLive
		if mobLive == nil {
			mobLive = mobs.NewLiveStore()
		}
		live.SetMobObservations(deps.Mobs)
		live.SetMobLive(mobLive)
		npcLive := deps.NPCLive
		if npcLive == nil {
			npcLive = npcs.NewLiveStore()
		}
		live.SetNPCLive(npcLive)
		playerLive := deps.PlayerLive
		if playerLive == nil {
			playerLive = players.NewLiveStore()
		}
		live.SetPlayerLive(playerLive)
		positionStore := deps.Positions
		if positionStore == nil {
			positionStore = positions.NewStore()
		}
		live.SetPositions(positionStore)
		if live.navigation == nil {
			live.SetNavigation(navigation.NewStore())
		}
		handler := &agentHandler{
			store:      deps.Agents,
			registry:   deps.Registry,
			options:    deps.AgentOptions.withDefaults(),
			characters: deps.Characters,
			live:       live,
			commands:   deps.Commands,
			resources:  deps.Resources,
			events:     deps.Events,
			mobs:       deps.Mobs,
			mobLive:    mobLive,
			npcLive:    npcLive,
			playerLive: playerLive,
			positions:  positionStore,
			analytics:  deps.MapAnalytics,
			navigation: live.navigation,
		}
		mux.HandleFunc("GET /agent", handler.connect)
		live.SetThiefSightings(deps.ThiefSightings)
		register("GET /api/live", true, live.connect)
		if deps.Chat != nil {
			chatAPI := &chatHandler{store: deps.Chat, live: live}
			register("GET /api/chat/contacts", false, chatAPI.contacts)
			register("GET /api/chat/messages", false, chatAPI.messages)
			register("POST /api/chat/read", true, chatAPI.markRead)
			register("GET /api/chat/preferences", false, chatAPI.preferences)
			register("PUT /api/chat/preferences", true, chatAPI.preferences)
		}
		if deps.Events != nil {
			eventAPI := &eventHandler{store: deps.Events, resources: deps.Resources}
			register("GET /api/events", false, eventAPI.list)
		}
		if deps.Resources != nil {
			mapAPI := &mapHandler{resources: deps.Resources, mobs: deps.Mobs, analytics: deps.MapAnalytics}
			register("GET /api/map/profile", false, mapAPI.profile)
			register("GET /api/map/monster-reference/search", false, mapAPI.monsterReferenceSearch)
			register("GET /api/map/monster-reference/overlay", false, mapAPI.monsterReferenceOverlay)
			if deps.Mobs != nil {
				register("GET /api/map/density", false, mapAPI.density)
			}
			if deps.MapAnalytics != nil {
				register("GET /api/map/heatmap", false, mapAPI.heatmap)
				register("GET /api/map/heatmap/facets", false, mapAPI.heatmapFacets)
				register("POST /api/map/heatmap/reset", true, mapAPI.heatmapReset)
			}
		}
		register("GET /api/agents", false, handler.list)
		register("POST /api/agents/credentials", true, handler.createCredential)
		register("DELETE /api/agents/{id}", true, handler.remove)
		if deps.Characters != nil {
			ch := &characterHandler{store: deps.Characters, live: live, resources: deps.Resources}
			register("GET /api/characters", false, ch.list)
			register("GET /api/characters/{id}", false, ch.get)
			register("GET /api/groups", false, ch.groups)
			register("POST /api/groups", true, ch.createGroup)
			register("PATCH /api/groups/{id}", true, ch.renameGroup)
			register("DELETE /api/groups/{id}", true, ch.deleteGroup)
			register("PUT /api/groups/{id}/members/{characterID}", true, ch.addMember)
			register("DELETE /api/groups/{id}/members/{characterID}", true, ch.removeMember)
			handler.characters = deps.Characters
			if deps.Resources != nil {
				resourceAPI := &characterResourceHandler{store: deps.Resources}
				register("GET /api/characters/{id}/resources", false, resourceAPI.get)
			}
		}
	}
	if deps.Resources != nil {
		guildStorageAPI := &guildStorageHandler{store: deps.Resources, live: analyticsLive}
		register("GET /api/guild-storage", false, guildStorageAPI.get)
		register("DELETE /api/guild-storage", true, guildStorageAPI.delete)
	}
	if deps.Analytics != nil {
		analyticsAPI := &analyticsRateResetHandler{store: deps.Analytics, live: analyticsLive}
		register("POST /api/analytics/rate-resets", true, analyticsAPI.reset)
	}
	return mux
}

func respondJSON(w http.ResponseWriter, code int, value any) {
	w.Header().Set("Content-Type", "application/json")
	w.Header().Set("Cache-Control", "no-store")
	w.WriteHeader(code)
	_ = json.NewEncoder(w).Encode(value)
}
