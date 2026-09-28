import assert from 'node:assert/strict'
import test from 'node:test'
import type { ActivityEvent } from '../shared/types/live.ts'
import { groupRecentEvents } from '../app/utils/groupRecentEvents.ts'

function chatEvent(
  eventID: string,
  characterID: string,
  character: string,
  occurredAt: string,
  payload: Record<string, unknown> = {
    channel: 'unknown',
    raw_type: 'global',
    sender: 'nuker3',
    message: 'hi',
  },
  server = 'Zerkroad',
): ActivityEvent {
  return {
    event_id: eventID,
    schema_version: 1,
    kind: 'chat.message_received',
    category: 'chat',
    agent_id: `agent-${characterID}`,
    character_id: characterID,
    session_id: `session-${characterID}`,
    server,
    character,
    occurred_at: occurredAt,
    received_at: occurredAt,
    source: 'phbot.chat_callback',
    source_ref: 'handle_chat',
    payload,
  }
}

test('groups one chat occurrence observed by multiple characters', () => {
  const events = [
    chatEvent('event-2', 'char-2', 'nuker2', '2026-09-28T16:25:49.200Z'),
    chatEvent('event-1', 'char-1', 'nuker1', '2026-09-28T16:25:49.050Z'),
  ]

  const [group] = groupRecentEvents(events)
  assert.equal(group.event.event_id, 'event-2')
  assert.deepEqual(
    group.observers.map((observer) => observer.character),
    ['nuker1', 'nuker2'],
  )
})

test('keeps repeated messages from the same character as separate events', () => {
  const events = [
    chatEvent('event-2', 'char-1', 'nuker1', '2026-09-28T16:25:49.200Z'),
    chatEvent('event-1', 'char-1', 'nuker1', '2026-09-28T16:25:49.050Z'),
  ]

  assert.equal(groupRecentEvents(events).length, 2)
})

test('keeps messages separate when text, sender, channel, server, or timing differs', () => {
  const baseTime = '2026-09-28T16:25:49.000Z'
  const events = [
    chatEvent('event-1', 'char-1', 'nuker1', baseTime),
    chatEvent('event-2', 'char-2', 'nuker2', baseTime, {
      channel: 'unknown',
      raw_type: 'global',
      sender: 'nuker3',
      message: 'hello',
    }),
    chatEvent('event-3', 'char-3', 'nuker3', baseTime, {
      channel: 'unknown',
      raw_type: 'global',
      sender: 'nuker4',
      message: 'hi',
    }),
    chatEvent('event-4', 'char-4', 'nuker4', baseTime, {
      channel: 'unknown',
      raw_type: 'party',
      sender: 'nuker3',
      message: 'hi',
    }),
    chatEvent('event-5', 'char-5', 'nuker5', baseTime, undefined, 'Oasis'),
    chatEvent('event-6', 'char-6', 'nuker6', '2026-09-28T16:25:52.000Z'),
  ]

  assert.equal(groupRecentEvents(events, 10).length, events.length)
})

test('applies the dashboard limit after merging observer copies', () => {
  const events = [
    chatEvent('duplicate-1', 'char-1', 'nuker1', '2026-09-28T16:25:49.100Z'),
    chatEvent('duplicate-2', 'char-2', 'nuker2', '2026-09-28T16:25:49.050Z'),
    ...Array.from({ length: 5 }, (_, index) => ({
      ...chatEvent(
        `event-${index}`,
        `char-${index + 3}`,
        `nuker${index + 3}`,
        `2026-09-28T16:25:${48 - index}.000Z`,
      ),
      payload: {
        channel: 'unknown',
        raw_type: 'global',
        sender: `player${index}`,
        message: `message ${index}`,
      },
    })),
  ]

  assert.equal(groupRecentEvents(events, 5).length, 5)
})
