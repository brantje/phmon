import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapProfile } from '../shared/types/map.ts'
import {
  caveFloorForPosition,
  leafletToRasterPosition,
  rasterTileCenterToLeaflet,
  rasterPositionToGame,
  worldPositionToRaster,
} from '../app/utils/mapCoordinates.ts'
import {
  characterHasDisplayableMapPosition,
  characterMapMarkers,
  displayableMapCharacters,
} from '../app/utils/mapCharacterMarkers.ts'

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
    max_x: 40,
    min_y: 35,
    max_y: 50,
    tile_count: 240,
    semantics: 'test raster grid',
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
      units_per_tile_y: 500,
      axis_x: -1,
      axis_y: 1,
      command_z: 6,
    },
  ],
  region_mappings_status: 'validated',
  command_z_evidence_status: 'verified',
  quick_destinations: [],
  region_mappings: [],
  view_presets: [
    {
      id: 'root-reference',
      label: 'Root tile reference',
      area_id: 'world',
      floor_id: 'world',
      tile_x: 30,
      tile_y: 40,
      zoom: 0,
      status: 'raster-reference-only',
    },
  ],
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

const greatestOutdoorProfile = (): MapProfile => {
  const result = profile()
  result.server = 'greatest'
  result.dataset_id = 'gamedata-47c969ded0613d4c2a22'
  result.tiles.min_x = 26
  result.tiles.max_x = 252
  result.tiles.min_y = 35
  result.tiles.max_y = 126
  result.coordinate_transform_status = 'outdoor-region-grid'
  result.region_mappings_status = 'outdoor-region-grid'
  result.command_z_evidence_status = 'unverified'
  result.areas = [
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
  ]
  const regions = [
    { region: 24744, tileX: 168, tileY: 96 },
    { region: 25000, tileX: 168, tileY: 97 },
    { region: 25735, tileX: 135, tileY: 100 },
    { region: 23941, tileX: 133, tileY: 93 },
  ]
  result.region_mappings = regions.map(({ region, tileX, tileY }) => ({
    region,
    area_id: 'world',
    floor_id: 'world',
    tile_x: tileX,
    tile_y: tileY,
    status: 'validated',
  }))
  result.coordinate_transforms = regions.map(({ region, tileX, tileY }) => ({
    area_id: 'world',
    floor_id: 'world',
    region,
    status: 'validated',
    world_origin_x: (tileX - 135) * 192,
    world_origin_y: (tileY - 92) * 192,
    tile_origin_x: tileX,
    tile_origin_y: tileY,
    units_per_tile_x: 192,
    units_per_tile_y: 192,
    axis_x: 1,
    axis_y: 1,
  }))
  return result
}

test('documented phBot outdoor positions land inside their exported region tiles', () => {
  const mapProfile = greatestOutdoorProfile()
  for (const [region, x, y, tileX, tileY] of [
    [24744, 6435.89990234375, 828.7999877929688, 168, 96],
    [25000, 6428.2373046875, 1086.672607421875, 168, 97],
    [25735, 96.4, 1558.9, 135, 100],
    [23941, -215.07, 231.064, 133, 93],
  ]) {
    const raster = worldPositionToRaster(
      mapProfile,
      'world',
      'world',
      region!,
      x!,
      y!,
    )
    assert.ok(raster)
    assert.equal(raster.tileX, tileX)
    assert.equal(raster.tileY, tileY)
    assert.ok(raster.pixelX >= 0 && raster.pixelX < 256)
    assert.ok(raster.pixelY >= 0 && raster.pixelY < 256)
  }
  const live = worldPositionToRaster(
    mapProfile,
    'world',
    'world',
    25735,
    96.4,
    1558.9,
  )
  assert.ok(live)
  assert.ok(Math.abs(live.pixelX - (96.4 / 192) * 256) < 1e-9)
  assert.ok(Math.abs(live.pixelY - (1 - 22.9 / 192) * 256) < 1e-9)
  assert.equal(
    worldPositionToRaster(mapProfile, 'world', 'world', 25735, 192, 1558.9),
    null,
    'an old region ID cannot place a boundary-crossing coordinate in a new tile',
  )
  assert.equal(
    rasterPositionToGame(mapProfile, 'world', 'world', 25735, live),
    null,
    'map commands still lack verified Z',
  )
})

