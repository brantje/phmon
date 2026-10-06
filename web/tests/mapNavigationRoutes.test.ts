import assert from 'node:assert/strict'
import test from 'node:test'
import type { CharacterView, NavigationRoute } from '../shared/types/live.ts'
import type { MapProfile } from '../shared/types/map.ts'
import {
  mapNavigationRouteOverlays,
  mapNavigationStatusLabel,
} from '../app/utils/mapNavigationRoutes.ts'

const profile: MapProfile = {
  server: 'Greatest',
  dataset_id: 'route-fixture',
  dataset_version: 'route-v1',
  profile_status: 'partial',
  tiles: {
    status: 'available-for-inspection',
    orientation_status: 'outdoor-region-grid',
    tile_url_format: '/tiles/{x}x{y}.png',
    min_x: 26,
    max_x: 252,
    min_y: 35,
    max_y: 126,
    tile_count: 100,
    semantics: 'route map fixture',
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

const character: CharacterView = {
  character_id: 'character-one',
  server: 'Greatest',
  name: 'Wizard',
  online: true,
  session_id: 'session-one',
  region: 25000,
  x: 6420,
  y: 1080,
  z: 0,
  state_updated_at: '2026-09-30T12:00:01.000Z',
}

function route(overrides: Partial<NavigationRoute> = {}): NavigationRoute {
  return {
    command_id: 'cmd_navigation',
    character_id: character.character_id,
    session_id: character.session_id!,
    route_sequence: 1,
    server: 'Greatest',
    dataset_id: profile.dataset_id,
    dataset_version: profile.dataset_version,
    area_id: 'world',
    floor_id: 'world',
    destination: { region: 25000, x: 6460, y: 1080, z: 0 },
    destination_area_id: 'world',
    destination_floor_id: 'world',
    current_anchor: { region: 25000, x: 6430, y: 1080, z: 0 },
    status: 'moving',
    updated_at: '2026-09-30T12:00:01.000Z',
    blocks: [
      {
        area_id: 'world',
        floor_id: 'world',
        points: [
          { region: 25000, x: 6440, y: 1080, z: 0 },
          { region: 25000, x: 6450, y: 1080, z: 0 },
        ],
      },
      {
        area_id: 'world',
        floor_id: 'world',
        points: [{ region: 25000, x: 6460, y: 1080, z: 0 }],
      },
    ],
    ...overrides,
  }
}

function project(
  routes: NavigationRoute[],
  options: { stale?: boolean; character?: CharacterView } = {},
) {
  return mapNavigationRouteOverlays({
    routes,
    characters: [options.character || character],
    profile,
    server: 'greatest',
    areaID: 'world',
    floorID: 'world',
    region: 0,
    streamCurrent: !options.stale,
    liveStale: Boolean(options.stale),
    freshnessNow: Date.parse('2026-09-30T12:00:02.000Z'),
    selectedRouteID: '',
  })
}

test('keeps wait/teleport blocks independent and safely maps each segment', () => {
  const overlays = project([route()])
  assert.equal(overlays.length, 1)
  assert.equal(overlays[0]?.blocks.length, 2)
  assert.equal(overlays[0]?.blocks[0]?.length, 2)
  assert.equal(overlays[0]?.blocks[1]?.length, 1)
  assert.ok(overlays[0]?.currentAnchor)
})

test('stale routes freeze their remaining geometry and suppress the live connector', () => {
  const overlays = project([route()], { stale: true })
  assert.equal(overlays[0]?.status, 'stale')
  assert.equal(overlays[0]?.blocks.length, 2)
  assert.equal(overlays[0]?.currentAnchor, undefined)
  assert.equal(
    mapNavigationStatusLabel(overlays[0]!.status, overlays[0]!.blocks.length),
    'Stale · last route frozen',
  )
})

test('session replacement hides the old route and invalid profile geometry remains status-only', () => {
  assert.deepEqual(
    project([route()], {
      character: { ...character, session_id: 'replacement' },
    }),
    [],
  )
  const wrongDataset = project([route({ dataset_id: 'old-dataset' })])
  assert.deepEqual(wrongDataset, [])
  const wrongFloor = mapNavigationRouteOverlays({
    routes: [route()],
    characters: [character],
    profile,
    server: 'Greatest',
    areaID: 'unsupported-floor',
    floorID: '2F',
    region: 0,
    streamCurrent: true,
    liveStale: false,
    freshnessNow: Date.parse('2026-09-30T12:00:02.000Z'),
    selectedRouteID: '',
  })
  assert.equal(wrongFloor[0]?.blocks.length, 0)
  assert.equal(
    mapNavigationStatusLabel(wrongFloor[0]!.status, 0),
    'Route geometry unavailable',
  )
})

test('simultaneous character routes stay separate and route selection only dims overlays', () => {
  const secondCharacter = {
    ...character,
    character_id: 'character-two',
    name: 'Warrior',
    session_id: 'session-two',
  }
  const overlays = mapNavigationRouteOverlays({
    routes: [
      route(),
      route({
        command_id: 'cmd_navigation_two',
        character_id: secondCharacter.character_id,
        session_id: secondCharacter.session_id!,
        destination: { region: 25000, x: 6470, y: 1080, z: 0 },
        current_anchor: undefined,
      }),
    ],
    characters: [character, secondCharacter],
    profile,
    server: 'Greatest',
    areaID: 'world',
    floorID: 'world',
    region: 0,
    streamCurrent: true,
    liveStale: false,
    freshnessNow: Date.parse('2026-09-30T12:00:02.000Z'),
    selectedRouteID: secondCharacter.character_id,
  })
  assert.equal(overlays.length, 2)
  assert.notEqual(overlays[0]?.id, overlays[1]?.id)
  assert.equal(
    overlays.find((item) => item.characterID === character.character_id)
      ?.selected,
    false,
  )
  assert.equal(
    overlays.find((item) => item.characterID === secondCharacter.character_id)
      ?.selected,
    true,
  )
  assert.equal(
    overlays.find((item) => item.characterID === character.character_id)?.blocks
      .length,
    2,
  )
})

test('job temple route evidence scoped to 1F never renders on manually selected upper floors', () => {
  const temple = route({
    area_id: 'job-temple',
    floor_id: '1F',
    destination_area_id: 'job-temple',
    destination_floor_id: '1F',
    destination: { region: -32752, x: 500, y: 500, z: 100 },
    current_anchor: undefined,
    blocks: [
      {
        area_id: 'job-temple',
        floor_id: '1F',
        points: [{ region: -32752, x: 500, y: 500, z: 100 }],
      },
    ],
  })
  const upper = mapNavigationRouteOverlays({
    routes: [temple],
    characters: [{ ...character, region: -32752, x: 500, y: 500, z: 100 }],
    profile: {
      ...profile,
      areas: [
        {
          id: 'job-temple',
          label: 'Job Temple',
          kind: 'cave',
          region_mapping_status: 'reference-observed',
          floors: [
            {
              id: '1F',
              label: '1F',
              image_status: 'available',
              transform_status: 'validated',
            },
            {
              id: '2F',
              label: '2F',
              image_status: 'available',
              transform_status: 'validated',
            },
          ],
        },
      ],
      coordinate_transforms: [
        {
          area_id: 'job-temple',
          floor_id: '2F',
          region: -32752,
          status: 'validated',
          world_origin_x: 0,
          world_origin_y: 0,
          tile_origin_x: 10,
          tile_origin_y: 10,
          units_per_tile_x: 192,
          units_per_tile_y: 192,
          axis_x: 1,
          axis_y: 1,
        },
      ],
    },
    server: 'Greatest',
    areaID: 'job-temple',
    floorID: '2F',
    region: -32752,
    streamCurrent: true,
    liveStale: false,
    freshnessNow: Date.parse('2026-09-30T12:00:02.000Z'),
    selectedRouteID: '',
  })
  assert.equal(upper.length, 1)
  assert.equal(upper[0]?.blocks.length, 0)
})

test('outdoor tile seams remain one connected walk and allow its current connector', () => {
  const acrossSeam = route({
    current_anchor: { region: 25000, x: 6515, y: 1080, z: 0 },
    blocks: [
      {
        area_id: 'world',
        floor_id: 'world',
        points: [
          { region: 25001, x: 6540, y: 1080, z: 0 },
          { region: 25001, x: 6590, y: 1080, z: 0 },
          { region: 25002, x: 6740, y: 1080, z: 0 },
        ],
      },
    ],
  })
  const overlay = project([acrossSeam])[0]!
  assert.equal(overlay.blocks.length, 1)
  assert.equal(overlay.blocks[0]?.length, 3)
  assert.ok(overlay.currentAnchor)
})

test('a filtered prefix cannot attach the character to a later visible section', () => {
  const filtered = route({
    current_anchor: { region: 25001, x: 6540, y: 1080, z: 0 },
    blocks: [
      {
        area_id: 'world',
        floor_id: 'world',
        points: [
          { region: 25000, x: 6500, y: 1080, z: 0 },
          { region: 25001, x: 6580, y: 1080, z: 0 },
        ],
      },
    ],
  })
  const overlay = mapNavigationRouteOverlays({
    routes: [filtered],
    characters: [character],
    profile,
    server: 'Greatest',
    areaID: 'world',
    floorID: 'world',
    region: 25001,
    streamCurrent: true,
    liveStale: false,
    freshnessNow: Date.parse('2026-09-30T12:00:02.000Z'),
    selectedRouteID: '',
  })[0]!
  assert.equal(overlay.blocks[0]?.length, 1)
  assert.equal(overlay.currentAnchor, undefined)
})

test('a display-only route with no command still draws its remaining path', () => {
  const overlays = mapNavigationRouteOverlays({
    routes: [route({ command_id: '' })],
    characters: [character],
    profile,
    server: 'Greatest',
    areaID: 'world',
    floorID: 'world',
    region: 0,
    streamCurrent: true,
    liveStale: false,
    freshnessNow: Date.parse('2026-09-30T12:00:02.000Z'),
    selectedRouteID: '',
  })
  assert.equal(overlays.length, 1)
  assert.equal(overlays[0]?.commandID, '')
  assert.equal(overlays[0]?.status, 'moving')
  assert.ok((overlays[0]?.blocks.length ?? 0) > 0)
})
