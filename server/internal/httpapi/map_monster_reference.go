package httpapi

import (
	"math"
	"net/http"
	"sort"
	"strconv"
	"strings"

	"phmon/server/internal/mapprofile"
	"phmon/server/internal/resources"
)

type monsterRefPosition struct {
	TileX  int     `json:"tile_x"`
	TileY  int     `json:"tile_y"`
	PixelX float64 `json:"pixel_x"`
	PixelY float64 `json:"pixel_y"`
}

type monsterRefPoint struct {
	ModelID   int64              `json:"model_id"`
	Code      string             `json:"code"`
	Name      string             `json:"name"`
	Level     *int               `json:"level"`
	Region    int                `json:"region"`
	Position  monsterRefPosition `json:"position"`
	Precision string             `json:"precision"`
}

type monsterRefArea struct {
	ModelID   int64                        `json:"model_id"`
	Code      string                       `json:"code"`
	Name      string                       `json:"name"`
	Level     *int                         `json:"level"`
	Group     string                       `json:"group"`
	Cells     []resources.MonsterGuideCell `json:"cells"`
	Precision string                       `json:"precision"`
}

type monsterRefBounds struct {
	MinX int `json:"min_x"`
	MaxX int `json:"max_x"`
	MinY int `json:"min_y"`
	MaxY int `json:"max_y"`
}

func (b monsterRefBounds) contains(x, y int) bool {
	return x >= b.MinX && x <= b.MaxX && y >= b.MinY && y <= b.MaxY
}

// Guide cells are stored one square north of the terrain they describe.
// A square is the cell's own height in map tiles.
func placedGuideCell(c resources.MonsterGuideCell) resources.MonsterGuideCell {
	c.Y -= c.Height
	return c
}

func (b monsterRefBounds) intersects(c resources.MonsterGuideCell) bool {
	placed := placedGuideCell(c)
	return placed.X <= b.MaxX && placed.X+placed.Width-1 >= b.MinX && placed.Y <= b.MaxY && placed.Y+placed.Height-1 >= b.MinY
}

func monsterRefAreaFloor(group string) (string, string, bool) {
	switch group {
	case "field":
		return "world", "world", true
	case "temple":
		return "job-temple", "1F", true
	}
	if strings.HasPrefix(group, "dunhuang") && len(group) == len("dunhuang1") {
		n := group[len(group)-1]
		if n >= '1' && n <= '4' {
			return "donwhang-stone-cave", string(n) + "F", true
		}
	}
	if strings.HasPrefix(group, "jinsi") && len(group) == len("jinsi1") {
		n := group[len(group)-1]
		if n >= '1' && n <= '6' {
			return "tomb-of-qin-shi", "B" + string(n), true
		}
	}
	return "", "", false
}

func monsterRefTileCatalog(profile mapprofile.Profile, area, floor string) (mapprofile.TileCatalog, bool) {
	for _, candidate := range profile.Areas {
		if candidate.ID != area {
			continue
		}
		for _, level := range candidate.Floors {
			if level.ID == floor {
				if level.Tiles != nil {
					return *level.Tiles, true
				}
				return profile.TileCatalog, area == "world"
			}
		}
	}
	return mapprofile.TileCatalog{}, false
}

