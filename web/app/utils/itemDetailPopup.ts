import type { ActivityEvent } from '~~/shared/types/live'
import {
  formatScaledUnsigned,
  formatUnsignedCount,
} from './item-instance-format.ts'

export interface ItemDetailModel {
  name: string
  plus: number | null
  quantity: number | null
  rare: boolean
  icon: string
  iconFallback: string
  seal: string
  classifications: string[]
  statsBeforeDurability: string[]
  durability: string
  statsAfterDurability: string[]
  catalogNote: string
  requirements: { text: string }[]
  blues: string[]
  notes: string[]
}

export function relatedItemDetail(item: Record<string, unknown>) {
  const observation = asRecord(item.related_item_observation)
  const snapshot = asRecord(observation.item)
  if (
    ![
      'stored_unverified_drop_event_id',
      'nearby_same_model_observation',
      'gap_reconciled_same_model',
    ].includes(String(observation.association)) ||
    typeof observation.event_id !== 'string' ||
    !Object.keys(snapshot).length
  )
    return null
  const container = asRecord(observation.container)
  return {
    eventID: observation.event_id,
    afterGap: observation.association === 'gap_reconciled_same_model',
    source:
      container.type === 'pets'
        ? 'Pet inventory observation'
        : 'Inventory observation',
    model: buildItemDetail({ ...snapshot, owned_gain: true }),
  }
}

// Select one display source; never merge another observation's rolls into the
// canonical drop record or combine fields from two physical item copies.
export function itemTooltipPresentation(item: Record<string, unknown>) {
  const own = buildItemDetail(item)
  const related = item.drop_unobserved === true ? relatedItemDetail(item) : null
  const model = related?.model ?? own
  const hasStats =
    model.statsBeforeDurability.length ||
    model.statsAfterDurability.some(
      (line) => !line.startsWith('Attack distance'),
    ) ||
    model.blues.length
  return {
    model: { ...model, catalogNote: '' },
    caption: related
      ? hasStats
        ? related.afterGap
          ? 'Observed after a connection gap · pickup unverified'
          : 'Observed item · pickup unverified'
        : 'Observed item · stats unavailable'
      : model.catalogNote.startsWith('Catalog ranges')
        ? 'Catalog ranges · rolls unavailable'
        : model.catalogNote
          ? 'Item stats unavailable'
          : '',
    sourceDescription: related
      ? `${related.source}${related.afterGap ? ', found after a connection gap' : ''}. Relation to the drop is unverified. Source event: ${related.eventID}`
      : model.catalogNote,
  }
}

const statOrder = [
  'Phy. atk. pwr',
  'Mag. atk. pwr',
  'Phy. def. pwr',
  'Mag. def. pwr',
  'Phy. absorption',
  'Mag. absorption',
  'Durability',
  'Attack distance',
  'Attack rate',
  'Parry rate',
  'Blocking rate',
  'Critical',
  'Phy. reinforce',
  'Mag. reinforce',
]

// Order follows the in-game item description; unknown lines keep their place
// after the known ones (Array.sort is stable).
export function itemTooltipStatLines(model: ItemDetailModel) {
  const rank = (line: string) => {
    const index = statOrder.findIndex((label) => line.startsWith(label))
    return index < 0 ? statOrder.length : index
  }
  return [
    ...model.statsBeforeDurability,
    ...model.statsAfterDurability,
    model.durability,
  ]
    .filter(Boolean)
    .sort((left, right) => rank(left) - rank(right))
}

const statLabels: Record<string, string> = {
  phy_atk_pwr: 'Phy. atk. pwr',
  mag_atk_pwr: 'Mag. atk. pwr',
  phy_reinforce: 'Phy. reinforce',
  mag_reinforce: 'Mag. reinforce',
  phy_def_pwr: 'Phy. def. pwr',
  mag_def_pwr: 'Mag. def. pwr',
  parry_ratio: 'Parry rate',
  block_ratio: 'Blocking rate',
  phy_absorption: 'Phy. absorption',
  mag_absorption: 'Mag. absorption',
  hit_ratio: 'Attack rate',
  critical_ratio: 'Critical',
  durability: 'Durability',
}

