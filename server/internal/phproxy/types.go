package phproxy

import "time"

const (
	SchemaVersionV1                     = 1
	SchemaVersionV2                     = 2
	SchemaVersion                       = SchemaVersionV1
	ProfileName                         = "silkroad-equipment-v1"
	EvidenceReasonVerified              = "verified"
	EvidenceReasonUnverifiedReference   = "unverified_reference_mapping"
	EvidenceReasonInferredJobTransition = "inferred_job_item_transition"
	IdentityKindCharacterName           = "character_name"
	IdentityKindJobAlias                = "job_alias"
	IdentityLinkStatusInferred          = "inferred"
	IdentityLinkReasonJobTransition     = "job_item_transition"
	OpcodeSpawnEquipment                = 12309 // 0x3015
	OpcodeSpawnEquipment2               = 12313 // 0x3019
	OpcodeMetadataB007                  = 45063 // 0xB007
	OpcodeMetadata3065                  = 12389 // 0x3065
	OpcodeMetadata3864                  = 14436 // 0x3864

	MaxBodyBytes              = 256 * 1024
	MaxPlayersPerBatch        = 128
	MaxGearPerPlayer          = 64
	MaxIdentityLinksPerPlayer = 64
	MaxPlayerNameBytesV1      = 64
	MaxPlayerNameBytesV2      = 128
	MaxGuildBytes             = 1024
	MaxRaceBytes              = 64
	MaxPendingPerAgent        = 10_000
	PendingTTL                = 24 * time.Hour
	IdempotencyTTL            = 24 * time.Hour
)

type FieldAssociation struct {
	Alias      string    `json:"alias"`
	Status     string    `json:"status"`
	Reason     string    `json:"reason"`
	ObservedAt time.Time `json:"observed_at"`
}

type FieldEvidence struct {
	ObservedAt         time.Time         `json:"observed_at"`
	Profile            string            `json:"profile"`
	SourceOpcode       int               `json:"source_opcode"`
	Reason             string            `json:"reason"`
	SourceIdentityKind string            `json:"source_identity_kind,omitempty"`
	SourceName         string            `json:"source_name,omitempty"`
	Association        *FieldAssociation `json:"association,omitempty"`
}

type WirePosition struct {
	Region uint16  `json:"region"`
	X      float32 `json:"x"`
	Y      float32 `json:"y"`
	Z      float32 `json:"z"`
}

type IdentityLink struct {
	Alias        string    `json:"alias"`
	Status       string    `json:"status"`
	Reason       string    `json:"reason"`
	FirstMatched time.Time `json:"first_matched"`
	LastMatched  time.Time `json:"last_matched"`
	Matches      uint64    `json:"matches"`
	Profile      string    `json:"profile"`
	SourceOpcode int       `json:"source_opcode"`
}

type GearEntry struct {
	RefItemID int  `json:"ref_item_id"`
	OptLevel  *int `json:"opt_level"`
	Slot      *int `json:"slot"`
}

type PlayerResult struct {
	Name     string  `json:"name"`
	Status   string  `json:"status"`
	PlayerID *string `json:"player_id,omitempty"`
}

type SubmitResponse struct {
	RequestID string         `json:"request_id"`
	Results   []PlayerResult `json:"results"`
}

type ServerMetadata struct {
	GatewayName string    `json:"gateway_name"`
	Online      *int      `json:"online"`
	Max         *int      `json:"max"`
	ObservedAt  time.Time `json:"observed_at"`
}

type Coverage struct {
	Profile             string `json:"profile"`
	DroppedObservations int    `json:"dropped_observations"`
	CapacityReached     bool   `json:"capacity_reached"`
}

type StatusResponse struct {
	Server            string          `json:"server"`
	GatewayName       *string         `json:"gateway_name,omitempty"`
	GatewayOnline     *int            `json:"gateway_online,omitempty"`
	GatewayMax        *int            `json:"gateway_max,omitempty"`
	GatewayObservedAt *time.Time      `json:"gateway_observed_at,omitempty"`
	Coverage          *CoverageStatus `json:"coverage,omitempty"`
	LastReceiptAt     *time.Time      `json:"last_receipt_at,omitempty"`
}

