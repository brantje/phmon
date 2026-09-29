import assert from 'node:assert/strict'
import test from 'node:test'
import {
  dedupeCurrentMonsters,
  localMapAsset,
  monsterDisplayName,
  monsterHPFraction,
  monsterTypePresentation,
} from '../app/utils/mapMarkerPresentation.ts'

test('reference monster types retain the documented normal/champion/giant scale', () => {
  for (const [code, label, scale, party] of [
    [0, 'General', 1, false],
    [1, 'Champion', 1.2, false],
    [4, 'Giant', 1.5, false],
    [16, 'Party General', 1, true],
    [17, 'Party Champion', 1.2, true],
    [20, 'Party Giant', 1.5, true],
  ] as const) {
    assert.deepEqual(monsterTypePresentation({ type_code: code }), {
      code,
      label,
      scale,
      party,
      unknown: false,
    })
  }
  assert.equal(monsterTypePresentation({ type: '27' }).label, 'Unknown (27)')
  assert.equal(monsterHPFraction({ hp: 7515, max_hp: 9000 }), 7515 / 9000)
  assert.equal(monsterHPFraction({}), null)
})

test('current monster observers collapse cross-character sightings without merging nearby mobs', () => {
  const common = {
    server: 'Greatest',
    agent_id: 'a',
    character_id: 'c',
    session_id: 's',
    character: 'nuker1',
    status: 'observed' as const,
    region: 25735,
  }
  const monsters = dedupeCurrentMonsters([
    {
      ...common,
      observed_at: '2026-09-29T08:00:00Z',
      monsters: [
        { id: '42', model_id: 1933, region: 25735, x: 1, y: 2, hp: 20 },
      ],
    },
    {
      ...common,
      session_id: 's2',
      observed_at: '2026-09-29T08:00:01Z',
      monsters: [
        { id: '77', model_id: 1933, region: 25735, x: 3, y: 4, hp: 10 },
      ],
    },
  ])
  assert.equal(monsters.length, 1)
  assert.equal(monsters[0]?.x, 3)
  assert.equal(monsters[0]?.hp, 10)

  const distinct = dedupeCurrentMonsters([
    {
      ...common,
      observed_at: '2026-09-29T08:00:01Z',
      monsters: [
        { id: '42', model_id: 1933, region: 25735, x: 1, y: 2 },
        { id: '44', model_id: 1933, region: 25735, x: 30, y: 2 },
      ],
    },
    {
      ...common,
      session_id: 's2',
      observed_at: '2026-09-29T08:00:02Z',
      monsters: [
        { id: '42', model_id: 9999, region: 25735, x: 1, y: 2 },
        { id: '77', model_id: 1933, region: 25735, x: 2, y: 2 },
        { id: '78', model_id: 1933, region: 25735, x: 31, y: 2 },
      ],
    },
    {
      ...common,
      server: 'Other server',
      session_id: 'other-server',
      observed_at: '2026-09-29T08:00:03Z',
      monsters: [{ id: '45', model_id: 1933, region: 25735, x: 1, y: 2 }],
    },
  ])
  assert.equal(distinct.length, 4)
})

test('monster labels prefer names and prettify server names when name is numeric', () => {
  assert.equal(
    monsterDisplayName({ id: '1', name: 'Eldimmu', region: 1, x: 0, y: 0 }),
    'Eldimmu',
  )
  assert.equal(
    monsterDisplayName({
      id: '1',
      name: '16',
      servername: 'MOB_EU_ELDIMMU',
      region: 1,
      x: 0,
      y: 0,
    }),
    'Eldimmu',
  )
  assert.equal(
    monsterDisplayName({ id: '1', model_id: 1933, region: 1, x: 0, y: 0 }),
    'Unknown monster',
  )
})

test('map artwork accepts only local portrait and item assets', () => {
  assert.equal(
    localMapAsset(
      '/game-assets/interface/character/char_ch_man1.png',
      'portrait',
    ),
    '/game-assets/interface/character/char_ch_man1.png',
  )
  assert.equal(
    localMapAsset('/game-assets/icon/item/europe/weapon/sword_07.png', 'item'),
    '/game-assets/icon/item/europe/weapon/sword_07.png',
  )
  assert.equal(
    localMapAsset('https://example.com/portrait.png', 'portrait'),
    '',
  )
  assert.equal(localMapAsset('/game-assets/icon/../private.png', 'item'), '')
})
