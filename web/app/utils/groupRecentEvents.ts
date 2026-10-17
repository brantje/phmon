import type { ActivityEvent } from '~~/shared/types/live'

export interface RecentEventGroup {
  event: ActivityEvent
  observers: Array<{ characterID: string; character: string }>
}

export interface RecentEventFeed {
  events: readonly ActivityEvent[]
  // The page size requested for this feed.
  limit: number
}

// Merges several latest-N feeds into one newest-first list. A full page only
// reaches back to its oldest row, so rows from other feeds older than that are
// dropped; otherwise the full feed's missing older rows would leave a silent gap.
export function mergeRecentEventFeeds(
  feeds: readonly RecentEventFeed[],
): ActivityEvent[] {
  let cutoff = Number.NEGATIVE_INFINITY
  for (const feed of feeds) {
    if (feed.limit < 1 || feed.events.length < feed.limit) continue
    const times = feed.events
      .map((event) => Date.parse(event.occurred_at))
      .filter(Number.isFinite)
    if (times.length) cutoff = Math.max(cutoff, Math.min(...times))
  }

  const byID = new Map<string, ActivityEvent>()
  for (const feed of feeds) {
    for (const event of feed.events) {
      if (Date.parse(event.occurred_at) < cutoff) continue
      byID.set(event.event_id, event)
    }
  }
  return [...byID.values()].sort(
    (left, right) =>
      Date.parse(right.occurred_at) - Date.parse(left.occurred_at),
  )
}

const CHAT_OBSERVATION_WINDOW_MS = 2_000
const UNIQUE_OBSERVATION_WINDOW_MS = 15_000

function chatObservationKey(event: ActivityEvent) {
  if (event.kind !== 'chat.message_received') return null

  const sender = event.payload.sender
  const message = event.payload.message
  const rawType = event.payload.raw_type
  if (
    typeof sender !== 'string' ||
    sender.length === 0 ||
    typeof message !== 'string' ||
    typeof rawType !== 'string' ||
    event.server.length === 0
  ) {
    return null
  }

  return JSON.stringify([event.server, rawType, sender, message])
}

function uniqueObservationKey(event: ActivityEvent) {
  if (event.kind !== 'world.unique_spawned') return null
  const model = event.payload.model
  const notice = event.payload.notice
  if (
    typeof model !== 'number' ||
    !Number.isInteger(model) ||
    (notice !== 'spawn' && notice !== 'kill') ||
    event.server.length === 0
  ) {
    return null
  }
  return JSON.stringify([event.server.toLowerCase(), notice, model])
}

function observationKey(event: ActivityEvent) {
  return chatObservationKey(event) ?? uniqueObservationKey(event)
}

function observationWindow(event: ActivityEvent) {
  return event.kind === 'world.unique_spawned'
    ? UNIQUE_OBSERVATION_WINDOW_MS
    : CHAT_OBSERVATION_WINDOW_MS
}

function observerFor(event: ActivityEvent) {
  if (!event.character_id) return []
  return [
    {
      characterID: event.character_id,
      character: event.character || 'Character',
    },
  ]
}

export function groupRecentEvents(
  events: readonly ActivityEvent[],
  limit = 5,
): RecentEventGroup[] {
  const groups: Array<RecentEventGroup & { observationKey: string | null }> = []

  for (const event of events) {
    const key = observationKey(event)
    const occurredAt = Date.parse(event.occurred_at)
    const duplicate =
      key && event.character_id && Number.isFinite(occurredAt)
        ? groups.find((group) => {
            if (
              group.observationKey !== key ||
              !Number.isFinite(Date.parse(group.event.occurred_at))
            ) {
              return false
            }
            const withinWindow =
              Math.abs(Date.parse(group.event.occurred_at) - occurredAt) <=
              observationWindow(event)
            return (
              withinWindow &&
              !group.observers.some(
                (observer) => observer.characterID === event.character_id,
              )
            )
          })
        : undefined

    if (duplicate) {
      duplicate.observers.push(...observerFor(event))
      duplicate.observers.sort((left, right) =>
        left.character.localeCompare(right.character),
      )
      continue
    }

    groups.push({ event, observers: observerFor(event), observationKey: key })
  }

  return groups.slice(0, Math.max(0, limit)).map(({ event, observers }) => ({
    event,
    observers,
  }))
}
