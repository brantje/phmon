import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapProfile } from '../shared/types/map.ts'
import { mapFollowView, toggleMapFollow } from '../app/utils/mapFollow.ts'

const profile = (): MapProfile => ({
  server: 'test',
  dataset_id: 'gamedata-test',
  dataset_version: 'test',
  profile_status: 'partial',
  tiles: {
    status: 'available-for-inspection',
    orientation_status: 'edge-continuity-supported',
    tile_url_format: '/tiles/{x}x{y}.png',
    min_x: 26,
    max_x: 252,
    min_y: 35,
    max_y: 126,
    tile_count: 240,
    semantics: 'test raster grid',
  },
  coordinate_transform_status: 'outdoor-region-grid',
  coordinate_transforms: [
    {
      area_id: 'world',
      floor_id: 'world',
      region: 25735,
      status: 'validated',
      world_origin_x: 0,
      world_origin_y: 1536,
      tile_origin_x: 135,
      tile_origin_y: 100,
      units_per_tile_x: 192,
      units_per_tile_y: 192,
      axis_x: 1,
      axis_y: 1,
    },
  ],
  region_mappings_status: 'outdoor-region-grid',
  command_z_evidence_status: 'unverified',
  quick_destinations: [],
  region_mappings: [
    {
      region: 25735,
      area_id: 'world',
      floor_id: 'world',
      tile_x: 135,
      tile_y: 100,
      status: 'validated',
    },
  ],
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
          image_status: 'available-for-inspection',
          transform_status: 'outdoor-region-grid',
        },
      ],
    },
    {
      id: 'donwhang-stone-cave',
      label: 'Donwhang Stone Cave',
      kind: 'cave',
      region_mapping_status: 'reference-observed',
      floors: [
        {
          id: '1F',
          label: '1F',
          image_status: 'available',
          transform_status: 'reference-observed',
          auto_detect: true,
          region_ids: [-32767, 32767],
          min_z: -50,
          max_z: 70,
          tiles: {
            status: 'available-for-inspection',
            orientation_status: 'reference-observed',
            tile_url_format: '/cave/{x}x{y}.png',
            min_x: 127,
            max_x: 129,
            min_y: 88,
            max_y: 90,
            tile_count: 9,
            semantics: 'test cave',
          },
        },
      ],
    },
  ],
  validation_requirements: [],
})

test('follow toggles off the same character and replaces another', () => {
  assert.equal(toggleMapFollow('', 'a'), 'a')
  assert.equal(toggleMapFollow('a', 'a'), '')
  assert.equal(toggleMapFollow('a', 'b'), 'b')
})

test('follow view stays on the outdoor floor for a world character', () => {
  const view = mapFollowView(
    profile(),
    { region: 25735, x: 98, y: 1559, z: 0 },
    'world',
    'world',
  )
  assert.ok(view)
  assert.equal(view.areaID, 'world')
  assert.equal(view.floorID, 'world')
  assert.ok(view.position)
  assert.equal(view.position.tileX, 135)
  assert.equal(view.position.tileY, 100)
})

test('follow view switches from the world map into a detected cave floor', () => {
  const view = mapFollowView(
    profile(),
    { region: -32767, x: -24294, y: -91, z: -9 },
    'world',
    'world',
  )
  assert.ok(view)
  assert.equal(view.areaID, 'donwhang-stone-cave')
  assert.equal(view.floorID, '1F')
})

test('follow view returns to the world map from a cave', () => {
  const view = mapFollowView(
    profile(),
    { region: 25735, x: 98, y: 1559, z: 0 },
    'donwhang-stone-cave',
    '1F',
  )
  assert.ok(view)
  assert.equal(view.areaID, 'world')
  assert.equal(view.floorID, 'world')
})

test('follow view is unavailable without a region', () => {
  assert.equal(
    mapFollowView(profile(), { x: 98, y: 1559, z: 0 }, 'world', 'world'),
    null,
  )
})
