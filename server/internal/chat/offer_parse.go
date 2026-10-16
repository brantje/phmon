package chat

import (
	"regexp"
	"strconv"
	"strings"
)

// OfferParserVersion is stored with each parsed message. Bump it when the
// keyword tables change so the lazy re-parse refreshes saved tags.
const OfferParserVersion = 1

// OfferItem is a heuristic reading of advertisement text. It is not verified
// item identity: players abbreviate freely and the tags can be incomplete.
type OfferItem struct {
	Type     string   `json:"item_type"`
	Subtype  string   `json:"subtype,omitempty"`
	Part     string   `json:"part,omitempty"`
	Degree   int      `json:"degree,omitempty"`
	Race     string   `json:"race,omitempty"`
	Seal     string   `json:"seal,omitempty"`
	Plus     int      `json:"plus,omitempty"`
	Price    string   `json:"price,omitempty"`
	Currency string   `json:"currency,omitempty"`
	Notes    []string `json:"notes,omitempty"`
	IconURL  string   `json:"icon_url,omitempty"`
}

var (
	offerSegmentSplit = regexp.MustCompile(`(?i)\s*\|\s*|[\r\n]+|\s*;\s*|\s*,\s*|\s+/\s+|\s+(?:or|and|&)\s+`)
	offerIntentPrefix = regexp.MustCompile(`(?i)^\s*(?:(?:wtb(?:uy)?|wts(?:ell)?|wtt)\b[\s:,.\-]*)+`)
	offerIntentWord   = regexp.MustCompile(`(?i)\b(?:wtb(?:uy)?|wts(?:ell)?|wtt)\b`)
	offerDegreeRes    = []*regexp.Regexp{
		regexp.MustCompile(`(?i)\b(?:degree|dg|d)\s*(\d{1,2})\b`),
		regexp.MustCompile(`(?i)\b(\d{1,2})\s*(?:degree|dg|d)\b`),
	}
	offerPlusRe     = regexp.MustCompile(`\+\s*(\d{1,2})\b`)
	offerPriceRe    = regexp.MustCompile(`(?i)(?:->|\bfor)\s*(\d+(?:[.,]\d+)?\s*(?:b|m|s|mill(?:ion)?|bil(?:lion)?))\b`)
	offerEachRe     = regexp.MustCompile(`(?i)\b(\d+)\s*each\b`)
	offerPercentRe  = regexp.MustCompile(`(?i)\b(\d{1,3})\s*%\s*(?:fb|stats)\b`)
	offerQuantityRe = regexp.MustCompile(`(?i)(?:\b\d+(?:[.,]\d+)?k\+?\b|\b\d+\s*x\b|\bx\s*\d+\b|\b\d+\s*stacks?\b|\bfull\s+\d+/\d+\b)`)
	offerSeals      = []struct {
		key string
		re  *regexp.Regexp
	}{
		{"nova", regexp.MustCompile(`(?i)\bnova\b`)},
		{"sun", regexp.MustCompile(`(?i)\bsun\b|\bsos(?:un)?\b`)},
		{"moon", regexp.MustCompile(`(?i)\bmoon\b|\bsom\b`)},
		{"star", regexp.MustCompile(`(?i)\bstar\b`)},
		{"egy_a", regexp.MustCompile(`(?i)\begy[\s_]*a\b`)},
		{"egy_b", regexp.MustCompile(`(?i)\begy[\s_]*b\b`)},
	}
	offerRaceCH  = regexp.MustCompile(`(?i)\b(?:chinese|chn|china|chin|ch)\b`)
	offerRaceEU  = regexp.MustCompile(`(?i)\b(?:european|euro|eur|eu)\b`)
	offerImplyCH = regexp.MustCompile(`(?i)\b(?:protector|prot|garment|garm)\b|\barmou?r\b`)
	offerImplyEU = regexp.MustCompile(`(?i)\b(?:light\s+armou?r|heavy\s+armou?r|la|ha|robe)\b`)
)

