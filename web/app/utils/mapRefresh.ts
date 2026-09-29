import type { MapSnapshot, LiveFilter } from '~~/shared/types/live'

/** Keep rendered map data only while its spatial scope still matches the request. */
export function mapSnapshotMatchesScope(
  snapshot: Pick<MapSnapshot, 'server' | 'area_id' | 'floor_id' | 'region'>,
  filter: Pick<LiveFilter, 'server' | 'area' | 'floor' | 'region'>,
) {
  return (
    snapshot.server.toLocaleLowerCase() ===
      (filter.server || '').toLocaleLowerCase() &&
    snapshot.area_id === (filter.area || 'world') &&
    snapshot.floor_id === (filter.floor || 'world') &&
    snapshot.region === (filter.region || 0)
  )
}
