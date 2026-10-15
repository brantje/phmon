import assert from 'node:assert/strict'
import test from 'node:test'
import {
  createLatestRequestGate,
  economySearchQuery,
  itemHistoryTarget,
  itemOwnerHref,
  itemSearchQuery,
} from '../app/utils/searchPage.ts'

test('Item Search scopes every page to the selected server and exact filters', () => {
  const base = {
    server: 'Greatest',
    query: 'ITEM_CH_SWORD',
    container: 'guild_storage',
    owner: '  MyGuild  ',
    itemType: 'Weapon',
    subcategory: 'Sword',
    degree: '10',
    includeSets: false,
    cursor: 'opaque-page-two',
  }
  assert.deepEqual(itemSearchQuery(base), {
    server: 'Greatest',
    q: 'ITEM_CH_SWORD',
    container: 'guild_storage',
    owner: 'MyGuild',
    item_type: 'Weapon',
    subcategory: 'Sword',
    degree: '10',
    include_sets: 'false',
    cursor: 'opaque-page-two',
    limit: 30,
  })
  const allServers = itemSearchQuery({
    ...base,
    server: 'all',
    cursor: '',
    query: '',
  })
  assert.deepEqual(allServers, {
    server: undefined,
    q: undefined,
    container: 'guild_storage',
    owner: 'MyGuild',
    item_type: 'Weapon',
    subcategory: 'Sword',
    degree: '10',
    include_sets: 'false',
    cursor: undefined,
    limit: 30,
  })
})

test('owner links preserve character, pet and guild context', () => {
  assert.equal(
    itemOwnerHref({ owner_kind: 'guild', observer_character_id: 'c1' }),
    '/guild-storage',
  )
  assert.equal(
    itemOwnerHref({ owner_kind: 'pet', observer_character_id: 'c2' }),
    '/characters/c2',
  )
  assert.equal(
    itemOwnerHref({ owner_kind: 'character', observer_character_id: 'c3' }),
    '/characters/c3',
  )
})

test('Economy offers preserve sender, date scope, offer type and pagination', () => {
  const base = {
    server: 'Greatest',
    type: 'WTS',
    sender: '  trader  ',
    query: 'Seal of Sun',
    from: '2026-10-01',
    to: '2026-10-15',
    cursor: 'opaque-economy-page-two',
  }
  assert.deepEqual(economySearchQuery(base), {
    server: 'Greatest',
    offer_type: 'WTS',
    sender: 'trader',
    q: 'Seal of Sun',
    from: '2026-10-01',
    to: '2026-10-15',
    cursor: 'opaque-economy-page-two',
    limit: 30,
  })
  assert.equal(economySearchQuery({ ...base, server: 'all' }).server, undefined)
})

test('a stale filter request cannot replace the current page', () => {
  const gate = createLatestRequestGate()
  const first = gate.next()
  assert.equal(gate.isCurrent(first), true)
  const filtered = gate.next()
  assert.equal(gate.isCurrent(first), false)
  assert.equal(gate.isCurrent(filtered), true)
  const nextPage = gate.next()
  assert.equal(gate.isCurrent(filtered), false)
  assert.equal(gate.isCurrent(nextPage), true)
  gate.invalidate()
  assert.equal(gate.isCurrent(nextPage), false)
})

test('history links preserve server and item identity', () => {
  const observed = itemHistoryTarget({
    server: 'Greatest',
    item: { model: 777, servername: 'ITEM_CH_SWORD_10', name: 'Sword' },
  })
  assert.deepEqual(observed, {
    path: '/events',
    query: { server: 'Greatest', item: 'ITEM_CH_SWORD_10' },
  })
  assert.deepEqual(
    itemHistoryTarget({ server: 'Other', item: { model: 777 } }),
    { path: '/events', query: { server: 'Other', item: '777' } },
  )
})
