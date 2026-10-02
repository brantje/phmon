import type { FanOutOperation } from './commandFanOut'

export interface MapActionNotification {
  message: string
  tone: 'success' | 'warning'
}

function characters(count: number) {
  return `${count} character${count === 1 ? '' : 's'}`
}

/** Announce command outcomes, never infer arrival or success from admission. */
export function mapActionNotification(
  operation: FanOutOperation,
): MapActionNotification | null {
  if (
    ['prepared', 'cancelled'].includes(operation.state) ||
    !operation.children.length ||
    operation.children.some((child) =>
      ['ready', 'submitting'].includes(child.submission),
    )
  )
    return null

  const completed = operation.children.filter(
    (child) =>
      child.submission === 'accepted' &&
      child.executionState === 'completed' &&
      child.apiReturn !== false,
  )
  const skipped = operation.children.filter(
    (child) => child.submission === 'skipped',
  ).length
  const failed = operation.children.filter(
    (child) =>
      child.submission === 'rejected' ||
      (child.submission === 'accepted' &&
        (['failed', 'expired'].includes(child.executionState || '') ||
          (child.executionState === 'completed' && child.apiReturn === false))),
  ).length
  const unknown = operation.children.filter(
    (child) => child.submission === 'uncertain',
  ).length
  const pending =
    operation.children.length - completed.length - skipped - failed - unknown
  if (pending && !unknown) return null

  const count = completed.length
  const names = completed.map((child) => child.characterName).join(', ')
  let message: string
  switch (operation.command.name) {
    case 'bot.start':
      message = `Bot started for ${characters(count)}`
      break
    case 'bot.stop':
      message = `Bot stopped for ${characters(count)}`
      break
    case 'character.return':
      message = `Return scroll used by ${characters(count)}`
      break
    case 'character.disconnect':
      message = `Disconnected ${names}`
      break
    case 'trace.start':
      message = `${characters(count)} tracing ${String(completed[0]?.args?.name || '')}`
      break
    case 'trace.stop':
      message = `Trace stopped for ${characters(count)}`
      break
    case 'character.navigate':
      message = `Navigation sent to ${characters(count)}`
      break
    case 'character.teleport':
      message = `Teleport script sent to ${characters(count)}`
      break
    case 'training.area.set':
      message = `Training area set for ${characters(count)}`
      break
    case 'training.radius.set':
      message = `Training radius set for ${characters(count)}`
      break
    default:
      message = `${operation.command.label} completed for ${characters(count)}`
  }
  if (!count) message = operation.command.label
  const details = [
    failed ? `${failed} failed` : '',
    skipped ? `${skipped} skipped` : '',
    unknown ? `${unknown} outcome${unknown === 1 ? '' : 's'} unknown` : '',
    pending ? `${pending} pending` : '',
  ].filter(Boolean)
  return {
    message: [message, ...details].join(' · '),
    tone: failed || unknown || skipped || pending ? 'warning' : 'success',
  }
}

/** Live snapshots replace operation arrays; identical outcomes must not replay. */
export function collectMapActionNotifications(
  operations: FanOutOperation[],
  announced: Map<string, string>,
) {
  const notifications: MapActionNotification[] = []
  const activeIDs = new Set(
    operations.map((operation) => operation.operationID),
  )
  for (const id of announced.keys()) {
    if (!activeIDs.has(id)) announced.delete(id)
  }
  for (const operation of [...operations].reverse()) {
    const notification = mapActionNotification(operation)
    if (!notification) continue
    const signature = JSON.stringify(notification)
    if (announced.get(operation.operationID) === signature) continue
    announced.set(operation.operationID, signature)
    notifications.push(notification)
  }
  return notifications
}
