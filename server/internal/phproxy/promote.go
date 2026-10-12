package phproxy

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
)

// PromoteForIdentity merges any non-expired pending enrichment rows for the server/name
// into the matched player record.
func (s *Store) PromoteForIdentity(ctx context.Context, playerID, serverKey, nameKey string) error {
	if serverKey == "" || nameKey == "" {
		return nil
	}
	now := s.now()
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer tx.Rollback(ctx)

	st, err := s.loadMatchedFields(ctx, tx, playerID)
	if err != nil {
		return err
	}
	rows, err := tx.Query(ctx,
		`SELECT `+enrichmentSelectColumns+`
		 FROM player_enrichment_pending
		 WHERE server_key=$1 AND name_key=$2 AND expires_at >= $3`,
		serverKey, nameKey, now,
	)
	if err != nil {
		return err
	}
	defer rows.Close()

	changed := false
	for rows.Next() {
		pending, err := loadStoredFieldsFromRow(rows)
		if err != nil {
			return err
		}
		if mergeStoredInto(st, pending) {
			changed = true
		}
	}
	if err := rows.Err(); err != nil {
		return err
	}
	if changed {
		if err := s.saveMatched(ctx, tx, playerID, st); err != nil {
			return err
		}
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM player_enrichment_pending WHERE server_key=$1 AND name_key=$2`,
		serverKey, nameKey,
	); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) LoadEnrichmentWithPromotion(ctx context.Context, playerID, server, serverKey, nameKey string) (*EnrichmentProjection, error) {
	if serverKey != "" && nameKey != "" {
		if err := s.PromoteForIdentity(ctx, playerID, serverKey, nameKey); err != nil {
			return nil, err
		}
	}
	row := s.pool.QueryRow(ctx,
		`SELECT `+enrichmentSelectColumns+` FROM player_enrichments WHERE player_id=$1::uuid`,
		playerID,
	)
	st, err := loadStoredFieldsFromRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return s.projectEnrichment(server, st), nil
}

func (s *Store) ServerStatus(ctx context.Context, server string) (StatusResponse, error) {
	sk := serverKey(server)
	if !validServerName(server) {
		return StatusResponse{}, ErrInvalid
	}
	var gatewayName *string
	var gatewayOnline, gatewayMax *int
	var gatewayObserved, lastReceipt *time.Time
	var covProfile *string
	var covDropped *int
	var covCapacity *bool
	var storedName string
	err := s.pool.QueryRow(ctx,
		`SELECT server_name, gateway_name, gateway_online, gateway_max, gateway_observed_at,
			coverage_profile, coverage_dropped, coverage_capacity_reached, last_receipt_at
		 FROM ph_proxy_server_status WHERE server_key=$1`,
		sk,
	).Scan(&storedName, &gatewayName, &gatewayOnline, &gatewayMax, &gatewayObserved,
		&covProfile, &covDropped, &covCapacity, &lastReceipt)
	if errors.Is(err, pgx.ErrNoRows) {
		return StatusResponse{Server: strings.TrimSpace(server)}, nil
	}
	if err != nil {
		return StatusResponse{}, err
	}
	resp := StatusResponse{
		Server:            storedName,
		GatewayName:       gatewayName,
		GatewayOnline:     gatewayOnline,
		GatewayMax:        gatewayMax,
		GatewayObservedAt: gatewayObserved,
		LastReceiptAt:     lastReceipt,
	}
	if covProfile != nil {
		resp.Coverage = &CoverageStatus{
			Profile:             *covProfile,
			DroppedObservations: valueOrZero(covDropped),
			CapacityReached:     valueOrFalse(covCapacity),
		}
	}
	return resp, nil
}

func valueOrZero(v *int) int {
	if v == nil {
		return 0
	}
	return *v
}

func valueOrFalse(v *bool) bool {
	if v == nil {
		return false
	}
	return *v
}
