package resources

import "testing"

func TestOfferIconResolvesGearByRaceAndDegree(t *testing.T) {
	m := &ItemMetadata{
		Servers: map[string]string{"greatest": "ds"},
		Catalogs: map[string]ItemCatalog{"ds": {Items: map[string]ItemDefinition{
			"1": {Code: "ITEM_EU_TSTAFF_12_A", Presentation: map[string]any{"icon_url": "/staff12.png"}},
			"2": {Code: "ITEM_EU_TSTAFF_12_A_RARE", Presentation: map[string]any{"icon_url": "/staff12r.png"}},
			"3": {Code: "ITEM_CH_M_HEAVY_11_HA_A", Presentation: map[string]any{"icon_url": "/ha11.png"}},
			"4": {Code: "ITEM_CH_M_HEAVY_11_HA_SET_A", Presentation: map[string]any{"icon_url": "/set.png"}},
			"5": {Code: "ITEM_EU_SHIELD_13_C", Presentation: map[string]any{"icon_url": "/shield13.png"}},
		}}},
	}
	for _, c := range []struct {
		typ, sub, part, race string
		degree               int
		want                 string
	}{
		{"weapons", "staff", "", "", 12, "/staff12.png"},
		{"weapons", "staff", "", "", 13, ""},
		{"sets", "armor", "HA", "chinese", 11, "/ha11.png"},
		{"shields", "", "", "european", 13, "/shield13.png"},
		{"alchemy", "elixir", "", "", 11, ""},
	} {
		if got := m.OfferIcon("Greatest", c.typ, c.sub, c.part, c.race, c.degree); got != c.want {
			t.Errorf("%+v = %q, want %q", c, got, c.want)
		}
	}
}
