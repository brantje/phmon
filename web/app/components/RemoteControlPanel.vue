<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'
import type { FanOutOperation } from '~/utils/commandFanOut'
import { useRemoteControlActions } from '~/composables/useRemoteControlActions'
import {
  requiresRemoteControlConfirmation,
  remoteControlSignature,
  validateRemoteControlArgs,
  type RemoteControlActionName,
  type RemoteControlArgs,
} from '~/utils/remoteControlActions'

const props = withDefaults(
  defineProps<{
    selectedIds: string[]
    scopeKey: string
    scopeKeyForCharacter(character: CharacterView, scopeKey: string): string
    currentScopeKey(characterID: string): string
    currentCharacter(characterID: string): CharacterView | undefined
    variant?: 'default' | 'map'
    traceCandidates?: CharacterView[]
    mapSnapshotCurrent?: boolean
  }>(),
  {
    variant: 'default',
    traceCandidates: () => [],
    mapSnapshotCurrent: true,
  },
)
const reviewActions = useReviewActionsPreference()
const actions = useRemoteControlActions({
  scopeKey: () => props.scopeKey,
  scopeKeyForCharacter: props.scopeKeyForCharacter,
  currentScopeKey: props.currentScopeKey,
  currentCharacter: props.currentCharacter,
  mapSnapshotCurrent: () => props.mapSnapshotCurrent,
})

const selectedAction = ref<RemoteControlActionName>('bot.start')
const traceName = ref('')
const trainingAreaMode = ref<'current_position' | 'named'>('current_position')
const trainingAreaName = ref('')
const trainingRadius = ref('')
const formError = ref('')
const notice = ref('')
const mapActionTrigger = ref<HTMLButtonElement | null>(null)
const mapMoreOpen = ref(false)
const runButtonRef = ref<HTMLButtonElement | null>(null)
const activePreviewID = ref('')
const activePreviewSignature = ref('')
const reviewNotice = ref('')

const actionGroups: {
  title: string
  actions: { name: RemoteControlActionName; label: string; icon: string }[]
}[] = [
  {
    title: 'Training',
    actions: [
      { name: 'bot.start', label: 'Start Training', icon: 'i-lucide-play' },
      { name: 'bot.stop', label: 'Stop Training', icon: 'i-lucide-square' },
      {
        name: 'training.area.set',
        label: 'Set Training Area',
        icon: 'i-lucide-crosshair',
      },
      {
        name: 'training.radius.set',
        label: 'Set Training Radius',
        icon: 'i-lucide-circle-dot',
      },
    ],
  },
  {
    title: 'Trace',
    actions: [
      { name: 'trace.start', label: 'Start Trace', icon: 'i-lucide-route' },
      { name: 'trace.stop', label: 'Stop Trace', icon: 'i-lucide-route-off' },
    ],
  },
  {
    title: 'Character',
    actions: [
      {
        name: 'character.return',
        label: 'Return Scroll',
        icon: 'i-lucide-scroll',
      },
      {
        name: 'character.disconnect',
        label: 'Disconnect',
        icon: 'i-lucide-unplug',
      },
      {
        name: 'client.clientless',
        label: 'Go Clientless',
        icon: 'i-lucide-monitor-off',
      },
    ],
  },
]

