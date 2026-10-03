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

/** Some cave rows list cells a few tiles inside the floor while the same
 * rooms continue to the edge. On each row and column, stretch only the outer
 * listed cells across a gap of at most three tiles. */
export function displayedGuideCells<T extends GuideCellSquare>(
  cells: readonly T[],
  floor: GuideFloorPlacement,
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
  const rows = new Map<number, number[]>()
  displayed.forEach((cell, index) => {
    rows.set(cell.y, [...(rows.get(cell.y) || []), index])
  })
  for (const indexes of rows.values()) {
    let minX = Infinity
    let maxX = -Infinity
    for (const index of indexes) {
      const cell = displayed[index]!
      minX = Math.min(minX, cell.x)
      maxX = Math.max(maxX, cell.x + cell.width - 1)
    }
    const left = extension(minX - floor.minX)
    const right = extension(floor.maxX - maxX)
    for (const index of indexes) {
      const cell = displayed[index]!
      if (left > 0 && cell.x === minX) {
        cell.x -= left
        cell.width += left
      }
      if (right > 0 && cell.x + cell.width - 1 === maxX) cell.width += right
    }
  }
  return displayed
}
