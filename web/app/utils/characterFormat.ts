import type { CharacterView as Character } from '~~/shared/types/live'
export function formatTimestamp(value?: string) {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'
  return date.toISOString().replace('T', ' ').replace('.000Z', 'Z')
}

// Compact local date/time used for container "Last synced" labels.
export function formatSyncedTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime())
    ? 'Unknown'
    : date.toLocaleString(undefined, {
        month: 'short',
        day: 'numeric',
        year: 'numeric',
        hour: 'numeric',
        minute: '2-digit',
      })
}

export function formatHealthMana(character: Character) {
  return `HP ${character.hp?.toLocaleString() ?? '—'} / ${character.hp_max?.toLocaleString() ?? '—'} · MP ${character.mp?.toLocaleString() ?? '—'} / ${character.mp_max?.toLocaleString() ?? '—'}`
}

export function formatProgress(character: Character) {
  const xp =
    character.current_exp == null
      ? '—'
      : `${character.current_exp.toLocaleString()} / ${character.max_exp?.toLocaleString() ?? '—'} XP`
  const ratio =
    character.current_exp != null &&
    character.max_exp != null &&
    character.max_exp > 0
      ? ` (${Math.min(100, Math.round((character.current_exp / character.max_exp) * 100))}%)`
      : ''
  return `${xp}${ratio} · ${character.sp?.toLocaleString() ?? '—'} SP`
}
