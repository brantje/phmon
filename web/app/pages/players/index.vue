<script setup lang="ts">
import type { PlayerRecord } from '~/utils/playerRegistry'
import {
  formatModel,
  jobLabel,
  jobbingLabel,
  registryPlayerName,
  textOrUnknown,
} from '~/utils/playerRegistry'

type PlayerPage = {
  players: PlayerRecord[]
  total: number
  limit: number
  offset: number
}

const route = useRoute()
const router = useRouter()
const { serverOptions } = useServerScope()
const page = ref<PlayerPage | null>(null)
const loading = ref(true)
const failed = ref(false)
const nameDraft = ref(typeof route.query.q === 'string' ? route.query.q : '')
let nameTimer: ReturnType<typeof setTimeout> | undefined

const queryText = (key: string) =>
  typeof route.query[key] === 'string' ? String(route.query[key]) : ''

async function load() {
  loading.value = true
  failed.value = false
  try {
    page.value = await $fetch<PlayerPage>('/api/players', {
      query: route.query,
    })
  } catch {
    failed.value = true
    page.value = null
  } finally {
    loading.value = false
  }
}

function replaceQuery(
  patch: Record<string, string | undefined>,
  keepOffset = false,
) {
  const current: Record<string, string> = {}
  for (const [key, value] of Object.entries(route.query)) {
    if (typeof value === 'string') current[key] = value
  }
  const merged: Record<string, string | undefined> = { ...current, ...patch }
  const next: Record<string, string> = {}
  for (const [key, value] of Object.entries(merged)) {
    if (key === 'offset' && !keepOffset) continue
    if (value) next[key] = value
  }
  void router.replace({ query: next })
}

function onNameInput(value: string) {
  nameDraft.value = value
  clearTimeout(nameTimer)
  nameTimer = setTimeout(
    () => replaceQuery({ q: value.trim() || undefined }),
    250,
  )
}

function openPlayer(id: string) {
  void navigateTo({
    path: `/players/${id}`,
    query: { back: route.fullPath },
  })
}

function toggleSort(sort: string) {
  const current = queryText('sort') || 'last_seen_at'
  const descending = queryText('dir') !== 'asc'
  if (current === sort) {
    replaceQuery({ sort, dir: descending ? 'asc' : 'desc' }, true)
    return
  }
  replaceQuery({ sort, dir: 'desc' })
}

watch(
  () => route.query,
  () => {
    nameDraft.value = queryText('q')
    void load()
  },
  { immediate: true },
)
onBeforeUnmount(() => clearTimeout(nameTimer))

const total = computed(() => page.value?.total ?? 0)
const offset = computed(() => page.value?.offset ?? 0)
const limit = computed(() => page.value?.limit ?? 25)

function formatPlayerTime(value?: string | null) {
  if (!value) return 'Unknown'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'
  return date.toLocaleString()
}
</script>

