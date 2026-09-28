package resources

import (
	"bytes"
	"encoding/json"
	"os"
	"strings"
	"testing"
)

func liveAPIItems(t *testing.T) (*ItemMetadata, []map[string]any) {
	t.Helper()
	m, err := LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	data, err := os.ReadFile("testdata/phbot-20.1.1-api-items.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Items []map[string]any `json:"items"`
	}
	d := json.NewDecoder(bytes.NewReader(data))
	d.UseNumber()
	if err := d.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	return m, fixture.Items
}

func apiDetail(t *testing.T, m *ItemMetadata, item map[string]any) map[string]any {
	t.Helper()
	raw, err := json.Marshal(map[string]any{"availability": "observed", "slots": []any{map[string]any{"source_slot": 13, "item": item}}})
	if err != nil {
		t.Fatal(err)
	}
	var result map[string]any
	d := json.NewDecoder(bytes.NewReader(m.enrich("greatest", raw)))
	d.UseNumber()
	if err := d.Decode(&result); err != nil {
		t.Fatal(err)
	}
	return result["slots"].([]any)[0].(map[string]any)["item"].(map[string]any)
}

func TestCapturedAPIPercentagesAndBluesMatchScreenshotObservations(t *testing.T) {
	m, items := liveAPIItems(t)
	for _, item := range items {
		model := item["model"].(json.Number).String()
		if model != "4247" && model != "5293" && model != "1888" && model != "5744" {
			continue
		}
		t.Run(model, func(t *testing.T) {
			before, _ := json.Marshal(item)
			result := apiDetail(t, m, item)
			after, _ := json.Marshal(item)
			if !bytes.Equal(before, after) {
				t.Fatal("source observation modified")
			}
			details := result["instance_details"].(map[string]any)
			if details["source"] != "phbot_api" || details["absolute_stats_status"] != "unavailable" {
				t.Fatal(details)
			}
			percentages := details["percentages"].([]any)
			blues := details["blues"].([]any)
			wantFirst := map[string]string{"4247": "3", "5293": "61", "1888": "6", "5744": "0"}[model]
			if len(percentages) == 0 || percentages[0].(map[string]any)["value"].(json.Number).String() != wantFirst {
				t.Fatal(percentages)
			}
			if model == "4247" || model == "5293" {
				if len(blues) != 2 {
					t.Fatal(blues)
				}
				wantLabel, wantValue := "Int increase", "3"
				if model == "5293" {
					wantLabel, wantValue = "Steady", "2"
				}
				first := blues[0].(map[string]any)
				if first["label"] != wantLabel || first["raw_value"] != wantValue {
					t.Fatal(first)
				}
			}
			if _, exists := result["instance"]; exists {
				t.Fatal("presentation invented a packet observation")
			}
		})
	}
}

func TestAPIReplacementCannotReusePreviousBlues(t *testing.T) {
	m, items := liveAPIItems(t)
	for _, item := range items {
		if item["model"].(json.Number).String() != "4247" {
			continue
		}
		first := apiDetail(t, m, item)["instance_details"].(map[string]any)
		if len(first["blues"].([]any)) != 2 {
			t.Fatal(first)
		}
		delete(item["api_fields"].(map[string]any), "blues")
		second := apiDetail(t, m, item)["instance_details"].(map[string]any)
		if second["blues_status"] != "not_observed" || len(second["blues"].([]any)) != 0 {
			t.Fatal(second)
		}
		item["api_fields"].(map[string]any)["blues"] = map[string]any{}
		item["api_field_types"].(map[string]any)["blues"] = map[string]any{"type": "dict", "count": json.Number("0")}
		third := apiDetail(t, m, item)["instance_details"].(map[string]any)
		if third["blues_status"] != "observed_empty" {
			t.Fatal(third)
		}
		return
	}
	t.Fatal("missing captured fixture")
}