const scalarStatKeys = [
  'block_ratio',
  'phy_atk_pwr',
  'mag_atk_pwr',
  'phy_def_pwr',
  'mag_def_pwr',
  'parry_ratio',
  'phy_absorption',
  'mag_absorption',
  'phy_reinforce',
  'mag_reinforce',
  'hit_ratio',
  'critical_ratio',
]

const catalogNote =
  "Catalog ranges; this drop's rolled values were not observed."

// Weapons, armor, shields and accessories. Job suits, avatars and devil
// spirits (7, 13, 14) do not take Advanced elixirs.
const elixirFamilies = [1, 2, 3, 4, 5, 6, 9, 10, 11, 12]

function asRecord(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
}

function field(item: object | null | undefined, key: string) {
  if (!item || !(key in item)) return undefined
  return (item as Record<string, unknown>)[key]
}

function numberField(item: object | null | undefined, key: string) {
  const value = field(item, key)
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}

function stringField(item: object | null | undefined, key: string) {
  const value = field(item, key)
  return typeof value === 'string' && value.trim() ? value : ''
}

function formatNumber(value: number) {
  return value.toLocaleString(undefined, { maximumFractionDigits: 1 })
}

function safeIcon(path: string) {
  return path.startsWith('/game-assets/') && !path.includes('..') ? path : ''
}

function instanceStatLines(details: Record<string, unknown> | null) {
  const values = details?.stats
  if (!Array.isArray(values)) return []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const stat = entry as Record<string, unknown>
    if (typeof stat.label !== 'string' || typeof stat.value !== 'string')
      return []
    if (!/^(?:\d+(?:\.\d+)?%?)(?: ~ \d+(?:\.\d+)?%?)?$/.test(stat.value))
      return []
    const percent =
      typeof stat.percent === 'number' &&
      Number.isInteger(stat.percent) &&
      stat.percent >= 0 &&
      stat.percent <= 100
        ? ` (+${stat.percent}%)`
        : ''
    return [
      {
        key: String(stat.key ?? ''),
        text: `${statLabels[String(stat.key)] ?? stat.label} ${stat.value}${percent}`,
      },
    ]
  })
}

// A white roll for a stat the item does not have (a staff's physical attack)
// is not shown; the catalog marks such stats with an all-zero range.
function zeroReferenceStat(metadata: Record<string, unknown>, key: string) {
  const ranges = asRecord(metadata.reference_stats)
  const parts = [ranges[key], ranges[`${key}_min`], ranges[`${key}_max`]]
    .map(asRecord)
    .filter((range) => Object.keys(range).length)
  return (
    parts.length > 0 &&
    parts.every((range) => range.min === '0' && range.max === '0')
  )
}

function rollLines(
  details: Record<string, unknown> | null,
  statKeys: string[],
  metadata: Record<string, unknown>,
) {
  const values = details?.percentages
  if (!Array.isArray(values)) return []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const record = entry as Record<string, unknown>
    const key = String(record.key ?? '')
    if (statKeys.includes(key) || zeroReferenceStat(metadata, key)) return []
    if (
      typeof record.label !== 'string' ||
      typeof record.value !== 'number' ||
      !Number.isInteger(record.value) ||
      record.value < 0 ||
      record.value > 100
    )
      return []
    return [`${statLabels[key] ?? record.label} (+${record.value}%)`]
  })
}

function masteryRequirements(detail: Record<string, unknown>) {
  const values = Array.isArray(detail.mastery_requirements)
    ? detail.mastery_requirements
    : []
  return values.flatMap((entry) => {
    const record = asRecord(entry)
    return typeof record.mastery === 'string' &&
      /^[A-Za-z ]{1,24}$/.test(record.mastery) &&
      typeof record.level === 'number' &&
      Number.isInteger(record.level) &&
      record.level > 0
      ? [{ text: `Mastery level: ${record.mastery} ${record.level}` }]
      : []
  })
}

