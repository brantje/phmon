package resources

import (
	"bytes"
	"crypto/sha256"
	"encoding/json"
	"errors"
	"fmt"
	"strings"
)

const (
	MaxFrameBytes    = 256 * 1024
	MaxResources     = 12
	MaxSnapshotBytes = 2 * 1024 * 1024
)

var (
	ErrInvalid  = errors.New("invalid resource snapshot")
	ErrStale    = errors.New("stale character resource session")
	ErrSequence = errors.New("resource revision gap")
)

type Snapshot struct {
	Revision     uint64                     `json:"revision"`
	BaseRevision uint64                     `json:"base_revision,omitempty"`
	Full         bool                       `json:"full"`
	Resources    map[string]json.RawMessage `json:"resources"`
}

type Observation struct {
	ResourceKey         string          `json:"resource_key"`
	Availability        string          `json:"availability"`
	Payload             json.RawMessage `json:"payload"`
	Revision            uint64          `json:"revision,omitempty"`
	ObservedAt          string          `json:"observed_at,omitempty"`
	CheckedAt           string          `json:"checked_at,omitempty"`
	ObserverCharacterID string          `json:"observer_character_id,omitempty"`
	ObserverName        string          `json:"observer_name,omitempty"`
}

type View struct {
	CharacterID string                 `json:"character_id"`
	Revision    uint64                 `json:"revision"`
	Resources   map[string]Observation `json:"resources"`
	UpdatedAt   string                 `json:"updated_at,omitempty"`
}

type GuildView struct {
	Server string        `json:"server"`
	Guild  string        `json:"guild"`
	Items  []Observation `json:"items"`
}

// GuildStorageGoldEntry is the latest observed guild-chest gold for one server/guild scope.
type GuildStorageGoldEntry struct {
	Server string `json:"server"`
	Guild  string `json:"guild"`
	Gold   int64  `json:"gold"`
}

type Assembly struct {
	full       bool
	revision   uint64
	base       uint64
	chunkCount uint64
	chunks     map[uint64]map[string]json.RawMessage
	bytes      int
}

func (a *Assembly) Add(snapshot Snapshot, chunkIndex, chunkCount uint64) (Snapshot, bool, error) {
	if _, _, err := Validate(snapshot); err != nil || chunkCount == 0 || chunkCount > MaxResources || chunkIndex >= chunkCount {
		return Snapshot{}, false, ErrInvalid
	}
	if a.chunks == nil {
		a.full, a.revision, a.base, a.chunkCount = snapshot.Full, snapshot.Revision, snapshot.BaseRevision, chunkCount
		a.chunks = make(map[uint64]map[string]json.RawMessage, chunkCount)
	} else if a.full != snapshot.Full || a.revision != snapshot.Revision || a.base != snapshot.BaseRevision || a.chunkCount != chunkCount {
		return Snapshot{}, false, ErrInvalid
	}
	if previous, exists := a.chunks[chunkIndex]; exists {
		left, _ := json.Marshal(previous)
		right, _ := json.Marshal(snapshot.Resources)
		if !bytes.Equal(left, right) {
			return Snapshot{}, false, ErrInvalid
		}
		return Snapshot{}, uint64(len(a.chunks)) == a.chunkCount, nil
	}
	for key, value := range snapshot.Resources {
		for _, chunk := range a.chunks {
			if _, exists := chunk[key]; exists {
				return Snapshot{}, false, ErrInvalid
			}
		}
		a.bytes += len(key) + len(value)
		if a.bytes > MaxSnapshotBytes {
			return Snapshot{}, false, ErrInvalid
		}
	}
	a.chunks[chunkIndex] = snapshot.Resources
	if uint64(len(a.chunks)) != a.chunkCount {
		return Snapshot{}, false, nil
	}
	combined := Snapshot{Revision: a.revision, BaseRevision: a.base, Full: a.full, Resources: make(map[string]json.RawMessage)}
	for index := uint64(0); index < a.chunkCount; index++ {
		for key, value := range a.chunks[index] {
			combined.Resources[key] = value
		}
	}
	if _, _, err := Validate(combined); err != nil {
		return Snapshot{}, false, err
	}
	return combined, true, nil
}

func Validate(snapshot Snapshot) (map[string]Observation, map[string][32]byte, error) {
	if snapshot.Revision == 0 || snapshot.Revision > 1<<53 || (snapshot.Full && snapshot.BaseRevision != 0) || (!snapshot.Full && (snapshot.BaseRevision == 0 || snapshot.BaseRevision >= snapshot.Revision || snapshot.Revision != snapshot.BaseRevision+1)) || len(snapshot.Resources) == 0 || len(snapshot.Resources) > MaxResources {
		return nil, nil, ErrInvalid
	}
	out := make(map[string]Observation, len(snapshot.Resources))
	hashes := make(map[string][32]byte, len(snapshot.Resources))
	totalBytes := 0
	for key, raw := range snapshot.Resources {
		if !validResourceKey(key) || len(raw) == 0 || len(raw) > MaxFrameBytes {
			return nil, nil, ErrInvalid
		}
		var object map[string]json.RawMessage
		if err := json.Unmarshal(raw, &object); err != nil || object == nil {
			return nil, nil, ErrInvalid
		}
		var availability string
		if err := json.Unmarshal(object["availability"], &availability); err != nil || (availability != "observed" && availability != "unavailable" && availability != "not_observed") {
			return nil, nil, ErrInvalid
		}
		canonical, err := json.Marshal(object)
		if err != nil {
			return nil, nil, ErrInvalid
		}
		if len(canonical) > MaxFrameBytes || !json.Valid(canonical) {
			return nil, nil, ErrInvalid
		}
		totalBytes += len(key) + len(canonical)
		if totalBytes > MaxSnapshotBytes {
			return nil, nil, ErrInvalid
		}
		hash := sha256.Sum256(canonical)
		out[key] = Observation{ResourceKey: key, Availability: availability, Payload: canonical}
		hashes[key] = hash
	}
	return out, hashes, nil
}

func validResourceKey(value string) bool {
	if len(value) < 1 || len(value) > 64 || strings.TrimSpace(value) != value {
		return false
	}
	for _, r := range value {
		if (r >= 'a' && r <= 'z') || (r >= 'A' && r <= 'Z') || (r >= '0' && r <= '9') || r == '_' || r == '-' || r == ':' {
			continue
		}
		return false
	}
	return true
}

func ValidResourceKey(value string) bool { return validResourceKey(value) }

func Decode(raw json.RawMessage) (Snapshot, error) {
	if len(raw) == 0 || len(raw) > MaxFrameBytes {
		return Snapshot{}, ErrInvalid
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.DisallowUnknownFields()
	var snapshot Snapshot
	if err := decoder.Decode(&snapshot); err != nil {
		return Snapshot{}, fmt.Errorf("decode resource snapshot: %w", ErrInvalid)
	}
	if _, _, err := Validate(snapshot); err != nil {
		return Snapshot{}, err
	}
	return snapshot, nil
}
