import type { MapProfile } from '../../shared/types/map.ts'
import {
  regionTileCenter,
  worldPositionToRaster,
  type RasterPosition,
} from './mapCoordinates.ts'

export interface CharacterMarkerInput {
  character_id: string
  name: string
  region?: number
  x?: number
  y?: number
  z?: number
  level?: number
  zone?: string
  group_name?: string
  portrait_url?: string
  hp?: number
  hp_max?: number
  mp?: number
  mp_max?: number
  gold?: number
  sp?: number
  online?: boolean
  position_stale?: boolean
  dead?: boolean | null
  state_updated_at?: string
}

export interface CharacterMapMarker {
  id: string
  label: string
  kind: 'character'
  placement: 'exact' | 'region-tile'
  position: RasterPosition
  character: CharacterMarkerInput
}

export function characterHasDisplayableMapPosition(
  character: CharacterMarkerInput,
  now = Date.now(),
) {
  const updatedAt = character.state_updated_at
    ? Date.parse(character.state_updated_at)
    : Number.NaN
  const hasPosition =
    character.region != null &&
    Number.isInteger(character.region) &&
    Number.isFinite(updatedAt)
  if (!hasPosition) return false
  return updatedAt <= now + 5_000
}

export function displayableMapCharacters<T extends CharacterMarkerInput>(
  characters: T[],
  now = Date.now(),
): T[] {
  return characters.filter((character) =>
    characterHasDisplayableMapPosition(character, now),
  )
}

export function characterMapMarkers(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  characters: CharacterMarkerInput[],
): CharacterMapMarker[] {
  const markers: CharacterMapMarker[] = []
  const tileGroups = new Map<
    string,
    { center: RasterPosition; characters: CharacterMarkerInput[] }
  >()
  for (const character of characters) {
    const exact = worldPositionToRaster(
      profile,
      areaID,
      floorID,
      character.region,
      character.x,
      character.y,
    )
    if (exact) {
      markers.push({
        id: character.character_id,
        label: character.name,
        kind: 'character',
        placement: 'exact',
        position: exact,
        character,
      })
      continue
    }
    const center = regionTileCenter(profile, areaID, floorID, character.region)
    if (!center) continue
    const key = `${center.tileX}:${center.tileY}`
    const group = tileGroups.get(key) || { center, characters: [] }
    group.characters.push(character)
    tileGroups.set(key, group)
  }

  for (const group of tileGroups.values()) {
    const sorted = group.characters.sort((left, right) =>
      left.character_id.localeCompare(right.character_id),
    )
    const radius = Math.min(56, 18 + sorted.length * 3)
    sorted.forEach((character, index) => {
      const angle = -Math.PI / 2 + (2 * Math.PI * index) / sorted.length
      markers.push({
        id: character.character_id,
        label: `${character.name} · Region ${character.region} · tile ${group.center.tileX}×${group.center.tileY}. Exact pixel unverified; markers are spread for visibility.`,
        kind: 'character',
        placement: 'region-tile',
        position: {
          ...group.center,
          pixelX:
            group.center.pixelX +
            (sorted.length === 1 ? 0 : radius * Math.cos(angle)),
          pixelY:
            group.center.pixelY +
            (sorted.length === 1 ? 0 : radius * Math.sin(angle)),
        },
        character,
      })
    })
  }
  return markers
}
