<script setup lang="ts">
import type { RouteLocationRaw } from 'vue-router'
import { formatUnsignedCount } from '../utils/item-instance-format'
import {
  buildItemDetail,
  itemTooltipPresentation,
} from '../utils/itemDetailPopup'

const props = withDefaults(
  defineProps<{
    item: Record<string, unknown>
    layout?: 'inline' | 'slot' | 'tile'
    to?: RouteLocationRaw
    slotNumber?: number
    label?: string
    subtitle?: string
    quantityExact?: string
  }>(),
  {
    layout: 'inline',
    to: undefined,
    slotNumber: undefined,
    label: undefined,
    subtitle: undefined,
    quantityExact: undefined,
  },
)

const anchor = ref<HTMLElement | null>(null)
const tooltipEl = ref<HTMLElement | null>(null)
const tooltipID = `item-tooltip-${useId()}`
const visible = ref(false)
const placed = ref(false)
const iconFailed = ref(false)
const tooltipStyle = ref<Record<string, string>>({})
const model = computed(() => buildItemDetail(props.item))
const presentation = computed(() => itemTooltipPresentation(props.item))
const quantityLabel = computed(() => {
  const value = formatUnsignedCount(props.quantityExact ?? model.value.quantity)
  return value && value !== '0' && value !== '1' ? value : null
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
  document.addEventListener('pointerdown', closeOutside)
}

function unbind() {
  if (!listening) return
  listening = false
  window.removeEventListener('scroll', place, true)
  window.removeEventListener('resize', place)
  document.removeEventListener('pointerdown', closeOutside)
}

function closeOutside(event: PointerEvent) {
  if (event.target instanceof Node && anchor.value?.contains(event.target))
    return
  hide()
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

function hide() {
  visible.value = false
  unbind()
}

// Escape closes an open tooltip first; otherwise it reaches enclosing dialogs.
function dismiss(event: KeyboardEvent) {
  if (!visible.value) return
  event.stopPropagation()
  hide()
}

function close(event?: FocusEvent) {
  const next = event?.relatedTarget
  if (next instanceof Node && anchor.value?.contains(next)) return
  if (!event && anchor.value?.contains(document.activeElement)) return
  hide()
}

watch(presentation, () => {
  if (visible.value) nextTick(place)
})
watch(
  () =>
    `${props.item.model ?? ''}:${props.item.servername ?? ''}:${model.value.icon}:${model.value.name}`,
  () => {
    iconFailed.value = false
    hide()
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
      'item-search-tile-wrap': layout === 'tile',
      'has-item': true,
      'is-rare': model.rare,
      'has-blue-options': model.blues.length > 0,
    }"
    @mouseenter="open"
    @mouseleave="close()"
    @focusin="open"
    @focusout="close"
    @keydown.esc="dismiss"
  >
    <button
      v-if="layout === 'slot' || layout === 'tile'"
      :class="layout === 'tile' ? 'item-search-tile' : 'item-slot'"
      type="button"
      :aria-label="
        layout === 'tile'
          ? `Show details for ${model.name}, ${subtitle}${quantityLabel ? `, quantity ${quantityLabel}` : ''}`
          : slotLabel
      "
      :aria-describedby="visible ? tooltipID : undefined"
      @click="open"
    >
      <span
        :class="layout === 'tile' ? 'item-search-tile-icon' : 'item-slot-image'"
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
          v-if="quantityLabel"
          class="item-quantity"
          :title="`Quantity: ${quantityLabel}`"
          >{{ quantityLabel }}</span
        >
      </span>
      <span v-if="layout === 'tile'" class="item-search-tile-text">
        <span class="item-detail-name">{{ model.name }}</span>
        <span class="item-search-tile-subtitle">{{ subtitle }}</span>
      </span>
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
      <template v-if="to">
        <NuxtLink
          class="item-detail-name"
          :to="to"
          :aria-describedby="visible ? tooltipID : undefined"
        >
          {{ model.name
          }}<template v-if="model.plus !== null && model.plus > 0">
            (+{{ model.plus }})</template
          >
        </NuxtLink>
        <button
          class="item-detail-touch-trigger"
          type="button"
          :aria-label="`Show details for ${model.name}`"
          :aria-describedby="visible ? tooltipID : undefined"
          @click.stop="open"
        >
          <UIcon name="i-lucide-info" aria-hidden="true" />
        </button>
      </template>
      <button
        v-else
        class="item-detail-name"
        type="button"
        :aria-label="`Show details for ${model.name}`"
        :aria-describedby="visible ? tooltipID : undefined"
        @click="open"
      >
        {{ model.name
        }}<template v-if="model.plus !== null && model.plus > 0">
          (+{{ model.plus }})</template
        >
      </button>
    </template>
    <Teleport to="body">
      <div
        v-if="visible"
        :id="tooltipID"
        ref="tooltipEl"
        class="item-tooltip"
        :class="{ 'is-rare': model.rare, 'is-placed': placed }"
        :style="tooltipStyle"
        role="tooltip"
      >
        <ItemTooltipDetails :model="presentation.model" />
        <span
          v-if="presentation.caption"
          class="item-tooltip-caption"
          :title="presentation.sourceDescription"
          >{{ presentation.caption }}</span
        >
      </div>
    </Teleport>
  </div>
</template>
