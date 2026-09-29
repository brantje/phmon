export interface MarkerPosition {
  lat: number
  lng: number
}

export function interpolateMarkerPosition(
  start: MarkerPosition,
  target: MarkerPosition,
  progress: number,
): MarkerPosition {
  const boundedProgress = Math.max(0, Math.min(1, progress))
  const easedProgress =
    boundedProgress * boundedProgress * (3 - 2 * boundedProgress)

  return {
    lat: start.lat + (target.lat - start.lat) * easedProgress,
    lng: start.lng + (target.lng - start.lng) * easedProgress,
  }
}
