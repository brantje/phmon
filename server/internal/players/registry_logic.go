package players

import (
	"crypto/sha256"
	"encoding/hex"
	"errors"
	"strconv"
	"strings"
	"time"
	"unicode/utf8"
)

var errInvalidObservation = errors.New("invalid player observation")

const (
	lastSeenGap        = 30 * time.Second
	historyCheckpoint  = 5 * time.Minute
	positionChangeUnit = 50.0
	maxPageSize        = 100
	defaultPageSize    = 25
	maxOffset          = 100000
)

const (
	SourceMapPlayers    = "map.players"
	SourceSpawn         = "spawn"
	SourceThiefSighting = "thief_sighting"
)

// Observation is one verified sighting of a server-scoped displayed name.
// Nil optional fields are omitted and must not clear known player state.
type Observation struct {
	Server              string
	ObservedName        string
	PlayerName          *string
	JobName             *string
	Level               *int
	Guild               *string
	Job                 *string
	JobLevel            *int
	IsJobbing           *bool
	ModelID             *int64
	ModelName           string
	Region              *int
	Zone                string
	X                   *float64
	Y                   *float64
	Z                   *float64
	Source              string
	ObservedAt          time.Time
	ObserverAgentID     string
	ObserverCharacterID string
	ObserverSessionID   string
	RuntimeEntityID     string
}

type playerState struct {
	ObservedName    string
	PlayerName      *string
	JobName         *string
	Level           *int
	Guild           *string
	Job             *string
	JobLevel        *int
	IsJobbing       *bool
	ModelID         *int64
	ModelName       string
	Region          *int
	Zone            string
	X               *float64
	Y               *float64
	Z               *float64
	FirstSeenAt     time.Time
	LastSeenAt      time.Time
	StateObservedAt time.Time
	LastHistoryAt   time.Time
}

type snapshotState struct {
	Level        int
	FirstSeenAt  time.Time
	LastSeenAt   time.Time
	ObservedName string
	PlayerName   *string
	JobName      *string
	Guild        *string
	Job          *string
	JobLevel     *int
	IsJobbing    *bool
	ModelID      *int64
	Region       *int
	Zone         string
	X            *float64
	Y            *float64
	Z            *float64
	Source       string
}

func (o Observation) validate() error {
	if !validRegistryServer(o.Server) || !validRegistryName(o.ObservedName) {
		return errInvalidObservation
	}
	if o.PlayerName != nil && !validRegistryName(*o.PlayerName) {
		return errInvalidObservation
	}
	if o.JobName != nil && !validRegistryName(*o.JobName) {
		return errInvalidObservation
	}
	if o.Guild != nil && !validRegistryName(*o.Guild) {
		return errInvalidObservation
	}
	if o.Level != nil && (*o.Level < 1 || *o.Level > maxLevel) {
		return errInvalidObservation
	}
	if !validJobValue(o.Job) || o.JobLevel != nil && (*o.JobLevel < 0 || *o.JobLevel > maxLevel) {
		return errInvalidObservation
	}
	if o.ModelID != nil && (*o.ModelID < 1 || *o.ModelID > 4294967295) {
		return errInvalidObservation
	}
	if o.ModelName != "" && (len(o.ModelName) > 128 || strings.TrimSpace(o.ModelName) != o.ModelName) {
		return errInvalidObservation
	}
	if o.Region != nil && !validStoredRegion(*o.Region) {
		return errInvalidObservation
	}
	if o.Zone != "" && !validPlayerZone(o.Zone) {
		return errInvalidObservation
	}
	if (o.X == nil) != (o.Y == nil) || o.Z != nil && (o.X == nil || o.Y == nil) {
		return errInvalidObservation
	}
	if o.X != nil && (!validCoordinate(*o.X) || !validCoordinate(*o.Y) || o.Z != nil && !validCoordinate(*o.Z)) {
		return errInvalidObservation
	}
	switch o.Source {
	case SourceMapPlayers, SourceSpawn, SourceThiefSighting:
	default:
		return errInvalidObservation
	}
	if o.ObservedAt.IsZero() || strings.ContainsRune(o.RuntimeEntityID, 0) || len(o.RuntimeEntityID) > maxText {
		return errInvalidObservation
	}
	return nil
}

