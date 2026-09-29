package resources

import (
	"bytes"
	"encoding/json"
	"fmt"
	"math/big"
	"os"
	"path/filepath"
	"reflect"
	"regexp"
	"strconv"
	"strings"
)

// Static SRO item definitions are shared by stable item code across servers.
// Server profiles still resolve numeric model IDs; live item observations stay
// attached to the item that phBot reported.
type ItemMetadata struct {
	Servers             map[string]string
	Catalogs            map[string]ItemCatalog
	SharedIcons         map[string]string
	SharedPresentations map[string]map[string]any
	SharedMagicOptions  map[string]MagicOptionDefinition
}

// DatasetForServer returns the selected exported profile using the same
// normalized server mapping used by item reference lookups.
func (m *ItemMetadata) DatasetForServer(server string) (string, bool) {
	if m == nil {
		return "", false
	}
	dataset, ok := m.Servers[strings.ToLower(strings.TrimSpace(server))]
	return dataset, ok
}

type ItemCatalog struct {
	DatasetID          string                           `json:"dataset_id"`
	Items              map[string]ItemDefinition        `json:"items"`
	MagicOptions       map[string]MagicOptionDefinition `json:"magic_options,omitempty"`
	CharacterPortraits map[string]CharacterPortrait     `json:"character_portraits,omitempty"`
}

type CharacterPortrait struct {
	Code        string `json:"code"`
	PortraitURL string `json:"portrait_url"`
}
type ItemDefinition struct {
	Code         string         `json:"code"`
	Presentation map[string]any `json:"presentation"`
}
type MagicOptionDefinition struct {
	Label     string             `json:"label"`
	Code      string             `json:"code,omitempty"`
	Unit      string             `json:"unit,omitempty"`
	Scale     uint32             `json:"scale,omitempty"`
	Precision uint8              `json:"precision,omitempty"`
	Level     uint8              `json:"level,omitempty"`
	RawRanges []MagicOptionRange `json:"raw_ranges,omitempty"`
}
type MagicOptionRange struct {
	Minimum string `json:"minimum"`
	Maximum string `json:"maximum"`
}

var datasetName = regexp.MustCompile(`^gamedata-[a-z0-9]{1,64}$`)
var referenceDecimal = regexp.MustCompile(`^(?:0|[1-9][0-9]*)(?:\.[0-9]{1,12})?$`)
var unsignedDecimal = regexp.MustCompile(`^(?:0|[1-9][0-9]{0,19})$`)

