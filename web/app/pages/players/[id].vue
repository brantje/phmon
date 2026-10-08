<script setup lang="ts">
import type {
  PlayerRecord,
  PlayerAlias,
  PlayerObservation,
  PlayerEquipmentHistory,
  PlayerHistoryPage,
} from '~~/shared/types/players'
import {
  safePlayerReturn,
  playerJobLabel,
  playerLabel,
} from '~/utils/playerRegistry'
definePageMeta({
  validate: (route) =>
    typeof route.params.id === 'string' &&
    /^[0-9a-f]{8}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{4}-[0-9a-f]{12}$/i.test(
      route.params.id,
    ),
})
const route = useRoute()
const id = computed(() => String(route.params.id))
const {
  data: player,
  status,
  error,
  refresh,
} = useFetch<PlayerRecord>(() => `/api/players/${id.value}`, {
  query: computed(() => ({
    source: route.query.source === '1' ? '1' : undefined,
  })),
  server: false,
})
const historyCursor = ref(''),
  aliasCursor = ref(''),
  observationCursor = ref('')
const {
  data: history,
  error: historyError,
  refresh: refreshHistory,
} = useFetch<PlayerHistoryPage<PlayerEquipmentHistory>>(
  () => `/api/players/${id.value}/equipment/history`,
  {
    query: computed(() => ({ cursor: historyCursor.value, limit: 10 })),
    server: false,
  },
)
const {
  data: aliases,
  error: aliasError,
  refresh: refreshAliases,
} = useFetch<PlayerHistoryPage<PlayerAlias>>(
  () => `/api/players/${id.value}/aliases`,
  {
    query: computed(() => ({ cursor: aliasCursor.value, limit: 25 })),
    server: false,
  },
)
const {
  data: observations,
  error: observationError,
  refresh: refreshObservations,
} = useFetch<PlayerHistoryPage<PlayerObservation>>(
  () => `/api/players/${id.value}/observations`,
  {
    query: computed(() => ({ cursor: observationCursor.value, limit: 25 })),
    server: false,
  },
)
const selectedHistory = ref<PlayerEquipmentHistory | null>(null)
watch(id, () => {
  selectedHistory.value = null
  historyCursor.value = ''
  aliasCursor.value = ''
  observationCursor.value = ''
})
const linked = computed(() => player.value && player.value.id !== id.value)
async function changed() {
  selectedHistory.value = null
  await Promise.all([
    refresh(),
    refreshAliases(),
    refreshHistory(),
    refreshObservations(),
  ])
}
</script>
<template>
  <section class="player-profile-view">
    <PageHeader
      :title="player ? playerLabel(player) : 'Player profile'"
      icon="i-lucide-user-round"
      :description="
        player
          ? `${player.server} · ${player.resolved ? 'Resolved identity' : 'Unresolved identity'}`
          : 'Persistent player observations'
      "
      ><NuxtLink
        class="compact-button"
        :to="safePlayerReturn(route.query.return_to)"
        >Back to players</NuxtLink
      ><button class="compact-button" type="button" @click="changed">
        Refresh
      </button></PageHeader
    >
    <div v-if="error" class="panel empty-state" role="alert">
      <strong>{{
        error.statusCode === 404
          ? 'Player not found'
          : 'Player profile unavailable'
      }}</strong>
      <p>Check the player ID or retry the request.</p>
      <button class="compact-button" type="button" @click="changed">
        Retry
      </button>
    </div>
    <p v-else-if="!player" class="panel empty-state" role="status">
      Loading player profile…
    </p>
    <template v-else>
      <div v-if="linked" class="status-banner" role="status">
        This identity is associated with a canonical player.
        <NuxtLink
          :to="{
            path: `/players/${player.id}`,
            query: { return_to: safePlayerReturn(route.query.return_to) },
          }"
          >Open canonical profile</NuxtLink
        >
        ·
        <NuxtLink :to="{ path: `/players/${id}`, query: { source: '1' } }"
          >Inspect original record</NuxtLink
        >
      </div>
      <section
        class="panel player-profile-panel"
        :aria-busy="status === 'pending'"
      >
        <header class="player-panel-header">
          <h2>Character details</h2>
          <span>{{ player.resolved ? 'Resolved' : 'Unresolved' }}</span>
        </header>
        <dl class="player-detail-grid">
          <div>
            <dt>Normal name</dt>
            <dd>{{ player.name || 'Unknown' }}</dd>
          </div>
          <div>
            <dt>Observed name</dt>
            <dd>{{ player.observed_name }}</dd>
          </div>
          <div>
            <dt>Level</dt>
            <dd>{{ player.level ?? 'Unknown' }}</dd>
          </div>
          <div>
            <dt>Server</dt>
            <dd>{{ player.server }}</dd>
          </div>
          <div>
            <dt>Guild</dt>
            <dd>{{ player.guild_name ?? 'Unknown' }}</dd>
          </div>
          <div>
            <dt>Latest observed job</dt>
            <dd>{{ playerJobLabel(player.job) }}</dd>
          </div>
          <div>
            <dt>Job name</dt>
            <dd>{{ player.job_name || 'Unknown' }}</dd>
          </div>
          <div>
            <dt>First seen</dt>
            <dd>
              <time :datetime="player.first_seen_at">{{
                new Date(player.first_seen_at).toLocaleString()
              }}</time>
            </dd>
          </div>
          <div>
            <dt>Last seen</dt>
            <dd>
              <time :datetime="player.last_seen_at">{{
                new Date(player.last_seen_at).toLocaleString()
              }}</time>
            </dd>
          </div>
          <div>
            <dt>Last known location</dt>
            <dd v-if="player.location">
              {{ player.location.zone || `Region ${player.location.region}` }} ·
              {{ player.location.x.toFixed(1) }},
              {{ player.location.y.toFixed(1)
              }}<small
                >Observed
                {{
                  new Date(player.location.observed_at).toLocaleString()
                }}</small
              >
            </dd>
            <dd v-else>Unknown</dd>
          </div>
          <div>
            <dt>Player ID</dt>
            <dd class="player-id">{{ player.id }}</dd>
          </div>
        </dl>
        <details v-if="player.source_ids.length > 1">
          <summary>Original source records</summary>
          <ul>
            <li v-for="source in player.source_ids" :key="source">
              <NuxtLink
                :to="{ path: `/players/${source}`, query: { source: '1' } }"
                >{{ source }}</NuxtLink
              >
            </li>
          </ul>
        </details>
      </section>
      <section class="panel player-profile-panel">
        <header class="player-panel-header">
          <h2>
            {{
              selectedHistory
                ? 'Historical equipment'
                : 'Latest known equipment'
            }}
          </h2>
          <button
            v-if="selectedHistory"
            class="compact-button"
            type="button"
            @click="selectedHistory = null"
          >
            Latest equipment
          </button>
        </header>
        <PlayerEquipment
          :equipment="selectedHistory?.equipment || player.gear"
        />
        <details>
          <summary>Equipment fingerprints</summary>
          <p class="player-id">
            {{
              selectedHistory?.gear_hash ||
              player.gear_hash ||
              'Gear fingerprint unavailable'
            }}
          </p>
          <p class="player-id">
            {{
              selectedHistory?.identity_gear_hash ||
              player.identity_gear_hash ||
              'Identity fingerprint unavailable: comparable profile not verified'
            }}
          </p>
        </details>
      </section>
      <section class="panel player-profile-panel">
        <h2>Equipment history</h2>
        <p v-if="historyError" role="alert">Equipment history unavailable.</p>
        <p v-else-if="!history?.items.length" class="player-muted">
          No observed equipment configurations recorded.
        </p>
        <div class="player-history-list">
          <button
            v-for="snapshot in history?.items"
            :key="snapshot.id"
            class="compact-button"
            type="button"
            :aria-pressed="selectedHistory?.id === snapshot.id"
            @click="selectedHistory = snapshot"
          >
            {{ new Date(snapshot.first_seen_at).toLocaleString() }} —
            {{ new Date(snapshot.last_seen_at).toLocaleString() }} ·
            {{
              snapshot.equipment.slots.filter(
                (slot) => slot.state === 'occupied',
              ).length
            }}
            items
          </button>
        </div>
        <div class="player-actions">
          <button
            v-if="history?.next_cursor"
            class="compact-button"
            type="button"
            @click="historyCursor = history.next_cursor || ''"
          >
            Older equipment</button
          ><button
            v-if="historyCursor"
            class="compact-button"
            type="button"
            @click="historyCursor = ''"
          >
            Latest history
          </button>
        </div>
      </section>
      <section class="panel player-profile-panel">
        <h2>Aliases and job history</h2>
        <p v-if="aliasError" role="alert">Alias history unavailable.</p>
        <div v-else class="player-table-scroll">
          <table class="player-table">
            <thead>
              <tr>
                <th>Name</th>
                <th>Identity</th>
                <th>Job</th>
                <th>First seen</th>
                <th>Last seen</th>
                <th>Method</th>
                <th>Status</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="alias in aliases?.items" :key="alias.id">
                <td>{{ alias.alias_name }}</td>
                <td>{{ alias.alias_type }}</td>
                <td>{{ playerJobLabel(alias.job_type) }}</td>
                <td>{{ new Date(alias.first_seen_at).toLocaleString() }}</td>
                <td>{{ new Date(alias.last_seen_at).toLocaleString() }}</td>
                <td>{{ alias.match_method }}</td>
                <td>{{ alias.confirmation_status }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <div class="player-actions">
          <button
            v-if="aliases?.next_cursor"
            class="compact-button"
            type="button"
            @click="aliasCursor = aliases.next_cursor || ''"
          >
            Older aliases</button
          ><button
            v-if="aliasCursor"
            class="compact-button"
            type="button"
            @click="aliasCursor = ''"
          >
            Latest aliases
          </button>
        </div>
      </section>
      <PlayerIdentityReview
        :key="player.id"
        :player="player"
        @changed="changed"
      />
      <section class="panel player-profile-panel">
        <h2>Recent sightings</h2>
        <p class="player-muted">
          Repeated sightings are checkpointed. Routine history is retained for
          90 days by default; identity and equipment evidence is preserved.
        </p>
        <p v-if="observationError" role="alert">Sightings unavailable.</p>
        <div v-else class="player-table-scroll">
          <table class="player-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Observed name</th>
                <th>Job</th>
                <th>Location</th>
                <th>Source</th>
                <th>Evidence</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="observation in observations?.items"
                :key="observation.id"
              >
                <td>
                  {{ new Date(observation.observed_at).toLocaleString() }}
                </td>
                <td>{{ observation.observed_name }}</td>
                <td>{{ playerJobLabel(observation.job_type) }}</td>
                <td>
                  {{
                    observation.location
                      ? `${observation.location.region}: ${observation.location.x.toFixed(1)}, ${observation.location.y.toFixed(1)}`
                      : 'Unknown'
                  }}
                </td>
                <td>
                  {{ observation.source
                  }}<small v-if="observation.observer_character_id"
                    >Observer {{ observation.observer_character_id }}</small
                  >
                </td>
                <td>
                  <details>
                    <summary>Observation</summary>
                    <pre>{{ JSON.stringify(observation, null, 2) }}</pre>
                  </details>
                </td>
              </tr>
            </tbody>
          </table>
          <p v-if="!observations?.items.length" class="player-muted">
            No retained routine sightings.
          </p>
        </div>
        <div class="player-actions">
          <button
            v-if="observations?.next_cursor"
            class="compact-button"
            type="button"
            @click="observationCursor = observations.next_cursor || ''"
          >
            Older sightings</button
          ><button
            v-if="observationCursor"
            class="compact-button"
            type="button"
            @click="observationCursor = ''"
          >
            Latest sightings
          </button>
        </div>
      </section>
    </template>
  </section>
</template>
