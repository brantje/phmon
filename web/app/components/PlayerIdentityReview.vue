<script setup lang="ts">
import type {
  PlayerRecord,
  PlayerLink,
  PlayerHistoryPage,
} from '~~/shared/types/players'
import { playerLabel } from '~/utils/playerRegistry'
const props = defineProps<{ player: PlayerRecord }>()
const emit = defineEmits<{ changed: [] }>()
const cursor = ref('')
const {
  data: links,
  status,
  error,
  refresh,
} = useFetch<PlayerHistoryPage<PlayerLink>>('/api/players/match-candidates', {
  query: computed(() => ({
    player_id: props.player.id,
    cursor: cursor.value,
    limit: 25,
  })),
  server: false,
})
const mode = ref<'link' | 'classify' | 'candidate' | 'unlink' | null>(null)
const selected = ref<PlayerLink | null>(null)
const targetInput = ref('')
const target = ref<PlayerRecord | null>(null)
const candidateCanonical = ref<PlayerRecord | null>(null)
const reason = ref('')
const confirmed = ref(false)
const busy = ref(false)
const actionError = ref('')
const action = ref('confirm')
const aliasName = ref('')
const aliasType = ref('normal')
const job = ref('hunter')
const focusTarget = ref<HTMLInputElement | null>(null)
function open(value: typeof mode.value, link?: PlayerLink) {
  mode.value = value
  action.value = 'confirm'
  selected.value = link || null
  reason.value = ''
  confirmed.value = false
  target.value = null
  candidateCanonical.value = null
  targetInput.value = link?.linked_player_id || ''
  actionError.value = ''
  aliasName.value = props.player.observed_name
  void nextTick(() => focusTarget.value?.focus())
}
function review(link: PlayerLink) {
  open('candidate', link)
  void loadTarget()
}
async function loadTarget() {
  busy.value = true
  actionError.value = ''
  target.value = null
  try {
    if (!/^[0-9a-f-]{36}$/i.test(targetInput.value))
      throw new Error('Choose a valid player ID from the registry.')
    target.value = await $fetch<PlayerRecord>(
      `/api/players/${targetInput.value}`,
    )
    if (selected.value) {
      candidateCanonical.value = await $fetch<PlayerRecord>(
        `/api/players/${selected.value.canonical_player_id}`,
      )
    }
    const canonical = candidateCanonical.value || props.player
    if (
      target.value.server_key !== canonical.server_key ||
      target.value.id === canonical.id
    ) {
      throw new Error('Choose another player on the same server.')
    }
  } catch (e) {
    target.value = null
    actionError.value = e instanceof Error ? e.message : 'Player unavailable'
  } finally {
    busy.value = false
  }
}
async function submit() {
  if (!confirmed.value || !reason.value.trim() || busy.value) return
  busy.value = true
  actionError.value = ''
  try {
    if (mode.value === 'classify') {
      await $fetch(`/api/players/${props.player.id}/aliases`, {
        method: 'POST',
        body: {
          alias_name: aliasName.value,
          alias_type: aliasType.value,
          job_type: job.value,
          reason: reason.value.trim(),
          confirmed: true,
          revision: props.player.revision,
        },
      })
    } else if (mode.value === 'unlink' && selected.value) {
      await $fetch(`/api/players/links/${selected.value.id}`, {
        method: 'DELETE',
        body: {
          confirmed: true,
          reason: reason.value.trim(),
          revision: selected.value.revision,
        },
      })
    } else if (target.value) {
      const canonical = candidateCanonical.value || props.player
      await $fetch('/api/players/links', {
        method: 'POST',
        body: {
          canonical_player_id: canonical.id,
          linked_player_id: target.value.id,
          action: action.value,
          reason: reason.value.trim(),
          confirmed: true,
          canonical_revision: canonical.revision,
          linked_revision: target.value.revision,
          ...(selected.value
            ? {
                candidate_id: selected.value.id,
                revision: selected.value.revision,
              }
            : {}),
        },
      })
    } else
      throw new Error('Load and inspect the other player before confirming.')
    mode.value = null
    await refresh()
    emit('changed')
  } catch (e) {
    const error = e as { data?: { error?: string }; message?: string }
    actionError.value =
      error.data?.error ||
      error.message ||
      'Decision failed. Refresh the profile before retrying.'
  } finally {
    busy.value = false
  }
}
watch(
  () => props.player.id,
  () => {
    mode.value = null
    cursor.value = ''
  },
)
const disabled = computed(
  () =>
    busy.value ||
    !confirmed.value ||
    !reason.value.trim() ||
    ((mode.value === 'link' || mode.value === 'candidate') && !target.value),
)
</script>
<template>
  <section class="panel player-profile-panel">
    <header class="player-panel-header">
      <h2>Identity evidence and review</h2>
      <div class="player-actions">
        <button class="compact-button" type="button" @click="open('classify')">
          Classify observed alias</button
        ><button class="compact-button" type="button" @click="open('link')">
          Link another identity
        </button>
      </div>
    </header>
    <p class="player-muted">
      Equipment matches need supporting identity evidence. Transition matches
      require review until target-server behavior is verified.
    </p>
    <p v-if="error" role="alert">
      Identity history unavailable.
      <button class="compact-button" type="button" @click="refresh()">
        Retry
      </button>
    </p>
    <p v-else-if="status === 'pending'" role="status">
      Loading identity evidence…
    </p>
    <p v-else-if="!links?.items.length" class="player-muted">
      No identity decisions or candidates recorded.
    </p>
    <article
      v-for="link in links?.items"
      :key="link.id"
      class="player-evidence-row"
    >
      <div>
        <strong>{{ link.status }} · {{ link.method }}</strong>
        <p>
          <NuxtLink :to="`/players/${link.canonical_player_id}`"
            >Canonical player</NuxtLink
          >
          ↔
          <NuxtLink
            :to="{
              path: `/players/${link.linked_player_id}`,
              query: { source: '1' },
            }"
            >Original identity</NuxtLink
          >
        </p>
        <small
          >{{ new Date(link.created_at).toLocaleString()
          }}<template v-if="link.decided_by">
            · {{ link.decided_by }}</template
          ></small
        >
        <details>
          <summary>Source evidence and decision history</summary>
          <pre>{{ JSON.stringify(link.evidence_json, null, 2) }}</pre>
        </details>
      </div>
      <div class="player-actions">
        <button
          v-if="link.status === 'pending'"
          class="compact-button"
          type="button"
          @click="review(link)"
        >
          Review match</button
        ><button
          v-if="link.status === 'confirmed'"
          class="compact-button"
          type="button"
          @click="open('unlink', link)"
        >
          Unlink
        </button>
      </div>
    </article>
    <button
      v-if="links?.next_cursor"
      class="compact-button"
      type="button"
      @click="cursor = links.next_cursor || ''"
    >
      Older decisions
    </button>
    <button
      v-if="cursor"
      class="compact-button"
      type="button"
      @click="cursor = ''"
    >
      Latest decisions
    </button>
    <form
      v-if="mode"
      class="player-review-form"
      aria-label="Identity decision review"
      @submit.prevent="submit"
      @keydown.esc="mode = null"
    >
      <h3>
        {{
          mode === 'classify'
            ? 'Classify an observed name'
            : mode === 'unlink'
              ? 'Revoke association'
              : 'Review identity association'
        }}
      </h3>
      <template v-if="mode === 'classify'"
        ><label
          >Observed alias<input
            ref="focusTarget"
            v-model="aliasName"
            required
            maxlength="64" /></label
        ><label
          >Identity type<select v-model="aliasType" aria-label="Identity type">
            <option value="normal">Normal character name</option>
            <option value="job">Job alias</option>
          </select></label
        ><label v-if="aliasType === 'job'"
          >Job<select v-model="job">
            <option value="hunter">Hunter</option>
            <option value="trader">Trader</option>
            <option value="thief">Thief</option>
          </select></label
        ></template
      >
      <template v-else-if="mode !== 'unlink'"
        ><label
          >Other player ID<input
            ref="focusTarget"
            v-model="targetInput"
            :disabled="mode === 'candidate'"
            required
            placeholder="UUID from the player profile" /></label
        ><button
          class="compact-button"
          type="button"
          :disabled="busy"
          @click="loadTarget"
        >
          Load player
        </button>
        <p v-if="target">
          {{ playerLabel(candidateCanonical || player) }} ↔
          <NuxtLink :to="`/players/${target.id}`">{{
            playerLabel(target)
          }}</NuxtLink>
          · {{ target.server }}
        </p>
        <label v-if="mode === 'candidate'"
          >Decision<select v-model="action" aria-label="Decision">
            <option value="confirm">Confirm association</option>
            <option value="reject">Reject candidate</option>
          </select></label
        ></template
      >
      <p v-else>
        This revokes the association and restores independent registry records.
        Original observations and the decision history remain available.
      </p>
      <label
        >Evidence or correction reason<textarea
          v-model="reason"
          required
          maxlength="500"
          rows="3"
        />
      </label>
      <label class="player-confirm"
        ><input v-model="confirmed" type="checkbox" />I reviewed the identities
        and confirm this decision.</label
      >
      <p v-if="actionError" class="form-error" role="alert">
        {{ actionError }}
      </p>
      <div class="player-actions">
        <button class="compact-button" type="submit" :disabled="disabled">
          {{ busy ? 'Saving…' : 'Save decision' }}</button
        ><button
          class="compact-button"
          type="button"
          :disabled="busy"
          @click="mode = null"
        >
          Cancel
        </button>
      </div>
    </form>
  </section>
</template>
