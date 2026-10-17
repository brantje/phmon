import {
  containerPageLayout,
  displayedContainerSlot,
} from './containerLayout.ts'

// Item Search presents the canonical API rows; grouping never merges quantities
// or substitutes another server's model metadata for an item.
export type FoundItem = {
  server: string
  container: string
  owner_kind: 'character' | 'guild' | 'pet'
  owner_id: string
  owner_name?: string
  guild?: string
  observer_character_id: string
  observer_name: string
  pet_id?: string
  slot: number
  revision: number
  observed_at: string
  checked_at: string
  item: Record<string, unknown>
  quantity_exact?: string
  metadata?: Record<string, unknown>
}
export type ItemSearchPage = {
  items: FoundItem[]
  total: number
  next_cursor?: string
}
export type ItemSearchFilters = {
  query: string
  type: string
  subcategory: string
  degree: string
  includeSets: boolean
}
export const itemSearchTypes = [
  ['weapons', 'Weapons'],
  ['shields', 'Shields'],
  ['sets', 'Set Parts'],
  ['accessories', 'Accessories'],
  ['alchemy', 'Alchemy'],
  ['silk', 'Silk Items'],
  ['consumables', 'Potions & Pills'],
] as const
const armor = new Set([
  'Garment',
  'Protector',
  'Armor',
  'Robe',
  'Light armor',
  'Heavy armor',
])
const accessories = new Set(['Earring', 'Necklace', 'Ring'])
const weaponNames: Record<string, string> = {
  SWORD: 'Sword',
  BLADE: 'Blade',
  SPEAR: 'Spear',
  TBLADE: 'Glaive',
  BOW: 'Bow',
  TSWORD: 'Two-handed Sword',
  AXE: 'Axe',
  CROSSBOW: 'Crossbow',
  DAGGER: 'Dagger',
  STAFF: 'Cleric Rod',
  TSTAFF: 'Staff',
  DARKSTAFF: 'Warlock Rod',
  HARP: 'Harp',
}
const alchemyNames: Record<string, string> = {
  MATERIAL: 'Materials',
  MAGICSTONE: 'Magic Stones',
  MAGICTABLET: 'Magic Tablets',
  ATTRSTONE: 'Attribute Stones',
  ATTRTABLET: 'Attribute Tablets',
  ELEMENT: 'Elements',
  REINFORCE: 'Elixirs & Lucky Powders',
  UPPER: 'Enhancement Materials',
  RONDO: 'Rondos',
}
function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}
export function itemSearchTaxonomy(row: FoundItem) {
  const metadata = row.metadata || record(row.item.metadata)
  const sort = String(metadata.sort_type || '')
  const code = String(row.item.servername || '')
  let type = ''
  let subcategory = ''
  // These stable code namespaces come from the operator's exported item table;
  // unknown definitions remain searchable under Any type.
  if (code.startsWith('ITEM_MALL_')) type = 'silk'
  else if (code.startsWith('ITEM_ETC_ARCHEMY_')) {
    type = 'alchemy'
    subcategory = alchemyNames[code.split('_')[3] || ''] || ''
  } else if (sort === 'Weapon') {
    type = 'weapons'
    subcategory =
      weaponNames[/^ITEM_(?:CH|EU)_([A-Z]+)_/.exec(code)?.[1] || ''] || ''
  } else if (sort === 'Shield') type = 'shields'
  else if (armor.has(sort)) {
    type = 'sets'
    subcategory = sort
  } else if (accessories.has(sort)) {
    type = 'accessories'
    subcategory = sort
  } else if (sort === 'Potion' || sort === 'Pill') {
    type = 'consumables'
    subcategory = sort
  }
  const degree = Number(metadata.degree)
  return {
    type,
    subcategory,
    degree:
      Number.isInteger(degree) && degree >= 1 && degree <= 20
        ? String(degree)
        : '',
  }
}
export function filterSearchItems(
  rows: FoundItem[],
  filters: ItemSearchFilters,
) {
  const query = filters.query.trim().toLocaleLowerCase()
  return rows.filter((row) => {
    if (!filters.includeSets && row.container === 'equipment') return false
    if (
      query &&
      ![
        row.item.name,
        row.item.servername,
        row.item.model,
        row.metadata?.name,
        record(row.item.metadata).name,
      ].some(
        (value) =>
          value != null && String(value).toLocaleLowerCase().includes(query),
      )
    )
      return false
    const taxonomy = itemSearchTaxonomy(row)
    return (
      (!filters.type || taxonomy.type === filters.type) &&
      (!filters.subcategory || taxonomy.subcategory === filters.subcategory) &&
      (!filters.degree || taxonomy.degree === filters.degree)
    )
  })
}
export function itemSearchRowKey(row: FoundItem) {
  return JSON.stringify([
    row.server.toLocaleLowerCase(),
    row.observer_character_id,
    row.container,
    row.pet_id || '',
    row.slot,
  ])
}
export function itemSearchSource(row: FoundItem) {
  return {
    path: `/characters/${row.observer_character_id}`,
    query: { server: row.server, resource: row.container },
  }
}
export function itemSearchSlot(row: FoundItem) {
  // The reference and the container dialog page by zero-based displayed slot.
  const slot = displayedContainerSlot(row.container, row.slot)
  const { pageSize } = containerPageLayout(row.container)
  return `Page ${Math.floor(slot / pageSize) + 1} | Slot ${slot}`
}
const containerNames: Record<string, string> = {
  inventory: 'Inventory',
  equipment: 'Character Set',
  storage: 'Storage',
  guild_storage: 'Guild Storage',
  pets: 'Pet Inventory',
  job_pouch: 'Job Pouch',
}
const containerOrder = [
  'inventory',
  'equipment',
  'storage',
  'guild_storage',
  'pets',
  'job_pouch',
]
export function groupSearchItems(rows: FoundItem[]) {
  type Container = {
    key: string
    name: string
    source: ReturnType<typeof itemSearchSource>
    row: FoundItem
    items: FoundItem[]
  }
  type Owner = {
    key: string
    name: string
    server: string
    count: number
    containers: Container[]
  }
  const owners = new Map<string, Owner>()
  const containers = new Map<string, Container>()
  for (const row of rows) {
    const guild = row.owner_kind === 'guild'
    const ownerID = guild
      ? `guild:${row.guild || row.owner_id}`
      : row.observer_character_id
    const key = JSON.stringify([row.server.toLocaleLowerCase(), ownerID])
    let owner = owners.get(key)
    if (!owner) {
      owner = {
        key,
        name: guild
          ? row.guild || row.owner_name || row.owner_id
          : row.observer_name,
        server: row.server,
        count: 0,
        containers: [],
      }
      owners.set(key, owner)
    }
    owner.count++
    const containerKey = JSON.stringify([key, row.container, row.pet_id || ''])
    let container = containers.get(containerKey)
    if (!container) {
      const name =
        containerNames[row.container] || row.container.replaceAll('_', ' ')
      container = {
        key: containerKey,
        name: row.pet_id ? `${name} (${row.pet_id})` : name,
        source: itemSearchSource(row),
        row,
        items: [],
      }
      containers.set(containerKey, container)
      owner.containers.push(container)
    }
    container.items.push(row)
  }
  return [...owners.values()]
    .sort(
      (a, b) =>
        a.name.localeCompare(b.name) ||
        a.server.localeCompare(b.server) ||
        a.key.localeCompare(b.key),
    )
    .map((owner) => ({
      ...owner,
      containers: owner.containers
        .sort(
          (a, b) =>
            containerOrder.indexOf(a.row.container) -
              containerOrder.indexOf(b.row.container) ||
            a.key.localeCompare(b.key),
        )
        .map((container) => ({
          ...container,
          items: container.items.sort((a, b) => a.slot - b.slot),
        })),
    }))
}

