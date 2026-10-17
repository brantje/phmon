import type { ActivityEvent, ThiefSighting } from '~~/shared/types/live'

export function zoneNameText(zone?: string | null): string {
  return typeof zone === 'string' && zone.trim() ? zone.trim() : 'Unknown zone'
}

export function eventLocationText(
  event: ActivityEvent,
  coordinateSeparator = '·',
): string {
  const zone = zoneNameText(event.zone)
  const x = event.x
  const y = event.y

  if (x != null && y != null) {
    return (
      zone +
      ` ${coordinateSeparator} ` +
      x.toFixed(1) +
      ', ' +
      y.toFixed(1) +
      ', ' +
      (event.z?.toFixed(1) ?? '—')
    )
  }

  return event.zone?.trim() ? zone : 'Location unknown'
}

/** Zone name with whole-map coordinates, as shown for thief sightings. */
export function sightingLocationText(
  sighting: Pick<ThiefSighting, 'position' | 'zone'>,
): string {
  const { position, zone } = sighting
  if (!position) return 'Location unknown'
  return `${zoneNameText(zone)} · ${position.x.toFixed(1)}, ${position.y.toFixed(1)}`
}

export function eventRowLocationText(event: ActivityEvent): string {
  const isDrop = event.kind === 'drop.item' || event.kind === 'drop.rare'
  if (isDrop && event.zone?.trim()) return event.zone.trim()
  return eventLocationText(event, '|')
}
