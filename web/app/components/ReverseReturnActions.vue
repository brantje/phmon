<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'
import type { FanOutOperation } from '~/utils/commandFanOut'
import {
  namedLocationReason,
  reverseReturnModes,
  reverseReturnPartyNames,
  reverseReturnNamedLocations,
} from '~/utils/reverseReturn'

const props = withDefaults(
  defineProps<{
    selectedIds: string[]
    scopeKey: string
    scopeKeyForCharacter(character: CharacterView, scopeKey: string): string
    currentScopeKey(characterID: string): string
    currentCharacter(characterID: string): CharacterView | undefined
    presentation?: 'panel' | 'menu'
    menuVisible?: boolean
    mapSnapshotCurrent?: boolean
  }>(),
  { presentation: 'panel', menuVisible: true, mapSnapshotCurrent: true },
)
const emit = defineEmits<{
  chosen: []
  keepMenu: []
  leaveMenu: [event: MouseEvent]
  closeMenu: []
}>()
const actions = useRemoteControlActions({
  scopeKey: () => props.scopeKey,
  scopeKeyForCharacter: props.scopeKeyForCharacter,
  currentScopeKey: props.currentScopeKey,
  currentCharacter: props.currentCharacter,
  mapSnapshotCurrent: () => props.mapSnapshotCurrent,
})
const reviewID = ref('')
const review = computed(() =>
  actions.operations.value.find(
    (op) => op.operationID === reviewID.value && op.state === 'prepared',
  ),
)
const now = ref(Date.now())
let clock: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  window.addEventListener('pointerdown', closePartyOutside)
  clock = setInterval(() => {
    now.value = Date.now()
  }, 5000)
})
onBeforeUnmount(() => {
  window.removeEventListener('pointerdown', closePartyOutside)
  if (clock) clearInterval(clock)
})
const partyNames = computed(() =>
  reverseReturnPartyNames(
    props.selectedIds.map((id) => actions.feed.value?.targets[id]?.controls),
    now.value,
  ),
)
const locationNames = computed(() =>
  reverseReturnNamedLocations(
    props.selectedIds.map((id) => actions.feed.value?.targets[id]?.controls),
  ),
)
const choiceType = ref<2 | 3>(2)
const choiceNames = computed(() =>
  choiceType.value === 2 ? partyNames.value : locationNames.value,
)
const rootElement = ref<HTMLElement | null>(null)
const partyElement = ref<HTMLElement | null>(null)
const partyOpen = ref(false)
const partyAnchor = ref({ x: 0, y: 0 })
const reviewElement = ref<HTMLElement | null>(null)
let returnFocus: HTMLElement | null = null
let sequence = 0
const notice = ref('')
const resultOperations = computed(() =>
  actions.operations.value.filter(
    (op) => !['prepared', 'cancelled'].includes(op.state),
  ),
)
const targetSignature = computed(() =>
  JSON.stringify([
    props.scopeKey,
    [...props.selectedIds].sort(),
    props.selectedIds.map((id) => props.currentCharacter(id)?.session_id),
  ]),
)
watch(
  () => props.selectedIds.join('\0'),
  () => actions.setTargets(props.selectedIds),
  { immediate: true },
)
watch(targetSignature, () => {
  sequence++
  cancel()
  partyOpen.value = false
})

