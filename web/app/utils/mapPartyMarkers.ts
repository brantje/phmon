import type { MapPartyMember } from '../../shared/types/live.ts'
import type { MapProfile } from '../../shared/types/map.ts'
import { worldPositionToRaster, type RasterPosition } from './mapCoordinates.ts'

export interface PartyMapMarker {
  id: string
  label: string
  kind: 'party'
  position: RasterPosition
  party: MapPartyMember
}

const normalizedName = (value: string | undefined) =>
  value?.trim().toLocaleLowerCase() || ''

export function partyMapMarkers(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  members: MapPartyMember[],
  managedCharacterNames: string[] = [],
): PartyMapMarker[] {
  const managed = new Set(
    managedCharacterNames.map(normalizedName).filter(Boolean),
  )
  const markers: PartyMapMarker[] = []
  for (const member of members) {
    const name = normalizedName(member.name)
    if (name && managed.has(name)) continue
    const position = worldPositionToRaster(
      profile,
      areaID,
      floorID,
      member.observer_region,
      member.x,
      member.y,
      member.observer_z,
    )
    if (!position) continue
    markers.push({
      id: member.id,
      label: member.name || `Party member ${member.player_id}`,
      kind: 'party',
      position,
      party: member,
    })
  }
  return markers
}
