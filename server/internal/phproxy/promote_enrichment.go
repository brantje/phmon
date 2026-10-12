package phproxy

import (
	"context"
	"errors"
	"log/slog"

	"github.com/jackc/pgx/v5"
)

// PromoteEnrichment implements players.EnrichmentPromoter.
func (s *Store) PromoteEnrichment(ctx context.Context, server, observedName string) {
	if s == nil || s.pool == nil {
		return
	}
	sk := serverKey(server)
	nk := nameKey(observedName)
	var playerID string
	err := s.pool.QueryRow(ctx,
		`SELECT id::text FROM players WHERE server_key=$1 AND observed_name_key=$2`,
		sk, nk,
	).Scan(&playerID)
	if errors.Is(err, pgx.ErrNoRows) {
		return
	}
	if err != nil {
		slog.Warn("ph-proxy enrichment promotion lookup failed", "reason", err.Error())
		return
	}
	if err := s.PromoteForIdentity(ctx, playerID, sk, nk); err != nil {
		slog.Warn("ph-proxy enrichment promotion failed", "player_id", playerID, "reason", err.Error())
	}
}
