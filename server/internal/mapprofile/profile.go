package mapprofile

import (
	"errors"
	"strings"
)

const GreatestDatasetID = "gamedata-47c969ded0613d4c2a22"

var ErrUnknownServer = errors.New("map profile unavailable for server")

type Floor struct {
	ID              string `json:"id"`
	Label           string `json:"label"`
	ImageStatus     string `json:"image_status"`
	TransformStatus string `json:"transform_status"`
}

type Area struct {
	ID                  string  `json:"id"`
	Label               string  `json:"label"`
	Kind                string  `json:"kind"`
	RegionMappingStatus string  `json:"region_mapping_status"`
	Floors              []Floor `json:"floors"`
}

type TileCatalog struct {
	Status            string `json:"status"`
	OrientationStatus string `json:"orientation_status"`
	TileURLFormat     string `json:"tile_url_format"`
	MinX              int    `json:"min_x"`
	MaxX              int    `json:"max_x"`
	MinY              int    `json:"min_y"`
	MaxY              int    `json:"max_y"`
	TileCount         int    `json:"tile_count"`
	Semantics         string `json:"semantics"`
}

type CoordinateTransform struct {
	AreaID        string   `json:"area_id"`
	FloorID       string   `json:"floor_id"`
	Region        int      `json:"region"`
	Status        string   `json:"status"`
	WorldOriginX  float64  `json:"world_origin_x"`
	WorldOriginY  float64  `json:"world_origin_y"`
	TileOriginX   float64  `json:"tile_origin_x"`
	TileOriginY   float64  `json:"tile_origin_y"`
	UnitsPerTileX float64  `json:"units_per_tile_x"`
	UnitsPerTileY float64  `json:"units_per_tile_y"`
	AxisX         int      `json:"axis_x"`
	AxisY         int      `json:"axis_y"`
	CommandZ      *float64 `json:"command_z,omitempty"`
}

type RegionMapping struct {
	Region  int    `json:"region"`
	AreaID  string `json:"area_id"`
	FloorID string `json:"floor_id"`
	TileX   int    `json:"tile_x"`
	TileY   int    `json:"tile_y"`
	Status  string `json:"status"`
}

type QuickDestination struct {
	ID      string  `json:"id"`
	Label   string  `json:"label"`
	AreaID  string  `json:"area_id"`
	FloorID string  `json:"floor_id"`
	Region  int     `json:"region"`
	X       float64 `json:"x"`
	Y       float64 `json:"y"`
	Z       float64 `json:"z"`
	Status  string  `json:"status"`
}

type ViewPreset struct {
	ID      string `json:"id"`
	Label   string `json:"label"`
	AreaID  string `json:"area_id"`
	FloorID string `json:"floor_id"`
	TileX   int    `json:"tile_x"`
	TileY   int    `json:"tile_y"`
	Zoom    int    `json:"zoom"`
	Status  string `json:"status"`
}

type Profile struct {
	Server                 string                `json:"server"`
	DatasetID              string                `json:"dataset_id"`
	DatasetVersion         string                `json:"dataset_version"`
	ProfileStatus          string                `json:"profile_status"`
	TileCatalog            TileCatalog           `json:"tiles"`
	CoordinateTransform    string                `json:"coordinate_transform_status"`
	CoordinateTransforms   []CoordinateTransform `json:"coordinate_transforms"`
	RegionMappingsStatus   string                `json:"region_mappings_status"`
	CommandZEvidence       string                `json:"command_z_evidence_status"`
	QuickDestinations      []QuickDestination    `json:"quick_destinations"`
	RegionMappings         []RegionMapping       `json:"region_mappings"`
	ViewPresets            []ViewPreset          `json:"view_presets"`
	Areas                  []Area                `json:"areas"`
	ValidationRequirements []string              `json:"validation_requirements"`
}