func TestAPIEvidenceValidationAndDatasetFences(t *testing.T) {
	for _, change := range []string{"schema", "dataset", "count", "keytype", "percent", "unknownblue", "duplicateblue"} {
		t.Run(change, func(t *testing.T) {
			m, items := liveAPIItems(t)
			for _, item := range items {
				if item["model"].(json.Number).String() != "4247" {
					continue
				}
				fields := item["api_fields"].(map[string]any)
				whites := fields["whites"].(map[string]any)["mapping_entries"].([]any)
				blues := fields["blues"].(map[string]any)["mapping_entries"].([]any)
				switch change {
				case "schema":
					item["api_evidence_version"] = json.Number("1")
				case "dataset":
					item["servername"] = "DIFFERENT_CODE"
				case "count":
					item["api_field_types"].(map[string]any)["whites"].(map[string]any)["count"] = json.Number("99")
				case "keytype":
					whites[0].(map[string]any)["key_type"] = "string"
				case "percent":
					whites[0].(map[string]any)["value"] = json.Number("101")
				case "unknownblue":
					blues[0].(map[string]any)["key"] = "999999"
				case "duplicateblue":
					blues[1].(map[string]any)["key"] = blues[0].(map[string]any)["key"]
				}
				result := apiDetail(t, m, item)
				if change == "schema" || change == "dataset" {
					if _, ok := result["instance_details"]; ok {
						t.Fatal(result)
					}
					return
				}
				details := result["instance_details"].(map[string]any)
				if change == "unknownblue" {
					if len(details["blues"].([]any)) != 2 || details["unresolved_blue_count"].(json.Number).String() != "1" {
						t.Fatal(details)
					}
				} else if change == "duplicateblue" {
					if len(details["blues"].([]any)) != 2 {
						t.Fatal(details)
					}
				} else if details["whites_status"] != "invalid" || len(details["percentages"].([]any)) != 0 {
					t.Fatal(details)
				}
				return
			}
		})
	}
}

func TestAPIWeaponAttributeLabelsAndMalformedEvidence(t *testing.T) {
	m, items := liveAPIItems(t)
	for _, item := range items {
		if item["model"].(json.Number).String() != "89" {
			continue
		}
		details := apiDetail(t, m, item)["instance_details"].(map[string]any)
		if details["whites_status"] != "observed" || len(details["percentages"].([]any)) != 7 {
			t.Fatal(details)
		}
	}
	malformed := map[string]any{}
	d := json.NewDecoder(strings.NewReader(`{"api_fields":{"blues":{}},"api_field_types":{"blues":{"type":"dict","count":0.5}}}`))
	d.UseNumber()
	if err := d.Decode(&malformed); err != nil {
		t.Fatal(err)
	}
	if _, status := apiItemMap(malformed, "blues"); status != "invalid" {
		t.Fatal(status)
	}
}

