package analytics

import (
	"errors"
	"fmt"
	"strings"
	"time"
)

type View string

const (
	ViewDeaths      View = "deaths"
	ViewRareDrops   View = "rare_drops"
	ViewNormalDrops View = "normal_drops"
	ViewEconomy     View = "economy"
	ViewAcademy     View = "academy"
	ViewAlchemy     View = "alchemy"
	ViewPerformance View = "performance"
)

var ErrInvalidFilter = errors.New("invalid analytics filter")
var ErrCharacterScope = errors.New("character is outside the selected server scope")

type Filter struct {
	Server       string    `json:"server,omitempty"`
	CharacterID  string    `json:"character_id,omitempty"`
	GroupID      string    `json:"group_id,omitempty"`
	View         View      `json:"view"`
	From         time.Time `json:"from"`
	To           time.Time `json:"to"`
	Timezone     string    `json:"timezone"`
	Bucket       string    `json:"bucket"`
	GroupBy      string    `json:"group_by"`
	Guild        string    `json:"guild,omitempty"`
	BalanceScope string    `json:"balance_scope,omitempty"`
	ItemType     string    `json:"item_type,omitempty"`
	ItemDegree   string    `json:"item_degree,omitempty"`
	PageSize     int       `json:"page_size"`
	Cursor       string    `json:"cursor,omitempty"`
}

type Point struct {
	Bucket      string `json:"bucket"`
	Label       string `json:"label"`
	Value       string `json:"value"`
	Series      string `json:"series,omitempty"`
	CharacterID string `json:"character_id,omitempty"`
}

type Metric struct {
	Key    string   `json:"key"`
	Label  string   `json:"label"`
	Value  string   `json:"value,omitempty"`
	Number *float64 `json:"number,omitempty"`
	Unit   string   `json:"unit,omitempty"`
	Status string   `json:"status"`
	Reason string   `json:"reason,omitempty"`
	Href   string   `json:"href,omitempty"`
}

type Coverage struct {
	Status        string     `json:"status"`
	Reason        string     `json:"reason,omitempty"`
	OldestSample  *time.Time `json:"oldest_sample,omitempty"`
	NewestSample  *time.Time `json:"newest_sample,omitempty"`
	RetentionDays int        `json:"retention_days,omitempty"`
}

type Occurrence struct {
	ID           string         `json:"event_id"`
	Kind         string         `json:"kind"`
	CharacterID  string         `json:"character_id"`
	Character    string         `json:"character"`
	Server       string         `json:"server"`
	OccurredAt   time.Time      `json:"occurred_at"`
	Detail       string         `json:"detail,omitempty"`
	Location     string         `json:"location,omitempty"`
	Region       *int           `json:"region,omitempty"`
	ItemModel    *int64         `json:"item_model,omitempty"`
	ItemCode     string         `json:"item_code,omitempty"`
	Success      *bool          `json:"success,omitempty"`
	Plus         *int           `json:"plus,omitempty"`
	Payload      map[string]any `json:"payload"`
	ItemMetadata map[string]any `json:"item_metadata,omitempty"`
	ItemDetails  map[string]any `json:"item_details,omitempty"`
	ItemName     string         `json:"item_name,omitempty"`
	ItemIconURL  string         `json:"item_icon_url,omitempty"`
}

