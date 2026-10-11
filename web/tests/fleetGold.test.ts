import assert from 'node:assert/strict'
import test from 'node:test'
import {
  formatCompactObservedGold,
  characterGoldTotal,
  formatCharacterGoldTotal,
  characterGoldStatus,
  sumObservedFleetGold,
} from '../app/utils/fleetGold.ts'
import type {
  CharacterView,
  GuildStorageGoldEntry,
} from '../shared/types/live.ts'

const characters: CharacterView[] = [
  {
    character_id: 'a',
    server: 'Greatest',
    name: 'one',
    online: true,
    gold: 1000,
  },
  {
    character_id: 'b',
    server: 'Greatest',
    name: 'two',
    online: true,
    gold: 500,
  },
  {
    character_id: 'c',
    server: 'Other',
    name: 'three',
    online: true,
    gold: 9999,
  },
]

const guildGold: GuildStorageGoldEntry[] = [
  { server: 'Greatest', guild: 'GuildA', gold: 4000 },
  { server: 'Greatest', guild: 'GuildB', gold: 2500 },
  { server: 'Other', guild: 'Remote', gold: 777 },
]

test('sumObservedFleetGold includes character and guild gold within server scope', () => {
  const matchesGreatest = (server: string) =>
    server.toLocaleLowerCase() === 'greatest'
  assert.equal(
    sumObservedFleetGold(characters, guildGold, matchesGreatest),
    8000,
  )
})

test('sumObservedFleetGold includes every server when scope is all', () => {
  assert.equal(
    sumObservedFleetGold(characters, guildGold, () => true),
    18776,
  )
})

test('sumObservedFleetGold returns null when no gold is available in scope', () => {
  assert.equal(
    sumObservedFleetGold(characters, guildGold, () => false),
    null,
  )
})

test('formatCompactObservedGold formats totals with compact notation', () => {
  assert.equal(formatCompactObservedGold(8000), '8K')
  assert.equal(formatCompactObservedGold(null), '—')
})

test('bank gold contributes once per character while guild gold stays separate', () => {
  const bankCharacters = characters.map((character) => ({
    ...character,
    storage_gold: 4000,
    storage_gold_availability: 'observed',
  }))
  assert.equal(
    sumObservedFleetGold(
      bankCharacters,
      guildGold,
      (server) => server === 'Greatest',
    ),
    16000,
  )
  assert.equal(
    sumObservedFleetGold(bankCharacters, guildGold, () => true),
    30776,
  )
  assert.equal(
    sumObservedFleetGold(
      bankCharacters,
      guildGold,
      (server) => server === 'Other',
    ),
    14776,
  )
  assert.equal(characterGoldTotal(bankCharacters[0]!), 5000)
  assert.equal(
    characterGoldTotal({ ...characters[0]!, gold: 500, storage_gold: 4500 }),
    5000,
  )
  assert.equal(characterGoldStatus(bankCharacters), '')
})

test('unknown and stale bank balances are explicit without hiding carried gold', () => {
  assert.equal(formatCharacterGoldTotal([characters[0]!]), '1,000 · incomplete')
  assert.equal(
    characterGoldStatus([
      {
        ...characters[0]!,
        storage_gold: 0,
        storage_gold_availability: 'observed',
      },
    ]),
    '',
  )
  assert.equal(
    characterGoldStatus([
      {
        ...characters[0]!,
        storage_gold: 4000,
        storage_gold_availability: 'stale',
      },
    ]),
    'last known bank',
  )
  assert.equal(
    characterGoldStatus([
      {
        ...characters[0]!,
        online: false,
        storage_gold: 4000,
        storage_gold_availability: 'observed',
      },
    ]),
    'last known bank',
  )
  assert.equal(
    characterGoldTotal({
      ...characters[0]!,
      gold: undefined,
      storage_gold: undefined,
    }),
    null,
  )
  assert.equal(
    characterGoldTotal({ ...characters[0]!, gold: undefined, storage_gold: 0 }),
    0,
  )
  assert.equal(
    characterGoldTotal({ ...characters[0]!, storage_gold: -5 }),
    1000,
  )
  assert.equal(
    characterGoldTotal({ ...characters[0]!, storage_gold: NaN }),
    1000,
  )
})
