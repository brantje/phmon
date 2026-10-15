export interface EquipmentCell {
  sourceSlot: number | null
  label: string
}

// phMonitor v0.5.0 equipmentLayoutHtml: Silkroad source slots 0–12 placed
// in two seven-row columns around the character card. The left column ends
// with an empty spacer so both columns have the same height.
const left: EquipmentCell[] = [
  { sourceSlot: 6, label: 'Weapon' },
  { sourceSlot: 0, label: 'Head' },
  { sourceSlot: 1, label: 'Chest' },
  { sourceSlot: 4, label: 'Legs' },
  { sourceSlot: 9, label: 'Earring' },
  { sourceSlot: 11, label: 'Ring' },
  { sourceSlot: null, label: '' },
]
const right: EquipmentCell[] = [
  { sourceSlot: 7, label: 'Shield' },
  { sourceSlot: 2, label: 'Shoulders' },
  { sourceSlot: 3, label: 'Hands' },
  { sourceSlot: 5, label: 'Feet' },
  { sourceSlot: 10, label: 'Necklace' },
  { sourceSlot: 12, label: 'Ring' },
  { sourceSlot: 8, label: 'Job' },
]

export const equipmentColumns = { left, right }

/** Returns the item at a source slot, preferring an explicit source_slot. */
export function equipmentItemAt(
  slots: unknown,
  sourceSlot: number | null,
): Record<string, unknown> | null {
  if (sourceSlot == null || !Array.isArray(slots)) return null
  const explicit = slots.find(
    (slot) =>
      slot &&
      typeof slot === 'object' &&
      (slot as Record<string, unknown>).source_slot === sourceSlot,
  )
  const slot = explicit ?? slots[sourceSlot]
  if (!slot || typeof slot !== 'object' || !('item' in slot)) return null
  const item = (slot as Record<string, unknown>).item
  return item && typeof item === 'object'
    ? (item as Record<string, unknown>)
    : null
}

export interface CharacterInfoRow {
  label: string
  value: string
}

function record(value: unknown): Record<string, unknown> | null {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
}

function count(value: unknown): number | null {
  return typeof value === 'number' && Number.isInteger(value) && value >= 0
    ? value
    : null
}

/**
 * phMonitor v0.5.0 statsInfoPanelHtml rows. `info` is the plugin's
 * character_info resource payload; unobserved values render as "-".
 */
export function characterInfoRows(
  character: { name: string; guild?: string },
  info: Record<string, unknown>,
  lastSeen: string,
): CharacterInfoRow[] {
  const guild = record(info.guild)
  const union = record(info.union)
  const members = count(guild?.member_count)
  const online = count(guild?.online_count)
  const donated = count(guild?.donated_gp)
  const unionCount = count(union?.count)
  const unionNames = Array.isArray(union?.names)
    ? union.names.filter((name): name is string => typeof name === 'string')
    : []
  return [
    { label: 'Character', value: character.name || '-' },
    {
      label: 'Job name',
      value: typeof info.job_name === 'string' ? info.job_name : '-',
    },
    { label: 'Guild', value: character.guild || '-' },
    {
      label: 'Guild members',
      value: members ? `${online ?? 0} / ${members}` : '-',
    },
    {
      label: 'Donated GP',
      value: donated == null ? '-' : donated.toLocaleString('en-US'),
    },
    {
      label: 'Union guilds',
      value: unionCount
        ? `${unionCount}${unionNames.length ? ` - ${unionNames.join(', ')}` : ''}`
        : '-',
    },
    { label: 'Last seen', value: lastSeen || '-' },
  ]
}