<template>
  <section class="player-page">
    <PageHeader
      title="Player"
      icon="i-lucide-users"
      description="Observed Silkroad players, kept as separate names on each server."
    />
    <form class="player-filters" @submit.prevent>
      <label>
        Server
        <select
          :value="queryText('server')"
          @change="
            replaceQuery({
              server: ($event.target as HTMLSelectElement).value || undefined,
            })
          "
        >
          <option value="">All servers</option>
          <option v-for="server in serverOptions" :key="server" :value="server">
            {{ server }}
          </option>
        </select>
      </label>
      <label>
        Name
        <input
          :value="nameDraft"
          type="search"
          placeholder="Observed, player, or job name"
          @input="onNameInput(($event.target as HTMLInputElement).value)"
        />
      </label>
      <label>
        Min level
        <input
          :value="queryText('min_level')"
          inputmode="numeric"
          @change="
            replaceQuery({
              min_level: ($event.target as HTMLInputElement).value || undefined,
            })
          "
        />
      </label>
      <label>
        Max level
        <input
          :value="queryText('max_level')"
          inputmode="numeric"
          @change="
            replaceQuery({
              max_level: ($event.target as HTMLInputElement).value || undefined,
            })
          "
        />
      </label>
      <label>
        Guild
        <input
          :value="queryText('guild')"
          @change="
            replaceQuery({
              guild:
                ($event.target as HTMLInputElement).value.trim() || undefined,
            })
          "
        />
      </label>
      <label>
        Job
        <select
          :value="queryText('job')"
          @change="
            replaceQuery({
              job: ($event.target as HTMLSelectElement).value || undefined,
            })
          "
        >
          <option value="">All</option>
          <option value="none">None</option>
          <option value="trader">Trader</option>
          <option value="thief">Thief</option>
          <option value="hunter">Hunter</option>
          <option value="unknown">Unknown</option>
        </select>
      </label>
      <label>
        Min job level
        <input
          :value="queryText('min_job_level')"
          inputmode="numeric"
          @change="
            replaceQuery({
              min_job_level:
                ($event.target as HTMLInputElement).value || undefined,
            })
          "
        />
      </label>
      <label>
        Max job level
        <input
          :value="queryText('max_job_level')"
          inputmode="numeric"
          @change="
            replaceQuery({
              max_job_level:
                ($event.target as HTMLInputElement).value || undefined,
            })
          "
        />
      </label>
      <label>
        Jobbing
        <select
          :value="queryText('jobbing')"
          @change="
            replaceQuery({
              jobbing: ($event.target as HTMLSelectElement).value || undefined,
            })
          "
        >
          <option value="">All</option>
          <option value="yes">Yes</option>
          <option value="no">No</option>
          <option value="unknown">Unknown</option>
        </select>
      </label>
      <label>
        Model
        <input
          :value="queryText('model')"
          placeholder="Name or ID"
          @change="
            replaceQuery({
              model:
                ($event.target as HTMLInputElement).value.trim() || undefined,
            })
          "
        />
      </label>
      <label>
        Last seen
        <select
          :value="queryText('seen')"
          @change="
            replaceQuery({
              seen: ($event.target as HTMLSelectElement).value || undefined,
            })
          "
        >
          <option value="">All time</option>
          <option value="1h">Last hour</option>
          <option value="24h">Last 24 hours</option>
          <option value="7d">Last 7 days</option>
          <option value="30d">Last 30 days</option>
        </select>
      </label>
      <button
        type="button"
        class="player-clear"
        @click="router.replace('/players')"
      >
        Clear filters
      </button>
    </form>
    <p v-if="loading" class="player-note">Loading players…</p>
    <p v-else-if="failed" class="player-note">
      The player registry is unavailable.
    </p>
    <p v-else-if="total === 0" class="player-note">
      No observed players match these filters.
    </p>
    <div v-else class="event-table-scroll">
      <p class="player-note">{{ total }} matching players</p>
      <table class="event-table player-table">
        <thead>
          <tr>
            <th>
              <button type="button" @click="toggleSort('player_name')">
                Player Name
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('job_name')">
                Job Name
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('server')">
                Server
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('level')">Level</button>
            </th>
            <th>
              <button type="button" @click="toggleSort('guild_name')">
                Guild
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('job')">Job</button>
            </th>
            <th>
              <button type="button" @click="toggleSort('job_level')">
                Job Level
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('is_jobbing')">
                Jobbing
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('model_name')">
                Model Name
              </button>
            </th>
            <th>
              <button type="button" @click="toggleSort('last_seen_at')">
                Last Seen
              </button>
            </th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="player in page?.players"
            :key="player.id"
            tabindex="0"
            @click="openPlayer(player.id)"
            @keydown.enter="openPlayer(player.id)"
          >
            <td>{{ registryPlayerName(player) || 'Unknown' }}</td>
            <td>{{ textOrUnknown(player.job_name) }}</td>
            <td>{{ player.server }}</td>
            <td>{{ textOrUnknown(player.level) }}</td>
            <td>{{ textOrUnknown(player.guild_name) }}</td>
            <td>{{ jobLabel(player.job) }}</td>
            <td>{{ textOrUnknown(player.job_level) }}</td>
            <td>{{ jobbingLabel(player.is_jobbing) }}</td>
            <td>{{ formatModel(player.model_name, player.model_id) }}</td>
            <td>{{ formatPlayerTime(player.last_seen_at) }}</td>
          </tr>
        </tbody>
      </table>
      <div class="player-pager">
        <button
          type="button"
          :disabled="offset === 0"
          @click="
            replaceQuery({ offset: String(Math.max(0, offset - limit)) }, true)
          "
        >
          Previous
        </button>
        <span
          >{{ offset + 1 }}–{{ Math.min(offset + limit, total) }} of
          {{ total }}</span
        >
        <button
          type="button"
          :disabled="offset + limit >= total"
          @click="replaceQuery({ offset: String(offset + limit) }, true)"
        >
          Next
        </button>
      </div>
    </div>
  </section>
</template>
