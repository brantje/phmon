import type { CharacterView } from '~~/shared/types/live'

export type CharacterDeathState = 'alive' | 'dead' | 'unknown'

const CHARACTER_STATE_FRESH_MS = 30_000
const CLOCK_SKEW_ALLOWANCE_MS = 5_000

export function characterDeathState(
  character: Pick<CharacterView, 'online' | 'dead' | 'state_updated_at'>,
  stale = false,
  now = Date.now(),
): CharacterDeathState {
  if (stale || !character.online || typeof character.dead !== 'boolean')
    return 'unknown'
  if (!character.state_updated_at) return 'unknown'
  const observed = Date.parse(character.state_updated_at)
  if (!Number.isFinite(observed)) return 'unknown'
  const age = now - observed
  if (age > CHARACTER_STATE_FRESH_MS || age < -CLOCK_SKEW_ALLOWANCE_MS)
    return 'unknown'
  return character.dead ? 'dead' : 'alive'
}