func LoadItemMetadata(directory string) (*ItemMetadata, error) {
	m := &ItemMetadata{
		Servers:             map[string]string{},
		Catalogs:            map[string]ItemCatalog{},
		SharedIcons:         map[string]string{},
		SharedPresentations: map[string]map[string]any{},
		SharedMagicOptions:  map[string]MagicOptionDefinition{},
	}
	ambiguousIcons := map[string]bool{}
	ambiguousPresentations := map[string]map[string]bool{}
	ambiguousOptions := map[string]bool{}
	read := func(path string, target any) error {
		info, err := os.Stat(path)
		if err != nil {
			return err
		}
		if info.Size() > 16<<20 {
			return fmt.Errorf("item metadata exceeds size limit")
		}
		data, err := os.ReadFile(path)
		if err != nil {
			return err
		}
		decoder := json.NewDecoder(bytes.NewReader(data))
		decoder.UseNumber()
		return decoder.Decode(target)
	}
	if err := read(filepath.Join(directory, "servers.json"), &m.Servers); err != nil {
		return nil, err
	}
	for server, dataset := range m.Servers {
		if server == "" || server != strings.ToLower(strings.TrimSpace(server)) || !datasetName.MatchString(dataset) {
			return nil, fmt.Errorf("invalid server dataset mapping")
		}
		if _, ok := m.Catalogs[dataset]; ok {
			continue
		}
		var catalog ItemCatalog
		if err := read(filepath.Join(directory, dataset+".json"), &catalog); err != nil {
			return nil, err
		}
		if catalog.DatasetID != dataset || len(catalog.Items) > 100000 {
			return nil, fmt.Errorf("invalid item dataset")
		}
		for _, item := range catalog.Items {
			if len(item.Code) == 0 || len(item.Code) > 256 {
				return nil, fmt.Errorf("invalid item code")
			}
			addSharedPresentation(m.SharedPresentations, ambiguousPresentations, item.Code, item.Presentation)
			if icon, ok := item.Presentation["icon_url"]; ok {
				url, valid := icon.(string)
				if !valid || !strings.HasPrefix(url, "/game-assets/") || strings.ContainsAny(url, "\\?#") || strings.Contains(url, "..") {
					return nil, fmt.Errorf("invalid local item icon")
				}
				if !ambiguousIcons[item.Code] {
					if existing, exists := m.SharedIcons[item.Code]; exists && existing != url {
						delete(m.SharedIcons, item.Code)
						ambiguousIcons[item.Code] = true
					} else {
						m.SharedIcons[item.Code] = url
					}
				}
			}
			if referenceStats, ok := item.Presentation["reference_stats"]; ok && !validReferenceStats(referenceStats) {
				return nil, fmt.Errorf("invalid item reference stats")
			}
		}
		for model, portrait := range catalog.CharacterPortraits {
			id, err := strconv.ParseUint(model, 10, 32)
			if err != nil || id == 0 || strconv.FormatUint(id, 10) != model || portrait.Code == "" || len(portrait.Code) > 128 || strings.TrimSpace(portrait.Code) != portrait.Code || !strings.HasPrefix(portrait.Code, "CHAR_") || !validPortraitURL(portrait.PortraitURL) {
				return nil, fmt.Errorf("invalid character portrait definition")
			}
		}
		for id, option := range catalog.MagicOptions {
			parsed, err := strconv.ParseUint(id, 10, 32)
			if err != nil || parsed == 0 || len(option.Code) > 128 || strings.TrimSpace(option.Code) != option.Code || strings.TrimSpace(option.Label) != option.Label || len(option.Label) > 96 || len(option.Unit) > 24 || option.Precision > 6 || option.Scale > 1000000 || len(option.RawRanges) > 3 {
				return nil, fmt.Errorf("invalid magic option definition")
			}
			for _, valueRange := range option.RawRanges {
				minimum, minErr := strconv.ParseUint(valueRange.Minimum, 10, 16)
				maximum, maxErr := strconv.ParseUint(valueRange.Maximum, 10, 16)
				if minErr != nil || maxErr != nil || minimum > maximum {
					return nil, fmt.Errorf("invalid magic option raw range")
				}
			}
			if !ambiguousOptions[id] {
				if existing, exists := m.SharedMagicOptions[id]; exists && !reflect.DeepEqual(existing, option) {
					delete(m.SharedMagicOptions, id)
					ambiguousOptions[id] = true
				} else {
					m.SharedMagicOptions[id] = option
				}
			}
		}
		m.Catalogs[dataset] = catalog
	}
	return m, nil
}

var localCharacterPortrait = regexp.MustCompile(`^/game-assets/interface/character/char_(?:ch|eu)_(?:man|woman)(?:[1-9]|1[0-3])\.png$`)

func validPortraitURL(value string) bool {
	return len(value) <= 160 && localCharacterPortrait.MatchString(value)
}

func (m *ItemMetadata) PortraitURL(server string, model *int64) string {
	if m == nil || model == nil || *model < 1 || *model > 4294967295 {
		return ""
	}
	dataset := m.Servers[strings.ToLower(strings.TrimSpace(server))]
	if dataset == "" {
		return ""
	}
	portrait, ok := m.Catalogs[dataset].CharacterPortraits[strconv.FormatInt(*model, 10)]
	if !ok || !validPortraitURL(portrait.PortraitURL) {
		return ""
	}
	return portrait.PortraitURL
}

