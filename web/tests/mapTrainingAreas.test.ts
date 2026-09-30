import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapTrainingArea } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import {
  rasterPixelsToWorldRadius,
  worldRadiusToRasterPixels,
} from '../app/utils/mapCoordinates.ts'
import {
  clampTrainingRadius,
  reconcileTrainingDraft,
  runTrainingApplySteps,
  trainingApplySteps,
  trainingAreaOverlays,
  trainingDraftDirty,
  type TrainingAreaDraft,
} from '../app/utils/mapTrainingAreas.ts'

const transform = (
  area_id: string,
  floor_id: string,
  region: number,
  units: number,
) => ({
  area_id,
  floor_id,
  region,
  status: 'validated',
  world_origin_x: 0,
  world_origin_y: 0,
  tile_origin_x: 30,
  tile_origin_y: 40,
  units_per_tile_x: units,
  units_per_tile_y: units,
  axis_x: 1 as const,
  axis_y: 1 as const,
})

const profile = (): MapProfile => ({
  server: 'test',
  dataset_id: 'test',
  dataset_version: 'test',
  profile_status: 'partial',
  tiles: {
    status: 'available-for-inspection',
    orientation_status: 'edge-continuity-supported',
    tile_url_format: '/tiles/{x}x{y}.png',
    min_x: 20,
    max_x: 40,
    min_y: 20,
    max_y: 50,
    tile_count: 651,
    semantics: 'test',
  },
  coordinate_transform_status: 'validated',
  coordinate_transforms: [
    transform('world', 'world', 25735, 192),
    transform('cave', '1F', -32767, 1000),
  ],
  region_mappings_status: 'validated',
  command_z_evidence_status: 'verified',
  quick_destinations: [],
  region_mappings: [],
  view_presets: [],
  areas: [
    {
      id: 'world',
      label: 'World',
      kind: 'outdoor',
      region_mapping_status: 'validated',
      floors: [
        {
          id: 'world',
          label: 'World',
          image_status: 'available',
          transform_status: 'validated',
        },
      ],
    },
    {
      id: 'cave',
      label: 'Cave',
      kind: 'cave',
      region_mapping_status: 'validated',
      floors: [
        {
          id: '1F',
          label: '1F',
          image_status: 'available',
          transform_status: 'validated',
          region_ids: [-32767],
          min_z: -50,
          max_z: 70,
          auto_detect: true,
        },
      ],
    },
  ],
  validation_requirements: [],
})

const area = (overrides: Partial<MapTrainingArea> = {}): MapTrainingArea => ({
  character_id: 'char-a',
  session_id: 'session-a',
  name: 'Alpha',
  region: 25735,
  x: 96,
  y: 96,
  z: 0,
  radius: 48,
  ...overrides,
})

test('training radius uses the transform scale in both directions', () => {
  const map = profile()
  assert.equal(worldRadiusToRasterPixels(map, 'world', 'world', 25735, 48), 64)
  assert.equal(rasterPixelsToWorldRadius(map, 'world', 'world', 25735, 64), 48)
  assert.equal(worldRadiusToRasterPixels(map, 'cave', '1F', -32767, 500), 128)
  assert.equal(worldRadiusToRasterPixels(map, 'world', 'world', 1, 48), null)
  assert.equal(worldRadiusToRasterPixels(map, 'world', 'world', 25735, 0), null)
  assert.equal(clampTrainingRadius(0), 1)
  assert.equal(clampTrainingRadius(12.4), 12)
  assert.equal(clampTrainingRadius(20000), 10000)
})

test('overlays place observed areas and drop unmappable or filtered rows', () => {
  const map = profile()
  const overlays = trainingAreaOverlays({
    profile: map,
    areaID: 'world',
    floorID: 'world',
    areas: [area(), area({ character_id: 'char-b', region: 1 })],
    regionFilter: 0,
    selectedID: 'char-a',
    draft: null,
  })
  assert.equal(overlays.length, 1)
  assert.equal(overlays[0]?.radiusPixels, 64)
  assert.equal(overlays[0]?.selected, true)
  assert.equal(overlays[0]?.draft, false)

  const filtered = trainingAreaOverlays({
    profile: map,
    areaID: 'world',
    floorID: 'world',
    areas: [area()],
    regionFilter: 25000,
    selectedID: '',
    draft: null,
  })
  assert.equal(filtered.length, 0)

  const cave = trainingAreaOverlays({
    profile: map,
    areaID: 'cave',
    floorID: '1F',
    areas: [area({ region: -32767, x: -5000, y: -100, z: -9, radius: 500 })],
    regionFilter: 32767,
    selectedID: '',
    draft: null,
  })
  assert.equal(cave.length, 1)
  assert.equal(cave[0]?.radiusPixels, 128)
})

