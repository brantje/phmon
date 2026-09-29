import assert from 'node:assert/strict'
import test from 'node:test'
import type { HeatmapResult } from '../shared/types/mapAnalytics.ts'
import type { MapProfile } from '../shared/types/map.ts'
import {
  heatmapResultToLayer,
  historicalHeatmapWindow,
} from '../app/utils/mapHeatmap.ts'

const profile = {
  tiles: { status: 'available-for-inspection', min_x: 26, max_x: 252, min_y: 35, max_y: 126 },
  coordinate_transform_status: 'outdoor-region-grid',
  coordinate_transforms: [],
  region_mappings_status: 'outdoor-region-grid',
  areas: [{ id: 'world', floors: [{ id: 'world' }], region_mapping_status: 'outdoor-region-grid' }],
} as unknown as MapProfile

test('historical heatmap windows support month and validated custom bounds', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  const month = historicalHeatmapWindow('30d', now)
  assert.equal(Date.parse(month!.to) - Date.parse(month!.from), 30 * 24 * 60 * 60_000)
  assert.deepEqual(
    historicalHeatmapWindow('custom', now, '2026-09-28T10:00:00Z', '2026-09-28T11:00:00Z'),
    { from: '2026-09-28T10:00:00.000Z', to: '2026-09-28T11:00:00.000Z' },
  )
  assert.equal(historicalHeatmapWindow('custom', now, 'bad', 'also-bad'), null)
})

test('heatmap conversion reuses validated world coordinate transforms and normalizes weights', () => {
  const result = {
    layer: 'deaths',
    status: 'available',
    metric: 'death_occurrences',
    interpretation: 'fixture',
    server: 'greatest',
    dataset_version: 'gamedata-fixture',
    area_id: 'world',
    floor_id: 'world',
    from: '2026-09-29T10:00:00Z',
    to: '2026-09-29T11:00:00Z',
    resolution: 96,
    source_rows: 5,
    suppressed_rows: 0,
    truncated: false,
    points: [
      { region: 23687, x: 1, y: 1, weight: 1, count: 1 },
      { region: 23687, x: 20, y: 20, weight: 4, count: 4 },
    ],
  } satisfies HeatmapResult
  const layer = heatmapResultToLayer(result, profile)
  assert.ok(layer)
  assert.equal(layer!.points.length, 2)
  assert.equal(layer!.points[1]!.intensity, 1)
  assert.ok(layer!.points[0]!.intensity > 0 && layer!.points[0]!.intensity < 1)
})

test('unsupported historical layers never produce render points', () => {
  const result = {
    layer: 'mob_density',
    status: 'unsupported',
    metric: '',
    interpretation: 'coverage unverified',
    reason: 'observation_coverage_unverified',
    server: 'greatest',
    dataset_version: 'gamedata-fixture',
    area_id: 'world',
    floor_id: 'world',
    from: '2026-09-29T10:00:00Z',
    to: '2026-09-29T11:00:00Z',
    resolution: 96,
    source_rows: 0,
    suppressed_rows: 0,
    truncated: false,
    points: [],
  } satisfies HeatmapResult
  assert.equal(heatmapResultToLayer(result, profile), null)
})
