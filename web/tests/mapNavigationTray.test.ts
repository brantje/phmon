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
