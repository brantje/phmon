import {
  outdoorRegionTile,
  worldPositionToRaster,
  type RasterPosition,
} from './mapCoordinates.ts'
import type { MapProfile } from '../../shared/types/map.ts'

export interface MapPreviewCharacterScope {
  character_id?: string
  region?: number
}

export interface MapEventCoordinates {
  region?: number
  x?: number
  y?: number
}

export interface MappedEventLocation {
  status: 'mapped' | 'region-unmapped' | 'coordinates-unmappable'
  areaID: string
  floorID: string
  position: RasterPosition | null
}

export function mapEventRoute(
  server: string,
  eventID: string,
  characterID: string,
  region: number | undefined,
  location: MappedEventLocation | undefined,
) {
  return {
    path: '/map',
    query: {
      server,
      area: location?.areaID || 'world',
      floor: location?.floorID || 'world',
      ...(region != null && (location?.areaID || 'world') === 'world'
        ? { region: String(region) }
        : {}),
      character_id: characterID,
      event_id: eventID,
    },
  }
}

export function mapEventLocation(
  profile: MapProfile,
  event: MapEventCoordinates,
): MappedEventLocation {
  if (outdoorRegionTile(profile, 'world', 'world', event.region)) {
    const position = worldPositionToRaster(
      profile,
      'world',
      'world',
      event.region,
      event.x,
      event.y,
    )
    return {
      status: position ? 'mapped' : 'coordinates-unmappable',
      areaID: 'world',
      floorID: 'world',
      position,
    }
  }
  const mapping =
    event.region == null
      ? undefined
      : profile.region_mappings.find((item) => item.region === event.region)
  if (!mapping)
    return {
      status: 'region-unmapped',
      areaID: 'world',
      floorID: 'world',
      position: null,
    }
  if (mapping.status !== 'validated')
    return {
      status: 'coordinates-unmappable',
      areaID: mapping.area_id,
      floorID: mapping.floor_id,
      position: null,
    }
  const position = worldPositionToRaster(
    profile,
    mapping.area_id,
    mapping.floor_id,
    event.region,
    event.x,
    event.y,
  )
  return {
    status: position ? 'mapped' : 'coordinates-unmappable',
    areaID: mapping.area_id,
    floorID: mapping.floor_id,
    position,
  }
}

export function mapProfileRequestIsCurrent(
  requestID: number,
  activeRequestID: number,
  requestedServer: string,
  selectedServer: string,
) {
  return requestID === activeRequestID && requestedServer === selectedServer
}

export function mapFeedRegion(region: number, selectedCharacterID: string) {
  return selectedCharacterID ? undefined : region || undefined
}

export function mapPreviewLocation(
  server: string,
  character?: MapPreviewCharacterScope,
) {
  return {
    path: '/map',
    query: {
      server,
      area: 'world',
      floor: 'world',
      ...(character?.region ? { region: String(character.region) } : {}),
      ...(character?.character_id
        ? { character_id: character.character_id }
        : {}),
    },
  }
}
