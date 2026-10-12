package phproxy

import (
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

type ItemPresenter interface {
	ItemPresentation(server string, model *int64, code string) map[string]any
}

type Store struct {
	pool    *pgxpool.Pool
	present ItemPresenter
	now     func() time.Time
}

func NewStore(pool *pgxpool.Pool, present ItemPresenter) *Store {
	return &Store{
		pool:    pool,
		present: present,
		now:     func() time.Time { return time.Now().UTC() },
	}
}

func (s *Store) Submit(ctx context.Context, authenticatedAgentID string, body []byte) (SubmitResponse, error) {
	req, err := ValidateSubmitBody(body, authenticatedAgentID)
	if err != nil {
		return SubmitResponse{}, err
	}
	hash := sha256.Sum256(req.CanonicalJSON)
	now := s.now()

	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return SubmitResponse{}, err
	}
	defer tx.Rollback(ctx)

	lockKey, err := advisoryLockKey(req.AgentID, req.RequestID)
	if err != nil {
		return SubmitResponse{}, ErrInvalid
	}
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock($1)`, lockKey); err != nil {
		return SubmitResponse{}, err
	}

	if _, err := tx.Exec(ctx,
		`DELETE FROM ph_proxy_enrichment_requests WHERE created_at < $1`,
		now.Add(-IdempotencyTTL)); err != nil {
		return SubmitResponse{}, err
	}
	if _, err := tx.Exec(ctx,
		`DELETE FROM player_enrichment_pending WHERE expires_at < $1`,
		now); err != nil {
		return SubmitResponse{}, err
	}

	var storedHash []byte
	var storedResponse []byte
	err = tx.QueryRow(ctx,
		`SELECT body_hash, response_json FROM ph_proxy_enrichment_requests
		 WHERE agent_id=$1::uuid AND request_id=$2::uuid AND created_at >= $3`,
		req.AgentID, req.RequestID, now.Add(-IdempotencyTTL),
	).Scan(&storedHash, &storedResponse)
	if err == nil {
		if len(storedHash) != len(hash) || !bytesEqual(storedHash, hash[:]) {
			return SubmitResponse{}, ErrIdempotencyConflict
		}
		var cached SubmitResponse
		if err := json.Unmarshal(storedResponse, &cached); err != nil {
			return SubmitResponse{}, err
		}
		if err := tx.Commit(ctx); err != nil {
			return SubmitResponse{}, err
		}
		return cached, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return SubmitResponse{}, err
	}

	newPendingNeeded, err := s.countNewPendingSlots(ctx, tx, req)
	if err != nil {
		return SubmitResponse{}, err
	}
	if newPendingNeeded > 0 {
		var pendingCount int
		if err := tx.QueryRow(ctx,
			`SELECT COUNT(*)::int FROM player_enrichment_pending
			 WHERE agent_id=$1::uuid AND expires_at >= $2`,
			req.AgentID, now,
		).Scan(&pendingCount); err != nil {
			return SubmitResponse{}, err
		}
		if pendingCount+newPendingNeeded > MaxPendingPerAgent {
			retryAfter, err := s.pendingRetryAfter(ctx, tx, req.AgentID, now)
			if err != nil {
				return SubmitResponse{}, err
			}
			return SubmitResponse{}, PendingCapacityError{RetryAfter: retryAfter}
		}
	}

	results := make([]PlayerResult, 0, len(req.Players))
	for _, vp := range req.Players {
		playerID, found, err := s.lookupPlayerID(ctx, tx, vp.ServerKey, vp.NameKey)
		if err != nil {
			return SubmitResponse{}, err
		}
		if found {
			out, err := s.applyMatched(ctx, tx, playerID, vp, now)
			if err != nil {
				return SubmitResponse{}, err
			}
			results = append(results, out)
			continue
		}
		out, err := s.applyPending(ctx, tx, req.AgentID, req.Server, vp, now)
		if err != nil {
			return SubmitResponse{}, err
		}
		results = append(results, out)
	}

	if err := s.upsertServerReceipt(ctx, tx, req, now); err != nil {
		return SubmitResponse{}, err
	}

	response := SubmitResponse{RequestID: req.RequestID, Results: results}
	responseJSON, err := json.Marshal(response)
	if err != nil {
		return SubmitResponse{}, err
	}
	if _, err := tx.Exec(ctx,
		`INSERT INTO ph_proxy_enrichment_requests (agent_id, request_id, body_hash, response_json)
		 VALUES ($1::uuid, $2::uuid, $3, $4)`,
		req.AgentID, req.RequestID, hash[:], responseJSON); err != nil {
		return SubmitResponse{}, err
	}
	if err := tx.Commit(ctx); err != nil {
		return SubmitResponse{}, err
	}
	return response, nil
}

type PendingCapacityError struct {
	RetryAfter int
}

func (e PendingCapacityError) Error() string {
	return ErrPendingCapacity.Error()
}

func (s *Store) pendingRetryAfter(ctx context.Context, tx pgx.Tx, agentID string, now time.Time) (int, error) {
	var expires time.Time
	err := tx.QueryRow(ctx,
		`SELECT MIN(expires_at) FROM player_enrichment_pending
		 WHERE agent_id=$1::uuid AND expires_at >= $2`,
		agentID, now,
	).Scan(&expires)
	if errors.Is(err, pgx.ErrNoRows) || err != nil {
		return 60, nil
	}
	seconds := int(expires.Sub(now).Seconds())
	if seconds < 1 {
		return 1, nil
	}
	return seconds, nil
}

func (s *Store) countNewPendingSlots(ctx context.Context, tx pgx.Tx, req validatedRequest) (int, error) {
	now := s.now()
	needed := 0
	for _, vp := range req.Players {
		_, found, err := s.lookupPlayerID(ctx, tx, vp.ServerKey, vp.NameKey)
		if err != nil {
			return 0, err
		}
		if found || !vp.hasStorableFields() {
			continue
		}
		var exists bool
		err = tx.QueryRow(ctx,
			`SELECT EXISTS(
				SELECT 1 FROM player_enrichment_pending
				WHERE agent_id=$1::uuid AND server_key=$2 AND name_key=$3 AND expires_at >= $4)`,
			req.AgentID, vp.ServerKey, vp.NameKey, now,
		).Scan(&exists)
		if err != nil {
			return 0, err
		}
		if !exists {
			needed++
		}
	}
	return needed, nil
}

func (s *Store) lookupPlayerID(ctx context.Context, tx pgx.Tx, serverKey, nameKey string) (string, bool, error) {
	var id string
	err := tx.QueryRow(ctx,
		`SELECT id::text FROM players WHERE server_key=$1 AND observed_name_key=$2`,
		serverKey, nameKey,
	).Scan(&id)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", false, nil
	}
	if err != nil {
		return "", false, err
	}
	return id, true, nil
}

func (s *Store) loadMatchedFields(ctx context.Context, tx pgx.Tx, playerID string) (*storedFields, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+enrichmentSelectColumns+` FROM player_enrichments WHERE player_id=$1::uuid`,
		playerID,
	)
	st, err := loadStoredFieldsFromRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return &storedFields{FieldEvidence: map[string]FieldEvidence{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) saveMatched(ctx context.Context, tx pgx.Tx, playerID string, st *storedFields) error {
	syncProxyFallbacks(st)
	gearJSON, evJSON, extJSON, err := marshalStoredRow(st)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO player_enrichments (
			player_id, model_ref_id, model_ref_observed_at, gear, gear_observed_at, field_evidence,
			extended, proxy_level, proxy_job, proxy_job_level, proxy_guild, proxy_race, updated_at
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, now())
		ON CONFLICT (player_id) DO UPDATE SET
			model_ref_id=EXCLUDED.model_ref_id,
			model_ref_observed_at=EXCLUDED.model_ref_observed_at,
			gear=EXCLUDED.gear,
			gear_observed_at=EXCLUDED.gear_observed_at,
			field_evidence=EXCLUDED.field_evidence,
			extended=EXCLUDED.extended,
			proxy_level=EXCLUDED.proxy_level,
			proxy_job=EXCLUDED.proxy_job,
			proxy_job_level=EXCLUDED.proxy_job_level,
			proxy_guild=EXCLUDED.proxy_guild,
			proxy_race=EXCLUDED.proxy_race,
			updated_at=now()`,
		playerID, st.ModelRefID, st.ModelRefObservedAt, gearJSON, st.GearObservedAt, evJSON,
		extJSON, st.ProxyLevel, st.ProxyJob, st.ProxyJobLevel, st.ProxyGuild, st.ProxyRace,
	)
	return err
}

