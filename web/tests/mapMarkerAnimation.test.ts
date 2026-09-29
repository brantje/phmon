import assert from 'node:assert/strict'
import test from 'node:test'
import { interpolateMarkerPosition } from '../app/utils/mapMarkerAnimation.ts'

test('marker interpolation moves smoothly between snapshot positions', () => {
  assert.deepEqual(
    interpolateMarkerPosition({ lat: 0, lng: 10 }, { lat: 20, lng: 30 }, 0.5),
    { lat: 10, lng: 20 },
  )
  assert.deepEqual(
    interpolateMarkerPosition({ lat: 0, lng: 10 }, { lat: 20, lng: 30 }, 1),
    { lat: 20, lng: 30 },
  )
})

test('small live marker movements are interpolated instead of threshold-snapped', () => {
  assert.deepEqual(
    interpolateMarkerPosition(
      { lat: 0, lng: 0 },
      { lat: 0.25, lng: 0.25 },
      0.5,
    ),
    { lat: 0.125, lng: 0.125 },
  )
})

test('marker interpolation clamps progress to the transition interval', () => {
  const start = { lat: 0, lng: 10 }
  const target = { lat: 20, lng: 30 }
  assert.deepEqual(interpolateMarkerPosition(start, target, -1), start)
  assert.deepEqual(interpolateMarkerPosition(start, target, 2), target)
})
