export type MapActionTargetGroupState =
  'unchecked' | 'indeterminate' | 'checked'

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

export function toggleMapActionTarget(
  targets: ReadonlySet<string>,
  characterID: string,
) {
  const next = new Set(targets)
  if (next.has(characterID)) next.delete(characterID)
  else next.add(characterID)
  return next
}

export function selectAllMapActionTargets(applicableIDs: Iterable<string>) {
  return new Set(applicableIDs)
}

export function clearMapActionTargets() {
  return new Set<string>()
}

export function applyMapActionTargetGroup(
  targets: ReadonlySet<string>,
  applicableGroupIDs: Iterable<string>,
) {
  const members = new Set(applicableGroupIDs)
  const next = new Set(targets)
  if (members.size > 0 && [...members].every((id) => next.has(id))) {
    for (const id of members) next.delete(id)
  } else {
    for (const id of members) next.add(id)
  }
  return next
}

export function mapActionTargetGroupState(
  targets: ReadonlySet<string>,
  applicableGroupIDs: Iterable<string>,
): MapActionTargetGroupState {
  const members = [...new Set(applicableGroupIDs)]
  if (!members.length) return 'unchecked'
  const selectedCount = members.filter((id) => targets.has(id)).length
  if (!selectedCount) return 'unchecked'
  return selectedCount === members.length ? 'checked' : 'indeterminate'
}

export function reconcileMapActionTargets(
  targets: ReadonlySet<string>,
  applicableIDs: Iterable<string>,
  snapshotIsCurrent: boolean,
) {
  if (!snapshotIsCurrent) return new Set(targets)
  const applicable = new Set(applicableIDs)
  return new Set([...targets].filter((id) => applicable.has(id)))
}
