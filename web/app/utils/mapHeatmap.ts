import type {
  HeatmapLayerID,
  HeatmapResult,
} from '~~/shared/types/mapAnalytics'
import type { MapProfile } from '~~/shared/types/map'
import { worldPositionToRaster, type RasterPosition } from './mapCoordinates.ts'

export interface RenderedHeatPoint {
  position: RasterPosition
  intensity: number
  weight: number
  count: number
  numerator?: number
  denominator?: number
}

export interface MapHeatLayer {
  id: HeatmapLayerID
  label: string
  metric: string
  status: 'available' | 'limited'
  interpretation: string
  points: RenderedHeatPoint[]
}

const LABELS: Record<HeatmapLayerID, string> = {
  mob_density: 'Mob density',
  mob_observer_average: 'Observer-local mob average',
  mob_types: 'Monster rank sightings',
  deaths: 'Deaths',
  drops: 'Drops',
  unique_sightings: 'Unique sightings',
  player_movement: 'Player movement',
}

export function heatmapLayerLabel(layer: HeatmapLayerID) {
  return LABELS[layer]
}

export function historicalHeatmapWindow(
  range: string,
  now: number | Date = Date.now(),
  customFrom = '',
  customTo = '',
) {
  if (range === 'custom') {
    const from = Date.parse(customFrom)
    const to = Date.parse(customTo)
    if (!Number.isFinite(from) || !Number.isFinite(to) || to <= from)
      return null
    return {
      from: new Date(from).toISOString(),
      to: new Date(to).toISOString(),
    }
  }
  const durations: Record<string, number> = {
    '1h': 60 * 60_000,
    '24h': 24 * 60 * 60_000,
    '7d': 7 * 24 * 60 * 60_000,
    '30d': 30 * 24 * 60 * 60_000,
  }
  const duration = durations[range]
  if (!duration) return null
  const end = now instanceof Date ? now.getTime() : now
  return {
    from: new Date(end - duration).toISOString(),
    to: new Date(end).toISOString(),
  }
}

export function heatmapResultToLayer(
  result: HeatmapResult,
  profile: MapProfile,
): MapHeatLayer | null {
  if (result.status === 'unsupported') return null
  const mapped = result.points.flatMap((point) => {
    const position = worldPositionToRaster(
      profile,
      result.area_id,
      result.floor_id,
      point.region,
      point.x,
      point.y,
    )
    return position ? [{ point, position }] : []
  })
  const maximum = mapped.reduce(
    (value, entry) => Math.max(value, entry.point.weight),
    0,
  )
  return {
    id: result.layer,
    label: heatmapLayerLabel(result.layer),
    metric: result.metric,
    status: result.status,
    interpretation: result.interpretation,
    points: mapped.map(({ point, position }) => ({
      position,
      weight: point.weight,
      count: point.count,
      numerator: point.numerator,
      denominator: point.denominator,
      intensity:
        maximum > 0
          ? Math.max(0.08, Math.min(1, Math.sqrt(point.weight / maximum)))
          : 0,
    })),
  }
}
