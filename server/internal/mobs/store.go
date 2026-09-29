package mobs

import (
	"bytes"
	"context"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"math"
	"strconv"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	agentdomain "phmon/server/internal/agents"
)

const (
	CellSize       = 192.0
	MaxMonsters    = 128
	MaxQueryWindow = 31 * 24 * time.Hour
	MaxQueryCells  = 500
)

var (
	ErrInvalidSample     = errors.New("invalid mob observation sample")
	ErrUnauthorized      = errors.New("mob sample session is not current")
	ErrSamplingFrequency = errors.New("mob sample frequency limit exceeded")
	ErrConflict          = errors.New("mob sample id conflicts with stored sample")
)

type Position struct {
	X float64  `json:"x"`
	Y float64  `json:"y"`
	Z *float64 `json:"z,omitempty"`
}

type Monster struct {
	ID         string   `json:"id"`
	Model      *int64   `json:"model_id,omitempty"`
	Type       string   `json:"type,omitempty"`
	TypeCode   *int     `json:"type_code,omitempty"`
	Name       string   `json:"name,omitempty"`
	ServerName string   `json:"servername,omitempty"`
	Level      *int     `json:"level,omitempty"`
	HP         *int64   `json:"hp,omitempty"`
	MaxHP      *int64   `json:"max_hp,omitempty"`
	Attacking  bool     `json:"attacking,omitempty"`
	Region     int      `json:"region"`
	X          float64  `json:"x"`
	Y          float64  `json:"y"`
	Z          *float64 `json:"z,omitempty"`
}

// Sample contains only eligible, complete observations. unavailable and
// truncated live snapshots deliberately have no representation here.
type Sample struct {
	ID          string    `json:"sample_id"`
	CharacterID string    `json:"character_id"`
	SessionID   string    `json:"session_id"`
	AreaID      string    `json:"area_id"`
	FloorID     string    `json:"floor_id"`
	Region      int       `json:"region"`
	SampledAt   time.Time `json:"sampled_at"`
	Observer    Position  `json:"observer"`
	Monsters    []Monster `json:"monsters"`
}

type Store struct{ pool *pgxpool.Pool }

func NewStore(pool *pgxpool.Pool) *Store { return &Store{pool: pool} }

func ValidateSample(sample Sample, now time.Time) error {
	if !agentdomain.ValidAgentID(sample.ID) || !agentdomain.ValidAgentID(sample.CharacterID) ||
		!agentdomain.ValidAgentID(sample.SessionID) || !ValidRegion(sample.Region) ||
		sample.AreaID != "region:"+intString(sample.Region) || sample.FloorID != "unmapped" ||
		sample.SampledAt.IsZero() || sample.SampledAt.After(now.Add(2*time.Minute)) || sample.SampledAt.Before(now.Add(-24*time.Hour)) ||
		!validCoordinate(sample.Observer.X) || !validCoordinate(sample.Observer.Y) ||
		sample.Observer.Z != nil && !validCoordinate(*sample.Observer.Z) ||
		len(sample.Monsters) > MaxMonsters {
		return ErrInvalidSample
	}
	seen := make(map[string]struct{}, len(sample.Monsters))
	for _, monster := range sample.Monsters {
		if monster.ID == "" || len(monster.ID) > 64 || strings.ContainsRune(monster.ID, 0) ||
			!ValidRegion(monster.Region) || !RegionsMatch(sample.Region, monster.Region) || !validCoordinate(monster.X) || !validCoordinate(monster.Y) ||
			monster.Model != nil && (*monster.Model < 0 || *monster.Model > 4294967295) || !validMonsterDetails(monster) ||
			monster.Z != nil && !validCoordinate(*monster.Z) {
			return ErrInvalidSample
		}
		if _, ok := seen[monster.ID]; ok {
			return ErrInvalidSample
		}
		seen[monster.ID] = struct{}{}
	}
	return nil
}

func validCoordinate(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1_000_000 && value <= 1_000_000
}

