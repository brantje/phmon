import assert from 'node:assert/strict'
import test from 'node:test'
import type { MapSnapshot } from '../shared/types/live.ts'
import { mapSnapshotMatchesScope } from '../app/utils/mapRefresh.ts'

const snapshot = (): Pick<
  MapSnapshot,
  'server' | 'area_id' | 'floor_id' | 'region'
> => ({
  server: 'Greatest',
  area_id: 'world',
  floor_id: 'world',
  region: 25735,
})

test('map refresh keeps cached data when only the time window changes', () => {
  const current = snapshot()
  const refreshedFilter = {
    server: 'greatest',
    area: 'world',
    floor: 'world',
    region: 25735,
    from: '2026-09-29T10:00:00.000Z',
    to: '2026-09-29T11:00:00.000Z',
  }

  assert.equal(mapSnapshotMatchesScope(current, refreshedFilter), true)
})

test('map refresh drops cached data when server or spatial scope changes', () => {
  const current = snapshot()

  assert.equal(
    mapSnapshotMatchesScope(current, {
      server: 'Sevar',
      area: 'world',
      floor: 'world',
      region: 25735,
    }),
    false,
  )
  assert.equal(
    mapSnapshotMatchesScope(current, {
      server: 'Greatest',
      area: 'jangan-cave',
      floor: 'floor-1',
      region: 0,
    }),
    false,
  )
  assert.equal(
    mapSnapshotMatchesScope(current, {
      server: 'Greatest',
      area: 'world',
      floor: 'world',
      region: 25736,
    }),
    false,
  )
})
