import type { GlobalOfferItem } from '~~/shared/types/live'

// Labels and filter options for the heuristic item tags parsed from Global
// advertisement text. Keys match the Go parser in server/internal/chat.
export const offerItemTypes = [
  { key: 'weapons', label: 'Weapons' },
  { key: 'shields', label: 'Shields' },
  { key: 'sets', label: 'Set Parts' },
  { key: 'accessories', label: 'Accessories' },
  { key: 'alchemy', label: 'Alchemy' },
  { key: 'other', label: 'Other' },
]

const subtypeLabels: Record<string, string> = {
  staff: 'Staff',
  cleric: 'Cleric Rod',
  darkstaff: 'Dark Staff',
  sword_ch: 'Sword (CH)',
  sword_1h: 'Sword (1H)',
  sword_2h: 'Sword (2H)',
  blade: 'Blade',
  glaive: 'Glaive',
  spear: 'Spear',
  bow: 'Bow',
  crossbow: 'Crossbow',
  dagger: 'Dagger',
  harp: 'Harp',
  axe: 'Axe',
  garment: 'Garment',
  protector: 'Protector',
  armor: 'Armor',
  ring: 'Ring',
  earring: 'Earring',
  necklace: 'Necklace',
  elixir: 'Elixir',
  magic_stone: 'Magic Stone',
  attribute_stone: 'Attribute Stone',
  lucky_powder: 'Lucky Powder',
  rondo: 'Destroyer Rondo',
  pandora: 'Pandora',
  premium: 'Premium',
  flag: 'Dragon Flag',
  wing: 'Devil Wing',
  avatar: 'Avatar',
  taxi: 'Taxi',
  service: 'Service',
  box: 'Box',
}

const subtypesByType: Record<string, string[]> = {
  weapons: [
    'staff',
    'cleric',
    'darkstaff',
    'sword_ch',
    'sword_1h',
    'sword_2h',
    'blade',
    'glaive',
    'spear',
    'bow',
    'crossbow',
    'dagger',
    'harp',
    'axe',
  ],
  sets: ['garment', 'protector', 'armor'],
  accessories: ['ring', 'earring', 'necklace'],
  alchemy: [
    'elixir',
    'magic_stone',
    'attribute_stone',
    'lucky_powder',
    'rondo',
  ],
  other: [
    'pandora',
    'premium',
    'flag',
    'wing',
    'avatar',
    'taxi',
    'service',
    'box',
  ],
}

const sealLabels: Record<string, string> = {
  star: 'Star',
  moon: 'Moon',
  sun: 'Sun',
  nova: 'Nova',
  egy_a: 'Egy A',
  egy_b: 'Egy B',
}
const raceLabels: Record<string, string> = {
  chinese: 'Chinese',
  european: 'European',
}
const currencyLabels: Record<string, string> = {
  silk: 'Silk',
  gold: 'Gold',
  arena_coins: 'Arena coins',
}
const partLabels: Record<string, string> = {
  HA: 'Head',
  SA: 'Shoulder',
  BA: 'Chest',
  LA: 'Legs',
  AA: 'Hands',
  FA: 'Feet',
}

export const offerDegrees = Array.from({ length: 14 }, (_, i) => i + 1)

export function offerSubtypeOptions(type: string) {
  return (subtypesByType[type] || []).map((key) => ({
    key,
    label: subtypeLabels[key] || key,
  }))
}

export function offerTypeLabel(type: string) {
  return offerItemTypes.find((item) => item.key === type)?.label || type
}

// Chips shown on an offer card, in reading order. Only parsed values appear.
export function offerItemChips(item: GlobalOfferItem): string[] {
  const chips = [
    offerTypeLabel(item.item_type),
    item.subtype ? subtypeLabels[item.subtype] || item.subtype : '',
    item.part ? partLabels[item.part] || item.part : '',
    item.degree ? `D${item.degree}` : '',
    item.seal ? sealLabels[item.seal] || item.seal : '',
    item.race ? raceLabels[item.race] || item.race : '',
    item.plus ? `+${item.plus}` : '',
    item.price || '',
    item.currency ? currencyLabels[item.currency] || item.currency : '',
    ...(item.notes || []),
  ]
  return chips.filter(Boolean)
}
