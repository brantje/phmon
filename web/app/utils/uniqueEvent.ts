import type { ActivityEvent } from '~~/shared/types/live'

export interface UniqueEventDetails {
  name: string
  level?: number
  imageUrl?: string
  notice: 'spawn' | 'kill'
  killer?: string
}

// Kinds the dashboard's Recent Events card never shows. Hunters arrive through
// their own hide-own feed instead.
export const DASHBOARD_NOISY_KINDS: readonly string[] = [
  'item.quantity_increased',
  'item.quantity_decreased',
  'item.acquired',
  'item.transferred',
  'pet.transport_died',
  'job.hunter_trader_seen',
  'chat.message_received',
  'session.connected',
  'session.disconnected',
  'session.joined_game',
  'session.teleported',
]

export function uniqueEventDetails(
  event: ActivityEvent,
): UniqueEventDetails | null {
  if (event.kind !== 'world.unique_spawned') return null
  const enriched = event.unique
  const payload = event.payload || {}
  const noticeValue = enriched?.notice || payload.notice
  const notice = noticeValue === 'kill' ? 'kill' : 'spawn'
  const killer =
    enriched?.killer ||
    (typeof payload.killer === 'string' ? payload.killer : '')
  const name =
    enriched?.name ||
    (typeof payload.value === 'string' ? payload.value.trim() : '')
  const level = enriched?.level
  return {
    name: name || 'Unique',
    level: typeof level === 'number' ? level : undefined,
    imageUrl: enriched?.image_url || undefined,
    notice,
    killer: killer || undefined,
  }
}

export function uniqueEventHeadline(event: ActivityEvent) {
  const details = uniqueEventDetails(event)
  if (!details) return 'Unique spawned'
  const level = details.level ? ` Lv ${details.level}` : ''
  if (details.notice === 'kill') {
    return `${details.name}${level} was killed${details.killer ? ` by ${details.killer}` : ''}`
  }
  return `${details.name}${level} spawned`
}
