package resources

import (
	"encoding/json"
	"math"
	"math/big"
	"strconv"
	"strings"
)

// API evidence is interpreted on the same item observation, never cached by slot.
// These fields were captured from phBot 20.1.1 with plugin evidence schema 2.
type apiOption struct{ id, value string }

func apiItemMap(item map[string]any, key string) ([]apiOption, string) {
	fields, _ := item["api_fields"].(map[string]any)
	shapes, _ := item["api_field_types"].(map[string]any)
	raw, exists := fields[key]
	if !exists {
		return nil, "not_observed"
	}
	shape, _ := shapes[key].(map[string]any)
	countText, countOK := shape["count"].(json.Number)
	countValue, countErr := parseUnsigned64(countText.String())
	count := int64(countValue)
	object, ok := raw.(map[string]any)
	if !ok || !countOK || countErr != nil || shape["type"] != "dict" || count < 0 || count > 32 {
		return nil, "invalid"
	}
	if count == 0 && len(object) == 0 {
		return []apiOption{}, "observed_empty"
	}
	entries, ok := object["mapping_entries"].([]any)
	if !ok || len(object) != 1 || int64(len(entries)) != count || count == 0 {
		return nil, "invalid"
	}
	out := make([]apiOption, 0, len(entries))
	for _, raw := range entries {
		entry, ok := raw.(map[string]any)
		id, idOK := entry["key"].(string)
		if !ok || !idOK || entry["key_type"] != "integer" {
			return nil, "invalid"
		}
		parsed, err := parseUnsigned64(id)
		if err != nil || parsed > 0xffffffff {
			return nil, "invalid"
		}
		var value string
		switch v := entry["value"].(type) {
		case json.Number:
			value = v.String()
		case string:
			value = v
		default:
			return nil, "invalid"
		}
		if _, err := parseUnsigned64(value); err != nil {
			return nil, "invalid"
		}
		out = append(out, apiOption{id, value})
	}
	return out, "observed"
}

func resolveAPIItemDetails(item, metadata map[string]any, options map[string]MagicOptionDefinition) map[string]any {
	ids, _ := metadata["type_ids"].([]any)
	if integer(item["api_evidence_version"]) != 2 || len(ids) < 3 || integer(ids[0]) != 3 || integer(ids[1]) != 1 {
		return nil
	}
	whites, whiteStatus := apiItemMap(item, "whites")
	blues, blueStatus := apiItemMap(item, "blues")
	if whiteStatus == "not_observed" && blueStatus == "not_observed" && !hasAPIStatFields(item) {
		return nil
	}
	details := map[string]any{
		"source": "phbot_api", "status": "partial", "definition_version": "phbot-20.1.1-api-v1",
		"absolute_stats_status": "unavailable",
		"percentages":           []any{}, "blues": []any{}, "whites_status": whiteStatus, "blues_status": blueStatus,
	}
	// The current value is a documented API field; maximum comes from the
	// 1.2.6 named getter field only when its type and value validate.
	if current, ok := item["durability"]; ok {
		var maximum any
		if value, ok := apiStatNumber(item, "max_durability"); ok && value > 0 && value == math.Trunc(value) {
			maximum = uint64(value)
		}
		details["durability"] = map[string]any{"current": current, "maximum": maximum}
	}
	mapping := verifiedAPIWhiteFields(integer(ids[2]))
	byID := map[string]uint64{}
	if whiteStatus == "observed" {
		valid := true
		for _, white := range whites {
			value, err := strconv.ParseUint(white.value, 10, 64)
			if _, duplicate := byID[white.id]; duplicate || err != nil || value > 100 {
				valid = false
				break
			}
			byID[white.id] = value
		}
		if !valid {
			details["whites_status"] = "invalid"
			byID = map[string]uint64{}
		} else if len(mapping) == 0 {
			details["whites_status"] = "family_mapping_unverified"
		} else {
			lines := []any{}
			for _, field := range mapping {
				if value, exists := byID[strconv.Itoa(int(field.Index))]; exists {
					lines = append(lines, map[string]any{"key": field.Key, "label": field.Label, "value": value})
				}
			}
			details["percentages"] = lines
			details["unresolved_white_count"] = len(byID) - len(lines)
		}
	}
	// Named API scalars remain useful when a getter does not expose whites.
	// Invalid white evidence never supplies a percentage to those scalars.
	details["stats"] = resolveAPIStats(item, metadata, byID, blues, options)
	if len(details["stats"].([]any)) > 0 {
		details["absolute_stats_status"] = "observed"
	}
	unresolved := 0
	resolved := []any{}
	for _, option := range blues {
		definition, exists := options[option.id]
		presentation, verified := verifiedAPIBlueDefinition(definition.Code)
		if !exists || !verified || option.id == "0" {
			unresolved++
		}
		if !verified {
			presentation = MagicOptionDefinition{Label: "Option " + option.id, Scale: 1}
			if exists && definition.Code != "" {
				presentation.Label = definition.Code
			}
		}
		resolved = append(resolved, map[string]any{
			"id": option.id, "label": presentation.Label, "raw_value": option.value,
			"unit": presentation.Unit, "scale": 1, "precision": 0, "code": definition.Code,
			"label_verified": verified,
		})
	}
	details["blues"] = resolved
	details["unresolved_blue_count"] = unresolved
	if unresolved > 0 {
		details["blues_status"] = "unavailable_definitions"
	}
	return details
}

