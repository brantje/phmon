<script setup lang="ts">
defineProps<{
  item: Record<string, unknown> | null
  slotNumber: number
  label?: string
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
      :aria-label="`Empty slot ${slotNumber + 1}`"
    />
  </div>
</template>
