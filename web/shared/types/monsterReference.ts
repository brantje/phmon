export interface MonsterReferenceBounds {
  min_x: number
  max_x: number
  min_y: number
  max_y: number
}

export interface MonsterReferenceCell {
  x: number
  y: number
  width: number
  height: number
}

export interface MonsterReferenceArea {
  model_id: number
  code: string
  name: string
  level?: number
  group: string
  precision: 'guide-grid-cell'
  cells: MonsterReferenceCell[]
}

export interface MonsterReferencePoint {
  model_id: number
  code: string
  name: string
  level?: number
  region: number
  precision: 'client-reference-point'
  position: {
    tile_x: number
    tile_y: number
    pixel_x: number
    pixel_y: number
  }
}

export interface MonsterReferenceSearchRow {
  model_id: number
  code: string
  name: string
  level?: number
  area_id: string
  floor_id: string
  area_cells: number
  point_count: number
  bounds?: MonsterReferenceBounds
  location_status: 'placed' | 'unplaced'
}

export interface MonsterReferenceSearchResponse {
  status: 'available' | 'unavailable'
  dataset_id: string
  results?: MonsterReferenceSearchRow[]
  total?: number
  next_offset?: number | null
  reason?: string
}

export interface MonsterReferenceOverlayResponse {
  status: 'available' | 'unavailable'
  dataset_id: string
  area_id?: string
  floor_id?: string
  areas?: MonsterReferenceArea[]
  points?: MonsterReferencePoint[]
  point_total?: number
  next_offset?: number | null
  reason?: string
  provenance?: string
}
