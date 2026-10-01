export type ActionTargetGroupState = 'unchecked' | 'indeterminate' | 'checked'

export function toggleActionTarget(
  targets: ReadonlySet<string>,
  characterID: string,
) {
  const next = new Set(targets)
  if (next.has(characterID)) next.delete(characterID)
  else next.add(characterID)
  return next
}

export function selectAllActionTargets(applicableIDs: Iterable<string>) {
  return new Set(applicableIDs)
}

export function clearActionTargets() {
  return new Set<string>()
}

export function applyActionTargetGroup(
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

export function actionTargetGroupState(
  targets: ReadonlySet<string>,
  applicableGroupIDs: Iterable<string>,
): ActionTargetGroupState {
  const members = [...new Set(applicableGroupIDs)]
  if (!members.length) return 'unchecked'
  const selectedCount = members.filter((id) => targets.has(id)).length
  if (!selectedCount) return 'unchecked'
  return selectedCount === members.length ? 'checked' : 'indeterminate'
}

export function reconcileActionTargets(
  targets: ReadonlySet<string>,
  applicableIDs: Iterable<string>,
  snapshotIsCurrent: boolean,
) {
  if (!snapshotIsCurrent) return new Set(targets)
  const applicable = new Set(applicableIDs)
  return new Set([...targets].filter((id) => applicable.has(id)))
}
