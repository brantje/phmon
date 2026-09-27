package characters

import (
	"context"
	"errors"
	"fmt"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
)

var ErrNotFound = errors.New("character not found")

type Identity struct {
	Server, Name string
	// Guild nil means unavailable; a pointer to an empty string means the
	// plugin observed that the character currently has no guild.
	Guild *string
}
type State struct {
	Level      *int     `json:"level"`
	HP         *int64   `json:"hp"`
	HPMax      *int64   `json:"hp_max"`
	MP         *int64   `json:"mp"`
	MPMax      *int64   `json:"mp_max"`
	CurrentEXP *int64   `json:"current_exp"`
	MaxEXP     *int64   `json:"max_exp"`
	SP         *int64   `json:"sp"`
	Gold       *int64   `json:"gold"`
	Region     *int     `json:"region"`
	Zone       *string  `json:"zone"`
	X          *float64 `json:"x"`
	Y          *float64 `json:"y"`
	Z          *float64 `json:"z"`
	Botting    *bool    `json:"botting"`
}
type Character struct {
	ID               string     `json:"character_id"`
	Server           string     `json:"server"`
	Name             string     `json:"name"`
	Guild            *string    `json:"guild,omitempty"`
	Zone             *string    `json:"zone,omitempty"`
	Online           bool       `json:"online"`
	SessionID        *string    `json:"session_id,omitempty"`
	AgentID          *string    `json:"agent_id,omitempty"`
	SessionStartedAt *time.Time `json:"session_started_at,omitempty"`
	LastActivityAt   *time.Time `json:"last_activity_at,omitempty"`
	StateUpdatedAt   *time.Time `json:"state_updated_at,omitempty"`
	Level            *int       `json:"level,omitempty"`
	HP               *int64     `json:"hp,omitempty"`
	HPMax            *int64     `json:"hp_max,omitempty"`
	MP               *int64     `json:"mp,omitempty"`
	MPMax            *int64     `json:"mp_max,omitempty"`
	CurrentEXP       *int64     `json:"current_exp,omitempty"`
	MaxEXP           *int64     `json:"max_exp,omitempty"`
	SP               *int64     `json:"sp,omitempty"`
	Gold             *int64     `json:"gold,omitempty"`
	Region           *int       `json:"region,omitempty"`
	X                *float64   `json:"x,omitempty"`
	Y                *float64   `json:"y,omitempty"`
	Z                *float64   `json:"z,omitempty"`
	Botting          *bool      `json:"botting,omitempty"`
}
type Group struct {
	ID      string      `json:"group_id"`
	Name    string      `json:"name"`
	Members []Character `json:"members"`
}
type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

