import assert from 'node:assert/strict'
import test from 'node:test'
import { effectScope } from 'vue'
import type {
  HeatmapResult,
  MobHeatmapFacet,
} from '../shared/types/mapAnalytics.ts'
import {
  type HeatmapQuery,
  useMapHeatmaps,
} from '../app/composables/useMapHeatmaps.ts'

function deferred<T>() {
  let resolve!: (value: T) => void
  let reject!: (error: unknown) => void
  const promise = new Promise<T>((res, rej) => {
    resolve = res
    reject = rej
  })
  return { promise, resolve, reject }
}

function result(server: string): HeatmapResult {
  return {
    layer: 'mob_types',
    status: 'available',
    metric: 'monster_sighting_count',
    interpretation: 'fixture',
    server,
    dataset_version: 'gamedata-fixture',
    area_id: 'world',
    floor_id: 'world',
    from: '2026-09-29T10:00:00Z',
    to: '2026-09-29T11:00:00Z',
    resolution: 96,
    points: [],
    source_rows: 0,
    suppressed_rows: 0,
    truncated: false,
  }
}

test('stale heatmap refresh cannot start or install facets after a newer scope', async () => {
  const requests: Array<{
    url: string
    server: string
    monsterType?: string
    signal?: AbortSignal
    deferred: ReturnType<typeof deferred<unknown>>
  }> = []
  const fetcher = <T>(
    url: string,
    options?: {
      query?: Record<string, unknown>
      signal?: AbortSignal
      method?: string
      body?: unknown
    },
  ) => {
    const pending = deferred<unknown>()
    requests.push({
      url,
      server: String(options?.query?.server || ''),
      monsterType: options?.query?.monster_type as string | undefined,
      signal: options?.signal,
      deferred: pending,
    })
    return pending.promise as Promise<T>
  }

  const scope = effectScope()
  const heatmaps = scope.run(() => useMapHeatmaps(fetcher))!
  heatmaps.enabled.mob_types = true

  const queryA: HeatmapQuery = {
    server: 'scope-a',
    area: 'world',
    floor: 'world',
    monster_type: 'General',
    from: '2026-09-29T10:00:00Z',
    to: '2026-09-29T11:00:00Z',
  }
  const queryB: HeatmapQuery = {
    ...queryA,
    server: 'scope-b',
    monster_type: 'Champion',
  }

  const refreshA = heatmaps.refresh(queryA)
  await Promise.resolve()
  const aHeat = requests.find(
    (request) => request.url === '/api/map/heatmap' && request.server === 'scope-a',
  )!
  const aFacet = requests.find(
    (request) =>
      request.url === '/api/map/heatmap/facets' && request.server === 'scope-a',
  )!
  assert.ok(aHeat)
  assert.ok(aFacet)
  assert.equal(aFacet.monsterType, undefined)

  const refreshB = heatmaps.refresh(queryB)
  await Promise.resolve()
  const bHeat = requests.find(
    (request) => request.url === '/api/map/heatmap' && request.server === 'scope-b',
  )!
  const bFacet = requests.find(
    (request) =>
      request.url === '/api/map/heatmap/facets' && request.server === 'scope-b',
  )!
  assert.ok(bHeat)
  assert.ok(bFacet)
  assert.equal(bFacet.monsterType, undefined)
  assert.equal(aHeat.signal?.aborted, true)
  assert.equal(aFacet.signal?.aborted, true)
  assert.equal(bFacet.signal?.aborted, false)

  bHeat.deferred.resolve(result('scope-b'))
  bFacet.deferred.resolve({
    facets: [{ monster_type: 'B', count: 2 } satisfies MobHeatmapFacet],
  })
  await refreshB
  assert.deepEqual(heatmaps.facets.value, [{ monster_type: 'B', count: 2 }])

  aHeat.deferred.resolve(result('scope-a'))
  aFacet.deferred.resolve({
    facets: [{ monster_type: 'A', count: 1 } satisfies MobHeatmapFacet],
  })
  await refreshA

  assert.equal(bFacet.signal?.aborted, false)
  assert.deepEqual(heatmaps.facets.value, [{ monster_type: 'B', count: 2 }])
  assert.equal(heatmaps.results.mob_types?.server, 'scope-b')

  scope.stop()
})