// These scalar values were read from the live 1.2.6 observation of the same
// screenshot items. Expected display values come from the supplied in-game
// screenshots, independent of the backend implementation.
func TestLive126AbsoluteValuesMatchInGameScreenshots(t *testing.T) {
	m, items := liveAPIItems(t)
	for _, item := range items {
		if item["model"].(json.Number).String() != "4247" {
			continue
		}
		fields := item["api_fields"].(map[string]any)
		shapes := item["api_field_types"].(map[string]any)
		for key, value := range map[string]string{"phys_def": "54", "mag_def": "73", "parry": "23", "max_durability": "77", "phys_reinf_min": "13.872", "phys_reinf_max": "13.872", "mag_reinf_min": "18.233", "mag_reinf_max": "18.233"} {
			fields[key] = json.Number(value)
			kind := "number"
			if key == "phys_def" || key == "mag_def" || key == "parry" || key == "max_durability" {
				kind = "integer"
			}
			shapes[key] = map[string]any{"type": kind}
		}
		result := apiDetail(t, m, item)["instance_details"].(map[string]any)
		stats := result["stats"].([]any)
		want := map[string]string{"phy_def_pwr": "54.8", "mag_def_pwr": "73.3", "parry_ratio": "23", "phy_reinforce": "13.9%", "mag_reinforce": "18.2%"}
		for _, raw := range stats {
			stat := raw.(map[string]any)
			key := stat["key"].(string)
			if expected, ok := want[key]; ok {
				if stat["value"] != expected {
					t.Fatalf("%s got %v, want %s", key, stat["value"], expected)
				}
				delete(want, key)
			}
		}
		if len(want) != 0 {
			t.Fatal("missing stats", want)
		}
		if result["durability"].(map[string]any)["maximum"].(json.Number).String() != "77" {
			t.Fatal(result["durability"])
		}
		break
	}
	var spear map[string]any
	d := json.NewDecoder(strings.NewReader(`{"name":"Phoenix Horn Spear","model":161,"servername":"ITEM_CH_SPEAR_07_A","plus":5,"durability":64,"api_evidence_version":2,"api_fields":{"whites":{"mapping_entries":[{"key":"8","key_type":"integer","value":3},{"key":"0","key_type":"integer","value":19},{"key":"1","key_type":"integer","value":6},{"key":"2","key_type":"integer","value":22},{"key":"3","key_type":"integer","value":12},{"key":"6","key_type":"integer","value":16},{"key":"7","key_type":"integer","value":0}]},"critical":4,"attack_rate":124,"max_durability":64,"phys_atk_min":375,"phys_atk_max":435,"mag_atk_min":640,"mag_atk_max":757,"phys_reinf_min":88.516,"phys_reinf_max":105.358,"mag_reinf_min":152.742,"mag_reinf_max":186.728},"api_field_types":{"whites":{"type":"dict","count":7},"critical":{"type":"integer"},"attack_rate":{"type":"integer"},"max_durability":{"type":"integer"},"phys_atk_min":{"type":"integer"},"phys_atk_max":{"type":"integer"},"mag_atk_min":{"type":"integer"},"mag_atk_max":{"type":"integer"},"phys_reinf_min":{"type":"number"},"phys_reinf_max":{"type":"number"},"mag_reinf_min":{"type":"number"},"mag_reinf_max":{"type":"number"}}}`))
	d.UseNumber()
	if err := d.Decode(&spear); err != nil {
		t.Fatal(err)
	}
	result := apiDetail(t, m, spear)["instance_details"].(map[string]any)
	got := map[string]string{}
	for _, raw := range result["stats"].([]any) {
		stat := raw.(map[string]any)
		got[stat["key"].(string)] = stat["value"].(string)
	}
	for key, want := range map[string]string{"phy_atk_pwr": "375 ~ 435", "mag_atk_pwr": "640 ~ 757", "hit_ratio": "124", "critical_ratio": "4", "phy_reinforce": "88.5% ~ 105.4%", "mag_reinforce": "152.7% ~ 186.7%"} {
		if got[key] != want {
			t.Fatalf("%s got %q want %q", key, got[key], want)
		}
	}
}

func TestNamedAPIStatsSurviveMissingAndInvalidWhiteMaps(t *testing.T) {
	m, items := liveAPIItems(t)
	for _, item := range items {
		if item["model"].(json.Number).String() != "4247" {
			continue
		}
		fields := item["api_fields"].(map[string]any)
		shapes := item["api_field_types"].(map[string]any)
		fields["parry"] = json.Number("23")
		shapes["parry"] = map[string]any{"type": "integer"}
		delete(fields, "whites")
		delete(shapes, "whites")
		got := apiDetail(t, m, item)["instance_details"].(map[string]any)
		stats := got["stats"].([]any)
		if len(stats) != 1 || stats[0].(map[string]any)["value"] != "23" {
			t.Fatal(got)
		}
		if _, hasPercent := stats[0].(map[string]any)["percent"]; hasPercent {
			t.Fatal("unobserved percentage was attached", stats)
		}
		fields["whites"] = map[string]any{}
		shapes["whites"] = map[string]any{"type": "dict", "count": json.Number("2")}
		got = apiDetail(t, m, item)["instance_details"].(map[string]any)
		if got["whites_status"] != "invalid" || len(got["stats"].([]any)) != 1 {
			t.Fatal(got)
		}
		return
	}
	t.Fatal("missing fixture")
}
