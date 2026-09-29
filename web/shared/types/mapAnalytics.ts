export type HeatmapLayerID =
  | 'mob_density'
  | 'mob_observer_average'
  | 'mob_types'
  | 'deaths'
  | 'drops'
  | 'unique_sightings'
  | 'player_movement'

export interface HeatmapPoint {
  region: number
  x: number
  y: number
  weight: number
  count: number
  numerator?: number
  denominator?: number
}

export interface HeatmapResult {
  layer: HeatmapLayerID
  status: 'available' | 'limited' | 'unsupported'
  metric: string
  interpretation: string
  reason?: string
  server: string
  dataset_version: string
  area_id: string
  floor_id: string
  region?: number
  from: string
  to: string
  resolution: number
  points: HeatmapPoint[]
  source_rows: number
  suppressed_rows: number
  truncated: boolean
}

export interface MobHeatmapFacet {
  monster_type?: string
  model_id?: number
  count: number
}

export interface HeatmapResetRequest {
  layer: HeatmapLayerID
  server: string
  area_id: string
  floor_id: string
  region?: number
  character_id?: string
  monster_type?: string
  model_id?: number
  from: string
  to: string
  confirm_broad: boolean
}

export interface HeatmapResetResult extends Omit<
  HeatmapResetRequest,
  'confirm_broad'
> {
  dataset_version: string
  broad_scope: boolean
  created_at: string
  reset_id: string
}
