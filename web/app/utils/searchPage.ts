// Shared, read-only page query and request identity helpers for Item Search and
// Global Economy. Server scope and cursor must come from the same filter frame.
export type ItemSearchInputs = {
  server: string
  query: string
  container: string
  owner: string
  itemType: string
  subcategory: string
  degree: string
  includeSets: boolean
  cursor: string
}

export type EconomySearchInputs = {
  server: string
  type: string
  sender: string
  query: string
  from: string
  to: string
  cursor: string
}

export function itemSearchQuery(filters: ItemSearchInputs) {
  return {
    server: filters.server === 'all' ? undefined : filters.server,
    q: filters.query || undefined,
    container: filters.container || undefined,
    owner: filters.owner.trim() || undefined,
    item_type: filters.itemType.trim() || undefined,
    subcategory: filters.subcategory.trim() || undefined,
    degree: filters.degree || undefined,
    include_sets: String(filters.includeSets),
    cursor: filters.cursor || undefined,
    limit: 30,
  }
}

export function economySearchQuery(filters: EconomySearchInputs) {
  return {
    server: filters.server === 'all' ? undefined : filters.server,
    offer_type: filters.type || undefined,
    sender: filters.sender.trim() || undefined,
    q: filters.query || undefined,
    from: filters.from || undefined,
    to: filters.to || undefined,
    cursor: filters.cursor || undefined,
    limit: 30,
  }
}

export function itemOwnerHref(item: {
  owner_kind: 'character' | 'guild' | 'pet'
  observer_character_id: string
}) {
  return item.owner_kind === 'guild'
    ? '/guild-storage'
    : `/characters/${item.observer_character_id}`
}

// Reuse the canonical event history; Item Search must not grow a second
// historical writer or borrow an instance's current rolls for older events.
export function itemHistoryTarget(item: {
  server: string
  item: Record<string, unknown>
}) {
  const code = typeof item.item.servername === 'string' ? item.item.servername : ''
  const model = item.item.model
  const name = typeof item.item.name === 'string' ? item.item.name : ''
  let identity = code || name
  if (!code && typeof model === 'number' && Number.isInteger(model)) {
    identity = String(model)
  }
  return {
    path: '/events',
    query: {
      server: item.server,
      item: identity || undefined,
    },
  }
}

// An old HTTP response is never authorized to replace a newer filter's page.
export function createLatestRequestGate() {
  let generation = 0
  return {
    next: () => ++generation,
    isCurrent: (id: number) => id === generation,
    invalidate: () => {
      generation++
    },
  }
}
