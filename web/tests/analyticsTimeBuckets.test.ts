import assert from 'node:assert/strict'
import test from 'node:test'
import { buildAnalyticsTimeBuckets } from '../app/utils/analyticsTimeBuckets.ts'
import { normalizeAnalyticsDateRange } from '../app/utils/analyticsDateRange.ts'

test('hourly event buckets follow local DST transitions and match SQL wall keys', () => {
  const previousTimezone = process.env.TZ
  process.env.TZ = 'Europe/Amsterdam'
  try {
    const spring = normalizeAnalyticsDateRange('2026-03-29', '2026-03-29')
    const fall = normalizeAnalyticsDateRange('2026-10-25', '2026-10-25')
    const springBuckets = buildAnalyticsTimeBuckets(
      spring.from,
      spring.to,
      'hour',
      'Europe/Amsterdam',
    )
    const fallBuckets = buildAnalyticsTimeBuckets(
      fall.from,
      fall.to,
      'hour',
      'Europe/Amsterdam',
    )
    assert.equal(springBuckets.length, 23)
    assert.equal(springBuckets.includes('2026-03-29 02:00:00'), false)
    assert.equal(fallBuckets.length, 24)
    assert.equal(
      fallBuckets.filter((bucket) => bucket === '2026-10-25 02:00:00').length,
      1,
    )
  } finally {
    if (previousTimezone === undefined) delete process.env.TZ
    else process.env.TZ = previousTimezone
  }
})
