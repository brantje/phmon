import assert from 'node:assert/strict'
import test from 'node:test'
import {
  mapNavigationTrayRows,
  navigationTrayGroup,
} from '../app/utils/mapNavigationTray.ts'
import type { FanOutOperation } from '../app/utils/commandFanOut.ts'
import type { MapRouteOverlay } from '../app/utils/mapNavigationRoutes.ts'
const route: MapRouteOverlay = {
  id: 'c:s:1',
  characterID: 'c',
  characterName: 'Alpha',
  sessionID: 's',
  commandID: 'command',
  status: 'moving',
  stale: false,
  selected: true,
  blocks: [],
}
const operation = {
  operationID: 'op',
  command: { name: 'character.navigate' },
  state: 'tracking',
  children: [
    {
      characterID: 'c',
      characterName: 'Alpha',
      server: 'Greatest',
      sessionID: 's',
      commandID: 'command',
      submission: 'accepted',
      executionState: 'completed',
    },
  ],
} as FanOutOperation

test('backend routes replace only matching session and command submissions', () => {
  assert.equal(
    mapNavigationTrayRows([route], [operation], 'greatest', new Set()).length,
    1,
  )
  assert.equal(
    mapNavigationTrayRows(
      [{ ...route, sessionID: 'replacement' }],
      [operation],
      'greatest',
      new Set(),
    ).length,
    2,
  )
})
test('command completion does not fabricate arrival; dismissals do not hide replacement routes', () => {
  assert.equal(
    mapNavigationTrayRows([], [operation], 'greatest', new Set())[0]?.status,
    'waiting_for_movement',
  )
  assert.equal(
    mapNavigationTrayRows([route], [], 'greatest', new Set([route.id])).length,
    0,
  )
  assert.equal(
    mapNavigationTrayRows(
      [{ ...route, id: 'c:s:2' }],
      [],
      'greatest',
      new Set([route.id]),
    ).length,
    1,
  )
  assert.equal(
    mapNavigationTrayRows([], [operation], 'other server', new Set()).length,
    0,
  )
  assert.equal(navigationTrayGroup('arrived'), 'done')
  assert.equal(navigationTrayGroup('progress_uncertain'), 'attention')
})

test('navigation tray exposes progress and stop metadata from backend routes', () => {
  const rows = mapNavigationTrayRows(
    [route],
    [],
    'greatest',
    new Set(),
    [
      {
        command_id: 'command',
        character_id: 'c',
        session_id: 's',
        route_sequence: 1,
        server: 'Greatest',
        dataset_id: 'd',
        dataset_version: 'v',
        destination: { region: 1, x: 1, y: 1, z: 0 },
        status: 'moving',
        updated_at: '2026-01-01T00:00:00Z',
        blocks: [],
        progress: 0.42,
        eta_seconds: 30,
      },
    ],
    { c: true },
  )
  assert.equal(rows[0]?.progress, 0.42)
  assert.equal(rows[0]?.etaSeconds, 30)
  assert.equal(rows[0]?.canStop, true)
})

test('unassigned command IDs never suppress a submitted child row', () => {
  for (const commandID of [undefined, '']) {
    const pending = {
      ...operation,
      children: [{ ...operation.children[0]!, commandID }],
    }
    assert.equal(
      mapNavigationTrayRows(
        [{ ...route, commandID: '' }],
        [pending],
        'greatest',
        new Set(),
      ).length,
      2,
    )
  }
})