func validRegistryServer(server string) bool {
	server = strings.TrimSpace(server)
	return server != "" && len(server) <= 100 && !strings.ContainsRune(server, 0)
}

func validRegistryName(name string) bool {
	if name == "" || name != strings.TrimSpace(name) || strings.ContainsRune(name, 0) {
		return false
	}
	return len(name) <= maxText && utf8.ValidString(name)
}

func validJobValue(job *string) bool {
	if job == nil {
		return true
	}
	switch *job {
	case "none", "trader", "thief", "hunter", "unknown":
		return true
	default:
		return false
	}
}

func validStoredRegion(region int) bool {
	return region != 0 && region >= -32768 && region <= 65535
}

func serverKey(server string) string {
	return strings.ToLower(strings.TrimSpace(server))
}

func nameKey(name string) string {
	return strings.ToLower(strings.TrimSpace(name))
}

// mergePlayer applies freshness rules. Omitted fields never clear known values,
// and an older observation does not replace newer current state.
func mergePlayer(current *playerState, obs Observation) (playerState, bool) {
	if current == nil {
		state := playerState{
			ObservedName:    obs.ObservedName,
			PlayerName:      cloneString(obs.PlayerName),
			JobName:         cloneString(obs.JobName),
			Level:           cloneInt(obs.Level),
			Guild:           cloneString(obs.Guild),
			Job:             cloneString(obs.Job),
			JobLevel:        cloneInt(obs.JobLevel),
			IsJobbing:       cloneBool(obs.IsJobbing),
			ModelID:         cloneInt64(obs.ModelID),
			ModelName:       obs.ModelName,
			Region:          cloneInt(obs.Region),
			Zone:            obs.Zone,
			X:               cloneFloat(obs.X),
			Y:               cloneFloat(obs.Y),
			Z:               cloneFloat(obs.Z),
			FirstSeenAt:     obs.ObservedAt.UTC(),
			LastSeenAt:      obs.ObservedAt.UTC(),
			StateObservedAt: obs.ObservedAt.UTC(),
			LastHistoryAt:   obs.ObservedAt.UTC(),
		}
		return state, true
	}
	next := *current
	at := obs.ObservedAt.UTC()
	if !at.Before(current.StateObservedAt) {
		next.ObservedName = obs.ObservedName
		assignString(&next.PlayerName, obs.PlayerName)
		assignString(&next.JobName, obs.JobName)
		assignInt(&next.Level, obs.Level)
		assignString(&next.Guild, obs.Guild)
		assignString(&next.Job, obs.Job)
		assignInt(&next.JobLevel, obs.JobLevel)
		assignBool(&next.IsJobbing, obs.IsJobbing)
		assignInt64(&next.ModelID, obs.ModelID)
		if obs.ModelName != "" {
			next.ModelName = obs.ModelName
		}
		if obs.X != nil && obs.Y != nil {
			next.X = cloneFloat(obs.X)
			next.Y = cloneFloat(obs.Y)
			if obs.Z != nil {
				next.Z = cloneFloat(obs.Z)
			}
			if obs.Region != nil {
				next.Region = cloneInt(obs.Region)
			}
			if obs.Zone != "" {
				next.Zone = obs.Zone
			}
		}
		if at.After(current.StateObservedAt) {
			next.StateObservedAt = at
		}
	}
	if at.Before(next.FirstSeenAt) {
		next.FirstSeenAt = at
	}
	changed := playerFieldsChanged(current, obs)
	if at.After(current.LastSeenAt) && (changed || at.Sub(current.LastSeenAt) >= lastSeenGap) {
		next.LastSeenAt = at
	}
	history := changed
	if !history && !at.Before(current.LastSeenAt) && (current.LastHistoryAt.IsZero() || at.Sub(current.LastHistoryAt) >= historyCheckpoint) {
		history = true
	}
	if history && at.After(current.LastHistoryAt) {
		next.LastHistoryAt = at
	}
	return next, history
}

