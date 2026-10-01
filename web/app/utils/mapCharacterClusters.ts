/** Group portrait anchors in screen pixels without changing reported positions. */
export function mapCharacterClusters(
  points: { id: string; x: number; y: number }[],
  distance = 24,
): string[][] {
  const parents = points.map((_, index) => index)
  function root(index: number): number {
    return parents[index] === index
      ? index
      : (parents[index] = root(parents[index]!))
  }
  for (let a = 0; a < points.length; a++)
    for (let b = a + 1; b < points.length; b++) {
      if (
        Math.hypot(points[a]!.x - points[b]!.x, points[a]!.y - points[b]!.y) <=
        distance
      )
        parents[root(b)] = root(a)
    }
  const groups = new Map<number, string[]>()
  points.forEach((point, index) => {
    const key = root(index),
      group = groups.get(key) || []
    group.push(point.id)
    groups.set(key, group)
  })
  return [...groups.values()]
    .filter((group) => group.length > 1)
    .map((group) => group.sort())
}
