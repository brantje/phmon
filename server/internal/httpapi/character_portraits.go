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
		}
	}
	return page
}