test('four fresh characters in one outdoor tile retain distinct exact pixels', () => {
  const markers = characterMapMarkers(
    greatestOutdoorProfile(),
    'world',
    'world',
    [
      {
        character_id: 'one',
        name: 'nuker1',
        region: 25735,
        x: 96.4,
        y: 1558.9,
      },
      {
        character_id: 'two',
        name: 'nuker2',
        region: 25735,
        x: 94.4,
        y: 1558.7,
      },
      {
        character_id: 'three',
        name: 'nuker3',
        region: 25735,
        x: 96.3,
        y: 1557.8,
      },
      {
        character_id: 'four',
        name: 'nuker4',
        region: 25735,
        x: 93.4,
        y: 1553.6,
      },
    ],
  )
  assert.equal(markers.length, 4)
  assert.ok(markers.every((marker) => marker.placement === 'exact'))
  assert.equal(
    new Set(
      markers.map(
        (marker) => `${marker.position.pixelX}:${marker.position.pixelY}`,
      ),
    ).size,
    4,
  )
})

test('offline and stale online characters retain last observed map positions', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  const offline = {
    character_id: 'offline',
    name: 'MagicBuff',
    online: false,
    state_updated_at: '2026-09-20T12:00:00Z',
    region: 25735,
    x: 96.4,
    y: 1558.9,
  }
  assert.equal(characterHasDisplayableMapPosition(offline, now), true)
  assert.equal(
    characterHasDisplayableMapPosition({ ...offline, x: undefined }, now),
    true,
  )
  assert.equal(
    characterHasDisplayableMapPosition(
      { ...offline, online: true, state_updated_at: '2026-09-29T11:00:00Z' },
      now,
    ),
    true,
  )
})

test('list filtering passes the actual clock to every character', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  const characters = ['one', 'two'].map((character_id) => ({
    character_id,
    name: character_id,
    region: 25735,
    state_updated_at: '2026-09-29T11:59:59Z',
  }))
  assert.deepEqual(displayableMapCharacters(characters, now), characters)
})

test('a lagged UI clock still keeps last observed map pins', () => {
  const laggedNow = Date.parse('2026-09-29T12:00:00Z')
  const character = {
    character_id: 'nuker1',
    name: 'nuker1',
    online: true,
    region: 25735,
    x: 96.4,
    y: 1558.9,
    state_updated_at: '2026-09-29T12:00:20Z',
  }
  assert.equal(characterHasDisplayableMapPosition(character, laggedNow), true)
  assert.deepEqual(displayableMapCharacters([character], laggedNow), [
    character,
  ])
})

test('encoded outdoor region is primary without an explicit region mapping', () => {
  const mapProfile = greatestOutdoorProfile()
  mapProfile.region_mappings = []
  mapProfile.coordinate_transforms = []
  for (const [region, x, y, tileX, tileY] of [
    [23687, 114, 16, 135, 92], // Hotan
    [26520, 3423.1, 2115.2, 152, 103], // live Donwhang
  ]) {
    const raster = worldPositionToRaster(
      mapProfile,
      'world',
      'world',
      region!,
      x!,
      y!,
    )
    assert.ok(raster)
    assert.equal(raster.tileX, tileX)
    assert.equal(raster.tileY, tileY)
  }
  assert.equal(
    worldPositionToRaster(mapProfile, 'world', 'world', 23687, 3423.1, 2115.2),
    null,
    'a mixed region and coordinate sample must not jump to a different tile',
  )
  const markers = characterMapMarkers(mapProfile, 'world', 'world', [
    { character_id: 'nuker1', name: 'nuker1', region: 23687, x: 114, y: 16 },
    {
      character_id: 'donwhang',
      name: 'donwhang',
      region: 26520,
      x: 3423.1,
      y: 2115.2,
    },
  ])
  assert.equal(markers.length, 2)
  assert.ok(markers.every((marker) => marker.placement === 'exact'))
})

