import type { MapCoordinateTransform, MapProfile } from '~~/shared/types/map'

export interface RasterPosition {
  tileX: number
  tileY: number
  pixelX: number
  pixelY: number
}

export interface GamePosition {
  region: number
  x: number
  y: number
  z: number
}

const TILE_SIZE = 256
const OUTDOOR_UNITS_PER_TILE = 192
const OUTDOOR_ORIGIN_TILE_X = 135
const OUTDOOR_ORIGIN_TILE_Y = 92

export interface MapTileGrid {
  min_x: number
  max_x: number
  min_y: number
  max_y: number
}

export function tileCatalogForFloor(
  profile: MapProfile,
  areaID: string,
  floorID: string,
) {
  return (
    profile.areas
      .find((area) => area.id === areaID)
      ?.floors.find((floor) => floor.id === floorID)?.tiles || profile.tiles
  )
}

export function caveFloorForPosition(
  profile: MapProfile,
  region: number | undefined,
  z: number | undefined,
) {
  if (region == null) return null
  for (const area of profile.areas) {
    if (area.kind !== 'cave') continue
    for (const floor of area.floors) {
      if (!floor.auto_detect || !floor.region_ids?.includes(region)) continue
      if (floor.min_z != null && (z == null || z < floor.min_z)) continue
      if (floor.max_z != null && (z == null || z > floor.max_z)) continue
      return { areaID: area.id, floorID: floor.id }
    }
  }
  return null
}

export interface LeafletPoint {
  lat: number
  lng: number
}

export function rasterTileCenterToLeaflet(
  grid: MapTileGrid,
  tileX: number,
  tileY: number,
): LeafletPoint | null {
  if (
    tileX < grid.min_x ||
    tileX > grid.max_x ||
    tileY < grid.min_y ||
    tileY > grid.max_y
  )
    return null
  const column = tileX - grid.min_x
  const row = grid.max_y - tileY
  return {
    lat: -(row * TILE_SIZE + TILE_SIZE / 2),
    lng: column * TILE_SIZE + TILE_SIZE / 2,
  }
}

export function leafletToRasterPosition(
  grid: MapTileGrid,
  latitude: number,
  longitude: number,
): RasterPosition {
  const rasterX = longitude / TILE_SIZE
  const rasterY = -latitude / TILE_SIZE
  const tileColumn = Math.floor(rasterX)
  const tileRow = Math.floor(rasterY)
  return {
    tileX: grid.min_x + tileColumn,
    tileY: grid.max_y - tileRow,
    pixelX: (rasterX - tileColumn) * TILE_SIZE,
    pixelY: (rasterY - tileRow) * TILE_SIZE,
  }
}

function outdoorGridEnabled(
  profile: MapProfile,
  areaID: string,
  floorID: string,
) {
  return (
    areaID === 'world' &&
    floorID === 'world' &&
    profile.tiles.status === 'available-for-inspection' &&
    profile.coordinate_transform_status === 'outdoor-region-grid'
  )
}

export function outdoorRegionTile(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
) {
  if (
    !outdoorGridEnabled(profile, areaID, floorID) ||
    region == null ||
    !Number.isInteger(region) ||
    region < 1 ||
    region > 65535
  )
    return null
  const tileX = region % 256
  const tileY = Math.floor(region / 256)
  return inCatalog(profile, tileX, tileY) ? { tileX, tileY } : null
}

