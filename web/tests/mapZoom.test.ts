import assert from 'node:assert/strict'
import test from 'node:test'
import { INITIAL_MAP_ZOOM, MAP_ZOOM_OPTIONS } from '../app/utils/mapZoom.ts'

test('raster map and tile layer share zoom bounds below the GridLayer default', () => {
  assert.deepEqual(MAP_ZOOM_OPTIONS, { minZoom: -1, maxZoom: 4 })
  assert.equal(2 ** INITIAL_MAP_ZOOM, 1.25)
})
