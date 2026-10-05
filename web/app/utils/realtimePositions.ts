import type {
  RealtimePosition,
  RealtimePositionDelta,
} from '~~/shared/types/live'

export type RealtimePositionState = Record<string, RealtimePosition>

export function validRealtimePosition(
  position: unknown,
): position is RealtimePosition {
  if (!position || typeof position !== 'object' || Array.isArray(position))
    return false
  const value = position as RealtimePosition
  return (
    typeof value.character_id === 'string' &&
    value.character_id.length > 0 &&
    typeof value.session_id === 'string' &&
    value.session_id.length > 0 &&
    Number.isSafeInteger(value.sequence) &&
    value.sequence > 0 &&
    Number.isInteger(value.region) &&
    value.region !== 0 &&
    value.region >= -32768 &&
    value.region <= 65535 &&
    Number.isFinite(value.x) &&
    Math.abs(value.x) <= 10_000_000 &&
    Number.isFinite(value.y) &&
    Math.abs(value.y) <= 10_000_000 &&
    (value.z == null ||
      (Number.isFinite(value.z) && Math.abs(value.z) <= 10_000_000)) &&
    typeof value.observed_at === 'string' &&
    Number.isFinite(Date.parse(value.observed_at))
  )
}

export function realtimePositionSnapshotState(
  positions: readonly RealtimePosition[],
  current: RealtimePositionState = {},
): RealtimePositionState {
  const next: RealtimePositionState = {}
  for (const position of positions) {
    if (!validRealtimePosition(position)) continue
    const pending = next[position.character_id]
    const existing = current[position.character_id]
    if (
      pending &&
      pending.session_id === position.session_id &&
      pending.sequence >= position.sequence
    )
      continue
    if (
      existing &&
      existing.session_id === position.session_id &&
      existing.sequence > position.sequence
    ) {
      next[position.character_id] = existing
      continue
    }
    next[position.character_id] = position
  }
  return next
}

export function applyRealtimePositionDeltaState(
  current: RealtimePositionState,
  delta: RealtimePositionDelta,
): RealtimePositionState {
  const next = { ...current }

  // Remove first so a same-batch session replacement can be admitted.
  for (const removal of delta.removed || []) {
    if (
      !removal ||
      typeof removal.character_id !== 'string' ||
      typeof removal.session_id !== 'string'
    )
      continue
    if (next[removal.character_id]?.session_id === removal.session_id)
      delete next[removal.character_id]
  }

  for (const position of delta.positions || []) {
    if (!validRealtimePosition(position)) continue
    const previous = next[position.character_id]
    if (previous) {
      if (previous.session_id !== position.session_id) continue
      if (position.sequence <= previous.sequence) continue
    }
    next[position.character_id] = position
  }
  return next
}
