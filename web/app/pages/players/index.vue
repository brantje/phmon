<script setup lang="ts">
import type { PlayerPage } from '~~/shared/types/players'
import {
  normalizePlayerQuery,
  playerJobs,
  playerJobLabel,
  playerLabel,
  equipmentLabel,
  playerRelativeTime,
} from '~/utils/playerRegistry'

const route = useRoute()
const router = useRouter()
const { serverScope, serverOptions } = useServerScope()
const registryServers = useState<string[]>('player-registry-servers', () => [])
const filters = computed(() =>
  normalizePlayerQuery(route.query, serverScope.value),
)
const apiQuery = computed(() => {
  const query = { ...filters.value }
  if (query.server === 'all') delete query.server
  return query
})
const {
  data: page,
  status,
  error,
  refresh,
} = useFetch<PlayerPage>('/api/players', {
  query: apiQuery,
  server: false,
  dedupe: 'cancel',
})
watch(page, (value) => {
  if (value) registryServers.value = value.servers
})
const nameInput = ref(filters.value.q || '')
const guildInput = ref(filters.value.guild || '')
let debounce: ReturnType<typeof setTimeout> | undefined
const now = ref(Date.now())
let clock: ReturnType<typeof setInterval> | undefined
const currentPage = computed(() =>
  Math.min(
    100000,
    Math.max(
      1,
      Math.trunc(
        Number(typeof route.query.page === 'string' ? route.query.page : '1'),
      ) || 1,
    ),
  ),
)
async function change(
  values: Record<string, string | undefined>,
  reset = true,
) {
  const query: Record<string, string> = {
    ...filters.value,
    server: filters.value.server || 'all',
  }
  for (const [key, value] of Object.entries(values)) {
    if (value) query[key] = value
    else Reflect.deleteProperty(query, key)
  }
  if (reset) {
    delete query.cursor
  } else {
    query.page = String(currentPage.value)
  }
  await router.push({ path: '/players', query })
}
function textChange() {
  if (debounce) clearTimeout(debounce)
  debounce = setTimeout(
    () =>
      void change({
        q: nameInput.value.trim(),
        guild: guildInput.value.trim(),
      }),
    250,
  )
}
watch(
  () => route.query,
  () => {
    nameInput.value = filters.value.q || ''
    guildInput.value = filters.value.guild || ''
    const server = filters.value.server || 'all'
    if (serverScope.value !== server) serverScope.value = server
  },
  { immediate: true },
)
watch(serverScope, (server) => {
  if (server !== (filters.value.server || 'all')) void change({ server })
})
onMounted(() => {
  clock = setInterval(() => {
    now.value = Date.now()
  }, 30000)
  const query = { ...filters.value, server: filters.value.server || 'all' }
  if (!route.query.server) void router.replace({ path: '/players', query })
})
onBeforeUnmount(() => {
  if (debounce) clearTimeout(debounce)
  if (clock) clearInterval(clock)
})
async function nextPage() {
  if (!page.value?.next_cursor) return
  await router.push({
    path: '/players',
    query: {
      ...filters.value,
      server: filters.value.server || 'all',
      cursor: page.value.next_cursor,
      page: String(currentPage.value + 1),
    },
  })
}
async function previousPage() {
  if (!page.value?.previous_cursor) return
  await router.push({
    path: '/players',
    query: {
      ...filters.value,
      server: filters.value.server || 'all',
      cursor: page.value.previous_cursor,
      page: String(Math.max(1, currentPage.value - 1)),
    },
  })
}
function sortBy(key: string) {
  void change({
    sort: key,
    direction:
      filters.value.sort === key && filters.value.direction === 'asc'
        ? 'desc'
        : 'asc',
  })
}
const hasFilters = computed(() =>
  Object.keys(filters.value).some(
    (key) => !['server', 'sort', 'direction', 'limit', 'cursor'].includes(key),
  ),
)
const dateValue = (event: Event) => (event.target as HTMLInputElement).value
</script>

