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
    protocolVersion: 10,
  })
  assert.equal(nearbyStatus, 'available')
  assert.deepEqual(
    options.map((item) => `${item.group}:${item.value}`),
    ['managed:Alpha', 'nearby:Bravo'],
  )
})

test('nearby players unavailable before protocol 10', () => {
  const { nearbyStatus, options } = tracePickerOptions({
    managed: [],
    players: { status: 'observed', players: [] },
    protocolVersion: 9,
  })
  assert.equal(nearbyStatus, 'unavailable')
  assert.equal(options.length, 0)
})
