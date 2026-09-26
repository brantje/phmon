package main

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"os"
	"os/signal"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/config"
	"phmon/server/internal/database"
	"phmon/server/internal/httpapi"
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

	// Keep liveness available during database outages; readiness checks the pool.
	srv := &http.Server{
		Addr: cfg.HTTPAddr, Handler: httpapi.New(pool),
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
