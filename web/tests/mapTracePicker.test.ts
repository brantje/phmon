import assert from 'node:assert/strict'
import test from 'node:test'
import { tracePickerOptions } from '../app/utils/mapTracePicker.ts'

test('trace picker groups managed characters and deduped nearby players', () => {
  const { options, nearbyStatus } = tracePickerOptions({
    managed: [
      {
        character_id: 'a',
        name: 'Alpha',
        server: 'Greatest',
        online: true,
      } as const,
    ],
    players: {
      status: 'observed',
      players: [
        {
          id: 'p1',
          player_id: '1',
          name: 'Bravo',
          region: 1,
          x: 1,
          y: 1,
          observer_region: 1,
          observed_at: '2026-01-01T00:00:00Z',
          observers: [],
        },
        {
          id: 'p2',
          player_id: '2',
          name: 'Alpha',
          region: 1,
          x: 2,
          y: 2,
          observer_region: 1,
          observed_at: '2026-01-01T00:00:00Z',
          observers: [],
        },
      ],
    },
  })
  assert.equal(nearbyStatus, 'available')
  assert.deepEqual(
    options.map((item) => `${item.group}:${item.value}`),
    ['managed:Alpha', 'nearby:Bravo'],
  )
})

test('nearby players empty when snapshot is observed with no rows', () => {
  const { nearbyStatus, options } = tracePickerOptions({
    managed: [],
    players: { status: 'observed', players: [] },
  })
  assert.equal(nearbyStatus, 'empty')
  assert.equal(options.length, 0)
})

test('nearby players unavailable when map projection is unavailable', () => {
  const { nearbyStatus } = tracePickerOptions({
    managed: [],
    players: { status: 'unavailable', players: [] },
  })
  assert.equal(nearbyStatus, 'unavailable')
})
