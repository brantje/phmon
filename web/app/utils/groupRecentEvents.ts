import type { ActivityEvent } from '~~/shared/types/live'

export interface RecentEventGroup {
  event: ActivityEvent
  observers: Array<{ characterID: string; character: string }>
}

const CHAT_OBSERVATION_WINDOW_MS = 2_000

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
  const groups: Array<RecentEventGroup & { chatKey: string | null }> = []

  for (const event of events) {
    const chatKey = chatObservationKey(event)
    const occurredAt = Date.parse(event.occurred_at)
    const duplicate =
      chatKey && event.character_id && Number.isFinite(occurredAt)
        ? groups.find((group) => {
            if (
              group.chatKey !== chatKey ||
              !Number.isFinite(Date.parse(group.event.occurred_at))
            ) {
              return false
            }
            const withinWindow =
              Math.abs(Date.parse(group.event.occurred_at) - occurredAt) <=
              CHAT_OBSERVATION_WINDOW_MS
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

    groups.push({ event, observers: observerFor(event), chatKey })
  }

  return groups.slice(0, Math.max(0, limit)).map(({ event, observers }) => ({
    event,
    observers,
  }))
}
