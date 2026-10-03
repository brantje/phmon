package mapprofile

import (
	"errors"
	"fmt"
	"strings"
)

const GreatestDatasetID = "gamedata-e184cceb0b359ca140c1"

var ErrUnknownServer = errors.New("map profile unavailable for server")

type Floor struct {
	ID              string       `json:"id"`
	Label           string       `json:"label"`
	ImageStatus     string       `json:"image_status"`
	TransformStatus string       `json:"transform_status"`
	Tiles           *TileCatalog `json:"tiles,omitempty"`
	RegionIDs       []int        `json:"region_ids,omitempty"`
	MinZ            *float64     `json:"min_z,omitempty"`
	MaxZ            *float64     `json:"max_z,omitempty"`
	AutoDetect      bool         `json:"auto_detect"`
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
	CaveFloors             []caveFloor           `json:"-"`
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
			Semantics: "outdoor region ID encodes root tile X/Y; outdoor positions use 192 world units per tile with X right and Y up",
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
		addCaveFloors(&profile)
	}
	worldRegionStatus := "unvalidated"
	worldTransformStatus := "unvalidated"
	if profile.RegionMappingsStatus == "outdoor-region-grid" {
		worldRegionStatus = "outdoor-region-grid"
		worldTransformStatus = "outdoor-region-grid"
	}
	profile.Areas = []Area{
		{ID: "world", Label: "World map", Kind: "outdoor", RegionMappingStatus: worldRegionStatus, Floors: []Floor{{ID: "world", Label: "World", ImageStatus: profile.TileCatalog.Status, TransformStatus: worldTransformStatus}}},
		caveArea(profile, "tomb-of-qin-shi", "Jangan Cave / Tomb of Qin-Shi", floorIDs("B", 1, 6)),
		caveArea(profile, "donwhang-stone-cave", "Donwhang Cave / Donwhang Stone Cave", floorIDs("", 1, 4)),
		caveArea(profile, "job-temple", "Job Temple / Temple", []string{"1F", "2F", "Annex 1", "Annex 2", "Annex 3", "Annex 4", "Annex 5"}),
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

func caveArea(profile Profile, id, label string, floorIDs []string) Area {
	floors := make([]Floor, 0, len(floorIDs))
	for _, floorID := range floorIDs {
		floor := Floor{ID: floorID, Label: floorID, ImageStatus: "missing", TransformStatus: "unvalidated"}
		for _, candidate := range profile.CaveFloors {
			if candidate.AreaID == id && candidate.Floor.ID == floorID {
				floor = candidate.Floor
				break
			}
		}
		floors = append(floors, floor)
	}
	status := "unvalidated"
	if len(profile.CaveFloors) > 0 {
		status = "reference-observed"
	}
	return Area{ID: id, Label: label, Kind: "cave", RegionMappingStatus: status, Floors: floors}
}

type caveFloor struct {
	AreaID string
	Floor  Floor
}

type caveDefinition struct {
	area, floor, directory, prefix         string
	region, minX, maxX, minY, maxY, count  int
	anchorX, anchorY, topRightX, topRightY float64
	minZ, maxZ                             *float64
	autoDetect                             bool
}

func zBand(value float64) *float64 { return &value }

// ClassifyCave uses only observed region and Z rules. The shared Job Temple
// region identifies 1F by default; upper/annex floors require explicit choice.
func ClassifyCave(profile Profile, region *int, z *float64) (string, string, bool) {
	if region == nil {
		return "", "", false
	}
	for _, entry := range profile.CaveFloors {
		floor := entry.Floor
		if !floor.AutoDetect {
			continue
		}
		matched := false
		for _, candidate := range floor.RegionIDs {
			if candidate == *region {
				matched = true
				break
			}
		}
		if !matched {
			continue
		}
		if floor.MinZ != nil && (z == nil || *z < *floor.MinZ) {
			continue
		}
		if floor.MaxZ != nil && (z == nil || *z > *floor.MaxZ) {
			continue
		}
		return entry.AreaID, floor.ID, true
	}
	return "", "", false
}

func addCaveFloors(profile *Profile) {
	defs := []caveDefinition{
		{"tomb-of-qin-shi", "B1", "jinsi", "qt_a01_floor01", -32761, 126, 129, 125, 132, 32, 129, 129, -22848, 386, nil, nil, true},
		{"tomb-of-qin-shi", "B2", "jinsi", "qt_a01_floor02", -32762, 117, 138, 123, 132, 220, 130, 129, -22848, 386, nil, nil, true},
		{"tomb-of-qin-shi", "B3", "jinsi", "qt_a01_floor03", -32763, 116, 139, 116, 139, 576, 130.8927, 129, -22848, 386, nil, nil, true},
		{"tomb-of-qin-shi", "B4", "jinsi", "qt_a01_floor04", -32764, 118, 137, 117, 138, 440, 131.96875, 129, -22848, 386, nil, nil, true},
		{"tomb-of-qin-shi", "B5", "jinsi", "qt_a01_floor05", -32765, 119, 137, 118, 137, 380, 119, 129, -22848, 386, nil, nil, true},
		{"tomb-of-qin-shi", "B6", "jinsi", "qt_a01_floor06", -32766, 123, 132, 123, 132, 100, 123, 129, -22848, 386, nil, nil, true},
		{"donwhang-stone-cave", "1F", "donwhang", "dh_a01_floor01", -32767, 127, 129, 126, 128, 9, 128, 127, -24192, 0, zBand(-50), zBand(70), true},
		{"donwhang-stone-cave", "2F", "donwhang", "dh_a01_floor02", -32767, 127, 129, 126, 128, 9, 128, 127, -24192, 0, zBand(71), zBand(210), true},
		{"donwhang-stone-cave", "3F", "donwhang", "dh_a01_floor03", -32767, 127, 129, 126, 128, 9, 128, 127, -24192, 0, zBand(211), zBand(350), true},
		{"donwhang-stone-cave", "4F", "donwhang", "dh_a01_floor04", -32767, 127, 129, 126, 128, 9, 128, 127, -24192, 0, zBand(351), zBand(490), true},
		{"job-temple", "1F", "egypt", "rn_sd_egypt1_01", -32752, 123, 133, 125, 130, 65, 132, 127, -20543, -1, nil, nil, true},
		{"job-temple", "2F", "egypt", "rn_sd_egypt1_02", -32752, 126, 129, 126, 129, 16, 132, 127, -20543, -1, nil, nil, false},
		{"job-temple", "Annex 1", "egypt", "rn_sd_egypt01_02", -32752, 127, 128, 125, 126, 4, 132, 127, -20543, -1, nil, nil, false},
		{"job-temple", "Annex 2", "egypt", "rn_sd_egypt01_03", -32752, 132, 133, 127, 128, 4, 132, 127, -20543, -1, nil, nil, false},
		{"job-temple", "Annex 3", "egypt", "rn_sd_egypt01_04", -32752, 128, 129, 126, 128, 6, 132, 127, -20543, -1, nil, nil, false},
		{"job-temple", "Annex 4", "egypt", "rn_sd_egypt01_05", -32752, 124, 126, 129, 130, 6, 132, 127, -20543, -1, nil, nil, false},
		{"job-temple", "Annex 5", "egypt", "rn_sd_egypt01_06", -32752, 123, 124, 128, 130, 6, 132, 127, -20543, -1, nil, nil, false},
	}
	for _, def := range defs {
		regions := []int{def.region}
		if def.area == "donwhang-stone-cave" {
			regions = append(regions, 32767)
		}
		floor := Floor{ID: def.floor, Label: def.floor, ImageStatus: "available", TransformStatus: "reference-observed",
			Tiles: &TileCatalog{Status: "available-for-inspection", OrientationStatus: "reference-observed",
				TileURLFormat: fmt.Sprintf("/game-assets/minimap_d/%s/%s_{x}x{y}.png", def.directory, def.prefix),
				MinX:          def.minX, MaxX: def.maxX, MinY: def.minY, MaxY: def.maxY, TileCount: def.count,
				Semantics: "named cave floor grid, 192 game X/Y units per tile; destination Z comes from the selected character"},
			RegionIDs: regions, MinZ: def.minZ, MaxZ: def.maxZ, AutoDetect: def.autoDetect}
		profile.CaveFloors = append(profile.CaveFloors, caveFloor{def.area, floor})
		for _, region := range regions {
			profile.CoordinateTransforms = append(profile.CoordinateTransforms, CoordinateTransform{
				AreaID: def.area, FloorID: def.floor, Region: region, Status: "validated",
				WorldOriginX: def.topRightX - 192, WorldOriginY: def.topRightY - 192,
				TileOriginX: def.anchorX, TileOriginY: def.anchorY,
				UnitsPerTileX: 192, UnitsPerTileY: 192, AxisX: 1, AxisY: 1,
			})
		}
		profile.ViewPresets = append(profile.ViewPresets, ViewPreset{ID: def.area + ":" + def.floor,
			Label: def.floor, AreaID: def.area, FloorID: def.floor,
			TileX: (def.minX + def.maxX) / 2, TileY: (def.minY + def.maxY) / 2,
			Zoom: 0, Status: "validated"})
	}
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
