<script setup lang="ts">
type Slot = {
  source_slot: number
  displayed_slot?: number
  item: Record<string, unknown>
} | null

const props = defineProps<{
  title: string
  observation?: Record<string, unknown>
  availability?: string
  reason?: string
  gold?: number | null
}>()
const page = ref(0)
const pageSize = 32
const columns = 4
const rows = 8
const capacity = computed(() => {
  const value = props.observation?.capacity
  return typeof value === 'number' && Number.isFinite(value) && value >= 0
    ? Math.floor(value)
    : 0
})
const slots = computed<Slot[]>(() =>
  Array.isArray(props.observation?.slots)
    ? (props.observation.slots as Slot[])
    : [],
)
const pageCount = computed(() =>
  Math.max(1, Math.ceil(capacity.value / pageSize)),
)
const used = computed(() => slots.value.filter(Boolean).length)
const pageSlots = computed(() => {
  const start = page.value * pageSize
  return Array.from({ length: rows * columns }, (_, index) => {
    const displaySlot = start + index
    if (displaySlot >= capacity.value) return null
    return (
      slots.value.find(
        (entry) =>
          entry !== null &&
          (entry.displayed_slot ?? entry.source_slot) === displaySlot,
      ) ?? null
    )
  })
})
watch(pageCount, (count) => {
  if (page.value >= count) page.value = Math.max(0, count - 1)
})
const observed = computed(
  () => (props.availability || props.observation?.availability) === 'observed',
)
const hasLastContents = computed(() => Array.isArray(props.observation?.slots))
const displayable = computed(() => observed.value || hasLastContents.value)
const staleContents = computed(() => displayable.value && !observed.value)
const lastObserved = computed(() => {
  const value = props.observation?.observed_at
  return typeof value === 'string' ? formatTimestamp(value) : 'time unavailable'
})
const reasonText = computed(() => {
  if (props.reason) return props.reason.replaceAll('_', ' ')
  return (props.availability || props.observation?.availability) ===
    'not_observed'
    ? 'Open this container in game to collect its contents.'
    : 'This container is not available from the current phBot observation.'
})
</script>

<template>
  <section
    class="inventory-view"
    :class="{ 'is-stale': staleContents }"
    :aria-label="title"
  >
    <div v-if="displayable" class="inventory-topline">
      <h3>{{ title }}</h3>
      <span
        >{{ used }}/{{ capacity }} occupied<span v-if="staleContents">
          · Last observed {{ lastObserved }}</span
        ></span
      >
    </div>
    <div v-if="staleContents" class="inventory-stale" role="status">
      Showing the last confirmed contents from {{ lastObserved }}. phBot
      currently reports this container as {{ availability || 'unavailable' }}.
    </div>
    <div
      v-if="displayable"
      class="inventory-pages"
      role="group"
      :aria-label="`${title} pages`"
    >
      <button
        v-for="number in pageCount"
        :key="number"
        class="compact-button"
        :class="{ selected: page === number - 1 }"
        type="button"
        @click="page = number - 1"
      >
        Page {{ number }}
      </button>
    </div>
    <div
      v-if="displayable && capacity > 0"
      class="inventory-grid"
      :style="{ '--inventory-columns': columns, '--inventory-rows': rows }"
    >
      <ItemSlot
        v-for="(slot, index) in pageSlots"
        :key="`${page}-${index}`"
        :item="slot?.item ?? null"
        :slot-number="page * pageSize + index"
        :label="slot ? `Source slot ${slot.source_slot + 1}` : undefined"
      />
    </div>
    <div v-else-if="displayable" class="inventory-empty">
      No slots reported for this container.
    </div>
    <div v-else class="inventory-unavailable" role="status">
      <strong>{{
        availability === 'not_observed' ? 'Not observed yet' : 'Unavailable'
      }}</strong>
      <span>{{ reasonText }}</span>
    </div>
    <div v-if="displayable" class="inventory-footer">
      <span>{{ used }} / {{ capacity }} occupied</span>
      <span>Page {{ page + 1 }} of {{ pageCount }}</span>
    </div>
    <div
      v-if="displayable && gold !== null && gold !== undefined"
      class="inventory-gold"
    >
      <span aria-hidden="true">◈</span> Gold {{ gold.toLocaleString() }}
    </div>
  </section>
</template>
