import assert from 'node:assert/strict'
import test from 'node:test'
import { normalizeAnalyticsDateRange } from '../app/utils/analyticsDateRange.ts'

test('date filters require two ordered valid dates within the analytics limit', () => {
  assert.match(normalizeAnalyticsDateRange('', '2026-10-07').error, /both/)
  assert.match(
    normalizeAnalyticsDateRange('2026-02-30', '2026-03-01').error,
    /valid calendar dates/,
  )
  assert.match(
    normalizeAnalyticsDateRange('2026-10-08', '2026-10-07').error,
    /on or before/,
  )
  assert.match(
    normalizeAnalyticsDateRange('2026-01-01', '2027-01-03').error,
    /366 normalized days/,
  )
})

test('inclusive local dates normalize DST days to their actual UTC duration', () => {
  const previousTimezone = process.env.TZ
  process.env.TZ = 'Europe/Amsterdam'
  try {
    const spring = normalizeAnalyticsDateRange('2026-03-29', '2026-03-29')
    const fall = normalizeAnalyticsDateRange('2026-10-25', '2026-10-25')
    assert.equal(spring.error, '')
    assert.equal(fall.error, '')
    assert.equal(
      Date.parse(spring.to) - Date.parse(spring.from),
      23 * 60 * 60 * 1000,
    )
    assert.equal(
      Date.parse(fall.to) - Date.parse(fall.from),
      25 * 60 * 60 * 1000,
    )
  } finally {
    if (previousTimezone === undefined) delete process.env.TZ
    else process.env.TZ = previousTimezone
  }
})
