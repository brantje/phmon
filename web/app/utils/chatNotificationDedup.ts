import type { NotificationEvent } from '~~/shared/types/live'

// Chat history collapses server-wide copies from different observers within 2s.
// The live notification stream carries the underlying observations, so its
// presentation needs the same boundary without discarding durable events.
const observationWindowMs = 2000
const retentionMs = 30000
const maxGroups = 512

type Group = {
  key: string
  occurredAt: number
  receivedAt: number
  observers: Set<string>
}

export function createChatNotificationDeduper() {
  const groups: Group[] = []

  return (event: NotificationEvent, receivedAt = Date.now()): boolean => {
    if (event.kind !== 'chat.message_received') return true
    const { channel, raw_type: rawType, sender, message } = event.payload
    if (
      typeof channel !== 'string' ||
      ['private', 'guild', 'union'].includes(channel) ||
      typeof sender !== 'string' ||
      !sender ||
      typeof message !== 'string' ||
      !message ||
      !event.character_id
    )
      return true

    const occurredAt = Date.parse(event.occurred_at)
    if (!Number.isFinite(occurredAt)) return true
    const key = JSON.stringify([
      channel,
      event.server.toLowerCase(),
      typeof rawType === 'string' ? rawType : '',
      sender,
      message,
    ])

    for (let index = groups.length - 1; index >= 0; index--) {
      if (receivedAt - groups[index]!.receivedAt > retentionMs)
        groups.splice(index, 1)
    }

    let match: Group | undefined
    let nearest = observationWindowMs + 1
    for (const group of groups) {
      if (group.key !== key || group.observers.has(event.character_id)) continue
      const distance = Math.abs(group.occurredAt - occurredAt)
      if (distance <= observationWindowMs && distance < nearest) {
        match = group
        nearest = distance
      }
    }
    if (match) {
      match.observers.add(event.character_id)
      match.receivedAt = receivedAt
      return false
    }

    groups.push({
      key,
      occurredAt,
      receivedAt,
      observers: new Set([event.character_id]),
    })
    if (groups.length > maxGroups) groups.shift()
    return true
  }
}
