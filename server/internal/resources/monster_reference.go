package resources

import (
	"encoding/json"
	"fmt"
	"math"
	"os"
	"regexp"
	"strings"
)

type MonsterDefinition struct {
	ModelID int64  `json:"model_id"`
	Code    string `json:"code"`
	Name    string `json:"name"`
	Level   *int   `json:"level"`
	Enabled bool   `json:"enabled"`
}

type MonsterGuideCell struct {
	X      int `json:"x"`
	Y      int `json:"y"`
	Width  int `json:"width"`
	Height int `json:"height"`
}

type MonsterGuideArea struct {
	Code      string             `json:"code"`
	ModelID   int64              `json:"model_id"`
	Group     string             `json:"group"`
	Precision string             `json:"precision"`
	Cells     []MonsterGuideCell `json:"cells"`
}

type MonsterReferencePoint struct {
	ModelID        int64   `json:"model_id"`
	Code           string  `json:"code"`
	Region         int     `json:"region"`
	RawX           float64 `json:"raw_x"`
	RawY           float64 `json:"raw_y"`
	RawZ           float64 `json:"raw_z"`
	Precision      string  `json:"precision"`
	CoordinateKind string  `json:"coordinate_kind"`
}

type MonsterReference struct {
	CatalogVersion string                  `json:"catalogVersion"`
	DatasetID      string                  `json:"datasetId"`
	Family         string                  `json:"family"`
	Status         string                  `json:"status"`
	Definitions    []MonsterDefinition     `json:"records"`
	Areas          []MonsterGuideArea      `json:"areas"`
	Points         []MonsterReferencePoint `json:"points"`
	Coverage       map[string]int          `json:"coverage"`
	byModel        map[int64]MonsterDefinition
	byCode         map[string][]MonsterDefinition
}

var monsterCode = regexp.MustCompile(`^MOB_[A-Za-z0-9_]+$`)

func loadMonsterReference(path, dataset string) (*MonsterReference, error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stat.Size() > 32<<20 {
		return nil, fmt.Errorf("monster reference catalog exceeds size limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var catalog MonsterReference
	if err := json.Unmarshal(data, &catalog); err != nil {
		return nil, err
	}
	if catalog.DatasetID != dataset || catalog.Family != "monsterReference" || catalog.CatalogVersion == "" ||
		len(catalog.Definitions) > 100000 || len(catalog.Areas) > 100000 || len(catalog.Points) > 200000 {
		return nil, fmt.Errorf("invalid monster reference catalog identity or size")
	}
	catalog.byModel = make(map[int64]MonsterDefinition, len(catalog.Definitions))
	catalog.byCode = make(map[string][]MonsterDefinition)
	for _, item := range catalog.Definitions {
		if item.ModelID <= 0 || item.ModelID > 4294967295 || !monsterCode.MatchString(item.Code) ||
			len(item.Name) > 128 || strings.ContainsRune(item.Name, 0) || item.Level != nil && (*item.Level < 1 || *item.Level > 255) {
			return nil, fmt.Errorf("invalid monster definition")
		}
		if _, exists := catalog.byModel[item.ModelID]; exists {
			return nil, fmt.Errorf("duplicate monster model")
		}
		catalog.byModel[item.ModelID] = item
		if item.Enabled {
			catalog.byCode[item.Code] = append(catalog.byCode[item.Code], item)
		}
	}
	for _, area := range catalog.Areas {
		definition, ok := catalog.byModel[area.ModelID]
		if !ok || !definition.Enabled || definition.Code != area.Code || area.Precision != "guide-grid-cell" || len(area.Cells) > 2000 {
			return nil, fmt.Errorf("invalid monster guide area")
		}
		for _, cell := range area.Cells {
			if cell.X < 0 || cell.Y < 0 || cell.Width < 1 || cell.Height < 1 || cell.X+cell.Width > 256 || cell.Y+cell.Height > 256 {
				return nil, fmt.Errorf("invalid monster guide cell")
			}
		}
	}
	for _, point := range catalog.Points {
		definition, ok := catalog.byModel[point.ModelID]
		if !ok || !definition.Enabled || definition.Code != point.Code || point.Region == 0 || point.Region < -32768 || point.Region > 65535 ||
			point.Precision != "client-reference-point" ||
			!validReferenceNumber(point.RawX) || !validReferenceNumber(point.RawY) || !validReferenceNumber(point.RawZ) {
			return nil, fmt.Errorf("invalid monster reference point")
		}
	}
	return &catalog, nil
}

func validReferenceNumber(value float64) bool {
	return !math.IsNaN(value) && !math.IsInf(value, 0) && value >= -1000000 && value <= 1000000
}

func (m *ItemMetadata) MonsterReferenceForDataset(dataset string) *MonsterReference {
	if m == nil {
		return nil
	}
	return m.MonsterReferences[dataset]
}

// MonsterLevel uses numeric models only within their dataset. A supplied code
// must agree with the model; a code-only lookup must have exactly one target.
func (m *ItemMetadata) MonsterLevel(dataset string, model *int64, code string) (int, bool) {
	catalog := m.MonsterReferenceForDataset(dataset)
	if catalog == nil {
		return 0, false
	}
	var definition MonsterDefinition
	if model != nil {
		definition = catalog.byModel[*model]
		if !definition.Enabled || code != "" && definition.Code != code {
			return 0, false
		}
	} else {
		matches := catalog.byCode[code]
		if len(matches) != 1 {
			return 0, false
		}
		definition = matches[0]
	}
	if definition.Level == nil {
		return 0, false
	}
	return *definition.Level, true
}

func (s *Store) MonsterLevel(dataset string, model *int64, code string) (int, bool) {
	if s == nil || s.metadata == nil {
		return 0, false
	}
	return s.metadata.MonsterLevel(dataset, model, code)
}

func (s *Store) MonsterReferenceForServer(server string) (*MonsterReference, string, bool) {
	if s == nil || s.metadata == nil {
		return nil, "", false
	}
	dataset, ok := s.metadata.DatasetForServer(server)
	if !ok {
		return nil, "", false
	}
	return s.metadata.MonsterReferenceForDataset(dataset), dataset, true
}
