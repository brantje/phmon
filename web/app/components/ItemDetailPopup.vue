<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import { buildItemDetail } from '../utils/itemDetailPopup'

const props = withDefaults(
  defineProps<{
    item: Record<string, unknown>
    layout?: 'inline' | 'slot'
    to?: RouteLocationRaw
    slotNumber?: number
    label?: string
  }>(),
  {
    layout: 'inline',
    to: undefined,
    slotNumber: undefined,
    label: undefined,
  },
)

const anchor = ref<HTMLElement | null>(null)
const tooltipEl = ref<HTMLElement | null>(null)
const visible = ref(false)
const placed = ref(false)
const iconFailed = ref(false)
const tooltipStyle = ref<Record<string, string>>({})
const model = computed(() => buildItemDetail(props.item))
const detailProvenance = computed(() => {
  const instance =
    props.item.instance && typeof props.item.instance === 'object'
      ? (props.item.instance as Record<string, unknown>)
      : {}
  const details =
    props.item.instance_details &&
    typeof props.item.instance_details === 'object'
      ? (props.item.instance_details as Record<string, unknown>)
      : {}
  const rawSources = Array.isArray(details.sources)
    ? details.sources
    : typeof details.source === 'string'
      ? [details.source]
      : []
  const sources = rawSources.flatMap((source) => {
    if (source === 'phbot_api') return ['phBot API']
    if (source === 'vsro_1188_packet') return ['Joymax packet']
    return typeof source === 'string' ? [source] : []
  })
  const observationID =
    typeof instance.observation_id === 'string' ? instance.observation_id : ''
  const association =
    instance.association === 'unique_model_inventory_gain'
      ? 'Matched inventory gain; acquisition cause unverified'
      : ''
  if (!sources.length && !observationID && !association) return ''
  return [
    sources.length ? `Detail source: ${[...new Set(sources)].join(' + ')}` : '',
    observationID ? `Packet observation: ${observationID}` : '',
    association,
  ]
    .filter(Boolean)
    .join(' · ')
})
const narrow = () => window.matchMedia('(max-width: 640px)').matches
let listening = false

function place() {
  const trigger = anchor.value
  if (!trigger || narrow()) {
    tooltipStyle.value = {}
    return
  }
  const rect = trigger.getBoundingClientRect()
  const width = tooltipEl.value?.offsetWidth || 270
  const height = tooltipEl.value?.offsetHeight || 0
  let left = rect.right + 7
  if (left + width > window.innerWidth - 8) left = rect.left - 7 - width
  if (left < 8) left = 8
  let top = rect.top - 6
  if (height && top + height > window.innerHeight - 8)
    top = Math.max(8, window.innerHeight - height - 8)
  if (top < 8) top = 8
  tooltipStyle.value = { top: `${top}px`, left: `${left}px` }
}

function bind() {
  if (listening) return
  listening = true
  window.addEventListener('scroll', place, true)
  window.addEventListener('resize', place)
}

function unbind() {
  if (!listening) return
  listening = false
  window.removeEventListener('scroll', place, true)
  window.removeEventListener('resize', place)
}

function open() {
  visible.value = true
  placed.value = narrow()
  nextTick(() => {
    place()
    nextTick(() => {
      place()
      placed.value = true
      bind()
    })
  })
}

function close(event?: FocusEvent) {
  const next = event?.relatedTarget
  if (next instanceof Node && anchor.value?.contains(next)) return
  visible.value = false
  unbind()
}

watch(
  () =>
    `${props.item.model ?? ''}:${props.item.servername ?? ''}:${model.value.icon}:${model.value.name}`,
  () => {
    iconFailed.value = false
    close()
  },
)
onBeforeUnmount(unbind)