const selectedArgs = computed<RemoteControlArgs>(() => {
  if (selectedAction.value === 'trace.start')
    return { traceName: traceName.value }
  if (selectedAction.value === 'training.area.set')
    return {
      trainingAreaMode: trainingAreaMode.value,
      trainingAreaName: trainingAreaName.value,
    }
  if (selectedAction.value === 'training.radius.set')
    return { trainingRadius: trainingRadius.value }
  return {}
})
const validatedArgs = computed(() =>
  validateRemoteControlArgs(selectedAction.value, selectedArgs.value),
)
const preview = computed(() =>
  validatedArgs.value
    ? actions.preview(
        props.selectedIds,
        selectedAction.value,
        validatedArgs.value,
        props.scopeKey,
      )
    : null,
)
const selectedOperation = computed(() =>
  actions.operations.value.find(
    (operation) => operation.operationID === activePreviewID.value,
  ),
)
const livePreviewSignature = computed(() =>
  remoteControlSignature(
    selectedAction.value,
    selectedArgs.value,
    props.selectedIds,
    props.scopeKey,
  ),
)
const selectedLabel = computed(
  () =>
    actionGroups
      .flatMap((group) => group.actions)
      .find((action) => action.name === selectedAction.value)?.label ||
    selectedAction.value,
)
const confirmationRequired = computed(() =>
  requiresRemoteControlConfirmation(selectedAction.value, false),
)
const reviewBeforeSubmit = computed(() =>
  requiresRemoteControlConfirmation(selectedAction.value, reviewActions.value),
)
const previewCounts = computed(() => ({
  selected: preview.value?.selectedCount ?? props.selectedIds.length,
  eligible: preview.value?.eligibleCount ?? 0,
  skipped: preview.value?.skippedCount ?? 0,
}))
const actionButtonLabel = computed(
  () =>
    `${selectedLabel.value} ${previewCounts.value.eligible} character${previewCounts.value.eligible === 1 ? '' : 's'} · ${previewCounts.value.selected} selected · ${previewCounts.value.skipped} skipped`,
)
const runDisabled = computed(
  () =>
    !props.selectedIds.length ||
    !actions.controlsCurrent.value ||
    !preview.value ||
    previewCounts.value.eligible === 0 ||
    !validatedArgs.value ||
    actions.preparing.value ||
    actions.submitting.value,
)
const previewTargets = computed(
  () =>
    preview.value?.children.map((child) => ({
      id: child.characterID,
      name: child.characterName,
      server: child.server,
      eligible: !child.skipReason,
      reason: child.skipReason?.message,
      argsSummary: child.argsSummary,
    })) || [],
)

watch(
  () => props.selectedIds.join('\0'),
  () => actions.setTargets(props.selectedIds),
  { immediate: true },
)

watch(
  [() => props.selectedIds.join('\0'), () => props.scopeKey],
  ([targetKey, scopeKey], previous) => {
    if (!previous) return
    const targetsChanged = targetKey !== previous[0]
    const scopeChanged = scopeKey !== previous[1]
    if (!targetsChanged && !scopeChanged) return
    const operation = selectedOperation.value
    if (operation?.state === 'prepared') {
      actions.cancel(operation)
      actions.dismiss(operation)
      activePreviewID.value = ''
      reviewNotice.value =
        'Targets or scope changed. Choose the action again to prepare a new request.'
    }
    if (targetsChanged) actions.setTargets(props.selectedIds)
    if (scopeChanged) formError.value = ''
  },
)

watch(livePreviewSignature, (signature) => {
  const operation = selectedOperation.value
  if (
    operation?.state === 'prepared' &&
    activePreviewSignature.value &&
    signature !== activePreviewSignature.value
  ) {
    actions.cancel(operation)
    actions.dismiss(operation)
    activePreviewID.value = ''
    reviewNotice.value =
      'Action details changed. Choose the action again to review the new request.'
  }
})

watch(selectedAction, (name) => {
  formError.value = ''
  if (name === 'training.area.set') {
    const modes = Object.values(actions.feed.value?.targets || {}).flatMap(
      (target) => target.controls?.capabilities[name]?.modes || [],
    )
    trainingAreaMode.value = modes.includes('current_position')
      ? 'current_position'
      : 'named'
  }
})