// IDs below are API attribute IDs, NOT the packed variance bit positions.
// Enabled families match captured items and the operator's supplied screenshots.
// Other families remain explicit gaps until their meanings are cross-checked.
func verifiedAPIWhiteFields(family int64) []varianceField {
	switch family {
	case 2, 10: // Protector and light armor
		return []varianceField{{1, "phy_def_pwr", "Phy. def. pwr"}, {4, "parry_ratio", "Parry ratio"}, {2, "mag_def_pwr", "Mag. def. pwr"}, {9, "phy_reinforce", "Phy. reinforce"}, {10, "mag_reinforce", "Mag. reinforce"}, {0, "durability", "Durability"}}
	case 1, 3, 9, 11: // Garment, armor, robe, heavy armor
		return []varianceField{{9, "phy_def_pwr", "Phy. def. pwr"}, {4, "parry_ratio", "Parry ratio"}, {10, "mag_def_pwr", "Mag. def. pwr"}, {1, "phy_reinforce", "Phy. reinforce"}, {2, "mag_reinforce", "Mag. reinforce"}, {0, "durability", "Durability"}}
	case 4: // Shields
		return []varianceField{{9, "phy_def_pwr", "Phy. def. pwr"}, {5, "block_ratio", "Blocking rate"}, {10, "mag_def_pwr", "Mag. def. pwr"}, {1, "phy_reinforce", "Phy. reinforce"}, {2, "mag_reinforce", "Mag. reinforce"}, {0, "durability", "Durability"}}
	case 5, 12: // Accessories
		return []varianceField{{11, "phy_absorption", "Phy. absorption"}, {12, "mag_absorption", "Mag. absorption"}}
	case 6: // Weapons
		return []varianceField{{6, "phy_atk_pwr", "Phy. atk. pwr"}, {7, "mag_atk_pwr", "Mag. atk. pwr"}, {3, "hit_ratio", "Attack rate"}, {8, "critical_ratio", "Critical"}, {1, "phy_reinforce", "Phy. reinforce"}, {2, "mag_reinforce", "Mag. reinforce"}, {0, "durability", "Durability"}}
	default:
		return nil
	}
}

