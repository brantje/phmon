export interface GuideCellSquare {
  x: number
  y: number
  width: number
  height: number
}

/**
 * Client guide cells are stored one square north of the terrain they describe.
 * A square is that cell's own height in map tiles (4 for field groups, 1 for caves).
 */
export function guideCellOneSquareDown<T extends GuideCellSquare>(cell: T): T {
  return { ...cell, y: cell.y - cell.height }
}