const slotLabel = computed(() => {
  const quantity =
    model.value.quantity !== null && model.value.quantity > 1
      ? `, quantity ${model.value.quantity}`
      : ''
  const slot =
    props.slotNumber === undefined ? '' : `, slot ${props.slotNumber + 1}`
  const prefix = props.label ? `${props.label}. ` : ''
  return `${prefix}${model.value.name}${slot}${quantity}`
})
</script>

<template>
  <div
    ref="anchor"
    class="item-detail-popup"
    :class="{
      'item-slot-wrap': layout === 'slot',
      'has-item': true,
      'is-rare': model.rare,
    }"
    @mouseenter="open"
    @mouseleave="close()"
    @focusin="open"
    @focusout="close"
  >
    <button
      v-if="layout === 'slot'"
      class="item-slot"
      type="button"
      :aria-label="slotLabel"
    >
      <img
        v-if="model.icon && !iconFailed"
        :src="model.icon"
        class="item-icon"
        alt=""
        @error="iconFailed = true"
      />
      <span v-else class="item-fallback" aria-hidden="true">{{
        model.iconFallback
      }}</span>
      <span v-if="model.plus !== null && model.plus > 0" class="item-plus"
        >+{{ model.plus }}</span
      >
      <span
        v-if="model.quantity !== null && model.quantity > 1"
        class="item-quantity"
        >{{ model.quantity.toLocaleString() }}</span
      >
    </button>
    <template v-else>
      <img
        v-if="model.icon && !iconFailed"
        :src="model.icon"
        class="item-detail-icon"
        alt=""
        @error="iconFailed = true"
      />
      <span v-else class="item-detail-icon item-fallback" aria-hidden="true">{{
        model.iconFallback
      }}</span>
      <NuxtLink v-if="to" class="item-detail-name" :to="to">
        {{ model.name
        }}<template v-if="model.plus !== null && model.plus > 0">
          (+{{ model.plus }})</template
        >
      </NuxtLink>
      <span v-else class="item-detail-name" tabindex="0">
        {{ model.name
        }}<template v-if="model.plus !== null && model.plus > 0">
          (+{{ model.plus }})</template
        >
      </span>
    </template>
    <Teleport to="body">
      <div
        v-if="visible"
        ref="tooltipEl"
        class="item-tooltip"
        :class="{ 'is-rare': model.rare, 'is-placed': placed }"
        :style="tooltipStyle"
        role="tooltip"
      >
        <strong class="item-tooltip-name">
          {{ model.name
          }}<template v-if="model.plus !== null && model.plus > 0">
            (+{{ model.plus }})</template
          >
        </strong>
        <span v-if="detailProvenance" class="item-tooltip-provenance">
          {{ detailProvenance }}
        </span>
        <span v-if="model.seal" class="item-tooltip-seal">{{
          model.seal
        }}</span>
        <span
          v-for="line in model.classifications"
          :key="line"
          class="item-tooltip-classification"
          >{{ line }}</span
        >
        <span
          v-for="line in model.statsBeforeDurability"
          :key="line"
          class="item-tooltip-stat"
          >{{ line }}</span
        >
        <span v-if="model.durability" class="item-tooltip-stat">{{
          model.durability
        }}</span>
        <span
          v-for="line in model.statsAfterDurability"
          :key="line"
          class="item-tooltip-stat"
          >{{ line }}</span
        >
        <span v-if="model.catalogNote" class="item-tooltip-classification">{{
          model.catalogNote
        }}</span>
        <span
          v-for="requirement in model.requirements"
          :key="requirement.text"
          class="item-tooltip-requirement"
          :class="{ 'item-tooltip-level': requirement.level }"
          >{{ requirement.text }}</span
        >
        <span
          v-for="blue in model.blues"
          :key="blue"
          class="item-tooltip-blue"
          >{{ blue }}</span
        >
        <span
          v-if="model.quantity !== null && model.quantity > 1"
          class="item-tooltip-quantity"
          >Quantity: {{ model.quantity.toLocaleString() }}</span
        >
      </div>
    </Teleport>
  </div>
</template>
