import assert from 'node:assert/strict'
import test from 'node:test'
import type { ActivityEvent } from '../shared/types/live.ts'
import { buildAlchemyAttemptSegments } from '../app/utils/alchemyAttemptSegments.ts'

function attempt(overrides: Partial<ActivityEvent> = {}): ActivityEvent {
  return {
    event_id: crypto.randomUUID(),
    schema_version: 1,
    kind: 'alchemy.attempt',
    category: 'alchemy',
    agent_id: 'agent',
    character_id: 'character',
    session_id: 'session',
    server: 'test',
    character: 'Aria',
    occurred_at: '2026-10-07T10:00:00.000Z',
    received_at: '2026-10-07T10:00:00.000Z',
    source: 'phbot.alchemy_callback',
    source_ref: 'alchemy_update',
    item_model: 77,
    item_code: 'ITEM_TEST',
    item_name: 'Fixture item',
    payload: {
      slot: 3,
      success: true,
      item: { model: 77, servername: 'ITEM_TEST', degree: 10, plus: 4 },
    },
    ...overrides,
  }
}

test('candidate runs require matching fingerprint, session, slot and a short gap', () => {
  const first = attempt({ event_id: '1' })
  const second = attempt({
    event_id: '2',
    occurred_at: '2026-10-07T10:00:30.000Z',
    payload: {
      ...first.payload,
      success: false,
      item: { ...(first.payload.item as object), plus: 5 },
    },
  })
  const replacement = attempt({
    event_id: '3',
    occurred_at: '2026-10-07T10:00:40.000Z',
    payload: {
      ...first.payload,
      item: { model: 77, servername: 'ITEM_TEST', degree: 11, plus: 0 },
    },
  })
  const reconnected = attempt({
    event_id: '4',
    session_id: 'new-session',
    occurred_at: '2026-10-07T10:00:45.000Z',
  })
  const gap = attempt({
    event_id: '5',
    occurred_at: '2026-10-07T10:02:00.000Z',
  })
  const segments = buildAlchemyAttemptSegments([
    gap,
    reconnected,
    replacement,
    second,
    first,
  ])

  assert.equal(segments.length, 4)
  assert.equal(segments[3]?.attempts, 2)
  assert.equal(segments[3]?.successes, 1)
  assert.equal(segments[3]?.failures, 1)
  assert.ok(segments.every((segment) => segment.ambiguous))
})

test('missing item continuity or fingerprints keep attempts separate and unknown outcomes distinct', () => {
  const missing = attempt({
    event_id: 'missing',
    payload: { slot: 3, success: null },
  })
  const noStableTraits = attempt({
    event_id: 'traits',
    payload: {
      slot: 3,
      success: null,
      item: { model: 77, servername: 'ITEM_TEST' },
    },
  })
  const segments = buildAlchemyAttemptSegments([missing, noStableTraits])

  assert.equal(segments.length, 2)
  assert.equal(segments[0]?.unknown, 1)
  assert.equal(segments[1]?.unknown, 1)
  assert.ok(
    segments.every((segment) => segment.attempts === 1 && segment.ambiguous),
  )
})
