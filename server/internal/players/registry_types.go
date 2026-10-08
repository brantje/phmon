package players

import (
	"crypto/rand"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
	"time"
)

var (
	ErrInvalid  = errors.New("invalid player registry request")
	ErrNotFound = errors.New("player record not found")
	ErrConflict = errors.New("identity conflict or stale review")
)

const CheckpointInterval = 30 * time.Second

type Location struct {
	Region     int       `json:"region"`
	X          float64   `json:"x"`
	Y          float64   `json:"y"`
	Z          *float64  `json:"z,omitempty"`
	Zone       string    `json:"zone,omitempty"`
	Scope      string    `json:"scope,omitempty"`
	ObservedAt time.Time `json:"observed_at"`
}

type Record struct {
	ID               string     `json:"id"`
	ServerKey        string     `json:"server_key"`
	Server           string     `json:"server"`
	Name             *string    `json:"name"`
	ObservedName     string     `json:"observed_name"`
	Level            *int       `json:"level"`
	Guild            *string    `json:"guild_name"`
	Job              *string    `json:"job"`
	JobName          *string    `json:"job_name"`
	Gear             *Equipment `json:"gear"`
	GearHash         *string    `json:"gear_hash"`
	IdentityGearHash *string    `json:"identity_gear_hash"`
	Location         *Location  `json:"location"`
	FirstSeen        time.Time  `json:"first_seen_at"`
	LastSeen         time.Time  `json:"last_seen_at"`
	Resolved         bool       `json:"resolved"`
	Revision         int64      `json:"revision"`
	SourceIDs        []string   `json:"source_ids"`
}

type Alias struct {
	ID        string    `json:"id"`
	PlayerID  string    `json:"player_id"`
	Name      string    `json:"alias_name"`
	Type      string    `json:"alias_type"`
	Job       *string   `json:"job_type"`
	Method    string    `json:"match_method"`
	Status    string    `json:"confirmation_status"`
	FirstSeen time.Time `json:"first_seen_at"`
	LastSeen  time.Time `json:"last_seen_at"`
}

// Observation contains only observed facts. Runtime identifiers never identify a
// durable player outside this observer/session/epoch. Packet decoders must supply
// a verified profile before adding normal/job classifications or equipment.
type Observation struct {
	ID            string          `json:"id"`
	PlayerID      string          `json:"player_id,omitempty"`
	Server        string          `json:"server"`
	Name          string          `json:"observed_name"`
	NameType      string          `json:"name_type"`
	AgentID       string          `json:"observer_agent_id,omitempty"`
	CharacterID   string          `json:"observer_character_id,omitempty"`
	SessionID     string          `json:"observer_session_id,omitempty"`
	RuntimeID     string          `json:"runtime_entity_id,omitempty"`
	Epoch         string          `json:"runtime_epoch"`
	Model         *int64          `json:"character_model_id,omitempty"`
	Level         *int            `json:"level,omitempty"`
	Guild         *string         `json:"guild_name,omitempty"`
	Job           *string         `json:"job_type,omitempty"`
	Location      *Location       `json:"location,omitempty"`
	Equipment     *Equipment      `json:"equipment,omitempty"`
	Source        string          `json:"source"`
	SourceRef     string          `json:"source_ref"`
	ObservedAt    time.Time       `json:"observed_at"`
	LastSeen      time.Time       `json:"last_seen_at"`
	ReceivedAt    time.Time       `json:"received_at"`
	Evidence      json.RawMessage `json:"evidence,omitempty"`
	Pinned        bool            `json:"pinned,omitempty"`
	AliasConflict bool            `json:"alias_conflict,omitempty"`
}

type EquipmentHistory struct {
	ID               string      `json:"id"`
	PlayerID         string      `json:"player_id"`
	ObservationID    string      `json:"observation_id"`
	LastEvidence     Observation `json:"last_evidence"`
	Equipment        Equipment   `json:"equipment"`
	GearHash         string      `json:"gear_hash"`
	IdentityGearHash *string     `json:"identity_gear_hash"`
	FirstSeen        time.Time   `json:"first_seen_at"`
	LastSeen         time.Time   `json:"last_seen_at"`
}

func ServerKey(server string) string { return strings.ToLower(strings.TrimSpace(server)) }
func ValidID(id string) bool {
	if len(id) != 36 || id[8] != '-' || id[13] != '-' || id[18] != '-' || id[23] != '-' {
		return false
	}
	raw := strings.ReplaceAll(id, "-", "")
	_, err := hex.DecodeString(raw)
	return len(raw) == 32 && err == nil
}
func newID() string {
	var b [16]byte
	if _, err := rand.Read(b[:]); err != nil {
		panic(err)
	}
	b[6] = (b[6] & 15) | 64
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func evidenceID(parts ...string) string {
	sum := sha256.Sum256([]byte(strings.Join(parts, "\x00")))
	b := sum[:16]
	b[6] = (b[6] & 15) | 80
	b[8] = (b[8] & 63) | 128
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[:4], b[4:6], b[6:8], b[8:10], b[10:])
}
func validText(s string, max int) bool {
	return s == strings.TrimSpace(s) && len(s) <= max && !strings.ContainsRune(s, 0)
}
func validJob(s string) bool {
	return s == "trader" || s == "thief" || s == "hunter" || s == "none" || s == "unknown"
}

