import assert from 'node:assert/strict'
import test from 'node:test'
import { guideCellOneSquareDown } from '../app/utils/guideCells.ts'

test('mob areas move down by exactly one of their own squares', () => {
  assert.deepEqual(
    guideCellOneSquareDown({ x: 62, y: 85, width: 4, height: 4 }),
    { x: 62, y: 81, width: 4, height: 4 },
  )
  assert.deepEqual(
    guideCellOneSquareDown({ x: 125, y: 123, width: 1, height: 1 }),
    { x: 125, y: 122, width: 1, height: 1 },
  )
})
