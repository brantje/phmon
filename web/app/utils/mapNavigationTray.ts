import type { NavigationRoute } from '../../shared/types/live'
import type { FanOutOperation } from './commandFanOut.ts'
import type { MapRouteOverlay } from './mapNavigationRoutes.ts'
const NAVIGATION_STOP_ELIGIBLE = new Set([
  'moving',
  'waiting_for_arrival',
  'transition_awaiting_evidence',
  'progress_uncertain',
])

export interface MapNavigationTrayRow {
  id: string
  characterID: string
  sessionID?: string
  commandID?: string
  routeSequence?: number
  name: string
  status: string
  detail: string
  geometryCount: number
  group: 'active' | 'done' | 'attention'
  progress?: number
  etaSeconds?: number
  canStop?: boolean
}

export function navigationTrayProgressSummary(
  rows: MapNavigationTrayRow[],
): string | null {
  const active = rows.filter((row) => row.group === 'active')
  const withProgress = active.filter(
    (row) => typeof row.progress === 'number' && Number.isFinite(row.progress),
  )
  if (!withProgress.length) return null
  const average =
    withProgress.reduce((sum, row) => sum + row.progress!, 0) /
    withProgress.length
  const excluded = active.length - withProgress.length
  const percent = Math.round(average * 100)
  const suffix =
    excluded > 0
      ? ` · ${excluded} active route${excluded === 1 ? '' : 's'} without observed steps`
      : ''
  return `~${percent}% observed steps (approximate)${suffix}`
}

function routeFields(
  route: NavigationRoute | undefined,
  stopSupported: boolean,
): Pick<
  MapNavigationTrayRow,
  | 'sessionID'
  | 'commandID'
  | 'routeSequence'
  | 'progress'
  | 'etaSeconds'
  | 'canStop'
> {
  if (!route) return {}
  return {
    sessionID: route.session_id,
    commandID: route.command_id,
    routeSequence: route.route_sequence,
    progress: route.progress,
    etaSeconds: route.eta_seconds,
    canStop:
      stopSupported &&
      NAVIGATION_STOP_ELIGIBLE.has(route.status) &&
      Boolean(route.command_id && route.route_sequence),
  }
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
  navigationRoutes?: NavigationRoute[],
  stopSupportedByCharacter?: Readonly<Record<string, boolean>>,
): MapNavigationTrayRow[] {
  const routeByID = new Map(
    (navigationRoutes || []).map((route) => [
      `${route.character_id}:${route.session_id}:${route.route_sequence}`,
      route,
    ]),
  )
  const rows: MapNavigationTrayRow[] = routes.map((route) => {
    const backend = routeByID.get(route.id)
    const stopSupported = stopSupportedByCharacter?.[route.characterID] ?? false
    return {
      id: route.id,
      characterID: route.characterID,
      name: route.characterName,
      status: route.status,
      detail: route.reason || '',
      geometryCount: route.blocks.length,
      group: navigationTrayGroup(route.status),
      ...routeFields(backend, stopSupported),
    }
  })
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
            Boolean(child.commandID) &&
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