// The HTTP API snapshots each bounded page. If a container changes between
// cursor pages, retry the read instead of mixing two revisions in one group.
export async function readItemSearchItems(
  readPage: (cursor?: string) => Promise<ItemSearchPage>,
): Promise<FoundItem[]> {
  for (let attempt = 0; attempt < 2; attempt++) {
    const found = new Map<string, FoundItem>()
    const revisions = new Map<string, string>()
    const cursors = new Set<string>()
    let total: number | undefined
    let changed = false
    let cursor: string | undefined
    do {
      const page = await readPage(cursor)
      total ??= page.total
      if (total !== page.total) {
        changed = true
        break
      }
      for (const row of page.items) {
        const key = JSON.stringify([
          row.server.toLocaleLowerCase(),
          row.owner_kind === 'guild'
            ? `guild:${(row.guild || row.owner_id).toLocaleLowerCase()}`
            : row.observer_character_id,
          row.container,
        ])
        const snapshot = JSON.stringify([
          row.observer_character_id,
          row.revision,
          row.observed_at,
        ])
        const revision = revisions.get(key)
        if (revision !== undefined && revision !== snapshot) {
          changed = true
          break
        }
        revisions.set(key, snapshot)
        found.set(itemSearchRowKey(row), row)
      }
      if (changed) break
      cursor = page.next_cursor
      if (cursor && cursors.has(cursor)) throw new Error('Repeated item cursor')
      if (cursor) cursors.add(cursor)
    } while (cursor)
    if (!changed && found.size === total) return [...found.values()]
  }
  throw new Error('Items changed while loading')
}
