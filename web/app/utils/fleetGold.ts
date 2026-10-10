import type { CharacterView, GuildStorageGoldEntry } from '~~/shared/types/live'

export function sumObservedFleetGold(
  characters: CharacterView[],
  guildStorageGold: readonly GuildStorageGoldEntry[],
  matchesServer: (server: string) => boolean,
): number | null {
  const values: number[] = []
  for (const character of characters) {
    if (!matchesServer(character.server)) continue
    if (typeof character.gold === 'number' && Number.isFinite(character.gold)) {
      values.push(character.gold)
    }
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
