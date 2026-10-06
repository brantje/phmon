import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapThief } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import {
  thiefMapMarkers,
  thiefOriginLabel,
} from '../app/utils/mapThiefMarkers.ts'

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
  command_z_evidence_status: 'unverified',
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

const thief = (overrides: Partial<MapThief> = {}): MapThief => ({
  id: 'thief:greatest:bandit',
  sighting_id: 'sighting-1',
  name: 'Bandit',
  region: 25000,
  x: 6461.4,
  y: 1097.4,
  position_source: 'thief',
  origin: 'external',
  reporter_app: 'AdvancedAutoTrade',
  observed_at: '2026-10-06T19:00:00Z',
  ...overrides,
})

test('thief markers use the stable id and skip unmappable positions', () => {
  const markers = thiefMapMarkers(profile(), 'world', 'world', [
    thief(),
    thief({ id: 'thief:greatest:missing', name: 'Missing', region: 1 }),
  ])
  assert.equal(markers.length, 1)
  assert.equal(markers[0]?.id, 'thief:greatest:bandit')
  assert.equal(markers[0]?.kind, 'thief')
  assert.equal(markers[0]?.label, 'Bandit')
})

test('thief origin labels distinguish PhMon from an external app', () => {
  assert.equal(thiefOriginLabel(thief({ origin: 'phmon' })), 'PhMon')
  assert.equal(thiefOriginLabel(thief()), 'AdvancedAutoTrade')
  assert.equal(
    thiefOriginLabel(thief({ reporter_app: '' })),
    'AdvancedAutoTrade',
  )
})
