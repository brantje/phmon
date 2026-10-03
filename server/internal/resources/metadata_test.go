package resources

import (
	"encoding/json"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"testing"
)

func TestMetadataIdentityAndPrecision(t *testing.T) {
	m := &ItemMetadata{Servers: map[string]string{"greatest": "gamedata-test"}, Catalogs: map[string]ItemCatalog{
		"gamedata-test": {Items: map[string]ItemDefinition{"847": {Code: "ITEM_TEST", Presentation: map[string]any{"icon_url": "/game-assets/icon/item.png", "rare": false}}}},
	}}
	raw := json.RawMessage(`{"slots":[null,{"item":{"model":847,"servername":"ITEM_TEST","variance":18446744073709551615}}]}`)
	result := string(m.enrich("greatest", raw))
	if !strings.Contains(result, `"metadata"`) || !strings.Contains(result, "18446744073709551615") {
		t.Fatal(result)
	}
	if string(m.enrich("other", raw)) != string(raw) {
		t.Fatal("cross-server enrichment")
	}
	mismatch := json.RawMessage(strings.ReplaceAll(string(raw), "ITEM_TEST", "ITEM_OTHER"))
	if strings.Contains(string(m.enrich("greatest", mismatch)), `"metadata"`) {
		t.Fatal("conflicting code enriched")
	}
}

func TestCharacterPortraitResolutionIsScopedToServerProfile(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "servers.json"), []byte(`{"greatest":"gamedata-a","other":"gamedata-b"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for name, catalog := range map[string]string{
		"gamedata-a": `{"dataset_id":"gamedata-a","items":{},"character_portraits":{"1907":{"code":"CHAR_CH_MAN_ADVENTURER","portrait_url":"/game-assets/interface/character/char_ch_man1.png"}}}`,
		"gamedata-b": `{"dataset_id":"gamedata-b","items":{},"character_portraits":{"1907":{"code":"CHAR_CH_MAN_ADVENTURER","portrait_url":"/game-assets/interface/character/char_ch_man2.png"}}}`,
	} {
		if err := os.WriteFile(filepath.Join(directory, name+".json"), []byte(catalog), 0600); err != nil {
			t.Fatal(err)
		}
	}
	metadata, err := LoadItemMetadata(directory)
	if err != nil {
		t.Fatal(err)
	}
	model := int64(1907)
	if got := metadata.PortraitURL("Greatest", &model); got != "/game-assets/interface/character/char_ch_man1.png" {
		t.Fatalf("Greatest portrait = %q", got)
	}
	if got := metadata.PortraitURL("other", &model); got != "/game-assets/interface/character/char_ch_man2.png" {
		t.Fatalf("other profile portrait = %q", got)
	}
	if got := metadata.PortraitURL("unmapped", &model); got != "" {
		t.Fatalf("unmapped server received a cross-profile portrait: %q", got)
	}
	if got := metadata.PortraitURL("greatest", nil); got != "" {
		t.Fatalf("missing model received a portrait: %q", got)
	}
}

func TestCharacterPortraitCatalogRejectsRemoteAndUnlistedFiles(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "servers.json"), []byte(`{"greatest":"gamedata-test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	for _, url := range []string{"https://example.test/portrait.png", "/game-assets/other/portrait.png", "/game-assets/interface/character/char_ch_man14.png"} {
		catalog := `{"dataset_id":"gamedata-test","items":{},"character_portraits":{"1907":{"code":"CHAR_CH_MAN_ADVENTURER","portrait_url":"` + url + `"}}}`
		if err := os.WriteFile(filepath.Join(directory, "gamedata-test.json"), []byte(catalog), 0600); err != nil {
			t.Fatal(err)
		}
		if _, err := LoadItemMetadata(directory); err == nil {
			t.Fatalf("invalid portrait URL accepted: %s", url)
		}
	}
}

