import assert from 'node:assert/strict'
import test from 'node:test'
import {
  normalizePlayerQuery,
  safePlayerReturn,
  playerJobLabel,
  playerLabel,
  equipmentLabel,
  playerRelativeTime,
} from '../app/utils/playerRegistry.ts'

test('player filters survive URL round trip, including combined filters', () => {
  const filters = {
    server: 'Fixture',
    q: 'Alias',
    guild: 'Guild',
    min_level: '100',
    max_level: '110',
    job: 'hunter',
    seen: '7d',
    identity: 'unresolved',
    equipment: 'partial',
    sort: 'name',
    direction: 'asc',
    limit: '25',
    cursor: 'eyJ0ZXN0IjoidGVzdCJ9',
  }
  const parsed = Object.fromEntries(new URLSearchParams(filters))
  assert.deepEqual(normalizePlayerQuery(parsed), filters)
  assert.equal(normalizePlayerQuery({ server: 'all' }, 'Saved').server, 'all')
  assert.equal(normalizePlayerQuery({}, 'Saved').server, 'Saved')
})
test('invalid player query fields are excluded safely', () => {
  assert.deepEqual(
    normalizePlayerQuery({
      q: ['a', 'b'],
      job: 'fake',
      min_level: '-1',
      max_level: 'Infinity',
      cursor: '<script>',
      limit: '10000',
      equipment: 'fake',
      sort: 'sql',
      from: 'invalid',
    }),
    {},
  )
  assert.deepEqual(
    normalizePlayerQuery({ min_level: '150', max_level: '100' }),
    {},
  )
  assert.deepEqual(
    normalizePlayerQuery({ from: '2026-10-09', to: '2026-10-08' }),
    {},
  )
})
test('profile return links remain inside the player list', () => {
  assert.equal(
    safePlayerReturn('/players?server=Fixture&job=thief'),
    '/players?server=Fixture&job=thief',
  )
  for (const value of [
    'https://example.com',
    '//example.com',
    '/players/other',
    '/players\\evil',
    '/players?x=\n',
    'javascript:alert(1)',
  ])
    assert.equal(safePlayerReturn(value), '/players')
})
test('unknown roles, equipment and timestamps do not become invented values', () => {
  assert.equal(
    playerLabel({ name: null, observed_name: 'NoobTrader' }),
    'NoobTrader',
  )
  assert.equal(
    playerLabel({ name: 'Veyra', observed_name: 'NoobTrader' }),
    'Veyra',
  )
  assert.equal(playerJobLabel(null), 'Unknown')
  assert.equal(playerJobLabel('none'), 'None')
  assert.equal(equipmentLabel(null), 'Unavailable')
  assert.equal(playerRelativeTime('invalid'), 'Unknown')
  assert.equal(
    playerRelativeTime(
      '2026-10-08T00:00:00Z',
      Date.parse('2026-10-08T00:01:00Z'),
    ),
    '1m ago',
  )
})