// Dataset ID -> exact code lookup happens before this semantic whitelist. These
// four meanings/scales were confirmed by captured API values and screenshots.
// Unknown codes are retained as diagnostics, never given guessed display labels.
func verifiedAPIBlueDefinition(code string) (MagicOptionDefinition, bool) {
	base := strings.TrimPrefix(code, "MATTR_")
	base = strings.TrimPrefix(base, "AVATAR_")
	base = strings.TrimSuffix(base, "_SET")
	base = strings.TrimSuffix(base, "_AVATAR")
	base = strings.TrimSuffix(base, "_3JOB")
	switch base {
	case "INT":
		return MagicOptionDefinition{Label: "Int increase", Scale: 1}, true
	case "STR":
		return MagicOptionDefinition{Label: "Str increase", Scale: 1}, true
	case "MP":
		return MagicOptionDefinition{Label: "MP increase", Scale: 1}, true
	case "HP":
		return MagicOptionDefinition{Label: "HP increase", Scale: 1}, true
	case "SOLID":
		return MagicOptionDefinition{Label: "Steady", Unit: " uses", Scale: 1}, true
	case "ER":
		return MagicOptionDefinition{Label: "Parry rate increase", Unit: "%", Scale: 1}, true
	case "HR":
		return MagicOptionDefinition{Label: "Attack rate increase", Unit: "%", Scale: 1}, true
	case "DUR":
		return MagicOptionDefinition{Label: "Durability increase", Unit: "%", Scale: 1}, true
	case "LUCK", "ATHANASIA", "ASTRAL":
		return MagicOptionDefinition{Label: map[string]string{"LUCK": "Lucky", "ATHANASIA": "Immortal", "ASTRAL": "Astral"}[base], Unit: " uses", Scale: 1}, true
	case "EVADE_BLOCK", "BLOCKRATE":
		return MagicOptionDefinition{Label: "Blocking rate", Scale: 1}, true
	case "EVADE_CRITICAL", "CRITICAL":
		return MagicOptionDefinition{Label: "Critical", Scale: 1}, true
	case "REPAIR":
		return MagicOptionDefinition{Label: "Able to use Advanced elixir.", Scale: 1}, true
	case "REINFORCE_ITEM":
		return MagicOptionDefinition{Label: "Advanced elixir is in effect", Scale: 1}, true
	case "RESIST_FROSTBITE":
		return MagicOptionDefinition{Label: "Freezing and frostbite duration reduction", Unit: "%", Scale: 1}, true
	case "RESIST_ESHOCK":
		return MagicOptionDefinition{Label: "Electric shock duration reduction", Unit: "%", Scale: 1}, true
	case "RESIST_BURN":
		return MagicOptionDefinition{Label: "Burn duration reduction", Unit: "%", Scale: 1}, true
	case "RESIST_POISON":
		return MagicOptionDefinition{Label: "Poison duration reduction", Unit: "%", Scale: 1}, true
	case "RESIST_ZOMBIE":
		return MagicOptionDefinition{Label: "Zombie duration reduction", Unit: "%", Scale: 1}, true
	case "RESIST_STUN", "RESIST_SLEEP", "RESIST_FEAR", "RESIST_DISEASE", "RESIST_CSMP":
		return MagicOptionDefinition{Label: readableOptionCode(code) + " reduction", Unit: "%", Scale: 1}, true
	case "HPRG":
		return MagicOptionDefinition{Label: "HP recovery", Unit: "%", Scale: 1}, true
	case "MPRG":
		return MagicOptionDefinition{Label: "MP recovery", Unit: "%", Scale: 1}, true
	case "DEC_MAXDUR":
		return MagicOptionDefinition{Label: "Maximum durability reduction", Unit: "%", Scale: 1}, true
	default:
		return MagicOptionDefinition{}, false
	}
}

func readableOptionCode(code string) string {
	base := strings.TrimPrefix(code, "MATTR_")
	base = strings.ReplaceAll(base, "_", " ")
	return strings.Title(strings.ToLower(base))
}

func hasAPIStatFields(item map[string]any) bool {
	fields, _ := item["api_fields"].(map[string]any)
	for _, key := range []string{"phys_def", "mag_def", "parry", "block", "critical", "attack_rate", "phys_atk_min", "mag_atk_min", "phys_reinf_min", "mag_reinf_min", "phys_absorb_min", "mag_absorb_min", "max_durability"} {
		if _, ok := fields[key]; ok {
			return true
		}
	}
	return false
}

func apiStatNumber(item map[string]any, key string) (float64, bool) {
	fields, _ := item["api_fields"].(map[string]any)
	shapes, _ := item["api_field_types"].(map[string]any)
	shape, _ := shapes[key].(map[string]any)
	kind := shape["type"]
	if kind != "integer" && kind != "number" {
		return 0, false
	}
	value, ok := fields[key].(json.Number)
	if !ok {
		return 0, false
	}
	n, err := strconv.ParseFloat(value.String(), 64)
	return n, err == nil && !math.IsNaN(n) && !math.IsInf(n, 0) && n >= 0 && n <= 1e9 && (kind != "integer" || n == math.Trunc(n))
}

func oneDecimal(value float64) string {
	return strconv.FormatFloat(math.Floor(value*10+0.5)/10, 'f', 1, 64)
}

func referenceRoll(meta map[string]any, key string, plus int64, quality uint64) (float64, bool) {
	refs, _ := meta["reference_stats"].(map[string]any)
	r, _ := refs[key].(map[string]any)
	minText, a := r["min"].(string)
	maxText, b := r["max"].(string)
	if !a || !b || plus < 0 || plus > 255 || quality > 100 {
		return 0, false
	}
	lo, ok1 := new(big.Rat).SetString(minText)
	hi, ok2 := new(big.Rat).SetString(maxText)
	if !ok1 || !ok2 || lo.Cmp(hi) > 0 {
		return 0, false
	}
	delta := new(big.Rat).Sub(hi, lo)
	result := new(big.Rat).Add(lo, new(big.Rat).Mul(delta, big.NewRat(int64(quality), 100)))
	if stepText, exists := r["increment"].(string); exists {
		step, ok := new(big.Rat).SetString(stepText)
		if !ok {
			return 0, false
		}
		result.Add(result, new(big.Rat).Mul(step, big.NewRat(plus, 1)))
	} else if plus > 0 && (key == "phy_def_pwr" || key == "mag_def_pwr") {
		return 0, false
	}
	value, _ := result.Float64()
	return value, value >= 0 && value <= 1e9
}

