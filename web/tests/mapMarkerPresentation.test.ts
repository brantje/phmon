import assert from 'node:assert/strict'
import test from 'node:test'
import {
  DEFAULT_SHOW_NEARBY_MONSTER_NAMES,
  dedupeCurrentMonsters,
  localMapAsset,
  monsterDisplayName,
  monsterHPBarFraction,
  monsterHPFraction,
  monsterMapKey,
  monsterMapName,
  monsterTypePresentation,
} from '../app/utils/mapMarkerPresentation.ts'
import type { MapMonster, MapMonsterObservation } from '../shared/types/live.ts'

function observation(
  session: string,
  at: string,
  monsters: MapMonster[],
  server = 'Greatest',
): MapMonsterObservation {
  return {
    server,
    agent_id: session,
    character_id: session,
    session_id: session,
    character: session,
    status: 'observed',
    observed_at: at,
    monsters,
  }
}

const fixtureMonster: MapMonster = {
  id: '42',
  model_id: 1933,
  name: 'Mangyang',
  type_code: 0,
  region: 25735,
  x: 1,
  y: 2,
  hp: 20,
  max_hp: 100,
}

test('reference monster ranks retain scale and resolve the committed icon artwork', () => {
  for (const [code, label, scale, party, iconUrl, partyBadgeUrl] of [
    [0, 'General', 1, false, '/game-assets/monster-types/0_general.png', ''],
    [
      1,
      'Champion',
      1.2,
      false,
      '/game-assets/monster-types/1_champion.png',
      '',
    ],
    [4, 'Giant', 1.5, false, '/game-assets/monster-types/4_giant.png', ''],
    [
      16,
      'General (Party)',
      1,
      true,
      '/game-assets/monster-types/0_general.png',
      '/game-assets/monster-types/16_party_general.png',
    ],
    [
      17,
      'Champion (Party)',
      1.2,
      true,
      '/game-assets/monster-types/1_champion.png',
      '/game-assets/monster-types/17_party_champion.png',
    ],
    [
      20,
      'Giant (Party)',
      1.5,
      true,
      '/game-assets/monster-types/4_giant.png',
      '/game-assets/monster-types/20_party_giant.png',
    ],
  ] as const) {
    assert.deepEqual(monsterTypePresentation({ type_code: code }), {
      code,
      label,
      scale,
      party,
      iconUrl,
      partyBadgeUrl,
      unknown: false,
    })
  }
  assert.deepEqual(monsterTypePresentation({ type: '27' }), {
    code: 27,
    label: 'Unknown (27)',
    scale: 1,
    party: false,
    iconUrl: '',
    partyBadgeUrl: '',
    unknown: true,
  })
  assert.equal(monsterHPFraction({ hp: 7515, max_hp: 9000 }), 7515 / 9000)
  assert.equal(monsterHPFraction({}), null)
})
test('current monster observers merge matching IDs and preserve distinct nearby IDs', () => {
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
        { id: '42', model_id: 1933, region: 25735, x: 3, y: 4, hp: 10 },
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
  assert.equal(distinct.length, 5)

  const differentRanks = dedupeCurrentMonsters([
    {
      ...common,
      observed_at: '2026-09-29T08:00:01Z',
      monsters: [
        {
          id: '100',
          model_id: 2450,
          name: 'Shakram',
          type_code: 0,
          region: 25735,
          x: 72,
          y: 1565,
        },
      ],
    },
    {
      ...common,
      session_id: 's2',
      observed_at: '2026-09-29T08:00:02Z',
      monsters: [
        {
          id: '101',
          model_id: 2450,
          name: 'Shakram',
          type_code: 16,
          region: 25735,
          x: 72,
          y: 1565,
        },
      ],
    },
  ])
  assert.equal(differentRanks.length, 2)
})

test('nearby identical mobs do not exchange HP across observers', () => {
  const snapshots = [
    observation('a', '2026-10-02T12:00:00Z', [fixtureMonster]),
    observation('b', '2026-10-02T12:00:01Z', [
      { ...fixtureMonster, id: '77', x: 2, hp: 100 },
    ]),
  ]
  const before = JSON.stringify(snapshots)
  const monsters = dedupeCurrentMonsters(snapshots)
  assert.equal(monsters.length, 2)
  assert.deepEqual(
    monsters.map((monster) => [monster.id, monster.hp]),
    [
      ['42', 20],
      ['77', 100],
    ],
  )
  assert.equal(JSON.stringify(snapshots), before)
})

