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
  jobSourceLabel,
  jobbingLabel,
  registryPlayerName,
  registryJobAlias,
  identityStatusLabel,
  safePlayersReturn,
  textOrUnknown,
} from '~/utils/playerRegistry'
import {
  type PlayerEnrichment,
  visibleGearItemRecord,
  visibleGearLabel,
} from '~/utils/visibleGear'

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
const player = ref<
  (PlayerRecord & { enrichment?: PlayerEnrichment | null }) | null
>(null)
const enrichment = computed(() => player.value?.enrichment ?? null)
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
      $fetch<PlayerRecord & { enrichment?: PlayerEnrichment | null }>(
        `/api/players/${id}`,
      ),
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
      :title="
        (player && registryPlayerName(player)) ||
        player?.observed_name ||
        'Player'
      "
      icon="i-lucide-users"
      description="Player identity, job aliases, and sampled level history. Original sightings are retained."
    />
    <NuxtLink class="player-back" :to="backTo">Back to Players</NuxtLink>
    <p v-if="loading" class="player-note">Loading player…</p>
    <p v-else-if="failed || !player" class="player-note">
      This player could not be loaded.
    </p>
    <template v-else>
      <section class="player-card">
        <h2>Character Information</h2>
        <p v-if="identityStatusLabel(player)" class="player-enrichment-note">
          {{ identityStatusLabel(player) }}
        </p>
        <p
          v-if="player.display_identity?.aliases.length"
          class="player-enrichment-note"
        >
          Job aliases: {{ player.display_identity.aliases.join(', ') }}
        </p>
        <p
          v-if="(player.display_identity?.related_record_ids.length ?? 0) > 1"
          class="player-enrichment-note"
        >
          Original sighting records:
          <NuxtLink
            v-for="recordID in player.display_identity?.related_record_ids"
            :key="recordID"
            :to="{ path: `/players/${recordID}`, query: { back: backTo } }"
            class="player-related-record"
            >{{
              recordID === player.display_identity?.canonical_id
                ? 'Player name'
                : 'Job alias'
            }}{{ recordID === player.id ? ' (current)' : '' }}</NuxtLink
          >
        </p>
        <dl class="player-facts">
          <div>
            <dt>Observed Name</dt>
            <dd>{{ player.observed_name }}</dd>
          </div>
          <div>
            <dt>Player Name</dt>
            <dd>{{ textOrUnknown(registryPlayerName(player)) }}</dd>
          </div>
          <div>
            <dt>Job alias</dt>
            <dd>{{ textOrUnknown(registryJobAlias(player)) }}</dd>
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
            <dd>
              {{ jobLabel(player.job ?? player.display_fallback?.job) }}
              <small
                v-if="
                  player.job == null && player.display_fallback?.job != null
                "
                class="player-identity-source"
              >
                {{ jobSourceLabel(player.display_fallback.job_evidence) }}
              </small>
            </dd>
          </div>
          <div>
            <dt>Job Level</dt>
            <dd>
              {{
                textOrUnknown(
                  player.job_level ?? player.display_fallback?.job_level,
                )
              }}
              <small
                v-if="
                  player.job_level == null &&
                  player.display_fallback?.job_level != null
                "
                class="player-identity-source"
              >
                {{ jobSourceLabel(player.display_fallback.job_level_evidence) }}
              </small>
            </dd>
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
            <dd>
              {{
                formatPlayerTime(
                  player.display_identity?.first_seen_at ??
                    player.first_seen_at,
                )
              }}
            </dd>
          </div>
          <div>
            <dt>Last Seen</dt>
            <dd>
              {{
                formatPlayerTime(
                  player.display_identity?.last_seen_at ?? player.last_seen_at,
                )
              }}
            </dd>
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

      <section
        v-if="
          enrichment &&
          (enrichment.level_observed_at ||
            enrichment.job != null ||
            enrichment.job_level != null ||
            enrichment.job_item_ids != null ||
            enrichment.position_observed_at ||
            enrichment.identity_links?.length)
        "
        class="player-card"
      >
        <h2>ph-proxy enrichment</h2>
        <dl
          v-if="
            enrichment.level_observed_at ||
            enrichment.position_observed_at ||
            enrichment.job != null ||
            enrichment.job_level != null ||
            enrichment.job_item_ids != null
          "
          class="player-facts"
        >
          <div v-if="enrichment.level_observed_at">
            <dt>Level (ph-proxy)</dt>
            <dd>{{ textOrUnknown(enrichment.level) }}</dd>
          </div>
          <div v-if="enrichment.guild">
            <dt>Guild (ph-proxy)</dt>
            <dd>{{ enrichment.guild }}</dd>
          </div>
          <div v-if="enrichment.job != null">
            <dt>Job (ph-proxy)</dt>
            <dd>
              {{ jobLabel(enrichment.job) }} (code {{ enrichment.job }})
              <small class="player-identity-source">{{
                jobSourceLabel(enrichment.field_evidence?.job)
              }}</small>
            </dd>
          </div>
          <div v-if="enrichment.job_level != null">
            <dt>Job Level (ph-proxy)</dt>
            <dd>
              {{ enrichment.job_level }}
              <small class="player-identity-source">{{
                jobSourceLabel(enrichment.field_evidence?.job_level)
              }}</small>
            </dd>
          </div>
          <div v-if="enrichment.job_item_ids != null">
            <dt>Worn job items (ph-proxy observation)</dt>
            <dd>
              {{
                enrichment.job_item_ids.length
                  ? enrichment.job_item_ids.join(', ')
                  : 'None observed'
              }}
            </dd>
          </div>
          <div v-if="enrichment.position_observed_at && enrichment.position">
            <dt>Spawn position</dt>
            <dd>
              region {{ enrichment.position.region }}, x
              {{ enrichment.position.x }}, y {{ enrichment.position.y }}, z
              {{ enrichment.position.z }}
            </dd>
          </div>
        </dl>
        <ul
          v-if="enrichment.identity_links?.length"
          class="player-enrichment-links"
        >
          <li v-for="(link, index) in enrichment.identity_links" :key="index">
            Inferred alias {{ link.alias }} ({{ link.matches }} matches)
          </li>
        </ul>
      </section>

      <section
        v-if="enrichment && enrichment.gear_observed_at"
        class="player-card"
      >
        <h2>Visible Gear (ph-proxy)</h2>
        <p class="player-enrichment-note">
          Observed {{ formatPlayerTime(enrichment.gear_observed_at) }}. This
          observation contains visible item references and verified enhancement
          only. Rolled stats, blues, durability and untransmitted equipment are
          unavailable.
        </p>
        <div
          v-if="enrichment.gear?.length"
          class="equipment-slots player-visible-gear"
        >
          <div
            v-for="(entry, index) in enrichment.gear"
            :key="`${entry.ref_item_id}-${index}`"
            class="equipment-slot"
          >
            <ItemSlot
              :item="visibleGearItemRecord(entry)"
              :label="visibleGearLabel(entry)"
            />
          </div>
        </div>
        <p v-else class="player-note">No visible items were observed.</p>
      </section>

      <section v-if="enrichment?.field_evidence" class="player-card">
        <h2>Enrichment sources</h2>
        <dl class="player-facts">
          <div
            v-for="(evidence, field) in enrichment.field_evidence"
            :key="field"
          >
            <dt>{{ field }}</dt>
            <dd>
              {{ evidence.reason }} ·
              {{ evidence.source_identity_kind || 'character_name' }}
              <span v-if="evidence.source_name">{{
                evidence.source_name
              }}</span>
              · {{ formatPlayerTime(evidence.observed_at) }} · opcode
              {{ evidence.source_opcode }}
              <span v-if="evidence.association">
                · inferred association with {{ evidence.association.alias }} ({{
                  evidence.association.reason
                }}, {{ formatPlayerTime(evidence.association.observed_at) }})
              </span>
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
            <dd>
              {{
                formatPlayerTime(
                  player.display_identity?.first_seen_at ??
                    player.first_seen_at,
                )
              }}
            </dd>
          </div>
          <div>
            <dt>Last Seen</dt>
            <dd>
              {{
                formatPlayerTime(
                  player.display_identity?.last_seen_at ?? player.last_seen_at,
                )
              }}
            </dd>
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
              <tr v-for="row in history?.observations || []" :key="row.id">
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
