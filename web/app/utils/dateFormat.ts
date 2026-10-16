export type DateFormatVariant =
  'datetime' | 'datetime-seconds' | 'date' | 'time'

type DateInput = string | number | Date | null | undefined

function parseDate(value: DateInput): Date | null {
  if (value == null || value === '') return null
  const date = value instanceof Date ? value : new Date(value)
  return Number.isNaN(date.getTime()) ? new Date(Number.NaN) : date
}

export function toIsoAttribute(value: DateInput): string | undefined {
  const date = parseDate(value)
  return date && !Number.isNaN(date.getTime()) ? date.toISOString() : undefined
}

// Formats in the viewer's locale and time zone. `locale`/`timeZone` exist so
// tests can pin the output.
export function formatDate(
  value: DateInput,
  variant: DateFormatVariant = 'datetime',
  fallback = '—',
  locale?: string,
  timeZone?: string,
): string {
  const date = parseDate(value)
  if (!date) return fallback
  if (Number.isNaN(date.getTime())) return 'Unknown'
  const day = { year: 'numeric', month: 'short', day: 'numeric' } as const
  switch (variant) {
    case 'date':
      return date.toLocaleDateString(locale, { ...day, timeZone })
    case 'time':
      return date.toLocaleTimeString(locale, {
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hourCycle: 'h23',
        timeZone,
      })
    case 'datetime-seconds':
      return date.toLocaleDateString(locale, {
        ...day,
        hour: '2-digit',
        minute: '2-digit',
        second: '2-digit',
        hourCycle: 'h23',
        timeZone,
      })
    default:
      return date.toLocaleDateString(locale, {
        ...day,
        hour: '2-digit',
        minute: '2-digit',
        hourCycle: 'h23',
        timeZone,
      })
  }
}
