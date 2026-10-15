// Page geometry shared by Item Search labels and the container dialog. Bag
// inventory pages are 4 × 8; personal and guild storage pages are 6 × 5.
export type ContainerPageLayout = {
  columns: number
  rows: number
  pageSize: number
}

// Bag source slots include the 13 equipment slots that precede the bag.
const inventoryEquipmentSlots = 13

export function containerPageLayout(container?: string): ContainerPageLayout {
  if (container === 'storage' || container === 'guild_storage')
    return { columns: 6, rows: 5, pageSize: 30 }
  return { columns: 4, rows: 8, pageSize: 32 }
}

export function displayedContainerSlot(container: string, sourceSlot: number) {
  return container === 'inventory'
    ? Math.max(0, sourceSlot - inventoryEquipmentSlots)
    : sourceSlot
}

export function containerPageCount(capacity: number, pageSize: number) {
  return Math.max(1, Math.ceil(Math.max(0, capacity) / pageSize))
}

export function clampContainerPage(page: number, pageCount: number) {
  return Math.min(Math.max(0, page), Math.max(0, pageCount - 1))
}

// Returns the displayed slot numbers for one page, with null where the page
// extends beyond the reported capacity. Capacity is never inferred.
export function containerPageSlotNumbers(
  page: number,
  capacity: number,
  layout: ContainerPageLayout,
) {
  const start = page * layout.pageSize
  return Array.from({ length: layout.columns * layout.rows }, (_, index) =>
    start + index < capacity ? start + index : null,
  )
}
