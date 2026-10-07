<script setup lang="ts">
import type { ActivityEvent, AnalyticsMetric } from '~~/shared/types/live'
import { itemRecordFromActivityEvent } from '~/utils/itemDetailPopup'

const route = useRoute()
const {
  eventFeeds,
  analyticsFeeds,
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
const pageSize = ref(10)
const cursor = ref('')
const previousCursors = ref<string[]>([])
const feedID = 'alchemy-attempts'
const page = computed(() => eventFeeds.value[feedID])
const statisticsView = computed(() => route.query.view === 'statistics')
const statisticsFeedID = 'alchemy-statistics'
const statistics = computed(() => analyticsFeeds.value[statisticsFeedID])
const statisticsMetrics = computed(() => statistics.value?.summary || [])
const statistic = (key: string): AnalyticsMetric | undefined =>
  statisticsMetrics.value.find((metric) => metric.key === key)
const summary = computed(() => page.value?.alchemy_summary)
const invalidDateRange = computed(
  () => !!fromDate.value && !!toDate.value && fromDate.value > toDate.value,
)
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
    if (from && to && from > to) {
      clearEventFeed(feedID)
      return
    }
    setEventFeed(feedID, {
      server: server === 'all' ? undefined : server,
      from: from ? localDateBoundary(from, 0) : undefined,
      to: to ? localDateBoundary(to, 1) : undefined,
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
  [statisticsView, serverScope, fromDate, toDate],
  ([enabled, server, from, to]) => {
    if (!enabled) {
      clearAnalyticsFeed(statisticsFeedID)
      return
    }
    setAnalyticsFeed(statisticsFeedID, {
      view: 'alchemy',
      server: server === 'all' ? undefined : server,
      from: from ? localDateBoundary(from, 0) : undefined,
      to: to ? localDateBoundary(to, 1) : undefined,
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
function localDateBoundary(value: string, addDays: number) {
  const boundary = new Date(`${value}T00:00:00`)
  boundary.setDate(boundary.getDate() + addDays)
  return boundary.toISOString()
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
    <template v-if="statisticsView">
      <section class="alchemy-summary-grid" aria-label="Alchemy statistics">
        <article
          v-for="key in [
            'alchemy_attempts',
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
        </article>
      </section>
      <AnalyticsChart
        title="Observed success share by weekday"
        description="Successes divided by attempts with known outcomes, grouped by the selected local timezone. Unknown outcomes are excluded and shown in each bar's details."
        :points="statistics?.time_series || []"
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
          <span>Highest observed plus</span
          ><strong>{{
            summary?.highest_plus == null ? '—' : `+${summary.highest_plus}`
          }}</strong>
        </article>
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
        <div v-if="invalidDateRange" class="status-banner warning" role="alert">
          The start date must be on or before the end date.
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