function blueLines(details: Record<string, unknown> | null) {
  const values = Array.isArray(details?.blues) ? details.blues : []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const record = entry as Record<string, unknown>
    const name = record.label
    const value = record.value ?? record.raw_value
    if (
      typeof name !== 'string' ||
      typeof value !== 'string' ||
      !/^\d+$/.test(value)
    )
      return []
    const formatted = formatScaledUnsigned(
      value,
      record.scale,
      record.precision ?? 0,
    )
    if (formatted === null) return []
    const unit = typeof record.unit === 'string' ? record.unit : ''
    const blueNames: Record<string, (amount: string) => string> = {
      'Int increase': (amount) => `Int ${amount} Increase`,
      'Str increase': (amount) => `Str ${amount} Increase`,
      'MP increase': (amount) => `MP ${amount} Increase`,
      'HP increase': (amount) => `HP ${amount} Increase`,
      Steady: (amount) => `Steady(${amount}Time/times)`,
      'Parry rate increase': (amount) => `Parry rate ${amount}${unit} Increase`,
      'Attack rate increase': (amount) =>
        `Attack rate ${amount}${unit} Increase`,
      'Durability increase': (amount) => `Durability ${amount}${unit} Increase`,
      Lucky: (amount) => `Lucky(${amount}Time/times)`,
      Immortal: (amount) => `Immortal(${amount}Time/times)`,
      Astral: (amount) => `Astral(${amount}Time/times)`,
      'Advanced elixir is in effect': (amount) =>
        `Advanced elixir is in effect [+${amount}]`,
      'Freezing and frostbite duration reduction': (amount) =>
        `Freezing, frostbite ${amount}${unit} Reduce`,
      'Electric shock duration reduction': (amount) =>
        `Electric shock ${amount}${unit} Reduce`,
      'Burn duration reduction': (amount) => `Burn ${amount}${unit} Reduce`,
      'Poison duration reduction': (amount) =>
        `Poisoning ${amount}${unit} Reduce`,
      'Zombie duration reduction': (amount) => `Zombie ${amount}${unit} Reduce`,
    }
    return [blueNames[name]?.(formatted) ?? `${name} ${formatted}${unit}`]
  })
}

function scalarStatLines(item: Record<string, unknown>) {
  return scalarStatKeys.flatMap((key) => {
    const value = numberField(item, key)
    if (value === null) return []
    const percent = numberField(item, `${key}_percent`)
    const percentText = percent === null ? '' : ` (+${formatNumber(percent)}%)`
    return [
      `${statLabels[key] || key.replaceAll('_', ' ')} ${formatNumber(value)}${percentText}`,
    ]
  })
}

// itemdata stores reinforce as tenths of a percent (4608 -> 460.8%).
function reinforcePercent(value: string) {
  return /^\d+$/.test(value) ? `${(Number(value) / 10).toFixed(1)}%` : value
}

function referenceRangeLines(metadata: Record<string, unknown>) {
  const ranges = asRecord(metadata.reference_stats)
  const lines: string[] = []
  const done = new Set<string>()
  for (const [key, raw] of Object.entries(ranges)) {
    const base = key.replace(/_(min|max)$/, '')
    if (done.has(base)) continue
    done.add(base)
    const lowRange = asRecord(ranges[key === base ? key : `${base}_min`])
    const highRange = asRecord(ranges[key === base ? key : `${base}_max`])
    const range = key === base ? asRecord(raw) : lowRange
    const low = range.min
    const high = key === base ? range.max : highRange.max
    if (typeof low !== 'string' || typeof high !== 'string') continue
    if (low === '0' && high === '0') continue
    const label = statLabels[base] || base.replaceAll('_', ' ')
    const format = base.endsWith('_reinforce') ? reinforcePercent : String
    lines.push(
      `${label} ${low === high ? format(low) : `${format(low)}–${format(high)}`}`,
    )
  }
  return lines
}

function blueEntries(details: Record<string, unknown> | null) {
  const values = Array.isArray(details?.blues) ? details.blues : []
  return values
    .flatMap((entry) => {
      const text = blueLines({ blues: [entry] })[0]
      if (!text) return []
      const record = asRecord(entry)
      const id = Number(record.id)
      const percent = record.percent
      return [
        {
          id: Number.isFinite(id) ? id : Number.MAX_SAFE_INTEGER,
          text:
            typeof percent === 'number' &&
            Number.isInteger(percent) &&
            percent >= 0 &&
            percent <= 100
              ? `${text} (+${percent}%)`
              : text,
        },
      ]
    })
    .sort((left, right) => left.id - right.id)
    .map((entry) => entry.text)
}

