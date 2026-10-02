import type { MapNpc, MapTeleportRoute } from '../../shared/types/live'

export function sortedTeleportRoutes(npc: Pick<MapNpc, 'teleport_routes'>): MapTeleportRoute[] {
  const routes = npc.teleport_routes
  if (!routes?.length) return []
  return [...routes].sort((left, right) =>
    left.destination.localeCompare(right.destination),
  )
}

export function pickDefaultTeleportDestination(
  npc: Pick<MapNpc, 'name' | 'teleport_routes'>,
  preferred?: string,
): string {
  const routes = sortedTeleportRoutes(npc)
  if (!routes.length) {
    if (preferred?.trim()) return preferred.trim()
    if (npc.name?.toLowerCase() === 'hotan') return 'Jangan'
    return ''
  }
  const wanted = preferred?.trim().toLowerCase()
  if (wanted) {
    const match = routes.find(
      (route) => route.destination.toLowerCase() === wanted,
    )
    if (match) return match.destination
  }
  const jangan = routes.find(
    (route) => route.destination.toLowerCase() === 'jangan',
  )
  if (jangan) return jangan.destination
  return routes[0]!.destination
}