const mapActions = [
  { name: 'bot.start' as const, label: 'Start bot', icon: 'i-lucide-play' },
  { name: 'bot.stop' as const, label: 'Stop bot', icon: 'i-lucide-square' },
  {
    name: 'character.return' as const,
    label: 'Return scroll',
    icon: 'i-lucide-scroll',
  },
  {
    name: 'character.disconnect' as const,
    label: 'Disconnect',
    icon: 'i-lucide-unplug',
  },
]
const mapMoreActions = [
  'training.area.set',
  'training.radius.set',
  'client.clientless',
]
function mapActionReason(name: RemoteControlActionName) {
  if (!props.selectedIds.length) return 'Select characters below.'
  if (actions.preparing.value || actions.submitting.value)
    return 'Another action is being submitted.'
  if (!actions.controlsCurrent.value)
    return 'Current session capabilities are unavailable.'
  const args = validateRemoteControlArgs(
    name,
    name === 'trace.start' ? { traceName: traceName.value } : {},
  )
  if (!args) return 'Enter a player name.'
  const plan = actions.preview(props.selectedIds, name, args, props.scopeKey)
  return plan?.eligibleCount
    ? ''
    : plan?.children
        .map((child) => child.skipReason?.message)
        .filter(Boolean)
        .join(' · ') || 'No eligible targets.'
}
async function runMapAction(name: RemoteControlActionName, event: MouseEvent) {
  mapActionTrigger.value = event.currentTarget as HTMLButtonElement
  if (mapActionReason(name)) return
  chooseAction(name)
  await nextTick()
  await runAction()
}
const mapResultSummary = computed(() => {
  const operation = actions.operations.value
    .filter((item) => item.state !== 'prepared' && item.state !== 'cancelled')
    .at(-1)
  if (!operation) return ''
  const completed = operation.children.filter(
    (child) => child.executionState === 'completed',
  ).length
  const skipped = operation.children.filter(
    (child) => child.submission === 'skipped',
  ).length
  const failed = operation.children.filter(
    (child) =>
      child.submission === 'rejected' ||
      child.executionState === 'failed' ||
      child.executionState === 'expired',
  ).length
  const pending = operation.children.length - completed - skipped - failed
  return `${operation.command.label}: ${completed} completed${skipped ? ` · ${skipped} skipped` : ''}${failed ? ` · ${failed} failed` : ''}${pending ? ` · ${pending} pending or unknown` : ''}`
})
function chooseAction(name: RemoteControlActionName) {
  selectedAction.value = name
  notice.value = ''
  reviewNotice.value = ''
}

async function runAction() {
  if (runDisabled.value || !validatedArgs.value) return
  formError.value = ''
  notice.value = ''
  reviewNotice.value = ''
  const signature = livePreviewSignature.value
  const operation = await actions.run(
    [...props.selectedIds],
    selectedAction.value,
    validatedArgs.value,
    props.scopeKey,
  )
  if (!operation) return
  if (signature !== livePreviewSignature.value) {
    actions.cancel(operation)
    actions.dismiss(operation)
    notice.value =
      'Targets, scope or arguments changed while eligibility was refreshed. Choose the action again.'
    return
  }
  if (!operation.children.some((child) => child.submission === 'ready')) {
    notice.value = 'No selected character is eligible for this action.'
    return
  }
  if (reviewBeforeSubmit.value) {
    activePreviewID.value = operation.operationID
    activePreviewSignature.value = signature
    return
  }
  await actions.submit(operation)
}

async function submitReviewed(operation: FanOutOperation) {
  if (actions.preparing.value || actions.submitting.value) return
  reviewNotice.value = ''
  const changed = await actions.refreshPreview(operation)
  if (changed) {
    reviewNotice.value =
      'Eligibility or command details changed. Review the updated plan before submitting.'
    activePreviewSignature.value = livePreviewSignature.value
    return
  }
  if (livePreviewSignature.value !== activePreviewSignature.value) {
    actions.cancel(operation)
    actions.dismiss(operation)
    activePreviewID.value = ''
    reviewNotice.value = 'Action details changed. Choose the action again.'
    return
  }
  activePreviewID.value = ''
  await actions.submit(operation)
}

