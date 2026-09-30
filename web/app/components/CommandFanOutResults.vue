<script setup lang="ts">
import type { FanOutOperation } from '~/utils/commandFanOut'
import { fanOutCounts } from '~/utils/commandFanOut'

const props = defineProps<{
  operation: FanOutOperation
  stale?: boolean
  statusNote?: string
  onRetry?: (characterID: string) => void
  onDismiss?: () => void
}>()

const counts = computed(() => fanOutCounts(props.operation))
</script>

<template>
  <section
    class="panel fanout-results"
    :aria-label="`${props.operation.command.label} results`"
  >
    <header class="fanout-heading">
      <div>
        <h3>{{ props.operation.command.label }}</h3>
        <p>Per-character command results</p>
      </div>
      <span class="status-chip" :class="props.stale ? 'stale' : 'online'">
        <span />{{ props.stale ? 'Result feed stale' : props.operation.state }}
      </span>
    </header>

    <div
      class="fanout-counts"
      aria-label="Command summary"
      role="status"
      aria-live="polite"
    >
      <div>
        <strong>Submission</strong>
        <span>{{ counts.selected }} selected</span>
        <span>{{ counts.eligible }} eligible</span>
        <span>{{ counts.skipped }} skipped</span>
        <span>{{ counts.awaitingSubmission }} awaiting submission</span>
        <span>{{ counts.accepted }} accepted</span>
        <span>{{ counts.rejected }} rejected</span>
        <span>{{ counts.uncertain }} uncertain</span>
      </div>
      <div>
        <strong>Execution</strong>
        <span
          >{{
            counts.accepted -
            counts.queued -
            counts.inProgress -
            counts.completed -
            counts.failed -
            counts.expired -
            counts.unknown
          }}
          awaiting result</span
        >
        <span>{{ counts.queued }} queued</span>
        <span>{{ counts.inProgress }} in progress</span>
        <span>{{ counts.completed }} completed</span>
        <span>{{ counts.failed }} failed</span>
        <span>{{ counts.expired }} expired</span>
        <span>{{ counts.unknown }} unknown</span>
      </div>
    </div>

    <p v-if="props.statusNote" class="fanout-result-note" role="status">
      {{ props.statusNote }}
    </p>

    <ol class="fanout-targets">
      <li
        v-for="child in props.operation.children"
        :key="child.idempotencyKey || child.characterID"
      >
        <div class="fanout-target-main">
          <div>
            <NuxtLink
              v-if="child.server"
              :to="`/characters/${child.characterID}`"
              class="fanout-character"
              >{{ child.characterName }}</NuxtLink
            >
            <strong v-else class="fanout-character">{{
              child.characterName
            }}</strong>
            <span v-if="child.server" class="fanout-server">{{
              child.server
            }}</span>
          </div>
          <span
            class="status-chip"
            :class="
              child.submission === 'accepted'
                ? 'online'
                : child.submission === 'rejected' ||
                    child.submission === 'skipped'
                  ? 'stale'
                  : 'pending'
            "
          >
            <span />{{ child.executionState || child.submission }}
          </span>
        </div>
        <p v-if="child.skipReason" class="fanout-reason">
          {{ child.skipReason.message }}
        </p>
        <p v-if="child.message" class="fanout-reason">{{ child.message }}</p>
        <p v-if="child.verification" class="fanout-reason">
          Verification: {{ child.verification
          }}{{ child.apiReturn === false ? ' · phBot returned false' : '' }}
        </p>
        <button
          v-if="child.submission === 'uncertain' && props.onRetry"
          type="button"
          class="compact-button"
          @click="props.onRetry(child.characterID)"
        >
          Retry this exact submission
        </button>
        <details v-if="child.args || child.commandID">
          <summary>Arguments and command evidence</summary>
          <p v-if="child.argsSummary">{{ child.argsSummary }}</p>
          <pre v-else-if="child.args">{{
            JSON.stringify(child.args, null, 2)
          }}</pre>
          <p v-if="child.commandID">Command {{ child.commandID }}</p>
          <p v-if="child.resultCode">Result code: {{ child.resultCode }}</p>
          <p v-if="child.finishedAt">Finished: {{ child.finishedAt }}</p>
          <pre v-if="child.apiReturn !== undefined">
phBot result: {{ JSON.stringify(child.apiReturn, null, 2) }}</pre>
          <pre v-if="child.effectiveArgs">
Effective arguments: {{ JSON.stringify(child.effectiveArgs, null, 2) }}</pre>
          <pre v-if="child.observedAfter">
Observed after: {{ JSON.stringify(child.observedAfter, null, 2) }}</pre>
        </details>
      </li>
    </ol>

    <button
      v-if="props.onDismiss && props.operation.state !== 'submitting'"
      type="button"
      class="compact-button fanout-dismiss"
      @click="props.onDismiss"
    >
      Dismiss results
    </button>
  </section>
</template>

<style scoped>
.fanout-results {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 12px;
}
.fanout-result-note {
  margin: 0;
  padding: 8px 10px;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  color: var(--ph-primary);
  font-size: 12px;
}
.fanout-heading,
.fanout-target-main {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 12px;
  flex-wrap: wrap;
}
.fanout-heading > div,
.fanout-target-main > div {
  min-width: 0;
}
.fanout-heading h3 {
  margin: 0;
  color: var(--ph-primary);
  font-size: 14px;
}
.fanout-heading p,
.fanout-reason,
.fanout-server {
  margin: 3px 0 0;
  color: var(--ph-muted);
  font-size: 12px;
}
.fanout-counts {
  display: grid;
  gap: 8px;
  color: var(--ph-muted);
  font-size: 12px;
}
.fanout-counts > div {
  display: flex;
  flex-wrap: wrap;
  gap: 5px 12px;
}
.fanout-counts strong {
  color: var(--ph-text);
  font-variant-numeric: tabular-nums;
}
.fanout-counts span {
  overflow-wrap: anywhere;
}
.fanout-targets {
  display: grid;
  gap: 7px;
  max-height: 320px;
  overflow: auto;
  margin: 0;
  padding: 0;
  list-style: none;
}
.fanout-targets > li {
  min-width: 0;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  padding: 8px;
  background: rgb(13 19 29 / 72%);
}
.fanout-character {
  overflow-wrap: anywhere;
  color: var(--ph-text);
  font-size: 13px;
  font-weight: 600;
}
a.fanout-character:hover {
  color: var(--ph-primary);
}
.fanout-server {
  display: block;
}
.fanout-reason {
  overflow-wrap: anywhere;
}
.fanout-targets details {
  margin-top: 6px;
  color: var(--ph-muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}
.fanout-targets p {
  overflow-wrap: anywhere;
}
.fanout-targets summary {
  width: fit-content;
  cursor: pointer;
}
.fanout-targets summary:focus-visible,
.fanout-dismiss:focus-visible,
.fanout-targets button:focus-visible,
.fanout-character:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}
.fanout-targets pre {
  max-width: 100%;
  overflow: auto;
  color: var(--ph-text);
  white-space: pre-wrap;
}
.fanout-dismiss {
  justify-self: end;
}
</style>
