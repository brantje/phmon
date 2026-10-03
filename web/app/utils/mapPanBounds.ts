export type MapPanBounds = {
  south: number
  west: number
  north: number
  east: number
}

/** Leaflet locks an axis when maxBounds is smaller than the viewport.
 * Pad only those axes by half the viewport so any point on a small cave
 * floor can reach the center. Larger floors and the outdoor grid keep
 * their existing edges. */
export function paddedMapPanBounds(
  content: MapPanBounds,
  viewport: { width: number; height: number },
  pixelsPerUnit: number,
): MapPanBounds {
  if (viewport.width <= 0 || viewport.height <= 0 || pixelsPerUnit <= 0)
    return content
  const width = (content.east - content.west) * pixelsPerUnit
  const height = (content.north - content.south) * pixelsPerUnit
  const padX = width <= viewport.width ? viewport.width / 2 / pixelsPerUnit : 0
  const padY =
    height <= viewport.height ? viewport.height / 2 / pixelsPerUnit : 0
  return {
    south: content.south - padY,
    west: content.west - padX,
    north: content.north + padY,
    east: content.east + padX,
  }
}
