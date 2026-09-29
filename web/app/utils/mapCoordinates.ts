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

export interface MapTileGrid {
  min_x: number
  max_x: number
  min_y: number
  max_y: number
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

function usableTransform(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number,
): MapCoordinateTransform | undefined {
  const partialOutdoor =
    areaID === 'world' &&
    floorID === 'world' &&
    profile.coordinate_transform_status === 'partial-validated-outdoor' &&
    profile.region_mappings_status === 'partial-validated'
  if (
    !partialOutdoor &&
    (profile.coordinate_transform_status !== 'validated' ||
      profile.region_mappings_status !== 'validated')
  )
    return undefined
  const area = profile.areas.find((item) => item.id === areaID)
  const floor = area?.floors.find((item) => item.id === floorID)
  if (
    (area?.region_mapping_status !== 'validated' &&
      !(
        partialOutdoor && area?.region_mapping_status === 'partial-validated'
      )) ||
    (floor?.transform_status !== 'validated' &&
      !(partialOutdoor && floor?.transform_status === 'partial-validated'))
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

function inCatalog(profile: MapProfile, tileX: number, tileY: number) {
  return (
    tileX >= profile.tiles.min_x &&
    tileX <= profile.tiles.max_x &&
    tileY >= profile.tiles.min_y &&
    tileY <= profile.tiles.max_y
  )
}

// A documented refregion-to-root-tile join can locate an outdoor region on
// the raster without claiming a character's pixel inside that tile.
export function regionTileCenter(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
): RasterPosition | null {
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
): RasterPosition | null {
  if (
    region == null ||
    x == null ||
    y == null ||
    !Number.isFinite(x) ||
    !Number.isFinite(y)
  )
    return null
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
  if (!inCatalog(profile, tileX, tileY)) return null
  if (profile.coordinate_transform_status === 'partial-validated-outdoor') {
    const mapping = profile.region_mappings.find(
      (item) =>
        item.region === region &&
        item.area_id === areaID &&
        item.floor_id === floorID &&
        item.status === 'validated',
    )
    // A phBot position that crosses a region seam before its region ID changes
    // must wait for a consistent snapshot instead of appearing in another tile.
    if (!mapping || mapping.tile_x !== tileX || mapping.tile_y !== tileY)
      return null
  }
  return {
    tileX,
    tileY,
    pixelX: (rasterX - tileX) * TILE_SIZE,
    pixelY: (1 - fractionY) * TILE_SIZE,
  }
}

export function rasterPositionToGame(
  profile: MapProfile,
  areaID: string,
  floorID: string,
  region: number | undefined,
  position: RasterPosition,
): GamePosition | null {
  if (
    region == null ||
    !inCatalog(profile, position.tileX, position.tileY) ||
    !Number.isFinite(position.pixelX) ||
    !Number.isFinite(position.pixelY) ||
    position.pixelX < 0 ||
    position.pixelX >= TILE_SIZE ||
    position.pixelY < 0 ||
    position.pixelY >= TILE_SIZE
  )
    return null
  const transform = usableTransform(profile, areaID, floorID, region)
  if (
    !transform ||
    profile.command_z_evidence_status !== 'verified' ||
    transform.command_z == null ||
    !Number.isFinite(transform.command_z)
  )
    return null
  const rasterX = position.tileX + position.pixelX / TILE_SIZE
  const rasterY = position.tileY + 1 - position.pixelY / TILE_SIZE
  return {
    region,
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
    z: transform.command_z,
  }
}
