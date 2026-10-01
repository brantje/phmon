import type { FanOutOperation } from './commandFanOut.ts'
import type { MapRouteOverlay } from './mapNavigationRoutes.ts'

export interface MapNavigationTrayRow {
  id: string
  characterID: string
  name: string
  status: string
  detail: string
  geometryCount: number
  group: 'active' | 'done' | 'attention'
}
export function navigationTrayGroup(
  status: string,
): MapNavigationTrayRow['group'] {
  if (status === 'arrived') return 'done'
  return [
    'submitting',
    'waiting_for_movement',
    'moving',
    'transition_awaiting_evidence',
    'waiting_for_arrival',
  ].includes(status)
    ? 'active'
    : 'attention'
}
export function mapNavigationTrayRows(
  routes: MapRouteOverlay[],
  operations: FanOutOperation[],
  server: string,
  dismissed: ReadonlySet<string>,
): MapNavigationTrayRow[] {
  const rows: MapNavigationTrayRow[] = routes.map((route) => ({
    id: route.id,
    characterID: route.characterID,
    name: route.characterName,
    status: route.status,
    detail: route.reason || '',
    geometryCount: route.blocks.length,
    group: navigationTrayGroup(route.status),
  }))
  for (const operation of operations) {
    if (
      operation.command.name !== 'character.navigate' ||
      ['prepared', 'cancelled'].includes(operation.state)
    )
      continue
    for (const child of operation.children) {
      if (
        child.server.toLowerCase() !== server.toLowerCase() ||
        routes.some(
          (route) =>
            route.commandID === child.commandID &&
            route.sessionID === child.sessionID &&
            route.characterID === child.characterID,
        )
      )
        continue
      const status =
        child.submission === 'ready' || child.submission === 'submitting'
          ? 'submitting'
          : child.submission === 'accepted' &&
              !['failed', 'expired', 'unknown'].includes(
                child.executionState || '',
              )
            ? 'waiting_for_movement'
            : child.executionState || child.submission
      rows.push({
        id: `${operation.operationID}:${child.characterID}:${child.sessionID}`,
        characterID: child.characterID,
        name: child.characterName,
        status,
        detail:
          child.skipReason?.message || child.message || child.argsSummary || '',
        geometryCount: 0,
        group: navigationTrayGroup(status),
      })
    }
  }
  return rows
    .filter((row) => !dismissed.has(row.id))
    .sort((a, b) => a.name.localeCompare(b.name) || a.id.localeCompare(b.id))
}