func playerFieldsChanged(current *playerState, obs Observation) bool {
	if current == nil {
		return true
	}
	if stringChanged(current.PlayerName, obs.PlayerName) || stringChanged(current.JobName, obs.JobName) ||
		stringChanged(current.Guild, obs.Guild) || stringChanged(current.Job, obs.Job) ||
		intChanged(current.Level, obs.Level) || intChanged(current.JobLevel, obs.JobLevel) ||
		boolChanged(current.IsJobbing, obs.IsJobbing) || int64Changed(current.ModelID, obs.ModelID) {
		return true
	}
	if obs.Zone != "" && obs.Zone != current.Zone {
		return true
	}
	return positionChanged(current, obs)
}

func positionChanged(current *playerState, obs Observation) bool {
	if obs.X == nil || obs.Y == nil {
		return false
	}
	if current.X == nil || current.Y == nil {
		return true
	}
	if obs.Region != nil && current.Region != nil && *obs.Region != *current.Region {
		return true
	}
	if obs.Region != nil && current.Region == nil {
		return true
	}
	dx := *obs.X - *current.X
	dy := *obs.Y - *current.Y
	return dx*dx+dy*dy >= positionChangeUnit*positionChangeUnit
}

func mergeSnapshot(current *snapshotState, obs Observation) (snapshotState, bool) {
	if obs.Level == nil {
		return snapshotState{}, false
	}
	at := obs.ObservedAt.UTC()
	if current == nil {
		return snapshotState{
			Level: *obs.Level, FirstSeenAt: at, LastSeenAt: at, ObservedName: obs.ObservedName,
			PlayerName: cloneString(obs.PlayerName), JobName: cloneString(obs.JobName),
			Guild: cloneString(obs.Guild), Job: cloneString(obs.Job), JobLevel: cloneInt(obs.JobLevel),
			IsJobbing: cloneBool(obs.IsJobbing), ModelID: cloneInt64(obs.ModelID),
			Region: cloneInt(obs.Region), Zone: obs.Zone, X: cloneFloat(obs.X), Y: cloneFloat(obs.Y),
			Z: cloneFloat(obs.Z), Source: obs.Source,
		}, true
	}
	next := *current
	changed := false
	if at.Before(current.FirstSeenAt) {
		next.FirstSeenAt = at
		changed = true
	}
	if !at.Before(current.LastSeenAt) {
		metadata := snapshotMetadataChanged(current, obs)
		accept := metadata || at.Equal(current.LastSeenAt) || at.Sub(current.LastSeenAt) >= lastSeenGap
		if accept {
			if at.After(current.LastSeenAt) {
				next.LastSeenAt = at
				changed = true
			}
			if assignSnapshotMetadata(&next, obs) {
				changed = true
			}
		}
	}
	return next, changed
}

func snapshotMetadataChanged(current *snapshotState, obs Observation) bool {
	return stringChanged(current.PlayerName, obs.PlayerName) || stringChanged(current.JobName, obs.JobName) ||
		stringChanged(current.Guild, obs.Guild) || stringChanged(current.Job, obs.Job) ||
		intChanged(current.JobLevel, obs.JobLevel) || boolChanged(current.IsJobbing, obs.IsJobbing) ||
		int64Changed(current.ModelID, obs.ModelID) || positionChanged(&playerState{
		Region: current.Region, Zone: current.Zone, X: current.X, Y: current.Y,
	}, obs) || (obs.Zone != "" && obs.Zone != current.Zone)
}