type Snapshot struct {
	Filter                Filter                `json:"filter"`
	CalculationVersion    string                `json:"calculation_version"`
	AsOf                  time.Time             `json:"as_of"`
	Status                string                `json:"status"`
	Reason                string                `json:"reason,omitempty"`
	Coverage              Coverage              `json:"coverage"`
	Total                 string                `json:"total"`
	Summary               []Metric              `json:"summary"`
	TimeSeries            []Point               `json:"time_series"`
	Breakdown             []Point               `json:"breakdown"`
	Occurrences           []Occurrence          `json:"occurrences"`
	NextCursor            string                `json:"next_cursor,omitempty"`
	Truncated             bool                  `json:"truncated,omitempty"`
	Performance           *CharacterPerformance `json:"performance,omitempty"`
	Taxonomy              []Point               `json:"taxonomy,omitempty"`
	TaxonomyOptions       []TaxonomyOption      `json:"taxonomy_options,omitempty"`
	TaxonomyKnown         int64                 `json:"taxonomy_known"`
	TaxonomyUnknown       int64                 `json:"taxonomy_unknown"`
	TaxonomyDegreeUnknown int64                 `json:"taxonomy_degree_unknown"`
	TaxonomyComplete      bool                  `json:"taxonomy_complete"`
}

type TaxonomyOption struct {
	Type   string `json:"type"`
	Degree string `json:"degree,omitempty"`
}

type TrainingPoint struct {
	Region        *int    `json:"region,omitempty"`
	Zone          string  `json:"zone,omitempty"`
	CoveredSecond float64 `json:"covered_seconds"`
}

type SessionPerformance struct {
	SessionID      string     `json:"session_id"`
	StartedAt      time.Time  `json:"started_at"`
	FirstSeen      time.Time  `json:"first_seen"`
	LastSeen       time.Time  `json:"last_seen"`
	EndedAt        *time.Time `json:"ended_at,omitempty"`
	CoveredSeconds float64    `json:"covered_seconds"`
}

type CharacterPerformance struct {
	CharacterID       string                `json:"character_id"`
	Character         string                `json:"character"`
	Server            string                `json:"server"`
	Rates             map[string]RateResult `json:"rates"`
	CoverageSeconds   float64               `json:"coverage_seconds"`
	UnknownBotSeconds float64               `json:"unknown_bot_seconds"`
	BottingSeconds    float64               `json:"botting_seconds"`
	IdleSeconds       float64               `json:"idle_seconds"`
	LastSampleAt      *time.Time            `json:"last_sample_at,omitempty"`
	CurrentLevel      *int                  `json:"current_level,omitempty"`
	CurrentXP         *int64                `json:"current_xp,omitempty"`
	MaxXP             *int64                `json:"max_xp,omitempty"`
	LevelETASeconds   *float64              `json:"level_eta_seconds,omitempty"`
	XPPercentPerHour  *float64              `json:"xp_percent_per_hour,omitempty"`
	Deaths24h         int64                 `json:"deaths_24h"`
	NormalDrops24h    int64                 `json:"normal_drops_24h"`
	RareDrops24h      int64                 `json:"rare_drops_24h"`
	Training          []TrainingPoint       `json:"training"`
	Sessions          []SessionPerformance  `json:"sessions"`
	SessionCount      int                   `json:"session_count"`
	Truncated         bool                  `json:"truncated,omitempty"`
	Stale             bool                  `json:"stale,omitempty"`
}