// ItemPresentation resolves static data using the server's model mapping. A
// model from an unmapped server is never assumed to identify the same item.
func (m *ItemMetadata) ItemPresentation(server string, model *int64, code string) map[string]any {
	if m == nil {
		return nil
	}
	dataset := m.Servers[strings.ToLower(strings.TrimSpace(server))]
	if dataset != "" && model != nil && *model >= 0 {
		if definition, ok := m.Catalogs[dataset].Items[strconv.FormatInt(*model, 10)]; ok && (code == "" || code == definition.Code) {
			result := make(map[string]any, len(definition.Presentation)+1)
			for key, value := range definition.Presentation {
				result[key] = value
			}
			result["dataset_id"] = dataset
			return result
		}
	}
	if code != "" {
		if presentation, ok := m.SharedPresentations[code]; ok {
			result := make(map[string]any, len(presentation))
			for key, value := range presentation {
				result[key] = value
			}
			return result
		}
	}
	return nil
}

// MapItemPresentation uses the selected server's model catalog first. A stable
// item code is used only when its shared presentation is unambiguous.
func (m *ItemMetadata) MapItemPresentation(server string, model *int64, code string) (name, iconURL string) {
	if m == nil {
		return "", ""
	}
	dataset := m.Servers[strings.ToLower(strings.TrimSpace(server))]
	if dataset == "" {
		return "", ""
	}
	if model != nil && *model > 0 && *model <= 4294967295 {
		if item, ok := m.Catalogs[dataset].Items[strconv.FormatInt(*model, 10)]; ok {
			name, _ = item.Presentation["name"].(string)
			iconURL, _ = item.Presentation["icon_url"].(string)
			return name, iconURL
		}
	}
	if code == "" {
		return "", ""
	}
	name, _ = m.SharedPresentations[code]["name"].(string)
	iconURL = m.SharedIcons[code]
	return name, iconURL
}

func addSharedPresentation(index map[string]map[string]any, ambiguous map[string]map[string]bool, code string, presentation map[string]any) {
	if code == "" || len(presentation) == 0 {
		return
	}
	shared := index[code]
	if shared == nil {
		shared = map[string]any{}
		index[code] = shared
	}
	conflicts := ambiguous[code]
	for key, value := range presentation {
		if conflicts != nil && conflicts[key] {
			continue
		}
		if existing, exists := shared[key]; exists && !reflect.DeepEqual(existing, value) {
			delete(shared, key)
			if conflicts == nil {
				conflicts = map[string]bool{}
				ambiguous[code] = conflicts
			}
			conflicts[key] = true
			continue
		}
		shared[key] = value
	}
}

func validReferenceStats(raw any) bool {
	stats, ok := raw.(map[string]any)
	if !ok || len(stats) == 0 || len(stats) > 16 {
		return false
	}
	for _, value := range stats {
		fields, ok := value.(map[string]any)
		if !ok || len(fields) < 2 || len(fields) > 3 {
			return false
		}
		minimum, minOK := fields["min"].(string)
		maximum, maxOK := fields["max"].(string)
		if !minOK || !maxOK || !validReferenceDecimal(minimum) || !validReferenceDecimal(maximum) {
			return false
		}
		minValue, _ := new(big.Rat).SetString(minimum)
		maxValue, _ := new(big.Rat).SetString(maximum)
		if minValue.Cmp(maxValue) > 0 {
			return false
		}
		if increment, exists := fields["increment"]; exists {
			text, ok := increment.(string)
			if !ok || !validReferenceDecimal(text) {
				return false
			}
		}
		for key := range fields {
			if key != "min" && key != "max" && key != "increment" {
				return false
			}
		}
	}
	return true
}

func validReferenceDecimal(value string) bool {
	if len(value) == 0 || len(value) > 32 || !referenceDecimal.MatchString(value) {
		return false
	}
	parsed, ok := new(big.Rat).SetString(value)
	if !ok || parsed.Sign() < 0 {
		return false
	}
	return parsed.Cmp(big.NewRat(1000000000, 1)) <= 0
}