func ValidCoordinate(value float64) bool { return validCoordinate(value) }

func validMonsterDetails(monster Monster) bool {
	const maxSafeJSONInteger = 9007199254740991
	return len(monster.Type) <= 64 && !strings.ContainsRune(monster.Type, 0) &&
		(monster.TypeCode == nil || *monster.TypeCode >= 0 && *monster.TypeCode <= 255) &&
		len(monster.Name) <= 128 && !strings.ContainsRune(monster.Name, 0) &&
		len(monster.ServerName) <= 128 && !strings.ContainsRune(monster.ServerName, 0) &&
		(monster.Level == nil || *monster.Level >= 1 && *monster.Level <= 255) &&
		(monster.HP == nil || *monster.HP >= 0 && *monster.HP <= maxSafeJSONInteger) &&
		(monster.MaxHP == nil || *monster.MaxHP >= 0 && *monster.MaxHP <= maxSafeJSONInteger)
}

func ValidateLiveSnapshot(status string, region int, monsters []Monster, now time.Time, observedAt time.Time) error {
	if status != "observed" && status != "unavailable" && status != "truncated" || !ValidRegion(region) ||
		observedAt.IsZero() || observedAt.After(now.Add(2*time.Minute)) || observedAt.Before(now.Add(-2*time.Minute)) || len(monsters) > MaxMonsters {
		return ErrInvalidSample
	}
	if status == "unavailable" && len(monsters) != 0 {
		return ErrInvalidSample
	}
	seen := make(map[string]struct{}, len(monsters))
	for _, monster := range monsters {
		if monster.ID == "" || len(monster.ID) > 64 || strings.ContainsRune(monster.ID, 0) ||
			!ValidRegion(monster.Region) || !RegionsMatch(region, monster.Region) ||
			!validCoordinate(monster.X) || !validCoordinate(monster.Y) || monster.Model != nil && (*monster.Model < 0 || *monster.Model > 4294967295) ||
			!validMonsterDetails(monster) || monster.Z != nil && !validCoordinate(*monster.Z) {
			return ErrInvalidSample
		}
		if _, ok := seen[monster.ID]; ok {
			return ErrInvalidSample
		}
		seen[monster.ID] = struct{}{}
	}
	return nil
}

// ValidRegion accepts SRO's signed cave region IDs and unsigned outdoor IDs.
func ValidRegion(region int) bool { return region != 0 && region >= -32768 && region <= 65535 }