export function buildItemDetail(
  item: Record<string, unknown>,
): ItemDetailModel {
  const metadata = asRecord(item.metadata)
  const detail = { ...metadata, ...item }
  const instance = asRecord(item.instance_details)
  const packetInstance = asRecord(item.instance)
  const instanceStats = instanceStatLines(
    Object.keys(instance).length ? instance : null,
  )
  const rolls = rollLines(
    Object.keys(instance).length ? instance : null,
    instanceStats.map((stat) => stat.key),
    metadata,
  )
  const hasDurability =
    Array.isArray(metadata.type_ids) &&
    metadata.type_ids[1] === 1 &&
    [1, 2, 3, 4, 6, 9, 10, 11].includes(metadata.type_ids[2])
  let durability = ''
  if (hasDurability) {
    const observed = asRecord(instance.durability)
    const current =
      formatUnsignedCount(observed.current) ??
      formatUnsignedCount(item.durability)
    if (current !== null) {
      const maximum = formatUnsignedCount(observed.maximum)
      const durabilityRoll = rolls.find((line) =>
        line.startsWith('Durability '),
      )
      durability = `Durability ${current}${maximum ? `/${maximum}` : ''}${
        durabilityRoll ? durabilityRoll.slice('Durability'.length) : ''
      }`
    }
  }
  const scalars = instanceStats.length ? [] : scalarStatLines(item)
  const blues = blueEntries(Object.keys(instance).length ? instance : null)
  const ranges =
    instanceStats.length ||
    scalars.length ||
    item.drop_unobserved === true ||
    item.instance_unobserved === true ||
    item.owned_gain === true ||
    item.visible_equipment_only === true
      ? []
      : referenceRangeLines(metadata)
  const name =
    stringField(detail, 'name') ||
    stringField(item, 'servername') ||
    'Unknown item'
  const degree = numberField(detail, 'degree')
  const classifications = [
    stringField(detail, 'item_group') || stringField(detail, 'sort_type')
      ? `Sort of item: ${stringField(detail, 'item_group') || stringField(detail, 'sort_type')}`
      : '',
    stringField(detail, 'mounted_part')
      ? `Mounting part: ${stringField(detail, 'mounted_part')}`
      : '',
    degree === null
      ? ''
      : `Degree: ${degree} ${degree === 1 ? 'degree' : 'degrees'}`,
  ].filter(Boolean)
  const requirements = [
    ...['required_level', 'required_strength', 'required_intelligence'].flatMap(
      (key) => {
        const value = numberField(detail, key)
        if (value === null) return []
        return [
          {
            text: `Required ${key.replace('required_', '').replaceAll('_', ' ')} ${value}`,
          },
        ]
      },
    ),
    ...masteryRequirements(detail),
    ...['required_gender', 'required_race'].flatMap((key) => {
      const value = stringField(detail, key)
      return value ? [{ text: value }] : []
    }),
    ...(numberField(detail, 'max_magic_options') === null
      ? []
      : [
          {
            text: `Max no. of magic options: ${numberField(detail, 'max_magic_options')}`,
          },
        ]),
  ]
  const typeIDs = Array.isArray(metadata.type_ids) ? metadata.type_ids : []
  const attackDistance = stringField(detail, 'attack_distance')
  return {
    name,
    plus:
      numberField(item, 'plus') ??
      (packetInstance.availability === 'observed' &&
      packetInstance.source === 'vsro_1188_packet'
        ? numberField(packetInstance, 'plus')
        : null),
    quantity: numberField(item, 'quantity'),
    rare: metadata.rare === true || !!stringField(item, 'seal'),
    icon: safeIcon(stringField(metadata, 'icon_url')),
    iconFallback:
      stringField(item, 'icon_fallback') || name.slice(0, 1).toUpperCase(),
    seal: stringField(detail, 'seal') || stringField(detail, 'seal_type'),
    classifications,
    statsBeforeDurability: instanceStats.length
      ? instanceStats.slice(0, 2).map((stat) => stat.text)
      : scalars.length
        ? scalars
        : ranges,
    durability,
    statsAfterDurability: [
      ...instanceStats.slice(2).map((stat) => stat.text),
      ...rolls.filter((line) => !line.startsWith('Durability ')),
      ...(attackDistance && /^\d+(?:\.\d)?$/.test(attackDistance)
        ? [`Attack distance ${attackDistance}`]
        : []),
    ],
    catalogNote:
      item.visible_equipment_only === true
        ? 'Visible equipment references and verified enhancement only. Rolled stats, blues, durability and untransmitted equipment were not observed.'
        : item.drop_unobserved === true
          ? 'Rolled stats and blue options were not observed for this drop.'
          : item.instance_unobserved === true
            ? 'The gained instance could not be identified among copies of this item.'
            : item.owned_gain === true &&
                !instanceStats.length &&
                !scalars.length &&
                !durability &&
                !blues.length
              ? 'Rolled stats and blue options were not observed for this item gain.'
              : ranges.length
                ? catalogNote
                : '',
    requirements,
    blues,
    notes:
      typeIDs[0] === 3 &&
      typeIDs[1] === 1 &&
      elixirFamilies.includes(Number(typeIDs[2])) &&
      !blues.some((line) => line.startsWith('Advanced elixir is in effect'))
        ? ['Able to use Advanced elixir.']
        : [],
  }
}

