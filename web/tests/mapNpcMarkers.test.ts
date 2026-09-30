import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapNpc } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import {
  NPC_MARKER_ICON,
  npcDisplayLabel,
  npcMapMarkers,
} from '../app/utils/mapNpcMarkers.ts'

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
    {
      area_id: 'world',
      floor_id: 'world',
      region: 25000,
      status: 'validated',
      world_origin_x: 6400,
      world_origin_y: 1000,
      tile_origin_x: 30,
      tile_origin_y: 40,
      units_per_tile_x: 192,
      units_per_tile_y: 192,
      axis_x: 1,
      axis_y: 1,
    },
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
  ],
  validation_requirements: [],
})

const npc = (overrides: Partial<MapNpc> = {}): MapNpc => ({
  id: 'npc:jangan',
  name: 'Jangan',
  servername: 'GATE_CH',
  model_id: 2094,
  role: 'teleporter',
  region: 25000,
  x: 6461.4,
  y: 1097.4,
  observed_at: '2026-10-01T00:00:00Z',
  observers: [{ character_id: 'alpha', session_id: 'session', name: 'Alpha' }],
  ...overrides,
})

test('npc marker uses the minimap npc icon and a name label', () => {
  assert.equal(
    NPC_MARKER_ICON,
    '/game-assets/interface/minimap/mm_sign_npc.png',
  )
  assert.equal(npcDisplayLabel(npc()), 'Jangan')
  assert.equal(npcDisplayLabel(npc({ name: '  ' })), 'GATE_CH')
  assert.equal(npcDisplayLabel(npc({ name: '', servername: '' })), 'NPC 2094')
  const markers = npcMapMarkers(profile(), 'world', 'world', [npc()])
  assert.equal(markers.length, 1)
  assert.equal(markers[0]?.label, 'Jangan')
  assert.equal(markers[0]?.kind, 'npc')
})

test('npc markers collapse duplicate ids and hide when the layer is off', () => {
  const map = profile()
  const rows = [
    npc(),
    npc({ id: 'npc:jangan' }),
    npc({
      id: 'npc:potion',
      name: 'Herbalist Yangyun',
      servername: 'NPC_CH_POTION',
      role: 'npc',
      x: 6494,
    }),
    npc({ id: 'npc:missing', region: 1 }),
  ]
  assert.equal(npcMapMarkers(map, 'world', 'world', rows).length, 2)
  assert.equal(npcMapMarkers(map, 'world', 'world', rows, false).length, 0)
})
