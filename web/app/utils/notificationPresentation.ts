import type { NotificationEvent } from '~~/shared/types/live'

export type NotificationTone = 'urgent' | 'highlight' | 'activity' | 'routine'

export interface NotificationPresentation {
  title: string
  body: string
  tone: NotificationTone
  to: { path: string; query?: Record<string, string> }
}

function value(input: unknown, fallback = '') {
  return typeof input === 'string' && input.trim()
    ? input.trim().slice(0, 240)
    : fallback
}

function number(input: unknown) {
  return typeof input === 'number' && Number.isFinite(input)
    ? String(input)
    : ''
}

function itemName(event: NotificationEvent) {
  const item = event.payload.item
  const observed =
    item && typeof item === 'object' && !Array.isArray(item)
      ? (item as Record<string, unknown>)
      : {}
  return (
    event.item_name ||
    value(event.payload.item_name) ||
    value(observed.name) ||
    value(observed.servername) ||
    'Item details unavailable'
  )
}

function observedName(input: unknown) {
  if (!input || typeof input !== 'object' || Array.isArray(input)) return ''
  return value((input as Record<string, unknown>).name)
}

/**
 * Zone and coordinates of a sighting. The callback does not report where a
 * trader or hunter stands, so unless the server saw the spotted player's own
 * position the text says it is the reporting character's location.
 */
function sightingLocation(event: NotificationEvent) {
  const location = event.location
  if (!location) return ''
  const zone = value(location.zone)
  const coordinates =
    typeof location.x === 'number' && typeof location.y === 'number'
      ? `${location.x.toFixed(1)}, ${location.y.toFixed(1)}`
      : ''
  const place = [zone || (coordinates ? 'Unknown zone' : ''), coordinates]
    .filter(Boolean)
    .join(' · ')
  if (!place) return ''
  return location.source === 'thief' ? place : `Reporter location: ${place}`
}

export function notificationPresentation(
  event: NotificationEvent,
): NotificationPresentation {
  const p = event.payload
  const named = value(p.value, 'Name unavailable')
  const item = itemName(event)
  let title = event.kind.replaceAll('.', ' ')
  let body = ''
  let tone: NotificationTone = 'routine'
  switch (event.kind) {
    case 'session.connected':
      title = 'Connected'
      break
    case 'session.disconnected':
      title = 'Disconnected'
      break
    case 'session.joined_game':
      title = 'Joined game'
      break
    case 'session.teleported':
      title = 'Teleported'
      break
    case 'character.died':
      title = 'Character died'
      body =
        p.reason_type === 'attacker'
          ? `Recent attacker: ${value(p.reason_value, 'Unknown')}`
          : value(p.reason_value) ||
            value(p.cause, 'Cause unknown').replaceAll('_', ' ')
      tone = 'urgent'
      break
    case 'character.level_up':
      title = 'Level up'
      body = number(p.level)
        ? `Reached level ${number(p.level)}`
        : 'Level unavailable'
      tone = 'highlight'
      break
    case 'character.attacked':
      title = 'Attacked'
      body = `Attacker: ${named}`
      tone = 'urgent'
      break
    case 'drop.item':
      title = 'Normal drop'
      body = item
      tone = 'activity'
      break
    case 'drop.rare':
      title = 'Rare drop'
      body = item
      tone = 'highlight'
      break
    case 'world.unique_spawned': {
      const notice = value(event.unique?.notice) || value(p.notice)
      const uniqueName = value(event.unique?.name) || named
      const killer = value(event.unique?.killer) || value(p.killer)
      title = notice === 'kill' ? 'Unique killed' : 'Unique spawned'
      body = `${uniqueName}${killer && notice === 'kill' ? ` · Killed by ${killer}` : ''}`
      tone = 'highlight'
      break
    }
    case 'world.gm_spawned':
      title = 'GM spawned'
      body = named
      tone = 'highlight'
      break
    case 'job.hunter_trader_seen':
      title = 'Trader / Hunter seen'
      body = `${named} · ${sightingLocation(event) || 'Location unknown'}`
      tone = 'activity'
      break
    case 'job.thief_seen': {
      title = 'Thief seen'
      const where = sightingLocation(event)
      body = where ? `${named} · ${where}` : named
      tone = 'activity'
      break
    }
    case 'pet.transport_died':
      title = 'Transport died'
      body = named
      tone = 'urgent'
      break
    case 'pet.summoned':
      title = 'Pet summoned'
      body = observedName(p.pet) || value(p.pet_id) || named
      break
    case 'pet.dismissed':
      title = 'Pet dismissed'
      body = observedName(p.pet) || value(p.pet_id) || named
      break
    case 'alchemy.attempt':
      title = 'Alchemy attempt'
      body = `${p.success === true ? 'Success' : p.success === false ? 'Failure' : 'Result unknown'}${number(p.plus) ? ` · +${number(p.plus)}` : ''}`
      tone = p.success === false ? 'urgent' : 'highlight'
      break
    case 'alchemy.finished':
      title = 'Alchemy finished'
      tone = 'activity'
      break
    case 'chat.message_received':
      title = `${value(p.channel, 'Chat')} · ${value(p.sender, 'Sender unknown')}`
      body = value(p.message, 'Message unavailable')
      tone = 'activity'
      break
    case 'party.member_joined':
      title = 'Party member joined'
      body = observedName(p.member) || value(p.member_id) || named
      break
    case 'party.member_left':
      title = 'Party member left'
      body = observedName(p.member) || value(p.member_id) || named
      break
    case 'academy.member_joined':
      title = 'Academy member joined'
      body = observedName(p.member) || value(p.member_id) || named
      break
    case 'academy.member_left':
      title = 'Academy member left'
      body = observedName(p.member) || value(p.member_id) || named
      break
    case 'academy.member_graduated':
      title = 'Academy member graduated'
      body = observedName(p.member) || value(p.member_id) || named
      tone = 'highlight'
      break
    case 'trade.completed':
    case 'trade.failed': {
      title =
        event.kind === 'trade.completed' ? 'Trade completed' : 'Trade failed'
      const route = [value(p.route_from), value(p.route_to)].filter(Boolean)
      body =
        [route.length === 2 ? route.join(' → ') : '', value(p.reason)]
          .filter(Boolean)
          .join(' · ') || 'Trade details unavailable'
      tone = event.kind === 'trade.failed' ? 'urgent' : 'highlight'
      break
    }
    case 'item.acquired':
      title = 'Item acquired'
      body = item
      tone = 'activity'
      break
    case 'item.transferred':
      title = 'Item transferred'
      body = item
      tone = 'activity'
      break
    case 'item.quantity_increased':
      title = 'Item quantity increased'
      body = item
      break
    case 'item.quantity_decreased':
      title = 'Item quantity decreased'
      body = item
      break
  }
  const to: NotificationPresentation['to'] = {
    path: '/events',
    query: { server: event.server, event_id: event.event_id },
  }
  if (event.kind === 'chat.message_received') {
    const channel = value(p.channel)
    to.path = '/chat'
    to.query = { channel, character_id: event.character_id }
    if (channel === 'private' && value(p.sender))
      to.query.peer = value(p.sender)
  }
  if (event.kind.startsWith('trade.')) {
    to.query = {
      view: 'trades',
      server: event.server,
      trade_id: event.event_id,
    }
  }
  return { title, body, tone, to }
}
