import type { ActivityEvent } from '~~/shared/types/live'

export interface AlchemyAttemptSegment {
  key: string
  character: string
  characterID: string
  sessionID: string
  item: string
  slot: number | null
  firstAt: string
  lastAt: string
  attempts: number
  successes: number
  failures: number
  unknown: number
  ambiguous: true
}

const MAX_CONTINUITY_GAP_MS = 60_000

function stableValue(value: unknown): string {
  if (Array.isArray(value)) return `[${value.map(stableValue).join(',')}]`
  if (value && typeof value === 'object') {
    return `{${Object.entries(value)
      .sort(([left], [right]) => left.localeCompare(right))
      .map(([key, entry]) => `${key}:${stableValue(entry)}`)
      .join(',')}}`
  }
  return JSON.stringify(value) ?? 'null'
}

function attemptFingerprint(event: ActivityEvent) {
  const item = event.payload.item
  if (!item || typeof item !== 'object' || Array.isArray(item)) return null
  const record = item as Record<string, unknown>
  const stableTraits = [
    'degree',
    'seal',
    'blues',
    'magic_options',
    'attributes',
  ]
    .filter((key) => record[key] !== undefined && record[key] !== null)
    .map((key) => `${key}=${stableValue(record[key])}`)
  // A model/code and bag slot alone are common to many physical items. Require
  // an additional stable item trait before joining attempts into a candidate run.
  if (!stableTraits.length) return null
  const model = event.item_model ?? record.model
  const code = event.item_code ?? record.servername
  return `${String(model ?? '')}|${String(code ?? '')}|${stableTraits.join('|')}`
}

function itemLabel(event: ActivityEvent) {
  return (
    event.item_name ||
    event.item_code ||
    event.payload.item_name?.toString() ||
    'Item details unavailable'
  )
}

/**
 * Return bounded-page candidates only. A matching fingerprint is not persistent
 * item identity, so every segment remains explicitly ambiguous and must not be
 * used to calculate target probabilities or attempts-to-target.
 */
export function buildAlchemyAttemptSegments(events: readonly ActivityEvent[]) {
  const attempts = [...events].sort(
    (left, right) =>
      left.occurred_at.localeCompare(right.occurred_at) ||
      left.event_id.localeCompare(right.event_id),
  )
  const segments: AlchemyAttemptSegment[] = []

  for (const event of attempts) {
    const slot = event.payload.slot
    const validSlot =
      typeof slot === 'number' && Number.isInteger(slot) && slot >= 0
    const fingerprint = validSlot ? attemptFingerprint(event) : null
    const at = Date.parse(event.occurred_at)
    const previous = segments[segments.length - 1]
    const mayContinue =
      fingerprint !== null &&
      previous !== undefined &&
      previous.sessionID === event.session_id &&
      previous.characterID === event.character_id &&
      previous.slot === slot &&
      previous.key.endsWith(fingerprint) &&
      at - Date.parse(previous.lastAt) <= MAX_CONTINUITY_GAP_MS

    if (mayContinue) {
      previous.attempts++
      previous.lastAt = event.occurred_at
      if (event.payload.success === true) previous.successes++
      else if (event.payload.success === false) previous.failures++
      else previous.unknown++
      continue
    }

    const state = event.payload.success
    segments.push({
      key: `${event.session_id}|${event.character_id}|${validSlot ? slot : 'unknown'}|${fingerprint ?? event.event_id}`,
      character: event.character,
      characterID: event.character_id,
      sessionID: event.session_id,
      item: itemLabel(event),
      slot: validSlot ? slot : null,
      firstAt: event.occurred_at,
      lastAt: event.occurred_at,
      attempts: 1,
      successes: state === true ? 1 : 0,
      failures: state === false ? 1 : 0,
      unknown: state === true || state === false ? 0 : 1,
      ambiguous: true,
    })
  }

  return segments.reverse()
}
