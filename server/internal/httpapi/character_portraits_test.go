package httpapi

import (
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
	}}, metadata)
	if page.Events[0].ItemMetadata["name"] != "Gold Armor" || page.Events[0].ItemMetadata["reference_stats"] == nil {
		t.Fatalf("rare drop lacked catalog name and reference ranges: %+v", page.Events[0])
	}
	if page.Events[1].ItemMetadata != nil {
		t.Fatalf("model from an unmapped server was guessed: %+v", page.Events[1])
	}
}
