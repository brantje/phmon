export const MAP_POSITION_FRESH_MS = 35_000

export function characterStateObservedAt(character: {
  state_updated_at?: string
}) {
  return character.state_updated_at
    ? Date.parse(character.state_updated_at)
    : Number.NaN
}

export function characterPositionIsFresh(
  character: { online?: boolean; state_updated_at?: string },
  now = Date.now(),
) {
  const observedAt = characterStateObservedAt(character)
  if (!character.online || !Number.isFinite(observedAt)) return false
  const age = now - observedAt
  // Allow bounded UI clock lag while preserving the live window.
  return age >= -MAP_POSITION_FRESH_MS && age <= MAP_POSITION_FRESH_MS
}
