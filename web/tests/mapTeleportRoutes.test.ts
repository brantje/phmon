import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapNpc } from '../shared/types/live.ts'
import {
  pickDefaultTeleportDestination,
  sortedTeleportRoutes,
} from '../app/utils/mapTeleportRoutes.ts'

const npc: MapNpc = {
  id: 'gate-1',
  name: 'Hotan',
  servername: 'GATE_KT',
  role: 'teleporter',
  region: 23687,
  x: 1,
  y: 2,
  observed_at: '2026-10-02T10:00:00Z',
  observers: [],
  teleport_routes: [
    { destination: 'Samarkand', teleport_code: 2 },
    { destination: 'Jangan', teleport_code: 1 },
  ],
}

test('sortedTeleportRoutes orders destinations alphabetically', () => {
  assert.deepEqual(
    sortedTeleportRoutes(npc).map((route) => route.destination),
    ['Jangan', 'Samarkand'],
  )
})

test('pickDefaultTeleportDestination prefers Jangan at Hotan when listed', () => {
  assert.equal(pickDefaultTeleportDestination(npc), 'Jangan')
})

test('pickDefaultTeleportDestination keeps a valid custom preference', () => {
  assert.equal(pickDefaultTeleportDestination(npc, 'Samarkand'), 'Samarkand')
})
