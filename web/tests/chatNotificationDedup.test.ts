import assert from 'node:assert/strict'
import test from 'node:test'
import type { NotificationEvent } from '../shared/types/live.ts'
import { createChatNotificationDeduper } from '../app/utils/chatNotificationDedup.ts'

function chatEvent(
  observer: string,
  occurredAt: string,
  overrides: Partial<NotificationEvent> = {},
): NotificationEvent {
  return {
    event_id: `${observer}-${occurredAt}`,
    kind: 'chat.message_received',
    category: 'chat',
    server: 'Greatest',
    character_id: observer,
    character: observer,
    occurred_at: occurredAt,
    payload: {
      channel: 'global',
      raw_type: '6',
      sender: 'HighPriestes',
      message: 'WTS any seal of star for 10 silk 12D',
    },
    ...overrides,
  }
}

test('one broadcast observed by two characters produces one notification', () => {
  const shouldShow = createChatNotificationDeduper()
  assert.equal(shouldShow(chatEvent('Kalypso', '2026-10-16T10:00:00Z')), true)
  assert.equal(
    shouldShow(
      chatEvent('Odin', '2026-10-16T10:00:00.100Z', {
        server: 'greatest',
      }),
    ),
    false,
  )
})

test('a repeat from the same observer stays distinct and copies match the nearest repeat', () => {
  const shouldShow = createChatNotificationDeduper()
  assert.equal(shouldShow(chatEvent('Kalypso', '2026-10-16T10:00:00Z')), true)
  assert.equal(
    shouldShow(chatEvent('Kalypso', '2026-10-16T10:00:00.500Z')),
    true,
  )
  assert.equal(shouldShow(chatEvent('Odin', '2026-10-16T10:00:00.100Z')), false)
  assert.equal(shouldShow(chatEvent('Odin', '2026-10-16T10:00:00.600Z')), false)
  assert.equal(shouldShow(chatEvent('Kalypso', '2026-10-16T10:00:03Z')), true)
})

test('different broadcasts and private observations remain separate', () => {
  const base = chatEvent('Kalypso', '2026-10-16T10:00:00Z')
  const variants: Partial<NotificationEvent>[] = [
    { server: 'Other' },
    { payload: { ...base.payload, channel: 'general' } },
    { payload: { ...base.payload, raw_type: '7' } },
    { payload: { ...base.payload, sender: 'OtherSender' } },
    { payload: { ...base.payload, message: 'Different text' } },
  ]
  for (const variant of variants) {
    const shouldShow = createChatNotificationDeduper()
    assert.equal(shouldShow(base), true)
    assert.equal(
      shouldShow(chatEvent('Odin', '2026-10-16T10:00:00.100Z', variant)),
      true,
    )
  }

  const shouldShowPrivate = createChatNotificationDeduper()
  const privatePayload = { ...base.payload, channel: 'private' }
  assert.equal(shouldShowPrivate({ ...base, payload: privatePayload }), true)
  assert.equal(
    shouldShowPrivate(
      chatEvent('Odin', '2026-10-16T10:00:00.100Z', {
        payload: privatePayload,
      }),
    ),
    true,
  )
})
