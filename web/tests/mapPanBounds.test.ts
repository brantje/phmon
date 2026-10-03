import assert from 'node:assert/strict'
import test from 'node:test'
import { paddedMapPanBounds } from '../app/utils/mapPanBounds.ts'

const donwhang = { south: -768, west: 0, north: 0, east: 768 }

test('small cave floors can slide until any point reaches the viewport center', () => {
  const padded = paddedMapPanBounds(
    donwhang,
    { width: 1000, height: 800 },
    1.25,
  )
  assert.equal(padded.west, -400)
  assert.equal(padded.east, 1168)
  assert.equal(padded.south, -768)
  assert.equal(padded.north, 0)
})

test('floors larger than the viewport keep their existing edges', () => {
  const world = { south: -20000, west: 0, north: 0, east: 20000 }
  assert.deepEqual(
    paddedMapPanBounds(world, { width: 1000, height: 800 }, 1.25),
    world,
  )
})

test('an unknown viewport does not invent padding', () => {
  assert.deepEqual(
    paddedMapPanBounds(donwhang, { width: 0, height: 800 }, 1.25),
    donwhang,
  )
})
