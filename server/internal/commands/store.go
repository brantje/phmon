package commands

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"phmon/server/internal/chat"
)

type Store struct {
	pool *pgxpool.Pool
}

// ClaimDispatch atomically claims an unexpired queued command. The caller must
// still route it to the exact recorded socket and must never retry a failed write.
func (s *Store) ClaimDispatch(ctx context.Context, id string, now time.Time) (Command, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Command{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state State
	var expires time.Time
	err = tx.QueryRow(ctx, `SELECT state,expires_at FROM commands WHERE command_id=$1 FOR UPDATE`, id).Scan(&state, &expires)
	if errors.Is(err, pgx.ErrNoRows) {
		return Command{}, false, ErrNotFound
	}
	if err != nil {
		return Command{}, false, err
	}
	if state != StateQueued {
		return Command{}, false, nil
	}
	if !now.Before(expires) {
		if _, err = tx.Exec(ctx, `UPDATE commands SET state='expired',finished_at=$2,result_code='expired_before_dispatch',result_message='command expired before dispatch' WHERE command_id=$1`, id, now); err != nil {
			return Command{}, false, err
		}
		if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind,evidence) VALUES($1,'expired',jsonb_build_object('phase','before_dispatch'))`, id); err != nil {
			return Command{}, false, err
		}
		if err = tx.Commit(ctx); err != nil {
			return Command{}, false, err
		}
		return Command{}, false, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE commands SET state='dispatching',dispatch_started_at=$2 WHERE command_id=$1`, id, now); err != nil {
		return Command{}, false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind) VALUES($1,'dispatching')`, id); err != nil {
		return Command{}, false, err
	}
	var command Command
	if err = scanCommand(tx.QueryRow(ctx, selectCommand+`WHERE command_id=$1`, id), &command, new([]byte)); err != nil {
		return Command{}, false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return Command{}, false, err
	}
	return command, true, nil
}

func (s *Store) MarkSent(ctx context.Context, id string, at time.Time) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state State
	var sentAt *time.Time
	if err = tx.QueryRow(ctx, `SELECT state,sent_at FROM commands WHERE command_id=$1 FOR UPDATE`, id).Scan(&state, &sentAt); err != nil {
		return err
	}
	if state != StateDispatching && state != StateAcknowledged && state != StateCompleted && state != StateFailed && state != StateUnknown {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `UPDATE commands SET state=CASE WHEN state='dispatching' THEN 'sent' ELSE state END,sent_at=COALESCE(sent_at,$2) WHERE command_id=$1`, id, at); err != nil {
		return err
	}
	if state == StateDispatching {
		if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind) VALUES($1,'sent')`, id); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Store) Acknowledge(ctx context.Context, id, agentID, sessionID string, generation uint64, at time.Time) (bool, error) {
	return s.transition(ctx, id, agentID, sessionID, generation, StateAcknowledged, at, nil)
}