function usableTransform(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number,
): MapCoordinateTransform | undefined {
  const outdoor = outdoorRegionTile(profile, areaID, floorID, region)
  if (outdoor) {
    const observed = profile.coordinate_transforms.find(
      (transform) =>
        transform.area_id === areaID &&
        transform.floor_id === floorID &&
        transform.region === region,
    )
    return {
      area_id: areaID,
      floor_id: floorID,
      region,
      status: 'outdoor-region-grid',
      world_origin_x:
        (outdoor.tileX - OUTDOOR_ORIGIN_TILE_X) * OUTDOOR_UNITS_PER_TILE,
      world_origin_y:
        (outdoor.tileY - OUTDOOR_ORIGIN_TILE_Y) * OUTDOOR_UNITS_PER_TILE,
      tile_origin_x: outdoor.tileX,
      tile_origin_y: outdoor.tileY,
      units_per_tile_x: OUTDOOR_UNITS_PER_TILE,
      units_per_tile_y: OUTDOOR_UNITS_PER_TILE,
      axis_x: 1,
      axis_y: 1,
      command_z: observed?.command_z,
    }
  }
  const area = profile.areas.find((item) => item.id === areaID)
  const floor = area?.floors.find((item) => item.id === floorID)
  const profiledCave =
    area?.kind === 'cave' && Boolean(floor?.region_ids?.length)
  if (
    !profiledCave &&
    (profile.coordinate_transform_status !== 'validated' ||
      profile.region_mappings_status !== 'validated')
  )
    return undefined
  if (
    !(
      profiledCave ? ['validated', 'reference-observed'] : ['validated']
    ).includes(area?.region_mapping_status || '') ||
    !(
      profiledCave ? ['validated', 'reference-observed'] : ['validated']
    ).includes(floor?.transform_status || '')
  )
    return undefined
  return profile.coordinate_transforms.find(
    (transform) =>
      transform.area_id === areaID &&
      transform.floor_id === floorID &&
      transform.region === region &&
      transform.status === 'validated' &&
      Number.isFinite(transform.world_origin_x) &&
      Number.isFinite(transform.world_origin_y) &&
      Number.isFinite(transform.tile_origin_x) &&
      Number.isFinite(transform.tile_origin_y) &&
      Number.isFinite(transform.units_per_tile_x) &&
      transform.units_per_tile_x > 0 &&
      Number.isFinite(transform.units_per_tile_y) &&
      transform.units_per_tile_y > 0 &&
      (transform.axis_x === 1 || transform.axis_x === -1) &&
      (transform.axis_y === 1 || transform.axis_y === -1),
  )
}

function inCatalog(
  profile: MapProfile,
  tileX: number,
  tileY: number,
  areaID = 'world',
  floorID = 'world',
) {
  const grid = tileCatalogForFloor(profile, areaID, floorID)
  if (!grid) return false
  return (
    tileX >= grid.min_x &&
    tileX <= grid.max_x &&
    tileY >= grid.min_y &&
    tileY <= grid.max_y
  )
}

// The encoded outdoor region tile can locate a character even when phBot has
// not yet supplied a usable X/Y pair during a teleport transition.
export function regionTileCenter(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
): RasterPosition | null {
  const outdoor = outdoorRegionTile(profile, areaID, floorID, region)
  if (outdoor)
    return { ...outdoor, pixelX: TILE_SIZE / 2, pixelY: TILE_SIZE / 2 }
  if (
    region == null ||
    !Number.isInteger(region) ||
    profile.tiles.status !== 'available-for-inspection' ||
    !['partial-tile-only', 'partial-validated', 'validated'].includes(
      profile.region_mappings_status,
    ) ||
    areaID !== 'world' ||
    floorID !== 'world'
  )
    return null
  const mapping = profile.region_mappings.find(
    (item) =>
      item.region === region &&
      item.area_id === areaID &&
      item.floor_id === floorID &&
      (item.status === 'exact-grid-match' || item.status === 'validated'),
  )
  if (!mapping || !inCatalog(profile, mapping.tile_x, mapping.tile_y))
    return null
  return {
    tileX: mapping.tile_x,
    tileY: mapping.tile_y,
    pixelX: TILE_SIZE / 2,
    pixelY: TILE_SIZE / 2,
  }
}

