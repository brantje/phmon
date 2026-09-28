<script setup lang="ts">
import {
  formatScaledUnsigned,
  formatUnsignedCount,
} from '../utils/item-instance-format'

const props = defineProps<{
  item: Record<string, unknown> | null
  slotNumber: number
  label?: string
}>()
const metadata = computed(() => {
  const value = props.item?.metadata
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : {}
})
const detail = computed(() => ({ ...metadata.value, ...props.item }))
const instanceDetails = computed(() => {
  const value = props.item?.instance_details
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
})
const hasDurability = computed(() => {
  const type = metadata.value.type_ids
  return (
    Array.isArray(type) &&
    type[1] === 1 &&
    [1, 2, 3, 4, 6, 9, 10, 11].includes(type[2])
  )
})
const rare = computed(
  () => metadata.value.rare === true || !!stringField(props.item, 'seal'),
)
const iconFailed = ref(false)
const icon = computed(() => {
  const path = stringField(metadata.value, 'icon_url')
  return path.startsWith('/game-assets/') && !path.includes('..') ? path : ''
})
watch(
  () =>
    `${props.item?.model ?? ''}:${props.item?.servername ?? ''}:${icon.value}`,
  () => {
    iconFailed.value = false
    dismiss()
  },
)
const pinned = ref(false)
const visible = ref(false)
const name = computed(
  () =>
    stringField(detail.value, 'name') ||
    stringField(props.item, 'servername') ||
    'Unknown item',
)
const quantity = computed(() => numberField(props.item, 'quantity'))
const plus = computed(() => numberField(props.item, 'plus'))
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
const statLines = computed(() => {
  const values = instanceDetails.value?.stats
  if (!Array.isArray(values)) return []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const stat = entry as Record<string, unknown>
    if (typeof stat.label !== 'string' || typeof stat.value !== 'string')
      return []
    if (!/^(?:\d+(?:\.\d+)?)(?: ~ \d+(?:\.\d+)?)?$/.test(stat.value)) return []
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
})

