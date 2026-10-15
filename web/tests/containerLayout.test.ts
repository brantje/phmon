import assert from 'node:assert/strict'
import test from 'node:test'
import {
  clampContainerPage,
  containerPageCount,
  containerPageLayout,
  containerPageSlotNumbers,
  displayedContainerSlot,
} from '../app/utils/containerLayout.ts'
import { formatCompactObservedGold } from '../app/utils/fleetGold.ts'

test('inventory pages are 4 × 8 and storage pages are 6 × 5', () => {
  assert.deepEqual(containerPageLayout('inventory'), {
    columns: 4,
    rows: 8,
    pageSize: 32,
  })
  for (const container of ['storage', 'guild_storage'])
    assert.deepEqual(containerPageLayout(container), {
      columns: 6,
      rows: 5,
      pageSize: 30,
    })
})

test('page counts follow capacity boundaries without inventing pages', () => {
  assert.equal(containerPageCount(0, 30), 1)
  assert.equal(containerPageCount(30, 30), 1)
  assert.equal(containerPageCount(31, 30), 2)
  assert.equal(containerPageCount(60, 30), 2)
  assert.equal(containerPageCount(32, 32), 1)
  assert.equal(containerPageCount(33, 32), 2)
  assert.equal(containerPageCount(64, 32), 2)
})

test('partial final pages mark positions beyond capacity as absent', () => {
  const layout = containerPageLayout('storage')
  const last = containerPageSlotNumbers(1, 45, layout)
  assert.equal(last.length, 30)
  assert.equal(last[0], 30)
  assert.equal(last[14], 44)
  assert.equal(last[15], null)
  assert.equal(last.at(-1), null)
  const full = containerPageSlotNumbers(1, 64, containerPageLayout('inventory'))
  assert.equal(full.at(-1), 63)
})

test('capacity decreases clamp the selected page', () => {
  assert.equal(clampContainerPage(2, containerPageCount(64, 32)), 1)
  assert.equal(clampContainerPage(1, containerPageCount(30, 30)), 0)
  assert.equal(clampContainerPage(-1, 3), 0)
})

test('bag source slots skip equipment while storage slots are direct', () => {
  assert.equal(displayedContainerSlot('inventory', 13), 0)
  assert.equal(displayedContainerSlot('inventory', 45), 32)
  assert.equal(displayedContainerSlot('storage', 29), 29)
})

test('known zero gold shows 0 while unknown gold shows a dash', () => {
  assert.equal(formatCompactObservedGold(0), '0')
  assert.equal(formatCompactObservedGold(null), '—')
  assert.equal(formatCompactObservedGold(22_700_000_000), '22.7B')
})
