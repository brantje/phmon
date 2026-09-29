import type { ActivityEvent } from '~~/shared/types/live'

export function eventLocationText(event: ActivityEvent): string {
  const zone = typeof event.zone === 'string' ? event.zone.trim() : ''
  const x = event.x
  const y = event.y

  if (x != null && y != null) {
    const label = zone || 'Unknown zone'
    return `${label} · ${x.toFixed(1)}, ${y.toFixed(1)}, ${event.z?.toFixed(1) ?? '—'}`
  }

  return zone || 'Location unknown'
}
