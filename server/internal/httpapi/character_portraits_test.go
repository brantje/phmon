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