func TestBundledGreatestPortraitCatalogHasVerifiedLocalMappings(t *testing.T) {
	metadata, err := LoadItemMetadata(filepath.Join("..", "..", "game-data"))
	if err != nil {
		t.Fatal(err)
	}
	catalog := metadata.Catalogs[metadata.Servers["greatest"]]
	if len(catalog.CharacterPortraits) != 52 {
		t.Fatalf("bundled character portrait mapping count = %d, want 52", len(catalog.CharacterPortraits))
	}
	for model, expectedURL := range map[int64]string{
		1907:  "/game-assets/interface/character/char_ch_man1.png",
		1932:  "/game-assets/interface/character/char_ch_woman13.png",
		14875: "/game-assets/interface/character/char_eu_man1.png",
		14900: "/game-assets/interface/character/char_eu_woman13.png",
	} {
		if got := metadata.PortraitURL("greatest", &model); got != expectedURL {
			t.Errorf("model %d portrait = %q, want %q", model, got, expectedURL)
		}
	}
	itemModel := int64(13310)
	name, icon := metadata.MapItemPresentation("Greatest", &itemModel, "")
	if name != "Crater Gold Cuirassir Poleyn" || icon != "/game-assets/icon/item/europe/woman_item/heavy_07_la.png" {
		t.Fatalf("map drop presentation is not sourced from the selected catalog: %q %q", name, icon)
	}
	if name, icon := metadata.MapItemPresentation("unmapped", &itemModel, ""); name != "" || icon != "" {
		t.Fatalf("unknown server received a map drop icon: %q %q", name, icon)
	}
}