type offerKeyword struct {
	key string
	re  *regexp.Regexp
}

func offerWords(key string, words ...string) offerKeyword {
	parts := make([]string, len(words))
	for i, word := range words {
		parts[i] = strings.ReplaceAll(regexp.QuoteMeta(word), `\ `, `\s+`)
	}
	return offerKeyword{key, regexp.MustCompile(`(?i)\b(?:` + strings.Join(parts, "|") + `)\b`)}
}

var (
	offerAlchemy = []offerKeyword{
		offerWords("attribute_stone", "attribute stone", "attr stone", "green stone"),
		offerWords("magic_stone", "magic stone", "magic stones", "immo stone", "immo stones", "luck stone", "luck stones"),
		offerWords("lucky_powder", "lucky powder", "lucky powders"),
		offerWords("elixir", "elixir", "elixirs"),
		offerWords("rondo", "destroyer rondo", "rondo"),
	}
	offerAccessories = []offerKeyword{
		offerWords("necklace", "necklace", "necklaces", "neck"),
		offerWords("earring", "earring", "earrings"),
		offerWords("ring", "ring", "rings"),
	}
	offerSetFamilies = []offerKeyword{
		offerWords("garment", "garment", "garm", "robe"),
		offerWords("protector", "protector", "prot", "light armor", "light armour", "la"),
		offerWords("armor", "heavy armor", "heavy armour", "ha", "armor", "armour"),
	}
	offerSetParts = []offerKeyword{
		offerWords("HA", "head", "helm", "helmet", "hat", "cap"),
		offerWords("SA", "shoulder", "shoulders"),
		offerWords("BA", "chest", "body"),
		offerWords("LA", "legs", "leg", "pants"),
		offerWords("AA", "hands", "hand", "gloves", "gauntlet"),
		offerWords("FA", "foot", "feet", "boots", "shoes"),
	}
	offerWeapons = []offerKeyword{
		offerWords("darkstaff", "dark staff", "warlock rod", "warlock"),
		offerWords("cleric", "cleric rod", "cleric"),
		offerWords("sword_2h", "2h sword", "2 h sword", "two hand sword", "two handed sword", "tsword"),
		offerWords("sword_1h", "1h sword", "1 h sword", "one hand sword", "one handed sword"),
		offerWords("sword_ch", "sword ch", "chn sword", "china sword", "chinese sword", "ch sword"),
		offerWords("glaive", "glaive", "glavie", "polearm"),
		offerWords("blade", "blade"),
		offerWords("spear", "spear"),
		offerWords("crossbow", "crossbow", "xbow"),
		offerWords("bow", "bow"),
		offerWords("dagger", "dagger"),
		offerWords("harp", "harp"),
		offerWords("axe", "axe"),
		offerWords("staff", "staff", "light staff", "rod"),
		offerWords("sword", "sword"),
	}
	offerShield = offerWords("shield", "shield", "shields")
	offerOther  = []offerKeyword{
		offerWords("pandora", "pandora"),
		offerWords("premium", "premium", "gold time"),
		offerWords("flag", "dragon flag"),
		offerWords("wing", "devil wing"),
		offerWords("avatar", "avatar"),
		offerWords("taxi", "taxi"),
		offerWords("service", "plvl", "service"),
		offerWords("box", "box"),
	}
)

func firstOffer(text string, list []offerKeyword) string {
	for _, keyword := range list {
		if keyword.re.MatchString(text) {
			return keyword.key
		}
	}
	return ""
}

func offerDegree(text string) int {
	for _, re := range offerDegreeRes {
		if match := re.FindStringSubmatch(text); match != nil {
			if n, err := strconv.Atoi(match[1]); err == nil && n >= 1 && n <= 14 {
				return n
			}
		}
	}
	return 0
}