func (s *Store) RecordResult(ctx context.Context, id, agentID, sessionID string, generation uint64, at time.Time, result ResultInput) (bool, error) {
	if result.Status != StateCompleted && result.Status != StateFailed && result.Status != StateUnknown {
		return false, ErrInvalid
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state State
	err = tx.QueryRow(ctx, `SELECT state FROM commands WHERE command_id=$1 AND agent_id=$2 AND session_id=$3 AND connection_generation=$4 FOR UPDATE`, id, agentID, sessionID, generation).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if state == StateCompleted || state == StateFailed || state == StateExpired {
		return false, nil
	}
	if state != StateDispatching && state != StateSent && state != StateAcknowledged && state != StateUnknown {
		return false, nil
	}
	_, err = tx.Exec(ctx, `UPDATE commands SET state=$2,finished_at=$3,result_code=$4,result_message=$5,verification=$6,api_return=NULLIF($7::jsonb,'null'::jsonb),effective_args=NULLIF($8::jsonb,'null'::jsonb),observed_after=NULLIF($9::jsonb,'null'::jsonb) WHERE command_id=$1`, id, result.Status, at, result.Code, result.Message, result.Verification, jsonOrNull(result.APIReturn), jsonOrNull(result.EffectiveArgs), jsonOrNull(result.ObservedAfter))
	if err != nil {
		return false, err
	}
	evidence, _ := json.Marshal(map[string]any{"code": result.Code, "verification": result.Verification, "message": result.Message})
	if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind,evidence) VALUES($1,$2,$3)`, id, result.Status, evidence); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func jsonOrNull(raw []byte) string {
	if len(raw) == 0 {
		return "null"
	}
	return string(raw)
}

func (s *Store) transition(ctx context.Context, id, agentID, sessionID string, generation uint64, next State, at time.Time, evidence any) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var state State
	err = tx.QueryRow(ctx, `SELECT state FROM commands WHERE command_id=$1 AND agent_id=$2 AND session_id=$3 AND connection_generation=$4 FOR UPDATE`, id, agentID, sessionID, generation).Scan(&state)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrNotFound
	}
	if err != nil {
		return false, err
	}
	if state == StateAcknowledged || state == StateCompleted || state == StateFailed || state == StateExpired {
		return false, nil
	}
	if state != StateDispatching && state != StateSent {
		return false, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE commands SET state='acknowledged',acknowledged_at=COALESCE(acknowledged_at,$2) WHERE command_id=$1`, id, at); err != nil {
		return false, err
	}
	if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind,evidence) VALUES($1,'acknowledged','{}'::jsonb)`, id); err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

func (s *Store) ListHistory(ctx context.Context, characterID, name, state string, limit int) ([]Command, error) {
	if limit < 1 || limit > 100 {
		return nil, ErrInvalid
	}
	rows, err := s.pool.Query(ctx, selectCommand+`WHERE character_id=$1 AND ($2='' OR command_name=$2) AND ($3='' OR state=$3) ORDER BY created_at DESC,command_id DESC LIMIT $4`, characterID, name, state, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Command, 0)
	for rows.Next() {
		var c Command
		if err := scanCommand(rows, &c, new([]byte)); err != nil {
			return nil, err
		}
		items = append(items, c)
	}
	return items, rows.Err()
}

// ListByIdempotencyKeys resolves exact submissions for one operator. Unlike
// history, this lookup has no recent-row limit and can recover a submission
// whose HTTP response was lost.
func (s *Store) ListByIdempotencyKeys(ctx context.Context, operatorIdentity string, keys []string) ([]Command, error) {
	rows, err := s.pool.Query(ctx, selectCommand+`WHERE operator_identity=$1 AND idempotency_key=ANY($2::text[]) ORDER BY created_at,command_id`, operatorIdentity, keys)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	items := make([]Command, 0, len(keys))
	for rows.Next() {
		var command Command
		if err := scanCommand(rows, &command, new([]byte)); err != nil {
			return nil, err
		}
		items = append(items, command)
	}
	return items, rows.Err()
}

// CurrentControlTargets takes one set-based snapshot of sessions and their
// current control state. Offline and missing IDs are left for the caller to
// classify against its requested list.
func (s *Store) CurrentControlTargets(ctx context.Context, characterIDs []string) (map[string]TargetControl, error) {
	rows, err := s.pool.Query(ctx, `
SELECT c.character_id::text, cs.session_id::text, cs.agent_id::text,
       cs.connection_generation, c.region,
       COALESCE(cc.training_available,false), cc.training_region, cc.training_zone,
       cc.training_x, cc.training_y, cc.training_z, cc.training_radius, cc.observed_at
