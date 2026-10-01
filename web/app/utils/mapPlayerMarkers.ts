import type { MapOtherPlayer, MapPartyMember } from '../../shared/types/live.ts'
import type { MapProfile } from '../../shared/types/map.ts'
import { worldPositionToRaster, type RasterPosition } from './mapCoordinates.ts'

export interface PlayerMapMarker {
  id: string
  label: string
  kind: 'player'
  position: RasterPosition
  player: MapOtherPlayer
}

const normalizedName = (value: string | undefined) =>
  value?.trim().toLocaleLowerCase() || ''

export function playerMapMarkers(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  players: MapOtherPlayer[],
  managedCharacterNames: string[] = [],
  partyMembers: MapPartyMember[] = [],
): PlayerMapMarker[] {
  const managed = new Set(
    managedCharacterNames.map(normalizedName).filter(Boolean),
  )
  const partyIDs = new Set(
    partyMembers
      .filter((member) => member.player_id > 0)
      .map((member) => String(member.player_id)),
  )
  const partyNames = new Set(
    partyMembers.map((member) => normalizedName(member.name)).filter(Boolean),
  )
  const markers: PlayerMapMarker[] = []
  for (const player of players) {
    const name = normalizedName(player.name)
    if (name && managed.has(name)) continue
    if (partyIDs.has(player.player_id)) continue
    if (name && partyNames.has(name)) continue
    const position = worldPositionToRaster(
      profile,
      areaID,
      floorID,
      player.region,
      player.x,
      player.y,
      player.observer_z,
    )
    if (!position) continue
    markers.push({
      id: player.id,
      label: player.name || `Player ${player.player_id}`,
      kind: 'player',
      position,
      player,
    })
  }
  return markers
}
