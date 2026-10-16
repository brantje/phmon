import assert from 'node:assert/strict'
import test from 'node:test'
import { masterySummary } from '../app/utils/masterySummary.ts'

test('shows up to two learned masteries ordered by level and stable ID', () => {
  assert.equal(
    masterySummary([
      { id: 275, name: 'Fire', level: 100 },
      { id: 257, name: 'Bicheon', level: 0 },
      { id: 259, name: 'Pacheon', level: 100 },
      { id: 274, name: 'Lightning', level: 91 },
    ]),
    'Pacheon · Fire',
  )
})

test('returns an empty summary for missing, empty, or unusable mastery data', () => {
  assert.equal(masterySummary(undefined), '')
  assert.equal(masterySummary([]), '')
  assert.equal(
    masterySummary([
      { id: 257, name: 'Bicheon', level: 0 },
      { id: 258, name: '', level: 100 },
      { id: true, name: 'Invalid', level: 100 },
      { id: 259, name: 'Pacheon', level: 256 },
    ]),
    '',
  )
})

test('ignores malformed entries and bounds displayed mastery names', () => {
  assert.equal(
    masterySummary([
      null,
      { id: 259, name: ` ${'P'.repeat(70)} `, level: 100 },
      { id: 275, name: 'Fire', level: 100 },
    ]),
    `${'P'.repeat(64)} · Fire`,
  )
})
