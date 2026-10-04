import assert from 'node:assert/strict'
import test from 'node:test'
import type { CharacterView, ControlsSnapshot } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import {
  createMapNavigationIntent,
  mapNavigationCommand,
  mapTrainingPositionCommand,
  resolveMapNavigationDestination,
  resolveMapTrainingPosition,
} from '../app/utils/mapNavigationAction.ts'
import type {
  FanOutChild,
  FanOutCommandRequest,
} from '../app/utils/commandFanOut.ts'

function baseProfile(): MapProfile {
  return {
    server: 'greatest',
    dataset_id: 'fixture-dataset',
    dataset_version: 'fixture-v1',
    profile_status: 'partial',
    tiles: {
      status: 'available-for-inspection',
      orientation_status: 'validated',
      tile_url_format: '/tiles/{x}x{y}.png',
      min_x: 26,
      max_x: 252,
      min_y: 35,
      max_y: 126,
      tile_count: 100,
      semantics: 'test raster',
    },
    coordinate_transform_status: 'outdoor-region-grid',
    coordinate_transforms: [],
    region_mappings_status: 'outdoor-region-grid',
    command_z_evidence_status: 'unverified',
    quick_destinations: [],
    region_mappings: [],
    view_presets: [],
    areas: [
      {
        id: 'world',
        label: 'World',
        kind: 'outdoor',
        region_mapping_status: 'outdoor-region-grid',
        floors: [
          {
            id: 'world',
            label: 'World',
            image_status: 'available',
            transform_status: 'outdoor-region-grid',
          },
        ],
      },
    ],
    validation_requirements: [],
  }
}

function character(id: string, region = 25000, z = 40): CharacterView {
  return {
    character_id: id,
    server: 'Greatest',
    name: id,
    online: true,
    session_id: `${id}-session`,
    region,
    x: 6420,
    y: 1080,
    z,
    state_updated_at: new Date(1_000_000).toISOString(),
  }
}

function intent(
  overrides: Partial<Parameters<typeof createMapNavigationIntent>[0]> = {},
) {
  return createMapNavigationIntent({
    point: { tileX: 168, tileY: 97, pixelX: 128, pixelY: 128 },
    server: 'Greatest',
    areaID: 'world',
    floorID: 'world',
    explicitRegion: 0,
    datasetID: 'fixture-dataset',
    datasetVersion: 'fixture-v1',
    targetIDs: ['one', 'two', 'one'],
    ...overrides,
  })
}

test('intent freezes the raster destination and ordered-deduplicates action targets', () => {
  const point = { tileX: 168, tileY: 97, pixelX: 128, pixelY: 128 }
  const selected = ['one', 'two', 'one']
  const captured = intent({ point, targetIDs: selected })
  point.pixelX = 0
  selected.push('three')
  assert.deepEqual(captured.point, {
    tileX: 168,
    tileY: 97,
    pixelX: 128,
    pixelY: 128,
  })
  assert.deepEqual(captured.targetIDs, ['one', 'two'])
})

test('outdoor destination uses the clicked tile and each target current Z', () => {
  const mapProfile = baseProfile()
  const captured = intent()
  const first = resolveMapNavigationDestination(
    captured,
    mapProfile,
    character('one', 25000, 71),
    1_000_000,
  )
  const second = resolveMapNavigationDestination(
    captured,
    mapProfile,
    character('two', 24744, 92),
    1_000_000,
  )
  assert.equal(first.destination?.region, 25000)
  assert.equal(second.destination?.region, 25000)
  assert.equal(first.destination?.z, 71)
  assert.equal(second.destination?.z, 92)
  assert.equal(first.destination?.x, second.destination?.x)
  assert.equal(first.destination?.y, second.destination?.y)
})

