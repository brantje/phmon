<script setup lang="ts">
import type { FanOutOperation } from '~/utils/commandFanOut'
import { fanOutCounts } from '~/utils/commandFanOut'

const props = defineProps<{
  operation: FanOutOperation
  busy?: boolean
  notice?: string
}>()
const emit = defineEmits<{
  submit: []
  cancel: []
}>()
const counts = computed(() => fanOutCounts(props.operation))
</script>

<template>
  <section class="panel fanout-preview" aria-labelledby="fanout-preview-title">
    <header>
      <div>
        <h3 id="fanout-preview-title">
          Review {{ props.operation.command.label }}
        </h3>
        <p>
          {{ props.operation.command.impact }} action · browser-local review
        </p>
      </div>
      <span class="status-chip pending"><span />Review before submitting</span>
    </header>

    <p v-if="props.notice" class="fanout-preview-notice" role="status">
      {{ props.notice }}
    </p>

    <dl class="fanout-preview-counts">
      <div>
        <dt>Selected</dt>
        <dd>{{ counts.selected }}</dd>
      </div>
      <div>
        <dt>Eligible</dt>
        <dd>{{ counts.eligible }}</dd>
      </div>
      <div>
        <dt>Skipped</dt>
        <dd>{{ counts.skipped }}</dd>
      </div>
      <div>
        <dt>Planned commands</dt>
        <dd>{{ counts.eligible }}</dd>
      </div>
    </dl>

    <ol class="fanout-preview-targets">
      <li v-for="child in props.operation.children" :key="child.characterID">
        <div class="fanout-preview-target-heading">
          <strong>{{ child.characterName }}</strong>
          <span>{{
            child.submission === 'skipped' ? 'Skipped' : 'Eligible'
          }}</span>
        </div>
        <p v-if="child.argsSummary">{{ child.argsSummary }}</p>
        <pre v-else-if="child.args">{{
          JSON.stringify(child.args, null, 2)
        }}</pre>
        <p v-if="child.skipReason" class="fanout-preview-reason">
          {{ child.skipReason.message }}
        </p>
      </li>
    </ol>

    <div class="fanout-preview-actions">
      <button
        type="button"
        class="compact-button"
        :disabled="props.busy"
        @click="emit('cancel')"
      >
        Cancel
      </button>
      <button
        type="button"
        class="compact-button primary"
        :disabled="props.busy || counts.eligible === 0"
        :aria-busy="props.busy"
        @click="emit('submit')"
      >
        {{
          props.busy
            ? 'Rechecking targets…'
            : `Submit ${counts.eligible} commands`
        }}
      </button>
    </div>
  </section>
</template>

<style scoped>
.fanout-preview {
  display: grid;
  gap: 10px;
  min-width: 0;
  padding: 12px;
}
.fanout-preview header,
.fanout-preview-target-heading,
.fanout-preview-actions {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  gap: 10px;
  flex-wrap: wrap;
}
.fanout-preview header h3 {
  margin: 0;
  color: var(--ph-primary);
  font-size: 14px;
}
.fanout-preview header p,
.fanout-preview-targets p {
  margin: 3px 0 0;
  color: var(--ph-muted);
  font-size: 12px;
}
.fanout-preview-notice {
  margin: 0;
  color: var(--ph-primary);
  font-size: 12px;
}
.fanout-preview-counts {
  display: flex;
  flex-wrap: wrap;
  gap: 6px 14px;
  margin: 0;
  color: var(--ph-muted);
  font-size: 12px;
}
.fanout-preview-counts div {
  display: flex;
  gap: 4px;
}
.fanout-preview-counts dd {
  margin: 0;
  color: var(--ph-text);
  font-variant-numeric: tabular-nums;
}
.fanout-preview-targets {
  display: grid;
  gap: 7px;
  max-height: 300px;
  overflow: auto;
  margin: 0;
  padding: 0;
  list-style: none;
}
.fanout-preview-targets li {
  min-width: 0;
  padding: 8px;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  background: rgb(13 19 29 / 72%);
}
.fanout-preview-target-heading strong {
  min-width: 0;
  overflow-wrap: anywhere;
  color: var(--ph-text);
  font-size: 13px;
}
.fanout-preview-target-heading span {
  color: var(--ph-muted);
  font-size: 12px;
}
.fanout-preview-targets pre {
  max-width: 100%;
  margin: 5px 0 0;
  overflow: auto;
  color: var(--ph-text);
  font-size: 12px;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
}
.fanout-preview-reason {
  overflow-wrap: anywhere;
}
.fanout-preview-actions {
  justify-content: flex-end;
}
.fanout-preview-actions button:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}
</style>
