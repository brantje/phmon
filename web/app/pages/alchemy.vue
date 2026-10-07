<script setup lang="ts">
import type { ActivityEvent, AnalyticsMetric } from '~~/shared/types/live'
import { itemRecordFromActivityEvent } from '~/utils/itemDetailPopup'
import { normalizeAnalyticsDateRange } from '~/utils/analyticsDateRange'
import { buildAlchemyAttemptSegments } from '~/utils/alchemyAttemptSegments'

const route = useRoute()
const {
  eventFeeds,
  analyticsFeeds,
  analyticsFeedStatus,
  connectionState,
  liveStale,
  setEventFeed,
  clearEventFeed,
  setAnalyticsFeed,
  clearAnalyticsFeed,
} = useLiveData()
const { serverScope } = useServerScope()
const fromDate = ref(dateInput(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000)))
const toDate = ref(dateInput(new Date()))
const characterInput = ref('')
const characterQuery = ref('')
const itemInput = ref('')
const itemQuery = ref('')
const itemType = ref('')
const itemDegree = ref('')
const pageSize = ref(10)
const cursor = ref('')
const previousCursors = ref<string[]>([])
const feedID = 'alchemy-attempts'
const page = computed(() => eventFeeds.value[feedID])
const statisticsView = computed(() => route.query.view === 'statistics')
const statisticsFeedID = 'alchemy-statistics'
const statistics = computed(() => analyticsFeeds.value[statisticsFeedID])
const statisticsStatus = computed(
  () => analyticsFeedStatus.value[statisticsFeedID] || 'loading',
)
const statisticsMetrics = computed(() => statistics.value?.summary || [])
const taxonomyTypes = computed(() =>
  [
    ...new Set(
      (statistics.value?.taxonomy_options || []).map((option) => option.type),
    ),
  ].sort(),
)
const taxonomyDegrees = computed(() =>
  [
    ...new Set(
      (statistics.value?.taxonomy_options || [])
        .filter((option) => !itemType.value || option.type === itemType.value)
        .map((option) => option.degree)
        .filter((degree): degree is string => !!degree),
    ),
  ].sort((a, b) => Number(a) - Number(b)),
)
const statistic = (key: string): AnalyticsMetric | undefined =>
  statisticsMetrics.value.find((metric) => metric.key === key)
const summary = computed(() => page.value?.alchemy_summary)
const attemptSegments = computed(() =>
  buildAlchemyAttemptSegments(page.value?.events || []),
)
const latestAttempt = computed(() => statistics.value?.occurrences?.[0])
const normalizedDates = computed(() =>
  normalizeAnalyticsDateRange(fromDate.value, toDate.value),
)
const dateRangeError = computed(() => normalizedDates.value.error)
const invalidDateRange = computed(() => !!dateRangeError.value)
let characterTimer: ReturnType<typeof setTimeout> | undefined
let itemTimer: ReturnType<typeof setTimeout> | undefined

watch(characterInput, (value) => {
  if (characterTimer) clearTimeout(characterTimer)
  characterTimer = setTimeout(() => {
    characterQuery.value = value.trim()
  }, 250)
})
watch(itemInput, (value) => {
  if (itemTimer) clearTimeout(itemTimer)
  itemTimer = setTimeout(() => {
    itemQuery.value = value.trim()
  }, 250)
})
watch(
  [serverScope, fromDate, toDate, characterQuery, itemQuery, cursor, pageSize],
  ([server, from, to, character, item, pageCursor, size]) => {
    const dates = normalizeAnalyticsDateRange(from, to)
    if (dates.error) {
      clearEventFeed(feedID)
      return
    }
    setEventFeed(feedID, {
      server: server === 'all' ? undefined : server,
      from: dates.from,
      to: dates.to,
      q: character || undefined,
      item: item || undefined,
      kind: 'alchemy.attempt',
      cursor: pageCursor || undefined,
      limit: size,
    })
  },
  { immediate: true },
)
watch(
  [
    statisticsView,
    serverScope,
    fromDate,
    toDate,
    characterQuery,
    itemQuery,
    itemType,
    itemDegree,
  ],
  ([enabled, server, from, to, character, item, type, degree]) => {
    const dates = normalizeAnalyticsDateRange(from, to)
    if (!enabled || dates.error) {
      clearAnalyticsFeed(statisticsFeedID)
      return
    }
    setAnalyticsFeed(statisticsFeedID, {
      view: 'alchemy',
      server: server === 'all' ? undefined : server,
      from: dates.from,
      to: dates.to,
      q: character || undefined,
      item: item || undefined,
      item_type: type || undefined,
      item_degree: degree || undefined,
      timezone: Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC',
      bucket: 'day',
      group_by: 'character',
      page_size: 25,
    })
  },
  { immediate: true },
)
watch(
  [serverScope, fromDate, toDate, characterQuery, itemQuery, pageSize],
  () => {
    cursor.value = ''
    previousCursors.value = []
  },
)
onBeforeUnmount(() => {
  if (characterTimer) clearTimeout(characterTimer)
  if (itemTimer) clearTimeout(itemTimer)
  clearEventFeed(feedID)
  clearAnalyticsFeed(statisticsFeedID)
})

