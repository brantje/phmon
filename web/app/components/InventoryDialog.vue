<script setup lang="ts">
import {
  containerDialogTargetKey,
  useContainerDialog,
  type ContainerDialogTarget,
} from '~/composables/useContainerDialog'

const props = defineProps<{ target: ContainerDialogTarget }>()
const emit = defineEmits<{ close: [] }>()
const target = toRef(props, 'target')
const {
  resource,
  status,
  observation,
  gold,
  occupied,
  capacity,
  stale,
  retry,
} = useContainerDialog(target)
const dialogElement = ref<HTMLElement | null>(null)
const closeButton = ref<HTMLButtonElement | null>(null)
const titleID = `inventory-dialog-${useId().replace(/[^a-zA-Z0-9_-]/g, '')}`
const targetKey = computed(() => containerDialogTargetKey(props.target))
const container = computed(() =>
  props.target.kind === 'guild' ? 'guild_storage' : props.target.container,
)
const title = computed(() =>
  props.target.kind === 'guild'
    ? 'Guild Storage'
    : props.target.container === 'storage'
      ? 'Storage'
      : 'Inventory',
)
const owner = computed(() =>
  props.target.kind === 'guild'
    ? props.target.guild
    : props.target.characterName,
)
const lastSynced = computed(() =>
  resource.value?.observed_at ? formatDate(resource.value.observed_at) : null,
)
const slotSummary = computed(() =>
  occupied.value !== null && capacity.value !== null
    ? `${occupied.value}/${capacity.value} slots`
    : null,
)
const hasContents = computed(() => Array.isArray(observation.value.slots))

function handleKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    emit('close')
    return
  }
  if (event.key !== 'Tab' || !dialogElement.value) return
  const focusable = Array.from(
    dialogElement.value.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  )
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (!first || !last) {
    event.preventDefault()
    return
  }
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first.focus()
  }
}

onMounted(() => {
  void nextTick(() => closeButton.value?.focus())
})
</script>

<template>
  <Teleport to="#teleports">
    <div
      class="dialog-layer inventory-dialog-layer"
      role="presentation"
      @click.self="emit('close')"
    >
      <section
        ref="dialogElement"
        class="inventory-dialog"
        role="dialog"
        aria-modal="true"
        :aria-labelledby="titleID"
        :aria-busy="status === 'loading'"
        @keydown="handleKeydown"
      >
        <div class="dialog-header">
          <div>
            <h2 :id="titleID">{{ title }}</h2>
            <p>
              Owner: {{ owner
              }}<template v-if="slotSummary"> · {{ slotSummary }}</template
              ><template v-if="lastSynced">
                · Last synced: {{ lastSynced }}</template
              ><template
                v-if="target.kind === 'guild' && resource?.observer_name"
              >
                · Viewed by: {{ resource.observer_name }}</template
              >
            </p>
          </div>
          <button
            ref="closeButton"
            class="icon-button"
            type="button"
            aria-label="Close"
            @click="emit('close')"
          >
            <UIcon name="i-lucide-x" />
          </button>
        </div>
        <p
          v-if="status === 'loading' && !hasContents"
          class="inventory-dialog-status"
          role="status"
        >
          Loading {{ title.toLocaleLowerCase() }}…
        </p>
        <p
          v-else-if="status === 'unavailable'"
          class="status-banner warning"
          role="alert"
        >
          {{ title }} could not be loaded.
          <button type="button" class="compact-button" @click="retry">
            Retry
          </button>
        </p>
        <p
          v-if="stale && hasContents"
          class="status-banner warning"
          role="status"
        >
          Disconnected · showing last synced contents.
        </p>
        <InventoryGrid
          v-if="status === 'current' || hasContents"
          variant="dialog"
          :title="`${title} | ${owner}`"
          :container="container"
          :reset-key="targetKey"
          :observation="observation"
          :availability="observation.availability"
          :gold="gold"
        />
      </section>
    </div>
  </Teleport>
</template>
