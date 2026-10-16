<script setup lang="ts">
import type { ItemDetailModel } from '../utils/itemDetailPopup'
import { itemTooltipStatLines } from '../utils/itemDetailPopup'

defineProps<{ model: ItemDetailModel }>()
</script>

<template>
  <strong class="item-tooltip-name">
    {{ model.name
    }}<template v-if="model.plus !== null && model.plus > 0">
      (+{{ model.plus }})</template
    >
  </strong>
  <div class="item-tooltip-identity">
    <span v-if="model.seal" class="item-tooltip-seal">{{ model.seal }}</span>
    <span
      v-for="line in model.classifications"
      :key="line"
      class="item-tooltip-classification"
      >{{ line }}</span
    >
  </div>
  <div v-if="itemTooltipStatLines(model).length" class="item-tooltip-stats">
    <span
      v-for="line in itemTooltipStatLines(model)"
      :key="line"
      class="item-tooltip-stat"
      >{{ line }}</span
    >
  </div>
  <div v-if="model.requirements.length" class="item-tooltip-requirements">
    <span
      v-for="requirement in model.requirements"
      :key="requirement.text"
      class="item-tooltip-requirement"
      >{{ requirement.text }}</span
    >
  </div>
  <div v-if="model.blues.length" class="item-tooltip-blues">
    <span v-for="blue in model.blues" :key="blue" class="item-tooltip-blue">{{
      blue
    }}</span>
  </div>
  <div v-if="model.notes.length" class="item-tooltip-notes">
    <span v-for="note in model.notes" :key="note" class="item-tooltip-stat">{{
      note
    }}</span>
  </div>
  <span
    v-if="model.quantity !== null && model.quantity > 1"
    class="item-tooltip-quantity"
    >Quantity: {{ model.quantity.toLocaleString() }}</span
  >
</template>
