<script setup lang="ts">
defineProps<{
  item: Record<string, unknown> | null
  slotNumber: number
  label?: string
  unknown?: boolean
}>()
</script>

<template>
  <ItemDetailPopup
    v-if="item"
    layout="slot"
    :item="item"
    :slot-number="slotNumber"
    :label="label"
  />
  <div v-else class="item-slot-wrap">
    <button
      class="item-slot"
      type="button"
      disabled
      :aria-label="
        unknown
          ? `Unknown ${label || `slot ${slotNumber + 1}`}`
          : `Empty slot ${slotNumber + 1}`
      "
      :title="
        unknown
          ? 'Equipment in this slot has not been observed'
          : 'Observed empty slot'
      "
    >
      <span v-if="unknown" aria-hidden="true">?</span>
    </button>
  </div>
</template>
