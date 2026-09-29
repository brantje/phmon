package commands

import (
	"encoding/json"
	"errors"
	"strings"
	"time"
	"unicode/utf8"
)

var (
	ErrInvalid             = errors.New("invalid command")
	ErrNotFound            = errors.New("command target not found")
	ErrStaleSession        = errors.New("stale character session")
	ErrUnsupported         = errors.New("command unsupported")
	ErrInFlight            = errors.New("character already has an in-flight command")
	ErrIdempotencyConflict = errors.New("idempotency key conflicts with another request")
	ErrRateLimited         = errors.New("command admission rate limited")
)

type State string

const (
	StateQueued       State = "queued"
	StateDispatching  State = "dispatching"
	StateSent         State = "sent"
	StateAcknowledged State = "acknowledged"
	StateCompleted    State = "completed"
	StateFailed       State = "failed"
	StateExpired      State = "expired"
	StateUnknown      State = "unknown"
)

type Target struct {
	CharacterID string
	SessionID   string
	AgentID     string
	Generation  uint64
	Region      *int
}

type Capability struct {
	Supported bool     `json:"supported"`
	Reason    string   `json:"reason,omitempty"`
	Modes     []string `json:"modes,omitempty"`
}

type CapabilityChecker interface {
	CommandSupport(agentID string, generation uint64, commandName string) (bool, string)
}

type Validated struct {
	Name         string
	Args         json.RawMessage
	Confirmation bool
}

type Admission struct {
	OperatorIdentity  string
	IdempotencyKey    string
	RequestHash       [32]byte
	ExpectedSessionID string
	Validated         Validated
	ExpiresAt         time.Time
}

type Command struct {
	ID                   string          `json:"command_id"`
	CharacterID          string          `json:"character_id"`
	SessionID            string          `json:"session_id"`
	AgentID              string          `json:"agent_id"`
	ConnectionGeneration uint64          `json:"connection_generation"`
	OperatorIdentity     string          `json:"operator_identity"`
	IdempotencyKey       string          `json:"idempotency_key"`
	Name                 string          `json:"name"`
	SchemaVersion        int             `json:"schema_version"`
	Args                 json.RawMessage `json:"args"`
	State                State           `json:"state"`
	CreatedAt            time.Time       `json:"created_at"`
	ExpiresAt            time.Time       `json:"expires_at"`
	DispatchStartedAt    *time.Time      `json:"dispatch_started_at,omitempty"`
	SentAt               *time.Time      `json:"sent_at,omitempty"`
	AcknowledgedAt       *time.Time      `json:"acknowledged_at,omitempty"`
	FinishedAt           *time.Time      `json:"finished_at,omitempty"`
	ResultCode           *string         `json:"result_code,omitempty"`
	ResultMessage        *string         `json:"message,omitempty"`
	Verification         *string         `json:"verification,omitempty"`
	APIReturn            json.RawMessage `json:"api_return,omitempty"`
	EffectiveArgs        json.RawMessage `json:"effective_args,omitempty"`
	ObservedAfter        json.RawMessage `json:"observed_after,omitempty"`
}

type ResultInput struct {
	Status        State
	Code          string
	Message       string
	Verification  string
	APIReturn     json.RawMessage
	EffectiveArgs json.RawMessage
	ObservedAfter json.RawMessage
}

type ControlState struct {
	SessionID         string     `json:"session_id"`
	TrainingAvailable bool       `json:"training_available"`
	TrainingRegion    *int       `json:"training_region,omitempty"`
	TrainingZone      *string    `json:"training_zone,omitempty"`
	TrainingX         *float64   `json:"training_x,omitempty"`
	TrainingY         *float64   `json:"training_y,omitempty"`
	TrainingZ         *float64   `json:"training_z,omitempty"`
	TrainingRadius    *float64   `json:"training_radius,omitempty"`
	ObservedAt        *time.Time `json:"observed_at,omitempty"`
}

func (s ControlState) ZoneNameValid() bool {
	if s.TrainingZone == nil {
		return true
	}
	return *s.TrainingZone != "" && *s.TrainingZone == strings.TrimSpace(*s.TrainingZone) && utf8.RuneCountInString(*s.TrainingZone) <= 100
}
