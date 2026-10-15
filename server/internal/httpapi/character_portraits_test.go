package httpapi

import (
	"bytes"
	"encoding/json"
	"os"
	"testing"

	"phmon/server/internal/characters"
	"phmon/server/internal/events"
	"phmon/server/internal/resources"
)

func TestCharacterPortraitIsAddedToCharacterGroupAndEventViews(t *testing.T) {
	model := int64(1907)
	metadata := resources.NewStore(nil)
	metadata.SetItemMetadata(&resources.ItemMetadata{
		Servers: map[string]string{"greatest": "gamedata-test"},
		Catalogs: map[string]resources.ItemCatalog{
			"gamedata-test": {
				CharacterPortraits: map[string]resources.CharacterPortrait{
					"1907": {Code: "CHAR_CH_MAN_ADVENTURER", PortraitURL: "/game-assets/interface/character/char_ch_man1.png"},
				},
			},
		},
	})
	character := characters.Character{Server: "Greatest", Name: "Alpha", ModelID: &model}
	const wantURL = "/game-assets/interface/character/char_ch_man1.png"

	if got := characterWithPortrait(character, metadata).PortraitURL; got != wantURL {
		t.Fatalf("character portrait URL = %q", got)
	}
	mapCharacters := mapCharactersWithPortraits([]characters.Character{character}, metadata)
	if got := mapCharacters[0].PortraitURL; got != wantURL {
		t.Fatalf("map character portrait URL = %q", got)
	}
	group := groupsWithPortraits([]characters.Group{{Members: []characters.Character{character}}}, metadata)
	if got := group[0].Members[0].PortraitURL; got != wantURL {
		t.Fatalf("group member portrait URL = %q", got)
	}
	page := eventsWithPortraits(events.Page{Events: []events.Event{{Server: "Greatest", ModelID: &model}}}, metadata)
	if got := page.Events[0].PortraitURL; got != wantURL {
		t.Fatalf("event portrait URL = %q", got)
	}

	unknown := int64(99999)
	if got := characterWithPortrait(characters.Character{Server: "Greatest", ModelID: &unknown}, metadata).PortraitURL; got != "" {
		t.Fatalf("unknown model unexpectedly resolved to %q", got)
	}
}

func TestDropRelatedObservationRetainsRealAPIStatsSeparately(t *testing.T) {
	raw, err := os.ReadFile("../resources/testdata/phbot-20.1.1-api-items.json")
	if err != nil {
		t.Fatal(err)
	}
	var fixture struct {
		Items []map[string]any `json:"items"`
	}
	decoder := json.NewDecoder(bytes.NewReader(raw))
	decoder.UseNumber()
	if err := decoder.Decode(&fixture); err != nil {
		t.Fatal(err)
	}
	metadata := resources.NewStore(nil)
	catalog, err := resources.LoadItemMetadata("../../game-data")
	if err != nil {
		t.Fatal(err)
	}
	metadata.SetItemMetadata(catalog)
	var item map[string]any
	for _, candidate := range fixture.Items {
		if candidate["name"] == "Python Casque" {
			item = candidate
			break
		}
	}
	if item == nil {
		t.Fatal("missing captured Python Casque")
	}
	model := int64(0)
	if n, ok := item["model"].(json.Number); ok {
		model, _ = n.Int64()
	}
	for _, kind := range []string{"drop.item", "drop.rare"} {
		t.Run(kind, func(t *testing.T) {
			page := eventsWithPortraits(events.Page{Events: []events.Event{{
				Kind: kind, Server: "Greatest", ItemModel: &model,
				Payload: json.RawMessage(`{"model":` + item["model"].(json.Number).String() + `}`),
				RelatedItemObservation: &events.ItemObservation{
					EventID: "8d16ab39-10b8-4b26-80aa-a4be01e10f18", Association: "nearby_same_model_observation",
					Source: "phbot.state_diff", Item: item,
				},
			}}}, metadata)
			event := page.Events[0]
			if event.ItemDetails != nil || bytes.Contains(event.Payload, []byte(`"item"`)) {
				t.Fatal("related inventory evidence was promoted to the drop's own item")
			}
			details, _ := event.RelatedItemObservation.Item["instance_details"].(map[string]any)
			percentages, _ := details["percentages"].([]any)
			blues, _ := details["blues"].([]any)
			if len(percentages) == 0 || len(blues) == 0 {
				t.Fatalf("captured API stats/blues disappeared from related observation: %+v", details)
			}
		})
	}
}

func TestDropEventsResolveStaticItemPresentation(t *testing.T) {
	itemModel := int64(847)
	metadata := resources.NewStore(nil)
	metadata.SetItemMetadata(&resources.ItemMetadata{
		Servers: map[string]string{"greatest": "gamedata-test"},
		Catalogs: map[string]resources.ItemCatalog{
			"gamedata-test": {Items: map[string]resources.ItemDefinition{
				"847": {Code: "ITEM_TEST", Presentation: map[string]any{
					"name": "Gold Armor", "icon_url": "/game-assets/icon/item.png",
					"reference_stats": map[string]any{"phy_def_pwr": map[string]any{"min": "50", "max": "60"}},
				}},
			}},
		},
	})
	page := eventsWithPortraits(events.Page{Events: []events.Event{
		{Kind: "drop.rare", Server: "Greatest", ItemModel: &itemModel},
		{Kind: "drop.item", Server: "Other", ItemModel: &itemModel},
		{Kind: "drop.item", Server: "Greatest", ItemModel: &itemModel,
			Payload: []byte(`{"model":847,"item":{"model":847,"servername":"ITEM_TEST","plus":3,"api_fields":{"blues":{"9":5}}}}`)},
	}}, metadata)
	if page.Events[0].ItemMetadata["name"] != "Gold Armor" || page.Events[0].ItemMetadata["reference_stats"] == nil {
		t.Fatalf("rare drop lacked catalog name and reference ranges: %+v", page.Events[0])
	}
	if page.Events[1].ItemMetadata != nil {
		t.Fatalf("model from an unmapped server was guessed: %+v", page.Events[1])
	}
	if page.Events[2].ItemDetails == nil || page.Events[2].ItemDetails["plus"] != float64(3) {
		t.Fatalf("observed drop item was not enriched: %+v", page.Events[2])
	}
}