test('a known region without X/Y is labelled approximate and an unknown region is omitted', () => {
  const markers = characterMapMarkers(
    greatestOutdoorProfile(),
    'world',
    'world',
    [
      {
        character_id: 'tile-only',
        name: 'tile-only',
        region: 25735,
        zone: 'Jangan',
      },
      { character_id: 'unknown', name: 'unknown', region: 1, x: 1, y: 1 },
    ],
  )
  assert.equal(markers.length, 1)
  assert.equal(markers[0]?.placement, 'region-tile')
  assert.match(markers[0]?.label || '', /Jangan/)
  assert.doesNotMatch(markers[0]?.label || '', /Region 25735/)
  assert.deepEqual(markers[0]?.position, {
    tileX: 135,
    tileY: 100,
    pixelX: 128,
    pixelY: 128,
  })
})

test('validated transform converts position to raster and back with axis reversal', () => {
  const mapProfile = profile()
  const raster = worldPositionToRaster(
    mapProfile,
    'world',
    'world',
    25273,
    750,
    2250,
  )
  assert.deepEqual(raster, { tileX: 30, tileY: 40, pixelX: 64, pixelY: 128 })
  assert.deepEqual(
    raster && rasterPositionToGame(mapProfile, 'world', 'world', 25273, raster),
    { region: 25273, x: 750, y: 2250, z: 6 },
  )
})

test('root view preset opens and reads back its exact raster tile', () => {
  const tiles = {
    ...profile().tiles,
    max_x: 252,
    max_y: 126,
  }
  const center = rasterTileCenterToLeaflet(tiles, 168, 97)
  assert.deepEqual(center, {
    lat: -(29 * 256 + 128),
    lng: (168 - 26) * 256 + 128,
  })
  assert.deepEqual(leafletToRasterPosition(tiles, center!.lat, center!.lng), {
    tileX: 168,
    tileY: 97,
    pixelX: 128,
    pixelY: 128,
  })
})

test('game Y increases upward while tile-local pixel Y increases downward', () => {
  const mapProfile = profile()
  const north = worldPositionToRaster(
    mapProfile,
    'world',
    'world',
    25273,
    1000,
    2251,
  )
  assert.ok(north)
  assert.equal(north.tileX, 30)
  assert.equal(north.tileY, 40)
  assert.ok(Math.abs(north.pixelY - 127.488) < 1e-9)
  const converted =
    north && rasterPositionToGame(mapProfile, 'world', 'world', 25273, north)
  assert.ok(converted)
  assert.equal(converted.region, 25273)
  assert.equal(converted.x, 1000)
  assert.ok(Math.abs(converted.y - 2251) < 1e-9)
  assert.equal(converted.z, 6)
})

test('coordinate conversion refuses unvalidated, wrong-region, off-catalog and no-Z transforms', () => {
  const mapProfile = profile()
  assert.equal(
    worldPositionToRaster(mapProfile, 'world', 'world', 1, 750, 2250),
    null,
  )
  assert.equal(
    worldPositionToRaster(mapProfile, 'cave', '1F', 25273, 750, 2250),
    null,
  )
  assert.equal(
    worldPositionToRaster(mapProfile, 'world', 'world', 25273, 50_000, 2250),
    null,
  )
  mapProfile.coordinate_transforms[0]!.status = 'unvalidated'
  assert.equal(
    worldPositionToRaster(mapProfile, 'world', 'world', 25273, 750, 2250),
    null,
  )
  mapProfile.coordinate_transforms[0]!.status = 'validated'
  delete mapProfile.coordinate_transforms[0]!.command_z
  assert.equal(
    rasterPositionToGame(mapProfile, 'world', 'world', 25273, {
      tileX: 30,
      tileY: 40,
      pixelX: 64,
      pixelY: 128,
    }),
    null,
  )
})