func NormalizeFilter(input Filter) (Filter, error) {
	input.Server = strings.ToLower(strings.TrimSpace(input.Server))
	input.CharacterID = strings.ToLower(strings.TrimSpace(input.CharacterID))
	input.GroupID = strings.ToLower(strings.TrimSpace(input.GroupID))
	input.Guild = strings.ToLower(strings.TrimSpace(input.Guild))
	input.BalanceScope = strings.TrimSpace(input.BalanceScope)
	input.ItemType = strings.TrimSpace(input.ItemType)
	input.ItemDegree = strings.TrimSpace(input.ItemDegree)
	input.Timezone = strings.TrimSpace(input.Timezone)
	input.Bucket = strings.TrimSpace(input.Bucket)
	input.GroupBy = strings.TrimSpace(input.GroupBy)
	if input.View != ViewDeaths && input.View != ViewRareDrops && input.View != ViewNormalDrops && input.View != ViewEconomy && input.View != ViewAcademy && input.View != ViewAlchemy && input.View != ViewPerformance {
		return Filter{}, fmt.Errorf("%w: unknown view", ErrInvalidFilter)
	}
	if len(input.Server) > 100 || len(input.CharacterID) > 36 || len(input.GroupID) > 36 || len(input.Guild) > 100 {
		return Filter{}, fmt.Errorf("%w: scope too long", ErrInvalidFilter)
	}
	if input.From.IsZero() || input.To.IsZero() || !input.To.After(input.From) || input.To.Sub(input.From) > 366*24*time.Hour {
		return Filter{}, fmt.Errorf("%w: date range must be positive and at most 366 days", ErrInvalidFilter)
	}
	input.From, input.To = input.From.UTC(), input.To.UTC()
	if input.Timezone == "" {
		input.Timezone = "UTC"
	}
	if _, err := time.LoadLocation(input.Timezone); err != nil {
		return Filter{}, fmt.Errorf("%w: invalid timezone", ErrInvalidFilter)
	}
	if input.Bucket == "" {
		input.Bucket = "day"
	}
	switch input.Bucket {
	case "hour", "day", "week":
	default:
		return Filter{}, fmt.Errorf("%w: invalid time bucket", ErrInvalidFilter)
	}
	span := input.To.Sub(input.From)
	if input.Bucket == "hour" && span > 500*time.Hour {
		input.Bucket = "day"
	}
	if input.Bucket == "day" && span > 500*24*time.Hour {
		input.Bucket = "week"
	}
	if input.GroupBy == "" {
		input.GroupBy = "character"
	}
	switch input.GroupBy {
	case "character", "group", "location", "item", "type", "degree", "academy":
	default:
		return Filter{}, fmt.Errorf("%w: invalid grouping", ErrInvalidFilter)
	}
	if input.GroupBy == "group" && input.View != ViewDeaths {
		return Filter{}, fmt.Errorf("%w: current-group breakdown applies only to deaths", ErrInvalidFilter)
	}
	if input.View == ViewPerformance && (input.CharacterID == "" || input.To.Sub(input.From) > 24*time.Hour) {
		return Filter{}, fmt.Errorf("%w: character performance requires one character and a window of at most 24 hours", ErrInvalidFilter)
	}
	if input.GroupBy == "item" && input.View != ViewRareDrops && input.View != ViewNormalDrops {
		return Filter{}, fmt.Errorf("%w: item breakdown applies only to drop views", ErrInvalidFilter)
	}
	if (input.GroupBy == "type" || input.GroupBy == "degree") && input.View != ViewRareDrops && input.View != ViewNormalDrops {
		return Filter{}, fmt.Errorf("%w: taxonomy breakdown applies only to drop views", ErrInvalidFilter)
	}
	if input.GroupBy == "academy" && input.View != ViewAcademy {
		return Filter{}, fmt.Errorf("%w: academy grouping applies only to academy view", ErrInvalidFilter)
	}
	if input.PageSize == 0 {
		input.PageSize = 25
	}
	if input.BalanceScope == "" {
		input.BalanceScope = "characters"
	}
	if input.BalanceScope != "characters" && input.BalanceScope != "guild_storage" {
		return Filter{}, fmt.Errorf("%w: invalid balance scope", ErrInvalidFilter)
	}
	if len(input.ItemType) > 64 || len(input.ItemDegree) > 2 || (input.ItemDegree != "" && (input.View != ViewRareDrops && input.View != ViewNormalDrops)) || (input.ItemType != "" && (input.View != ViewRareDrops && input.View != ViewNormalDrops)) {
		return Filter{}, fmt.Errorf("%w: invalid item taxonomy filter", ErrInvalidFilter)
	}
	if input.PageSize < 1 || input.PageSize > 100 {
		return Filter{}, fmt.Errorf("%w: page size out of range", ErrInvalidFilter)
	}
	if len(input.Cursor) > 256 {
		return Filter{}, fmt.Errorf("%w: cursor too long", ErrInvalidFilter)
	}
	return input, nil
}
