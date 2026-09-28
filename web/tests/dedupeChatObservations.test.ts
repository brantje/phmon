import assert from 'node:assert/strict'
import test from 'node:test'
import type { ChatMessage } from '../shared/types/live.ts'
import { dedupeChatObservations } from '../app/utils/dedupeChatObservations.ts'

function message(
  messageID: string,
  characterID: string,
  occurredAt: string,
  overrides: Partial<ChatMessage> = {},
): ChatMessage {
  return {
    message_id: messageID,
    event_id: messageID,
    character_id: characterID,
    server: 'Greatest',
    character: characterID,
    channel: 'general',
    direction: 'inbound',
    raw_type: '1',
    sender: 'Veyra',
    message: 'Hello',
    state: 'received',
    occurred_at: occurredAt,
    ...overrides,
  }
}

test('deduplicates one General/Global message observed by different characters', () => {
  const messages = [
    message('alpha', 'alpha-id', '2026-09-28T16:25:49.050Z'),
    message('beta', 'beta-id', '2026-09-28T16:25:49.200Z'),
  ]

  const [group] = dedupeChatObservations(messages)
  assert.equal(dedupeChatObservations(messages).length, 1)
  assert.equal(group.message_id, 'beta')

  const globalCopies = messages.map((item) => ({
    ...item,
    channel: 'global',
    raw_type: '6',
  }))
  assert.equal(dedupeChatObservations(globalCopies).length, 1)
})

test('keeps repeated messages by the same character as separate occurrences', () => {
  const messages = [
    message('first', 'alpha-id', '2026-09-28T16:25:49.050Z'),
    message('second', 'alpha-id', '2026-09-28T16:25:49.200Z'),
  ]

  assert.equal(dedupeChatObservations(messages).length, 2)
})

test('pairs observer copies one-to-one and preserves distinct chat messages', () => {
  const messages = [
    message('alpha-first', 'alpha-id', '2026-09-28T16:25:49.000Z'),
    message('beta-first', 'beta-id', '2026-09-28T16:25:49.100Z'),
    message('alpha-second', 'alpha-id', '2026-09-28T16:25:49.500Z'),
    message('beta-second', 'beta-id', '2026-09-28T16:25:49.600Z'),
    message('other-channel', 'beta-id', '2026-09-28T16:25:49.150Z', {
      channel: 'party',
      raw_type: '3',
    }),
    message('other-sender', 'beta-id', '2026-09-28T16:25:49.150Z', {
      sender: 'Veyra2',
    }),
    message('too-late', 'beta-id', '2026-09-28T16:25:52.000Z'),
  ]

  const result = dedupeChatObservations(messages)
  assert.equal(result.length, 5)
  assert.deepEqual(
    result.map((item) => item.message_id),
    ['beta-first', 'other-channel', 'other-sender', 'beta-second', 'too-late'],
  )
})