type CoverageStatus struct {
	Profile             string `json:"profile"`
	DroppedObservations int    `json:"dropped_observations"`
	CapacityReached     bool   `json:"capacity_reached"`
}

type GearProjection struct {
	RefItemID int            `json:"ref_item_id"`
	OptLevel  *int           `json:"opt_level"`
	Slot      *int           `json:"slot"`
	Item      map[string]any `json:"item,omitempty"`
}

type EnrichmentProjection struct {
	Source                  string                   `json:"source"`
	ModelRefID              *int                     `json:"model_ref_id,omitempty"`
	Gear                    []GearProjection         `json:"gear,omitempty"`
	AvatarGear              []GearProjection         `json:"avatar_gear,omitempty"`
	Level                   *int                     `json:"level,omitempty"`
	Job                     *int                     `json:"job,omitempty"`
	JobLevel                *int                     `json:"job_level,omitempty"`
	Race                    *string                  `json:"race,omitempty"`
	Guild                   *string                  `json:"guild,omitempty"`
	Position                *WirePosition            `json:"position,omitempty"`
	JobItemIDs              []int                    `json:"job_item_ids,omitempty"`
	IdentityLinks           []IdentityLink           `json:"identity_links,omitempty"`
	FieldEvidence           map[string]FieldEvidence `json:"field_evidence,omitempty"`
	GearObservedAt          *time.Time               `json:"gear_observed_at,omitempty"`
	AvatarGearObservedAt    *time.Time               `json:"avatar_gear_observed_at,omitempty"`
	LevelObservedAt         *time.Time               `json:"level_observed_at,omitempty"`
	JobObservedAt           *time.Time               `json:"job_observed_at,omitempty"`
	PositionObservedAt      *time.Time               `json:"position_observed_at,omitempty"`
	JobItemIDsObservedAt    *time.Time               `json:"job_item_ids_observed_at,omitempty"`
	IdentityLinksObservedAt *time.Time               `json:"identity_links_observed_at,omitempty"`
}

// ExtendedPayload stores v2-only columns in JSONB alongside proxy list fallbacks.
type ExtendedPayload struct {
	AvatarGear              []GearEntry    `json:"avatar_gear,omitempty"`
	AvatarGearObservedAt    *time.Time     `json:"avatar_gear_observed_at,omitempty"`
	Level                   *int           `json:"level,omitempty"`
	LevelObservedAt         *time.Time     `json:"level_observed_at,omitempty"`
	Job                     *int           `json:"job,omitempty"`
	JobLevel                *int           `json:"job_level,omitempty"`
	JobObservedAt           *time.Time     `json:"job_observed_at,omitempty"`
	JobLevelObservedAt      *time.Time     `json:"job_level_observed_at,omitempty"`
	Race                    *string        `json:"race,omitempty"`
	RaceObservedAt          *time.Time     `json:"race_observed_at,omitempty"`
	Guild                   *string        `json:"guild,omitempty"`
	GuildObservedAt         *time.Time     `json:"guild_observed_at,omitempty"`
	Position                *WirePosition  `json:"position,omitempty"`
	PositionObservedAt      *time.Time     `json:"position_observed_at,omitempty"`
	JobItemIDs              []int          `json:"job_item_ids,omitempty"`
	JobItemIDsObservedAt    *time.Time     `json:"job_item_ids_observed_at,omitempty"`
	IdentityLinks           []IdentityLink `json:"identity_links,omitempty"`
	IdentityLinksObservedAt *time.Time     `json:"identity_links_observed_at,omitempty"`
}

type ListDisplayFallback struct {
	Level          *int   `json:"level,omitempty"`
	LevelSource    string `json:"level_source,omitempty"`
	Job            string `json:"job,omitempty"`
	JobSource      string `json:"job_source,omitempty"`
	JobLevel       *int   `json:"job_level,omitempty"`
	JobLevelSource string `json:"job_level_source,omitempty"`
	Guild          string `json:"guild,omitempty"`
	GuildSource    string `json:"guild_source,omitempty"`
	ModelID        *int64 `json:"model_id,omitempty"`
	ModelSource    string `json:"model_source,omitempty"`
}
