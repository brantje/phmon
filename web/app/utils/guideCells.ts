export interface GuideCellSquare {
  x: number
  y: number
  width: number
  height: number
}

/**
 * Field guide cells are stored one square north of the terrain they describe.
 * A square is that cell's own height in map tiles. Cave cells use the floor's
 * MAP_MANAGER start row instead, so Job Temple moves onto the floor art while
 * Donwhang and the tomb keep their stored tiles.
 */
export function placedGuideCell<T extends GuideCellSquare>(
  cell: T,
  placement?: { floorMaxY?: number; guideOriginY?: number },
): T {
  if (placement?.guideOriginY != null && placement.floorMaxY != null)
    return {
      ...cell,
      y: cell.y + placement.floorMaxY - placement.guideOriginY,
    }
  return { ...cell, y: cell.y - cell.height }
}

export interface GuideFloorPlacement {
  minX: number
  maxX: number
  minY: number
  maxY: number
  guideOriginY?: number
}

export interface GuideRowEdge {
  y: number
  minX: number
  maxX: number
}

/** Some cave rows list cells a few tiles inside the floor while the same
 * rooms continue to the edge. Stretch only the outer cell on each row, by at
 * most three tiles. Full-floor edges come from the server so a viewport that
 * dropped the real outer cell cannot promote an inner one. */
export function displayedGuideCells<T extends GuideCellSquare>(
  cells: readonly T[],
  floor: GuideFloorPlacement,
  rowEdges?: readonly GuideRowEdge[],
): T[] {
  const placed = cells.map((cell) =>
    placedGuideCell(cell, {
      floorMaxY: floor.maxY,
      guideOriginY: floor.guideOriginY,
    }),
  )
  if (floor.guideOriginY == null || placed.length === 0) return placed
  const extension = (gap: number) => Math.max(0, Math.min(3, gap))
  const displayed = placed.map((cell) => ({ ...cell }))
  const rows =
    rowEdges ??
    [...new Map(displayed.map((cell) => [cell.y, cell])).keys()].map((y) => {
      const row = displayed.filter((cell) => cell.y === y)
      return {
        y,
        minX: Math.min(...row.map((cell) => cell.x)),
        maxX: Math.max(...row.map((cell) => cell.x + cell.width - 1)),
      }
    })
  for (const edge of rows) {
    const left = extension(edge.minX - floor.minX)
    const right = extension(floor.maxX - edge.maxX)
    for (const cell of displayed) {
      if (cell.y !== edge.y) continue
      if (left > 0 && cell.x === edge.minX) {
        cell.x -= left
        cell.width += left
      }
      if (right > 0 && cell.x + cell.width - 1 === edge.maxX)
        cell.width += right
    }
  }
  return displayed
}