function nextPage() {
  if (!page.value?.next_cursor) return
  previousCursors.value.push(cursor.value)
  cursor.value = page.value.next_cursor
}
function previousPage() {
  if (previousCursors.value.length)
    cursor.value = previousCursors.value.pop() || ''
}
function resetFilters() {
  characterInput.value = ''
  characterQuery.value = ''
  itemInput.value = ''
  itemQuery.value = ''
  fromDate.value = dateInput(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000))
  toDate.value = dateInput(new Date())
  cursor.value = ''
  previousCursors.value = []
}
function outcome(event: ActivityEvent) {
  if (typeof event.payload.success !== 'boolean') return 'Unknown'
  return event.payload.success ? 'Success' : 'Failure'
}
function dateInput(value: Date) {
  const local = new Date(value.getTime() - value.getTimezoneOffset() * 60000)
  return local.toISOString().slice(0, 10)
}
</script>

<template>
  <div class="events-page">
    <PageHeader
      :title="`Alchemy · ${statisticsView ? 'Statistics' : 'Sessions'}`"
      icon="i-lucide-flask-conical"
      description="Recorded attempt outcomes and item-session statistics, using only accepted phBot observations."
    />
    <nav class="events-tabs" aria-label="Alchemy views">
      <NuxtLink
        class="compact-button"
        :class="{ selected: !statisticsView }"
        :to="{ path: '/alchemy', query: { ...route.query, view: 'sessions' } }"
        >Sessions</NuxtLink
      >
      <NuxtLink
        class="compact-button"
        :class="{ selected: statisticsView }"
        :to="{
          path: '/alchemy',
          query: { ...route.query, view: 'statistics' },
        }"
        >Statistics</NuxtLink
      >
    </nav>
    <div v-if="invalidDateRange" class="status-banner warning" role="alert">
      {{ dateRangeError }}
    </div>
    <div
      v-else-if="statisticsView && statisticsStatus === 'unavailable'"
      class="status-banner warning"
      role="status"
    >
      Alchemy Statistics are temporarily unavailable. Retry the live connection
      to reload this range.
    </div>
    <div
      v-else-if="statisticsView && liveStale"
      class="status-banner warning"
      role="status"
    >
      Showing the last delivered Alchemy Statistics snapshot as stale.
    </div>
    <template v-if="statisticsView">
      <section class="events-filters" aria-label="Alchemy statistics filters">
        <label
          ><span>Character</span
          ><input
            v-model="characterInput"
            maxlength="64"
            placeholder="Filter by character"
        /></label>
        <label
          ><span>Item</span
          ><input
            v-model="itemInput"
            maxlength="128"
            placeholder="Filter by item"
        /></label>
        <label
          ><span>Item type</span
          ><select v-model="itemType">
            <option value="">All types</option>
            <option v-for="type in taxonomyTypes" :key="type" :value="type">
              {{ type }}
            </option>
          </select></label
        >
        <label
          ><span>Degree</span
          ><select v-model="itemDegree">
            <option value="">All degrees</option>
            <option
              v-for="degree in taxonomyDegrees"
              :key="degree"
              :value="degree"
            >
              {{ degree }}
            </option>
          </select></label
        >
        <label
          ><span>From date</span><input v-model="fromDate" type="date"
        /></label>
        <label
          ><span>To date</span><input v-model="toDate" type="date"
        /></label>
        <button class="compact-button" type="button" @click="resetFilters">
          Reset
        </button>
      </section>
      <p
        v-if="statisticsStatus === 'loading' && !statistics"
        class="status-banner"
        role="status"
      >
        Loading Alchemy Statistics…
      </p>
      <p
        v-else-if="statistics?.status === 'empty'"
        class="status-banner"
        role="status"
      >
        {{ statistics.reason || 'No attempts match these filters.' }}
      </p>
      <section class="alchemy-summary-grid" aria-label="Alchemy statistics">
        <article
          v-for="key in [
            'alchemy_attempts',
            'alchemy_successes',
            'alchemy_failures',
            'alchemy_unknown',
            'alchemy_success_rate',
            'alchemy_highest_plus',
            'alchemy_reach_target',
          ]"
          :key="key"
          class="panel alchemy-summary-card"
        >
          <span>{{ statistic(key)?.label || key }}</span>
          <strong>{{
            statistic(key)?.value ||
            (statistic(key)?.number == null
              ? 'Unavailable'
              : `${statistic(key)?.number?.toLocaleString(undefined, { maximumFractionDigits: 2 })}${statistic(key)?.unit === '%' ? '%' : ''}`)
          }}</strong>
          <small v-if="statistic(key)?.reason">{{
            statistic(key)?.reason
          }}</small>
          <small v-if="key === 'alchemy_success_rate'">
            Known-outcome denominator:
            {{ statistic('alchemy_successes')?.value || '0' }} successes +
            {{ statistic('alchemy_failures')?.value || '0' }} failures
          </small>
        </article>
        <article class="panel alchemy-summary-card alchemy-latest-attempt">
          <span>Latest recorded attempt</span>
          <strong v-if="latestAttempt">
            {{ latestAttempt.character || 'Character' }} ·
            {{
              latestAttempt.payload.success === true
                ? 'Success'
                : latestAttempt.payload.success === false
                  ? 'Failure'
                  : 'Outcome unknown'
            }}
          </strong>
          <strong v-else>Unavailable</strong>
          <small v-if="latestAttempt"
            >{{ formatTimestamp(latestAttempt.occurred_at) }} ·
            {{
              latestAttempt.item_name ||
              latestAttempt.item_code ||
              'Item details unavailable'
            }}</small
          >
          <small v-else>There are no matching recorded attempts.</small>
        </article>
      </section>
      <AnalyticsChart
        title="Observed success share by weekday"
        description="Successes divided by attempts with known outcomes, grouped by the selected local timezone. Unknown outcomes are excluded and shown in each bar's details."
        :points="statistics?.time_series || []"
        variant="category"
        :category-labels="[
          'Monday',
          'Tuesday',
          'Wednesday',
          'Thursday',
          'Friday',
          'Saturday',
          'Sunday',
        ]"
        unit="percent"
        :zero-fill-missing="false"
        empty-label="No attempts with known outcomes were recorded in this range."
      />
      <section class="panel events-panel">
        <h2>Recorded attempt counts by character</h2>
        <p>Observed attempts, not a prediction of future alchemy results.</p>
        <ol class="analytics-breakdown">
          <li v-for="point in statistics?.breakdown || []" :key="point.label">
            <span>{{ point.label }}</span
            ><strong>{{ Number(point.value).toLocaleString() }}</strong>
          </li>
        </ol>
      </section>
    </template>
    <template v-else>
      <section
        class="alchemy-summary-grid"
        aria-label="Alchemy attempt summary"
      >
        <article class="panel alchemy-summary-card">
          <span>Recorded attempts</span
          ><strong>{{ summary?.attempts ?? '—' }}</strong>
        </article>
        <article class="panel alchemy-summary-card">
          <span>Successes recorded</span
          ><strong>{{ summary?.successes ?? '—' }}</strong>
        </article>
        <article class="panel alchemy-summary-card">
          <span>Failures recorded</span
          ><strong>{{ summary?.failures ?? '—' }}</strong>
        </article>
        <article class="panel alchemy-summary-card">
          <span>Unknown outcomes</span
          ><strong>{{ summary?.unknown ?? '—' }}</strong>
        </article>
        <article class="panel alchemy-summary-card">
          <span>Highest observed plus</span
          ><strong>{{
            summary?.highest_plus == null ? '—' : `+${summary.highest_plus}`
          }}</strong>
        </article>
      </section>

      <section
        class="panel events-panel alchemy-attempt-segments"
        aria-label="Conservative alchemy attempt segments"
      >
        <header class="panel-header compact">
          <div>
            <h2>Conservative attempt segments</h2>
            <p>
              Candidate runs from this visible page only. Same session,
              character, slot, stable item traits and a gap of at most one
              minute are required; every run remains ambiguous because these
              observations do not prove physical item continuity.
            </p>
          </div>
        </header>
        <div class="event-table-scroll">
          <table class="event-table">
            <thead>
              <tr>
                <th>Observed period</th>
                <th>Character / item</th>
                <th>Slot</th>
                <th>Attempts</th>
                <th>Success / failure / unknown</th>
                <th>Continuity</th>
              </tr>
            </thead>
            <tbody>
              <tr
                v-for="segment in attemptSegments"
                :key="`${segment.key}|${segment.firstAt}`"
              >
                <td>
                  {{ formatTimestamp(segment.firstAt)
                  }}<span v-if="segment.lastAt !== segment.firstAt">
                    – {{ formatTimestamp(segment.lastAt) }}</span
                  >
                </td>
                <td>{{ segment.character }} · {{ segment.item }}</td>
                <td>{{ segment.slot ?? 'Unknown' }}</td>
                <td>{{ segment.attempts }}</td>
                <td>
                  {{ segment.successes }} / {{ segment.failures }} /
                  {{ segment.unknown }}
                </td>
                <td>
                  <span class="status-chip warning">Ambiguous candidate</span>
                </td>
              </tr>
              <tr v-if="!attemptSegments.length">
                <td colspan="6" class="event-table-empty">
                  No attempt segments are available on this page.
                </td>
              </tr>
            </tbody>
          </table>
        </div>
      </section>

      <section
        class="panel events-panel"
        aria-label="Recorded alchemy attempts"
      >
        <div class="events-tabs">
          <span class="compact-button selected">Sessions</span
          ><span class="event-count">{{ summary?.attempts ?? '—' }}</span>
        </div>
        <div class="events-filters">
          <label
            ><span>Character</span
            ><input
              v-model="characterInput"
              maxlength="64"
              placeholder="Filter by character"
          /></label>
          <label
            ><span>Item</span
            ><input
              v-model="itemInput"
              maxlength="128"
              placeholder="Filter by item"
          /></label>
          <label
            ><span>From date</span><input v-model="fromDate" type="date"
          /></label>
          <label
            ><span>To date</span><input v-model="toDate" type="date"
          /></label>
          <button class="compact-button" type="button" @click="resetFilters">
            Reset
          </button>
        </div>
        <div v-if="liveStale" class="status-banner warning" role="status">
          Showing the last received attempt page as stale while PhMon
          reconnects.
        </div>
        <div class="event-table-scroll">
          <table class="event-table">
            <thead>
              <tr>
                <th>Time</th>
                <th>Character</th>
                <th>Item</th>
                <th>Outcome</th>
                <th>Observed plus</th>
                <th>Slot</th>
              </tr>
            </thead>
            <tbody>
              <tr v-for="event in page?.events || []" :key="event.event_id">
                <td>
                  <time :datetime="event.occurred_at">{{
                    formatTimestamp(event.occurred_at)
                  }}</time>
                </td>
                <td>
                  <NuxtLink
                    v-if="event.character_id"
                    class="event-character-link"
                    :to="`/characters/${event.character_id}`"
                    >{{ event.character || 'Character' }}</NuxtLink
                  ><span v-else>—</span>
                </td>
                <td>
                  <ItemDetailPopup
                    v-if="itemRecordFromActivityEvent(event)"
                    :item="itemRecordFromActivityEvent(event)!"
                  />
                  <span v-else>Item unknown</span>
                </td>
                <td>{{ outcome(event) }}</td>
                <td>
                  {{
                    typeof event.payload.plus === 'number'
                      ? `+${event.payload.plus}`
                      : '—'
                  }}
                </td>
                <td>
                  {{
                    typeof event.payload.slot === 'number'
                      ? event.payload.slot
                      : '—'
                  }}
                </td>
              </tr>
            </tbody>
          </table>
          <div v-if="!page?.events.length" class="event-empty-state">
            <strong>{{
              connectionState === 'current'
                ? 'No attempts found'
                : 'Loading alchemy history'
            }}</strong>
            <span>{{
              connectionState === 'current'
                ? 'No recorded attempts match these filters.'
                : 'Waiting for a current event snapshot.'
            }}</span>
          </div>
        </div>
        <footer class="event-pagination">
          <label
            ><span>Entries per page</span
            ><select v-model.number="pageSize">
              <option :value="10">10</option>
              <option :value="25">25</option>
              <option :value="50">50</option>
            </select></label
          >
          <div>
            <button
              class="compact-button"
              type="button"
              :disabled="!previousCursors.length"
              @click="previousPage"
            >
              Previous
            </button>
            <span
              >Page {{ previousCursors.length + 1 }} ·
              {{ summary?.attempts || 0 }} attempts</span
            >
            <button
              class="compact-button"
              type="button"
              :disabled="!page?.next_cursor"
              @click="nextPage"
            >
              Next
            </button>
          </div>
        </footer>
      </section>
    </template>
  </div>
</template>
