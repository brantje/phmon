import assert from 'node:assert/strict'
import test from 'node:test'
import { relativeMapEventWindow } from '../app/utils/mapTimeRange.ts'

test('relative map event windows advance both ends while preserving their duration', () => {
  const first = relativeMapEventWindow(
    '1h',
    Date.parse('2026-09-29T10:00:00.000Z'),
  )
  const later = relativeMapEventWindow(
    '1h',
    Date.parse('2026-09-29T10:05:00.000Z'),
  )
  assert.equal(first.from, '2026-09-29T09:00:00.000Z')
  assert.equal(first.to, '2026-09-29T10:00:00.000Z')
  assert.equal(later.from, '2026-09-29T09:05:00.000Z')
  assert.equal(later.to, '2026-09-29T10:05:00.000Z')
  assert.equal(Date.parse(later.to) - Date.parse(later.from), 60 * 60_000)
})

test('relative map windows support the displayed day and week ranges', () => {
  const now = Date.parse('2026-09-29T10:00:00.000Z')
  assert.equal(
    Date.parse(relativeMapEventWindow('24h', now).from),
    now - 24 * 60 * 60_000,
  )
  assert.equal(
    Date.parse(relativeMapEventWindow('7d', now).from),
    now - 7 * 24 * 60 * 60_000,
  )
})
