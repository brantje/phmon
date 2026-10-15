import assert from 'node:assert/strict'
import test from 'node:test'
import {
  equipmentColumns,
  characterInfoRows,
  equipmentItemAt,
} from '../app/utils/equipmentLayout.ts'

test('equipment columns follow the phMonitor source-slot order', () => {
  assert.deepEqual(
    equipmentColumns.left.map((cell) => cell.sourceSlot),
    [6, 0, 1, 4, 9, 11, null],
  )
  assert.deepEqual(
    equipmentColumns.right.map((cell) => cell.sourceSlot),
    [7, 2, 3, 5, 10, 12, 8],
  )
  const placed = [...equipmentColumns.left, ...equipmentColumns.right]
    .map((cell) => cell.sourceSlot)
    .filter((slot) => slot != null)
    .sort((a, b) => a - b)
  assert.deepEqual(
    placed,
    Array.from({ length: 13 }, (_, index) => index),
  )
})

test('equipment items resolve by source slot', () => {
  const slots = Array.from({ length: 13 }, (_, index) =>
    index === 3 ? null : { item: { model: 100 + index } },
  )
  assert.deepEqual(equipmentItemAt(slots, 6), { model: 106 })
  assert.equal(equipmentItemAt(slots, 3), null)
  assert.equal(equipmentItemAt(slots, null), null)
  assert.deepEqual(
    equipmentItemAt([{ source_slot: 8, item: { model: 9 } }], 8),
    { model: 9 },
  )
})

test('character info rows follow the phMonitor info card', () => {
  const rows = characterInfoRows(
    { name: 'nuker1', guild: 'iBot' },
    {
      job_name: 'MrTrade',
      guild: { member_count: 10, online_count: 8, donated_gp: 486241 },
      union: { count: 0, names: [] },
    },
    'Oct 15, 2026, 10:21 PM',
  )
  assert.deepEqual(
    rows.map((row) => [row.label, row.value]),
    [
      ['Character', 'nuker1'],
      ['Job name', 'MrTrade'],
      ['Guild', 'iBot'],
      ['Guild members', '8 / 10'],
      ['Donated GP', '486,241'],
      ['Union guilds', '-'],
      ['Last seen', 'Oct 15, 2026, 10:21 PM'],
    ],
  )
  const unknown = characterInfoRows({ name: 'solo' }, {}, '')
  assert.deepEqual(
    unknown.slice(1).map((row) => row.value),
    ['-', '-', '-', '-', '-', '-'],
  )
  assert.equal(
    characterInfoRows(
      { name: 'x' },
      { union: { count: 2, names: ['A', 'B'] } },
      '',
    )[5]?.value,
    '2 - A, B',
  )
})