test('point conversion derives outdoor region from the clicked tile and reuses current Z', () => {
  const mapProfile = greatestOutdoorProfile()
  const result = rasterPositionToGame(
    mapProfile,
    'world',
    'world',
    25735,
    {
      tileX: 168,
      tileY: 97,
      pixelX: 128,
      pixelY: 128,
    },
    -93.5,
  )
  assert.deepEqual(result, { region: 25000, x: 6432, y: 1056, z: -93.5 })
})

test('cave floor grids convert signed X/Y and keep Z from the selected character', () => {
  const mapProfile = profile()
  mapProfile.coordinate_transform_status = 'outdoor-region-grid'
  mapProfile.region_mappings_status = 'outdoor-region-grid'
  const floor = (id: string, minZ: number, maxZ: number) => ({
    id,
    label: id,
    image_status: 'available',
    transform_status: 'reference-observed',
    tiles: {
      status: 'available-for-inspection',
      orientation_status: 'reference-observed',
      tile_url_format: `/game-assets/minimap_d/donwhang/dh_a01_floor${id === '1F' ? '01' : '02'}_{x}x{y}.png`,
      min_x: 127,
      max_x: 129,
      min_y: 126,
      max_y: 128,
      tile_count: 9,
      semantics: 'test cave grid',
    },
    region_ids: [-32767, 32767],
    min_z: minZ,
    max_z: maxZ,
    auto_detect: true,
  })
  mapProfile.areas.push({
    id: 'donwhang-stone-cave',
    label: 'Donwhang',
    kind: 'cave',
    region_mapping_status: 'reference-observed',
    floors: [floor('1F', -50, 70), floor('2F', 71, 210)],
  })
  mapProfile.coordinate_transforms.push(
    ...[-32767, 32767].flatMap((region) => [
      {
        area_id: 'donwhang-stone-cave',
        floor_id: '1F',
        region,
        status: 'validated' as const,
        world_origin_x: -24384,
        world_origin_y: -192,
        tile_origin_x: 128,
        tile_origin_y: 127,
        units_per_tile_x: 192,
        units_per_tile_y: 192,
        axis_x: 1 as const,
        axis_y: 1 as const,
      },
      {
        area_id: 'donwhang-stone-cave',
        floor_id: '2F',
        region,
        status: 'validated' as const,
        world_origin_x: -24384,
        world_origin_y: -192,
        tile_origin_x: 128,
        tile_origin_y: 127,
        units_per_tile_x: 192,
        units_per_tile_y: 192,
        axis_x: 1 as const,
        axis_y: 1 as const,
      },
    ]),
  )
  assert.deepEqual(caveFloorForPosition(mapProfile, -32767, 0), {
    areaID: 'donwhang-stone-cave',
    floorID: '1F',
  })
  assert.deepEqual(caveFloorForPosition(mapProfile, 32767, 71), {
    areaID: 'donwhang-stone-cave',
    floorID: '2F',
  })
  assert.equal(caveFloorForPosition(mapProfile, -32767, undefined), null)
  assert.equal(caveFloorForPosition(mapProfile, -32767, 70.5), null)
  const raster = worldPositionToRaster(
    mapProfile,
    'donwhang-stone-cave',
    '1F',
    -32767,
    -24272.5,
    -93.5,
    0,
  )
  assert.ok(raster)
  assert.equal(raster.tileX, 128)
  const selectedOnOtherFloor =
    raster &&
    rasterPositionToGame(
      mapProfile,
      'donwhang-stone-cave',
      '2F',
      -32767,
      raster,
      0,
    )
  assert.ok(selectedOnOtherFloor)
  assert.equal(selectedOnOtherFloor.region, -32767)
  assert.equal(selectedOnOtherFloor.z, 0)
  const fallback =
    raster &&
    rasterPositionToGame(
      mapProfile,
      'donwhang-stone-cave',
      '2F',
      -32767,
      raster,
      undefined,
    )
  assert.ok(fallback)
  assert.equal(fallback.z, 0)
  assert.equal(
    rasterPositionToGame(
      mapProfile,
      'donwhang-stone-cave',
      '2F',
      undefined,
      raster!,
      0,
    ),
    null,
  )
})
