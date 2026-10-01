import {
  actionTargetGroupState,
  applyActionTargetGroup,
  clearActionTargets,
  reconcileActionTargets,
  selectAllActionTargets,
  toggleActionTarget,
  type ActionTargetGroupState,
} from './actionTargets.ts'

export type MapActionTargetGroupState = ActionTargetGroupState

export interface MapActionTargetScope {
  server: string
  area: string
  floor: string
  region: number
}

export function mapActionTargetScopeKey(scope: MapActionTargetScope) {
  return [
    scope.server.trim().toLocaleLowerCase(),
    scope.area,
    scope.floor,
    String(scope.region),
  ].join('\u0000')
}

export const toggleMapActionTarget = toggleActionTarget

export const selectAllMapActionTargets = selectAllActionTargets

export const clearMapActionTargets = clearActionTargets

export const applyMapActionTargetGroup = applyActionTargetGroup

export const mapActionTargetGroupState = actionTargetGroupState

export const reconcileMapActionTargets = reconcileActionTargets