// npcpos X/Y are region-local client units (ten client units per world unit).
// Interior coordinates are relative to the verified cave map's top-right anchor.
func projectMonsterRefPoint(profile mapprofile.Profile, point resources.MonsterReferencePoint) (string, string, monsterRefPosition, bool) {
	region := point.Region
	if region > 32767 {
		region -= 65536
	}
	area, floor := "world", "world"
	var rasterX, rasterY float64
	if region > 0 && region < 32767 {
		if point.RawX < 0 || point.RawX >= 1920 || point.RawY < 0 || point.RawY >= 1920 || profile.CoordinateTransform != "outdoor-region-grid" {
			return "", "", monsterRefPosition{}, false
		}
		rasterX = float64(region%256) + point.RawX/1920
		rasterY = float64(region/256) + point.RawY/1920
	} else {
		// The Job Temple region is shared by multiple floors without a Z rule.
		if region == -32752 {
			return "", "", monsterRefPosition{}, false
		}
		var ok bool
		area, floor, ok = mapprofile.ClassifyCave(profile, &region, &point.RawZ)
		if !ok {
			return "", "", monsterRefPosition{}, false
		}
		found := false
		for _, transform := range profile.CoordinateTransforms {
			if transform.AreaID == area && transform.FloorID == floor && transform.Region == region && transform.Status == "validated" {
				rasterX = transform.TileOriginX + 1 + point.RawX/(10*transform.UnitsPerTileX)*float64(transform.AxisX)
				rasterY = transform.TileOriginY + 1 + point.RawY/(10*transform.UnitsPerTileY)*float64(transform.AxisY)
				found = true
				break
			}
		}
		if !found {
			return "", "", monsterRefPosition{}, false
		}
	}
	if math.IsNaN(rasterX) || math.IsNaN(rasterY) || math.IsInf(rasterX, 0) || math.IsInf(rasterY, 0) {
		return "", "", monsterRefPosition{}, false
	}
	tileX, tileY := int(math.Floor(rasterX)), int(math.Floor(rasterY))
	fractionY := rasterY - float64(tileY)
	if fractionY == 0 {
		tileY--
		fractionY = 1
	}
	grid, ok := monsterRefTileCatalog(profile, area, floor)
	if !ok || tileX < grid.MinX || tileX > grid.MaxX || tileY < grid.MinY || tileY > grid.MaxY {
		return "", "", monsterRefPosition{}, false
	}
	return area, floor, monsterRefPosition{tileX, tileY, (rasterX - float64(tileX)) * 256, (1 - fractionY) * 256}, true
}

func monsterRefFilter(r *http.Request) (string, int, int, int, int, bool) {
	query := strings.TrimSpace(r.URL.Query().Get("q"))
	if len(query) > 128 {
		return "", 0, 0, 0, 0, false
	}
	minLevel, maxLevel := 0, 255
	if raw := r.URL.Query().Get("min_level"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 255 {
			return "", 0, 0, 0, 0, false
		}
		minLevel = value
	}
	if raw := r.URL.Query().Get("max_level"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 255 {
			return "", 0, 0, 0, 0, false
		}
		maxLevel = value
	}
	if minLevel > maxLevel {
		return "", 0, 0, 0, 0, false
	}
	limit, offset := 100, 0
	if raw := r.URL.Query().Get("limit"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 1 || value > 1000 {
			return "", 0, 0, 0, 0, false
		}
		limit = value
	}
	if raw := r.URL.Query().Get("offset"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil || value < 0 || value > 200000 {
			return "", 0, 0, 0, 0, false
		}
		offset = value
	}
	return strings.ToLower(query), minLevel, maxLevel, limit, offset, true
}

func matchMonsterRef(def resources.MonsterDefinition, q string, minLevel, maxLevel int) bool {
	if !def.Enabled || def.Level == nil && (minLevel > 0 || maxLevel < 255) || def.Level != nil && (*def.Level < minLevel || *def.Level > maxLevel) {
		return false
	}
	return q == "" || strings.Contains(strings.ToLower(def.Name), q) || strings.Contains(strings.ToLower(def.Code), q)
}

func (h *mapHandler) monsterReferenceScope(w http.ResponseWriter, r *http.Request) (*resources.MonsterReference, mapprofile.Profile, bool) {
	server := strings.TrimSpace(r.URL.Query().Get("server"))
	if h.resources == nil || server == "" || !validServerFilter(server) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid server"})
		return nil, mapprofile.Profile{}, false
	}
	catalog, dataset, known := h.resources.MonsterReferenceForServer(server)
	if !known {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "unknown server"})
		return nil, mapprofile.Profile{}, false
	}
	profile, err := mapprofile.ForServer(server, dataset)
	if err != nil {
		respondJSON(w, http.StatusNotFound, map[string]string{"error": "map profile unavailable"})
		return nil, mapprofile.Profile{}, false
	}
	if catalog == nil {
		respondJSON(w, http.StatusOK, map[string]any{"status": "unavailable", "dataset_id": dataset, "reason": "monster reference catalog unavailable"})
		return nil, mapprofile.Profile{}, false
	}
	return catalog, profile, true
}

