export interface MapFloorProfile {
  id: string
  label: string
  image_status: string
  transform_status: string
}

export interface MapAreaProfile {
  id: string
  label: string
  kind: 'outdoor' | 'cave'
  region_mapping_status: string
  floors: MapFloorProfile[]
}

export interface MapProfile {
  server: string
  dataset_id: string
  dataset_version: string
  profile_status: string
  tiles: {
    status: string
    orientation_status: string
    tile_url_format: string
    min_x: number
    max_x: number
    min_y: number
    max_y: number
    tile_count: number
    semantics: string
  }
  coordinate_transform_status: string
  coordinate_transforms: MapCoordinateTransform[]
  region_mappings_status: string
  command_z_evidence_status: string
  quick_destinations: MapQuickDestination[]
  region_mappings: MapRegionMapping[]
  view_presets: MapViewPreset[]
  areas: MapAreaProfile[]
  validation_requirements: string[]
}

export interface MapQuickDestination {
  id: string
  label: string
  area_id: string
  floor_id: string
  region: number
  x: number
  y: number
  z: number
  status: string
}

export interface MapRegionMapping {
  region: number
  area_id: string
  floor_id: string
  tile_x: number
  tile_y: number
  status: string
}

export interface MapViewPreset {
  id: string
  label: string
  area_id: string
  floor_id: string
  tile_x: number
  tile_y: number
  zoom: number
  status: string
}

export interface MapCoordinateTransform {
  area_id: string
  floor_id: string
  region: number
  status: 'unvalidated' | 'validated'
  world_origin_x: number
  world_origin_y: number
  tile_origin_x: number
  tile_origin_y: number
  units_per_tile_x: number
  units_per_tile_y: number
  axis_x: 1 | -1
  axis_y: 1 | -1
  command_z?: number
}
