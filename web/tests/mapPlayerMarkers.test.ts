import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapOtherPlayer } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import { playerMapMarkers } from '../app/utils/mapPlayerMarkers.ts'
import { OTHER_PLAYER_ICON } from '../app/utils/mapPlayerPresentation.ts'

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
})

const player = (overrides: Partial<MapOtherPlayer> = {}): MapOtherPlayer => ({
  id: 'player:1',
  player_id: '8654977',
  name: 'Nearby',
  region: 25000,
  x: 6461.4,
  y: 1097.4,
  observer_region: 25000,
  observer_z: 0,
  observed_at: '2026-01-01T00:00:00Z',
  observers: [{ character_id: 'c1', session_id: 's1', name: 'Observer' }],
  ...overrides,
})

test('playerMapMarkers uses the configured other-player icon path', () => {
  assert.equal(
    OTHER_PLAYER_ICON,
    '/game-assets/interface/minimap/mm_sign_otherplayer.png',
  )
})

test('playerMapMarkers skips managed characters and party overlaps', () => {
  const markers = playerMapMarkers(
    profile(),
    'world',
    'world',
    [
      player({ name: 'Alpha', player_id: '1' }),
      player({ name: 'Bravo', player_id: '2' }),
      player({ name: 'Nearby', player_id: '3' }),
    ],
    ['alpha'],
    [
      {
        id: 'party:2',
        player_id: 2,
        name: 'Bravo',
        x: 1,
        y: 1,
        observer_character_id: 'c',
        observer_name: 'O',
        observer_session_id: 's',
        observer_region: 25000,
        observed_at: '',
      },
    ],
  )
  assert.deepEqual(
    markers.map((marker) => marker.label),
    ['Nearby'],
  )
})

test('playerMapMarkers rejects unsafe placement', () => {
  const markers = playerMapMarkers(profile(), 'world', 'world', [
    player({ region: 999999, x: 1, y: 1 }),
  ])
  assert.deepEqual(markers, [])
})
