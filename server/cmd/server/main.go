package main

import (
	"context"
	"errors"
	"fmt"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/agents"
	"phmon/server/internal/analytics"
	authdomain "phmon/server/internal/auth"
	"phmon/server/internal/characters"
	"phmon/server/internal/chat"
	"phmon/server/internal/commands"
	"phmon/server/internal/config"
	"phmon/server/internal/database"
	"phmon/server/internal/events"
	"phmon/server/internal/httpapi"
	"phmon/server/internal/mapanalytics"
	"phmon/server/internal/mobs"
	"phmon/server/internal/npcs"
	"phmon/server/internal/players"
	"phmon/server/internal/positions"
	"phmon/server/internal/resources"
	"phmon/server/internal/tradenexus"
)

func main() {
	if err := run(); err != nil {
		slog.Error(err.Error())
		os.Exit(1)
	}
}

func run() error {
	cfg, err := config.Load()
	if err != nil {
		return err
	}
	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()
	poolCfg, err := pgxpool.ParseConfig(cfg.DatabaseURL)
	if err != nil {
		return errors.New("invalid database configuration")
	}
	poolCfg.MaxConns = 5
	poolCfg.ConnConfig.ConnectTimeout = 2 * time.Second
	pool, err := pgxpool.NewWithConfig(ctx, poolCfg)
	if err != nil {
		return errors.New("cannot initialize database pool")
	}
	defer pool.Close()

	migrationCtx, migrationCancel := context.WithTimeout(ctx, 15*time.Second)
	defer migrationCancel()
	if err := database.Migrate(migrationCtx, pool); err != nil {
		return errors.New("database migrations failed")
	}

	operatorAuth, err := authdomain.New(cfg.OperatorAccessSecret, cfg.OperatorSessionCookie, cfg.OperatorAllowedOrigins, cfg.OperatorAllowInsecureHTTP)
	if err != nil {
		return err
	}

	store := agents.NewStore(pool)
	analyticsStore := analytics.NewStore(pool)
	analyticsStore.SetRetentionDays(cfg.AnalyticsRetentionDays)
	characterStore := characters.NewStore(pool)
	resourceStore := resources.NewStore(pool)
	eventStore := events.NewStore(pool)
	chatStore := chat.NewStore(pool)
	mobStore := mobs.NewStore(pool)
	mapAnalyticsStore := mapanalytics.NewStore(pool)
	mobLive := mobs.NewLiveStore()
	npcLive := npcs.NewLiveStore()
	playerLive := players.NewLiveStore()
	playerRegistry := players.NewRegistry(pool)
	positionStore := positions.NewStore()
	metadataDir := os.Getenv("ITEM_METADATA_DIR")
	if metadataDir == "" {
		metadataDir = "game-data"
	}
	metadata, metadataErr := resources.LoadItemMetadata(metadataDir)
	if metadataErr != nil {
		return fmt.Errorf("load item metadata: %w", metadataErr)
	}
	resourceStore.SetItemMetadata(metadata)
	playerRegistry.SetModelNamer(metadata)
	analyticsStore.SetItemTaxonomy(
		resourceStore.AnalyticsItemTaxonomy,
		resourceStore.AnalyticsItemModels,
		resourceStore.AnalyticsTaxonomyOptions,
	)
	analyticsStore.SetItemDetailsResolver(resourceStore.AnalyticsItemDetails)
	eventStore.SetDropClassifier(metadata.ItemDropClassification)
	mobStore.SetLevelLookup(metadata.MonsterLevel)
	reconcileCtx, reconcileCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := characterStore.ReconcileSessions(reconcileCtx); err != nil {
		reconcileCancel()
		return errors.New("cannot reconcile character sessions")
	}
	reconcileCancel()
	registry := agents.NewRegistry()
	commandService := commands.NewService(commands.NewStore(pool), registry)
	commandService.SetReverseReturnContext(resourceStore)
	commandService.SetReverseReturnLocations(metadata)
	live := httpapi.NewLiveHub(store, registry, characterStore)
	go analyticsStore.RunRetention(ctx, cfg.AnalyticsRetentionDays, 15*time.Minute, live.InvalidateAnalytics)
	live.SetCommands(commandService)
	live.SetResources(resourceStore)
	live.SetEvents(eventStore)
	live.SetChat(chatStore)
	live.SetMobObservations(mobStore)
	live.SetMobLive(mobLive)
	live.SetNPCLive(npcLive)
	live.SetPlayerLive(playerLive)
	live.SetPositions(positionStore)
	var tradeHub *tradenexus.Hub
	var thiefSightings *tradenexus.Store
	if cfg.TradeNexusEnabled {
		thiefSightings = tradenexus.NewStore(pool)
		tradeHub = tradenexus.NewHub(tradenexus.Options{
			Store: thiefSightings, Trades: thiefSightings, Invalidate: live.Invalidate,
			PlayerRecorder: func(recordCtx context.Context, sighting tradenexus.Sighting) {
				job := "thief"
				obs := players.Observation{
					Server: sighting.Server, ObservedName: sighting.ThiefName, Job: &job,
					Source: players.SourceThiefSighting, ObservedAt: sighting.ObservedAt,
				}
				if sighting.Position != nil {
					region := sighting.Position.Region
					x := sighting.Position.X
					y := sighting.Position.Y
					obs.Region = &region
					obs.X = &x
					obs.Y = &y
					obs.Z = sighting.Position.Z
				}
				if err := playerRegistry.Apply(recordCtx, []players.Observation{obs}); err != nil {
					slog.Warn("player registry thief sighting was not stored", "reason", err.Error())
				}
			},
		})
		live.SetThiefSightings(thiefSightings)
		go thiefSightings.RunRetention(ctx, cfg.TradeNexusRetentionDays, time.Hour)
		eventStore.SetAcceptedHook(func(event events.Event) {
			sighting, ok := tradenexus.SightingFromEvent(event)
			if !ok {
				return
			}
			hookCtx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
			defer cancel()
			recorded, fresh, err := tradeHub.Record(hookCtx, sighting)
			if err != nil {
				slog.Warn("phmon thief sighting was not relayed", "reason", err.Error())
				return
			}
			if fresh {
				tradeHub.Broadcast(recorded)
			}
		})
	}
	dispatchStore := commands.NewStore(pool)
	if err := dispatchStore.RecoverInterrupted(ctx, time.Now().UTC()); err != nil {
		return errors.New("cannot recover interrupted commands")
	}
	dispatcher := commands.NewDispatcher(dispatchStore, registry, live)
	commandService.SetDispatcher(dispatcher)
	commandService.SetNavigationAdmission(live.NavigationAdmission())
	go dispatcher.Run(ctx)
	go httpapi.RunSessionReconciler(ctx, pool, registry, characterStore, live, 3*time.Second)
	handler := httpapi.New(httpapi.Dependencies{
		Database:       pool,
		Auth:           operatorAuth,
		Agents:         store,
		Registry:       registry,
		Characters:     characterStore,
		Commands:       commandService,
		Dispatcher:     dispatcher,
		Live:           live,
		Resources:      resourceStore,
		Events:         eventStore,
		Chat:           chatStore,
		Mobs:           mobStore,
		MobLive:        mobLive,
		NPCLive:        npcLive,
		PlayerLive:     playerLive,
		PlayerRegistry: playerRegistry,
		Positions:      positionStore,
		MapAnalytics:   mapAnalyticsStore,
		Analytics:      analyticsStore,
		TradeNexus:     tradeHub,
		ThiefSightings: thiefSightings,
	})

	// Keep liveness available during database outages; readiness checks the pool.
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: handler,
		ReadHeaderTimeout: 5 * time.Second, ReadTimeout: 10 * time.Second,
		WriteTimeout: 10 * time.Second, IdleTimeout: 60 * time.Second,
	}
	done := make(chan error, 1)
	go func() { done <- srv.ListenAndServe() }()
	slog.Info("HTTP server starting", "address", cfg.HTTPAddr)
	select {
	case err := <-done:
		if errors.Is(err, http.ErrServerClosed) {
			return nil
		}
		return errors.New("HTTP server failed to listen or serve")
	case <-ctx.Done():
		shutdownCtx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		defer cancel()
		if err := srv.Shutdown(shutdownCtx); err != nil {
			_ = srv.Close()
			return errors.New("HTTP server shutdown timed out")
		}
		return nil
	}
}