function cancelReview(operation: FanOutOperation) {
  actions.cancel(operation)
  actions.dismiss(operation)
  activePreviewID.value = ''
  activePreviewSignature.value = ''
  reviewNotice.value = ''
  void nextTick(() =>
    (props.variant === 'map'
      ? mapActionTrigger.value
      : runButtonRef.value
    )?.focus(),
  )
}
</script>

<template>
  <section
    class="panel remote-control-panel"
    :class="{ 'map-remote-panel': variant === 'map' }"
    aria-labelledby="remote-control-title"
  >
    <header class="remote-control-heading">
      <div>
        <h2 id="remote-control-title">
          {{ variant === 'map' ? 'Actions' : 'Character actions' }}
        </h2>
        <p v-if="variant === 'map'">
          {{
            props.selectedIds.length
              ? `${props.selectedIds.length} selected`
              : 'Select characters below'
          }}
        </p>
        <p v-else>
          {{ props.selectedIds.length }} selected ·
          {{ previewCounts.eligible }} eligible ·
          {{ previewCounts.skipped }} skipped
        </p>
      </div>
      <span
        v-if="variant === 'default'"
        class="status-chip"
        :class="actions.controlsCurrent.value ? 'online' : 'stale'"
      >
        <span />{{
          actions.controlsCurrent.value
            ? 'Eligibility current'
            : 'Checking live eligibility'
        }}
      </span>
    </header>

    <p
      v-if="!actions.controlsCurrent.value"
      class="remote-control-state"
      role="status"
    >
      Character actions are disabled until current session capabilities arrive.
    </p>
    <p
      v-else-if="props.mapSnapshotCurrent === false"
      class="remote-control-state warning"
      role="status"
    >
      The selected map scope is stale. Refresh the map before using these
      actions.
    </p>
    <p
      v-else-if="variant === 'default' && !props.selectedIds.length"
      class="remote-control-state"
      role="status"
    >
      Select one or more characters to prepare an action.
    </p>

    <div v-if="variant === 'map'" class="map-remote-actions">
      <button
        v-for="action in mapActions"
        :key="action.name"
        class="compact-button"
        type="button"
        :disabled="Boolean(mapActionReason(action.name))"
        :title="mapActionReason(action.name)"
        @click="runMapAction(action.name, $event)"
      >
        <UIcon :name="action.icon" />{{ action.label }}
      </button>
      <div class="map-trace-row remote-control-form">
        <select v-model="traceName" aria-label="Trace leader">
          <option value="">Trace player…</option>
          <option
            v-for="character in traceCandidates"
            :key="character.character_id"
            :value="character.name"
          >
            Trace {{ character.name }}
          </option>
          <option
            v-if="
              traceName &&
              !traceCandidates.some((item) => item.name === traceName)
            "
            :value="traceName"
          >
            Trace {{ traceName }}
          </option>
        </select>
        <button
          class="compact-button"
          type="button"
          :disabled="Boolean(mapActionReason('trace.start'))"
          :title="mapActionReason('trace.start')"
          @click="runMapAction('trace.start', $event)"
        >
          Start
        </button>
        <button
          class="compact-button"
          type="button"
          :disabled="Boolean(mapActionReason('trace.stop'))"
          :title="mapActionReason('trace.stop')"
          @click="runMapAction('trace.stop', $event)"
        >
          Stop
        </button>
        <button
          class="compact-button"
          type="button"
          disabled
          title="Nearby-player discovery is unavailable; tracked in issue #57."
          aria-label="Refresh nearby players"
        >
          <UIcon name="i-lucide-refresh-cw" />
        </button>
      </div>
    </div>
    <details
      v-if="variant === 'map'"
      class="remote-control-eligibility map-more-controls"
      @toggle="mapMoreOpen = ($event.target as HTMLDetailsElement).open"
    >
      <summary>More controls</summary>
      <label class="remote-control-form map-manual-trace"
        >Player name<input
          v-model="traceName"
          maxlength="64"
          autocomplete="off"
          aria-label="Manual trace player name"
      /></label>
      <div class="remote-control-choices">
        <button
          v-for="action in actionGroups
            .flatMap((group) => group.actions)
            .filter((action) => mapMoreActions.includes(action.name))"
          :key="action.name"
          class="remote-control-choice"
          :class="{ 'is-selected': selectedAction === action.name }"
          type="button"
          @click="chooseAction(action.name)"
        >
          {{ action.label }}
        </button>
      </div>
    </details>
    <div v-if="variant === 'default'" class="remote-control-groups">
      <section
        v-for="group in actionGroups"
        :key="group.title"
        class="remote-control-group"
      >
        <h3>{{ group.title }}</h3>
        <div class="remote-control-choices">
          <button
            v-for="action in group.actions"
            :key="action.name"
            type="button"
            class="remote-control-choice"
            :class="{
              'is-selected': selectedAction === action.name,
              'is-disruptive': [
                'character.disconnect',
                'client.clientless',
              ].includes(action.name),
            }"
            :aria-pressed="selectedAction === action.name"
            @click="chooseAction(action.name)"
          >
            <UIcon :name="action.icon" />{{ action.label }}
          </button>
        </div>
      </section>
    </div>

    <div
      v-if="variant === 'default' && selectedAction === 'trace.start'"
      class="remote-control-form"
    >
      <label>
        Player name
        <input v-model="traceName" maxlength="64" autocomplete="off" />
      </label>
      <small>All eligible characters will trace this same player.</small>
    </div>
    <div
      v-if="selectedAction === 'training.area.set'"
      class="remote-control-form"
    >
      <label>
        Set training area from
        <select v-model="trainingAreaMode">
          <option value="current_position">
            Each character’s current position
          </option>
          <option value="named">Named area in each phBot profile</option>
        </select>
      </label>
      <label v-if="trainingAreaMode === 'named'">
        Area name
        <input v-model="trainingAreaName" maxlength="100" autocomplete="off" />
      </label>
      <small v-if="trainingAreaMode === 'current_position'">
        Each character’s position is read by phBot when that command runs.
      </small>
      <small v-else>
        Area names must exist in each character’s phBot profile; profiles may
        differ.
      </small>
    </div>
    <div
      v-if="selectedAction === 'training.radius.set'"
      class="remote-control-form"
    >
      <label>
        Radius (1–10,000)
        <input
          v-model="trainingRadius"
          type="number"
          min="1"
          max="10000"
          step="any"
        />
      </label>
      <small>Changes the active training area’s radius only.</small>
    </div>
    <p v-if="!validatedArgs" class="form-error" role="status">
      Enter valid action values before submitting.
    </p>
    <p v-if="formError" class="form-error" role="alert">{{ formError }}</p>
    <p v-if="actions.error.value" class="form-error" role="alert">
      {{ actions.error.value }}
    </p>
    <p v-if="notice" class="remote-control-state" role="status">{{ notice }}</p>
    <p v-if="reviewNotice" class="remote-control-state" role="status">
      {{ reviewNotice }}
    </p>

    <details
      v-if="preview && (variant === 'default' || mapMoreOpen)"
      class="remote-control-eligibility"
      :open="variant === 'default'"
    >
      <summary>
        {{ preview.eligibleCount }} eligible ·
        {{ preview.skippedCount }} skipped for {{ selectedLabel }}
      </summary>
      <ul>
        <li
          v-for="target in previewTargets"
          :key="target.id"
          :class="target.eligible ? 'is-eligible' : 'is-skipped'"
        >
          <span>
            <strong>{{ target.name }}</strong
            ><small>{{ target.server }}</small>
            <span v-if="target.argsSummary">{{ target.argsSummary }}</span>
          </span>
          <span v-if="target.reason">{{ target.reason }}</span>
          <span v-else>Eligible</span>
        </li>
      </ul>
    </details>

    <button
      v-if="variant === 'default' || mapMoreActions.includes(selectedAction)"
      ref="runButtonRef"
      type="button"
      class="compact-button remote-control-run"
      :class="{
        'is-disruptive':
          selectedAction === 'character.disconnect' ||
          selectedAction === 'client.clientless',
      }"
      :disabled="runDisabled"
      :aria-busy="actions.preparing.value || actions.submitting.value"
      @click="runAction"
    >
      <UIcon
        v-if="
          selectedAction === 'character.disconnect' ||
          selectedAction === 'client.clientless'
        "
        name="i-lucide-triangle-alert"
      />
      {{
        actions.preparing.value
          ? 'Checking targets…'
          : actions.submitting.value
            ? 'Submitting…'
            : actionButtonLabel
      }}
    </button>
    <p v-if="confirmationRequired" class="remote-control-impact">
      Explicit confirmation is required before sending
      {{ selectedLabel }} to {{ previewCounts.eligible }} eligible character{{
        previewCounts.eligible === 1 ? '' : 's'
      }}.
    </p>
    <p
      v-if="selectedAction === 'client.clientless'"
      class="remote-control-impact"
    >
      Current phBot runtimes report no supported per-session clientless action.
    </p>

    <CommandFanOutPreview
      v-if="selectedOperation"
      :operation="selectedOperation"
      :confirmation-required="confirmationRequired"
      :busy="actions.preparing.value || actions.submitting.value"
      :notice="reviewNotice"
      @submit="submitReviewed(selectedOperation)"
      @cancel="cancelReview(selectedOperation)"
    />
    <p
      v-if="variant === 'map' && mapResultSummary"
      class="remote-control-state"
      role="status"
    >
      {{ mapResultSummary }}
    </p>
    <details
      v-if="
        variant === 'default' ||
        actions.operations.value.some(
          (item) => !['prepared', 'cancelled'].includes(item.state),
        )
      "
      :open="variant === 'default'"
      class="remote-control-results"
    >
      <summary v-if="variant === 'map' && actions.operations.value.length">
        Action results
      </summary>
      <CommandFanOutResults
        v-for="operation in actions.operations.value.filter(
          (item) => item.operationID !== activePreviewID,
        )"
        :key="operation.operationID"
        :operation="operation"
        :stale="actions.stale.value"
        :on-retry="
          (characterID: string) =>
            actions.retrySubmission(operation, characterID)
        "
        :on-dismiss="() => actions.dismiss(operation)"
      />
    </details>
  </section>