FROM characters c
LEFT JOIN character_sessions cs ON cs.character_id=c.character_id AND cs.ended_at IS NULL
LEFT JOIN character_control_state cc ON cc.session_id=cs.session_id
WHERE c.character_id::text=ANY($1::text[])
ORDER BY c.character_id`, characterIDs)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	targets := make(map[string]TargetControl, len(characterIDs))
	for rows.Next() {
		var target TargetControl
		var sessionID, agentID *string
		var generation *uint64
		var training ControlState
		if err := rows.Scan(
			&target.CharacterID, &sessionID, &agentID, &generation, &target.Region,
			&training.TrainingAvailable, &training.TrainingRegion, &training.TrainingZone,
			&training.TrainingX, &training.TrainingY, &training.TrainingZ,
			&training.TrainingRadius, &training.ObservedAt,
		); err != nil {
			return nil, err
		}
		target.Training = &training
		if sessionID != nil {
			target.SessionID = *sessionID
			training.SessionID = *sessionID
		}
		if agentID != nil {
			target.AgentID = *agentID
		}
		if generation != nil {
			target.Generation = *generation
		}
		targets[target.CharacterID] = target
	}
	return targets, rows.Err()
}

func (s *Store) Queued(ctx context.Context, limit int) ([]string, error) {
	rows, err := s.pool.Query(ctx, `SELECT command_id FROM commands WHERE state='queued' ORDER BY created_at,command_id LIMIT $1`, limit)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	ids := make([]string, 0, limit)
	for rows.Next() {
		var id string
		if err := rows.Scan(&id); err != nil {
			return nil, err
		}
		ids = append(ids, id)
	}
	return ids, rows.Err()
}

func (s *Store) CurrentTargetMatches(ctx context.Context, command Command) (bool, error) {
	target, err := s.ResolveTarget(ctx, command.CharacterID)
	if err != nil {
		return false, err
	}
	return target.SessionID == command.SessionID && target.AgentID == command.AgentID && target.Generation == command.ConnectionGeneration, nil
}

func (s *Store) FailBeforeSend(ctx context.Context, id, reason string, at time.Time) error {
	if len(reason) > 64 {
		reason = reason[:64]
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE commands SET state='failed',finished_at=$2,result_code=$3,result_message='command was not sent to the current target' WHERE command_id=$1 AND state='dispatching'`, id, at, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind,evidence) VALUES($1,'failed',jsonb_build_object('reason',$2::text,'phase','before_send'))`, id, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) MarkUnknown(ctx context.Context, id, reason string, at time.Time) error {
	if len(reason) > 64 {
		reason = reason[:64]
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	tag, err := tx.Exec(ctx, `UPDATE commands SET state='unknown',finished_at=$2,result_code=$3,result_message='execution outcome could not be confirmed' WHERE command_id=$1 AND state IN ('dispatching','sent','acknowledged')`, id, at, reason)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return tx.Commit(ctx)
	}
	if _, err = tx.Exec(ctx, `INSERT INTO command_events(command_id,kind,evidence) VALUES($1,'unknown',jsonb_build_object('reason',$2::text))`, id, reason); err != nil {
		return err
	}
	return tx.Commit(ctx)
}

func (s *Store) SaveControlState(ctx context.Context, characterID, sessionID string, state ControlState) error {
	_, err := s.pool.Exec(ctx, `INSERT INTO character_control_state(session_id,character_id,training_available,training_region,training_zone,training_x,training_y,training_z,training_radius,observed_at)
VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,COALESCE($10::timestamptz,now())) ON CONFLICT(session_id) DO UPDATE SET training_available=EXCLUDED.training_available,training_region=EXCLUDED.training_region,training_zone=EXCLUDED.training_zone,training_x=EXCLUDED.training_x,training_y=EXCLUDED.training_y,training_z=EXCLUDED.training_z,training_radius=EXCLUDED.training_radius,observed_at=EXCLUDED.observed_at`, sessionID, characterID, state.TrainingAvailable, state.TrainingRegion, state.TrainingZone, state.TrainingX, state.TrainingY, state.TrainingZ, state.TrainingRadius, state.ObservedAt)
	return err
}

func (s *Store) CurrentControlState(ctx context.Context, characterID string) (*ControlState, error) {
	var state ControlState
	err := s.pool.QueryRow(ctx, `SELECT cs.session_id::text,COALESCE(cc.training_available,false),cc.training_region,cc.training_zone,cc.training_x,cc.training_y,cc.training_z,cc.training_radius,cc.observed_at FROM character_sessions cs LEFT JOIN character_control_state cc ON cc.session_id=cs.session_id WHERE cs.character_id=$1 AND cs.ended_at IS NULL`, characterID).Scan(&state.SessionID, &state.TrainingAvailable, &state.TrainingRegion, &state.TrainingZone, &state.TrainingX, &state.TrainingY, &state.TrainingZ, &state.TrainingRadius, &state.ObservedAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	return &state, nil
}

func (s *Store) RecoverInterrupted(ctx context.Context, now time.Time) error {
	_, err := s.pool.Exec(ctx, `WITH changed AS (UPDATE commands SET state=CASE WHEN state='queued' AND expires_at<=$1 THEN 'expired' WHEN state='queued' THEN 'failed' ELSE 'unknown' END,finished_at=$1,result_code='backend_restart',result_message='backend restarted; command was not replayed' WHERE state IN ('queued','dispatching','sent','acknowledged') RETURNING command_id,state) INSERT INTO command_events(command_id,kind,evidence) SELECT command_id,state,jsonb_build_object('reason','backend_restart_no_replay') FROM changed`, now)
	return err
}

func (s *Store) Reconcile(ctx context.Context, now time.Time, resultWait, walkResultWait time.Duration) (bool, error) {
	tag, err := s.pool.Exec(ctx, `WITH changed AS (
UPDATE commands SET state=CASE WHEN state='queued' THEN 'expired' ELSE 'unknown' END,
 finished_at=$1,result_code=CASE WHEN state='queued' THEN 'expired_before_dispatch' ELSE 'result_timeout' END,
 result_message=CASE WHEN state='queued' THEN 'command expired before dispatch' ELSE 'no authoritative result arrived; execution may have occurred' END
WHERE (state='queued' AND expires_at<=$1) OR (state IN ('dispatching','sent','acknowledged') AND expires_at + make_interval(secs => CASE WHEN command_name='character.walk' THEN $3::double precision ELSE $2::double precision END) <= $1)
RETURNING command_id,state,result_code)
INSERT INTO command_events(command_id,kind,evidence) SELECT command_id,state,jsonb_build_object('reason',result_code) FROM changed`, now, resultWait.Seconds(), walkResultWait.Seconds())
	return tag.RowsAffected() > 0, err
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func (s *Store) ResolveTarget(ctx context.Context, characterID string) (Target, error) {
	var target Target
	var sessionID, agentID *string
	var generation *uint64
	err := s.pool.QueryRow(ctx, `
SELECT c.character_id::text, cs.session_id::text, cs.agent_id::text,
       cs.connection_generation, c.region
FROM characters c
LEFT JOIN character_sessions cs
  ON cs.character_id=c.character_id AND cs.ended_at IS NULL
WHERE c.character_id=$1`, characterID).Scan(
		&target.CharacterID, &sessionID, &agentID, &generation, &target.Region,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Target{}, ErrNotFound
	}
	if err != nil {
		return Target{}, fmt.Errorf("resolve command target: %w", err)
	}
	if sessionID == nil || agentID == nil || generation == nil {
		return Target{}, ErrStaleSession
	}
	target.SessionID = *sessionID
	target.AgentID = *agentID
	target.Generation = *generation
	return target, nil
}

func (s *Store) FindByIdempotency(ctx context.Context, operatorIdentity, key string) (Command, [32]byte, bool, error) {
	var command Command
	var hashBytes []byte
	err := scanCommand(s.pool.QueryRow(ctx, selectCommand+`
WHERE operator_identity=$1 AND idempotency_key=$2`, operatorIdentity, key), &command, &hashBytes)
	if errors.Is(err, pgx.ErrNoRows) {
		return Command{}, [32]byte{}, false, nil
	}
	if err != nil {
		return Command{}, [32]byte{}, false, fmt.Errorf("find idempotent command: %w", err)
	}
	var hash [32]byte
	if len(hashBytes) != len(hash) {
		return Command{}, [32]byte{}, false, errors.New("stored command request hash has invalid length")
	}
	copy(hash[:], hashBytes)
	return command, hash, true, nil
}

func (s *Store) Admit(ctx context.Context, target Target, admission Admission) (Command, bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return Command{}, false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()

	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, target.CharacterID); err != nil {
		return Command{}, false, err
	}
	var current Target
	err = tx.QueryRow(ctx, `
SELECT cs.character_id::text, cs.session_id::text, cs.agent_id::text,
       cs.connection_generation, c.region
