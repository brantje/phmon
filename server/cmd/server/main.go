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
	"phmon/server/internal/resources"
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
	characterStore := characters.NewStore(pool)
	resourceStore := resources.NewStore(pool)
	eventStore := events.NewStore(pool)
	chatStore := chat.NewStore(pool)
	mobStore := mobs.NewStore(pool)
	mapAnalyticsStore := mapanalytics.NewStore(pool)
	mobLive := mobs.NewLiveStore()
	npcLive := npcs.NewLiveStore()
	playerLive := players.NewLiveStore()
	metadataDir := os.Getenv("ITEM_METADATA_DIR")
	if metadataDir == "" {
		metadataDir = "game-data"
	}
	metadata, metadataErr := resources.LoadItemMetadata(metadataDir)
	if metadataErr != nil {
		return fmt.Errorf("load item metadata: %w", metadataErr)
	}
	resourceStore.SetItemMetadata(metadata)
	reconcileCtx, reconcileCancel := context.WithTimeout(ctx, 5*time.Second)
	if err := characterStore.ReconcileSessions(reconcileCtx); err != nil {
		reconcileCancel()
		return errors.New("cannot reconcile character sessions")
	}
	reconcileCancel()
	registry := agents.NewRegistry()
	commandService := commands.NewService(commands.NewStore(pool), registry)
	live := httpapi.NewLiveHub(store, registry, characterStore)
	live.SetCommands(commandService)
	live.SetResources(resourceStore)
	live.SetEvents(eventStore)
	live.SetChat(chatStore)
	live.SetMobObservations(mobStore)
	live.SetMobLive(mobLive)
	live.SetNPCLive(npcLive)
	live.SetPlayerLive(playerLive)
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
		Database:     pool,
		Auth:         operatorAuth,
		Agents:       store,
		Registry:     registry,
		Characters:   characterStore,
		Commands:     commandService,
		Dispatcher:   dispatcher,
		Live:         live,
		Resources:    resourceStore,
		Events:       eventStore,
		Chat:         chatStore,
		Mobs:         mobStore,
		MobLive:      mobLive,
		NPCLive:      npcLive,
		PlayerLive:   playerLive,
		MapAnalytics: mapAnalyticsStore,
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
