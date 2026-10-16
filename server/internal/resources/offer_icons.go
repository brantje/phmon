package resources

import (
	"regexp"
	"strconv"
	"strings"
	"sync"
)

var (
	offerGearCode = regexp.MustCompile(`^ITEM_(CH|EU)_([A-Z]+)_(\d{2})_[ABC](?:_RARE)?$`)
	offerSetCode  = regexp.MustCompile(`^ITEM_(CH|EU)_[MW]_(CLOTHES|LIGHT|HEAVY)_(\d{2})_(HA|SA|BA|LA|AA|FA)_[ABC](?:_RARE)?$`)
	offerIconMu   sync.Mutex
	offerIconIdx  = map[string]map[string]string{}
)

// offerWeaponCodes maps the parsed weapon subtype to its race and code segment.
var offerWeaponCodes = map[string][2]string{
	"staff": {"EU", "TSTAFF"}, "cleric": {"EU", "STAFF"}, "darkstaff": {"EU", "DARKSTAFF"},
	"sword_ch": {"CH", "SWORD"}, "sword_1h": {"EU", "SWORD"}, "sword_2h": {"EU", "TSWORD"},
	"blade": {"CH", "BLADE"}, "glaive": {"CH", "TBLADE"}, "spear": {"CH", "SPEAR"}, "bow": {"CH", "BOW"},
	"crossbow": {"EU", "CROSSBOW"}, "dagger": {"EU", "DAGGER"}, "harp": {"EU", "HARP"}, "axe": {"EU", "AXE"},
}

func offerIconIndex(m *ItemMetadata, dataset string) map[string]string {
	offerIconMu.Lock()
	defer offerIconMu.Unlock()
	if index, ok := offerIconIdx[dataset]; ok {
		return index
	}
	best := map[string]string{}
	bestCode := map[string]string{}
	for _, definition := range m.Catalogs[dataset].Items {
		icon, _ := definition.Presentation["icon_url"].(string)
		if icon == "" {
			continue
		}
		code := definition.Code
		var key string
		if match := offerSetCode.FindStringSubmatch(code); match != nil {
			key = strings.Join([]string{match[1], "SET", match[2], match[3], match[4]}, "|")
		} else if match := offerGearCode.FindStringSubmatch(code); match != nil {
			key = strings.Join([]string{match[1], match[2], match[3]}, "|")
		} else {
			continue
		}
		if current, ok := bestCode[key]; !ok || code < current {
			bestCode[key], best[key] = code, icon
		}
	}
	offerIconIdx[dataset] = best
	return best
}

// OfferIcon returns a local gear icon for a heuristic advertisement tag, or
// an empty string. Alchemy and miscellaneous items have no gear icon.
func (m *ItemMetadata) OfferIcon(server, itemType, subtype, part, race string, degree int) string {
	dataset, ok := m.DatasetForServer(server)
	if !ok || degree < 1 {
		return ""
	}
	index := offerIconIndex(m, dataset)
	dd := strconv.Itoa(degree)
	if degree < 10 {
		dd = "0" + dd
	}
	races := []string{"EU", "CH"}
	switch race {
	case "chinese":
		races = []string{"CH", "EU"}
	case "european":
		races = []string{"EU", "CH"}
	}
	lookup := func(parts ...string) string {
		for _, r := range races {
			if icon := index[strings.Join(append([]string{r}, parts...), "|")]; icon != "" {
				return icon
			}
		}
		return ""
	}
	switch itemType {
	case "weapons":
		code, ok := offerWeaponCodes[subtype]
		if !ok {
			return ""
		}
		return index[strings.Join([]string{code[0], code[1], dd}, "|")]
	case "shields":
		return lookup("SHIELD", dd)
	case "accessories":
		return lookup(strings.ToUpper(subtype), dd)
	case "sets":
		family := map[string]string{"garment": "CLOTHES", "protector": "LIGHT", "armor": "HEAVY"}[subtype]
		if family == "" {
			return ""
		}
		if part == "" {
			part = "BA"
		}
		return lookup("SET", family, dd, part)
	}
	return ""
}
