import type { ActivityEvent } from '~~/shared/types/live'

export function zoneNameText(zone?: string | null): string {
  return typeof zone === 'string' && zone.trim() ? zone.trim() : 'Unknown zone'
}

export function eventLocationText(event: ActivityEvent): string {
  const zone = zoneNameText(event.zone)
  const x = event.x
  const y = event.y

  if (x != null && y != null) {
    return (
      zone +
      ' · ' +
      x.toFixed(1) +
      ', ' +
      y.toFixed(1) +
      ', ' +
      (event.z?.toFixed(1) ?? '—')
    )
  }

  return event.zone?.trim() ? zone : 'Location unknown'
}
