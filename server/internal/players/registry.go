package players

import (
	"context"
	"errors"
	"fmt"
	"regexp"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
)

var playerUUID = regexp.MustCompile(`^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$`)

var (
	ErrInvalidObservation = errInvalidObservation
	ErrNotFound           = errors.New("player not found")
	ErrInvalidFilter      = errors.New("invalid player filter")
)

type ModelNamer interface {
	CharacterModelName(server string, modelID *int64) string
}

type Registry struct {
	pool  *pgxpool.Pool
	namer ModelNamer
}

func NewRegistry(pool *pgxpool.Pool) *Registry { return &Registry{pool: pool} }

func (s *Registry) SetModelNamer(namer ModelNamer) {
	if s != nil {
		s.namer = namer
	}
}

type ListFilter struct {
	Server      string
	Name        string
	Guild       string
	Job         string
	Jobbing     string
	Model       string
	MinLevel    *int
	MaxLevel    *int
	MinJobLevel *int
	MaxJobLevel *int
	SeenSince   *time.Time
	Sort        string
	Descending  bool
	Limit       int
	Offset      int
}

type PlayerRecord struct {
	ID           string    `json:"id"`
	Server       string    `json:"server"`
	ObservedName string    `json:"observed_name"`
	PlayerName   *string   `json:"player_name,omitempty"`
	JobName      *string   `json:"job_name,omitempty"`
	Level        *int      `json:"level,omitempty"`
	GuildName    *string   `json:"guild_name,omitempty"`
	Job          *string   `json:"job,omitempty"`
	JobLevel     *int      `json:"job_level,omitempty"`
	IsJobbing    *bool     `json:"is_jobbing,omitempty"`
	ModelID      *int64    `json:"model_id,omitempty"`
	ModelName    string    `json:"model_name,omitempty"`
	LastRegion   *int      `json:"last_region,omitempty"`
	LastZone     string    `json:"last_zone,omitempty"`
	LastX        *float64  `json:"last_x,omitempty"`
	LastY        *float64  `json:"last_y,omitempty"`
	LastZ        *float64  `json:"last_z,omitempty"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	Progression  *Progress `json:"progression,omitempty"`
}

type Progress struct {
	FirstObservedLevel     *int       `json:"first_observed_level,omitempty"`
	LatestObservedLevel    *int       `json:"latest_observed_level,omitempty"`
	HighestObservedLevel   *int       `json:"highest_observed_level,omitempty"`
	DistinctLevelsObserved int        `json:"distinct_levels_observed"`
	LatestLevelChange      *int       `json:"latest_level_change,omitempty"`
	LatestLevelChangeAt    *time.Time `json:"latest_level_change_at,omitempty"`
	Note                   string     `json:"note"`
}

type ObservationRecord struct {
	ID              string    `json:"id"`
	PlayerID        string    `json:"player_id"`
	ObservedName    string    `json:"observed_name"`
	PlayerName      *string   `json:"player_name,omitempty"`
	JobName         *string   `json:"job_name,omitempty"`
	Level           *int      `json:"level,omitempty"`
	GuildName       *string   `json:"guild_name,omitempty"`
	Job             *string   `json:"job,omitempty"`
	JobLevel        *int      `json:"job_level,omitempty"`
	IsJobbing       *bool     `json:"is_jobbing,omitempty"`
	ModelID         *int64    `json:"model_id,omitempty"`
	Region          *int      `json:"region,omitempty"`
	Zone            string    `json:"zone,omitempty"`
	X               *float64  `json:"x,omitempty"`
	Y               *float64  `json:"y,omitempty"`
	Z               *float64  `json:"z,omitempty"`
	Source          string    `json:"source"`
	ObservedAt      time.Time `json:"observed_at"`
	RuntimeEntityID string    `json:"runtime_entity_id,omitempty"`
}

type LevelSnapshot struct {
	ID           string    `json:"id"`
	Level        int       `json:"level"`
	FirstSeenAt  time.Time `json:"first_seen_at"`
	LastSeenAt   time.Time `json:"last_seen_at"`
	ObservedName string    `json:"observed_name"`
	PlayerName   *string   `json:"player_name,omitempty"`
	JobName      *string   `json:"job_name,omitempty"`
	GuildName    *string   `json:"guild_name,omitempty"`
	Job          *string   `json:"job,omitempty"`
	JobLevel     *int      `json:"job_level,omitempty"`
	IsJobbing    *bool     `json:"is_jobbing,omitempty"`
	ModelID      *int64    `json:"model_id,omitempty"`
	ModelName    string    `json:"model_name,omitempty"`
	Region       *int      `json:"region,omitempty"`
	Zone         string    `json:"zone,omitempty"`
	X            *float64  `json:"x,omitempty"`
	Y            *float64  `json:"y,omitempty"`
	Z            *float64  `json:"z,omitempty"`
	Source       string    `json:"source"`
}

type LevelPage struct {
	PlayerID  string          `json:"player_id"`
	Progress  Progress        `json:"progress"`
	Chart     []LevelSnapshot `json:"chart"`
	Snapshots []LevelSnapshot `json:"snapshots"`
	Total     int             `json:"total"`
	Limit     int             `json:"limit"`
	Offset    int             `json:"offset"`
}

type PlayerPage struct {
	Players []PlayerRecord `json:"players"`
	Total   int            `json:"total"`
	Limit   int            `json:"limit"`
	Offset  int            `json:"offset"`
}

type ObservationPage struct {
	PlayerID     string              `json:"player_id"`
	Observations []ObservationRecord `json:"observations"`
	Total        int                 `json:"total"`
	Limit        int                 `json:"limit"`
	Offset       int                 `json:"offset"`
}

const progressNote = "Observed levels are sampled sightings. Missing levels are not filled in, and the first observation time is not the level-up time."

func (s *Registry) Apply(ctx context.Context, observations []Observation) error {
	if s == nil || s.pool == nil {
		return errors.New("player registry is unavailable")
	}
	if len(observations) == 0 {
		return nil
	}
	for i := range observations {
		s.resolveModel(&observations[i])
		if err := observations[i].validate(); err != nil {
			return err
		}
	}
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	for _, obs := range observations {
		if err := applyPlayer(ctx, tx, obs); err != nil {
			return err
		}
	}
	return tx.Commit(ctx)
}

func (s *Registry) ApplyLive(ctx context.Context, server, agentID, characterID, sessionID string, observedAt time.Time, rows []Player) error {
	observations := make([]Observation, 0, len(rows))
	for _, row := range rows {
		if row.Name == "" {
			continue
		}
		obs := Observation{
			Server: server, ObservedName: row.Name, Level: cloneInt(row.Level),
			JobLevel: cloneInt(row.JobLevel), IsJobbing: cloneBool(row.IsJobbing),
			ModelID: cloneInt64(row.ModelID), Region: cloneInt(row.Region), Zone: row.Zone,
			X: cloneFloat(&row.X), Y: cloneFloat(&row.Y), Z: cloneFloat(row.Z),
			Source: SourceMapPlayers, ObservedAt: observedAt,
			ObserverAgentID: agentID, ObserverCharacterID: characterID, ObserverSessionID: sessionID,
			RuntimeEntityID: row.PlayerID,
		}
		if row.Guild != "" {
			guild := row.Guild
			obs.Guild = &guild
		}
		if row.Job != "" {
			job := row.Job
			obs.Job = &job
		}
		observations = append(observations, obs)
	}
	return s.Apply(ctx, observations)
}

func (s *Registry) resolveModel(obs *Observation) {
	if obs == nil || obs.ModelName != "" || obs.ModelID == nil || s == nil || s.namer == nil {
		return
	}
	obs.ModelName = s.namer.CharacterModelName(obs.Server, obs.ModelID)
}

func applyPlayer(ctx context.Context, tx pgx.Tx, obs Observation) error {
	if _, err := tx.Exec(ctx, "SAVEPOINT player_row"); err != nil {
		return err
	}
	err := applyPlayerOnce(ctx, tx, obs)
	if uniqueViolation(err) {
		if _, rollbackErr := tx.Exec(ctx, "ROLLBACK TO SAVEPOINT player_row"); rollbackErr != nil {
			return rollbackErr
		}
		err = applyPlayerOnce(ctx, tx, obs)
	}
	if err != nil {
		return err
	}
	_, err = tx.Exec(ctx, "RELEASE SAVEPOINT player_row")
	return err
}

func applyPlayerOnce(ctx context.Context, tx pgx.Tx, obs Observation) error {
	current, err := lockPlayer(ctx, tx, serverKey(obs.Server), nameKey(obs.ObservedName))
	if errors.Is(err, pgx.ErrNoRows) {
		next, _ := mergePlayer(nil, obs)
		id, insertErr := insertPlayer(ctx, tx, obs.Server, next)
		if insertErr != nil {
			return insertErr
		}
		if err := insertObservation(ctx, tx, id, obs); err != nil {
			return err
		}
		_, err := writeSnapshot(ctx, tx, id, nil, obs)
		return err
	}
	if err != nil {
		return err
	}
	next, history := mergePlayer(&current.state, obs)
	if !playerStateEqual(current.state, next) {
		if err := updatePlayer(ctx, tx, current.ID, obs.Server, next); err != nil {
			return err
		}
	}
	if history {
		if err := insertObservation(ctx, tx, current.ID, obs); err != nil {
			return err
		}
	}
	_, err = writeSnapshot(ctx, tx, current.ID, nil, obs)
	return err
}

type lockedPlayer struct {
	ID    string
	state playerState
}

func lockPlayer(ctx context.Context, tx pgx.Tx, server, name string) (lockedPlayer, error) {
	var row lockedPlayer
	var zone *string
	err := tx.QueryRow(ctx, `SELECT id::text, observed_name, player_name, job_name, level, guild_name, job, job_level, is_jobbing,
		model_id, COALESCE(model_name, ''), last_region, last_zone, last_x, last_y, last_z,
		first_seen_at, last_seen_at, state_observed_at, last_history_at
		FROM players WHERE server_key=$1 AND observed_name_key=$2 FOR UPDATE`, server, name).Scan(
		&row.ID, &row.state.ObservedName, &row.state.PlayerName, &row.state.JobName, &row.state.Level, &row.state.Guild,
		&row.state.Job, &row.state.JobLevel, &row.state.IsJobbing, &row.state.ModelID, &row.state.ModelName,
		&row.state.Region, &zone, &row.state.X, &row.state.Y, &row.state.Z,
		&row.state.FirstSeenAt, &row.state.LastSeenAt, &row.state.StateObservedAt, &row.state.LastHistoryAt,
	)
	if zone != nil {
		row.state.Zone = *zone
	}
	return row, err
}

func insertPlayer(ctx context.Context, tx pgx.Tx, server string, state playerState) (string, error) {
	var id string
	err := tx.QueryRow(ctx, `INSERT INTO players (
		server_name, server_key, observed_name, observed_name_key, player_name, job_name, level, guild_name, job, job_level, is_jobbing,
		model_id, model_name, last_region, last_zone, last_x, last_y, last_z, first_seen_at, last_seen_at, state_observed_at, last_history_at
	) VALUES (
		$1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22
	) RETURNING id::text`,
		strings.TrimSpace(server), serverKey(server), state.ObservedName, nameKey(state.ObservedName),
		state.PlayerName, state.JobName, state.Level, state.Guild, state.Job, state.JobLevel, state.IsJobbing,
		state.ModelID, emptyToNil(state.ModelName), state.Region, emptyToNil(state.Zone), state.X, state.Y, state.Z,
		state.FirstSeenAt, state.LastSeenAt, state.StateObservedAt, state.LastHistoryAt,
	).Scan(&id)
	return id, err
}

func updatePlayer(ctx context.Context, tx pgx.Tx, id, server string, state playerState) error {
	_, err := tx.Exec(ctx, `UPDATE players SET server_name=$2, observed_name=$3, player_name=$4, job_name=$5, level=$6, guild_name=$7,
		job=$8, job_level=$9, is_jobbing=$10, model_id=$11, model_name=$12, last_region=$13, last_zone=$14, last_x=$15, last_y=$16, last_z=$17,
		first_seen_at=$18, last_seen_at=$19, state_observed_at=$20, last_history_at=$21, updated_at=now()
		WHERE id=$1::uuid`,
		id, strings.TrimSpace(server), state.ObservedName, state.PlayerName, state.JobName, state.Level, state.Guild,
		state.Job, state.JobLevel, state.IsJobbing, state.ModelID, emptyToNil(state.ModelName), state.Region, emptyToNil(state.Zone),
		state.X, state.Y, state.Z, state.FirstSeenAt, state.LastSeenAt, state.StateObservedAt, state.LastHistoryAt,
	)
	return err
}

func insertObservation(ctx context.Context, tx pgx.Tx, playerID string, obs Observation) error {
	_, err := tx.Exec(ctx, `INSERT INTO player_observations (
		player_id, server_key, observed_name, player_name, job_name, observer_agent_id, observer_character_id, observer_session_id,
		runtime_entity_id, level, guild_name, job, job_level, is_jobbing, model_id, region, zone, x, y, z, source, observed_at, dedupe_key
	) VALUES (
		$1::uuid,$2,$3,$4,$5,$6::uuid,$7::uuid,$8::uuid,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18,$19,$20,$21,$22,$23
	) ON CONFLICT (dedupe_key) DO NOTHING`,
		playerID, serverKey(obs.Server), obs.ObservedName, obs.PlayerName, obs.JobName,
		emptyToNil(obs.ObserverAgentID), emptyToNil(obs.ObserverCharacterID), emptyToNil(obs.ObserverSessionID),
		emptyToNil(obs.RuntimeEntityID), obs.Level, obs.Guild, obs.Job, obs.JobLevel, obs.IsJobbing, obs.ModelID,
		obs.Region, emptyToNil(obs.Zone), obs.X, obs.Y, obs.Z, obs.Source, obs.ObservedAt.UTC(), observationDedupeKey(playerID, obs),
	)
	return err
}

func writeSnapshot(ctx context.Context, tx pgx.Tx, playerID string, current *snapshotState, obs Observation) (bool, error) {
	if obs.Level == nil {
		return false, nil
	}
	var existing *snapshotState
	if current != nil {
		existing = current
	} else {
		loaded, err := lockSnapshot(ctx, tx, playerID, *obs.Level)
		if err == nil {
			existing = &loaded
		} else if !errors.Is(err, pgx.ErrNoRows) {
			return false, err
		}
	}
	next, changed := mergeSnapshot(existing, obs)
	if existing == nil {
		_, err := tx.Exec(ctx, `INSERT INTO player_level_snapshots (
			player_id, level, first_seen_at, last_seen_at, observed_name, player_name, job_name, guild_name, job, job_level, is_jobbing,
			model_id, region, zone, x, y, z, source
		) VALUES ($1::uuid,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17,$18)`,
			playerID, next.Level, next.FirstSeenAt, next.LastSeenAt, next.ObservedName, next.PlayerName, next.JobName, next.Guild,
			next.Job, next.JobLevel, next.IsJobbing, next.ModelID, next.Region, emptyToNil(next.Zone), next.X, next.Y, next.Z, next.Source,
		)
		return err == nil, err
	}
	if !changed {
		return false, nil
	}
	_, err := tx.Exec(ctx, `UPDATE player_level_snapshots SET first_seen_at=$3, last_seen_at=$4, observed_name=$5, player_name=$6, job_name=$7,
		guild_name=$8, job=$9, job_level=$10, is_jobbing=$11, model_id=$12, region=$13, zone=$14, x=$15, y=$16, z=$17, source=$18, updated_at=now()
		WHERE player_id=$1::uuid AND level=$2`,
		playerID, next.Level, next.FirstSeenAt, next.LastSeenAt, next.ObservedName, next.PlayerName, next.JobName, next.Guild,
		next.Job, next.JobLevel, next.IsJobbing, next.ModelID, next.Region, emptyToNil(next.Zone), next.X, next.Y, next.Z, next.Source,
	)
	return err == nil, err
}

func lockSnapshot(ctx context.Context, tx pgx.Tx, playerID string, level int) (snapshotState, error) {
	var state snapshotState
	var zone *string
	err := tx.QueryRow(ctx, `SELECT level, first_seen_at, last_seen_at, observed_name, player_name, job_name, guild_name, job, job_level, is_jobbing,
		model_id, region, zone, x, y, z, source FROM player_level_snapshots WHERE player_id=$1::uuid AND level=$2 FOR UPDATE`,
		playerID, level).Scan(&state.Level, &state.FirstSeenAt, &state.LastSeenAt, &state.ObservedName, &state.PlayerName, &state.JobName,
		&state.Guild, &state.Job, &state.JobLevel, &state.IsJobbing, &state.ModelID, &state.Region, &zone, &state.X, &state.Y, &state.Z, &state.Source)
	if zone != nil {
		state.Zone = *zone
	}
	return state, err
}

func (s *Registry) List(ctx context.Context, filter ListFilter) (PlayerPage, error) {
	if err := filter.normalize(); err != nil {
		return PlayerPage{}, err
	}
	where, args := filter.where()
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM players `+where, args...).Scan(&total); err != nil {
		return PlayerPage{}, err
	}
	args = append(args, filter.Limit, filter.Offset)
	rows, err := s.pool.Query(ctx, `SELECT id::text, server_name, observed_name, player_name, job_name, level, guild_name, job, job_level, is_jobbing,
		model_id, COALESCE(model_name, ''), last_region, COALESCE(last_zone, ''), last_x, last_y, last_z, first_seen_at, last_seen_at
		FROM players `+where+filter.order()+fmt.Sprintf(" LIMIT $%d OFFSET $%d", len(args)-1, len(args)), args...)
	if err != nil {
		return PlayerPage{}, err
	}
	defer rows.Close()
	players := []PlayerRecord{}
	for rows.Next() {
		player, err := scanPlayer(rows)
		if err != nil {
			return PlayerPage{}, err
		}
		player.ModelName = s.modelName(player.Server, player.ModelID, player.ModelName)
		players = append(players, player)
	}
	if err := rows.Err(); err != nil {
		return PlayerPage{}, err
	}
	return PlayerPage{Players: players, Total: total, Limit: filter.Limit, Offset: filter.Offset}, nil
}

