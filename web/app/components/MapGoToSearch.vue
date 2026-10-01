<script setup lang="ts">
export interface GoToOption {
  id: string
  label: string
  meta: string
  group: 'Characters' | 'Places'
}
const props = defineProps<{ options: GoToOption[] }>()
const emit = defineEmits<{ choose: [id: string] }>()
const query = ref('')
const open = ref(false)
const active = ref(0)
const root = ref<HTMLElement | null>(null)
const filtered = computed(() =>
  props.options.filter((option) =>
    `${option.label} ${option.meta}`
      .toLocaleLowerCase()
      .includes(query.value.trim().toLocaleLowerCase()),
  ),
)
watch(
  query,
  () => {
    active.value = 0
    open.value = true
  },
  { flush: 'sync' },
)
watch(filtered, () => {
  active.value = Math.min(active.value, Math.max(0, filtered.value.length - 1))
})
function choose(option: GoToOption) {
  emit('choose', option.id)
  query.value = option.label
  open.value = false
}
function keydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    open.value = false
    return
  }
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    if (!open.value) {
      open.value = true
      active.value = 0
    } else
      active.value =
        (active.value +
          (event.key === 'ArrowDown' ? 1 : -1) +
          Math.max(1, filtered.value.length)) %
        Math.max(1, filtered.value.length)
    nextTick(() =>
      document
        .getElementById(`map-go-to-${active.value}`)
        ?.scrollIntoView({ block: 'nearest' }),
    )
  }
  if (event.key === 'Enter' && open.value) {
    event.preventDefault()
    const option = filtered.value[active.value]
    if (option) choose(option)
  }
}
function focusout(event: FocusEvent) {
  if (!root.value?.contains(event.relatedTarget as Node | null))
    open.value = false
}
</script>
<template>
  <div ref="root" class="map-go-to" @focusout="focusout">
    <input
      v-model="query"
      aria-label="Go to character or place"
      placeholder="Go to character or place"
      role="combobox"
      aria-autocomplete="list"
      aria-controls="map-go-to-list"
      :aria-expanded="open"
      :aria-activedescendant="
        open && filtered.length ? `map-go-to-${active}` : undefined
      "
      @focus="open = true"
      @keydown="keydown"
    />
    <div
      v-if="open"
      id="map-go-to-list"
      class="panel map-go-to-list"
      role="listbox"
      aria-label="Characters and places"
    >
      <template v-for="(option, index) in filtered" :key="option.id">
        <div
          v-if="index === 0 || option.group !== filtered[index - 1]?.group"
          class="map-list-heading"
        >
          {{ option.group }}
        </div>
        <button
          :id="`map-go-to-${index}`"
          class="compact-button map-navigation-route-row"
          :class="{ selected: active === index }"
          type="button"
          role="option"
          :aria-selected="active === index"
          tabindex="-1"
          @pointerdown.prevent
          @click="choose(option)"
        >
          <strong>{{ option.label }}</strong
          ><small>{{ option.meta }}</small>
        </button>
      </template>
      <p v-if="!filtered.length" class="map-empty-copy">No matches</p>
    </div>
  </div>
</template>
<style scoped>
.map-go-to {
  position: relative;
  flex: 1 1 220px;
  max-width: 320px;
}
.map-go-to input {
  width: 100%;
}
.map-go-to-list {
  position: absolute;
  top: 100%;
  right: 0;
  left: 0;
  z-index: 1100;
  max-height: 340px;
  overflow: auto;
  padding: 8px;
}
.map-go-to-list button {
  display: flex;
  flex-direction: column;
  width: 100%;
  align-items: flex-start;
}
</style>
