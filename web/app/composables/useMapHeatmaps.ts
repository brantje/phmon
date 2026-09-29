import { getCurrentScope, onScopeDispose, reactive, ref } from 'vue'
import type {
  HeatmapLayerID,
  HeatmapResetRequest,
  HeatmapResetResult,
  HeatmapResult,
  MobHeatmapFacet,
} from '~~/shared/types/mapAnalytics'

export interface HeatmapQuery {
  server: string
  area: string
  floor: string
  region?: number
  character_id?: string
  monster_type?: string
  model_id?: number
  from: string
  to: string
}

type HeatmapFetch = <T>(
  request: string,
  options?: {
    query?: Record<string, unknown>
    signal?: AbortSignal
    method?: string
    body?: unknown
  },
) => Promise<T>

const HISTORICAL_LAYERS: HeatmapLayerID[] = [
  'mob_observer_average',
  'mob_types',
  'deaths',
  'drops',
  'unique_sightings',
  'player_movement',
]

export function useMapHeatmaps(fetcher?: HeatmapFetch) {
  const request = fetcher || ($fetch as HeatmapFetch)
  const enabled = reactive<Record<HeatmapLayerID, boolean>>({
    mob_density: false,
    mob_observer_average: false,
    mob_types: false,
    deaths: false,
    drops: false,
    unique_sightings: false,
    player_movement: false,
  })
  const results = reactive<Partial<Record<HeatmapLayerID, HeatmapResult>>>({})
  const loading = reactive<Partial<Record<HeatmapLayerID, boolean>>>({})
  const errors = reactive<Partial<Record<HeatmapLayerID, string>>>({})
  const facets = ref<MobHeatmapFacet[]>([])
  const facetsLoading = ref(false)
  const facetsError = ref('')
  const controllers = new Map<HeatmapLayerID, AbortController>()
  let facetsController: AbortController | undefined
  let generation = 0

  function activeLayers() {
    return HISTORICAL_LAYERS.filter((layer) => enabled[layer])
  }

  async function refreshFacets(query: HeatmapQuery, currentGeneration: number) {
    if (currentGeneration !== generation) return
    facetsController?.abort()
    const controller = new AbortController()
    facetsController = controller
    facetsLoading.value = true
    facetsError.value = ''
    try {
      const {
        monster_type: _monsterType,
        model_id: _modelID,
        ...facetQuery
      } = query
      const response = await request<{ facets: MobHeatmapFacet[] }>(
        '/api/map/heatmap/facets',
        { query: facetQuery, signal: controller.signal },
      )
      if (currentGeneration === generation && facetsController === controller)
        facets.value = response.facets
    } catch (error) {
      if (
        currentGeneration === generation &&
        facetsController === controller &&
        !controller.signal.aborted
      ) {
        facetsError.value =
          error instanceof Error ? error.message : 'Mob facets unavailable'
      }
    } finally {
      if (currentGeneration === generation && facetsController === controller)
        facetsLoading.value = false
    }
  }

  async function refresh(query: HeatmapQuery) {
    const currentGeneration = ++generation
    const layers = activeLayers()
    const shouldLoadFacets = enabled.mob_types || enabled.mob_observer_average

    for (const layer of HISTORICAL_LAYERS) {
      if (!enabled[layer]) {
        controllers.get(layer)?.abort()
        controllers.delete(layer)
        results[layer] = undefined
        errors[layer] = undefined
        loading[layer] = undefined
      }
    }
    if (!shouldLoadFacets) {
      facetsController?.abort()
      facetsController = undefined
      facets.value = []
      facetsError.value = ''
      facetsLoading.value = false
    }

    const layerRequests = layers.map(async (layer) => {
      controllers.get(layer)?.abort()
      const controller = new AbortController()
      controllers.set(layer, controller)
      loading[layer] = true
      errors[layer] = ''
      try {
        const result = await request<HeatmapResult>('/api/map/heatmap', {
          query: { ...query, layer },
          signal: controller.signal,
        })
        if (
          currentGeneration === generation &&
          controllers.get(layer) === controller
        )
          results[layer] = result
      } catch (error) {
        if (
          currentGeneration === generation &&
          controllers.get(layer) === controller &&
          !controller.signal.aborted
        )
          errors[layer] =
            error instanceof Error ? error.message : 'Heatmap query failed'
      } finally {
        if (
          currentGeneration === generation &&
          controllers.get(layer) === controller
        )
          loading[layer] = false
      }
    })

    await Promise.all([
      ...layerRequests,
      shouldLoadFacets
        ? refreshFacets(query, currentGeneration)
        : Promise.resolve(),
    ])
  }

  async function reset(request: HeatmapResetRequest) {
    return await request<HeatmapResetResult>('/api/map/heatmap/reset', {
      method: 'POST',
      body: request,
    })
  }

  function clear() {
    generation++
    for (const controller of controllers.values()) controller.abort()
    controllers.clear()
    facetsController?.abort()
    facetsController = undefined
    for (const layer of HISTORICAL_LAYERS) {
      results[layer] = undefined
      loading[layer] = undefined
      errors[layer] = undefined
    }
    facets.value = []
    facetsError.value = ''
    facetsLoading.value = false
  }

  if (getCurrentScope()) onScopeDispose(clear)

  return {
    enabled,
    results,
    loading,
    errors,
    facets,
    facetsLoading,
    facetsError,
    activeLayers,
    refresh,
    reset,
    clear,
  }
}
