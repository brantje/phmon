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