func (m *ItemMetadata) enrich(server string, payload json.RawMessage) json.RawMessage {
	if m == nil || len(m.Catalogs) == 0 && len(m.SharedIcons) == 0 {
		return payload
	}
	dataset := m.Servers[strings.ToLower(strings.TrimSpace(server))]
	catalog := m.Catalogs[dataset]
	var root map[string]any
	decoder := json.NewDecoder(bytes.NewReader(payload))
	decoder.UseNumber()
	if decoder.Decode(&root) != nil {
		return payload
	}
	enrichSlots := func(value any) {
		slots, _ := value.([]any)
		for _, value := range slots {
			slot, _ := value.(map[string]any)
			item, _ := slot["item"].(map[string]any)
			if item == nil {
				continue
			}
			code, _ := item["servername"].(string)
			var metadata map[string]any
			var definition ItemDefinition
			matchedServerDefinition := false
			if dataset != "" {
				if model, ok := item["model"].(json.Number); ok {
					definition, matchedServerDefinition = catalog.Items[model.String()]
					// A conflicting code is evidence that this is the wrong dataset/model.
					if code != "" && code != definition.Code {
						matchedServerDefinition = false
					}
				}
			}
			if matchedServerDefinition {
				metadata = make(map[string]any, len(definition.Presentation)+2)
				for key, value := range definition.Presentation {
					metadata[key] = value
				}
			} else if code != "" {
				if presentation, ok := m.SharedPresentations[code]; ok {
					metadata = make(map[string]any, len(presentation))
					for key, value := range presentation {
						metadata[key] = value
					}
				}
			}
			if metadata == nil {
				continue
			}
			if matchedServerDefinition {
				metadata["dataset_id"] = dataset
			}
			item["metadata"] = metadata
			if matchedServerDefinition || code != "" {
				options := m.SharedMagicOptions
				if matchedServerDefinition {
					options = catalog.MagicOptions
				}
				if details := resolveAPIItemDetails(item, metadata, options); details != nil {
					item["instance_details"] = details
				} else if instance, ok := item["instance"].(map[string]any); ok {
					item["instance_details"] = resolveInstanceDetails(instance, metadata, options)
				}
			}
		}
	}
	enrichSlots(root["slots"])
	if pets, ok := root["pets"].([]any); ok {
		for _, value := range pets {
			if pet, ok := value.(map[string]any); ok {
				enrichSlots(pet["slots"])
			}
		}
	}
	result, err := json.Marshal(root)
	if err != nil {
		return payload
	}
	return result
}

type varianceField struct {
	Index uint8
	Key   string
	Label string
}

func resolveInstanceDetails(instance, metadata map[string]any, options map[string]MagicOptionDefinition) map[string]any {
	details := map[string]any{
		"status":                "partial",
		"absolute_stats_status": "unavailable",
		"absolute_stats_reason": "reference_formula_not_verified",
		"percentages":           []any{},
		"blues":                 []any{},
	}
	if instance["availability"] != "observed" || instance["source"] != "vsro_1188_packet" {
		details["status"] = "unavailable"
		details["reason"] = "unverified_instance_source"
		return details
	}
	if current, ok := instance["durability"]; ok {
		details["durability"] = map[string]any{"current": current, "maximum": nil, "status": "maximum_unavailable"}
	}
	if rawVariance, ok := instance["variance"].(string); ok {
		if variance, err := parseUnsigned64(rawVariance); err == nil {
			for _, field := range varianceFields(metadata) {
				roll := (variance >> (5 * field.Index)) & 31
				percentage := roll * 100 / 31
				percentages := details["percentages"].([]any)
				details["percentages"] = append(percentages, map[string]any{
					"key": field.Key, "label": field.Label, "value": percentage,
					"roll": roll,
				})
			}
		} else {
			details["variance_status"] = "invalid_unsigned_64_bit_value"
		}
	} else {
		details["variance_status"] = "not_observed"
	}
	resolved, unresolved := resolveBlueOptions(instance, options)
	details["blues"] = resolved
	details["unresolved_blue_count"] = unresolved
	if len(resolved) > 0 {
		details["blues_status"] = "partial"
	} else if instance["magic_options_availability"] == "observed" {
		details["blues_status"] = "unavailable_definitions"
	} else {
		details["blues_status"] = "not_observed"
	}
	return details
}

