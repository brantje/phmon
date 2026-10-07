export type AnalyticsBucketUnit = 'hour' | 'day' | 'week'

function localBucketKey(
  timestamp: number,
  unit: AnalyticsBucketUnit,
  timezone: string,
) {
  const parts = new Intl.DateTimeFormat('en-CA', {
    timeZone: timezone,
    year: 'numeric',
    month: '2-digit',
    day: '2-digit',
    hour: '2-digit',
    hourCycle: 'h23',
  }).formatToParts(timestamp)
  const values = Object.fromEntries(
    parts.map((part) => [part.type, part.value]),
  )
  const date = `${values.year}-${values.month}-${values.day}`
  if (unit === 'hour') return `${date} ${values.hour}:00:00`
  if (unit === 'day') return `${date} 00:00:00`

  const day = new Date(`${date}T00:00:00Z`)
  const mondayOffset = (day.getUTCDay() + 6) % 7
  day.setUTCDate(day.getUTCDate() - mondayOffset)
  return `${day.toISOString().slice(0, 10)} 00:00:00`
}

/**
 * Build the local-calendar buckets in a UTC range. Repeated fall-back wall
 * hours merge because PostgreSQL date_trunc uses the same local bucket key;
 * skipped spring-forward hours are absent. Each generated key is compatible
 * with the analytics query's `YYYY-MM-DD HH24:MI:SS` output.
 */
export function buildAnalyticsTimeBuckets(
  from: string,
  to: string,
  unit: AnalyticsBucketUnit,
  timezone: string,
) {
  const start = Date.parse(from)
  const end = Date.parse(to)
  if (!Number.isFinite(start) || !Number.isFinite(end) || end <= start)
    return []

  const buckets = new Set<string>()
  for (let at = start; at < end; at += 60 * 60 * 1000)
    buckets.add(localBucketKey(at, unit, timezone))
  return [...buckets].sort()
}
