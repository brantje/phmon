import assert from 'node:assert/strict'
import test from 'node:test'
import type { ActivityEvent } from '../shared/types/live.ts'
import { eventLocationText } from '../app/utils/event-location.ts'

function event(fields: Partial<ActivityEvent> = {}): ActivityEvent {
  return {
    event_id: 'event',
    schema_version: 1,
    kind: 'character.died',
    category: 'character',
    agent_id: 'agent',
    character_id: 'character',
    session_id: 'session',
    server: 'Silkroad',
    character: 'Alpha',
    occurred_at: '2026-09-29T00:00:00Z',
    received_at: '2026-09-29T00:00:00Z',
    source: 'phbot.callback',
    source_ref: 'EVENT_DIED',
    payload: {},
    ...fields,
  }
}

test('event location shows zone name with observed coordinates', () => {
  assert.equal(
    eventLocationText(event({ zone: 'Jangan', x: 100, y: 200, z: -1 })),
    'Jangan · 100.0, 200.0, -1.0',
  )
})

test('event location uses an unknown zone label for legacy coordinate-only events', () => {
  assert.equal(
    eventLocationText(event({ x: 100, y: 200 })),
    'Unknown zone · 100.0, 200.0, —',
  )
})

test('event location keeps a zone name without coordinates and handles missing data', () => {
  assert.equal(eventLocationText(event({ zone: 'Jangan' })), 'Jangan')
  assert.equal(eventLocationText(event()), 'Location unknown')
})
