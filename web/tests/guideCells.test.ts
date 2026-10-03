import assert from 'node:assert/strict'
import test from 'node:test'
import {
  displayedGuideCells,
  placedGuideCell,
} from '../app/utils/guideCells.ts'

test('field mob areas move down by exactly one of their own squares', () => {
  assert.deepEqual(placedGuideCell({ x: 62, y: 85, width: 4, height: 4 }), {
    x: 62,
    y: 81,
    width: 4,
    height: 4,
  })
})

test('tomb edge cells stretch across the unlisted rooms to the floor edge', () => {
  const floor = {
    minX: 117,
    maxX: 138,
    minY: 123,
    maxY: 132,
    guideOriginY: 132,
  }
  const displayed = displayedGuideCells(
    [
      { x: 123, y: 131, width: 1, height: 1 },
      { x: 135, y: 131, width: 1, height: 1 },
      { x: 120, y: 128, width: 1, height: 1 },
      { x: 128, y: 123, width: 1, height: 1 },
    ],
    floor,
  )
  assert.deepEqual(displayed[0], { x: 120, y: 131, width: 4, height: 1 })
  assert.deepEqual(displayed[1], { x: 135, y: 131, width: 4, height: 1 })
  assert.deepEqual(displayed[2], { x: 117, y: 128, width: 7, height: 1 })
  assert.deepEqual(displayed[3], { x: 125, y: 123, width: 7, height: 1 })
})

test('a visible inner cell keeps the server row edge', () => {
  const floor = {
    minX: 117,
    maxX: 138,
    minY: 123,
    maxY: 132,
    guideOriginY: 132,
  }
  assert.deepEqual(
    displayedGuideCells([{ x: 128, y: 125, width: 1, height: 1 }], floor, [
      { y: 125, minX: 120, maxX: 135 },
    ]),
    [{ x: 128, y: 125, width: 1, height: 1 }],
  )
  assert.deepEqual(
    displayedGuideCells([{ x: 120, y: 125, width: 1, height: 1 }], floor, [
      { y: 125, minX: 120, maxX: 135 },
    ]),
    [{ x: 117, y: 125, width: 4, height: 1 }],
  )
})

test('cave mob areas use the floor guide origin instead of the field shift', () => {
  const cell = { x: 126, y: 123, width: 1, height: 1 }
  assert.deepEqual(
    placedGuideCell(cell, { floorMaxY: 130, guideOriginY: 125 }),
    { x: 126, y: 128, width: 1, height: 1 },
  )
  assert.deepEqual(
    placedGuideCell(cell, { floorMaxY: 128, guideOriginY: 128 }),
    cell,
  )
})