FROM character_sessions cs
JOIN characters c ON c.character_id=cs.character_id
WHERE cs.character_id=$1 AND cs.ended_at IS NULL
FOR UPDATE OF cs`, target.CharacterID).Scan(
		&current.CharacterID, &current.SessionID, &current.AgentID, &current.Generation, &current.Region,
	)
	if errors.Is(err, pgx.ErrNoRows) {
		return Command{}, false, ErrStaleSession
	}
	if err != nil {
		return Command{}, false, err
	}
	if current.SessionID != admission.ExpectedSessionID ||
		current.SessionID != target.SessionID ||
		current.AgentID != target.AgentID ||
		current.Generation != target.Generation {
		return Command{}, false, ErrStaleSession
	}

	var command Command
	command.CharacterID = target.CharacterID
	command.SessionID = target.SessionID
	command.AgentID = target.AgentID
	command.ConnectionGeneration = target.Generation
	command.OperatorIdentity = admission.OperatorIdentity
	command.IdempotencyKey = admission.IdempotencyKey
	command.Name = admission.Validated.Name
	command.SchemaVersion = 1
	command.Args = admission.Validated.Args
	command.State = StateQueued
	command.ExpiresAt = admission.ExpiresAt

	err = tx.QueryRow(ctx, `
INSERT INTO commands (
    character_id, session_id, agent_id, connection_generation,
    operator_identity, idempotency_key, request_hash,
    command_name, schema_version, args, state, expires_at
) VALUES ($1,$2,$3,$4,$5,$6,$7,$8,1,$9,'queued',$10)
RETURNING command_id, created_at, expires_at`,
		target.CharacterID, target.SessionID, target.AgentID, target.Generation,
		admission.OperatorIdentity, admission.IdempotencyKey, admission.RequestHash[:],
		admission.Validated.Name, admission.Validated.Args, admission.ExpiresAt,
	).Scan(&command.ID, &command.CreatedAt, &command.ExpiresAt)
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" {
			_ = tx.Rollback(ctx)
			switch pgErr.ConstraintName {
			case "commands_operator_identity_idempotency_key_key", "commands_idempotency_unique":
				existing, requestHash, found, findErr := s.FindByIdempotency(ctx, admission.OperatorIdentity, admission.IdempotencyKey)
				if findErr != nil {
					return Command{}, false, findErr
				}
				if !found {
					return Command{}, false, ErrIdempotencyConflict
				}
				if requestHash != admission.RequestHash {
					return Command{}, false, ErrIdempotencyConflict
				}
				return existing, true, nil
			case "commands_one_inflight_per_character_idx":
				return Command{}, false, ErrInFlight
			}
		}
		return Command{}, false, fmt.Errorf("insert command: %w", err)
	}
	if _, err := tx.Exec(ctx, `
INSERT INTO command_events(command_id,kind,evidence)
VALUES($1,'queued',jsonb_build_object('session_id',$2::text,'agent_id',$3::text,'connection_generation',$4::bigint))`,
		command.ID, command.SessionID, command.AgentID, command.ConnectionGeneration,
	); err != nil {
		return Command{}, false, fmt.Errorf("insert command audit event: %w", err)
	}
	if command.Name == "chat.send" {
		if err := chat.ProjectCommandTx(ctx, tx, command.ID, command.CharacterID, command.SessionID, command.Args, command.CreatedAt); err != nil {
			return Command{}, false, fmt.Errorf("project chat command: %w", err)
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return Command{}, false, err
	}
	return command, false, nil
}

const selectCommand = `SELECT command_id,character_id::text,session_id::text,agent_id::text,
connection_generation,operator_identity,idempotency_key,command_name,schema_version,args,state,
created_at,expires_at,dispatch_started_at,sent_at,acknowledged_at,finished_at,
result_code,result_message,verification,api_return,effective_args,observed_after,request_hash
FROM commands `

func scanCommand(row pgx.Row, command *Command, requestHash *[]byte) error {
	return row.Scan(
		&command.ID,
		&command.CharacterID,
		&command.SessionID,
		&command.AgentID,
		&command.ConnectionGeneration,
		&command.OperatorIdentity,
		&command.IdempotencyKey,
		&command.Name,
		&command.SchemaVersion,
		&command.Args,
		&command.State,
		&command.CreatedAt,
		&command.ExpiresAt,
		&command.DispatchStartedAt,
		&command.SentAt,
		&command.AcknowledgedAt,
		&command.FinishedAt,
		&command.ResultCode,
		&command.ResultMessage,
		&command.Verification,
		&command.APIReturn,
		&command.EffectiveArgs,
		&command.ObservedAfter,
		requestHash,
	)
}