</template>

<style scoped>
.map-remote-actions {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 5px;
}
.map-trace-row {
  grid-column: 1 / -1;
  display: flex;
  align-items: center;
  gap: 4px;
}
.map-trace-row select {
  min-width: 0;
  flex: 1;
}
.map-manual-trace {
  grid-column: 1 / -1;
}
.remote-control-results {
  min-width: 0;
}

.remote-control-panel {
  display: grid;
  min-width: 0;
  gap: 9px;
  padding: 11px;
}
.remote-control-heading,
.remote-control-heading > div {
  min-width: 0;
}
.remote-control-heading {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 8px;
}
.remote-control-heading h2,
.remote-control-heading p {
  margin: 0;
}
.remote-control-heading h2 {
  color: var(--ph-primary);
  font-size: 14px;
}
.remote-control-heading p,
.remote-control-state,
.remote-control-impact {
  color: var(--ph-muted);
  font-size: 12px;
}
.remote-control-state,
.remote-control-impact {
  margin: 0;
}
.remote-control-state.warning,
.remote-control-impact {
  color: #efb6ad;
}
.remote-control-groups {
  display: grid;
  gap: 8px;
}
.remote-control-group h3 {
  margin: 0 0 4px;
  color: #c7b987;
  font-size: 11px;
  text-transform: uppercase;
  letter-spacing: 0.05em;
}
.remote-control-choices {
  display: flex;
  flex-wrap: wrap;
  gap: 5px;
}
.remote-control-choice {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 30px;
  max-width: 100%;
  padding: 4px 7px;
  border: 1px solid #344253;
  border-radius: 4px;
  background: rgb(13 19 29 / 82%);
  color: #cbd5e5;
  font-size: 11px;
  text-align: left;
  overflow-wrap: anywhere;
}
.remote-control-choice.is-selected {
  border-color: #55708e;
  background: rgb(45 69 96 / 45%);
  color: var(--ph-primary);
}
.remote-control-choice.is-disruptive,
.remote-control-run.is-disruptive {
  border-color: #70464a;
  color: #ffd6cf;
}
.remote-control-form,
.remote-control-form label {
  display: grid;
  gap: 5px;
}
.remote-control-form {
  color: #d1dae8;
  font-size: 12px;
}
.remote-control-form input,
.remote-control-form select {
  width: 100%;
  min-height: 32px;
  padding: 5px 7px;
  border: 1px solid #344253;
  border-radius: 4px;
  background: #0d131d;
  color: #eaf1ff;
}
.remote-control-form small {
  color: var(--ph-muted);
  font-size: 11px;
}
.remote-control-eligibility {
  min-width: 0;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  background: rgb(13 19 29 / 55%);
}
.remote-control-eligibility summary {
  padding: 7px;
  color: var(--ph-primary);
  cursor: pointer;
  font-size: 12px;
}
.remote-control-eligibility ul {
  display: grid;
  gap: 4px;
  max-height: 220px;
  overflow: auto;
  margin: 0;
  padding: 0 6px 6px;
  list-style: none;
}
.remote-control-eligibility li {
  display: flex;
  justify-content: space-between;
  gap: 8px;
  min-width: 0;
  padding: 5px;
  border-top: 1px solid var(--ph-border-soft);
  color: var(--ph-muted);
  font-size: 11px;
  overflow-wrap: anywhere;
}
.remote-control-eligibility li > span:first-child {
  display: grid;
  min-width: 0;
}
.remote-control-eligibility li small {
  color: var(--ph-muted);
}
.remote-control-eligibility strong {
  color: var(--ph-text);
}
.remote-control-eligibility .is-eligible > span:last-child {
  color: #a6d8c2;
}
.remote-control-eligibility .is-skipped > span:last-child {
  color: #efb6ad;
}
.remote-control-run {
  justify-self: start;
  min-height: 34px;
  max-width: 100%;
  white-space: normal;
  text-align: left;
  overflow-wrap: anywhere;
}
.remote-control-choice:focus-visible,
.remote-control-run:focus-visible,
.remote-control-form input:focus-visible,
.remote-control-form select:focus-visible,
.remote-control-eligibility summary:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}

