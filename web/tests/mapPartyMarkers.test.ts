import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapPartyMember } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import { partyMapMarkers } from '../app/utils/mapPartyMarkers.ts'
import { PARTY_MEMBER_ICON } from '../app/utils/mapPartyPresentation.ts'

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
      region: 25273,
      status: 'validated',
      world_origin_x: 1000,
      world_origin_y: 2000,
      tile_origin_x: 30,
      tile_origin_y: 40,
      units_per_tile_x: 1000,
      units_per_tile_y: 1000,
      axis_x: 1,
      axis_y: 1,
    },
    {
      area_id: 'cave',
      floor_id: '1F',
      region: -32767,
      status: 'validated',
      world_origin_x: -25000,
      world_origin_y: -1000,
      tile_origin_x: 30,
      tile_origin_y: 40,
      units_per_tile_x: 1000,
      units_per_tile_y: 1000,
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

const member = (overrides: Partial<MapPartyMember> = {}): MapPartyMember => ({
  id: 'party:55:ally',
  party_id: '55',
  player_id: 500,
  name: 'Ally',
  hp_percent: 80,
  mp_percent: 90,
  x: 1000,
  y: 2000,
  observer_character_id: 'observer',
  observer_name: 'Observer',
  observer_session_id: 'session',
  observer_region: 25273,
  observed_at: '2026-09-30T00:00:00Z',
  ...overrides,
})

test('party marker uses the configured Silkroad party minimap icon', () => {
  assert.equal(
    PARTY_MEMBER_ICON,
    '/game-assets/interface/minimap/mm_sign_party.png',
  )
})

test('party markers reuse exact world transform and fail closed on bad scope', () => {
  assert.equal(
    partyMapMarkers(profile(), 'world', 'world', [member()]).length,
    1,
  )
  assert.equal(
    partyMapMarkers(profile(), 'world', 'world', [
      member({ observer_region: 25274 }),
    ]).length,
    0,
  )
  assert.equal(
    partyMapMarkers(profile(), 'world', 'world', [
      member({ x: 999999 }),
    ]).length,
    0,
  )
})

test('managed character suppresses duplicate only while supplied as visible', () => {
  assert.equal(
    partyMapMarkers(profile(), 'world', 'world', [member()], [' ally '])
      .length,
    0,
  )
  assert.equal(
    partyMapMarkers(profile(), 'world', 'world', [member()], []).length,
    1,
  )
})

test('cave party placement requires observer floor Z', () => {
  const cave = member({
    id: 'party:1:caveally',
    name: 'CaveAlly',
    observer_region: -32767,
    observer_z: -9,
    x: -24294,
    y: -91,
  })
  assert.equal(partyMapMarkers(profile(), 'cave', '1F', [cave]).length, 1)
  const { observer_z: _ignored, ...withoutZ } = cave
  assert.equal(
    partyMapMarkers(profile(), 'cave', '1F', [
      withoutZ as MapPartyMember,
    ]).length,
    0,
  )
})
