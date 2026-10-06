import type { MapProfile } from '../../shared/types/map.ts'
import {
  caveFloorForPosition,
  regionTileCenter,
  worldPositionToRaster,
  type RasterPosition,
} from './mapCoordinates.ts'

export function toggleMapFollow(currentID: string, characterID: string) {
  return currentID === characterID ? '' : characterID
}

export function mapFollowView(
  profile: MapProfile,
  character: {
    region?: number
    x?: number
    y?: number
    z?: number
  },
  currentAreaID: string,
  currentFloorID: string,
): {
  areaID: string
  floorID: string
  position: RasterPosition | null
} | null {
  if (character.region == null || !Number.isInteger(character.region))
    return null
  const cave = caveFloorForPosition(profile, character.region, character.z)
  const areaID =
    cave?.areaID || (character.region > 0 ? 'world' : currentAreaID)
  const floorID =
    cave?.floorID || (areaID === 'world' ? 'world' : currentFloorID)
  if (!areaID || !floorID) return null
  const position =
    worldPositionToRaster(
      profile,
      areaID,
      floorID,
      character.region,
      character.x,
      character.y,
      character.z,
    ) || regionTileCenter(profile, areaID, floorID, character.region)
  return { areaID, floorID, position }
}