.map-remote-panel {
  border: 0;
  background: transparent;
  padding: 0;
  gap: 6px;
}
.map-remote-panel .remote-control-heading > div {
  display: flex;
  justify-content: space-between;
  align-items: center;
  width: 100%;
  gap: 6px;
}
.map-remote-panel .remote-control-heading h2 {
  color: var(--ph-muted);
  font-size: 12px;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.map-remote-panel .remote-control-heading p {
  color: var(--ph-muted);
  font-size: 11px;
}
.map-remote-actions .compact-button {
  justify-content: flex-start;
  min-height: 28px;
  font-size: 12px;
  padding: 3px 7px;
}
.map-remote-actions .map-trace-row {
  display: flex;
  flex-direction: row;
  grid-template-columns: none;
  gap: 5px;
}
.map-remote-actions .map-trace-row select {
  width: 0;
  min-height: 30px;
  padding: 3px 6px;
}
.map-trace-row .compact-button {
  flex: none;
  justify-content: center;
}
.map-remote-panel .map-more-controls {
  border: 0;
  background: transparent;
}
.map-remote-panel .map-more-controls summary {
  padding: 2px 0;
  color: var(--ph-muted);
  font-size: 11px;
}
.map-remote-panel .remote-control-results summary {
  color: var(--ph-muted);
  font-size: 11px;
}
.map-remote-panel .remote-control-state {
  font-size: 11px;
}
</style>
