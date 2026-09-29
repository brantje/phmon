/** Shared by Leaflet's map and GridLayer; its GridLayer default minZoom is 0. */
export const MAP_ZOOM_OPTIONS = Object.freeze({
  minZoom: -1,
  maxZoom: 4,
})

export const INITIAL_MAP_ZOOM = Math.log2(1.25)
