<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'

const props = defineProps<{
  character: CharacterView
  selected: boolean
  targeted: boolean
  positionFresh: boolean
}>()

const emit = defineEmits<{
  focus: []
  toggleTarget: []
}>()

function resourcePercent(current?: number, maximum?: number) {
  if (
    current == null ||
    maximum == null ||
    !Number.isFinite(current) ||
    !Number.isFinite(maximum) ||
    maximum <= 0
  )
    return null
  return Math.max(0, Math.min(100, (current / maximum) * 100))
}

function resourceLabel(
  label: 'HP' | 'MP',
  current?: number,
  maximum?: number,
) {
  if (current == null || maximum == null) return `${label} — / —`
  return `${label} ${current.toLocaleString()} / ${maximum.toLocaleString()}`
}

const hpPercent = computed(() =>
  resourcePercent(props.character.hp, props.character.hp_max),
)
const mpPercent = computed(() =>
  resourcePercent(props.character.mp, props.character.mp_max),
)
const statusLabel = computed(() => {
  if (!props.character.online) return 'Offline · last position'
  return props.positionFresh ? 'Online' : 'Online · last observed position'
})
</script>

<template>
  <div
    class="map-character-status-row"
    :class="{ selected, targeted }"
  >
    <label class="map-character-status-target">
      <input
        type="checkbox"
        :checked="targeted"
        :aria-label="`Target ${character.name} for actions`"
        @change="emit('toggleTarget')"
      />
    </label>

    <button
      class="map-character-status-focus"
      type="button"
      :aria-pressed="selected"
      @click="emit('focus')"
    >
      <span class="map-character-status-heading">
        <span class="map-character-status-identity">
          <strong>{{ character.name }}</strong>
          <small v-if="character.level != null" class="map-character-level">
            Lv. {{ character.level }}
          </small>
        </span>
        <small class="map-character-presence">{{ statusLabel }}</small>
      </span>

      <span class="map-character-resource hp">
        <span
          class="map-character-resource-fill"
          :style="{ width: `${hpPercent ?? 0}%` }"
        />
        <span class="map-character-resource-label">
          {{ resourceLabel('HP', character.hp, character.hp_max) }}
        </span>
      </span>

      <span class="map-character-resource mp">
        <span
          class="map-character-resource-fill"
          :style="{ width: `${mpPercent ?? 0}%` }"
        />
        <span class="map-character-resource-label">
          {{ resourceLabel('MP', character.mp, character.mp_max) }}
        </span>
      </span>

      <small class="map-character-location">
        {{ character.server }} · {{ character.zone || 'Unknown zone' }}
      </small>
    </button>
  </div>
</template>

<style scoped>
.map-character-status-row {
  display: grid;
  grid-template-columns: 1.35rem minmax(0, 1fr);
  gap: 0.35rem;
  padding: 0.5rem 0.3rem;
  border-top: 1px solid rgba(50, 66, 87, 0.7);
  color: #e0e8f4;
}

.map-character-status-row.selected {
  background: rgba(86, 125, 171, 0.15);
}

.map-character-status-row.targeted {
  border-left: 2px solid rgba(92, 143, 205, 0.72);
  padding-left: calc(0.3rem - 2px);
}

.map-character-status-row:hover,
.map-character-status-row:focus-within {
  background: rgba(96, 132, 178, 0.12);
}

.map-character-status-target {
  display: flex;
  align-items: flex-start;
  justify-content: center;
  padding-top: 0.15rem;
}

.map-character-status-target input {
  width: 1rem;
  height: 1rem;
  margin: 0;
  accent-color: #2d75c7;
}

.map-character-status-focus {
  display: grid;
  width: 100%;
  min-width: 0;
  gap: 0.24rem;
  padding: 0;
  border: 0;
  background: transparent;
  color: inherit;
  font: inherit;
  text-align: left;
  cursor: pointer;
}

.map-character-status-focus:focus-visible {
  outline: 1px solid #6f9bce;
  outline-offset: 2px;
}

.map-character-status-heading {
  display: flex;
  min-width: 0;
  align-items: baseline;
  justify-content: space-between;
  gap: 0.55rem;
}

.map-character-status-identity {
  display: flex;
  min-width: 0;
  align-items: baseline;
  gap: 0.35rem;
}

.map-character-status-identity strong {
  overflow: hidden;
  color: #eaf1ff;
  font-size: 0.76rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.map-character-level {
  flex: 0 0 auto;
  color: #e0a44f;
  font-size: 0.61rem;
}

.map-character-presence {
  flex: 0 1 auto;
  color: #a9bad0;
  font-size: 0.64rem;
  text-align: right;
}

.map-character-resource {
  position: relative;
  display: block;
  height: 0.72rem;
  overflow: hidden;
  border: 1px solid rgba(14, 21, 31, 0.85);
  border-radius: 3px;
  background: rgba(5, 9, 15, 0.8);
}

.map-character-resource-fill {
  position: absolute;
  inset: 0 auto 0 0;
  min-width: 0;
  transition: width 180ms ease-out;
}

.map-character-resource.hp .map-character-resource-fill {
  background: #d92332;
}

.map-character-resource.mp .map-character-resource-fill {
  background: #3159cb;
}

.map-character-resource-label {
  position: relative;
  z-index: 1;
  display: flex;
  height: 100%;
  align-items: center;
  justify-content: center;
  padding: 0 0.2rem;
  color: #e9eff8;
  font-size: 0.55rem;
  line-height: 1;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.95);
  white-space: nowrap;
}

.map-character-location {
  overflow: hidden;
  margin-top: 0.05rem;
  color: #91a0b4;
  font-size: 0.64rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}
</style>
