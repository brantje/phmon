import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapProfile } from '../shared/types/map.ts'
import { createMapNavigationIntent } from '../app/utils/mapNavigationAction.ts'
import { sendMapClickWalk } from '../app/utils/mapClickWalk.ts'

test('click walk posts all eight targets immediately without controls, freshness or arrival waits', () => {
  const characters = Array.from({ length: 8 }, (_, index) => ({
    character_id: `char-${index}`,
    name: `Char ${index}`,
    server: 'Greatest',
    online: false,
    session_id: `session-${index}`,
    z: index,
  }))
  const profile = {
    tiles: {
      status: 'available-for-inspection',
      orientation_status: 'validated',
      min_x: 26,
      max_x: 252,
      min_y: 35,
      max_y: 126,
    },
    coordinate_transform_status: 'outdoor-region-grid',
    coordinate_transforms: [],
    areas: [
      {
        id: 'world',
        floors: [{ id: 'world', transform_status: 'outdoor-region-grid' }],
      },
    ],
  } as unknown as MapProfile
  const requests: Record<string, unknown>[] = []
  sendMapClickWalk({
    intent: createMapNavigationIntent({
      point: { tileX: 168, tileY: 97, pixelX: 128, pixelY: 128 },
      server: 'Greatest',
      areaID: 'world',
      floorID: 'world',
      explicitRegion: 0,
      datasetID: 'fixture',
      datasetVersion: 'fixture',
      targetIDs: characters.map((row) => row.character_id),
    }),
    profile,
    characters,
    key: () => `key-${requests.length}`,
    post: (request) => {
      requests.push(request)
      return new Promise(() => {})
    },
    failed: () => assert.fail('unexpected failure'),
  })
  assert.equal(requests.length, 8)
  for (let index = 0; index < 8; index++) {
    assert.equal(requests[index]!.name, 'character.move_to')
    assert.deepEqual(requests[index]!.args, { x: 6432, y: 1056, z: index })
    assert.equal(requests[index]!.expected_session_id, `session-${index}`)
  }
})
