import test from 'node:test'
import assert from 'node:assert/strict'
import { mapCharacterClusters } from '../app/utils/mapCharacterClusters.ts'
test('overlap uses screen anchors and keeps separated portraits independent', () => {
  assert.deepEqual(
    mapCharacterClusters([
      { id: 'b', x: 0, y: 0 },
      { id: 'a', x: 24, y: 0 },
      { id: 'c', x: 70, y: 0 },
    ]),
    [['a', 'b']],
  )
  assert.deepEqual(
    mapCharacterClusters([
      { id: 'b', x: 0, y: 0 },
      { id: 'a', x: 25, y: 0 },
    ]),
    [],
  )
})
