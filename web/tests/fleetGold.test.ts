import assert from 'node:assert/strict'
import test from 'node:test'
import {
  combinedCharacterGold,
  formatCompactObservedGold,
  personalStorageGold,
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
    gold: 100_000_000,
    personal_storage_gold: 900_000_000,
  },
  {
    character_id: 'b',
    server: 'Greatest',
    name: 'two',
    online: true,
    gold: 500,
    personal_storage_gold: 0,
  },
  {
    character_id: 'c',
    server: 'Other',
    name: 'three',
    online: true,
    gold: 9999,
  },
  {
    character_id: 'd',
    server: 'Greatest',
    name: 'four',
    online: true,
    gold: 1000,
  },
]

const guildGold: GuildStorageGoldEntry[] = [
  { server: 'Greatest', guild: 'GuildA', gold: 4000 },
  { server: 'Greatest', guild: 'GuildB', gold: 2500 },
  { server: 'Other', guild: 'Remote', gold: 777 },
]

test('combinedCharacterGold adds inventory and personal storage', () => {
  assert.equal(combinedCharacterGold(characters[0]), 1_000_000_000)
})

test('combinedCharacterGold treats zero storage as observed', () => {
  assert.equal(combinedCharacterGold(characters[1]), 500)
})

test('combinedCharacterGold leaves inventory-only when storage is unknown', () => {
  assert.equal(combinedCharacterGold(characters[2]), 9999)
  assert.equal(personalStorageGold(characters[2]), null)
})

test('sumObservedFleetGold includes combined character and distinct guild gold', () => {
  const matchesGreatest = (server: string) =>
    server.toLocaleLowerCase() === 'greatest'
  assert.equal(
    sumObservedFleetGold(characters, guildGold, matchesGreatest),
    1_000_008_000,
  )
})

test('sumObservedFleetGold does not count guild gold per character', () => {
  const matchesGreatest = (server: string) =>
    server.toLocaleLowerCase() === 'greatest'
  const total = sumObservedFleetGold(characters, guildGold, matchesGreatest)
  assert.ok(total !== null)
  const withoutGuild = 1_000_000_000 + 500 + 1000
  assert.equal(total - withoutGuild, 6500)
})

test('sumObservedFleetGold includes every server when scope is all', () => {
  assert.equal(
    sumObservedFleetGold(characters, guildGold, () => true),
    1_000_000_000 + 500 + 9999 + 1000 + 4000 + 2500 + 777,
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
