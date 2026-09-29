package mapanalytics

import "time"

const (
	LayerMobDensity      = "mob_density"
	LayerMobObserverAvg  = "mob_observer_average"
	LayerMobTypes        = "mob_types"
	LayerDeaths          = "deaths"
	LayerDrops           = "drops"
	LayerUniqueSightings = "unique_sightings"
	LayerPlayerMovement  = "player_movement"
	StatusAvailable      = "available"
	StatusLimited        = "limited"
	StatusUnsupported    = "unsupported"
)

type Filter struct {
	Layer       string
	Server      string
	DatasetID   string
	AreaID      string
	FloorID     string
	Region      *int
	CharacterID string
	MonsterType string
	ModelID     *int64
	From        time.Time
	To          time.Time
	Resolution  float64
	Limit       int
}

type Point struct {
	Region      int     `json:"region"`
	X           float64 `json:"x"`
	Y           float64 `json:"y"`
	Weight      float64 `json:"weight"`
	Count       int64   `json:"count"`
	Numerator   int64   `json:"numerator,omitempty"`
	Denominator int64   `json:"denominator,omitempty"`
}

type Result struct {
	Layer          string    `json:"layer"`
	Status         string    `json:"status"`
	Metric         string    `json:"metric"`
	Interpretation string    `json:"interpretation"`
	Reason         string    `json:"reason,omitempty"`
	Server         string    `json:"server"`
	DatasetVersion string    `json:"dataset_version"`
	AreaID         string    `json:"area_id"`
	FloorID        string    `json:"floor_id"`
	Region         *int      `json:"region,omitempty"`
	From           time.Time `json:"from"`
	To             time.Time `json:"to"`
	Resolution     float64   `json:"resolution"`
	Points         []Point   `json:"points"`
	// SourceRows is the total unsuppressed canonical source population contributing
	// to the aggregation before the bounded point limit is applied.
	SourceRows     int64     `json:"source_rows"`
	SuppressedRows int64     `json:"suppressed_rows"`
	Truncated      bool      `json:"truncated"`
}

type MobFacet struct {
	MonsterType string `json:"monster_type,omitempty"`
	ModelID     *int64 `json:"model_id,omitempty"`
	Count       int64  `json:"count"`
}

type ResetScope struct {
	Layer        string    `json:"layer"`
	Server       string    `json:"server"`
	DatasetID    string    `json:"dataset_version"`
	AreaID       string    `json:"area_id"`
	FloorID      string    `json:"floor_id"`
	Region       *int      `json:"region,omitempty"`
	CharacterID  string    `json:"character_id,omitempty"`
	MonsterType  string    `json:"monster_type,omitempty"`
	ModelID      *int64    `json:"model_id,omitempty"`
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	BroadScope   bool      `json:"broad_scope"`
	ConfirmBroad bool      `json:"-"`
	CreatedAt    time.Time `json:"created_at,omitempty"`
	ResetID      string    `json:"reset_id,omitempty"`
}
