import assert from 'node:assert/strict'
import test from 'node:test'
import { formatDate, toIsoAttribute } from '../app/utils/dateFormat.ts'

const at = '2026-10-16T07:24:05Z'

test('each variant formats in a pinned locale and zone', () => {
  assert.equal(
    formatDate(at, 'datetime', '—', 'en-US', 'UTC'),
    'Oct 16, 2026, 07:24',
  )
  assert.equal(
    formatDate(at, 'datetime-seconds', '—', 'en-US', 'UTC'),
    'Oct 16, 2026, 07:24:05',
  )
  assert.equal(formatDate(at, 'date', '—', 'en-US', 'UTC'), 'Oct 16, 2026')
  assert.equal(formatDate(at, 'time', '—', 'en-US', 'UTC'), '07:24:05')
})

test('missing and invalid input use the fallback or Unknown', () => {
  assert.equal(formatDate(undefined), '—')
  assert.equal(formatDate('', 'datetime', 'Never'), 'Never')
  assert.equal(formatDate('nope'), 'Unknown')
  assert.equal(toIsoAttribute('nope'), undefined)
  assert.equal(toIsoAttribute(at), '2026-10-16T07:24:05.000Z')
})
