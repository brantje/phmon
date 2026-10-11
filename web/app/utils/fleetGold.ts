import type { CharacterView, GuildStorageGoldEntry } from '~~/shared/types/live'

export function personalStorageGold(character: CharacterView): number | null {
  const value = character.personal_storage_gold
  if (typeof value === 'number' && Number.isFinite(value) && value >= 0) {
    return value
  }
  return null
}

/** Inventory gold plus personal storage when both are known; inventory-only when bank is unknown. */
export function combinedCharacterGold(character: CharacterView): number | null {
  const inventory =
    typeof character.gold === 'number' && Number.isFinite(character.gold)
      ? character.gold
      : null
  const storage = personalStorageGold(character)
  if (inventory === null && storage === null) return null
  if (inventory === null) return storage
  if (storage === null) return inventory
  return inventory + storage
}

export function sumObservedFleetGold(
  characters: CharacterView[],
  guildStorageGold: readonly GuildStorageGoldEntry[],
  matchesServer: (server: string) => boolean,
): number | null {
  const values: number[] = []
  for (const character of characters) {
    if (!matchesServer(character.server)) continue
    const combined = combinedCharacterGold(character)
    if (combined !== null) values.push(combined)
  }
  for (const entry of guildStorageGold) {
    if (!matchesServer(entry.server)) continue
    if (
      typeof entry.gold === 'number' &&
      Number.isFinite(entry.gold) &&
      entry.gold >= 0
    ) {
      values.push(entry.gold)
    }
  }
  if (!values.length) return null
  return values.reduce((sum, value) => sum + value, 0)
}

export function formatCompactObservedGold(total: number | null): string {
  if (total === null) return '—'
  return new Intl.NumberFormat('en', {
    notation: 'compact',
    maximumFractionDigits: 1,
  }).format(total)
}
