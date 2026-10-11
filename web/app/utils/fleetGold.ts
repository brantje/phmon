import type { CharacterView, GuildStorageGoldEntry } from '~~/shared/types/live'

export function validObservedGold(value: unknown): value is number {
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
}

export function characterGoldTotal(character: CharacterView): number | null {
  const values = [character.gold, character.storage_gold].filter(
    validObservedGold,
  )
  return values.length ? values.reduce((sum, value) => sum + value, 0) : null
}

export function characterGoldStatus(
  characters: readonly CharacterView[],
): string {
  const incomplete = characters.some(
    (character) =>
      !validObservedGold(character.gold) ||
      !validObservedGold(character.storage_gold),
  )
  const stale = characters.some(
    (character) =>
      validObservedGold(character.storage_gold) &&
      (!character.online || character.storage_gold_availability !== 'observed'),
  )
  return [incomplete ? 'incomplete' : '', stale ? 'last known bank' : '']
    .filter(Boolean)
    .join(' · ')
}

export function formatCharacterGoldTotal(
  characters: readonly CharacterView[],
): string {
  const values = characters.map(characterGoldTotal).filter(validObservedGold)
  if (!values.length) return '—'
  const total = values.reduce((sum, value) => sum + value, 0).toLocaleString()
  const status = characterGoldStatus(characters)
  return status ? `${total} · ${status}` : total
}

export function sumObservedFleetGold(
  characters: CharacterView[],
  guildStorageGold: readonly GuildStorageGoldEntry[],
  matchesServer: (server: string) => boolean,
): number | null {
  const values: number[] = []
  for (const character of characters) {
    if (!matchesServer(character.server)) continue
    const total = characterGoldTotal(character)
    if (total !== null) values.push(total)
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