func assignSnapshotMetadata(next *snapshotState, obs Observation) bool {
	changed := false
	changed = assignString(&next.PlayerName, obs.PlayerName) || changed
	changed = assignString(&next.JobName, obs.JobName) || changed
	changed = assignString(&next.Guild, obs.Guild) || changed
	changed = assignString(&next.Job, obs.Job) || changed
	changed = assignInt(&next.JobLevel, obs.JobLevel) || changed
	changed = assignBool(&next.IsJobbing, obs.IsJobbing) || changed
	changed = assignInt64(&next.ModelID, obs.ModelID) || changed
	if obs.ObservedName != "" && obs.ObservedName != next.ObservedName && !obs.ObservedAt.Before(next.LastSeenAt) {
		next.ObservedName = obs.ObservedName
		changed = true
	}
	if obs.X != nil && obs.Y != nil && (next.X == nil || *next.X != *obs.X || next.Y == nil || *next.Y != *obs.Y) {
		next.X = cloneFloat(obs.X)
		next.Y = cloneFloat(obs.Y)
		changed = true
	}
	if obs.Z != nil && (next.Z == nil || *next.Z != *obs.Z) {
		next.Z = cloneFloat(obs.Z)
		changed = true
	}
	if obs.Region != nil && (next.Region == nil || *next.Region != *obs.Region) {
		next.Region = cloneInt(obs.Region)
		changed = true
	}
	if obs.Zone != "" && obs.Zone != next.Zone {
		next.Zone = obs.Zone
		changed = true
	}
	if obs.Source != "" && !obs.ObservedAt.Before(next.LastSeenAt) && obs.Source != next.Source {
		next.Source = obs.Source
		changed = true
	}
	return changed
}

func observationDedupeKey(playerID string, obs Observation) string {
	parts := []string{
		playerID, obs.Source, obs.ObserverSessionID, obs.RuntimeEntityID,
		obs.ObservedAt.UTC().Format(time.RFC3339Nano), nameKey(obs.ObservedName),
		optionalString(obs.PlayerName), optionalString(obs.JobName), optionalString(obs.Guild),
		optionalString(obs.Job), optionalInt(obs.Level), optionalInt(obs.JobLevel),
		optionalBool(obs.IsJobbing), optionalInt64(obs.ModelID), optionalInt(obs.Region), obs.Zone,
		optionalFloat(obs.X), optionalFloat(obs.Y), optionalFloat(obs.Z),
	}
	sum := sha256.Sum256([]byte(strings.Join(parts, "|")))
	return hex.EncodeToString(sum[:])
}

func stringChanged(current, next *string) bool {
	return next != nil && (current == nil || *current != *next)
}

func intChanged(current, next *int) bool {
	return next != nil && (current == nil || *current != *next)
}

func int64Changed(current, next *int64) bool {
	return next != nil && (current == nil || *current != *next)
}

func boolChanged(current, next *bool) bool {
	return next != nil && (current == nil || *current != *next)
}

func assignString(dest **string, value *string) bool {
	if value == nil || *dest != nil && **dest == *value {
		return false
	}
	*dest = cloneString(value)
	return true
}

func assignInt(dest **int, value *int) bool {
	if value == nil || *dest != nil && **dest == *value {
		return false
	}
	*dest = cloneInt(value)
	return true
}

func assignInt64(dest **int64, value *int64) bool {
	if value == nil || *dest != nil && **dest == *value {
		return false
	}
	*dest = cloneInt64(value)
	return true
}

func assignBool(dest **bool, value *bool) bool {
	if value == nil || *dest != nil && **dest == *value {
		return false
	}
	*dest = cloneBool(value)
	return true
}

func cloneString(value *string) *string {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneInt(value *int) *int {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneInt64(value *int64) *int64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneBool(value *bool) *bool {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func cloneFloat(value *float64) *float64 {
	if value == nil {
		return nil
	}
	copied := *value
	return &copied
}

func optionalString(value *string) string {
	if value == nil {
		return ""
	}
	return *value
}

func optionalInt(value *int) string {
	if value == nil {
		return ""
	}
	return strconv.Itoa(*value)
}

func optionalInt64(value *int64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatInt(*value, 10)
}

func optionalBool(value *bool) string {
	if value == nil {
		return ""
	}
	if *value {
		return "1"
	}
	return "0"
}

func optionalFloat(value *float64) string {
	if value == nil {
		return ""
	}
	return strconv.FormatFloat(*value, 'f', -1, 64)
}
