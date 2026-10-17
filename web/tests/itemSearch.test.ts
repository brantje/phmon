import assert from 'node:assert/strict'
import test from 'node:test'
import {
  filterSearchItems,
  groupSearchItems,
  itemSearchSlot,
  itemSearchSource,
  itemSearchTaxonomy,
  readItemSearchItems,
  type FoundItem,
} from '../app/utils/itemSearch.ts'

function row(overrides: Partial<FoundItem> = {}): FoundItem {
  return {
    server: 'Greatest',
    container: 'inventory',
    owner_kind: 'character',
    owner_id: 'char-a',
    observer_character_id: 'char-a',
    observer_name: 'nuker2',
    slot: 13,
    revision: 1,
    observed_at: '2026-10-15T18:46:00Z',
    checked_at: '2026-10-15T18:46:00Z',
    item: {
      model: 777,
      name: 'Dragon Horn Spear',
      servername: 'ITEM_CH_SPEAR_09_A',
    },
    metadata: { sort_type: 'Weapon', degree: 9 },
    ...overrides,
  }
}
const filters = {
  query: '',
  type: '',
  subcategory: '',
  degree: '',
  includeSets: false,
}

test('complete groups keep servers, guild ownership and individual pets separate', () => {
  const rows = [
    row({ slot: 45 }),
    row(),
    row({ container: 'storage', slot: 0 }),
    row({ container: 'pets', owner_kind: 'pet', pet_id: 'pet-1', slot: 0 }),
    row({ container: 'pets', owner_kind: 'pet', pet_id: 'pet-2', slot: 0 }),
    row({
      server: 'Other',
      observer_character_id: 'char-b',
      owner_id: 'char-b',
    }),
    row({
      container: 'guild_storage',
      owner_kind: 'guild',
      owner_id: 'Guild',
      guild: 'Guild',
      slot: 0,
    }),
  ]
  const groups = groupSearchItems(rows)
  assert.equal(groups.length, 3)
  const character = groups.find(
    (group) => group.name === 'nuker2' && group.server === 'Greatest',
  )!
  assert.equal(character.count, 5)
  assert.deepEqual(
    character.containers.map((container) => container.items.length),
    [2, 1, 1, 1],
  )
  assert.deepEqual(
    character.containers[0]!.items.map((item) => item.slot),
    [13, 45],
  )
  assert.equal(groups.find((group) => group.name === 'Guild')!.count, 1)
  assert.equal(
    groups.reduce((count, group) => count + group.count, 0),
    rows.length,
  )
})

test('filters use each row’s resolved metadata, even when model IDs collide across servers', () => {
  const sword = row()
  const necklace = row({
    server: 'Other',
    item: {
      model: 777,
      name: 'Gem Necklace',
      servername: 'ITEM_CH_NECKLACE_09_A',
    },
    metadata: { sort_type: 'Necklace', degree: 9 },
  })
  const equipped = row({ container: 'equipment', slot: 6 })
  const unknown = row({ metadata: {}, item: { model: 778, name: 'Unknown' } })
  const rows = [sword, necklace, equipped, unknown]
  assert.deepEqual(
    filterSearchItems(rows, {
      ...filters,
      type: 'weapons',
      subcategory: 'Spear',
      degree: '9',
    }),
    [sword],
  )
  assert.deepEqual(
    filterSearchItems(rows, { ...filters, query: 'gem', type: 'accessories' }),
    [necklace],
  )
  assert.equal(
    filterSearchItems(rows, { ...filters, includeSets: true }).length,
    4,
  )
  assert.deepEqual(filterSearchItems(rows, { ...filters, query: '778' }), [
    unknown,
  ])
  // Enriched rows carry their presentation only in item.metadata.
  const enriched = row({
    metadata: undefined,
    item: {
      model: 779,
      servername: 'ITEM_CH_RING_09_A',
      metadata: { name: 'Amber Ring', sort_type: 'Ring', degree: 9 },
    },
  })
  assert.deepEqual(
    filterSearchItems([...rows, enriched], { ...filters, query: 'amber' }),
    [enriched],
  )
  assert.deepEqual(itemSearchTaxonomy(unknown), {
    type: '',
    subcategory: '',
    degree: '',
  })
  assert.equal(
    filterSearchItems(rows, { ...filters, type: 'weapons', degree: '10' })
      .length,
    0,
  )
})