func (o *Observation) normalize() error {
	// Callers may share immutable observations between concurrent admissions.
	if o.Location != nil {
		location := *o.Location
		o.Location = &location
	}
	if o.Equipment != nil {
		equipment := *o.Equipment
		equipment.Slots = append([]EquipmentSlot{}, o.Equipment.Slots...)
		o.Equipment = &equipment
	}
	if o.Server == "" || !validText(o.Server, 100) || o.Name == "" || !validText(o.Name, 64) || !validText(o.RuntimeID, 64) || !validText(o.Epoch, 100) || !validText(o.Source, 64) || o.Source == "" || !validText(o.SourceRef, 200) || o.ObservedAt.IsZero() || o.ObservedAt.After(time.Now().Add(LiveFutureSkew)) {
		return ErrInvalid
	}
	if o.NameType == "" {
		o.NameType = "unknown"
	}
	if o.NameType != "unknown" && o.NameType != "normal" && o.NameType != "job" {
		return ErrInvalid
	}
	if o.Job != nil && !validJob(*o.Job) || o.Level != nil && (*o.Level < 1 || *o.Level > 255) || o.Guild != nil && !validText(*o.Guild, 64) || o.Model != nil && (*o.Model < 1 || *o.Model > 4294967295) {
		return ErrInvalid
	}
	if o.NameType == "job" && (o.Job == nil || *o.Job == "unknown" || *o.Job == "none") {
		return ErrInvalid
	}
	if l := o.Location; l != nil {
		if l.Region == 0 || l.Region < -32768 || l.Region > 65535 || !validCoordinate(l.X) || !validCoordinate(l.Y) || l.Z != nil && !validCoordinate(*l.Z) || !validText(l.Zone, 100) || !validText(l.Scope, 128) {
			return ErrInvalid
		}
		l.ObservedAt = o.ObservedAt
		if !o.LastSeen.IsZero() && o.LastSeen.After(o.ObservedAt) {
			l.ObservedAt = o.LastSeen
		}
	}
	if o.LastSeen.IsZero() {
		o.LastSeen = o.ObservedAt
	}
	if o.LastSeen.Before(o.ObservedAt) || o.LastSeen.After(time.Now().Add(LiveFutureSkew)) {
		return ErrInvalid
	}
	if o.ReceivedAt.IsZero() {
		o.ReceivedAt = time.Now().UTC()
	}
	if o.SourceRef == "" {
		o.SourceRef = evidenceID(ServerKey(o.Server), o.Source, o.SessionID, o.Epoch, o.RuntimeID, o.Name, o.ObservedAt.UTC().Format(time.RFC3339Nano))
	}
	if o.ID == "" {
		o.ID = evidenceID(ServerKey(o.Server), o.Source, o.SourceRef)
	}
	if !ValidID(o.ID) || len(o.Evidence) > 64*1024 || len(o.Evidence) > 0 && !json.Valid(o.Evidence) {
		return ErrInvalid
	}
	if o.Equipment != nil {
		if o.Equipment.ObservedAt.IsZero() {
			o.Equipment.ObservedAt = o.ObservedAt
		}
		o.Equipment.LastAvailability = o.Equipment.Availability
		o.Equipment.LastAttemptAt = o.Equipment.ObservedAt
		for i := range o.Equipment.Slots {
			if o.Equipment.Slots[i].ObservedAt.IsZero() {
				o.Equipment.Slots[i].ObservedAt = o.Equipment.ObservedAt
			}
			o.Equipment.Slots[i] = slotTimes(o.Equipment.Slots[i], o.Equipment.ObservedAt)
		}
		return o.Equipment.Validate()
	}
	return nil
}

func (o Observation) signature() string {
	b, _ := json.Marshal(struct {
		Name, Type    string
		Level         *int
		Guild, Job    *string
		Model         *int64
		Gear          string
		AliasConflict bool
	}{o.Name, o.NameType, o.Level, o.Guild, o.Job, o.Model, gearHash(o.Equipment), o.AliasConflict})
	sum := sha256.Sum256(b)
	return hex.EncodeToString(sum[:])
}

// ClassifyVisibleName separates a nearby player's visible name into a normal
// character name or a job alias. Silkroad job suits replace the character name
// with an alias that ends in Trader, Hunter, or Thief. A bare job word is left
// as a normal name so an exact character name is not treated as a suit.
func ClassifyVisibleName(name string) (nameType, job string) {
	for _, candidate := range []struct{ suffix, job string }{
		{"Trader", "trader"},
		{"Hunter", "hunter"},
		{"Thief", "thief"},
	} {
		if len(name) > len(candidate.suffix) && strings.HasSuffix(name, candidate.suffix) {
			return "job", candidate.job
		}
	}
	return "normal", ""
}

func LiveObservations(snapshot LiveSnapshot) []Observation {
	if snapshot.Status == "unavailable" {
		return nil
	}
	out := make([]Observation, 0, len(snapshot.Players))
	names := map[string]int{}
	for _, player := range snapshot.Players {
		names[strings.ToLower(player.Name)]++
	}
	for _, p := range snapshot.Players {
		region := snapshot.Region
		if p.Region != nil {
			region = *p.Region
		}
		var guild *string
		if p.Guild != "" {
			g := p.Guild
			guild = &g
		}
		nameType, job := ClassifyVisibleName(p.Name)
		var jobRef *string
		if job != "" {
			value := job
			jobRef = &value
		}
		out = append(out, Observation{Server: snapshot.Server, Name: p.Name, NameType: nameType, Job: jobRef, AgentID: snapshot.AgentID, CharacterID: snapshot.CharacterID, SessionID: snapshot.SessionID, RuntimeID: p.PlayerID, Epoch: fmt.Sprint(snapshot.Generation), Level: p.Level, Guild: guild, Source: "map.players", ObservedAt: snapshot.ObservedAt, AliasConflict: names[strings.ToLower(p.Name)] > 1, Location: &Location{Region: region, X: p.X, Y: p.Y, Zone: p.Zone}})
	}
	return out
}
