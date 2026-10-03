package resources

import (
	"encoding/json"
	"fmt"
	"os"
	"regexp"
	"strings"
	"unicode/utf8"
)

// UniqueMonster is one client unique model and its published portrait.
// Several model IDs can share one picture when they use the same figure.
type UniqueMonster struct {
	ModelID  int64  `json:"model_id"`
	Code     string `json:"code"`
	Name     string `json:"name"`
	Level    *int   `json:"level,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
}

type uniqueCatalogFile struct {
	DatasetID string          `json:"dataset_id"`
	Records   []UniqueMonster `json:"records"`
}

// UniqueCatalog resolves unique notices for one exported dataset.
type UniqueCatalog struct {
	DatasetID string
	byModel   map[int64]UniqueMonster
	byName    map[string][]UniqueMonster
}

type UniqueInfo struct {
	Name     string `json:"name,omitempty"`
	Level    *int   `json:"level,omitempty"`
	ImageURL string `json:"image_url,omitempty"`
	Notice   string `json:"notice,omitempty"`
	Killer   string `json:"killer,omitempty"`
	ModelID  int64  `json:"model_id,omitempty"`
}

var uniqueImageURL = regexp.MustCompile(`^/game-assets/monsters/[a-z0-9][a-z0-9_-]{0,127}\.png$`)
var uniqueCode = regexp.MustCompile(`^MOB_[A-Za-z0-9_]+$`)

func loadUniqueCatalog(path string) (*UniqueCatalog, error) {
	stat, err := os.Stat(path)
	if os.IsNotExist(err) {
		return nil, nil
	}
	if err != nil {
		return nil, err
	}
	if stat.Size() > 2<<20 {
		return nil, fmt.Errorf("unique monster catalog exceeds size limit")
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return nil, err
	}
	var file uniqueCatalogFile
	if err := json.Unmarshal(data, &file); err != nil {
		return nil, err
	}
	if !datasetName.MatchString(file.DatasetID) || len(file.Records) == 0 || len(file.Records) > 5000 {
		return nil, fmt.Errorf("invalid unique monster catalog identity or size")
	}
	catalog := &UniqueCatalog{
		DatasetID: file.DatasetID,
		byModel:   make(map[int64]UniqueMonster, len(file.Records)),
		byName:    map[string][]UniqueMonster{},
	}
	for _, item := range file.Records {
		if item.ModelID < 1 || item.ModelID > 4294967295 ||
			(item.Code != "" && !uniqueCode.MatchString(item.Code)) ||
			item.Name != strings.TrimSpace(item.Name) || utf8.RuneCountInString(item.Name) > 128 ||
			strings.ContainsRune(item.Name, 0) ||
			(item.ImageURL != "" && !uniqueImageURL.MatchString(item.ImageURL)) ||
			item.Level != nil && (*item.Level < 1 || *item.Level > 255) ||
			item.Name == "" && item.ImageURL == "" {
			return nil, fmt.Errorf("invalid unique monster definition")
		}
		if _, exists := catalog.byModel[item.ModelID]; exists {
			return nil, fmt.Errorf("duplicate unique monster model")
		}
		catalog.byModel[item.ModelID] = item
		if item.Name != "" {
			key := strings.ToLower(item.Name)
			catalog.byName[key] = append(catalog.byName[key], item)
		}
	}
	return catalog, nil
}

func (s *Store) UniqueInfo(server string, payload []byte) *UniqueInfo {
	if s == nil || s.metadata == nil {
		return nil
	}
	return s.metadata.UniqueInfo(server, payload)
}

func (m *ItemMetadata) uniqueCatalog(server string) *UniqueCatalog {
	if m == nil {
		return nil
	}
	dataset, ok := m.DatasetForServer(server)
	if !ok {
		return nil
	}
	return m.UniqueCatalogs[dataset]
}

// UniqueInfo resolves a stored unique occurrence against the dataset catalog.
// A model match wins. A callback that only carries the monster name is used
// when every matching unique shares the same picture and level.
func (m *ItemMetadata) UniqueInfo(server string, payload []byte) *UniqueInfo {
	catalog := m.uniqueCatalog(server)
	if catalog == nil {
		return nil
	}
	var fields struct {
		Model  *int64 `json:"model"`
		Notice string `json:"notice"`
		Killer string `json:"killer"`
		Value  string `json:"value"`
	}
	if json.Unmarshal(payload, &fields) != nil {
		return nil
	}
	info := &UniqueInfo{Notice: fields.Notice, Killer: strings.TrimSpace(fields.Killer)}
	if info.Notice != "spawn" && info.Notice != "kill" {
		info.Notice = "spawn"
	}
	if fields.Model != nil {
		if item, ok := catalog.byModel[*fields.Model]; ok {
			return applyUniqueMonster(info, item)
		}
	}
	name := strings.TrimSpace(fields.Value)
	matches := catalog.byName[strings.ToLower(name)]
	if name == "" || len(matches) == 0 {
		if name != "" {
			info.Name = name
			return info
		}
		return nil
	}
	info.Name = matches[0].Name
	if sharedUniqueImage(matches) {
		info.ImageURL = matches[0].ImageURL
	}
	if sharedUniqueLevel(matches) {
		info.Level = matches[0].Level
	}
	if len(matches) == 1 {
		info.ModelID = matches[0].ModelID
		info.ImageURL = matches[0].ImageURL
		info.Level = matches[0].Level
	}
	return info
}

func applyUniqueMonster(info *UniqueInfo, item UniqueMonster) *UniqueInfo {
	info.ModelID = item.ModelID
	info.Name = item.Name
	info.Level = item.Level
	info.ImageURL = item.ImageURL
	if info.Name == "" {
		info.Name = item.Code
	}
	return info
}

func sharedUniqueImage(items []UniqueMonster) bool {
	if len(items) == 0 || items[0].ImageURL == "" {
		return false
	}
	for _, item := range items[1:] {
		if item.ImageURL != items[0].ImageURL {
			return false
		}
	}
	return true
}

func sharedUniqueLevel(items []UniqueMonster) bool {
	if len(items) == 0 || items[0].Level == nil {
		return false
	}
	for _, item := range items[1:] {
		if item.Level == nil || *item.Level != *items[0].Level {
			return false
		}
	}
	return true
}