<template>
  <section class="player-registry-view">
    <PageHeader
      title="Player"
      icon="i-lucide-users"
      description="Persistent records of observed players and their known identities"
    >
      <button
        class="compact-button"
        type="button"
        :disabled="status === 'pending'"
        @click="refresh()"
      >
        Refresh
      </button>
    </PageHeader>
    <div
      v-if="page?.ingestion.last_error"
      class="status-banner warning"
      role="status"
    >
      {{ page.ingestion.last_error }}. Last committed records remain available.
    </div>
    <form
      class="panel player-filter-grid"
      aria-label="Player filters"
      @submit.prevent
    >
      <label
        >Server<select
          :value="filters.server || 'all'"
          @change="change({ server: dateValue($event) })"
        >
          <option value="all">All servers</option>
          <option v-for="server in serverOptions" :key="server" :value="server">
            {{ server }}
          </option>
        </select></label
      >
      <label
        >Name or alias<input
          v-model="nameInput"
          type="search"
          placeholder="Character or job name"
          maxlength="64"
          @input="textChange"
      /></label>
      <label
        >Guild<input
          v-model="guildInput"
          type="search"
          placeholder="Guild name"
          maxlength="64"
          @input="textChange"
      /></label>
      <label
        >Minimum level<input
          :value="filters.min_level || ''"
          type="number"
          min="1"
          max="255"
          @change="change({ min_level: dateValue($event) })"
      /></label>
      <label
        >Maximum level<input
          :value="filters.max_level || ''"
          type="number"
          min="1"
          max="255"
          @change="change({ max_level: dateValue($event) })"
      /></label>
      <label
        >Job<select
          :value="filters.job || ''"
          @change="change({ job: dateValue($event) })"
        >
          <option value="">All jobs</option>
          <option v-for="job in playerJobs" :key="job" :value="job">
            {{ playerJobLabel(job) }}
          </option>
        </select></label
      >
      <label
        >Last seen<select
          :value="filters.seen || ''"
          @change="
            change({ seen: dateValue($event), from: undefined, to: undefined })
          "
        >
          <option value="">All time</option>
          <option value="1h">Last hour</option>
          <option value="24h">Last 24 hours</option>
          <option value="7d">Last 7 days</option>
          <option value="30d">Last 30 days</option>
          <option value="custom">Custom range</option>
        </select></label
      >
      <template v-if="filters.seen === 'custom'"
        ><label
          >From<input
            :value="filters.from?.slice(0, 10) || ''"
            type="date"
            @change="change({ from: dateValue($event) })" /></label
        ><label
          >Through<input
            :value="filters.to?.slice(0, 10) || ''"
            type="date"
            @change="change({ to: dateValue($event) })" /></label
      ></template>
      <label
        >Identity status<select
          :value="filters.identity || ''"
          @change="change({ identity: dateValue($event) })"
        >
          <option value="">All identities</option>
          <option value="resolved">Resolved</option>
          <option value="unresolved">Unresolved</option>
        </select></label
      >
      <label
        >Equipment<select
          :value="filters.equipment || ''"
          @change="change({ equipment: dateValue($event) })"
        >
          <option value="">All equipment</option>
          <option value="complete">Complete</option>
          <option value="partial">Partial</option>
          <option value="unavailable">Unavailable</option>
        </select></label
      >
      <button
        class="compact-button"
        type="button"
        @click="router.push({ path: '/players', query: { server: 'all' } })"
      >
        Clear filters
      </button>
    </form>
    <div v-if="error" class="panel empty-state" role="alert">
      <strong>Player registry unavailable</strong>
      <p>Retry to retrieve the latest committed records.</p>
      <button class="compact-button" type="button" @click="refresh()">
        Retry
      </button>
    </div>
    <div
      v-else
      class="panel player-table-panel"
      :aria-busy="status === 'pending'"
    >
      <header class="player-panel-header">
        <h2>
          Observed players
          <span class="count-badge">{{ page?.total ?? '—' }}</span>
        </h2>
        <span v-if="status === 'pending'" role="status">Loading players…</span>
      </header>
      <div v-if="page?.players.length" class="player-table-scroll">
        <table class="player-table">
          <thead>
            <tr>
              <th
                scope="col"
                :aria-sort="
                  filters.sort === 'name'
                    ? filters.direction === 'asc'
                      ? 'ascending'
                      : 'descending'
                    : 'none'
                "
              >
                <button type="button" @click="sortBy('name')">Name ↕</button>
              </th>
              <th
                scope="col"
                :aria-sort="
                  filters.sort === 'level'
                    ? filters.direction === 'asc'
                      ? 'ascending'
                      : 'descending'
                    : 'none'
                "
              >
                <button type="button" @click="sortBy('level')">Level ↕</button>
              </th>
              <th scope="col">
                <button type="button" @click="sortBy('guild')">Guild ↕</button>
              </th>
              <th scope="col">
                <button type="button" @click="sortBy('job')">Job ↕</button>
              </th>
              <th scope="col">Job name</th>
              <th scope="col">Equipment</th>
              <th scope="col">
                <button type="button" @click="sortBy('last_seen')">
                  Last seen ↕
                </button>
              </th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="player in page.players"
              :key="player.id"
              tabindex="0"
              @click="
                router.push({
                  path: `/players/${player.id}`,
                  query: { return_to: route.fullPath },
                })
              "
              @keydown.enter="
                router.push({
                  path: `/players/${player.id}`,
                  query: { return_to: route.fullPath },
                })
              "
            >
              <td>
                <NuxtLink
                  :to="{
                    path: `/players/${player.id}`,
                    query: { return_to: route.fullPath },
                  }"
                  @click.stop
                  >{{ playerLabel(player) }}</NuxtLink
                ><small
                  v-if="
                    player.name &&
                    player.observed_name &&
                    player.observed_name !== player.name
                  "
                  >Observed: {{ player.observed_name }}</small
                ><small
                  >{{ player.server }} ·
                  {{ player.resolved ? 'Resolved' : 'Unresolved' }}</small
                >
              </td>
              <td>{{ player.level ?? '—' }}</td>
              <td>{{ player.guild_name || 'Unknown' }}</td>
              <td>{{ playerJobLabel(player.job) }}</td>
              <td>{{ player.job_name || '—' }}</td>
              <td>
                {{ equipmentLabel(player.gear)
                }}<small v-if="player.gear"
                  >{{
                    player.gear.slots.filter(
                      (slot) => slot.state === 'occupied',
                    ).length
                  }}
                  observed items</small
                >
              </td>
              <td>
                <time
                  :datetime="player.last_seen_at"
                  :title="new Date(player.last_seen_at).toLocaleString()"
                  >{{ playerRelativeTime(player.last_seen_at, now) }}</time
                >
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-else-if="status !== 'pending'" class="empty-state">
        <strong>{{
          hasFilters ? 'No matching players' : 'No players collected yet'
        }}</strong>
        <p>
          {{
            hasFilters
              ? 'Adjust or clear the filters to see other records.'
              : 'Nearby players will appear after a connected plugin observes them.'
          }}
        </p>
      </div>
      <footer class="event-pagination">
        <label
          >Rows<select
            :value="filters.limit || '25'"
            @change="change({ limit: dateValue($event) })"
          >
            <option value="10">10</option>
            <option value="25">25</option>
            <option value="50">50</option>
            <option value="100">100</option>
          </select></label
        >
        <div>
          <button
            class="compact-button"
            type="button"
            :disabled="!page?.previous_cursor || status === 'pending'"
            @click="previousPage"
          >
            Previous</button
          ><span>Page {{ currentPage }}</span
          ><button
            class="compact-button"
            type="button"
            :disabled="!page?.next_cursor || status === 'pending'"
            @click="nextPage"
          >
            Next
          </button>
        </div>
      </footer>
    </div>
  </section>
</template>
