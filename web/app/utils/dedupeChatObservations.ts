import type { ChatMessage } from '~~/shared/types/live'

const CHAT_OBSERVATION_WINDOW_MS = 2_000

interface MessageGroup {
  message: ChatMessage
  key: string | null
  occurredAt: number
  observers: Set<string>
}

function observationKey(message: ChatMessage) {
  if (
    message.direction !== 'inbound' ||
    (message.channel !== 'general' && message.channel !== 'global') ||
    !message.character_id ||
    !message.server ||
    !message.sender ||
    typeof message.raw_type !== 'string'
  ) {
    return null
  }

  return JSON.stringify([
    message.server.toLowerCase(),
    message.channel,
    message.raw_type,
    message.sender,
    message.message,
  ])
}

export function dedupeChatObservations(messages: readonly ChatMessage[]) {
  const groups: MessageGroup[] = []
  const groupsByKey = new Map<string, MessageGroup[]>()

  // The most recent observer copy is the visible representative of a message.
  for (const message of [...messages].reverse()) {
    const key = observationKey(message)
    const occurredAt = Date.parse(message.occurred_at)
    const candidates = key ? groupsByKey.get(key) || [] : []
    const duplicate =
      key && Number.isFinite(occurredAt)
        ? candidates.find(
            (group) =>
              Number.isFinite(group.occurredAt) &&
              Math.abs(group.occurredAt - occurredAt) <=
                CHAT_OBSERVATION_WINDOW_MS &&
              !group.observers.has(message.character_id),
          )
        : undefined

    if (duplicate) {
      duplicate.observers.add(message.character_id)
      continue
    }

    const group: MessageGroup = {
      message,
      key,
      occurredAt,
      observers: new Set(message.character_id ? [message.character_id] : []),
    }
    groups.push(group)
    if (key) groupsByKey.set(key, [...candidates, group])
  }

  return groups
    .map((group) => group.message)
    .sort(
      (left, right) =>
        Date.parse(left.occurred_at) - Date.parse(right.occurred_at) ||
        left.message_id.localeCompare(right.message_id),
    )
}