func (s *Store) applyMatched(ctx context.Context, tx pgx.Tx, playerID string, vp validatedPlayer, now time.Time) (PlayerResult, error) {
	st, err := s.loadMatchedFields(ctx, tx, playerID)
	if err != nil {
		return PlayerResult{}, err
	}
	out, err := mergeValidatedPlayer(st, vp)
	if err != nil {
		return PlayerResult{}, err
	}
	if out.Changed {
		if err := s.saveMatched(ctx, tx, playerID, st); err != nil {
			return PlayerResult{}, err
		}
	}
	return PlayerResult{
		Name:     vp.Name,
		Status:   playerResultStatus(out, true),
		PlayerID: &playerID,
	}, nil
}

func (s *Store) loadPendingFields(ctx context.Context, tx pgx.Tx, agentID string, vp validatedPlayer, now time.Time) (*storedFields, error) {
	row := tx.QueryRow(ctx,
		`SELECT `+enrichmentSelectColumns+`
		 FROM player_enrichment_pending
		 WHERE agent_id=$1::uuid AND server_key=$2 AND name_key=$3 AND expires_at >= $4`,
		agentID, vp.ServerKey, vp.NameKey, now,
	)
	st, err := loadStoredFieldsFromRow(row)
	if errors.Is(err, pgx.ErrNoRows) {
		return &storedFields{FieldEvidence: map[string]FieldEvidence{}}, nil
	}
	if err != nil {
		return nil, err
	}
	return st, nil
}

