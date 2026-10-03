package httpapi

import (
	"phmon/server/internal/characters"
	"phmon/server/internal/events"
	"phmon/server/internal/resources"
)

func characterWithPortrait(character characters.Character, metadata *resources.Store) characters.Character {
	if metadata != nil {
		character.PortraitURL = metadata.PortraitURL(character.Server, character.ModelID)
	}
	return character
}

func mapCharactersWithPortraits(items []characters.Character, metadata *resources.Store) []characters.Character {
	for i := range items {
		items[i] = characterWithPortrait(items[i], metadata)
	}
	return items
}

func groupsWithPortraits(groups []characters.Group, metadata *resources.Store) []characters.Group {
	for i := range groups {
		for j := range groups[i].Members {
			groups[i].Members[j] = characterWithPortrait(groups[i].Members[j], metadata)
		}
	}
	return groups
}

func eventsWithPortraits(page events.Page, metadata *resources.Store) events.Page {
	for i := range page.Events {
		if metadata != nil {
			page.Events[i].PortraitURL = metadata.PortraitURL(page.Events[i].Server, page.Events[i].ModelID)
			page.Events[i].ItemMetadata = metadata.ItemPresentation(page.Events[i].Server, page.Events[i].ItemModel, page.Events[i].ItemCode)
			page.Events[i].Unique = uniqueInfo(metadata, page.Events[i])
		}
	}
	return page
}

func uniqueInfo(metadata *resources.Store, event events.Event) *events.UniqueInfo {
	if metadata == nil || event.Kind != "world.unique_spawned" {
		return nil
	}
	info := metadata.UniqueInfo(event.Server, event.Payload)
	if info == nil {
		return nil
	}
	return &events.UniqueInfo{
		Name: info.Name, Level: info.Level, ImageURL: info.ImageURL,
		Notice: info.Notice, Killer: info.Killer, ModelID: info.ModelID,
	}
}
