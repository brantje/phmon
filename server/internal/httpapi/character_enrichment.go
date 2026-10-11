package httpapi

import (
	"context"

	"phmon/server/internal/characters"
	"phmon/server/internal/resources"
)

func enrichCharacter(ctx context.Context, character characters.Character, metadata *resources.Store) characters.Character {
	character = characterWithPortrait(character, metadata)
	if metadata == nil || character.ID == "" {
		return character
	}
	goldByCharacter, err := metadata.PersonalStorageGoldByCharacters(ctx, []string{character.ID})
	if err != nil {
		return character
	}
	if gold, ok := goldByCharacter[character.ID]; ok {
		character.PersonalStorageGold = &gold
	}
	return character
}

func enrichCharacters(ctx context.Context, items []characters.Character, metadata *resources.Store) []characters.Character {
	if len(items) == 0 {
		return items
	}
	ids := make([]string, len(items))
	for i := range items {
		ids[i] = items[i].ID
	}
	var goldByCharacter map[string]int64
	if metadata != nil {
		goldByCharacter, _ = metadata.PersonalStorageGoldByCharacters(ctx, ids)
	}
	for i := range items {
		items[i] = characterWithPortrait(items[i], metadata)
		if gold, ok := goldByCharacter[items[i].ID]; ok {
			items[i].PersonalStorageGold = &gold
		}
	}
	return items
}

func enrichGroups(ctx context.Context, groups []characters.Group, metadata *resources.Store) []characters.Group {
	for i := range groups {
		groups[i].Members = enrichCharacters(ctx, groups[i].Members, metadata)
	}
	return groups
}
