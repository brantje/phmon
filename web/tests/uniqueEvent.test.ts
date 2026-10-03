import assert from 'node:assert/strict'
import test from 'node:test'
import type { ActivityEvent } from '../shared/types/live.ts'
import {
  dashboardNoisyEvent,
  uniqueEventDetails,
  uniqueEventHeadline,
} from '../app/utils/uniqueEvent.ts'

function uniqueEvent(
  payload: Record<string, unknown>,
  unique?: ActivityEvent['unique'],
): ActivityEvent {
  return {
    event_id: 'event-1',
    schema_version: 1,
    kind: 'world.unique_spawned',
    category: 'world',
    agent_id: 'agent',
    character_id: 'char',
    session_id: 'session',
    server: 'Greatest',
    character: 'nuker1',
    occurred_at: '2026-10-03T11:00:00Z',
    received_at: '2026-10-03T11:00:00Z',
    source: 'joymax.unique_notice',
    source_ref: '0x300C',
    payload,
    unique,
  }
}

test('unique notices use the catalog portrait, name and level', () => {
  const event = uniqueEvent(
    { model: 1954, notice: 'spawn' },
    {
      name: 'Tiger Girl',
      level: 20,
      image_url: '/game-assets/monsters/tigerwoman.png',
      notice: 'spawn',
      model_id: 1954,
    },
  )
  assert.deepEqual(uniqueEventDetails(event), {
    name: 'Tiger Girl',
    level: 20,
    imageUrl: '/game-assets/monsters/tigerwoman.png',
    notice: 'spawn',
    killer: undefined,
  })
  assert.equal(uniqueEventHeadline(event), 'Tiger Girl Lv 20 spawned')
})

test('callback-only unique names stay visible without a portrait', () => {
  const event = uniqueEvent(
    { value: 'Tiger Girl' },
    { name: 'Tiger Girl', notice: 'spawn' },
  )
  event.source = 'phbot.callback'
  event.source_ref = 'EVENT_UNIQUE_SPAWN'
  assert.equal(uniqueEventHeadline(event), 'Tiger Girl spawned')
  assert.equal(uniqueEventDetails(event)?.imageUrl, undefined)
})

test('dashboard recent activity skips inventory and chat noise', () => {
  assert.equal(dashboardNoisyEvent('item.quantity_decreased'), true)
  assert.equal(dashboardNoisyEvent('world.unique_spawned'), false)
  assert.equal(dashboardNoisyEvent('character.died'), false)
})