test('stale positions and explicit outdoor region mismatch are skipped with reasons', () => {
  const mapProfile = baseProfile()
  const stale = resolveMapNavigationDestination(
    intent(),
    mapProfile,
    character('one'),
    1_100_000,
  )
  assert.equal(stale.reason?.code, 'stale_position')
  const mismatch = resolveMapNavigationDestination(
    intent({ explicitRegion: 24744 }),
    mapProfile,
    character('one'),
    1_000_000,
  )
  assert.equal(mismatch.reason?.code, 'unmappable_point')
})

test('cave region selection uses unique, explicit or target-local region evidence', () => {
  const mapProfile = baseProfile()
  mapProfile.areas = [
    {
      id: 'donwhang-cave',
      label: 'Donwhang Cave',
      kind: 'cave',
      region_mapping_status: 'validated',
      floors: [
        {
          id: '1F',
          label: '1F',
          image_status: 'available',
          transform_status: 'validated',
          region_ids: [-32767, -32766],
          min_z: -100,
          max_z: 100,
          auto_detect: true,
        },
      ],
    },
  ]
  mapProfile.tiles.min_x = 0
  mapProfile.tiles.max_x = 30
  mapProfile.tiles.min_y = 0
  mapProfile.tiles.max_y = 30
  mapProfile.coordinate_transform_status = 'validated'
  mapProfile.coordinate_transforms = [-32767, -32766].map((region) => ({
    area_id: 'donwhang-cave',
    floor_id: '1F',
    region,
    status: 'validated' as const,
    world_origin_x: 0,
    world_origin_y: 0,
    tile_origin_x: 10,
    tile_origin_y: 10,
    units_per_tile_x: 256,
    units_per_tile_y: 256,
    axis_x: 1 as const,
    axis_y: 1 as const,
  }))
  const captured = intent({
    areaID: 'donwhang-cave',
    floorID: '1F',
    point: { tileX: 10, tileY: 10, pixelX: 128, pixelY: 128 },
  })
  const selectedCharacter = character('cave', -32766, 5)
  const inferred = resolveMapNavigationDestination(
    captured,
    mapProfile,
    selectedCharacter,
    1_000_000,
  )
  assert.equal(inferred.destination?.region, -32766)
  const explicit = resolveMapNavigationDestination(
    createMapNavigationIntent({ ...captured, explicitRegion: -32767 }),
    mapProfile,
    selectedCharacter,
    1_000_000,
  )
  assert.equal(explicit.destination?.region, -32767)
  const ambiguous = resolveMapNavigationDestination(
    captured,
    mapProfile,
    character('elsewhere', 25000),
    1_000_000,
  )
  assert.equal(ambiguous.reason?.code, 'ambiguous_cave_region')
})

test('Job Temple manually selected upper floors stay unsupported', () => {
  const mapProfile = baseProfile()
  mapProfile.areas = [
    {
      id: 'job-temple',
      label: 'Job Temple',
      kind: 'cave',
      region_mapping_status: 'validated',
      floors: [
        {
          id: '2F',
          label: '2F',
          image_status: 'available',
          transform_status: 'validated',
          region_ids: [25273],
        },
      ],
    },
  ]
  const result = resolveMapNavigationDestination(
    intent({ areaID: 'job-temple', floorID: '2F' }),
    mapProfile,
    character('temple', 25273),
    1_000_000,
  )
  assert.equal(result.reason?.code, 'unsupported_profile')
  assert.match(result.reason?.message || '', /1F/)
})

