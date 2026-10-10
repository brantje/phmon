export const MIN_MAP_ZOOM_PERCENT = 0
export const MAX_MAP_ZOOM_PERCENT = 2000
export const INITIAL_MAP_ZOOM_PERCENT = 125
export const MAP_ZOOM_PERCENT_STEP = 5

export function mapZoomLevelForPercent(percent: number) {
  // 0% is not a finite map scale. The tile compositor treats that zoom as an
  // infinite grid, so the rendered floor is the lowest 5-point step.
  const safePercent =
    Number.isFinite(percent) && percent > 0 ? percent : MAP_ZOOM_PERCENT_STEP
  return Math.log2(safePercent / 100)
}

export function mapZoomPercentForLevel(level: number) {
  return Math.round(100 * 2 ** level)
}

export function snapMapZoomPercent(percent: number) {
  const stepped =
    Math.round(percent / MAP_ZOOM_PERCENT_STEP) * MAP_ZOOM_PERCENT_STEP
  return Math.min(MAX_MAP_ZOOM_PERCENT, Math.max(MIN_MAP_ZOOM_PERCENT, stepped))
}

/** Shared by Leaflet's map and GridLayer; its GridLayer default minZoom is 0. */
export const MAP_ZOOM_OPTIONS = Object.freeze({
  minZoom: mapZoomLevelForPercent(MIN_MAP_ZOOM_PERCENT),
  maxZoom: mapZoomLevelForPercent(MAX_MAP_ZOOM_PERCENT),
})

export const INITIAL_MAP_ZOOM = mapZoomLevelForPercent(INITIAL_MAP_ZOOM_PERCENT)

export function mapCanvasInitialZoomLevel(percent?: number | null) {
  if (percent == null || !Number.isFinite(percent)) return INITIAL_MAP_ZOOM
  return mapZoomLevelForPercent(snapMapZoomPercent(percent))
}
