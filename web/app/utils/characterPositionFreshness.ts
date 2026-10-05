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
  // A stalled UI clock makes a fresh sample look like it is in the future.
  // Keep that as live so markers do not blink out as "Stale 0s".
  return now - observedAt <= MAP_POSITION_FRESH_MS
}
