<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'

const props = defineProps<{
  character: CharacterView
  selected: boolean
  targeted: boolean
  positionFresh: boolean
  now: number
  activity?: string
}>()

const emit = defineEmits<{
  focus: []
  toggleTarget: []
}>()

const statusLabel = computed(() =>
  !props.character.online
    ? 'Offline'
    : props.character.botting === true
      ? 'Bot on'
      : props.character.botting === false
        ? 'Bot off'
        : 'Bot unknown',
)
const staleLabel = computed(() => {
  const at = Date.parse(props.character.state_updated_at || '')
  return Number.isFinite(at)
    ? `Stale ${Math.max(0, Math.floor((props.now - at) / 1000))}s`
    : 'Stale'
})
</script>

<template>
  <div
    class="map-character-status-row"
    :class="{ selected, targeted, offline: !character.online }"
  >
    <label class="map-character-status-target">
      <input
        type="checkbox"
        :checked="targeted"
        :disabled="!character.online"
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
        <small class="map-character-presence"
          ><span v-if="character.dead === true">Dead · </span
          ><span v-else-if="character.online && !positionFresh"
            >{{ staleLabel }} · </span
          >{{ statusLabel }}</small
        >
      </span>

      <MapCharacterResources v-if="character.online" :character="character" />

      <small class="map-character-location">
        {{ character.online ? character.server : 'Last seen' }} ·
        {{ character.zone || 'Unknown zone'
        }}{{ activity ? ` · ${activity}` : '' }}
      </small>
    </button>
  </div>
</template>

<style scoped>
.map-character-status-row {
  display: grid;
  grid-template-columns: 1.35rem minmax(0, 1fr);
  gap: 0.35rem;
  padding: 0.62rem 0.3rem;
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
  font-size: 0.61rem;
}

.map-character-presence {
  flex: 0 1 auto;
  color: #a9bad0;
  font-size: 0.64rem;
  text-align: right;
}

.map-character-location {
  overflow: hidden;
  margin-top: 0.05rem;
  color: #91a0b4;
  font-size: 0.64rem;
  text-overflow: ellipsis;
  white-space: nowrap;
}

.map-character-status-row {
  grid-template-columns: 18px minmax(0, 1fr);
  gap: 8px;
  padding: 12px 2px;
  border-top: 0;
}
.map-character-status-row.offline {
  opacity: 0.5;
}
.map-character-status-target input {
  width: 16px;
  height: 16px;
}
.map-character-status-focus {
  gap: 4px;
}
.map-character-status-identity strong {
  font-size: 13px;
}
.map-character-level,
.map-character-presence,
.map-character-location {
  font-size: 11px;
}
.map-character-status-heading {
  gap: 4px;
}
.map-character-presence {
  white-space: nowrap;
}

.map-character-status-target input {
  appearance: none;
  display: grid;
  place-items: center;
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: var(--ph-panel);
}
.map-character-status-target input:checked {
  border-color: var(--ph-blue);
  background: var(--ph-active);
}
.map-character-status-target input:checked::after {
  content: '✓';
  color: var(--ph-blue);
  font-size: 12px;
  line-height: 1;
}
.map-character-status-target input:disabled {
  opacity: 0.5;
}
</style>