test('a draft previews its own session only', () => {
  const map = profile()
  const draft: TrainingAreaDraft = {
    characterID: 'char-a',
    sessionID: 'session-a',
    center: { region: 25735, x: 192, y: 96, z: 0 },
    radius: 96,
  }
  const [preview] = trainingAreaOverlays({
    profile: map,
    areaID: 'world',
    floorID: 'world',
    areas: [area()],
    regionFilter: 0,
    selectedID: 'char-a',
    draft,
  })
  assert.equal(preview?.draft, true)
  assert.equal(preview?.radiusPixels, 128)

  const [other] = trainingAreaOverlays({
    profile: map,
    areaID: 'world',
    floorID: 'world',
    areas: [area({ session_id: 'session-b' })],
    regionFilter: 0,
    selectedID: 'char-a',
    draft,
  })
  assert.equal(other?.draft, false)
  assert.equal(other?.radiusPixels, 64)
})

test('reconcile keeps only fields that still differ from readback', () => {
  const draft: TrainingAreaDraft = {
    characterID: 'char-a',
    sessionID: 'session-a',
    center: { region: 25735, x: 150, y: 96, z: 0 },
    radius: 60,
  }
  assert.equal(reconcileTrainingDraft(draft, area()), draft)

  const centerApplied = reconcileTrainingDraft(draft, area({ x: 150.3, y: 96 }))
  assert.deepEqual(centerApplied, {
    characterID: 'char-a',
    sessionID: 'session-a',
    radius: 60,
  })
  assert.equal(trainingDraftDirty(centerApplied, area({ x: 150 })), true)

  const allApplied = reconcileTrainingDraft(draft, area({ x: 150, radius: 60 }))
  assert.equal(
    trainingDraftDirty(allApplied, area({ x: 150, radius: 60 })),
    false,
  )

  assert.equal(
    reconcileTrainingDraft(draft, area({ session_id: 'session-b' })),
    null,
  )
  assert.equal(reconcileTrainingDraft(draft, undefined), null)
})

test('apply steps are ordered center then radius and omit clean fields', () => {
  const draft: TrainingAreaDraft = {
    characterID: 'char-a',
    sessionID: 'session-a',
    center: { region: 25735, x: 150, y: 96, z: 0 },
    radius: 60,
  }
  assert.deepEqual(trainingApplySteps(draft, area()), [
    {
      name: 'training.area.set',
      args: { mode: 'position', region: 25735, x: 150, y: 96, z: 0 },
    },
    { name: 'training.radius.set', args: { radius: 60 } },
  ])
  assert.deepEqual(trainingApplySteps(draft, area({ radius: 60 })), [
    {
      name: 'training.area.set',
      args: { mode: 'position', region: 25735, x: 150, y: 96, z: 0 },
    },
  ])
  assert.deepEqual(trainingApplySteps(draft, area({ session_id: 'x' })), [])
})

test('an uncertain step blocks the rest while known failures continue', async () => {
  const steps = trainingApplySteps(
    {
      characterID: 'char-a',
      sessionID: 'session-a',
      center: { region: 25735, x: 150, y: 96, z: 0 },
      radius: 60,
    },
    area(),
  )
  const calls: string[] = []
  const blocked = await runTrainingApplySteps(steps, async (step) => {
    calls.push(step.name)
    return { outcome: 'uncertain' }
  })
  assert.deepEqual(calls, ['training.area.set'])
  assert.deepEqual(
    blocked.map((result) => result.outcome),
    ['uncertain', 'not_sent'],
  )

  calls.length = 0
  const continued = await runTrainingApplySteps(steps, async (step) => {
    calls.push(step.name)
    return {
      outcome: step.name === 'training.area.set' ? 'failed' : 'completed',
    }
  })
  assert.deepEqual(calls, ['training.area.set', 'training.radius.set'])
  assert.deepEqual(
    continued.map((result) => result.outcome),
    ['failed', 'completed'],
  )
})
