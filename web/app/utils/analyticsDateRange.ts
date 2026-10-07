const MAX_ANALYTICS_RANGE_MS = 366 * 24 * 60 * 60 * 1000

export type AnalyticsGroupBy =
  | 'character'
  | 'group'
  | 'location'
  | 'item'
  | 'type'
  | 'degree'
  | 'academy'

/** Keep a grouping the selected analytics view is allowed to query. */
export function analyticsGrouping(
  view: string,
  groupBy: string,
): AnalyticsGroupBy {
  const drops = view === 'rare_drops' || view === 'normal_drops'
  if (
    (groupBy === 'item' || groupBy === 'type' || groupBy === 'degree') &&
    drops
  ) {
    return groupBy
  }
  if (groupBy === 'group' && view === 'deaths') return 'group'
  if (groupBy === 'academy' && view === 'academy') return 'academy'
  if (groupBy === 'location' && view !== 'academy') return 'location'
  if (groupBy === 'character') return 'character'
  return 'character'
}

function validDate(value: string) {
  if (!/^\d{4}-\d{2}-\d{2}$/.test(value)) return false
  const parsed = new Date(`${value}T00:00:00Z`)
  return (
    Number.isFinite(parsed.getTime()) &&
    parsed.toISOString().slice(0, 10) === value
  )
}

/** Normalize inclusive local calendar dates to UTC [from, to) bounds. */
export function normalizeAnalyticsDateRange(from: string, to: string) {
  if (!from || !to) {
    return { from: '', to: '', error: 'Choose both a From and To date.' }
  }
  if (!validDate(from) || !validDate(to)) {
    return {
      from: '',
      to: '',
      error: 'Enter valid calendar dates for both bounds.',
    }
  }
  if (from > to) {
    return {
      from: '',
      to: '',
      error: 'Choose a From date on or before the To date.',
    }
  }

  const start = new Date(`${from}T00:00:00`)
  const end = new Date(`${to}T00:00:00`)
  end.setDate(end.getDate() + 1)
  const span = end.getTime() - start.getTime()
  if (!Number.isFinite(span) || span <= 0 || span > MAX_ANALYTICS_RANGE_MS) {
    return {
      from: '',
      to: '',
      error: 'Choose a date range no longer than 366 normalized days.',
    }
  }
  return { from: start.toISOString(), to: end.toISOString(), error: '' }
}
