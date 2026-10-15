import assert from 'node:assert/strict'
import test from 'node:test'
import { formatDropTimestamp } from '../app/utils/dropHistory.ts'

test('drop dates use the reference readable calendar and operator timezone', () => {
  assert.equal(
    formatDropTimestamp('2026-10-10T22:49:00Z', 'Europe/Amsterdam'),
    'Oct 11, 2026, 12:49 AM',
  )
  assert.equal(
    formatDropTimestamp('2026-10-15T16:13:48Z', 'Europe/Amsterdam'),
    'Oct 15, 2026, 6:13 PM',
  )
  assert.equal(formatDropTimestamp('invalid'), 'Unknown time')
})
