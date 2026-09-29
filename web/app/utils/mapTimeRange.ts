export function relativeMapEventWindow(
  range: string,
  now: number | Date = Date.now(),
) {
  const durations: Record<string, number> = {
    '1h': 60 * 60_000,
    '24h': 24 * 60 * 60_000,
    '7d': 7 * 24 * 60 * 60_000,
  }
  const end = now instanceof Date ? now.getTime() : now
  const duration = durations[range] ?? durations['24h']!
  return {
    from: new Date(end - duration).toISOString(),
    to: new Date(end).toISOString(),
  }
}