// Resolve uses server-scoped, case-folded character names. Silkroad names are
// assumed unique within a server; no undocumented player/account ID is trusted.
func (s *Store) Resolve(ctx context.Context, identity Identity) (string, error) {
	server, name := strings.TrimSpace(identity.Server), strings.TrimSpace(identity.Name)
	if len(server) < 1 || len(server) > 100 || len(name) < 1 || len(name) > 64 || strings.ContainsRune(server, 0) || strings.ContainsRune(name, 0) {
		return "", errors.New("invalid character identity")
	}
	key, serverKey := strings.ToLower(name), strings.ToLower(server)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1 || ':' || $2, 0))`, serverKey, key); err != nil {
		return "", err
	}
	var id string
	var guild any
	guildObserved := identity.Guild != nil
	if identity.Guild != nil {
		guild = strings.TrimSpace(*identity.Guild)
		if guild == "" {
			guild = nil
		}
	}
	err = tx.QueryRow(ctx, `INSERT INTO characters(server_name,server_key,character_name,identity_key,guild_name)
	VALUES($1,$2,$3,$4,$5) ON CONFLICT(server_key,identity_key) DO UPDATE SET server_name=EXCLUDED.server_name,character_name=EXCLUDED.character_name,guild_name=CASE WHEN $6 THEN EXCLUDED.guild_name ELSE characters.guild_name END,updated_at=now() RETURNING character_id::text`, server, serverKey, name, key, guild, guildObserved).Scan(&id)
	if err != nil {
		return "", fmt.Errorf("resolve character: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return id, nil
}

// ClaimSession is called only after this connection explicitly identifies a
// character. It transfers that character's live authority to this generation;
// other characters observed by the same agent remain independent.
func (s *Store) ClaimSession(ctx context.Context, agentID, characterID string, generation uint64) error {
	_, err := s.ClaimSessionID(ctx, agentID, characterID, generation)
	return err
}

func (s *Store) ClaimSessionID(ctx context.Context, agentID, characterID string, generation uint64) (string, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return "", err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, characterID); err != nil {
		return "", err
	}
	var currentSession, currentAgent string
	var currentGeneration uint64
	err = tx.QueryRow(ctx, `SELECT session_id::text,agent_id::text,connection_generation FROM character_sessions WHERE character_id=$1 AND ended_at IS NULL FOR UPDATE`, characterID).Scan(&currentSession, &currentAgent, &currentGeneration)
	if err != nil && !errors.Is(err, pgx.ErrNoRows) {
		return "", err
	}
	if err == nil && currentAgent == agentID && currentGeneration == generation {
		if _, err = tx.Exec(ctx, `UPDATE character_sessions SET last_activity_at=now() WHERE session_id=$1`, currentSession); err != nil {
			return "", err
		}
		if err = tx.Commit(ctx); err != nil {
			return "", err
		}
		return currentSession, nil
	}
	if _, err = tx.Exec(ctx, `UPDATE character_sessions SET ended_at=now(),last_activity_at=now(),end_reason='switched' WHERE character_id=$1 AND ended_at IS NULL`, characterID); err != nil {
		return "", err
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET level=NULL,hp=NULL,hp_max=NULL,mp=NULL,mp_max=NULL,current_exp=NULL,max_exp=NULL,sp=NULL,gold=NULL,region=NULL,zone_name=NULL,x=NULL,y=NULL,z=NULL,botting=NULL,state_updated_at=NULL,updated_at=now() WHERE character_id=$1`, characterID); err != nil {
		return "", err
	}
	var sessionID string
	if err = tx.QueryRow(ctx, `INSERT INTO character_sessions(character_id,agent_id,connection_generation) VALUES($1,$2,$3) RETURNING session_id::text`, characterID, agentID, generation).Scan(&sessionID); err != nil {
		return "", fmt.Errorf("open character session: %w", err)
	}
	if err = tx.Commit(ctx); err != nil {
		return "", err
	}
	return sessionID, nil
}

func (s *Store) Snapshot(ctx context.Context, agentID, characterID string, generation uint64, state State) error {
	return s.SnapshotSession(ctx, agentID, characterID, generation, "", state)
}