func (s *Store) savePending(ctx context.Context, tx pgx.Tx, agentID, serverName string, vp validatedPlayer, st *storedFields, expires time.Time) error {
	syncProxyFallbacks(st)
	gearJSON, evJSON, extJSON, err := marshalStoredRow(st)
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx,
		`INSERT INTO player_enrichment_pending (
			agent_id, server_key, name_key, server_name, observed_name,
			model_ref_id, model_ref_observed_at, gear, gear_observed_at, field_evidence,
			extended, proxy_level, proxy_job, proxy_job_level, proxy_guild, proxy_race,
			expires_at, updated_at
		) VALUES ($1::uuid, $2, $3, $4, $5, $6, $7, $8, $9, $10, $11, $12, $13, $14, $15, $16, $17, now())
		ON CONFLICT (agent_id, server_key, name_key) DO UPDATE SET
			server_name=EXCLUDED.server_name,
			observed_name=EXCLUDED.observed_name,
			model_ref_id=EXCLUDED.model_ref_id,
			model_ref_observed_at=EXCLUDED.model_ref_observed_at,
			gear=EXCLUDED.gear,
			gear_observed_at=EXCLUDED.gear_observed_at,
			field_evidence=EXCLUDED.field_evidence,
			extended=EXCLUDED.extended,
			proxy_level=EXCLUDED.proxy_level,
			proxy_job=EXCLUDED.proxy_job,
			proxy_job_level=EXCLUDED.proxy_job_level,
			proxy_guild=EXCLUDED.proxy_guild,
			proxy_race=EXCLUDED.proxy_race,
			expires_at=EXCLUDED.expires_at,
			updated_at=now()`,
		agentID, vp.ServerKey, vp.NameKey, serverName, vp.Name,
		st.ModelRefID, st.ModelRefObservedAt, gearJSON, st.GearObservedAt, evJSON,
		extJSON, st.ProxyLevel, st.ProxyJob, st.ProxyJobLevel, st.ProxyGuild, st.ProxyRace, expires,
	)
	return err
}

