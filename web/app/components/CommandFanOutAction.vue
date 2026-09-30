<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'
import type {
  FanOutCommandDefinition,
  FanOutOperation,
} from '~/utils/commandFanOut'

const props = defineProps<{
  command: FanOutCommandDefinition
  characterIds: string[]
  scopeKey: string
  scopeKeyForCharacter(character: CharacterView): string
  currentScopeKey(characterID: string): string
  currentCharacter(characterID: string): CharacterView | undefined
}>()
const reviewActions = useReviewActionsPreference()

const fanOut = useCommandFanOut({
  get command() {
    return props.command
  },
  get scopeKey() {
    return props.scopeKey
  },
  scopeKeyForCharacter: props.scopeKeyForCharacter,
  currentScopeKey: props.currentScopeKey,
  currentCharacter: props.currentCharacter,
})
const reviewingOperationID = ref('')
const reviewNotice = ref('')

async function runAction() {
  const operation = await fanOut.prepare(props.characterIds)
  if (!operation) return
  if (reviewActions.value) {
    reviewingOperationID.value = operation.operationID
    reviewNotice.value = ''
    return
  }
  if (!operation.children.some((child) => child.submission === 'ready')) return
  await fanOut.submit(operation)
}

async function submitReviewed(operation: FanOutOperation) {
  if (fanOut.preparing.value || fanOut.submitting.value) return
  reviewNotice.value = ''
  const changed = await fanOut.refreshPreview(operation)
  if (changed) {
    reviewNotice.value =
      'Eligibility or command details changed. Review the updated plan before submitting.'
    return
  }
  reviewingOperationID.value = ''
  await fanOut.submit(operation)
}

function cancelReview(operation: FanOutOperation) {
  fanOut.cancel(operation)
  reviewingOperationID.value = ''
  reviewNotice.value = ''
}

onBeforeUnmount(fanOut.dispose)
</script>

<template>
  <div class="fanout-action" :class="`impact-${props.command.impact}`">
    <button
      type="button"
      class="compact-button fanout-run"
      :disabled="
        !props.characterIds.length ||
        fanOut.preparing.value ||
        fanOut.submitting.value
      "
      :aria-busy="fanOut.preparing.value || fanOut.submitting.value"
      @click="runAction"
    >
      <UIcon
        :name="
          props.command.impact === 'disruptive'
            ? 'i-lucide-triangle-alert'
            : 'i-lucide-play'
        "
      />
      {{
        fanOut.preparing.value
          ? 'Checking targets…'
          : fanOut.submitting.value
            ? 'Submitting…'
            : `${props.command.label} · ${props.characterIds.length} selected`
      }}
    </button>
    <p v-if="props.command.impact === 'disruptive'" class="fanout-impact-note">
      Disruptive action · submits after checking each selected session.
    </p>
    <p v-if="fanOut.error.value" class="form-error" role="alert">
      {{ fanOut.error.value }}
    </p>
    <CommandFanOutPreview
      v-for="operation in fanOut.operations.value.filter(
        (item) => item.operationID === reviewingOperationID,
      )"
      :key="operation.operationID"
      :operation="operation"
      :busy="fanOut.preparing.value || fanOut.submitting.value"
      :notice="reviewNotice"
      @submit="submitReviewed(operation)"
      @cancel="cancelReview(operation)"
    />
    <CommandFanOutResults
      v-for="operation in fanOut.operations.value"
      v-show="operation.operationID !== reviewingOperationID"
      :key="operation.operationID"
      :operation="operation"
      :stale="
        fanOut.stale.value &&
        operation.children.some((child) =>
          ['accepted', 'uncertain'].includes(child.submission),
        )
      "
      :on-retry="
        (characterID: string) => fanOut.retrySubmission(operation, characterID)
      "
      :on-dismiss="() => fanOut.dismiss(operation)"
    />
  </div>
</template>

<style scoped>
.fanout-action {
  display: grid;
  gap: 9px;
  min-width: 0;
}
.fanout-run {
  justify-self: start;
  max-width: 100%;
  min-height: 34px;
  text-align: left;
  white-space: normal;
  overflow-wrap: anywhere;
}
.fanout-run :deep(svg) {
  width: 14px;
  height: 14px;
  margin-right: 5px;
  vertical-align: -2px;
}
.impact-disruptive .fanout-run {
  border-color: #70464a;
  color: #ffd6cf;
}
.fanout-run:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}
.fanout-impact-note {
  margin: -4px 0 0;
  color: #efb6ad;
  font-size: 12px;
  overflow-wrap: anywhere;
}
</style>
