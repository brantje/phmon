<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'
const props = defineProps<{ character: CharacterView }>()
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

function resourceLabel(label: 'HP' | 'MP', current?: number, maximum?: number) {
  if (
    current == null ||
    maximum == null ||
    !Number.isFinite(current) ||
    !Number.isFinite(maximum)
  )
    return `${label} — / —`
  return `${label} ${current.toLocaleString()} / ${maximum.toLocaleString()}`
}

const hpPercent = computed(() =>
  resourcePercent(props.character.hp, props.character.hp_max),
)
const mpPercent = computed(() =>
  resourcePercent(props.character.mp, props.character.mp_max),
)
</script>
<template>
  <div class="map-resource-bars">
    <span
      class="map-character-resource hp"
      role="progressbar"
      aria-label="Character health"
      :aria-valuemin="0"
      :aria-valuemax="hpPercent == null ? undefined : character.hp_max"
      :aria-valuenow="
        hpPercent == null
          ? undefined
          : Math.max(0, Math.min(character.hp!, character.hp_max!))
      "
      :aria-valuetext="resourceLabel('HP', character.hp, character.hp_max)"
    >
      <span
        class="map-character-resource-fill"
        :style="{ width: `${hpPercent ?? 0}%` }"
      />
      <span class="map-character-resource-label">
        {{ resourceLabel('HP', character.hp, character.hp_max) }}
      </span>
    </span>

    <span
      class="map-character-resource mp"
      role="progressbar"
      aria-label="Character mana"
      :aria-valuemin="0"
      :aria-valuemax="mpPercent == null ? undefined : character.mp_max"
      :aria-valuenow="
        mpPercent == null
          ? undefined
          : Math.max(0, Math.min(character.mp!, character.mp_max!))
      "
      :aria-valuetext="resourceLabel('MP', character.mp, character.mp_max)"
    >
      <span
        class="map-character-resource-fill"
        :style="{ width: `${mpPercent ?? 0}%` }"
      />
      <span class="map-character-resource-label">
        {{ resourceLabel('MP', character.mp, character.mp_max) }}
      </span>
    </span>
  </div>
</template>
<style scoped>
.map-resource-bars {
  display: grid;
  gap: 3px;
  width: 100%;
}
.map-character-resource {
  position: relative;
  display: block;
  height: 1rem;
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
  font-size: 0.8rem;
  line-height: 1;
  text-shadow: 0 1px 1px rgba(0, 0, 0, 0.95);
  white-space: nowrap;
}

.map-character-resource {
  height: 14px;
}
.map-character-resource-label {
  font-size: 11px;
}
</style>
