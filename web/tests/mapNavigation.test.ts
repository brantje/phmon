import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapProfile } from '../shared/types/map.ts'
import {
  mapEventLocation,
  mapEventRoute,
  mapPreviewLocation,
  mapProfileRequestIsCurrent,
} from '../app/utils/mapNavigation.ts'

test('Stats map previews carry server, outdoor scope, region, and selected fresh character', () => {
  assert.deepEqual(
    mapPreviewLocation('greatest', {
      character_id: 'character-123',
      region: 25273,
    }),
    {
      path: '/map',
      query: {
        server: 'greatest',
        area: 'world',
        floor: 'world',
        region: '25273',
        character_id: 'character-123',
      },
    },
  )
})

test('Stats map previews omit an unavailable selection rather than retaining stale scope', () => {
  assert.deepEqual(mapPreviewLocation('greatest'), {
    path: '/map',
    query: { server: 'greatest', area: 'world', floor: 'world' },
  })
})

test('event coordinates resolve through the selected profile region mapping', () => {
  const profile: MapProfile = {
    server: 'test',
    dataset_id: 'gamedata-test',
    dataset_version: 'test',
    profile_status: 'partial',
    tiles: {
      status: 'available-for-inspection',
      orientation_status: 'edge-continuity-supported',
      tile_url_format: '/tiles/{x}x{y}.png',
      min_x: 0,
      max_x: 9,
      min_y: 0,
      max_y: 9,
      tile_count: 100,
      semantics: 'test raster grid',
    },
    coordinate_transform_status: 'validated',
    coordinate_transforms: [
      {
        area_id: 'tomb',
        floor_id: 'B1',
        region: 25273,
        status: 'validated',
        world_origin_x: 0,
        world_origin_y: 0,
        tile_origin_x: 5,
        tile_origin_y: 5,
        units_per_tile_x: 100,
        units_per_tile_y: 100,
        axis_x: 1,
        axis_y: 1,
      },
    ],
    region_mappings_status: 'validated',
    command_z_evidence_status: 'unverified',
    quick_destinations: [],
    region_mappings: [
      {
        region: 25273,
        area_id: 'tomb',
        floor_id: 'B1',
        tile_x: 5,
        tile_y: 5,
        status: 'validated',
      },
    ],
    view_presets: [],
    areas: [
      {
        id: 'tomb',
        label: 'Tomb',
        kind: 'cave',
        region_mapping_status: 'validated',
        floors: [
          {
            id: 'B1',
            label: 'B1',
            image_status: 'available',
            transform_status: 'validated',
          },
        ],
      },
    ],
    validation_requirements: [],
  }
  const mapped = mapEventLocation(profile, { region: 25273, x: 25, y: 75 })
  assert.equal(mapped.status, 'mapped')
  assert.equal(mapped.areaID, 'tomb')
  assert.equal(mapped.floorID, 'B1')
  assert.ok(mapped.position)
  assert.deepEqual(
    mapEventRoute('test', 'event-42', 'character-9', 25273, mapped),
    {
      path: '/map',
      query: {
        server: 'test',
        area: 'tomb',
        floor: 'B1',
        character_id: 'character-9',
        event_id: 'event-42',
      },
    },
  )
  assert.deepEqual(
    mapEventRoute('test', 'event-43', 'character-9', 25273, {
      ...mapped,
      areaID: 'world',
      floorID: 'world',
    }),
    {
      path: '/map',
      query: {
        server: 'test',
        area: 'world',
        floor: 'world',
        region: '25273',
        character_id: 'character-9',
        event_id: 'event-43',
      },
    },
  )

  profile.coordinate_transform_status = 'unvalidated'
  assert.equal(
    mapEventLocation(profile, { region: 25273, x: 25, y: 75 }).status,
    'coordinates-unmappable',
  )
  assert.equal(
    mapEventLocation(profile, { region: 1, x: 25, y: 75 }).status,
    'region-unmapped',
  )
})

test('a late map profile response is current only for the latest selected server', () => {
  assert.equal(mapProfileRequestIsCurrent(2, 2, 'greatest', 'greatest'), true)
  assert.equal(mapProfileRequestIsCurrent(1, 2, 'old', 'greatest'), false)
  assert.equal(mapProfileRequestIsCurrent(2, 2, 'old', 'greatest'), false)
})
