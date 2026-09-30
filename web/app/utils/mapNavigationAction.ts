import type { CharacterView } from '../../shared/types/live'
import type { MapProfile } from '../../shared/types/map'
import type { FanOutCommandDefinition, FanOutSkipReason } from './commandFanOut'
import {
  rasterPositionToGame,
  type GamePosition,
  type RasterPosition,
} from './mapCoordinates.ts'

export interface MapNavigationIntent {
  readonly point: Readonly<RasterPosition>
  readonly server: string
  readonly areaID: string
  readonly floorID: string
  readonly explicitRegion: number
  readonly datasetID: string
  readonly datasetVersion: string
  readonly targetIDs: readonly string[]
}

export interface MapNavigationIntentInput {
  point: RasterPosition
  server: string
  areaID: string
  floorID: string
  explicitRegion: number
  datasetID: string
  datasetVersion: string
  targetIDs: string[]
}

export interface MapNavigationResolution {
  destination?: GamePosition
  reason?: FanOutSkipReason
}

const reasons = {
  offline: { code: 'offline', message: 'Character is offline.' },
  stale: {
    code: 'stale_position',
    message:
      'Position is stale or unavailable; navigation requires a fresh position.',
  },
  session: {
    code: 'session_changed',
    message: 'Character session changed after navigation was prepared.',
  },
  profile: {
    code: 'unsupported_profile',
    message:
      'This area or floor has no supported navigation transform in the active map profile.',
  },
  temple: {
    code: 'unsupported_profile',
    message:
      'Job Temple navigation is supported only on the validated 1F floor.',
  },
  caveRegion: {
    code: 'ambiguous_cave_region',
    message:
      'This cave floor has multiple regions; select a compatible region or target a character currently in one.',
  },
  incompatibleRegion: {
    code: 'incompatible_region',
    message: 'The selected region does not belong to the chosen cave floor.',
  },
  point: {
    code: 'unmappable_point',
    message:
      'The selected raster point cannot be converted safely for this character.',
  },
} satisfies Record<string, FanOutSkipReason>

export function createMapNavigationIntent(
  input: MapNavigationIntentInput,
): MapNavigationIntent {
  const point = Object.freeze({ ...input.point })
  return Object.freeze({
    point,
    server: input.server,
    areaID: input.areaID,
    floorID: input.floorID,
    explicitRegion: input.explicitRegion,
    datasetID: input.datasetID,
    datasetVersion: input.datasetVersion,
    targetIDs: Object.freeze([...new Set(input.targetIDs)]),
  })
}

export function resolveMapNavigationDestination(
  intent: MapNavigationIntent,
  profile: MapProfile,
  character: CharacterView,
  now = Date.now(),
): MapNavigationResolution {
  if (!character.online || !character.session_id)
    return { reason: reasons.offline }
  if (
    character.server.toLowerCase() !== intent.server.toLowerCase() ||
    !character.session_id
  )
    return { reason: reasons.session }

  const observedAt = character.state_updated_at
    ? Date.parse(character.state_updated_at)
    : Number.NaN
  const age = now - observedAt
  if (
    !Number.isFinite(observedAt) ||
    age < -5_000 ||
    age > 35_000 ||
    character.region == null ||
    character.x == null ||
    character.y == null
  )
    return { reason: reasons.stale }

  if (
    profile.dataset_id !== intent.datasetID ||
    profile.dataset_version !== intent.datasetVersion
  )
    return { reason: reasons.profile }
  if (intent.areaID === 'job-temple' && intent.floorID !== '1F')
    return { reason: reasons.temple }

  const floor = profile.areas
    .find((area) => area.id === intent.areaID)
    ?.floors.find((item) => item.id === intent.floorID)
  if (!floor) return { reason: reasons.profile }

  let region: number | undefined
  if (intent.areaID === 'world') {
    // Outdoor region is encoded by the clicked raster tile. An explicit zone
    // filter constrains the point but never substitutes another character's region.
    region = intent.point.tileY * 256 + intent.point.tileX
    if (intent.explicitRegion && region !== intent.explicitRegion)
      return { reason: reasons.point }
  } else if (
    intent.explicitRegion &&
    !floor.region_ids?.includes(intent.explicitRegion)
  ) {
    return { reason: reasons.incompatibleRegion }
  } else if (floor.region_ids?.length === 1) {
    region = floor.region_ids[0]
  } else if (
    intent.explicitRegion &&
    floor.region_ids?.includes(intent.explicitRegion)
  ) {
    region = intent.explicitRegion
  } else if (character.region && floor.region_ids?.includes(character.region)) {
    region = character.region
  } else {
    return { reason: reasons.caveRegion }
  }

  const destination = rasterPositionToGame(
    profile,
    intent.areaID,
    intent.floorID,
    region,
    intent.point as RasterPosition,
    Number.isFinite(character.z) ? character.z : 0,
  )
  return destination ? { destination } : { reason: reasons.point }
}