const blueLines = computed(() => {
  const details = instanceDetails.value
  // Only render backend-resolved definitions. Raw API fields and legacy
  // `blues` keys are not trusted item semantics.
  const values = Array.isArray(details?.blues) ? details.blues : []
  if (!Array.isArray(values)) return []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const record = entry as Record<string, unknown>
    const name = record.label
    const value = record.value ?? record.raw_value
    if (
      typeof name === 'string' &&
      typeof value === 'string' &&
      /^\d+$/.test(value)
    ) {
      const formatted = formatScaledUnsigned(
        value,
        record.scale,
        record.precision ?? 0,
      )
      if (formatted === null) return []
      const unit = typeof record.unit === 'string' ? record.unit : ''
      const blueNames: Record<string, (value: string) => string> = {
        'Int increase': (value) => `Int ${value} Increase`,
        'Str increase': (value) => `Str ${value} Increase`,
        'MP increase': (value) => `MP ${value} Increase`,
        'HP increase': (value) => `HP ${value} Increase`,
        Steady: (value) => `Steady(${value}Time/times)`,
        'Parry rate increase': (value) => `Parry rate ${value}${unit} Increase`,
        'Attack rate increase': (value) =>
          `Attack rate ${value}${unit} Increase`,
        'Durability increase': (value) => `Durability ${value}${unit} Increase`,
        Lucky: (value) => `Lucky(${value}Time/times)`,
        Immortal: (value) => `Immortal(${value}Time/times)`,
        Astral: (value) => `Astral(${value}Time/times)`,
        'Able to use Advanced elixir.': () => 'Able to use Advanced elixir.',
        'Advanced elixir is in effect': (value) =>
          `Advanced elixir is in effect [+${value}]`,
      }
      return [blueNames[name]?.(formatted) ?? `${name} ${formatted}${unit}`]
    }
    return []
  })
})
const rollLines = computed(() => {
  const values = instanceDetails.value?.percentages
  if (!Array.isArray(values)) return []
  return values.flatMap((entry) => {
    if (!entry || typeof entry !== 'object' || Array.isArray(entry)) return []
    const record = entry as Record<string, unknown>
    if (statLines.value.some((stat) => stat.key === record.key)) return []
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
})
const durabilityLine = computed(() => {
  if (!hasDurability.value) return ''
  const observed = instanceDetails.value?.durability
  const record =
    observed && typeof observed === 'object' && !Array.isArray(observed)
      ? (observed as Record<string, unknown>)
      : null
  const current =
    formatUnsignedCount(record?.current) ??
    formatUnsignedCount(props.item?.durability)
  if (current === null) return ''
  const maximum = formatUnsignedCount(record?.maximum)
  const durabilityRoll = rollLines.value.find((line) =>
    line.startsWith('Durability '),
  )
  const maximumText = maximum ? `/${maximum}` : ''
  const qualityText = durabilityRoll
    ? durabilityRoll.slice('Durability'.length)
    : ''
  return `Durability ${current}${maximumText}${qualityText}`
})

function numberField(item: Record<string, unknown> | null, key: string) {
  const value = item?.[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}
function stringField(item: Record<string, unknown> | null, key: string) {
  const value = item?.[key]
  return typeof value === 'string' && value.trim() ? value : ''
}
function formatNumber(value: number) {
  return value.toLocaleString(undefined, { maximumFractionDigits: 1 })
}
function onEscape(event: KeyboardEvent) {
  if (event.key === 'Escape') dismiss()
}
onMounted(() => document.addEventListener('keydown', onEscape))
onBeforeUnmount(() => document.removeEventListener('keydown', onEscape))
function togglePinned() {
  if (!props.item) return
  pinned.value = !pinned.value
  visible.value = pinned.value
}
function dismiss() {
  pinned.value = false
  visible.value = false
}
</script>

<template>
  <div
    class="item-slot-wrap"
    :class="{ 'has-item': !!item, 'is-pinned': pinned, 'is-rare': rare }"
    @mouseenter="visible = !!item"
    @mouseleave="visible = pinned"
    @focusin="visible = !!item"
    @focusout="visible = pinned"
  >
    <button
      class="item-slot"
      type="button"
      :disabled="!item"
      :aria-label="
        item
          ? `${name}, slot ${slotNumber + 1}${quantity ? `, quantity ${quantity}` : ''}`
          : `Empty slot ${slotNumber + 1}`
      "
      :aria-expanded="item ? visible : undefined"
      @click="togglePinned"
      @keydown.esc="dismiss"
    >
      <img
        v-if="item && icon && !iconFailed"
        :src="icon"
        class="item-icon"
        alt=""
        @error="iconFailed = true"
      />
      <span v-else-if="item" class="item-fallback" aria-hidden="true">
        {{
          stringField(item, 'icon_fallback') || name.slice(0, 1).toUpperCase()
        }}
      </span>
      <span v-if="item && plus !== null && plus > 0" class="item-plus"
        >+{{ plus }}</span
      >
      <span
        v-if="item && quantity !== null && quantity > 1"
        class="item-quantity"
        >{{ quantity.toLocaleString() }}</span
      >
    </button>
    <div v-if="item && visible" class="item-tooltip" role="tooltip">
      <strong class="item-tooltip-name">
        {{ name
        }}<template v-if="plus !== null && plus > 0"> (+{{ plus }})</template>
      </strong>
      <span
        v-if="stringField(detail, 'seal') || stringField(detail, 'seal_type')"
        class="item-tooltip-seal"
      >
        {{ stringField(detail, 'seal') || stringField(detail, 'seal_type') }}
      </span>
      <span
        v-if="stringField(detail, 'sort_type')"
        class="item-tooltip-classification"
        >Sort of item: {{ stringField(detail, 'sort_type') }}</span
      >
      <span
        v-if="stringField(detail, 'mounted_part')"
        class="item-tooltip-classification"
        >Mounting part: {{ stringField(detail, 'mounted_part') }}</span
      >
      <span
        v-if="numberField(detail, 'degree') !== null"
        class="item-tooltip-classification"
        >Degree: {{ numberField(detail, 'degree') }}
        {{ numberField(detail, 'degree') === 1 ? 'degree' : 'degrees' }}</span
      >
      <template v-if="!statLines.length">
        <template
          v-for="key in [
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
          ]"
          :key="key"
        >
          <span
            v-if="numberField(item, key) !== null"
            class="item-tooltip-stat"
          >
            {{ statLabels[key] || key.replaceAll('_', ' ') }}:
            {{ formatNumber(numberField(item, key)!)
            }}<template v-if="numberField(item, `${key}_percent`) !== null">
              (+{{
                formatNumber(numberField(item, `${key}_percent`)!)
              }}%)</template
            >
          </span>
        </template>
      </template>
      <span
        v-for="(stat, index) in statLines.slice(0, 2)"
        :key="`stat-first-${index}`"
        class="item-tooltip-stat"
        >{{ stat.text }}</span
      >
      <span v-if="durabilityLine" class="item-tooltip-stat">
        {{ durabilityLine }}
      </span>
      <span
        v-for="(stat, index) in statLines.slice(2)"
        :key="`stat-next-${index}`"
        class="item-tooltip-stat"
        >{{ stat.text }}</span
      >
      <span
        v-for="(line, index) in rollLines.filter(
          (line) => !line.startsWith('Durability '),
        )"
        :key="`roll-${index}`"
        class="item-tooltip-stat"
        >{{ line }}</span
      >
      <template
        v-for="key in [
          'required_level',
          'required_strength',
          'required_intelligence',
        ]"
        :key="key"
      >
        <span
          v-if="numberField(detail, key) !== null"
          class="item-tooltip-requirement"
          :class="{ 'item-tooltip-level': key === 'required_level' }"
        >
          Required {{ key.replace('required_', '').replaceAll('_', ' ') }}
          {{ numberField(detail, key) }}
        </span>
      </template>
      <template v-for="key in ['required_gender', 'required_race']" :key="key">
        <span v-if="stringField(detail, key)" class="item-tooltip-requirement">
          {{ stringField(detail, key) }}
        </span>
      </template>
      <span
        v-for="(blue, index) in blueLines"
        :key="index"
        class="item-tooltip-blue"
      >
        {{ blue }}
      </span>
      <span
        v-if="quantity !== null && quantity > 1"
        class="item-tooltip-quantity"
        >Quantity: {{ quantity.toLocaleString() }}</span
      >
    </div>
  </div>
</template>
