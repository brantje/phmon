import type { MapThief } from '../../shared/types/live.ts'
import type { MapProfile } from '../../shared/types/map.ts'
import { worldPositionToRaster, type RasterPosition } from './mapCoordinates.ts'

export const THIEF_ICON = '/game-assets/monsters/thief_01.png'

export interface ThiefMapMarker {
  id: string
  label: string
  kind: 'thief'
  position: RasterPosition
  thief: MapThief
}

export function thiefMapMarkers(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  thieves: MapThief[],
): ThiefMapMarker[] {
  const markers: ThiefMapMarker[] = []
  for (const thief of thieves) {
    const position = worldPositionToRaster(
      profile,
      areaID,
      floorID,
      thief.region,
      thief.x,
      thief.y,
      thief.z,
    )
    if (!position) continue
    markers.push({
      id: thief.id,
      label: thief.name,
      kind: 'thief',
      position,
      thief,
    })
  }
  return markers
}

export function thiefOriginLabel(
  thief: Pick<MapThief, 'origin' | 'reporter_app'>,
) {
  if (thief.origin === 'phmon') return 'PhMon'
  return thief.reporter_app?.trim() || 'AdvancedAutoTrade'
}