export function mapNavigationCommand(state: {
  getIntent(): MapNavigationIntent | null
  getProfile(): MapProfile | null
  getCharacter(characterID: string): CharacterView | undefined
  getControls(characterID: string): {
    character_id: string
    session_id: string
    capabilities: Record<string, { supported: boolean }>
  } | null
  mapFeedCurrent(): boolean
  now(): number
}): FanOutCommandDefinition {
  const resolve = (character: CharacterView) => {
    const intent = state.getIntent()
    const profile = state.getProfile()
    if (!intent || !profile)
      return { reason: reasons.profile } as MapNavigationResolution
    return resolveMapNavigationDestination(
      intent,
      profile,
      character,
      state.now(),
    )
  }
  return {
    name: 'character.navigate',
    label: 'Navigate to map point',
    impact: 'movement',
    preEligibility(character) {
      if (!state.mapFeedCurrent())
        return {
          code: 'stale_map_scope',
          message: 'The current server/area/floor map snapshot is stale.',
        }
      return resolve(character).reason || null
    },
    buildArgs(character) {
      const result = resolve(character)
      if (!result.destination)
        throw new Error(result.reason?.message || reasons.point.message)
      return { ...result.destination }
    },
    summarizeArgs(args) {
      const intent = state.getIntent()
      return `${args.x}, ${args.y}, Z ${args.z} · ${intent?.server || 'server'} · ${intent?.areaID || 'map'} / ${intent?.floorID || 'floor'}`
    },
    admissionGuard(child, request, context) {
      const intent = state.getIntent()
      const profile = state.getProfile()
      if (!intent || !profile) return reasons.profile
      if (
        intent.targetIDs.indexOf(child.characterID) < 0 ||
        request.character_id !== child.characterID ||
        request.expected_session_id !== child.sessionID
      )
        return reasons.session
      if (
        profile.dataset_id !== intent.datasetID ||
        profile.dataset_version !== intent.datasetVersion
      )
        return reasons.profile
      const controls = state.getControls(child.characterID)
      if (
        !controls ||
        controls.character_id !== child.characterID ||
        controls.session_id !== child.sessionID ||
        !controls.capabilities['character.navigate']?.supported
      )
        return {
          code: 'unsupported',
          message:
            'The current session no longer reports character.navigate support.',
        }
      const current = state.getCharacter(child.characterID)
      if (!current || current.session_id !== child.sessionID)
        return reasons.session
      const resolved = resolveMapNavigationDestination(
        intent,
        profile,
        current,
        state.now(),
      )
      if (!resolved.destination) return resolved.reason || reasons.point
      if (
        !context?.exactRetry &&
        JSON.stringify(resolved.destination) !== JSON.stringify(request.args)
      )
        return {
          code: 'arguments_changed',
          message:
            'Position, scope or destination changed after preparation. Reopen the map action to prepare a new command.',
        }
      if (!state.mapFeedCurrent())
        return {
          code: 'stale_map_scope',
          message: 'The current server/area/floor map snapshot is stale.',
        }
      return null
    },
  }
}