// ParseOfferText extracts item tags from one chat advertisement. A message may
// list several items; a fragment that only adds a seal or degree inherits the
// item kind of the previous fragment ("13d ch sword sun or moon").
func ParseOfferText(text string) []OfferItem {
	if len(text) > 512 {
		text = text[:512]
	}
	var items []OfferItem
	var previous *OfferItem
	for _, segment := range offerSegmentSplit.Split(text, -1) {
		segment = strings.TrimSpace(offerIntentPrefix.ReplaceAllString(segment, ""))
		segment = strings.TrimSpace(offerIntentWord.ReplaceAllString(segment, " "))
		if segment == "" {
			continue
		}
		item := parseOfferSegment(segment)
		if item.Type == "" && previous != nil && (item.Seal != "" || item.Degree != 0 || item.Plus != 0) {
			item.Type, item.Subtype, item.Part = previous.Type, previous.Subtype, previous.Part
			if item.Race == "" {
				item.Race = previous.Race
			}
			if item.Degree == 0 {
				item.Degree = previous.Degree
			}
		}
		if item.Type == "" {
			continue
		}
		items = append(items, item)
		previous = &items[len(items)-1]
	}
	return items
}

func parseOfferSegment(segment string) OfferItem {
	var item OfferItem
	switch {
	case firstOffer(segment, offerAlchemy) != "":
		item.Type, item.Subtype = "alchemy", firstOffer(segment, offerAlchemy)
	case firstOffer(segment, offerAccessories) != "":
		item.Type, item.Subtype = "accessories", firstOffer(segment, offerAccessories)
	case offerShield.re.MatchString(segment):
		item.Type = "shields"
	case firstOffer(segment, offerWeapons) != "":
		item.Type, item.Subtype = "weapons", firstOffer(segment, offerWeapons)
		if item.Subtype == "sword" {
			item.Subtype = "sword_1h"
		}
	case firstOffer(segment, offerSetFamilies) != "" || (firstOffer(segment, offerSetParts) != "" && offerDegree(segment) != 0) ||
		offerWords("", "set", "sets").re.MatchString(segment):
		item.Type, item.Subtype = "sets", firstOffer(segment, offerSetFamilies)
		item.Part = firstOffer(segment, offerSetParts)
	case firstOffer(segment, offerOther) != "":
		item.Type, item.Subtype = "other", firstOffer(segment, offerOther)
	}
	if item.Type == "weapons" && item.Subtype == "sword_1h" && offerRaceCH.MatchString(segment) {
		item.Subtype = "sword_ch"
	}
	item.Degree = offerDegree(segment)
	if match := offerPlusRe.FindStringSubmatch(segment); match != nil {
		item.Plus, _ = strconv.Atoi(match[1])
	}
	for _, seal := range offerSeals {
		if seal.re.MatchString(segment) {
			item.Seal = seal.key
			break
		}
	}
	switch {
	case offerRaceCH.MatchString(segment):
		item.Race = "chinese"
	case offerRaceEU.MatchString(segment):
		item.Race = "european"
	case offerImplyEU.MatchString(segment):
		item.Race = "european"
	case offerImplyCH.MatchString(segment) && item.Type == "sets":
		item.Race = "chinese"
	}
	if match := offerPriceRe.FindStringSubmatch(segment); match != nil {
		item.Price = strings.Join(strings.Fields(match[1]), "")
	} else if match := offerEachRe.FindStringSubmatch(segment); match != nil {
		item.Price = match[1] + " each"
	}
	lower := strings.ToLower(segment)
	switch {
	case strings.Contains(lower, "arena coin"):
		item.Currency = "arena_coins"
	case strings.Contains(lower, "silk"):
		item.Currency = "silk"
	case strings.Contains(lower, "gold"):
		item.Currency = "gold"
	}
	if strings.Contains(lower, "clean") {
		item.Notes = append(item.Notes, "clean")
	}
	if match := offerPercentRe.FindString(segment); match != "" {
		item.Notes = append(item.Notes, strings.ToLower(strings.Join(strings.Fields(match), "")))
	}
	if match := offerQuantityRe.FindString(segment); match != "" {
		item.Notes = append(item.Notes, strings.ToLower(strings.Join(strings.Fields(match), "")))
	}
	return item
}