function closePartyOutside(event: PointerEvent) {
  const target = event.target as Node | null
  if (
    target &&
    !rootElement.value?.contains(target) &&
    !partyElement.value?.contains(target)
  )
    partyOpen.value = false
}
watch(
  () => props.menuVisible,
  (visible) => {
    if (!visible) partyOpen.value = false
  },
)
function restoreFocus() {
  if (returnFocus?.getClientRects().length) returnFocus.focus()
  else document.querySelector<HTMLElement>('.map-canvas')?.focus()
}
function cancel(shouldRestore = true) {
  const operation = review.value
  if (operation) {
    actions.cancel(operation)
    actions.dismiss(operation)
  }
  reviewID.value = ''
  if (operation && shouldRestore) void nextTick(restoreFocus)
}
function modeReason(type: number) {
  if (!props.selectedIds.length) return 'Select characters in the panel first.'
  if (!actions.controlsCurrent.value)
    return 'Waiting for current character controls.'
  const mode = ['last_return', 'last_death', 'party_member', 'named_location'][
    type
  ]
  if (
    !props.selectedIds.some((id) => {
      const capability =
        actions.feed.value?.targets[id]?.controls?.capabilities[
          'character.reverse_return'
        ]
      return capability?.supported && capability.modes?.includes(mode!)
    })
  )
    return 'No selected character supports this mode.'
  if (type === 2 && !partyNames.value.length)
    return 'No fresh party member names are available.'
  if (type === 3 && !locationNames.value.length) return namedLocationReason
  return ''
}
function openParty(event: Event, type = 2) {
  if (type !== 2 && type !== 3) return
  emit('keepMenu')
  const element = event.currentTarget as HTMLElement
  const rect = element.getBoundingClientRect()
  const width = Math.min(260, window.innerWidth - 16)
  partyAnchor.value = {
    x:
      rect.right + width + 4 > window.innerWidth
        ? Math.max(8, rect.left - width - 4)
        : rect.right + 4,
    y: Math.max(8, Math.min(rect.top, window.innerHeight - 280)),
  }
  partyOpen.value = true
  choiceType.value = type
}
async function focusParty(event: Event, type = 2) {
  openParty(event, type)
  await nextTick()
  partyElement.value?.querySelector<HTMLButtonElement>('button')?.focus()
}
async function choose(type: number, name = '') {
  if (actions.preparing.value || actions.submitting.value) return
  const token = ++sequence
  cancel(false)
  returnFocus =
    document.activeElement instanceof HTMLElement
      ? document.activeElement
      : null
  const signature = targetSignature.value
  notice.value = ''
  const operation = await actions.run(
    [...props.selectedIds],
    'character.reverse_return',
    { reverseReturnType: type, reverseReturnName: name },
    props.scopeKey,
  )
  if (!operation) return
  if (token !== sequence || signature !== targetSignature.value) {
    actions.cancel(operation)
    actions.dismiss(operation)
    return
  }
  reviewID.value = operation.operationID
  partyOpen.value = false
  await nextTick()
  reviewElement.value?.querySelector<HTMLButtonElement>('button')?.focus()
}
async function confirm() {
  const operation = review.value
  if (!operation || actions.preparing.value || actions.submitting.value) return
  if (await actions.refreshPreview(operation)) {
    notice.value =
      'Eligibility changed. Review the updated targets before confirming.'
    return
  }
  if (review.value !== operation) return
  await actions.submit(operation)
  emit('chosen')
  reviewID.value = ''
  await nextTick()
  restoreFocus()
}
function dismiss(operation: FanOutOperation) {
  actions.dismiss(operation)
}
function moveFocus(event: KeyboardEvent) {
  const menu = event.currentTarget as HTMLElement
  const buttons = [
    ...menu.querySelectorAll<HTMLButtonElement>('button:not(:disabled)'),
  ]
  const index = buttons.indexOf(document.activeElement as HTMLButtonElement)
  if (event.key === 'ArrowDown' || event.key === 'ArrowUp') {
    event.preventDefault()
    buttons[
      (index + (event.key === 'ArrowDown' ? 1 : -1) + buttons.length) %
        buttons.length
    ]?.focus()
  }
  if (
    event.key === 'Escape' ||
    (event.key === 'ArrowLeft' && menu === partyElement.value)
  ) {
    event.preventDefault()
    event.stopPropagation()
    if (partyOpen.value && menu === partyElement.value) {
      partyOpen.value = false
      const triggers =
        rootElement.value?.querySelectorAll<HTMLButtonElement>(
          '[role="menuitem"]',
        )
      triggers?.[choiceType.value]?.focus()
      return
    }
    partyOpen.value = false
    emit('closeMenu')
    restoreFocus()
  }
}
function trapReview(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    event.stopPropagation()
    cancel()
    return
  }
  if (event.key !== 'Tab') return
  const buttons = [
    ...(reviewElement.value?.querySelectorAll<HTMLButtonElement>(
      'button:not(:disabled)',
    ) || []),
  ]
  const first = buttons[0],
    last = buttons.at(-1)
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
</script>