// RegionsMatch accounts for the observed Donwhang convention: get_position()
// reports -32767 while get_monsters() reports the same cave as 32767.
func RegionsMatch(observerRegion, monsterRegion int) bool {
	return observerRegion == monsterRegion ||
		observerRegion == -32767 && monsterRegion == 32767 ||
		observerRegion == 32767 && monsterRegion == -32767
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	negative := value < 0
	if negative {
		value = -value
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	if negative {
		i--
		buf[i] = '-'
	}
	return string(buf[i:])
}

// Append commits the sample and every monster row in one transaction. The
// authenticated agent and active character session are read from PostgreSQL;
// client-provided server and dataset identifiers are never accepted.
func (s *Store) Append(ctx context.Context, agentID string, datasetID string, sample Sample, now time.Time) (bool, error) {
	if !agentdomain.ValidAgentID(agentID) || !validDatasetID(datasetID) || ValidateSample(sample, now) != nil {
		return false, ErrInvalidSample
	}
	encodedSample, err := json.Marshal(sample)
	if err != nil {
		return false, ErrInvalidSample
	}
	sampleHash := sha256.Sum256(encodedSample)
	tx, err := s.pool.Begin(ctx)
	if err != nil {
		return false, err
	}
	defer func() { _ = tx.Rollback(ctx) }()
	var server string
	err = tx.QueryRow(ctx, `SELECT c.server_name FROM character_sessions cs
		JOIN characters c ON c.character_id=cs.character_id
		WHERE cs.session_id=$1 AND cs.character_id=$2 AND cs.agent_id=$3
		FOR SHARE OF cs`, sample.SessionID, sample.CharacterID, agentID).Scan(&server)
	if errors.Is(err, pgx.ErrNoRows) {
		return false, ErrUnauthorized
	}
	if err != nil {
		return false, err
	}
	cellX := int(math.Floor(sample.Observer.X / CellSize))
	cellY := int(math.Floor(sample.Observer.Y / CellSize))
	minute := sample.SampledAt.UTC().Truncate(time.Minute)
	var inserted string
	err = tx.QueryRow(ctx, `INSERT INTO mob_observation_samples
		(sample_id,agent_id,character_id,session_id,server_name,dataset_id,area_id,floor_id,region,sampled_at,sample_hash,sample_minute,observer_x,observer_y,observer_z,observer_cell_x,observer_cell_y)
		VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9,$10,$11,$12,$13,$14,$15,$16,$17)
		ON CONFLICT(sample_id) DO NOTHING RETURNING sample_id::text`,
		sample.ID, agentID, sample.CharacterID, sample.SessionID, server, datasetID, sample.AreaID, sample.FloorID,
		sample.Region, sample.SampledAt.UTC(), sampleHash[:], minute, sample.Observer.X, sample.Observer.Y, sample.Observer.Z, cellX, cellY).Scan(&inserted)
	if errors.Is(err, pgx.ErrNoRows) {
		var existingCharacter, existingSession string
		var existingServer, existingDataset, existingArea, existingFloor string
		var existingHash []byte
		var existingRegion int
		err = tx.QueryRow(ctx, `SELECT character_id::text,session_id::text,server_name,dataset_id,area_id,floor_id,region,sample_hash
			FROM mob_observation_samples WHERE sample_id=$1`, sample.ID).Scan(&existingCharacter, &existingSession, &existingServer, &existingDataset, &existingArea, &existingFloor, &existingRegion, &existingHash)
		if err != nil {
			return false, err
		}
		if existingCharacter != sample.CharacterID || existingSession != sample.SessionID || !strings.EqualFold(existingServer, server) ||
			existingDataset != datasetID || existingArea != sample.AreaID || existingFloor != sample.FloorID || existingRegion != sample.Region ||
			!bytes.Equal(existingHash, sampleHash[:]) {
			return false, ErrConflict
		}
		if err = tx.Commit(ctx); err != nil {
			return false, err
		}
		return false, nil
	}
	if err != nil {
		var pgErr *pgconn.PgError
		if errors.As(err, &pgErr) && pgErr.Code == "23505" && pgErr.ConstraintName == "mob_observation_sampling_rate_idx" {
			return false, ErrSamplingFrequency
		}
		return false, err
	}
	for ordinal, monster := range sample.Monsters {
		_, err = tx.Exec(ctx, `INSERT INTO mob_observations(sample_id,ordinal,monster_id,model_id,monster_type,region,x,y,z)
			VALUES($1,$2,$3,$4,$5,$6,$7,$8,$9)`, sample.ID, ordinal, monster.ID, monster.Model,
			monster.Type, monster.Region, monster.X, monster.Y, monster.Z)
		if err != nil {
			return false, err
		}
	}
	if err = tx.Commit(ctx); err != nil {
		return false, err
	}
	return true, nil
}

type DensityFilter struct {
	Server, AreaID, FloorID, MonsterType string
	From, To                             time.Time
	ModelID                              *int64
	Limit                                int
}

type DensityCell struct {
	DatasetID       string    `json:"dataset_id"`
	AreaID          string    `json:"area_id"`
	FloorID         string    `json:"floor_id"`
	Region          int       `json:"region"`
	ObserverCellX   int       `json:"observer_cell_x"`
	ObserverCellY   int       `json:"observer_cell_y"`
	MonsterRows     int64     `json:"monster_observations"`
	EligibleSamples int64     `json:"eligible_samples"`
	AverageObserved float64   `json:"average_observed_per_sample"`
	From            time.Time `json:"from"`
	To              time.Time `json:"to"`
}

type DensityResult struct {
	Metric         string        `json:"metric"`
	Interpretation string        `json:"interpretation"`
	DatasetVersion string        `json:"dataset_version"`
	Cells          []DensityCell `json:"cells"`
	From           time.Time     `json:"from"`
	To             time.Time     `json:"to"`
	CellSize       float64       `json:"observer_cell_size"`
}

func ValidateDensityFilter(filter DensityFilter) error {
	regionText := strings.TrimPrefix(filter.AreaID, "region:")
	region, regionErr := strconv.Atoi(regionText)
	if len(filter.Server) == 0 || len(filter.Server) > 100 || filter.AreaID == "" || len(filter.AreaID) > 96 ||
		!strings.HasPrefix(filter.AreaID, "region:") || regionErr != nil || !ValidRegion(region) || filter.AreaID != "region:"+intString(region) ||
		filter.FloorID != "unmapped" || filter.From.IsZero() || filter.To.IsZero() || !filter.To.After(filter.From) ||
		filter.To.Sub(filter.From) > MaxQueryWindow || filter.Limit < 1 || filter.Limit > MaxQueryCells ||
		len(filter.MonsterType) > 64 || filter.ModelID != nil && (*filter.ModelID < 0 || *filter.ModelID > 4294967295) {
		return ErrInvalidSample
	}
	return nil
}

func (s *Store) Density(ctx context.Context, filter DensityFilter) (DensityResult, error) {
	if err := ValidateDensityFilter(filter); err != nil {
		return DensityResult{}, err
	}
	// Keep observed rows with the sample's observer cell. Monster coordinates do
	// not define coverage, so distributing these rows into monster cells would
	// incorrectly present an observer-local average as spatial mob density.
	rows, err := s.pool.Query(ctx, `SELECT s.dataset_id,s.area_id,s.floor_id,s.region,s.observer_cell_x,s.observer_cell_y,
		count(DISTINCT s.sample_id)::bigint,count(o.ordinal)::bigint,
		CASE WHEN count(DISTINCT s.sample_id)=0 THEN 0 ELSE count(o.ordinal)::double precision/count(DISTINCT s.sample_id) END
		FROM mob_observation_samples s LEFT JOIN mob_observations o ON o.sample_id=s.sample_id
			AND ($6::bigint IS NULL OR o.model_id=$6) AND ($7='' OR o.monster_type=$7)
		WHERE lower(s.server_name)=lower($1) AND s.area_id=$2 AND s.floor_id=$3 AND s.sampled_at >= $4 AND s.sampled_at < $5
		GROUP BY s.dataset_id,s.area_id,s.floor_id,s.region,s.observer_cell_x,s.observer_cell_y
		ORDER BY count(o.ordinal) DESC,s.observer_cell_x,s.observer_cell_y LIMIT $8`,
		filter.Server, filter.AreaID, filter.FloorID, filter.From.UTC(), filter.To.UTC(), filter.ModelID, filter.MonsterType, filter.Limit)
	if err != nil {
		return DensityResult{}, err
	}
	defer rows.Close()
	result := DensityResult{
		Metric:         "observer_local_average_count",
		Interpretation: "Monster rows returned by eligible snapshots divided by complete observer samples centered in each observer cell; observed-area coverage is unverified, so this is not spatial mob density.",
		Cells:          make([]DensityCell, 0), From: filter.From.UTC(), To: filter.To.UTC(), CellSize: CellSize,
	}
	for rows.Next() {
		var cell DensityCell
		cell.From, cell.To = result.From, result.To
		if err := rows.Scan(&cell.DatasetID, &cell.AreaID, &cell.FloorID, &cell.Region, &cell.ObserverCellX, &cell.ObserverCellY, &cell.EligibleSamples, &cell.MonsterRows, &cell.AverageObserved); err != nil {
			return DensityResult{}, err
		}
		result.Cells = append(result.Cells, cell)
	}
	return result, rows.Err()
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