func (s *Store) SnapshotSession(ctx context.Context, agentID, characterID string, generation uint64, expectedSessionID string, state State) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, characterID); err != nil {
		return err
	}
	var sessionID string
	err = tx.QueryRow(ctx, `SELECT session_id::text FROM character_sessions WHERE character_id=$1 AND agent_id=$2 AND connection_generation=$3 AND ($4='' OR session_id=$4::uuid) AND ended_at IS NULL FOR UPDATE`, characterID, agentID, generation, expectedSessionID).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE characters SET level=$2,hp=$3,hp_max=$4,mp=$5,mp_max=$6,current_exp=$7,max_exp=$8,sp=$9,gold=$10,region=$11,zone_name=$12,x=$13,y=$14,z=$15,botting=$16,state_updated_at=now(),updated_at=now() WHERE character_id=$1`, characterID, state.Level, state.HP, state.HPMax, state.MP, state.MPMax, state.CurrentEXP, state.MaxEXP, state.SP, state.Gold, state.Region, state.Zone, state.X, state.Y, state.Z, state.Botting); err != nil {
		return err
	}
	if _, err = tx.Exec(ctx, `UPDATE character_sessions SET last_activity_at=now() WHERE character_id=$1 AND agent_id=$2 AND connection_generation=$3 AND ended_at IS NULL`, characterID, agentID, generation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) Update(ctx context.Context, agentID, characterID string, generation uint64, state State) error {
	return s.UpdateSession(ctx, agentID, characterID, generation, "", state)
}

func (s *Store) UpdateSession(ctx context.Context, agentID, characterID string, generation uint64, expectedSessionID string, state State) error {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, characterID); err != nil {
		return err
	}
	var sessionID string
	err = tx.QueryRow(ctx, `SELECT session_id::text FROM character_sessions WHERE character_id=$1 AND agent_id=$2 AND connection_generation=$3 AND ($4='' OR session_id=$4::uuid) AND ended_at IS NULL FOR UPDATE`, characterID, agentID, generation, expectedSessionID).Scan(&sessionID)
	if errors.Is(err, pgx.ErrNoRows) {
		return ErrNotFound
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, `UPDATE characters c SET
level=$2,hp=$3,hp_max=$4,mp=$5,mp_max=$6,current_exp=$7,max_exp=$8,sp=$9,gold=$10,region=$11,zone_name=$12,
x=$13,y=$14,z=$15,botting=$16,state_updated_at=now(),updated_at=now()
WHERE c.character_id=$1`, characterID, state.Level, state.HP, state.HPMax, state.MP, state.MPMax, state.CurrentEXP, state.MaxEXP, state.SP, state.Gold, state.Region, state.Zone, state.X, state.Y, state.Z, state.Botting)
	if err != nil {
		return fmt.Errorf("update character state: %w", err)
	}
	if _, err = tx.Exec(ctx, `UPDATE character_sessions SET last_activity_at=now() WHERE character_id=$1 AND agent_id=$2 AND connection_generation=$3 AND ended_at IS NULL`, characterID, agentID, generation); err != nil {
		return err
	}
	return tx.Commit(ctx)
}
func (s *Store) End(ctx context.Context, agentID, characterID string, generation uint64, reason string) error {
	return s.EndSession(ctx, agentID, characterID, generation, "", reason)
}

func (s *Store) EndSession(ctx context.Context, agentID, characterID string, generation uint64, expectedSessionID, reason string) error {
	if reason != "left" && reason != "agent_disconnected" {
		return errors.New("invalid session end reason")
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	if _, err = tx.Exec(ctx, `SELECT pg_advisory_xact_lock(hashtextextended($1, 0))`, characterID); err != nil {
		return err
	}
	tag, err := tx.Exec(ctx, `UPDATE character_sessions SET ended_at=now(),last_activity_at=now(),end_reason=$4 WHERE character_id=$1 AND agent_id=$2 AND connection_generation=$3 AND ($5='' OR session_id=$5::uuid) AND ended_at IS NULL`, characterID, agentID, generation, reason, expectedSessionID)
	if err != nil {
		return err
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return tx.Commit(ctx)
}
func (s *Store) EndAgent(ctx context.Context, agentID string, generation uint64) error {
	_, err := s.pool.Exec(ctx, `UPDATE character_sessions SET ended_at=now(),last_activity_at=now(),end_reason='agent_disconnected' WHERE agent_id=$1 AND connection_generation=$2 AND ended_at IS NULL`, agentID, generation)
	return err
}

func (s *Store) ReconcileSessions(ctx context.Context) error {
	_, err := s.pool.Exec(ctx, `UPDATE character_sessions SET ended_at=now(),last_activity_at=now(),end_reason='backend_restart' WHERE ended_at IS NULL`)
	return err
}

// ReconcileInactiveSessions closes durable sessions whose exact socket
// generation is no longer present in the in-memory registry. It is safe to run
// repeatedly and deliberately leaves sibling generations of the same agent
// untouched.
func (s *Store) ReconcileInactiveSessions(ctx context.Context, generationIsActive func(agentID string, generation uint64) bool) error {
	_, err := s.ReconcileInactiveSessionsChanged(ctx, generationIsActive)
	return err
}

// ReconcileInactiveSessionsChanged reports whether durable live presence changed.
// The browser live-data reconciler uses this to publish a replacement snapshot after
// recovery without turning reconciliation into a polling read path.
func (s *Store) ReconcileInactiveSessionsChanged(ctx context.Context, generationIsActive func(agentID string, generation uint64) bool) (bool, error) {
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	rows, err := tx.Query(ctx, `SELECT session_id::text,agent_id::text,connection_generation FROM character_sessions WHERE ended_at IS NULL FOR UPDATE`)
	if err != nil {
		return false, err
	}
	type sessionOwner struct {
		sessionID, agentID string
		generation         uint64
	}
	var stale []sessionOwner
	for rows.Next() {
		var owner sessionOwner
		if err := rows.Scan(&owner.sessionID, &owner.agentID, &owner.generation); err != nil {
			rows.Close()
			return false, err
		}
		if !generationIsActive(owner.agentID, owner.generation) {
			stale = append(stale, owner)
		}
	}
	if err := rows.Err(); err != nil {
		rows.Close()
		return false, err
	}
	rows.Close()
	for _, owner := range stale {
		if _, err := tx.Exec(ctx, `UPDATE character_sessions SET ended_at=now(),last_activity_at=now(),end_reason='agent_disconnected' WHERE session_id=$1 AND ended_at IS NULL`, owner.sessionID); err != nil {
			return false, err
		}
	}
	if err := tx.Commit(ctx); err != nil {
		return false, err
	}
	return len(stale) > 0, nil
}

const selectCharacters = `SELECT c.character_id::text,c.server_name,c.character_name,c.guild_name,c.zone_name,
(cs.session_id IS NOT NULL),cs.session_id::text,cs.agent_id::text,cs.started_at,cs.last_activity_at,c.state_updated_at,c.level,c.hp,c.hp_max,c.mp,c.mp_max,c.current_exp,c.max_exp,c.sp,c.gold,c.region,c.x,c.y,c.z,c.botting
FROM characters c LEFT JOIN character_sessions cs ON cs.character_id=c.character_id AND cs.ended_at IS NULL`

func scanCharacter(row pgx.Row) (Character, error) {
	var c Character
	err := row.Scan(&c.ID, &c.Server, &c.Name, &c.Guild, &c.Zone, &c.Online, &c.SessionID, &c.AgentID, &c.SessionStartedAt, &c.LastActivityAt, &c.StateUpdatedAt, &c.Level, &c.HP, &c.HPMax, &c.MP, &c.MPMax, &c.CurrentEXP, &c.MaxEXP, &c.SP, &c.Gold, &c.Region, &c.X, &c.Y, &c.Z, &c.Botting)
	return c, err
}
func (s *Store) List(ctx context.Context, query string, groupID string) ([]Character, error) {
	q := selectCharacters + ` WHERE ($1='' OR c.character_name ILIKE '%'||$1||'%' OR COALESCE(c.guild_name,'') ILIKE '%'||$1||'%' OR c.server_name ILIKE '%'||$1||'%' OR COALESCE(c.zone_name,'') ILIKE '%'||$1||'%') AND ($2='' OR EXISTS(SELECT 1 FROM character_group_members m WHERE m.character_id=c.character_id AND m.group_id=$2::uuid)) ORDER BY c.server_name,c.character_name LIMIT 500`
	rows, err := s.pool.Query(ctx, q, strings.TrimSpace(query), groupID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Character{}
	for rows.Next() {
		c, e := scanCharacter(rows)
		if e != nil {
			return nil, e
		}
		out = append(out, c)
	}
	return out, rows.Err()
}
func (s *Store) Get(ctx context.Context, id string) (Character, error) {
	c, err := scanCharacter(s.pool.QueryRow(ctx, selectCharacters+` WHERE c.character_id=$1`, id))
	if errors.Is(err, pgx.ErrNoRows) {
		return c, ErrNotFound
	}
	return c, err
}
func (s *Store) Groups(ctx context.Context) ([]Group, error) {
	rows, err := s.pool.Query(ctx, `SELECT group_id::text,name FROM character_groups ORDER BY name`)
	if err != nil {
		return nil, err
	}
	defer rows.Close()
	out := []Group{}
	for rows.Next() {
		var g Group
		if err := rows.Scan(&g.ID, &g.Name); err != nil {
			return nil, err
		}
		g.Members = []Character{}
		members, e := s.List(ctx, "", g.ID)
		if e != nil {
			return nil, e
		}
		g.Members = members
		out = append(out, g)
	}
	return out, rows.Err()
}
func (s *Store) CreateGroup(ctx context.Context, name string) (Group, error) {
	var g Group
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 80 {
		return g, errors.New("group name must be 1 to 80 characters")
	}
	err := s.pool.QueryRow(ctx, `INSERT INTO character_groups(name) VALUES($1) RETURNING group_id::text,name`, name).Scan(&g.ID, &g.Name)
	g.Members = []Character{}
	return g, err
}
func (s *Store) RenameGroup(ctx context.Context, id, name string) error {
	name = strings.TrimSpace(name)
	if len(name) < 1 || len(name) > 80 {
		return errors.New("group name must be 1 to 80 characters")
	}
	tag, err := s.pool.Exec(ctx, `UPDATE character_groups SET name=$2,updated_at=now() WHERE group_id=$1`, id, name)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (s *Store) DeleteGroup(ctx context.Context, id string) error {
	tag, err := s.pool.Exec(ctx, `DELETE FROM character_groups WHERE group_id=$1`, id)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
func (s *Store) SetMember(ctx context.Context, gid, cid string, add bool) error {
	if add {
		_, err := s.pool.Exec(ctx, `INSERT INTO character_group_members(group_id,character_id) VALUES($1,$2) ON CONFLICT DO NOTHING`, gid, cid)
		return err
	}
	tag, err := s.pool.Exec(ctx, `DELETE FROM character_group_members WHERE group_id=$1 AND character_id=$2`, gid, cid)
	if err == nil && tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return err
}
