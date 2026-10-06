import assert from 'node:assert/strict'
import test from 'node:test'
import {
  INITIAL_MAP_ZOOM,
  MAP_ZOOM_OPTIONS,
  mapCanvasInitialZoomLevel,
  mapZoomLevelForPercent,
  mapZoomPercentForLevel,
  snapMapZoomPercent,
} from '../app/utils/mapZoom.ts'

test('raster map and tile layer share the requested percentage zoom bounds', () => {
  assert.equal(mapZoomPercentForLevel(MAP_ZOOM_OPTIONS.minZoom), 25)
  assert.equal(mapZoomPercentForLevel(MAP_ZOOM_OPTIONS.maxZoom), 2000)
  assert.equal(mapZoomPercentForLevel(INITIAL_MAP_ZOOM), 125)
})

test('leaflet zoom levels snap back to 5 percentage point increments', () => {
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
  assert.equal(snapMapZoomPercent(30), 30)
  assert.equal(snapMapZoomPercent(5), 25)
  assert.equal(snapMapZoomPercent(0), 25)
  assert.equal(snapMapZoomPercent(2100), 2000)
})

test('map canvas initial zoom keeps a stored percent and falls back otherwise', () => {
  assert.equal(mapCanvasInitialZoomLevel(), INITIAL_MAP_ZOOM)
  assert.equal(mapCanvasInitialZoomLevel(Number.NaN), INITIAL_MAP_ZOOM)
  assert.equal(mapCanvasInitialZoomLevel(150), mapZoomLevelForPercent(150))
  assert.equal(mapCanvasInitialZoomLevel(5), mapZoomLevelForPercent(25))
})
