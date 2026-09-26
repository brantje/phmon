package httpapi

import (
	"context"
	"log/slog"
	"time"

	"phmon/server/internal/agents"
	"phmon/server/internal/characters"
)

// RunSessionReconciler periodically repairs durable presence from the live
// connection registry. This covers disconnect cleanup that could not reach
// PostgreSQL during an outage without restarting the server. It may be stopped
// by canceling ctx and never blocks shutdown on a database operation longer
// than its bounded context.
func RunSessionReconciler(ctx context.Context, db Pinger, registry *agents.Registry, store *characters.Store, live *LiveHub, interval time.Duration) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
	databaseUnavailable := false
	ticker := time.NewTicker(interval)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
		}
		checkCtx, cancel := context.WithTimeout(ctx, 2*time.Second)
		if err := db.Ping(checkCtx); err != nil {
			if !databaseUnavailable {
				live.Invalidate()
			}
			databaseUnavailable = true
			cancel()
			continue
		}
		changed, err := store.ReconcileInactiveSessionsChanged(checkCtx, registry.HasGeneration)
		cancel()
		if err != nil {
			if !databaseUnavailable {
				live.Invalidate()
			}
			databaseUnavailable = true
			if ctx.Err() == nil {
				slog.Warn("failed to reconcile inactive character sessions")
			}
			continue
		}
		if changed || databaseUnavailable {
			live.Invalidate()
		}
		databaseUnavailable = false
	}
}
