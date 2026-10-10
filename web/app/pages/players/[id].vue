<script setup lang="ts">
import type {
  LevelSnapshot,
  PlayerProgress,
  PlayerRecord,
} from '~/utils/playerRegistry'
import {
  formatModel,
  formatPlayerLocation,
  jobLabel,
  jobbingLabel,
  safePlayersReturn,
  textOrUnknown,
} from '~/utils/playerRegistry'

type Observation = {
  id: string
  observed_at: string
  observed_name: string
  level?: number | null
  guild_name?: string | null
  job?: string | null
  job_level?: number | null
  is_jobbing?: boolean | null
  model_id?: number | null
  region?: number | null
  zone?: string
  x?: number | null
  y?: number | null
  z?: number | null
  source: string
}

type ObservationPage = {
  observations: Observation[]
  total: number
  limit: number
  offset: number
}
type LevelPage = {
  snapshots: LevelSnapshot[]
  chart: LevelSnapshot[]
  total: number
  limit: number
  offset: number
  progress: PlayerProgress
}

const route = useRoute()
const player = ref<PlayerRecord | null>(null)
const levels = ref<LevelPage | null>(null)
const history = ref<ObservationPage | null>(null)
const loading = ref(true)
const failed = ref(false)
const openLevel = ref<number | null>(null)
const backTo = computed(() => safePlayersReturn(route.query.back))

function formatPlayerTime(value?: string | null) {
  if (!value) return 'Unknown'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'
  return date.toLocaleString()
}

async function load() {
  loading.value = true
  failed.value = false
  const id = String(route.params.id || '')
  try {
    const [detail, progression, sightings] = await Promise.all([
      $fetch<PlayerRecord>(`/api/players/${id}`),
      $fetch<LevelPage>(`/api/players/${id}/levels`, { query: route.query }),
      $fetch<ObservationPage>(`/api/players/${id}/observations`, {
        query: { limit: 25, offset: route.query.history_offset || 0 },
      }),
    ])
    player.value = detail
    levels.value = progression
    history.value = sightings
  } catch {
    failed.value = true
    player.value = null
  } finally {
    loading.value = false
  }
}

function setLevelPage(offset: number) {
  const next = {
    ...route.query,
    offset: offset > 0 ? String(offset) : undefined,
  }
  if (!next.offset) delete next.offset
  void navigateTo({ query: next })
}

function setHistoryPage(offset: number) {
  const next = {
    ...route.query,
    history_offset: offset > 0 ? String(offset) : undefined,
  }
  if (!next.history_offset) delete next.history_offset
  void navigateTo({ query: next })
}

watch(
  () => [route.params.id, route.query.offset, route.query.history_offset],
  load,
  { immediate: true },
)
</script>

