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
  requirements: { text: string; level: boolean }[]
  blues: string[]
}

const statLabels: Record<string, string> = {
  phy_atk_pwr: 'Phy. atk. pwr',
  mag_atk_pwr: 'Mag. atk. pwr',
  phy_reinforce: 'Phy. reinforce',
  mag_reinforce: 'Mag. reinforce',
  phy_def_pwr: 'Phy. def. pwr',
  mag_def_pwr: 'Mag. def. pwr',
  parry_ratio: 'Parry ratio',
  phy_absorption: 'Phy. absorption',
  mag_absorption: 'Mag. absorption',
  hit_ratio: 'Attack rating',
  critical_ratio: 'Critical',
  durability: 'Durability',
}

const scalarStatKeys = [
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

const referenceStatLabels: Record<string, string> = {
  phy_atk_pwr_min: 'Phy. atk. pwr min',
  phy_atk_pwr_max: 'Phy. atk. pwr max',
  mag_atk_pwr_min: 'Mag. atk. pwr min',
  mag_atk_pwr_max: 'Mag. atk. pwr max',
  phy_def_pwr: 'Phy. def. pwr',
  mag_def_pwr: 'Mag. def. pwr',
  phy_reinforce: 'Phy. reinforce',
  mag_reinforce: 'Mag. reinforce',
  parry_ratio: 'Parry ratio',
  durability: 'Durability',
  hit_ratio: 'Attack rating',
  critical_ratio: 'Critical',
}

const catalogNote =
  "Catalog ranges; this drop's rolled values were not observed."

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
        text: `${stat.label} ${stat.value}${percent}`,
      },
    ]
  })
}

function rollLines(
  details: Record<string, unknown> | null,
  statKeys: string[],
) {
  const values = details?.percentages
  if (!Array.isArray(values)) return []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const record = entry as Record<string, unknown>
    if (statKeys.includes(String(record.key ?? ''))) return []
    if (
      typeof record.label !== 'string' ||
      typeof record.value !== 'number' ||
      !Number.isInteger(record.value) ||
      record.value < 0 ||
      record.value > 100
    )
      return []
    return [`${record.label} (+${record.value}%)`]
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
      'Able to use Advanced elixir.': () => 'Able to use Advanced elixir.',
      'Advanced elixir is in effect': (amount) =>
        `Advanced elixir is in effect [+${amount}]`,
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
      `${statLabels[key] || key.replaceAll('_', ' ')}: ${formatNumber(value)}${percentText}`,
    ]
  })
}

function referenceRangeLines(metadata: Record<string, unknown>) {
  const ranges = asRecord(metadata.reference_stats)
  return Object.entries(ranges).flatMap(([key, raw]) => {
    const range = asRecord(raw)
    if (typeof range.min !== 'string' || typeof range.max !== 'string')
      return []
    const label = referenceStatLabels[key] || key.replaceAll('_', ' ')
    const value =
      range.min === range.max ? range.min : `${range.min}–${range.max}`
    return [`${label} ${value}`]
  })
}

export function buildItemDetail(
  item: Record<string, unknown>,
): ItemDetailModel {
  const metadata = asRecord(item.metadata)
  const detail = { ...metadata, ...item }
  const instance = asRecord(item.instance_details)
  const instanceStats = instanceStatLines(
    Object.keys(instance).length ? instance : null,
  )
  const rolls = rollLines(
    Object.keys(instance).length ? instance : null,
    instanceStats.map((stat) => stat.key),
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
  const ranges =
    instanceStats.length || scalars.length || item.drop_unobserved === true
      ? []
      : referenceRangeLines(metadata)
  const name =
    stringField(detail, 'name') ||
    stringField(item, 'servername') ||
    'Unknown item'
  const degree = numberField(detail, 'degree')
  const classifications = [
    stringField(detail, 'sort_type')
      ? `Sort of item: ${stringField(detail, 'sort_type')}`
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
            level: key === 'required_level',
          },
        ]
      },
    ),
    ...['required_gender', 'required_race'].flatMap((key) => {
      const value = stringField(detail, key)
      return value ? [{ text: value, level: false }] : []
    }),
  ]
  return {
    name,
    plus: numberField(item, 'plus'),
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
    ],
    catalogNote:
      item.drop_unobserved === true
        ? 'Rolled stats and blue options were not observed for this drop.'
        : ranges.length
          ? catalogNote
          : '',
    requirements,
    blues: blueLines(Object.keys(instance).length ? instance : null),
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
  if (association === 'unique_model_inventory_gain') {
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
