package commands

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

type Store struct {
	pool *pgxpool.Pool
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
VALUES($1,'queued',jsonb_build_object('session_id',$2::text,'agent_id',$3::text,'connection_generation',$4))`,
		command.ID, command.SessionID, command.AgentID, command.ConnectionGeneration,
	); err != nil {
		return Command{}, false, fmt.Errorf("insert command audit event: %w", err)
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
