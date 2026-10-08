<script setup lang="ts">
import type { PlayerEquipment } from '~~/shared/types/players'
import { playerSlots, equipmentLabel } from '~/utils/playerRegistry'
const props = defineProps<{ equipment: PlayerEquipment | null }>()
const slots = computed(() =>
  playerSlots.map((name) => ({
    name,
    observed: props.equipment?.slots.find((slot) => slot.slot === name),
  })),
)
const slotLabel = (name: string) => name.replaceAll('_', ' ')
</script>
<template>
  <div class="player-equipment">
    <p class="player-muted">
      {{ equipmentLabel(equipment)
      }}<template v-if="equipment">
        · {{ equipment.source }} ·
        <time :datetime="equipment.observed_at">{{
          new Date(equipment.observed_at).toLocaleString()
        }}</time></template
      >
    </p>
    <p
      v-if="equipment?.last_availability === 'unavailable'"
      class="player-muted"
    >
      The latest observation could not inspect equipment. Last known slots
      retain their original observation times.
    </p>
    <p v-if="!equipment" class="player-muted">
      Equipment has not been observed. Unknown slots and item statistics remain
      unavailable.
    </p>
    <div class="player-equipment-grid">
      <div
        v-for="(slot, index) in slots"
        :key="slot.name"
        class="player-equipment-slot"
      >
        <span>{{ slotLabel(slot.name) }}</span>
        <ItemSlot
          :slot-number="index"
          :label="slotLabel(slot.name)"
          :unknown="!slot.observed || slot.observed.state === 'unknown'"
          :item="
            slot.observed?.state === 'occupied'
              ? slot.observed.item || {
                  model: slot.observed.model_id,
                  plus: slot.observed.plus,
                }
              : null
          "
        />
        <small>{{
          !slot.observed || slot.observed.state === 'unknown'
            ? 'Unknown'
            : slot.observed.state === 'empty'
              ? 'Empty'
              : slot.observed.plus == null
                ? 'Enhancement unknown'
                : `+${slot.observed.plus}`
        }}</small>
        <small
          v-if="slot.observed?.state === 'occupied'"
          :title="new Date(slot.observed.observed_at).toLocaleString()"
          >Observed
          {{ new Date(slot.observed.observed_at).toLocaleDateString() }}</small
        >
        <details
          v-if="slot.observed?.state === 'occupied'"
          class="player-raw-stats"
        >
          <summary>Observed values</summary>
          <dl>
            <dt>Model</dt>
            <dd>{{ slot.observed.model_id }}</dd>
            <dt>Variance</dt>
            <dd>
              {{ slot.observed.variance ?? 'Unavailable'
              }}<small
                v-if="
                  slot.observed.variance && slot.observed.field_times?.variance
                "
                >Observed
                {{
                  new Date(slot.observed.field_times.variance).toLocaleString()
                }}</small
              >
            </dd>
            <dt>Durability</dt>
            <dd>{{ slot.observed.durability ?? 'Unavailable' }}</dd>
            <dt>Magic options</dt>
            <dd>
              <span v-if="!slot.observed.magic_options">Unavailable</span
              ><span
                v-else-if="!Object.keys(slot.observed.magic_options).length"
                >Observed empty</span
              ><span
                v-for="(value, key) in slot.observed.magic_options"
                v-else
                :key="key"
                >{{ key }}: {{ value }}<br
              /></span>
            </dd>
          </dl>
        </details>
      </div>
    </div>
  </div>
</template>
