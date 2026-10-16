import type { CharacterView as Character } from '~~/shared/types/live'
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