func TestSharedSROItemMetadataFallbackUsesItemCodeAcrossServers(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "servers.json"), []byte(`{"greatest":"gamedata-test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	catalog := `{"dataset_id":"gamedata-test","items":{"847":{"code":"ITEM_TEST","presentation":{"icon_url":"/game-assets/icon/item.png","name":"SRO Test Armor","rarity":2,"rare":true,"type_ids":[3,1,2,0],"reference_stats":{"phy_def_pwr":{"min":"50","max":"60"}}}}}}`
	if err := os.WriteFile(filepath.Join(directory, "gamedata-test.json"), []byte(catalog), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, err := LoadItemMetadata(directory)
	if err != nil {
		t.Fatal(err)
	}
	model := int64(847)
	if got := metadata.ItemPresentation("Greatest", &model, ""); got["name"] != "SRO Test Armor" || got["dataset_id"] != "gamedata-test" {
		t.Fatalf("server model presentation missing: %+v", got)
	}
	wrongModel := int64(99999)
	if got := metadata.ItemPresentation("other", &wrongModel, ""); got != nil {
		t.Fatalf("unmapped server model was guessed: %+v", got)
	}
	if got := metadata.ItemPresentation("other", &wrongModel, "ITEM_TEST"); got["name"] != "SRO Test Armor" {
		t.Fatalf("stable item code did not resolve shared presentation: %+v", got)
	}
	if got := metadata.ItemPresentation("Greatest", &model, "WRONG_CODE"); got != nil {
		t.Fatalf("conflicting code was ignored: %+v", got)
	}

	// The numeric model differs from the catalog. The stable SRO item code
	// resolves the shared static item definition, including rarity and reference
	// stats, and applies that definition to live API observations on another SRO.
	raw := json.RawMessage(`{"slots":[{"item":{"model":99999,"servername":"ITEM_TEST","plus":0,"api_evidence_version":2,"api_fields":{"phys_def":55,"whites":{"mapping_entries":[{"key":"1","key_type":"integer","value":"50"}]}},"api_field_types":{"phys_def":{"type":"integer"},"whites":{"type":"dict","count":1}}}}]}`)
	var payload struct {
		Slots []struct {
			Item struct {
				Metadata map[string]any `json:"metadata"`
				Details  any            `json:"instance_details"`
			} `json:"item"`
		} `json:"slots"`
	}
	if err := json.Unmarshal(metadata.enrich("Servar", raw), &payload); err != nil {
		t.Fatal(err)
	}
	if len(payload.Slots) != 1 || payload.Slots[0].Item.Metadata["icon_url"] != "/game-assets/icon/item.png" {
		t.Fatalf("shared item presentation missing for another SRO server: %+v", payload)
	}
	sharedMetadata := payload.Slots[0].Item.Metadata
	if sharedMetadata["rare"] != true || sharedMetadata["rarity"] != float64(2) || sharedMetadata["name"] != "SRO Test Armor" {
		t.Fatalf("shared rarity/name metadata missing: %+v", sharedMetadata)
	}
	if _, ok := sharedMetadata["reference_stats"].(map[string]any); !ok {
		t.Fatalf("shared reference stats missing: %+v", sharedMetadata)
	}
	if payload.Slots[0].Item.Details == nil {
		t.Fatal("observed API stats were not interpreted with shared item reference metadata")
	}
	details, ok := payload.Slots[0].Item.Details.(map[string]any)
	if !ok {
		t.Fatalf("shared reference stats did not resolve observed item stats: %+v", payload.Slots[0].Item.Details)
	}
	stats, ok := details["stats"].([]any)
	if !ok || len(stats) != 1 {
		t.Fatalf("unexpected resolved shared item stats: %+v", details["stats"])
	}
	stat, ok := stats[0].(map[string]any)
	if !ok || stat["key"] != "phy_def_pwr" || stat["value"] != "55.0" {
		t.Fatalf("shared reference range was not used for observed stats: %+v", details["stats"])
	}

	withoutCode := json.RawMessage(`{"slots":[{"item":{"model":847}}]}`)
	if got := string(metadata.enrich("Servar", withoutCode)); got != string(withoutCode) {
		t.Fatalf("shared icon lookup guessed by model number without an item code: %s", got)
	}
}

func TestItemDropClassificationRequiresExplicitProfileRarity(t *testing.T) {
	metadata := &ItemMetadata{
		Servers: map[string]string{"greatest": "gamedata-test"},
		Catalogs: map[string]ItemCatalog{"gamedata-test": {Items: map[string]ItemDefinition{
			"847": {Code: "ITEM_RARE", Presentation: map[string]any{"rare": true}},
			"848": {Code: "ITEM_NORMAL", Presentation: map[string]any{"rare": false}},
			"849": {Code: "ITEM_UNKNOWN", Presentation: map[string]any{"name": "Unknown"}},
		}}},
	}
	model := int64(847)
	if class, version := metadata.ItemDropClassification("Greatest", &model, "ITEM_RARE"); class != "rare" || version != "item-profile-rarity-v1" {
		t.Fatalf("rare class = %q, %q", class, version)
	}
	model = 848
	if class, _ := metadata.ItemDropClassification("Greatest", &model, "ITEM_NORMAL"); class != "normal" {
		t.Fatalf("normal class = %q", class)
	}
	model = 849
	if class, version := metadata.ItemDropClassification("Greatest", &model, "ITEM_UNKNOWN"); class != "" || version != "" {
		t.Fatalf("missing rarity should remain unknown, got %q %q", class, version)
	}
	model = 847
	if class, _ := metadata.ItemDropClassification("Unmapped", &model, "ITEM_RARE"); class != "" {
		t.Fatalf("unmapped server class = %q", class)
	}
}

func TestEnrichHistoricalItemRecordAndMergePartialEvidence(t *testing.T) {
	metadata := &ItemMetadata{
		Servers: map[string]string{"greatest": "gamedata-test"},
		Catalogs: map[string]ItemCatalog{"gamedata-test": {Items: map[string]ItemDefinition{
			"847": {Code: "ITEM_TEST", Presentation: map[string]any{"name": "Test Necklace", "rare": false, "type_ids": []any{json.Number("3"), json.Number("1"), json.Number("5"), json.Number("0")}}},
		}}},
		SharedPresentations: map[string]map[string]any{},
		SharedMagicOptions:  map[string]MagicOptionDefinition{},
	}
	item := map[string]any{
		"model": 847, "servername": "ITEM_TEST", "quantity": 2,
		"api_evidence_version": 2,
		"api_fields":           map[string]any{"blues": map[string]any{}},
		"api_field_types":      map[string]any{"blues": map[string]any{"type": "dict", "count": 0}},
	}
	enriched := metadata.EnrichItemRecord("Greatest", item)
	if enriched["quantity"] != float64(2) || enriched["metadata"].(map[string]any)["name"] != "Test Necklace" {
		t.Fatalf("historical item snapshot lost source or profile fields: %+v", enriched)
	}
	if details, ok := enriched["instance_details"].(map[string]any); !ok || details["source"] != "phbot_api" {
		t.Fatalf("API evidence was not resolved on the event snapshot: %+v", enriched)
	}

	api := map[string]any{
		"source": "phbot_api", "percentages": []any{map[string]any{"key": "durability", "value": 50}},
		"blues": []any{}, "blues_status": "not_observed", "absolute_stats_status": "unavailable",
	}
	packet := map[string]any{
		"source": "vsro_1188_packet", "percentages": []any{map[string]any{"key": "phy_def_pwr", "value": 70}},
		"blues": []any{map[string]any{"id": "1", "raw_value": "0"}}, "blues_status": "partial",
	}
	merged := mergeItemDetails(api, packet)
	if len(merged["percentages"].([]any)) != 2 || len(merged["blues"].([]any)) != 1 || len(merged["sources"].([]any)) != 2 {
		t.Fatalf("partial evidence was not merged by field: %+v", merged)
	}
	apiBlues := []any{
		map[string]any{"id": "1", "raw_value": "0", "label": "Option 1", "label_verified": false},
		map[string]any{"id": "1", "raw_value": "0", "label": "Option 1", "label_verified": false},
	}
	packetBlues := []any{
		map[string]any{"id": "1", "raw_value": "0", "label": "Option 1"},
		map[string]any{"id": "1", "raw_value": "0", "label": "Option 1"},
		map[string]any{"id": "2", "raw_value": "5", "label": "Option 2"},
	}
	blueUnion, _ := mergeBlueRows(apiBlues, packetBlues)
	if len(blueUnion) != 3 {
		t.Fatalf("merged blue rows did not preserve duplicate counts and packet-only options: %+v", blueUnion)
	}
}

func TestReferenceStatCatalogValidation(t *testing.T) {
	if !validReferenceStats(map[string]any{
		"phy_def_pwr": map[string]any{"min": "50", "max": "60", "increment": "1.25"},
	}) {
		t.Fatal("expected bounded decimal reference range to validate")
	}
	for _, value := range []any{
		map[string]any{"phy_def_pwr": map[string]any{"min": "60", "max": "50"}},
		map[string]any{"phy_def_pwr": map[string]any{"min": "NaN", "max": "60"}},
		map[string]any{"phy_def_pwr": map[string]any{"min": "1e3", "max": "1000"}},
		map[string]any{"phy_def_pwr": map[string]any{"min": json.Number("1"), "max": "2"}},
	} {
		if validReferenceStats(value) {
			t.Fatalf("invalid reference stats accepted: %#v", value)
		}
	}
}

func TestMetadataLoaderValidatesOptionRangesAndUnscaledLabelsStayHidden(t *testing.T) {
	directory := t.TempDir()
	if err := os.WriteFile(filepath.Join(directory, "servers.json"), []byte(`{"greatest":"gamedata-test"}`), 0600); err != nil {
		t.Fatal(err)
	}
	catalogPath := filepath.Join(directory, "gamedata-test.json")
	valid := `{"dataset_id":"gamedata-test","items":{"1":{"code":"ITEM_TEST"}},"magic_options":{"9":{"code":"MATTR_INT","label":"Int Increase","raw_ranges":[{"minimum":"1","maximum":"5"}]}}}`
	if err := os.WriteFile(catalogPath, []byte(valid), 0600); err != nil {
		t.Fatal(err)
	}
	metadata, err := LoadItemMetadata(directory)
	if err != nil {
		t.Fatal(err)
	}
	definition := metadata.Catalogs["gamedata-test"].MagicOptions["9"]
	details := resolveInstanceDetails(map[string]any{
		"availability": "observed", "source": "vsro_1188_packet",
		"magic_options_availability": "observed",
		"magic_options":              []any{map[string]any{"id": "9", "value": "3"}},
	}, map[string]any{"type_ids": []any{json.Number("3"), json.Number("1"), json.Number("6"), json.Number("2")}}, map[string]MagicOptionDefinition{"9": definition})
	if len(details["blues"].([]any)) != 0 || details["unresolved_blue_count"] != 1 {
		t.Fatalf("unscaled option must stay hidden: %#v", details)
	}
	invalid := strings.Replace(valid, `"maximum":"5"`, `"maximum":"0"`, 1)
	if err := os.WriteFile(catalogPath, []byte(invalid), 0600); err != nil {
		t.Fatal(err)
	}
	if _, err := LoadItemMetadata(directory); err == nil {
		t.Fatal("expected inverted option range rejection")
	}
}

func TestInstanceDetailsUseFamilySpecificRollSlotsAndExactIntegerMath(t *testing.T) {
	armorVariance := uint64(31) << (5 * 3)
	armor := resolveInstanceDetails(map[string]any{
		"availability": "observed", "source": "vsro_1188_packet",
		"variance":   strconv.FormatUint(armorVariance, 10),
		"durability": json.Number("60"), "magic_options_availability": "observed",
		"magic_options": []any{},
	}, map[string]any{"type_ids": []any{json.Number("3"), json.Number("1"), json.Number("1"), json.Number("1")}}, nil)
	percentages := armor["percentages"].([]any)
	if len(percentages) != 6 {
		t.Fatalf("armor percentages = %#v", percentages)
	}
	if percentages[3].(map[string]any)["value"] != uint64(100) {
		t.Fatalf("armor defense roll should occupy family slot 3: %#v", percentages[3])
	}
	if percentages[4].(map[string]any)["value"] != uint64(0) {
		t.Fatalf("empty variance slot should be 0%%: %#v", percentages[4])
	}
	if armor["absolute_stats_reason"] != "reference_formula_not_verified" {
		t.Fatalf("absolute stats must remain gated: %#v", armor)
	}
	if armor["blues_status"] != "unavailable_definitions" {
		t.Fatalf("confirmed empty options should differ from missing definitions: %#v", armor)
	}
}

func TestInstanceDetailsPreserveBlueOrderAndOmitUnknownLabels(t *testing.T) {
	catalog := ItemCatalog{MagicOptions: map[string]MagicOptionDefinition{
		"9": {Label: "Verified option", Unit: "%", Scale: 1},
	}}
	metadata := map[string]any{"type_ids": []any{json.Number("3"), json.Number("1"), json.Number("5"), json.Number("3")}}
	instance := map[string]any{
		"availability": "observed", "source": "vsro_1188_packet",
		"magic_options_availability": "observed",
		"magic_options": []any{
			map[string]any{"id": "9", "value": "3"},
			map[string]any{"id": "9", "value": "4"},
			map[string]any{"id": "999", "value": "1"},
		},
	}
	details := resolveInstanceDetails(instance, metadata, catalog.MagicOptions)
	blues := details["blues"].([]any)
	if len(blues) != 2 || details["unresolved_blue_count"] != 1 {
		t.Fatalf("blue resolution = %#v", details)
	}
	if blues[0].(map[string]any)["raw_value"] != "3" || blues[1].(map[string]any)["raw_value"] != "4" {
		t.Fatalf("blue order/duplicates changed: %#v", blues)
	}
	missing := resolveInstanceDetails(map[string]any{
		"availability": "observed", "source": "vsro_1188_packet",
		"magic_options_availability": "not_observed",
	}, metadata, nil)
	if missing["blues_status"] != "not_observed" {
		t.Fatalf("missing options should not become an empty observation: %#v", missing)
	}
}

func TestInstanceDetailsRejectMalformedOrOversizedDecimalFields(t *testing.T) {
	options := map[string]MagicOptionDefinition{
		"9": {Label: "Verified option", Scale: 1},
	}
	metadata := map[string]any{"type_ids": []any{json.Number("3"), json.Number("1"), json.Number("6"), json.Number("1")}}
	instance := map[string]any{
		"availability": "observed", "source": "vsro_1188_packet",
		"variance":                   "+31",
		"magic_options_availability": "observed",
		"magic_options":              []any{map[string]any{"id": "9", "value": strings.Repeat("9", 21)}},
	}
	details := resolveInstanceDetails(instance, metadata, options)
	if details["variance_status"] != "invalid_unsigned_64_bit_value" {
		t.Fatalf("noncanonical variance must not be accepted: %#v", details)
	}
	if len(details["blues"].([]any)) != 0 || details["unresolved_blue_count"] != 1 {
		t.Fatalf("oversized raw value must not be rendered: %#v", details)
	}
}

func TestRollQualityUsesWeaponShieldAndAccessoryOffsets(t *testing.T) {
	cases := []struct {
		name       string
		family     int
		offset     uint
		entryIndex int
		label      string
	}{
		{"weapon critical", 6, 30, 6, "Critical"},
		{"shield physical defense", 4, 20, 4, "Phy. def. pwr"},
		{"accessory magical absorption", 5, 5, 1, "Mag. absorption"},
	}
	for _, test := range cases {
		t.Run(test.name, func(t *testing.T) {
			variance := uint64(31) << test.offset
			details := resolveInstanceDetails(map[string]any{
				"availability": "observed", "source": "vsro_1188_packet",
				"variance": strconv.FormatUint(variance, 10),
			}, map[string]any{"type_ids": []any{json.Number("3"), json.Number("1"), json.Number(strconv.Itoa(test.family)), json.Number("1")}}, nil)
			percentages := details["percentages"].([]any)
			entry := percentages[test.entryIndex].(map[string]any)
			if entry["label"] != test.label || entry["value"] != uint64(100) {
				t.Fatalf("family %d slot %d = %#v", test.family, test.offset/5, entry)
			}
		})
	}

	rollThirty := resolveInstanceDetails(map[string]any{
		"availability": "observed", "source": "vsro_1188_packet",
		"variance": strconv.FormatUint(uint64(30), 10),
	}, map[string]any{"type_ids": []any{json.Number("3"), json.Number("1"), json.Number("6"), json.Number("1")}}, nil)
	if got := rollThirty["percentages"].([]any)[0].(map[string]any)["value"]; got != uint64(96) {
		t.Fatalf("30/31 must floor to 96%%, got %#v", got)
	}
}

func TestPackagedMetadata(t *testing.T) {
	m, err := LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	c := m.Catalogs[m.Servers["greatest"]]
	found := 0
	for _, item := range c.Items {
		switch item.Code {
		case "ITEM_CH_NECKLACE_06_C":
			if item.Presentation["rare"] != false || item.Presentation["sort_type"] != "Necklace" {
				t.Fatal(item)
			}
			if m.SharedIcons[item.Code] != item.Presentation["icon_url"] {
				t.Fatalf("item icon absent from shared SRO assets: %s", item.Code)
			}
			found++
		case "ITEM_CH_M_HEAVY_06_HA_C_RARE":
			if item.Presentation["seal"] != "Seal of Sun" || item.Presentation["mounted_part"] != "Head" {
				t.Fatal(item)
			}
			found++
		case "ITEM_CH_W_LIGHT_07_CA_B_RARE":
			if item.Presentation["seal"] != "Seal of Moon" || item.Presentation["required_gender"] != "Female" {
				t.Fatal(item)
			}
			found++
		case "ITEM_CH_RING_01_C_RARE":
			if item.Presentation["seal"] != "Seal of Sun" || item.Presentation["sort_type"] != "Ring" {
				t.Fatal(item)
			}
			found++
		}
	}
	if found != 4 {
		t.Fatalf("found %d screenshot fixtures", found)
	}
}