<template>
  <section class="player-page">
    <PageHeader
      :title="player?.observed_name || 'Player'"
      icon="i-lucide-users"
      description="Observed name, job information, and sampled level history. Different names stay separate."
    />
    <NuxtLink class="player-back" :to="backTo">Back to Players</NuxtLink>
    <p v-if="loading" class="player-note">Loading player…</p>
    <p v-else-if="failed || !player" class="player-note">
      This player could not be loaded.
    </p>
    <template v-else>
      <section class="player-card">
        <h2>Character Information</h2>
        <dl class="player-facts">
          <div>
            <dt>Observed Name</dt>
            <dd>{{ player.observed_name }}</dd>
          </div>
          <div>
            <dt>Player Name</dt>
            <dd>{{ textOrUnknown(player.player_name) }}</dd>
          </div>
          <div>
            <dt>Job Name</dt>
            <dd>{{ textOrUnknown(player.job_name) }}</dd>
          </div>
          <div>
            <dt>Server</dt>
            <dd>{{ player.server }}</dd>
          </div>
          <div>
            <dt>Character Level</dt>
            <dd>{{ textOrUnknown(player.level) }}</dd>
          </div>
          <div>
            <dt>Guild</dt>
            <dd>{{ textOrUnknown(player.guild_name) }}</dd>
          </div>
          <div>
            <dt>Job</dt>
            <dd>{{ jobLabel(player.job) }}</dd>
          </div>
          <div>
            <dt>Job Level</dt>
            <dd>{{ textOrUnknown(player.job_level) }}</dd>
          </div>
          <div>
            <dt>Jobbing</dt>
            <dd>{{ jobbingLabel(player.is_jobbing) }}</dd>
          </div>
          <div>
            <dt>Model Name</dt>
            <dd>{{ player.model_name || 'Unknown' }}</dd>
          </div>
          <div>
            <dt>Model ID</dt>
            <dd>{{ textOrUnknown(player.model_id) }}</dd>
          </div>
          <div>
            <dt>First Seen</dt>
            <dd>{{ formatPlayerTime(player.first_seen_at) }}</dd>
          </div>
          <div>
            <dt>Last Seen</dt>
            <dd>{{ formatPlayerTime(player.last_seen_at) }}</dd>
          </div>
          <div>
            <dt>Last Known Location</dt>
            <dd>
              {{
                formatPlayerLocation(
                  player.last_region,
                  player.last_zone,
                  player.last_x,
                  player.last_y,
                )
              }}
            </dd>
          </div>
        </dl>
      </section>

      <section class="player-card">
        <h2>Level Progression</h2>
        <dl v-if="levels?.progress" class="player-facts">
          <div>
            <dt>First Observed</dt>
            <dd>{{ textOrUnknown(levels.progress.first_observed_level) }}</dd>
          </div>
          <div>
            <dt>Latest Level</dt>
            <dd>{{ textOrUnknown(levels.progress.latest_observed_level) }}</dd>
          </div>
          <div>
            <dt>Highest Observed</dt>
            <dd>{{ textOrUnknown(levels.progress.highest_observed_level) }}</dd>
          </div>
          <div>
            <dt>Distinct Levels Observed</dt>
            <dd>{{ levels.progress.distinct_levels_observed }}</dd>
          </div>
          <div>
            <dt>First Seen</dt>
            <dd>{{ formatPlayerTime(player.first_seen_at) }}</dd>
          </div>
          <div>
            <dt>Last Seen</dt>
            <dd>{{ formatPlayerTime(player.last_seen_at) }}</dd>
          </div>
          <div>
            <dt>Latest Observed Level Change</dt>
            <dd>
              {{
                levels.progress.latest_level_change != null
                  ? `${levels.progress.latest_level_change} at ${formatPlayerTime(levels.progress.latest_level_change_at)}`
                  : 'Unknown'
              }}
            </dd>
          </div>
        </dl>
        <p class="player-note">{{ levels?.progress.note }}</p>
        <PlayerLevelChart :snapshots="levels?.chart || []" />
        <div class="event-table-scroll">
          <table class="event-table">
            <thead>
              <tr>
                <th>Level</th>
                <th>First Seen</th>
                <th>Last Seen</th>
                <th>Guild</th>
                <th>Job</th>
                <th>Job Level</th>
                <th>Model</th>
                <th>Location</th>
              </tr>
            </thead>
            <tbody>
              <template
                v-for="snapshot in levels?.snapshots || []"
                :key="snapshot.id"
              >
                <tr
                  tabindex="0"
                  @click="
                    openLevel =
                      openLevel === snapshot.level ? null : snapshot.level
                  "
                  @keydown.enter="
                    openLevel =
                      openLevel === snapshot.level ? null : snapshot.level
                  "
                >
                  <td>{{ snapshot.level }}</td>
                  <td>{{ formatPlayerTime(snapshot.first_seen_at) }}</td>
                  <td>{{ formatPlayerTime(snapshot.last_seen_at) }}</td>
                  <td>{{ textOrUnknown(snapshot.guild_name) }}</td>
                  <td>{{ jobLabel(snapshot.job) }}</td>
                  <td>{{ textOrUnknown(snapshot.job_level) }}</td>
                  <td>
                    {{ formatModel(snapshot.model_name, snapshot.model_id) }}
                  </td>
                  <td>
                    {{
                      formatPlayerLocation(
                        snapshot.region,
                        snapshot.zone,
                        snapshot.x,
                        snapshot.y,
                      )
                    }}
                  </td>
                </tr>
                <tr v-if="openLevel === snapshot.level">
                  <td colspan="8">
                    <dl class="player-facts">
                      <div>
                        <dt>Level</dt>
                        <dd>{{ snapshot.level }}</dd>
                      </div>
                      <div>
                        <dt>First Seen</dt>
                        <dd>{{ formatPlayerTime(snapshot.first_seen_at) }}</dd>
                      </div>
                      <div>
                        <dt>Last Seen</dt>
                        <dd>{{ formatPlayerTime(snapshot.last_seen_at) }}</dd>
                      </div>
                      <div>
                        <dt>Player Name</dt>
                        <dd>{{ textOrUnknown(snapshot.player_name) }}</dd>
                      </div>
                      <div>
                        <dt>Guild</dt>
                        <dd>{{ textOrUnknown(snapshot.guild_name) }}</dd>
                      </div>
                      <div>
                        <dt>Job</dt>
                        <dd>{{ jobLabel(snapshot.job) }}</dd>
                      </div>
                      <div>
                        <dt>Job Level</dt>
                        <dd>{{ textOrUnknown(snapshot.job_level) }}</dd>
                      </div>
                      <div>
                        <dt>Jobbing</dt>
                        <dd>{{ jobbingLabel(snapshot.is_jobbing) }}</dd>
                      </div>
                      <div>
                        <dt>Model</dt>
                        <dd>
                          {{
                            formatModel(snapshot.model_name, snapshot.model_id)
                          }}
                        </dd>
                      </div>
                      <div>
                        <dt>Region</dt>
                        <dd>{{ textOrUnknown(snapshot.region) }}</dd>
                      </div>
                      <div>
                        <dt>Position</dt>
                        <dd>
                          {{
                            snapshot.x != null && snapshot.y != null
                              ? `${snapshot.x}, ${snapshot.y}`
                              : 'Unknown'
                          }}
                        </dd>
                      </div>
                    </dl>
                  </td>
                </tr>
              </template>
            </tbody>
          </table>
        </div>
        <div
          v-if="(levels?.total || 0) > (levels?.limit || 25)"
          class="player-pager"
        >
          <button
            type="button"
            :disabled="(levels?.offset || 0) === 0"
            @click="setLevelPage((levels?.offset || 0) - (levels?.limit || 25))"
          >
            Previous
          </button>
          <button
            type="button"
            :disabled="
              (levels?.offset || 0) + (levels?.limit || 25) >=
              (levels?.total || 0)
            "
            @click="setLevelPage((levels?.offset || 0) + (levels?.limit || 25))"
          >
            Next
          </button>
        </div>
      </section>

      <section class="player-card">
        <h2>Sighting History</h2>
        <div class="event-table-scroll">
          <table class="event-table">
            <thead>
              <tr>
                <th>Timestamp</th>
                <th>Observed Name</th>
                <th>Level</th>
                <th>Guild</th>
                <th>Job</th>
                <th>Job Level</th>
                <th>Jobbing</th>
                <th>Model</th>
                <th>Region/Zone</th>
                <th>Coordinates</th>
                <th>Source</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="row in history?.observations || []"
                :key="row.id"
              >
                <td>{{ formatPlayerTime(row.observed_at) }}</td>
                <td>{{ row.observed_name }}</td>
                <td>{{ textOrUnknown(row.level) }}</td>
                <td>{{ textOrUnknown(row.guild_name) }}</td>
                <td>{{ jobLabel(row.job) }}</td>
                <td>{{ textOrUnknown(row.job_level) }}</td>
                <td>{{ jobbingLabel(row.is_jobbing) }}</td>
                <td>{{ textOrUnknown(row.model_id) }}</td>
                <td>{{ row.zone || textOrUnknown(row.region) }}</td>
                <td>
                  {{
                    row.x != null && row.y != null
                      ? `${row.x}, ${row.y}`
                      : 'Unknown'
                  }}
                </td>
                <td>{{ row.source }}</td>
              </tr>
            </tbody>
          </table>
        </div>
        <p v-if="(history?.total || 0) === 0" class="player-note">
          No sightings stored.
        </p>
        <div
          v-if="(history?.total || 0) > (history?.limit || 25)"
          class="player-pager"
        >
          <button
            type="button"
            :disabled="(history?.offset || 0) === 0"
            @click="
              setHistoryPage((history?.offset || 0) - (history?.limit || 25))
            "
          >
            Previous
          </button>
          <button
            type="button"
            :disabled="
              (history?.offset || 0) + (history?.limit || 25) >=
              (history?.total || 0)
            "
            @click="
              setHistoryPage((history?.offset || 0) + (history?.limit || 25))
            "
          >
            Next
          </button>
        </div>
      </section>
    </template>
  </section>
</template>