test('terrain height changes preserve both initial submission and exact retry destinations', () => {
  const mapProfile = baseProfile()
  const captured = intent({ targetIDs: ['one'] })
  const current = character('one', 25000, 40)
  const original = resolveMapNavigationDestination(
    captured,
    mapProfile,
    current,
    1_000_000,
  ).destination!
  const controls = {
    character_id: 'one',
    session_id: 'one-session',
    capabilities: { 'character.navigate': { supported: true } },
  } as ControlsSnapshot
  const definition = mapNavigationCommand({
    getIntent: () => captured,
    getProfile: () => mapProfile,
    getCharacter: () => current,
    getControls: () => controls,
    mapFeedCurrent: () => true,
    now: () => 1_000_000,
  })
  const child = { characterID: 'one', sessionID: 'one-session' } as FanOutChild
  const request = {
    character_id: 'one',
    expected_session_id: 'one-session',
    name: 'character.navigate',
    args: { ...original },
    idempotency_key: 'navigation-exact-key',
    confirmation: false,
  } satisfies FanOutCommandRequest

  assert.deepEqual(definition.buildArgs(current), original)
  current.z = 48
  assert.deepEqual(definition.buildArgs(current), original)
  assert.equal(
    definition.admissionGuard?.(child, request, { exactRetry: false }),
    null,
  )
  assert.equal(
    definition.admissionGuard?.(child, request, { exactRetry: true }),
    null,
  )
  assert.deepEqual(request.args, original)
})

test('training position allows stale positions and reuses current Z or zero', () => {
  const mapProfile = baseProfile()
  const stale = character('one', 25000, 71)
  stale.state_updated_at = new Date(0).toISOString()
  const resolved = resolveMapTrainingPosition(intent(), mapProfile, stale)
  assert.equal(resolved.destination?.region, 25000)
  assert.equal(resolved.destination?.z, 71)

  const noZ = { ...character('two'), z: undefined }
  assert.equal(
    resolveMapTrainingPosition(intent(), mapProfile, noZ).destination?.z,
    0,
  )
  const offline = { ...character('three'), online: false }
  assert.equal(
    resolveMapTrainingPosition(intent(), mapProfile, offline).reason?.code,
    'offline',
  )
  const otherDataset = resolveMapTrainingPosition(
    intent({ datasetVersion: 'fixture-v2' }),
    mapProfile,
    character('four'),
  )
  assert.equal(otherDataset.reason?.code, 'unsupported_profile')
})

test('training position command builds position mode and guards admission', () => {
  const mapProfile = baseProfile()
  const captured = intent({ targetIDs: ['one'] })
  const current = character('one', 25000, 40)
  let controls = {
    character_id: 'one',
    session_id: 'one-session',
    capabilities: {
      'training.area.set': { supported: true, modes: ['position', 'named'] },
    },
    training: { session_id: 'one-session', training_available: true },
  } as ControlsSnapshot
  const definition = mapTrainingPositionCommand({
    getIntent: () => captured,
    getProfile: () => mapProfile,
    getCharacter: () => current,
    getControls: () => controls,
    mapFeedCurrent: () => true,
  })
  assert.equal(definition.name, 'training.area.set')
  assert.equal(definition.preEligibility?.(current), null)
  const args = definition.buildArgs(current)
  assert.equal(args.mode, 'position')
  assert.equal(args.region, 25000)
  assert.equal(args.z, 40)

  const child = { characterID: 'one', sessionID: 'one-session' } as FanOutChild
  const request = {
    character_id: 'one',
    expected_session_id: 'one-session',
    name: 'training.area.set',
    args,
    idempotency_key: 'training-key',
    confirmation: false,
  } satisfies FanOutCommandRequest
  assert.equal(definition.admissionGuard?.(child, request), null)

  current.z = 55
  assert.equal(
    definition.admissionGuard?.(child, request, { exactRetry: false })?.code,
    'arguments_changed',
  )
  assert.equal(
    definition.admissionGuard?.(child, request, { exactRetry: true }),
    null,
  )

  controls = {
    ...controls,
    capabilities: {
      'training.area.set': { supported: true, modes: ['named'] },
    },
  }
  assert.equal(
    definition.admissionGuard?.(child, request, { exactRetry: true })?.code,
    'unsupported',
  )

  controls = {
    ...controls,
    training: { session_id: 'one-session', training_available: false },
  }
  assert.equal(definition.preEligibility?.(current)?.code, 'no_training_area')
})