func resolveAPIStats(item, meta map[string]any, whites map[string]uint64, blues []apiOption, options map[string]MagicOptionDefinition) []any {
	ids, _ := meta["type_ids"].([]any)
	if len(ids) < 3 {
		return []any{}
	}
	family := integer(ids[2])
	plus := integer(item["plus"])
	quality := map[string]uint64{}
	for _, mapping := range verifiedAPIWhiteFields(family) {
		if value, exists := whites[strconv.Itoa(int(mapping.Index))]; exists {
			quality[mapping.Key] = value
		}
	}
	lines := []any{}
	add := func(key, label, value string) {
		line := map[string]any{"key": key, "label": label, "value": value}
		if percent, exists := quality[key]; exists {
			line["percent"] = percent
		}
		lines = append(lines, line)
	}
	addScalar := func(key, label, source string) {
		if n, ok := apiStatNumber(item, source); ok {
			add(key, label, strconv.FormatUint(uint64(n), 10))
		}
	}
	addRange := func(key, label, low, high string, decimal bool) {
		a, okA := apiStatNumber(item, low)
		b, okB := apiStatNumber(item, high)
		if !okA || !okB || a > b {
			return
		}
		format := func(n float64) string {
			if decimal {
				return oneDecimal(n)
			}
			return strconv.FormatUint(uint64(n), 10)
		}
		unit := ""
		if key == "phy_reinforce" || key == "mag_reinforce" {
			unit = "%"
		}
		value := format(a) + unit
		if math.Abs(a-b) > 0.00001 {
			value += " ~ " + format(b) + unit
		}
		add(key, label, value)
	}
	switch family {
	case 1, 2, 3, 9, 10, 11, 4:
		for _, entry := range []struct{ key, label, raw string }{{"phy_def_pwr", "Phy. def. pwr", "phys_def"}, {"mag_def_pwr", "Mag. def. pwr", "mag_def"}} {
			raw, observed := apiStatNumber(item, entry.raw)
			percent, hasPercent := quality[entry.key]
			rolled, ok := referenceRoll(meta, entry.key, plus, percent)
			if observed && hasPercent && ok {
				add(entry.key, entry.label, oneDecimal(rolled))
			} else if observed {
				add(entry.key, entry.label, strconv.FormatUint(uint64(raw), 10))
			}
		}
		if family == 4 {
			addScalar("block_ratio", "Blocking rate", "block")
		} else {
			if parry, ok := apiStatNumber(item, "parry"); ok {
				for _, option := range blues {
					if def, exists := options[option.id]; exists && (def.Code == "MATTR_ER" || def.Code == "MATTR_ER_SET") {
						bonus, _ := strconv.ParseUint(option.value, 10, 64)
						if bonus <= 100 {
							parry = math.Floor(parry * (1 + float64(bonus)/100))
						}
					}
				}
				add("parry_ratio", "Parry ratio", strconv.FormatUint(uint64(parry), 10))
			}
		}
		addRange("phy_reinforce", "Phy. reinforce", "phys_reinf_min", "phys_reinf_max", true)
		addRange("mag_reinforce", "Mag. reinforce", "mag_reinf_min", "mag_reinf_max", true)
	case 6:
		addRange("phy_atk_pwr", "Phy. atk. pwr", "phys_atk_min", "phys_atk_max", false)
		addRange("mag_atk_pwr", "Mag. atk. pwr", "mag_atk_min", "mag_atk_max", false)
		addScalar("hit_ratio", "Attack rate", "attack_rate")
		addScalar("critical_ratio", "Critical", "critical")
		addRange("phy_reinforce", "Phy. reinforce", "phys_reinf_min", "phys_reinf_max", true)
		addRange("mag_reinforce", "Mag. reinforce", "mag_reinf_min", "mag_reinf_max", true)
	case 5, 12:
		addRange("phy_absorption", "Phy. absorption", "phys_absorb_min", "phys_absorb_max", true)
		addRange("mag_absorption", "Mag. absorption", "mag_absorb_min", "mag_absorb_max", true)
	}
	return lines
}