func varianceFields(metadata map[string]any) []varianceField {
	typeIDs, ok := metadata["type_ids"].([]any)
	if !ok || len(typeIDs) < 3 || integer(typeIDs[1]) != 1 {
		return nil
	}
	switch integer(typeIDs[2]) {
	case 1, 2, 3, 9, 10, 11:
		return []varianceField{{0, "durability", "Durability"}, {1, "phy_reinforce", "Phy. reinforce"}, {2, "mag_reinforce", "Mag. reinforce"}, {3, "phy_def_pwr", "Phy. def. pwr"}, {4, "mag_def_pwr", "Mag. def. pwr"}, {5, "parry_ratio", "Parry ratio"}}
	case 4:
		return []varianceField{{0, "durability", "Durability"}, {1, "phy_reinforce", "Phy. reinforce"}, {2, "mag_reinforce", "Mag. reinforce"}, {3, "block_ratio", "Block ratio"}, {4, "phy_def_pwr", "Phy. def. pwr"}, {5, "mag_def_pwr", "Mag. def. pwr"}}
	case 5, 12:
		return []varianceField{{0, "phy_absorption", "Phy. absorption"}, {1, "mag_absorption", "Mag. absorption"}}
	case 6:
		return []varianceField{{0, "durability", "Durability"}, {1, "phy_reinforce", "Phy. reinforce"}, {2, "mag_reinforce", "Mag. reinforce"}, {3, "hit_ratio", "Attack rating"}, {4, "phy_atk_pwr", "Phy. atk. pwr"}, {5, "mag_atk_pwr", "Mag. atk. pwr"}, {6, "critical_ratio", "Critical"}}
	default:
		return nil
	}
}

func integer(value any) int64 {
	switch number := value.(type) {
	case json.Number:
		parsed, _ := strconv.ParseInt(number.String(), 10, 64)
		return parsed
	case int:
		return int64(number)
	case int64:
		return number
	default:
		return -1
	}
}

func resolveBlueOptions(instance map[string]any, definitions map[string]MagicOptionDefinition) ([]any, int) {
	values, ok := instance["magic_options"].([]any)
	if !ok {
		return []any{}, 0
	}
	resolved := make([]any, 0, len(values))
	unresolved := 0
	for _, raw := range values {
		entry, ok := raw.(map[string]any)
		if !ok {
			unresolved++
			continue
		}
		id, ok := entry["id"].(string)
		definition, known := definitions[id]
		value, valid := entry["value"].(string)
		if !ok || !known || !valid || definition.Label == "" || definition.Scale == 0 {
			unresolved++
			continue
		}
		if optionID, err := parseUnsigned64(id); err != nil || optionID == 0 {
			unresolved++
			continue
		}
		if _, err := parseUnsigned64(value); err != nil {
			unresolved++
			continue
		}
		resolved = append(resolved, map[string]any{
			"id": id, "label": definition.Label, "raw_value": value,
			"unit": definition.Unit, "scale": definition.Scale,
			"precision": definition.Precision,
		})
	}
	return resolved, unresolved
}

func parseUnsigned64(value string) (uint64, error) {
	if !unsignedDecimal.MatchString(value) {
		return 0, strconv.ErrSyntax
	}
	return strconv.ParseUint(value, 10, 64)
}

func (s *Store) SetItemMetadata(metadata *ItemMetadata) { s.metadata = metadata }

func (s *Store) DatasetIDForServer(server string) (string, bool) {
	if s == nil {
		return "", false
	}
	return s.metadata.DatasetForServer(server)
}

func (s *Store) PortraitURL(server string, model *int64) string {
	if s == nil || s.metadata == nil {
		return ""
	}
	return s.metadata.PortraitURL(server, model)
}

func (s *Store) ItemPresentation(server string, model *int64, code string) map[string]any {
	if s == nil || s.metadata == nil {
		return nil
	}
	return s.metadata.ItemPresentation(server, model, code)
}

func (s *Store) MapItemPresentation(server string, model *int64, code string) (name, iconURL string) {
	if s == nil || s.metadata == nil {
		return "", ""
	}
	return s.metadata.MapItemPresentation(server, model, code)
}