func ForServer(server, dataset string) (Profile, error) {
	server = strings.TrimSpace(server)
	if server == "" || !validDatasetID(dataset) {
		return Profile{}, ErrUnknownServer
	}
	profile := Profile{
		Server:               server,
		DatasetID:            dataset,
		DatasetVersion:       dataset,
		ProfileStatus:        "partial",
		CoordinateTransform:  "unvalidated",
		CoordinateTransforms: []CoordinateTransform{},
		RegionMappingsStatus: "unavailable-in-active-profile",
		CommandZEvidence:     "unverified",
		QuickDestinations:    []QuickDestination{},
		RegionMappings:       []RegionMapping{},
		ViewPresets:          []ViewPreset{{ID: "root-reference", Label: "Root tile reference", AreaID: "world", FloorID: "world", TileX: 168, TileY: 97, Zoom: 0, Status: "raster-reference-only"}},
		TileCatalog: TileCatalog{
			Status:            "available-for-inspection",
			OrientationStatus: "edge-continuity-supported",
			TileURLFormat:     "/game-assets/minimap/{x}x{y}.png",
			MinX:              26, MaxX: 252, MinY: 35, MaxY: 126, TileCount: 5118,
			Semantics: "outdoor region ID encodes root tile X/Y; outdoor positions use 192 world units per tile with X right and Y up. Cave floors remain unmapped",
		},
		ValidationRequirements: []string{
			"compare decoded outdoor region placement across towns and region boundaries",
			"verified Z values for map-issued commands",
			"dedicated imagery and transforms for each cave floor",
		},
	}
	if dataset != GreatestDatasetID {
		profile.ProfileStatus = "unsupported"
		profile.TileCatalog.Status = "unavailable-for-dataset"
		profile.TileCatalog.OrientationStatus = "unavailable-for-dataset"
		profile.TileCatalog.TileCount = 0
		profile.ViewPresets = []ViewPreset{}
	} else {
		// The outdoor region ID encodes the root grid tile as high-byte Y and
		// low-byte X. Keep the four observed joins as evidence, not an allowlist.
		profile.CoordinateTransform = "outdoor-region-grid"
		profile.RegionMappingsStatus = "outdoor-region-grid"
		profile.RegionMappings = []RegionMapping{
			{Region: 24744, AreaID: "world", FloorID: "world", TileX: 168, TileY: 96, Status: "validated"},
			{Region: 25000, AreaID: "world", FloorID: "world", TileX: 168, TileY: 97, Status: "validated"},
			{Region: 25735, AreaID: "world", FloorID: "world", TileX: 135, TileY: 100, Status: "validated"},
			{Region: 23941, AreaID: "world", FloorID: "world", TileX: 133, TileY: 93, Status: "validated"},
		}
		for _, mapping := range profile.RegionMappings {
			profile.CoordinateTransforms = append(profile.CoordinateTransforms, CoordinateTransform{
				AreaID: mapping.AreaID, FloorID: mapping.FloorID, Region: mapping.Region,
				Status: "validated", WorldOriginX: float64(mapping.TileX-135) * 192,
				WorldOriginY: float64(mapping.TileY-92) * 192,
				TileOriginX:  float64(mapping.TileX), TileOriginY: float64(mapping.TileY),
				UnitsPerTileX: 192, UnitsPerTileY: 192, AxisX: 1, AxisY: 1,
			})
		}
	}
	worldRegionStatus := "unvalidated"
	worldTransformStatus := "unvalidated"
	if profile.RegionMappingsStatus == "outdoor-region-grid" {
		worldRegionStatus = "outdoor-region-grid"
		worldTransformStatus = "outdoor-region-grid"
	}
	profile.Areas = []Area{
		{ID: "world", Label: "World map", Kind: "outdoor", RegionMappingStatus: worldRegionStatus, Floors: []Floor{{ID: "world", Label: "World", ImageStatus: profile.TileCatalog.Status, TransformStatus: worldTransformStatus}}},
		cave("tomb-of-qin-shi", "Jangan Cave / Tomb of Qin-Shi", floorIDs("B", 1, 6)),
		cave("donwhang-stone-cave", "Donwhang Cave / Donwhang Stone Cave", floorIDs("", 1, 4)),
		cave("job-temple", "Job Temple / Temple", []string{"1F", "2F", "Annex 1", "Annex 2", "Annex 3", "Annex 4", "Annex 5"}),
	}
	return profile, nil
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

func cave(id, label string, floorIDs []string) Area {
	floors := make([]Floor, 0, len(floorIDs))
	for _, floorID := range floorIDs {
		floors = append(floors, Floor{ID: floorID, Label: floorID, ImageStatus: "missing", TransformStatus: "unvalidated"})
	}
	return Area{ID: id, Label: label, Kind: "cave", RegionMappingStatus: "unvalidated", Floors: floors}
}

func floorIDs(prefix string, start, end int) []string {
	result := make([]string, 0, end-start+1)
	for floor := start; floor <= end; floor++ {
		if prefix == "B" {
			result = append(result, "B"+intString(floor))
		} else {
			result = append(result, intString(floor)+"F")
		}
	}
	return result
}

func intString(value int) string {
	if value == 0 {
		return "0"
	}
	var buf [20]byte
	i := len(buf)
	for value > 0 {
		i--
		buf[i] = byte('0' + value%10)
		value /= 10
	}
	return string(buf[i:])
}