func (s *Store) applyPending(ctx context.Context, tx pgx.Tx, agentID, serverName string, vp validatedPlayer, now time.Time) (PlayerResult, error) {
	st, err := s.loadPendingFields(ctx, tx, agentID, vp, now)
	if err != nil {
		return PlayerResult{}, err
	}
	out, err := mergeValidatedPlayer(st, vp)
	if err != nil {
		return PlayerResult{}, err
	}
	if out.Changed {
		expires := now.Add(PendingTTL)
		if err := s.savePending(ctx, tx, agentID, serverName, vp, st, expires); err != nil {
			return PlayerResult{}, err
		}
	}
	return PlayerResult{Name: vp.Name, Status: playerResultStatus(out, false)}, nil
}

func (s *Store) upsertServerReceipt(ctx context.Context, tx pgx.Tx, req validatedRequest, now time.Time) error {
	var gatewayName *string
	var gatewayOnline, gatewayMax *int
	var gatewayObserved *time.Time
	if req.ServerMetadata != nil {
		gatewayName = &req.ServerMetadata.GatewayName
		gatewayOnline = req.ServerMetadata.Online
		gatewayMax = req.ServerMetadata.Max
		gatewayObserved = &req.ServerMetadata.ObservedAt
	}
	var covProfile *string
	var covDropped *int
	var covCapacity *bool
	if req.Coverage != nil {
		covProfile = &req.Coverage.Profile
		covDropped = &req.Coverage.DroppedObservations
		covCapacity = &req.Coverage.CapacityReached
	}
	_, err := tx.Exec(ctx,
		`INSERT INTO ph_proxy_server_status (
			server_key, server_name, gateway_name, gateway_online, gateway_max, gateway_observed_at,
			coverage_profile, coverage_dropped, coverage_capacity_reached, last_receipt_at, updated_at
		) VALUES ($1, $2, $3, $4, $5, $6, $7, $8, $9, $10, now())
		ON CONFLICT (server_key) DO UPDATE SET
			server_name=EXCLUDED.server_name,
			gateway_name=COALESCE(EXCLUDED.gateway_name, ph_proxy_server_status.gateway_name),
			gateway_online=COALESCE(EXCLUDED.gateway_online, ph_proxy_server_status.gateway_online),
			gateway_max=COALESCE(EXCLUDED.gateway_max, ph_proxy_server_status.gateway_max),
			gateway_observed_at=COALESCE(EXCLUDED.gateway_observed_at, ph_proxy_server_status.gateway_observed_at),
			coverage_profile=COALESCE(EXCLUDED.coverage_profile, ph_proxy_server_status.coverage_profile),
			coverage_dropped=COALESCE(EXCLUDED.coverage_dropped, ph_proxy_server_status.coverage_dropped),
			coverage_capacity_reached=COALESCE(EXCLUDED.coverage_capacity_reached, ph_proxy_server_status.coverage_capacity_reached),
			last_receipt_at=EXCLUDED.last_receipt_at,
			updated_at=now()`,
		req.ServerKey, req.Server, gatewayName, gatewayOnline, gatewayMax, gatewayObserved,
		covProfile, covDropped, covCapacity, now,
	)
	return err
}

func bytesEqual(a, b []byte) bool {
	if len(a) != len(b) {
		return false
	}
	for i := range a {
		if a[i] != b[i] {
			return false
		}
	}
	return true
}