export function itemRecordFromActivityEvent(
  event: Pick<
    ActivityEvent,
    | 'item_model'
    | 'item_code'
    | 'item_name'
    | 'item_icon_url'
    | 'item_metadata'
    | 'item_details'
    | 'related_item_observation'
    | 'payload'
  > & { kind?: ActivityEvent['kind'] },
): Record<string, unknown> | null {
  const payload = asRecord(event.payload)
  const snapshot = {
    ...asRecord(payload.item),
    ...asRecord(event.item_details),
  }
  const packetObservation = asRecord(payload.packet_observation)
  const observationID = stringField(packetObservation, 'observation_id')
  const itemObservation = asRecord(payload.item_observation)
  const association = stringField(itemObservation, 'association')
  if (observationID) {
    snapshot.instance = {
      ...asRecord(snapshot.instance),
      observation_id: observationID,
    }
  }
  if (
    association === 'unique_model_inventory_gain' ||
    association === 'unique_temporal_inventory_gain'
  ) {
    snapshot.instance = {
      ...asRecord(snapshot.instance),
      association,
    }
  }
  const metadata = asRecord(event.item_metadata)
  const snapshotMetadata = asRecord(snapshot.metadata)
  const name =
    stringField(metadata, 'name') ||
    stringField(snapshot, 'name') ||
    stringField(payload, 'item_name') ||
    stringField(event, 'item_name')
  const code =
    stringField(event, 'item_code') || stringField(snapshot, 'servername')
  const model =
    numberField(event, 'item_model') ??
    numberField(payload, 'model') ??
    numberField(snapshot, 'model')
  if (
    !name &&
    !code &&
    model === null &&
    Object.keys(metadata).length === 0 &&
    Object.keys(snapshot).length === 0
  )
    return null
  const icon =
    stringField(metadata, 'icon_url') || stringField(event, 'item_icon_url')
  return {
    ...snapshot,
    ...(event.related_item_observation
      ? { related_item_observation: event.related_item_observation }
      : {}),
    ...(event.kind === 'item.acquired' ||
    event.kind === 'item.quantity_increased'
      ? { owned_gain: true }
      : {}),
    ...(payload.item_instance_unobserved === true
      ? { instance_unobserved: true }
      : {}),
    ...((event.kind === 'drop.item' || event.kind === 'drop.rare') &&
    !Object.keys(asRecord(payload.item)).length
      ? { drop_unobserved: true }
      : {}),
    ...(name ? { name } : {}),
    ...(code ? { servername: code } : {}),
    ...(model === null ? {} : { model }),
    ...(numberField(snapshot, 'plus') === null &&
    numberField(payload, 'plus') !== null
      ? { plus: numberField(payload, 'plus') }
      : {}),
    metadata: {
      ...snapshotMetadata,
      ...metadata,
      ...(name ? { name } : {}),
      ...(icon ? { icon_url: icon } : {}),
    },
  }
}
