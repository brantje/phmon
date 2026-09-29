package agents

import (
	"context"
	"crypto/rand"
	"crypto/sha256"
	"encoding/base64"
	"encoding/hex"
	"errors"
	"fmt"
	"regexp"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var (
	ErrInvalidToken = errors.New("invalid agent token")
	uuidPattern     = regexp.MustCompile("^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$")
)

type Credential struct {
	AgentID string
	Token   string
}

type Record struct {
	AgentID            string
	CreatedAt          time.Time
	FirstSeenAt        *time.Time
	LastSeenAt         *time.Time
	LastConnectedAt    *time.Time
	LastDisconnectedAt *time.Time
	ProtocolVersion    *int
	PluginVersion      *string
	PhBotVersion       *string
}

type Store struct {
	pool *pgxpool.Pool
}

func NewStore(pool *pgxpool.Pool) *Store {
	return &Store{pool: pool}
}

func NewCredential() (Credential, error) {
	uuidBytes := make([]byte, 16)
	if _, err := rand.Read(uuidBytes); err != nil {
		return Credential{}, fmt.Errorf("generate agent id: %w", err)
	}
	uuidBytes[6] = (uuidBytes[6] & 0x0f) | 0x40
	uuidBytes[8] = (uuidBytes[8] & 0x3f) | 0x80
	encoded := hex.EncodeToString(uuidBytes)
	agentID := encoded[0:8] + "-" + encoded[8:12] + "-" + encoded[12:16] + "-" + encoded[16:20] + "-" + encoded[20:32]

	tokenBytes := make([]byte, 32)
	if _, err := rand.Read(tokenBytes); err != nil {
		return Credential{}, fmt.Errorf("generate agent token: %w", err)
	}
	return Credential{
		AgentID: agentID,
		Token:   "phm_" + base64.RawURLEncoding.EncodeToString(tokenBytes),
	}, nil
}

func ValidAgentID(agentID string) bool {
	return uuidPattern.MatchString(agentID)
}

func HashToken(token string) [32]byte {
	return sha256.Sum256([]byte(token))
}

func (s *Store) CreateCredential(ctx context.Context, credential Credential) error {
	if !ValidAgentID(credential.AgentID) || credential.Token == "" {
		return errors.New("invalid generated credential")
	}
	hash := HashToken(credential.Token)
	_, err := s.pool.Exec(ctx, "INSERT INTO agents (agent_id, token_hash) VALUES ($1, $2)", credential.AgentID, hash[:])
	if err != nil {
		return fmt.Errorf("store agent credential: %w", err)
	}
	return nil
}

func (s *Store) AuthenticateToken(ctx context.Context, token string) (string, error) {
	if token == "" || len(token) > 256 {
		return "", ErrInvalidToken
	}
	hash := HashToken(token)
	var agentID string
	err := s.pool.QueryRow(ctx, "SELECT agent_id::text FROM agents WHERE token_hash = $1 AND revoked_at IS NULL", hash[:]).Scan(&agentID)
	if errors.Is(err, pgx.ErrNoRows) {
		return "", ErrInvalidToken
	}
	if err != nil {
		return "", fmt.Errorf("authenticate agent: %w", err)
	}
	return agentID, nil
}

func (s *Store) RevokeCredential(ctx context.Context, agentID string) (bool, error) {
	if !ValidAgentID(agentID) {
		return false, nil
	}
	tag, err := s.pool.Exec(ctx, "UPDATE agents SET revoked_at = now() WHERE agent_id = $1 AND revoked_at IS NULL", agentID)
	if err != nil {
		return false, fmt.Errorf("revoke agent credential: %w", err)
	}
	return tag.RowsAffected() == 1, nil
}

func (s *Store) MarkConnected(ctx context.Context, agentID string, connectedAt time.Time, protocolVersion int, pluginVersion, phBotVersion string) error {
	_, err := s.pool.Exec(ctx, `UPDATE agents
SET first_seen_at = COALESCE(first_seen_at, $2),
    last_seen_at = GREATEST(COALESCE(last_seen_at, $2), $2),
    last_connected_at = GREATEST(COALESCE(last_connected_at, $2), $2),
    protocol_version = $3,
    plugin_version = $4,
    phbot_version = $5
WHERE agent_id = $1`, agentID, connectedAt, protocolVersion, pluginVersion, phBotVersion)
	if err != nil {
		return fmt.Errorf("mark agent connected: %w", err)
	}
	return nil
}

func (s *Store) MarkSeen(ctx context.Context, agentID string) error {
	_, err := s.pool.Exec(ctx, "UPDATE agents SET last_seen_at = now() WHERE agent_id = $1", agentID)
	if err != nil {
		return fmt.Errorf("mark agent seen: %w", err)
	}
	return nil
}

func (s *Store) MarkDisconnected(ctx context.Context, agentID string, connectedAt time.Time) error {
	_, err := s.pool.Exec(ctx, `UPDATE agents
SET last_seen_at = now(),
    last_disconnected_at = now()
WHERE agent_id = $1
  AND (last_connected_at IS NULL OR last_connected_at <= $2)`, agentID, connectedAt)
	if err != nil {
		return fmt.Errorf("mark agent disconnected: %w", err)
	}
	return nil
}

func (s *Store) ListSeen(ctx context.Context) ([]Record, error) {
	rows, err := s.pool.Query(ctx, `SELECT agent_id::text, created_at, first_seen_at, last_seen_at,
       last_connected_at, last_disconnected_at, protocol_version, plugin_version, phbot_version
FROM agents
WHERE revoked_at IS NULL
ORDER BY last_seen_at DESC NULLS LAST, created_at DESC, agent_id`)
	if err != nil {
		return nil, fmt.Errorf("list agents: %w", err)
	}
	defer rows.Close()

	var records []Record
	for rows.Next() {
		var record Record
		if err := rows.Scan(
			&record.AgentID,
			&record.CreatedAt,
			&record.FirstSeenAt,
			&record.LastSeenAt,
			&record.LastConnectedAt,
			&record.LastDisconnectedAt,
			&record.ProtocolVersion,
			&record.PluginVersion,
			&record.PhBotVersion,
		); err != nil {
			return nil, fmt.Errorf("scan agent: %w", err)
		}
		records = append(records, record)
	}
	if err := rows.Err(); err != nil {
		return nil, fmt.Errorf("iterate agents: %w", err)
	}
	return records, nil
}
