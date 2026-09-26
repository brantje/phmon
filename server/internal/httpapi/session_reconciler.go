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
func RunSessionReconciler(ctx context.Context, db Pinger, registry *agents.Registry, store *characters.Store, interval time.Duration) {
	if interval <= 0 {
		interval = 3 * time.Second
	}
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
			cancel()
			continue
		}
		err := store.ReconcileInactiveSessions(checkCtx, registry.HasGeneration)
		cancel()
		if err != nil && ctx.Err() == nil {
			slog.Warn("failed to reconcile inactive character sessions")
		}
	}
}