type monsterRefSearchRow struct {
	ModelID        int64             `json:"model_id"`
	Code           string            `json:"code"`
	Name           string            `json:"name"`
	Level          *int              `json:"level"`
	AreaID         string            `json:"area_id"`
	FloorID        string            `json:"floor_id"`
	AreaCells      int               `json:"area_cells"`
	PointCount     int               `json:"point_count"`
	Bounds         *monsterRefBounds `json:"bounds,omitempty"`
	LocationStatus string            `json:"location_status"`
}

func growMonsterRefBounds(bounds *monsterRefBounds, x, y int) *monsterRefBounds {
	if bounds == nil {
		return &monsterRefBounds{x, x, y, y}
	}
	if x < bounds.MinX {
		bounds.MinX = x
	}
	if x > bounds.MaxX {
		bounds.MaxX = x
	}
	if y < bounds.MinY {
		bounds.MinY = y
	}
	if y > bounds.MaxY {
		bounds.MaxY = y
	}
	return bounds
}

func (h *mapHandler) monsterReferenceSearch(w http.ResponseWriter, r *http.Request) {
	catalog, profile, ok := h.monsterReferenceScope(w, r)
	if !ok {
		return
	}
	q, minLevel, maxLevel, limit, offset, valid := monsterRefFilter(r)
	if !valid {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid monster reference filter"})
		return
	}
	definitions := make(map[int64]resources.MonsterDefinition, len(catalog.Definitions))
	rows := make(map[string]*monsterRefSearchRow)
	get := func(def resources.MonsterDefinition, area, floor string) *monsterRefSearchRow {
		key := strconv.FormatInt(def.ModelID, 10) + ":" + area + ":" + floor
		if row := rows[key]; row != nil {
			return row
		}
		row := &monsterRefSearchRow{ModelID: def.ModelID, Code: def.Code, Name: def.Name, Level: def.Level, AreaID: area, FloorID: floor, LocationStatus: "placed"}
		rows[key] = row
		return row
	}
	for _, def := range catalog.Definitions {
		if matchMonsterRef(def, q, minLevel, maxLevel) {
			definitions[def.ModelID] = def
		}
	}
	for _, area := range catalog.Areas {
		def, ok := definitions[area.ModelID]
		if !ok {
			continue
		}
		areaID, floorID, ok := monsterRefAreaFloor(area.Group)
		if !ok {
			continue
		}
		grid, ok := monsterRefTileCatalog(profile, areaID, floorID)
		if !ok {
			continue
		}
		row := get(def, areaID, floorID)
		for _, cell := range area.Cells {
			if cell.X < grid.MinX || cell.Y < grid.MinY || cell.X+cell.Width-1 > grid.MaxX || cell.Y+cell.Height-1 > grid.MaxY {
				continue
			}
			row.AreaCells++
			placed := placedGuideCell(cell)
			row.Bounds = growMonsterRefBounds(row.Bounds, placed.X, placed.Y)
			row.Bounds = growMonsterRefBounds(row.Bounds, placed.X+placed.Width-1, placed.Y+placed.Height-1)
		}
	}
	for _, point := range catalog.Points {
		def, ok := definitions[point.ModelID]
		if !ok {
			continue
		}
		areaID, floorID, position, ok := projectMonsterRefPoint(profile, point)
		if !ok {
			continue
		}
		row := get(def, areaID, floorID)
		row.PointCount++
		row.Bounds = growMonsterRefBounds(row.Bounds, position.TileX, position.TileY)
	}
	result := make([]monsterRefSearchRow, 0, len(rows))
	for _, row := range rows {
		if row.Bounds != nil {
			result = append(result, *row)
		}
	}
	if q != "" {
		for _, def := range definitions {
			placed := false
			for _, row := range result {
				if row.ModelID == def.ModelID {
					placed = true
					break
				}
			}
			if !placed {
				result = append(result, monsterRefSearchRow{ModelID: def.ModelID, Code: def.Code, Name: def.Name, Level: def.Level, LocationStatus: "unplaced"})
			}
		}
	}
	sort.Slice(result, func(i, j int) bool {
		left, right := result[i], result[j]
		if left.Name != right.Name {
			return left.Name < right.Name
		}
		if left.Code != right.Code {
			return left.Code < right.Code
		}
		if left.AreaID != right.AreaID {
			return left.AreaID < right.AreaID
		}
		if left.FloorID != right.FloorID {
			return left.FloorID < right.FloorID
		}
		return left.ModelID < right.ModelID
	})
	total := len(result)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "available", "dataset_id": catalog.DatasetID, "results": result[offset:end], "total": total, "next_offset": func() *int {
		if end < total {
			return &end
		}
		return nil
	}(), "coverage": catalog.Coverage})
}

