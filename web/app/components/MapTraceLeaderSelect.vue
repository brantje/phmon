<script setup lang="ts">
import { computed } from 'vue'
import type { NearbyPlayersStatus } from '~/utils/mapTracePicker'
import { useNativeSelectRenderLock } from '~/composables/useNativeSelectRenderLock'

export interface TraceLeaderOption {
  value: string
  label: string
  group: 'managed' | 'nearby'
}

const model = defineModel<string>({ default: '' })

const emit = defineEmits<{
  interacting: [open: boolean]
}>()

const props = defineProps<{
  managed: TraceLeaderOption[]
  nearby: TraceLeaderOption[]
  nearbyStatus: NearbyPlayersStatus
}>()

const optionSnapshot = () => ({
  managed: props.managed,
  nearby: props.nearby,
  nearbyStatus: props.nearbyStatus,
})

const {
  display: lockedOptions,
  lock,
  onPointerDown,
  onFocus,
  onChange,
  onBlur,
  selectRef,
} = useNativeSelectRenderLock(optionSnapshot)

function handlePointerDown() {
  onPointerDown()
  emit('interacting', true)
}

function handleFocus() {
  onFocus()
  emit('interacting', true)
}

function handleChange() {
  onChange()
  emit('interacting', false)
}

function handleBlur() {
  onBlur()
  queueMicrotask(() => {
    if (!selectRef.value?.matches(':focus')) emit('interacting', false)
  })
}

const displayManaged = computed(() => lockedOptions.value.managed)
const displayNearby = computed(() => lockedOptions.value.nearby)

const orphanTraceName = computed(() => {
  const name = model.value
  if (!name) return false
  const options = [...displayManaged.value, ...displayNearby.value]
  return !options.some((item) => item.value === name)
})
</script>

<template>
  <select
    ref="selectRef"
    v-model="model"
    aria-label="Trace leader"
    @pointerdown="handlePointerDown"
    @focus="handleFocus"
    @mousedown="lock"
    @change="handleChange"
    @blur="handleBlur"
  >
    <option value="">Trace player…</option>
    <optgroup label="Managed characters">
      <option
        v-for="option in displayManaged"
        :key="`managed-${option.value}`"
        :value="option.value"
      >
        {{ option.label }}
      </option>
    </optgroup>
    <optgroup label="Nearby players">
      <option
        v-for="option in displayNearby"
        :key="`nearby-${option.value}`"
        :value="option.value"
      >
        {{ option.label }}
      </option>
    </optgroup>
    <option v-if="orphanTraceName" :value="model">
      {{ model }}
    </option>
  </select>
</template>