func (s *Registry) Get(ctx context.Context, id string) (PlayerRecord, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !validPlayerUUID(id) {
		return PlayerRecord{}, ErrNotFound
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, server_name, observed_name, player_name, job_name, level, guild_name, job, job_level, is_jobbing,
		model_id, COALESCE(model_name, ''), last_region, COALESCE(last_zone, ''), last_x, last_y, last_z, first_seen_at, last_seen_at
		FROM players WHERE id=$1::uuid`, id)
	if err != nil {
		return PlayerRecord{}, err
	}
	defer rows.Close()
	if !rows.Next() {
		if err := rows.Err(); err != nil {
			return PlayerRecord{}, err
		}
		return PlayerRecord{}, ErrNotFound
	}
	player, err := scanPlayer(rows)
	if err != nil {
		return PlayerRecord{}, err
	}
	player.ModelName = s.modelName(player.Server, player.ModelID, player.ModelName)
	progress, err := s.progress(ctx, player.ID, player.Level)
	if err != nil {
		return PlayerRecord{}, err
	}
	player.Progression = &progress
	return player, nil
}

func (s *Registry) Observations(ctx context.Context, id string, limit, offset int) (ObservationPage, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !validPlayerUUID(id) {
		return ObservationPage{}, ErrNotFound
	}
	limit, offset, err := pageBounds(limit, offset)
	if err != nil {
		return ObservationPage{}, err
	}
	var exists bool
	if err := s.pool.QueryRow(ctx, `SELECT EXISTS (SELECT 1 FROM players WHERE id=$1::uuid)`, id).Scan(&exists); err != nil {
		return ObservationPage{}, err
	}
	if !exists {
		return ObservationPage{}, ErrNotFound
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_observations WHERE player_id=$1::uuid`, id).Scan(&total); err != nil {
		return ObservationPage{}, err
	}
	rows, err := s.pool.Query(ctx, `SELECT id::text, player_id::text, observed_name, player_name, job_name, level, guild_name, job, job_level, is_jobbing,
		model_id, region, COALESCE(zone, ''), x, y, z, source, observed_at, COALESCE(runtime_entity_id, '')
		FROM player_observations WHERE player_id=$1::uuid ORDER BY observed_at DESC, id DESC LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		return ObservationPage{}, err
	}
	defer rows.Close()
	observations := []ObservationRecord{}
	for rows.Next() {
		var row ObservationRecord
		if err := rows.Scan(&row.ID, &row.PlayerID, &row.ObservedName, &row.PlayerName, &row.JobName, &row.Level, &row.GuildName,
			&row.Job, &row.JobLevel, &row.IsJobbing, &row.ModelID, &row.Region, &row.Zone, &row.X, &row.Y, &row.Z, &row.Source,
			&row.ObservedAt, &row.RuntimeEntityID); err != nil {
			return ObservationPage{}, err
		}
		observations = append(observations, row)
	}
	return ObservationPage{PlayerID: id, Observations: observations, Total: total, Limit: limit, Offset: offset}, rows.Err()
}

func (s *Registry) Levels(ctx context.Context, id string, limit, offset int) (LevelPage, error) {
	id = strings.ToLower(strings.TrimSpace(id))
	if !validPlayerUUID(id) {
		return LevelPage{}, ErrNotFound
	}
	limit, offset, err := pageBounds(limit, offset)
	if err != nil {
		return LevelPage{}, err
	}
	player, err := s.Get(ctx, id)
	if err != nil {
		return LevelPage{}, err
	}
	var total int
	if err := s.pool.QueryRow(ctx, `SELECT COUNT(*) FROM player_level_snapshots WHERE player_id=$1::uuid`, id).Scan(&total); err != nil {
		return LevelPage{}, err
	}
	chartRows, err := s.pool.Query(ctx, snapshotSelect+` WHERE player_id=$1::uuid ORDER BY first_seen_at ASC, level ASC`, id)
	if err != nil {
		return LevelPage{}, err
	}
	chart, err := scanSnapshots(chartRows, player.Server, player.ModelID, player.ModelName, s)
	chartRows.Close()
	if err != nil {
		return LevelPage{}, err
	}
	pageRows, err := s.pool.Query(ctx, snapshotSelect+` WHERE player_id=$1::uuid ORDER BY first_seen_at DESC, level DESC LIMIT $2 OFFSET $3`, id, limit, offset)
	if err != nil {
		return LevelPage{}, err
	}
	defer pageRows.Close()
	snapshots, err := scanSnapshots(pageRows, player.Server, player.ModelID, player.ModelName, s)
	if err != nil {
		return LevelPage{}, err
	}
	progress := Progress{}
	if player.Progression != nil {
		progress = *player.Progression
	}
	return LevelPage{PlayerID: id, Progress: progress, Chart: chart, Snapshots: snapshots, Total: total, Limit: limit, Offset: offset}, nil
}

const snapshotSelect = `SELECT id::text, level, first_seen_at, last_seen_at, observed_name, player_name, job_name, guild_name, job, job_level, is_jobbing,
	model_id, region, COALESCE(zone, ''), x, y, z, source FROM player_level_snapshots`

func scanSnapshots(rows pgx.Rows, server string, playerModel *int64, playerModelName string, registry *Registry) ([]LevelSnapshot, error) {
	snapshots := []LevelSnapshot{}
	for rows.Next() {
		var row LevelSnapshot
		if err := rows.Scan(&row.ID, &row.Level, &row.FirstSeenAt, &row.LastSeenAt, &row.ObservedName, &row.PlayerName, &row.JobName,
			&row.GuildName, &row.Job, &row.JobLevel, &row.IsJobbing, &row.ModelID, &row.Region, &row.Zone, &row.X, &row.Y, &row.Z, &row.Source); err != nil {
			return nil, err
		}
		modelID := row.ModelID
		if modelID == nil {
			modelID = playerModel
		}
		stored := ""
		if modelID != nil && playerModel != nil && *modelID == *playerModel {
			stored = playerModelName
		}
		if registry != nil {
			row.ModelName = registry.modelName(server, modelID, stored)
		}
		snapshots = append(snapshots, row)
	}
	return snapshots, rows.Err()
}

func (s *Registry) progress(ctx context.Context, id string, currentLevel *int) (Progress, error) {
	progress := Progress{Note: progressNote, LatestObservedLevel: cloneInt(currentLevel)}
	var first, highest, latestChange *int
	var latestChangeAt *time.Time
	var distinct int
	err := s.pool.QueryRow(ctx, `SELECT
		(SELECT level FROM player_level_snapshots WHERE player_id=$1::uuid ORDER BY first_seen_at ASC, level ASC LIMIT 1),
		(SELECT MAX(level) FROM player_level_snapshots WHERE player_id=$1::uuid),
		(SELECT COUNT(*) FROM player_level_snapshots WHERE player_id=$1::uuid),
		(SELECT level FROM player_level_snapshots WHERE player_id=$1::uuid ORDER BY first_seen_at DESC, level DESC LIMIT 1),
		(SELECT first_seen_at FROM player_level_snapshots WHERE player_id=$1::uuid ORDER BY first_seen_at DESC, level DESC LIMIT 1)`,
		id).Scan(&first, &highest, &distinct, &latestChange, &latestChangeAt)
	if err != nil {
		return Progress{}, err
	}
	progress.FirstObservedLevel = first
	progress.HighestObservedLevel = highest
	progress.DistinctLevelsObserved = distinct
	if progress.LatestObservedLevel == nil {
		var latest *int
		if err := s.pool.QueryRow(ctx, `SELECT level FROM player_level_snapshots WHERE player_id=$1::uuid ORDER BY last_seen_at DESC, level DESC LIMIT 1`, id).Scan(&latest); err != nil && !errors.Is(err, pgx.ErrNoRows) {
			return Progress{}, err
		}
		progress.LatestObservedLevel = latest
	}
	if distinct > 1 {
		progress.LatestLevelChange = latestChange
		progress.LatestLevelChangeAt = latestChangeAt
	}
	return progress, nil
}

func scanPlayer(rows pgx.Rows) (PlayerRecord, error) {
	var player PlayerRecord
	err := rows.Scan(&player.ID, &player.Server, &player.ObservedName, &player.PlayerName, &player.JobName, &player.Level,
		&player.GuildName, &player.Job, &player.JobLevel, &player.IsJobbing, &player.ModelID, &player.ModelName,
		&player.LastRegion, &player.LastZone, &player.LastX, &player.LastY, &player.LastZ, &player.FirstSeenAt, &player.LastSeenAt)
	return player, err
}

func (s *Registry) modelName(server string, modelID *int64, stored string) string {
	if stored != "" {
		return stored
	}
	if s == nil || s.namer == nil || modelID == nil {
		return ""
	}
	return s.namer.CharacterModelName(server, modelID)
}

func (filter *ListFilter) normalize() error {
	if filter.Limit == 0 {
		filter.Limit = defaultPageSize
	}
	if filter.Limit < 1 || filter.Limit > maxPageSize || filter.Offset < 0 || filter.Offset > maxOffset {
		return ErrInvalidFilter
	}
	switch filter.Sort {
	case "", "last_seen_at", "first_seen_at", "observed_name", "player_name", "job_name", "server", "level", "guild_name", "job", "job_level", "is_jobbing", "model_name":
	default:
		return ErrInvalidFilter
	}
	if filter.Sort == "" {
		filter.Sort = "last_seen_at"
		filter.Descending = true
	}
	switch filter.Job {
	case "", "none", "trader", "thief", "hunter", "unknown":
	default:
		return ErrInvalidFilter
	}
	switch filter.Jobbing {
	case "", "yes", "no", "unknown":
	default:
		return ErrInvalidFilter
	}
	if len(filter.Server) > 100 || len(filter.Name) > 64 || len(filter.Guild) > 64 || len(filter.Model) > 128 {
		return ErrInvalidFilter
	}
	if filter.MinLevel != nil && (*filter.MinLevel < 1 || *filter.MinLevel > maxLevel) {
		return ErrInvalidFilter
	}
	if filter.MaxLevel != nil && (*filter.MaxLevel < 1 || *filter.MaxLevel > maxLevel) {
		return ErrInvalidFilter
	}
	if filter.MinLevel != nil && filter.MaxLevel != nil && *filter.MinLevel > *filter.MaxLevel {
		return ErrInvalidFilter
	}
	if filter.MinJobLevel != nil && (*filter.MinJobLevel < 0 || *filter.MinJobLevel > maxLevel) {
		return ErrInvalidFilter
	}
	if filter.MaxJobLevel != nil && (*filter.MaxJobLevel < 0 || *filter.MaxJobLevel > maxLevel) {
		return ErrInvalidFilter
	}
	if filter.MinJobLevel != nil && filter.MaxJobLevel != nil && *filter.MinJobLevel > *filter.MaxJobLevel {
		return ErrInvalidFilter
	}
	return nil
}

func (filter ListFilter) where() (string, []any) {
	clauses := []string{"TRUE"}
	args := []any{}
	add := func(clause string, value any) {
		args = append(args, value)
		clauses = append(clauses, fmt.Sprintf(clause, len(args)))
	}
	if filter.Server != "" {
		add("server_key=$%d", serverKey(filter.Server))
	}
	if filter.Name != "" {
		args = append(args, likeContains(filter.Name))
		n := len(args)
		clauses = append(clauses, fmt.Sprintf("(observed_name ILIKE $%d ESCAPE '\\' OR COALESCE(player_name, '') ILIKE $%d ESCAPE '\\' OR COALESCE(job_name, '') ILIKE $%d ESCAPE '\\')", n, n, n))
	}
	if filter.Guild != "" {
		add("COALESCE(guild_name, '') ILIKE $%d ESCAPE '\\'", likeContains(filter.Guild))
	}
	if filter.Job == "unknown" {
		clauses = append(clauses, "(job IS NULL OR job='unknown')")
	} else if filter.Job != "" {
		add("job=$%d", filter.Job)
	}
	switch filter.Jobbing {
	case "yes":
		clauses = append(clauses, "is_jobbing IS TRUE")
	case "no":
		clauses = append(clauses, "is_jobbing IS FALSE")
	case "unknown":
		clauses = append(clauses, "is_jobbing IS NULL")
	}
	if filter.MinLevel != nil {
		add("level >= $%d", *filter.MinLevel)
	}
	if filter.MaxLevel != nil {
		add("level <= $%d", *filter.MaxLevel)
	}
	if filter.MinJobLevel != nil {
		add("job_level >= $%d", *filter.MinJobLevel)
	}
	if filter.MaxJobLevel != nil {
		add("job_level <= $%d", *filter.MaxJobLevel)
	}
	if filter.SeenSince != nil {
		add("last_seen_at >= $%d", filter.SeenSince.UTC())
	}
	if filter.Model != "" {
		if modelID, ok := parseModelFilter(filter.Model); ok {
			args = append(args, modelID, likeContains(filter.Model))
			idPos := len(args) - 1
			namePos := len(args)
			clauses = append(clauses, fmt.Sprintf("(model_id=$%d OR COALESCE(model_name, '') ILIKE $%d ESCAPE '\\')", idPos, namePos))
		} else {
			add("COALESCE(model_name, '') ILIKE $%d ESCAPE '\\'", likeContains(filter.Model))
		}
	}
	return "WHERE " + strings.Join(clauses, " AND "), args
}

func (filter ListFilter) order() string {
	column := "last_seen_at"
	switch filter.Sort {
	case "first_seen_at", "observed_name", "player_name", "job_name", "level", "guild_name", "job", "job_level", "is_jobbing", "model_name":
		column = filter.Sort
	case "server":
		column = "server_name"
	}
	direction := "ASC"
	if filter.Descending {
		direction = "DESC"
	}
	return " ORDER BY " + column + " " + direction + " NULLS LAST, id DESC"
}

func pageBounds(limit, offset int) (int, int, error) {
	if limit == 0 {
		limit = defaultPageSize
	}
	if limit < 1 || limit > maxPageSize || offset < 0 || offset > maxOffset {
		return 0, 0, ErrInvalidFilter
	}
	return limit, offset, nil
}

func likeContains(value string) string {
	replacer := strings.NewReplacer(`\`, `\\`, `%`, `\%`, `_`, `\_`)
	return "%" + replacer.Replace(value) + "%"
}

func parseModelFilter(value string) (int64, bool) {
	if value == "" || len(value) > 10 {
		return 0, false
	}
	var parsed int64
	for _, ch := range value {
		if ch < '0' || ch > '9' {
			return 0, false
		}
		parsed = parsed*10 + int64(ch-'0')
	}
	if parsed < 1 || parsed > 4294967295 {
		return 0, false
	}
	return parsed, true
}

func validPlayerUUID(value string) bool {
	return playerUUID.MatchString(value)
}

func emptyToNil(value string) any {
	if value == "" {
		return nil
	}
	return value
}

func uniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}

func playerStateEqual(left, right playerState) bool {
	return left.ObservedName == right.ObservedName && stringPtrEqual(left.PlayerName, right.PlayerName) &&
		stringPtrEqual(left.JobName, right.JobName) && intPtrEqual(left.Level, right.Level) &&
		stringPtrEqual(left.Guild, right.Guild) && stringPtrEqual(left.Job, right.Job) &&
		intPtrEqual(left.JobLevel, right.JobLevel) && boolPtrEqual(left.IsJobbing, right.IsJobbing) &&
		int64PtrEqual(left.ModelID, right.ModelID) && left.ModelName == right.ModelName &&
		intPtrEqual(left.Region, right.Region) && left.Zone == right.Zone &&
		floatPtrEqual(left.X, right.X) && floatPtrEqual(left.Y, right.Y) && floatPtrEqual(left.Z, right.Z) &&
		left.FirstSeenAt.Equal(right.FirstSeenAt) && left.LastSeenAt.Equal(right.LastSeenAt) &&
		left.StateObservedAt.Equal(right.StateObservedAt) && left.LastHistoryAt.Equal(right.LastHistoryAt)
}

func stringPtrEqual(left, right *string) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func intPtrEqual(left, right *int) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func int64PtrEqual(left, right *int64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func boolPtrEqual(left, right *bool) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}

func floatPtrEqual(left, right *float64) bool {
	if left == nil || right == nil {
		return left == nil && right == nil
	}
	return *left == *right
}