func (h *mapHandler) monsterReferenceOverlay(w http.ResponseWriter, r *http.Request) {
	catalog, profile, ok := h.monsterReferenceScope(w, r)
	if !ok {
		return
	}
	q, minLevel, maxLevel, limit, offset, valid := monsterRefFilter(r)
	if !valid {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid monster reference filter"})
		return
	}
	areaID, floorID := r.URL.Query().Get("area"), r.URL.Query().Get("floor")
	if !validateAreaFloor(profile, areaID, floorID) {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid area or floor"})
		return
	}
	grid, _ := monsterRefTileCatalog(profile, areaID, floorID)
	bounds := monsterRefBounds{grid.MinX, grid.MaxX, grid.MinY, grid.MaxY}
	for _, field := range []struct {
		name   string
		target *int
	}{{"min_x", &bounds.MinX}, {"max_x", &bounds.MaxX}, {"min_y", &bounds.MinY}, {"max_y", &bounds.MaxY}} {
		if raw := r.URL.Query().Get(field.name); raw != "" {
			value, err := strconv.Atoi(raw)
			if err != nil {
				respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid viewport"})
				return
			}
			*field.target = value
		}
	}
	if bounds.MinX > bounds.MaxX || bounds.MinY > bounds.MaxY || bounds.MaxX-bounds.MinX > 256 || bounds.MaxY-bounds.MinY > 256 {
		respondJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid viewport"})
		return
	}
	includeAreas := r.URL.Query().Get("areas") == "1"
	includePoints := r.URL.Query().Get("points") == "1"
	definitions := make(map[int64]resources.MonsterDefinition)
	for _, def := range catalog.Definitions {
		if matchMonsterRef(def, q, minLevel, maxLevel) {
			definitions[def.ModelID] = def
		}
	}
	areas := make([]monsterRefArea, 0)
	areaCellTotal := 0
	if includeAreas {
		for _, area := range catalog.Areas {
			def, exists := definitions[area.ModelID]
			if !exists {
				continue
			}
			a, f, known := monsterRefAreaFloor(area.Group)
			if !known || a != areaID || f != floorID {
				continue
			}
			cells := make([]resources.MonsterGuideCell, 0)
			for _, cell := range area.Cells {
				if cell.X < grid.MinX || cell.Y < grid.MinY || cell.X+cell.Width-1 > grid.MaxX || cell.Y+cell.Height-1 > grid.MaxY {
					continue
				}
				if bounds.intersects(cell) {
					cells = append(cells, cell)
				}
			}
			if len(cells) > 0 {
				areaCellTotal += len(cells)
				if areaCellTotal > 5000 {
					respondJSON(w, http.StatusRequestEntityTooLarge, map[string]string{"error": "viewport contains too many guide cells; zoom in"})
					return
				}
				areas = append(areas, monsterRefArea{area.ModelID, area.Code, def.Name, def.Level, area.Group, cells, area.Precision})
			}
		}
	}
	points := make([]monsterRefPoint, 0)
	if includePoints {
		for _, point := range catalog.Points {
			def, exists := definitions[point.ModelID]
			if !exists {
				continue
			}
			a, f, position, known := projectMonsterRefPoint(profile, point)
			if !known || a != areaID || f != floorID || !bounds.contains(position.TileX, position.TileY) {
				continue
			}
			points = append(points, monsterRefPoint{point.ModelID, point.Code, def.Name, def.Level, point.Region, position, point.Precision})
		}
	}
	total := len(points)
	if offset > total {
		offset = total
	}
	end := offset + limit
	if end > total {
		end = total
	}
	respondJSON(w, http.StatusOK, map[string]any{"status": "available", "dataset_id": catalog.DatasetID, "area_id": areaID, "floor_id": floorID,
		"areas": areas, "area_cell_total": areaCellTotal, "areas_complete": true, "points": points[offset:end], "point_total": total, "next_offset": func() *int {
			if end < total {
				return &end
			}
			return nil
		}(),
		"coverage": catalog.Coverage, "provenance": "installed client reference dataset; not live spawns"})
}
