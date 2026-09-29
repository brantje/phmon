package mapanalytics

import (
	"context"
	"errors"
	"math"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	agentdomain "phmon/server/internal/agents"
)

const (
	MovementSampleInterval = 2 * time.Second
	MovementDistance       = 4.0
	MaxQueryWindow         = 31 * 24 * time.Hour
	MaxHeatmapPoints       = 2000
)

var (
	ErrInvalidPosition = errors.New("invalid map analytics position")
	ErrStaleSession    = errors.New("map analytics session is not current")
)

type PositionSample struct {
	AgentID, CharacterID, SessionID, DatasetID string
	SampledAt                                  time.Time
	Region                                     int
	X, Y                                       float64
	Z                                          *float64
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func validCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1_000_000 && value <= 1_000_000
}

func validMapRegion(region int) bool {
	return region >= -32768 && region <= 65535 && region != 0
}

func validDatasetID(value string) bool {
	if !strings.HasPrefix(value, "gamedata-") || len(value) < 10 || len(value) > 73 {
		return false
	}
	for _, char := range value[len("gamedata-"):] {
		if !(char >= 'a' && char <= 'z' || char >= '0' && char <= '9') {
			return false
		}
	}
	return true
}

func ValidatePositionSample(sample PositionSample, now time.Time) error {
	if !agentdomain.ValidAgentID(sample.AgentID) || !agentdomain.ValidAgentID(sample.CharacterID) ||
		!agentdomain.ValidAgentID(sample.SessionID) || !validDatasetID(sample.DatasetID) ||
		!validMapRegion(sample.Region) || sample.SampledAt.IsZero() ||
		sample.SampledAt.After(now.Add(5*time.Minute)) || sample.SampledAt.Before(now.Add(-24*time.Hour)) ||
		!validCoordinate(sample.X) || !validCoordinate(sample.Y) || sample.Z != nil && !validCoordinate(*sample.Z) {
		return ErrInvalidPosition
	}
	return nil
}

// RecordPosition persists a bounded movement sample only for the exact active
// character session. Core character-state persistence remains authoritative;
// callers may treat analytics failures as non-fatal after logging them.
func (s *Store) RecordPosition(ctx context.Context, sample PositionSample, now time.Time) (bool, error) {
	if s == nil || s.pool == nil || ValidatePositionSample(sample, now) != nil {
		return false, ErrInvalidPosition
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err := tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, sample.SessionID); err != nil {
		return false, err
	}
	var server string
	err = tx.QueryRow(ctx, `SELECT c.server_name FROM character_sessions cs
		JOIN characters c ON c.character_id=cs.character_id
		WHERE cs.session_id=$1 AND cs.character_id=$2 AND cs.agent_id=$3 AND cs.ended_at IS NULL
		FOR SHARE OF cs`, sample.SessionID, sample.CharacterID, sample.AgentID).Scan(&server)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrStaleSession
	}
	if err != nil {
		return false, err
	}
	var previousAt time.Time
	var previousRegion int
	var previousX, previousY float64
	err = tx.QueryRow(ctx, `SELECT sampled_at,region,x,y FROM character_position_samples
		WHERE session_id=$1 ORDER BY sampled_at DESC LIMIT 1 FOR UPDATE`, sample.SessionID).
		Scan(&previousAt, &previousRegion, &previousX, &previousY)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return false, err
	}
	if err == nil {
		if !sample.SampledAt.After(previousAt) || sample.SampledAt.Sub(previousAt) < MovementSampleInterval {
			if err = tx.Commit(ctx); err != nil {
				return false, err
			}
			return false, nil
		}
		distance := math.Hypot(sample.X-previousX, sample.Y-previousY)
		if sample.Region == previousRegion && distance < MovementDistance {
			if err = tx.Commit(ctx); err != nil {
				return false, err
			}
			return false, nil
		}
	}
	var inserted bool
	err = tx.QueryRow(ctx, `INSERT INTO character_position_samples
		(agent_id,character_id,session_id,server_name,dataset_id,sampled_at,region,x,y,z)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10)
		ON CONFLICT(session_id,sampled_at) DO NOTHING RETURNING true`,
		sample.AgentID, sample.CharacterID, sample.SessionID, server, sample.DatasetID,
		sample.SampledAt.UTC(), sample.Region, sample.X, sample.Y, sample.Z).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		inserted = false
	} else if err != nil {
		return false, err
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return inserted, nil
}