test('alchemy and silk filters use stable exported codes, without inventing missing degrees', () => {
  assert.deepEqual(
    itemSearchTaxonomy(
      row({
        item: { servername: 'ITEM_ETC_ARCHEMY_MAGICSTONE_STR_10' },
        metadata: {},
      }),
    ),
    { type: 'alchemy', subcategory: 'Magic Stones', degree: '' },
  )
  assert.equal(
    itemSearchTaxonomy(
      row({
        item: { servername: 'ITEM_MALL_REVERSE_RETURN_SCROLL' },
        metadata: {},
      }),
    ).type,
    'silk',
  )
  assert.equal(
    itemSearchTaxonomy(
      row({ item: { servername: 'UNKNOWN_ALCHEMY_LIKE_NAME' }, metadata: {} }),
    ).type,
    '',
  )
})

test('page/slot labels follow displayed bag slots while source links retain the exact container', () => {
  assert.equal(itemSearchSlot(row()), 'Page 1 | Slot 0')
  assert.equal(itemSearchSlot(row({ slot: 45 })), 'Page 2 | Slot 32')
  assert.equal(
    itemSearchSlot(row({ container: 'storage', slot: 32 })),
    'Page 2 | Slot 32',
  )
  assert.equal(
    itemSearchSlot(row({ container: 'storage', slot: 29 })),
    'Page 1 | Slot 29',
  )
  assert.equal(
    itemSearchSlot(row({ container: 'guild_storage', slot: 30 })),
    'Page 2 | Slot 30',
  )
  assert.deepEqual(itemSearchSource(row({ container: 'job_pouch' })), {
    path: '/characters/char-a',
    query: { server: 'Greatest', resource: 'job_pouch' },
  })
})

test('cursor traversal loads a whole container before deriving match counts', async () => {
  const items = Array.from({ length: 31 }, (_, index) =>
    row({ slot: 13 + index }),
  )
  const cursors: (string | undefined)[] = []
  const result = await readItemSearchItems(async (cursor) => {
    cursors.push(cursor)
    return cursor
      ? { items: items.slice(30), total: 31 }
      : { items: items.slice(0, 30), total: 31, next_cursor: 'second' }
  })
  assert.deepEqual(cursors, [undefined, 'second'])
  assert.equal(groupSearchItems(result)[0]!.containers[0]!.items.length, 31)
})

test('a container changing between pages is retried without combining revisions', async () => {
  let reads = 0
  const result = await readItemSearchItems(async () => {
    reads++
    if (reads === 1)
      return { items: [row({ revision: 1 })], total: 2, next_cursor: 'second' }
    if (reads === 2)
      return { items: [row({ slot: 14, revision: 2 })], total: 2 }
    return {
      items: [row({ revision: 3 }), row({ slot: 14, revision: 3 })],
      total: 2,
    }
  })
  assert.equal(reads, 3)
  assert.deepEqual(
    result.map((item) => item.revision),
    [3, 3],
  )
})

test('continued replacement or a broken cursor fails instead of showing incomplete groups', async () => {
  await assert.rejects(
    readItemSearchItems(async () => ({ items: [row()], total: 2 })),
    /Items changed/,
  )
  await assert.rejects(
    readItemSearchItems(async () => ({
      items: [row()],
      total: 2,
      next_cursor: 'same',
    })),
    /Repeated item cursor/,
  )
})

test('a guild observer swap between pages cannot become one mixed guild container', async () => {
  const first = row({
    container: 'guild_storage',
    owner_kind: 'guild',
    guild: 'Guild',
    owner_id: 'Guild',
  })
  const replacement = {
    ...first,
    observer_character_id: 'char-b',
    observer_name: 'nuker3',
  }
  let reads = 0
  const result = await readItemSearchItems(async () => {
    reads++
    if (reads === 1) return { items: [first], total: 2, next_cursor: 'second' }
    if (reads === 2) return { items: [{ ...replacement, slot: 14 }], total: 2 }
    return { items: [replacement, { ...replacement, slot: 14 }], total: 2 }
  })
  assert.equal(reads, 3)
  assert(result.every((item) => item.observer_character_id === 'char-b'))
})
