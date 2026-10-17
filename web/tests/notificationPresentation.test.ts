import assert from 'node:assert/strict'
import test from 'node:test'
import type { NotificationEvent } from '../shared/types/live.ts'
import { notificationPresentation } from '../app/utils/notificationPresentation.ts'

const kinds = [
  'session.connected',
  'session.disconnected',
  'session.joined_game',
  'session.teleported',
  'character.died',
  'character.level_up',
  'character.attacked',
  'drop.item',
  'drop.rare',
  'world.unique_spawned',
  'world.gm_spawned',
  'job.hunter_trader_seen',
  'job.thief_seen',
  'pet.transport_died',
  'pet.summoned',
  'pet.dismissed',
  'alchemy.finished',
  'alchemy.attempt',
  'chat.message_received',
  'party.member_joined',
  'party.member_left',
  'academy.member_joined',
  'academy.member_left',
  'academy.member_graduated',
  'item.acquired',
  'item.transferred',
  'item.quantity_increased',
  'item.quantity_decreased',
  'trade.completed',
  'trade.failed',
]

function event(
  kind: string,
  payload: Record<string, unknown> = {},
): NotificationEvent {
  return {
    event_id: 'event-1',
    kind,
    category: kind.split('.')[0]!,
    server: 'Ares',
    character_id: 'character-1',
    character: 'Observer',
    occurred_at: '2026-10-16T10:00:00Z',
    payload,
  }
}

test('every selectable kind has a readable missing-data fallback and exact record link', () => {
  assert.equal(kinds.length, 30)
  for (const kind of kinds) {
    const result = notificationPresentation(event(kind))
    assert.ok(result.title.length >= 3, kind)
    assert.ok(
      ['urgent', 'highlight', 'activity', 'routine'].includes(result.tone),
      kind,
    )
    assert.equal(
      result.to.query?.server,
      kind === 'chat.message_received' ? undefined : 'Ares',
      kind,
    )
    if (kind !== 'chat.message_received') {
      assert.equal(
        result.to.query?.[kind.startsWith('trade.') ? 'trade_id' : 'event_id'],
        'event-1',
        kind,
      )
    }
  }
})

test('trader, hunter and thief sightings include the location and its source', () => {
  const reporter = {
    source: 'reporter' as const,
    zone: 'Jangan',
    x: 98.14,
    y: 1551.06,
  }
  assert.equal(
    notificationPresentation({
      ...event('job.hunter_trader_seen', { value: 'TraderOne' }),
      location: reporter,
    }).body,
    'TraderOne · Reporter location: Jangan · 98.1, 1551.1',
  )
  assert.equal(
    notificationPresentation({
      ...event('job.thief_seen', { value: 'Pickpocket' }),
      location: reporter,
    }).body,
    'Pickpocket · Reporter location: Jangan · 98.1, 1551.1',
  )
  assert.equal(
    notificationPresentation({
      ...event('job.thief_seen', { value: 'Pickpocket' }),
      location: { source: 'thief', zone: 'Donwhang', x: 10, y: 20 },
    }).body,
    'Pickpocket · Donwhang · 10.0, 20.0',
  )
  assert.equal(
    notificationPresentation({
      ...event('job.thief_seen', { value: 'Pickpocket' }),
      location: { source: 'thief', x: 10, y: 20 },
    }).body,
    'Pickpocket · Unknown zone · 10.0, 20.0',
  )
  assert.equal(
    notificationPresentation({
      ...event('job.thief_seen', { value: 'Pickpocket' }),
      location: { source: 'reporter', zone: 'Jangan' },
    }).body,
    'Pickpocket · Reporter location: Jangan',
  )
  assert.equal(
    notificationPresentation(event('job.thief_seen', { value: 'Pickpocket' }))
      .body,
    'Pickpocket',
  )
})

test('unique kill, spotted location, member names and trade outcomes use observed details', () => {
  const unique = notificationPresentation(
    event('world.unique_spawned', {
      notice: 'kill',
      value: 'Tiger Girl',
      killer: 'Alpha',
    }),
  )
  assert.equal(unique.title, 'Unique killed')
  assert.match(unique.body, /Tiger Girl.*Alpha/)
  const hunter = notificationPresentation(
    event('job.hunter_trader_seen', { value: 'TraderOne' }),
  )
  assert.equal(hunter.body, 'TraderOne · Location unknown')
  assert.equal(
    notificationPresentation(
      event('party.member_joined', { member: { name: 'Beta' } }),
    ).body,
    'Beta',
  )
  const trade = notificationPresentation(
    event('trade.failed', {
      route_from: 'Jangan',
      route_to: 'Donwhang',
      reason: 'thief',
    }),
  )
  assert.equal(trade.body, 'Jangan → Donwhang · thief')
  assert.equal(trade.to.query?.view, 'trades')
})