export function worldPositionToRaster(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
  x: number | undefined,
  y: number | undefined,
  z?: number,
): RasterPosition | null {
  if (
    region == null ||
    x == null ||
    y == null ||
    !Number.isFinite(x) ||
    !Number.isFinite(y)
  )
    return null
  const area = profile.areas.find((item) => item.id === areaID)
  if (
    area?.kind === 'cave' &&
    area.floors.some((floor) => floor.region_ids?.length)
  ) {
    const classified = caveFloorForPosition(profile, region, z)
    if (
      !classified ||
      classified.areaID !== areaID ||
      classified.floorID !== floorID
    )
      return null
  }
  const transform = usableTransform(profile, areaID, floorID, region)
  if (!transform) return null
  const rasterX =
    transform.tile_origin_x +
    ((x - transform.world_origin_x) / transform.units_per_tile_x) *
      transform.axis_x
  const rasterY =
    transform.tile_origin_y +
    ((y - transform.world_origin_y) / transform.units_per_tile_y) *
      transform.axis_y
  if (!Number.isFinite(rasterX) || !Number.isFinite(rasterY)) return null
  const tileX = Math.floor(rasterX)
  // The transform's Y coordinate increases with tile Y (north/up), while
  // Leaflet's pixel Y increases from the tile's top edge. Store the tile-local
  // pixel using screen coordinates after resolving exact seam positions.
  let tileY = Math.floor(rasterY)
  let fractionY = rasterY - tileY
  if (fractionY === 0) {
    tileY -= 1
    fractionY = 1
  }
  if (!inCatalog(profile, tileX, tileY, areaID, floorID)) return null
  if (outdoorGridEnabled(profile, areaID, floorID)) {
    const encoded = outdoorRegionTile(profile, areaID, floorID, region)
    // A region update and X/Y may arrive in separate phBot samples. Wait for a
    // consistent pair instead of moving the marker into the wrong tile.
    if (!encoded || encoded.tileX !== tileX || encoded.tileY !== tileY)
      return null
  }
  return {
    tileX,
    tileY,
    pixelX: (rasterX - tileX) * TILE_SIZE,
    pixelY: (1 - fractionY) * TILE_SIZE,
  }
}

function unitsPerTile(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
) {
  if (region == null || !Number.isInteger(region)) return null
  const transform = usableTransform(profile, areaID, floorID, region)
  return transform
    ? (transform.units_per_tile_x + transform.units_per_tile_y) / 2
    : null
}

/** Converts a game-unit distance to raster pixels, which equal Leaflet units. */
export function worldRadiusToRasterPixels(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
  radius: number,
) {
  const units = unitsPerTile(profile, areaID, floorID, region)
  if (!units || !Number.isFinite(radius) || radius <= 0) return null
  return (radius / units) * TILE_SIZE
}

export function rasterPixelsToWorldRadius(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
  pixels: number,
) {
  const units = unitsPerTile(profile, areaID, floorID, region)
  if (!units || !Number.isFinite(pixels) || pixels <= 0) return null
  return (pixels / TILE_SIZE) * units
}

export function rasterPositionToGame(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
  position: RasterPosition,
  currentZ?: number,
): GamePosition | null {
  const floor = profile.areas
    .find((area) => area.id === areaID)
    ?.floors.find((item) => item.id === floorID)
  const regionIDs = floor?.region_ids || []
  const resolvedRegion = outdoorGridEnabled(profile, areaID, floorID)
    ? position.tileY * 256 + position.tileX
    : areaID === 'world'
      ? region
      : regionIDs.length === 1
        ? regionIDs[0]
        : regionIDs.includes(region ?? 0)
          ? region
          : undefined
  if (
    resolvedRegion == null ||
    !inCatalog(profile, position.tileX, position.tileY, areaID, floorID) ||
    !Number.isFinite(position.pixelX) ||
    !Number.isFinite(position.pixelY) ||
    position.pixelX < 0 ||
    position.pixelX >= TILE_SIZE ||
    position.pixelY < 0 ||
    position.pixelY >= TILE_SIZE
  )
    return null
  const transform = usableTransform(profile, areaID, floorID, resolvedRegion)
  const cave = profile.areas.find((area) => area.id === areaID)?.kind === 'cave'
  if (
    !transform ||
    (currentZ == null &&
      !cave &&
      (profile.command_z_evidence_status !== 'verified' ||
        transform.command_z == null))
  )
    return null
  const rasterX = position.tileX + position.pixelX / TILE_SIZE
  const rasterY = position.tileY + 1 - position.pixelY / TILE_SIZE
  return {
    region: resolvedRegion,
    x:
      transform.world_origin_x +
      (rasterX - transform.tile_origin_x) *
        transform.units_per_tile_x *
        transform.axis_x,
    y:
      transform.world_origin_y +
      (rasterY - transform.tile_origin_y) *
        transform.units_per_tile_y *
        transform.axis_y,
    z:
      currentZ != null && Number.isFinite(currentZ)
        ? currentZ
        : cave
          ? 0
          : (transform.command_z ?? 0),
  }
}
