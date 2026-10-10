import assert from 'node:assert/strict'
import test from 'node:test'
import {
  formatCompactObservedGold,
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
