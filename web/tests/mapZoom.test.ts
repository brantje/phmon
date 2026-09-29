import assert from 'node:assert/strict'
import test from 'node:test'
import {
  INITIAL_MAP_ZOOM,
  MAP_ZOOM_OPTIONS,
  mapZoomLevelForPercent,
  mapZoomPercentForLevel,
  snapMapZoomPercent,
} from '../app/utils/mapZoom.ts'

test('raster map and tile layer share the requested percentage zoom bounds', () => {
  assert.equal(mapZoomPercentForLevel(MAP_ZOOM_OPTIONS.minZoom), 50)
  assert.equal(mapZoomPercentForLevel(MAP_ZOOM_OPTIONS.maxZoom), 2000)
  assert.equal(mapZoomPercentForLevel(INITIAL_MAP_ZOOM), 125)
})

test('leaflet zoom levels snap back to 25 percentage point increments', () => {
  assert.equal(
    snapMapZoomPercent(mapZoomPercentForLevel(INITIAL_MAP_ZOOM + 0.25)),
    150,
  )
  assert.equal(
    snapMapZoomPercent(
      mapZoomPercentForLevel(mapZoomLevelForPercent(150) - 0.25),
    ),
    125,
  )
  assert.equal(snapMapZoomPercent(30), 50)
  assert.equal(snapMapZoomPercent(2100), 2000)
})