test('moving mob keeps one identity across observers and uses the newest complete row', () => {
  const previous = fixtureMonster
  const latest = { ...fixtureMonster, x: 50, hp: 15, max_hp: 150 }
  const older = observation('a', '2026-10-02T12:00:00Z', [previous])
  const newer = observation('b', '2026-10-02T12:00:01Z', [latest])
  for (const snapshots of [
    [older, newer],
    [newer, older],
  ]) {
    const monsters = dedupeCurrentMonsters(snapshots)
    assert.equal(monsters.length, 1)
    assert.equal(monsters[0]?.observer.session_id, 'b')
    assert.equal(monsters[0]?.x, 50)
    assert.equal(monsters[0]?.hp, 15)
    assert.equal(monsters[0]?.max_hp, 150)
    assert.equal(
      monsterMapKey(monsters[0]!, newer.server),
      monsterMapKey(previous, older.server),
    )
  }
})

test('monster identity is server scoped and independent of region, name, rank and position', () => {
  const changed = {
    ...fixtureMonster,
    id: '00042',
    region: 25736,
    x: 50,
    type_code: 16,
    name: 'Changed name',
  }
  assert.equal(
    monsterMapKey(fixtureMonster, 'Greatest'),
    monsterMapKey(changed, ' greatest '),
  )
  assert.notEqual(
    monsterMapKey(fixtureMonster, 'Greatest'),
    monsterMapKey(fixtureMonster, 'Other server'),
  )
  const unidentified = { ...fixtureMonster, id: '0' }
  assert.equal(
    monsterMapKey(unidentified, 'Greatest'),
    monsterMapKey({ ...unidentified, x: 1.1 }, 'Greatest'),
  )
  assert.notEqual(
    monsterMapKey(unidentified, 'Greatest'),
    monsterMapKey({ ...unidentified, x: 2 }, 'Greatest'),
  )
})

test('equal timestamp observer selection is stable and unavailable snapshots do not win', () => {
  const a = observation('a', '2026-10-02T12:00:00Z', [fixtureMonster])
  const b = observation('b', '2026-10-02T12:00:00.000Z', [
    { ...fixtureMonster, hp: 100 },
  ])
  for (const snapshots of [
    [a, b],
    [b, a],
  ]) {
    assert.equal(dedupeCurrentMonsters(snapshots)[0]?.hp, 20)
  }
  assert.equal(
    dedupeCurrentMonsters([a, { ...b, status: 'unavailable' }])[0]?.hp,
    20,
  )
  assert.equal(dedupeCurrentMonsters([{ ...a, monsters: [] }]).length, 0)
})

test('HP follows observed decreases, healing and respawn instead of retaining a floor', () => {
  for (const hp of [100, 40, 20, 60, 0, 100]) {
    const [monster] = dedupeCurrentMonsters([
      observation('a', '2026-10-02T12:00:00Z', [{ ...fixtureMonster, hp }]),
    ])
    assert.equal(monsterHPFraction(monster!), hp / 100)
  }
  const [updated] = dedupeCurrentMonsters([
    observation('a', '2026-10-02T12:00:00Z', [fixtureMonster]),
    observation('b', '2026-10-02T12:00:01Z', [{ ...fixtureMonster, hp: 100 }]),
  ])
  assert.equal(updated?.hp, 100)
})

test('HP ring and popup bar retain the reference zero-maximum behavior and clamp bounds', () => {
  assert.equal(monsterHPFraction({ hp: 20, max_hp: 100 }), 0.2)
  assert.equal(monsterHPBarFraction({ hp: 20, max_hp: 100 }), 0.2)
  assert.equal(monsterHPFraction({ hp: 20, max_hp: 0 }), 1)
  assert.equal(monsterHPBarFraction({ hp: 20, max_hp: 0 }), 0)
  assert.equal(monsterHPFraction({ hp: 20 }), 1)
  assert.equal(monsterHPFraction({ hp: 0, max_hp: 0 }), 0)
  assert.equal(monsterHPFraction({ hp: -20, max_hp: 100 }), 0)
  assert.equal(monsterHPFraction({ hp: 200, max_hp: 100 }), 1)
  assert.equal(monsterHPFraction({ hp: NaN, max_hp: 100 }), null)
  assert.equal(monsterHPFraction({ hp: 20, max_hp: Infinity }), null)
  assert.equal(monsterHPFraction({ max_hp: 100 }), null)
  assert.equal(monsterHPBarFraction({}), null)
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

test('nearby monster map names are opt-in and use the resolved monster name', () => {
  const monster = {
    id: '1',
    name: 'Eldimmu',
    region: 1,
    x: 0,
    y: 0,
  }
  assert.equal(DEFAULT_SHOW_NEARBY_MONSTER_NAMES, false)
  assert.equal(monsterMapName(monster, false), '')
  assert.equal(monsterMapName(monster, true), 'Eldimmu')
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
