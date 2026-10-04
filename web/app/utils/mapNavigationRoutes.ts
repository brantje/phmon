import type { CharacterView, NavigationRoute } from '../../shared/types/live'
import type { MapProfile } from '../../shared/types/map'
import { worldPositionToRaster, type RasterPosition } from './mapCoordinates.ts'

export interface MapRouteOverlay {
  id: string
  characterID: string
  characterName: string
  sessionID: string
  commandID: string
  status: string
  reason?: string
  stale: boolean
  selected: boolean
  blocks: RasterPosition[][]
  currentAnchor?: RasterPosition
}

function segmentsForBlock(
  route: NavigationRoute,
  profile: MapProfile,
  areaID: string,
  floorID: string,
  selectedRegion: number,
) {
  const segments: RasterPosition[][] = []
  for (const block of route.blocks) {
    if (block.area_id !== areaID || block.floor_id !== floorID) continue
    let current: RasterPosition[] = []
    let previousRegion = 0
    for (const point of block.points) {
      const position =
        selectedRegion !== 0 && point.region !== selectedRegion
          ? null
          : worldPositionToRaster(
              profile,
              areaID,
              floorID,
              point.region,
              point.x,
              point.y,
              point.z,
            )
      if (
        !position ||
        (areaID !== 'world' &&
          previousRegion !== 0 &&
          point.region !== previousRegion)
      ) {
        if (current.length) segments.push(current)
        current = []
      }
      if (position) current.push(position)
      previousRegion = point.region
    }
    if (current.length) segments.push(current)
  }
  return segments
}

export function mapNavigationRouteOverlays(input: {
  routes: NavigationRoute[] | undefined
  characters: CharacterView[]
  profile: MapProfile
  server: string
  areaID: string
  floorID: string
  region: number
  streamCurrent: boolean
  liveStale: boolean
  freshnessNow: number
  selectedRouteID: string
}): MapRouteOverlay[] {
  const overlays: MapRouteOverlay[] = []
  for (const route of input.routes || []) {
    if (
      route.server.toLowerCase() !== input.server.toLowerCase() ||
      route.dataset_id !== input.profile.dataset_id
    )
      continue
    const character = input.characters.find(
      (item) =>
        item.character_id === route.character_id &&
        item.server.toLowerCase() === input.server.toLowerCase(),
    )
    if (!character || character.session_id !== route.session_id) continue
    const stateAt = character.state_updated_at
      ? Date.parse(character.state_updated_at)
      : Number.NaN
    const positionFresh =
      character.online &&
      Number.isFinite(stateAt) &&
      input.freshnessNow - stateAt >= -5_000 &&
      input.freshnessNow - stateAt <= 35_000
    const stale = input.liveStale || !input.streamCurrent || !positionFresh
    const blocks =
      route.dataset_version === input.profile.dataset_version &&
      !route.geometry_omitted
        ? segmentsForBlock(
            route,
            input.profile,
            input.areaID,
            input.floorID,
            input.region,
          )
        : []
    const firstPoint = route.blocks[0]?.points[0]
    const firstRaster =
      firstPoint && (!input.region || firstPoint.region === input.region)
        ? worldPositionToRaster(
            input.profile,
            input.areaID,
            input.floorID,
            firstPoint.region,
            firstPoint.x,
            firstPoint.y,
            firstPoint.z,
          )
        : null
    const firstVisible = blocks[0]?.[0]
    // A filtered or unmappable prefix must never turn a later visible segment
    // into the next waypoint for the current-position connector.
    const keepsFirstWaypoint =
      firstRaster &&
      firstVisible &&
      firstRaster.tileX === firstVisible.tileX &&
      firstRaster.tileY === firstVisible.tileY &&
      firstRaster.pixelX === firstVisible.pixelX &&
      firstRaster.pixelY === firstVisible.pixelY
    const anchor =
      !stale &&
      route.status === 'moving' &&
      route.current_anchor &&
      keepsFirstWaypoint &&
      (!input.region || route.current_anchor.region === input.region) &&
      route.blocks[0]?.area_id === input.areaID &&
      route.blocks[0]?.floor_id === input.floorID &&
      (input.areaID === 'world' ||
        route.current_anchor.region === firstPoint?.region) &&
      route.dataset_version === input.profile.dataset_version
        ? worldPositionToRaster(
            input.profile,
            input.areaID,
            input.floorID,
            route.current_anchor.region,
            route.current_anchor.x,
            route.current_anchor.y,
            route.current_anchor.z,
          ) || undefined
        : undefined
    overlays.push({
      id: `${route.character_id}:${route.session_id}:${route.route_sequence}`,
      characterID: route.character_id,
      characterName: character.name,
      sessionID: route.session_id,
      commandID: route.command_id,
      status: stale ? 'stale' : route.status,
      reason:
        route.geometry_omitted ||
        (route.blocks.length > 0 && blocks.length === 0 && !route.arrived)
          ? route.reason || 'route_geometry_unavailable'
          : route.reason,
      stale,
      selected:
        !input.selectedRouteID || input.selectedRouteID === route.character_id,
      blocks,
      currentAnchor: anchor,
    })
  }
  return overlays.sort((left, right) =>
    left.characterName.localeCompare(right.characterName),
  )
}

export function mapNavigationStatusLabel(
  status: string,
  geometryCount: number,
) {
  switch (status) {
    case 'submitting':
      return 'Submitting command'
    case 'failed':
    case 'rejected':
    case 'skipped':
    case 'expired':
    case 'unknown':
    case 'stopped':
    case 'stop_failed':
      return status.replaceAll('_', ' ')
    case 'superseded':
      return 'Replaced by a newer route'
    case 'stale':
      return 'Stale · last route frozen'
    case 'waiting_for_movement':
      return 'Waiting for movement evidence'
    case 'moving':
      return geometryCount
        ? 'Moving · remaining path shown'
        : 'Route geometry unavailable'
    case 'transition_awaiting_evidence':
      return 'Transition awaiting fresh position evidence'
    case 'waiting_for_arrival':
      return 'Waiting for arrival observation'
    case 'progress_uncertain':
      return 'Progress uncertain · connector suppressed'
    case 'arrived':
      return 'Arrived · observed position'
    default:
      return geometryCount ? status : 'Route geometry unavailable'
  }
}