<template>
  <component
    :is="presentation === 'panel' ? 'details' : 'div'"
    ref="rootElement"
    class="reverse-return-actions"
    @keydown="moveFocus"
  >
    <summary v-if="presentation === 'panel'">Reverse return</summary>
    <div role="menu" aria-label="Reverse return modes">
      <button
        v-for="mode in reverseReturnModes"
        :key="mode.type"
        type="button"
        role="menuitem"
        class="map-navigation-menu-action"
        :disabled="
          !!modeReason(mode.type) ||
          actions.preparing.value ||
          actions.submitting.value
        "
        :title="modeReason(mode.type)"
        :aria-haspopup="mode.type >= 2 ? 'menu' : 'dialog'"
        :aria-expanded="
          mode.type >= 2 ? partyOpen && choiceType === mode.type : undefined
        "
        @click="
          mode.type >= 2 ? openParty($event, mode.type) : choose(mode.type)
        "
        @pointerenter="
          $event.pointerType !== 'touch' &&
          (mode.type >= 2 ? openParty($event, mode.type) : (partyOpen = false))
        "
        @focus="mode.type < 2 && (partyOpen = false)"
        @keydown.right.prevent="mode.type >= 2 && focusParty($event, mode.type)"
      >
        {{ mode.label
        }}<UIcon v-if="mode.type >= 2" name="i-lucide-chevron-right" />
      </button>
      <small v-if="!locationNames.length">{{ namedLocationReason }}</small>
      <small v-if="!selectedIds.length"
        >Tick characters in the panel to act on them.</small
      >
      <small v-else-if="modeReason(0)" role="status">{{ modeReason(0) }}</small>
      <small v-if="actions.error.value" role="alert">{{
        actions.error.value
      }}</small>
    </div>
    <Teleport to="body">
      <div
        v-if="partyOpen"
        ref="partyElement"
        role="menu"
        :aria-label="
          choiceType === 2
            ? 'Reverse return party members'
            : 'Reverse return named locations'
        "
        class="reverse-party-menu map-teleport-menu"
        :style="{ left: `${partyAnchor.x}px`, top: `${partyAnchor.y}px` }"
        @pointerdown.stop
        @keydown="moveFocus"
        @mouseenter="emit('keepMenu')"
        @mouseleave="emit('leaveMenu', $event)"
      >
        <button
          v-for="name in choiceNames"
          :key="name"
          type="button"
          role="menuitem"
          class="map-navigation-menu-action"
          @click="choose(choiceType, name)"
        >
          {{ name }}
        </button>
        <small v-if="!choiceNames.length">{{
          choiceType === 2
            ? 'No fresh party names are available.'
            : namedLocationReason
        }}</small>
        <button
          type="button"
          role="menuitem"
          class="map-navigation-menu-action"
          @click="partyOpen = false"
        >
          Back
        </button>
      </div>
      <div
        v-if="review"
        ref="reviewElement"
        class="reverse-review-backdrop map-teleport-menu"
        role="dialog"
        aria-modal="true"
        aria-label="Confirm Reverse return"
        @pointerdown.stop
        @keydown="trapReview"
      >
        <CommandFanOutPreview
          :operation="review"
          confirmation-required
          :busy="actions.preparing.value || actions.submitting.value"
          :notice="notice"
          @submit="confirm"
          @cancel="cancel"
        />
      </div>
    </Teleport>
    <Teleport to="body" :disabled="presentation !== 'menu'">
      <div
        v-if="resultOperations.length"
        :class="{
          'reverse-floating-results map-teleport-menu': presentation === 'menu',
        }"
        @pointerdown.stop
      >
        <CommandFanOutResults
          v-for="operation in resultOperations"
          :key="operation.operationID"
          :operation="operation"
          status-note="Scroll acceptance does not prove arrival; check zone or position separately."
          :stale="actions.stale.value"
          :on-retry="(id) => actions.retrySubmission(operation, id)"
          :on-dismiss="() => dismiss(operation)"
        />
      </div>
    </Teleport>
  </component>
</template>

<style scoped>
.reverse-return-actions {
  min-width: 0;
  padding: 4px;
}
.reverse-return-actions summary {
  cursor: pointer;
  padding: 6px;
  color: var(--ph-primary);
}
.reverse-return-actions [role='menu'] {
  display: grid;
  gap: 2px;
}
.reverse-return-actions button,
.reverse-party-menu button {
  display: flex;
  align-items: center;
  justify-content: space-between;
  width: 100%;
  padding: 7px 8px;
  border: 0;
  background: transparent;
  color: var(--ph-text);
  text-align: left;
  cursor: pointer;
  font-size: 13px;
}
.reverse-return-actions button:hover:not(:disabled),
.reverse-party-menu button:hover {
  background: rgb(72 112 159 / 20%);
}
.reverse-return-actions button:disabled {
  opacity: 0.5;
  cursor: default;
}
.reverse-return-actions small,
.reverse-party-menu small {
  display: block;
  padding: 4px 8px;
  color: var(--ph-muted);
  font-size: 11px;
  overflow-wrap: anywhere;
}
.reverse-return-actions button:focus-visible,
.reverse-party-menu button:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: -2px;
}
.reverse-party-menu {
  position: fixed;
  z-index: 1600;
  width: min(260px, calc(100vw - 16px));
  max-height: 260px;
  overflow: auto;
  padding: 5px;
  background: var(--ph-panel-solid, #0d131d);
  border: 1px solid var(--ph-border);
  border-radius: 4px;
}
.reverse-review-backdrop {
  position: fixed;
  inset: 0;
  z-index: 1700;
  display: grid;
  align-items: center;
  justify-items: center;
  padding: 12px;
  background: rgb(0 0 0 / 60%);
}
.reverse-review-backdrop :deep(.fanout-preview) {
  width: min(600px, 100%);
  max-height: calc(100dvh - 24px);
  overflow: auto;
}
.reverse-floating-results {
  position: fixed;
  z-index: 1550;
  right: 12px;
  bottom: 12px;
  width: min(480px, calc(100vw - 24px));
  max-height: 50dvh;
  overflow: auto;
}
</style>
