<script setup lang="ts">
import type { AnalyticsMetric, AnalyticsOccurrence } from '~~/shared/types/live'
import { itemRecordFromActivityEvent } from '~/utils/itemDetailPopup'

const route = useRoute()
const router = useRouter()
const {
  analyticsFeeds,
  analyticsFeedStatus,
  connectionState,
  liveStale,
  setAnalyticsFeed,
  clearAnalyticsFeed,
  refreshLiveData,
} = useLiveData()
const { serverScope } = useServerScope()

const views = [
  { key: 'deaths', label: 'Deaths', icon: 'i-lucide-heart-pulse' },
  { key: 'rare_drops', label: 'Rare Drops', icon: 'i-lucide-gem' },
  { key: 'normal_drops', label: 'Normal Drops', icon: 'i-lucide-package' },
  { key: 'economy', label: 'Economy', icon: 'i-lucide-coins' },
  { key: 'academy', label: 'Academy', icon: 'i-lucide-graduation-cap' },
] as const
const validView = (value: unknown): value is (typeof views)[number]['key'] =>
  typeof value === 'string' && views.some((view) => view.key === value)
const view = computed(() =>
  validView(route.query.view) ? route.query.view : 'deaths',
)
const selected = computed(
  () => views.find((item) => item.key === view.value) || views[0]!,
)
const validBucket = (value: unknown): value is 'hour' | 'day' | 'week' =>
  value === 'hour' || value === 'day' || value === 'week'
const bucket = ref<'hour' | 'day' | 'week'>(
  validBucket(route.query.bucket) ? route.query.bucket : 'day',
)
const fromDate = ref(dateInput(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000)))
const toDate = ref(dateInput(new Date()))
const groupBy = ref<
  'character' | 'group' | 'location' | 'item' | 'type' | 'degree'
>(
  route.query.view === 'rare_drops' || route.query.view === 'normal_drops'
    ? 'item'
    : 'character',
)
const balanceScope = ref<'characters' | 'guild_storage'>(
  route.query.balance_scope === 'guild_storage'
    ? 'guild_storage'
    : 'characters',
)
const itemType = ref(
  typeof route.query.item_type === 'string' ? route.query.item_type : '',
)
const itemDegree = ref(
  typeof route.query.item_degree === 'string' ? route.query.item_degree : '',
)
const cursor = ref('')
const previousCursors = ref<string[]>([])
const pageNumber = ref(1)
const feedID = 'analytics'
const snapshot = computed(() => analyticsFeeds.value[feedID])
const feedStatus = computed(
  () => analyticsFeedStatus.value[feedID] || 'loading',
)
const invalidDateRange = computed(
  () => !!fromDate.value && !!toDate.value && fromDate.value > toDate.value,
)
const filterTimezone = ref('UTC')
const pageDescription = computed(() => {
  switch (view.value) {
    case 'deaths':
      return 'Recorded deaths grouped by character, current group membership or observed location.'
    case 'rare_drops':
      return 'Rare world-drop observations from verified phBot callbacks and item profiles.'
    case 'normal_drops':
      return 'Normal world-drop observations from verified phBot callbacks and item profiles.'
    case 'economy':
      return 'Observed character and guild balances, with stall sales shown only when a verified transaction source exists.'
    default:
      return 'Observed academy membership changes. Graduation and exact member duration need verified source facts.'
  }
})
const leftChartTitle = computed(() => {
  if (view.value === 'deaths') return `Deaths by ${groupLabel.value}`
  if (view.value === 'rare_drops') return `Rare drops by ${groupLabel.value}`
  if (view.value === 'normal_drops')
    return `Normal drops by ${groupLabel.value}`
  if (view.value === 'economy')
    return balanceScope.value === 'guild_storage'
      ? 'Observed guild-storage balances'
      : 'Observed character balances'
  return 'Observed membership changes'
})
const rightChartTitle = computed(() =>
  view.value === 'deaths'
    ? 'Deaths by time'
    : view.value === 'academy'
      ? 'Membership changes by time'
      : `${selected.value.label} by time`,
)
const groupLabel = computed(() => {
  switch (groupBy.value) {
    case 'group':
      return 'current group'
    case 'location':
      return 'location'
    case 'item':
      return 'item'
    case 'type':
      return 'item type'
    case 'degree':
      return 'item degree'
    default:
      return 'character'
  }
})
const leftPoints = computed(() => snapshot.value?.breakdown || [])
const rightPoints = computed(() => snapshot.value?.time_series || [])
const taxonomyTypes = computed(() =>
  [
    ...new Set(
      (snapshot.value?.taxonomy_options || []).map((option) => option.type),
    ),
  ].sort(),
)
const taxonomyDegrees = computed(() =>
  [
    ...new Set(
      (snapshot.value?.taxonomy_options || [])
        .filter((option) => !itemType.value || option.type === itemType.value)
        .map((option) => option.degree)
        .filter((degree): degree is string => !!degree),
    ),
  ].sort((a, b) => Number(a) - Number(b)),
)
const rangeLabel = computed(
  () => `${fromDate.value || '—'} to ${toDate.value || '—'}`,
)
const statusText = computed(() => {
  if (feedStatus.value === 'loading' && !snapshot.value)
    return 'Loading analytics…'
  if (feedStatus.value === 'unavailable')
    return 'Analytics are temporarily unavailable. Retry to load the selected range.'
  if (liveStale.value || connectionState.value === 'stale')
    return 'Showing the last delivered snapshot; the live connection is stale.'
  if (snapshot.value?.status === 'unsupported')
    return snapshot.value.reason || 'This source is unsupported.'
  if (snapshot.value?.status === 'limited')
    return (
      snapshot.value.reason ||
      'Some source coverage or response rows are limited.'
    )
  if (snapshot.value?.status === 'empty')
    return snapshot.value.reason || 'No recorded occurrences match this range.'
  if (snapshot.value?.status === 'insufficient_history')
    return (
      snapshot.value.reason ||
      'More accepted history is needed for this calculation.'
    )
  return ''
})

watch(
  view,
  (next) => {
    groupBy.value =
      next === 'rare_drops' || next === 'normal_drops' ? 'item' : 'character'
    cursor.value = ''
    pageNumber.value = 1
    previousCursors.value = []
  },
  { immediate: true },
)
onMounted(() => {
  filterTimezone.value =
    Intl.DateTimeFormat().resolvedOptions().timeZone || 'UTC'
})
watch(
  [
    view,
    serverScope,
    fromDate,
    toDate,
    groupBy,
    cursor,
    filterTimezone,
    bucket,
    balanceScope,
    itemType,
    itemDegree,
  ],
  ([nextView, server, from, to, grouping, pageCursor, timezone]) => {
    if (from && to && from > to) {
      clearAnalyticsFeed(feedID)
      return
    }
    setAnalyticsFeed(feedID, {
      view: nextView,
      server: server === 'all' ? undefined : server,
      from: from ? localDateBoundary(from, 0) : undefined,
      to: to ? localDateBoundary(to, 1) : undefined,
      timezone,
      bucket: bucket.value,
      group_by: grouping,
      balance_scope: nextView === 'economy' ? balanceScope.value : undefined,
      item_type:
        nextView === 'rare_drops' || nextView === 'normal_drops'
          ? itemType.value || undefined
          : undefined,
      item_degree:
        nextView === 'rare_drops' || nextView === 'normal_drops'
          ? itemDegree.value || undefined
          : undefined,
      page_size: 25,
      cursor: pageCursor || undefined,
    })
  },
  { immediate: true },
)
watch(bucket, (value) => {
  const query = { ...route.query, bucket: value === 'day' ? undefined : value }
  void router.replace({ path: route.path, query, hash: route.hash })
})
watch(
  () => route.query.bucket,
  (value) => {
    bucket.value = validBucket(value) ? value : 'day'
  },
)
watch(balanceScope, (scope) => {
  const query = { ...route.query, balance_scope: scope }
  void router.replace({ path: route.path, query, hash: route.hash })
})
watch([itemType, itemDegree], ([type, degree]) => {
  if (degree && !taxonomyDegrees.value.includes(degree)) {
    itemDegree.value = ''
    return
  }
  const query = {
    ...route.query,
    item_type: type || undefined,
    item_degree: degree || undefined,
  }
  void router.replace({ path: route.path, query, hash: route.hash })
})
watch(
  () => [route.query.item_type, route.query.item_degree],
  ([type, degree]) => {
    itemType.value = typeof type === 'string' ? type : ''
    itemDegree.value = typeof degree === 'string' ? degree : ''
  },
)
watch(view, (next) => {
  if (next !== 'rare_drops' && next !== 'normal_drops') {
    itemType.value = ''
    itemDegree.value = ''
  }
})
watch([view, serverScope, fromDate, toDate, groupBy, bucket], () => {
  cursor.value = ''
  pageNumber.value = 1
  previousCursors.value = []
})
onBeforeUnmount(() => clearAnalyticsFeed(feedID))

function metricValue(metric: AnalyticsMetric) {
  if (metric.value !== undefined && metric.value !== '') {
    if (metric.value.startsWith('/events?')) return metric.value
    return metric.value
  }
  if (metric.number === undefined) return '—'
  const fractionDigits = metric.unit === '%' ? 1 : 2
  return metric.number.toLocaleString(undefined, {
    maximumFractionDigits: fractionDigits,
  })
}
function isLinkMetric(metric: AnalyticsMetric) {
  return !!metric.href
}
function eventHref(item: AnalyticsOccurrence) {
  const kind = item.kind
  return { path: '/events', query: { kind, q: item.character } }
}
function mapHref(item: AnalyticsOccurrence) {
  return { path: '/map', query: { event_id: item.event_id } }
}
function openView(next: string) {
  router.push({ path: '/analytics', query: { ...route.query, view: next } })
}
function nextPage() {
  const next = snapshot.value?.next_cursor
  if (!next) return
  previousCursors.value.push(cursor.value)
  cursor.value = next
  pageNumber.value += 1
}
function previousPage() {
  if (!previousCursors.value.length) return
  cursor.value = previousCursors.value.pop() || ''
  pageNumber.value = Math.max(1, pageNumber.value - 1)
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
  <div class="analytics-page">
    <PageHeader
      :title="`Analytics · ${selected.label}`"
      :icon="selected.icon"
      :description="pageDescription"
    >
      <span v-if="snapshot" class="analytics-version">{{
        snapshot.calculation_version
      }}</span>
    </PageHeader>

    <nav class="analytics-tabs" aria-label="Analytics views">
      <button
        v-for="item in views"
        :key="item.key"
        type="button"
        :class="{ active: view === item.key }"
        :aria-current="view === item.key ? 'page' : undefined"
        @click="openView(item.key)"
      >
        <UIcon :name="item.icon" />{{ item.label }}
      </button>
    </nav>

    <section class="analytics-filterbar" aria-label="Analytics filters">
      <label>From <input v-model="fromDate" type="date" /></label>
      <label>To <input v-model="toDate" type="date" /></label>
      <label>
        Time bucket
        <select v-model="bucket">
          <option value="hour">Hour</option>
          <option value="day">Day</option>
          <option value="week">Week</option>
        </select>
      </label>
      <label v-if="view === 'economy'">
        Balance scope
        <select v-model="balanceScope">
          <option value="characters">Characters</option>
          <option value="guild_storage">Guild storage</option>
        </select>
      </label>
      <label
        v-if="
          view === 'deaths' ||
          view === 'rare_drops' ||
          view === 'normal_drops' ||
          view === 'academy'
        "
      >
        Group by
        <select v-model="groupBy">
          <option value="character">Character</option>
          <option v-if="view === 'deaths'" value="group">Current group</option>
          <option v-if="view !== 'academy'" value="location">Location</option>
          <option
            v-if="view === 'rare_drops' || view === 'normal_drops'"
            value="item"
          >
            Item
          </option>
          <option
            v-if="view === 'rare_drops' || view === 'normal_drops'"
            value="type"
          >
            Item type
          </option>
          <option
            v-if="view === 'rare_drops' || view === 'normal_drops'"
            value="degree"
          >
            Item degree
          </option>
        </select>
      </label>
      <label v-if="view === 'rare_drops' || view === 'normal_drops'">
        Item type
        <select v-model="itemType" :disabled="taxonomyTypes.length === 0">
          <option value="">All known types</option>
          <option v-for="type in taxonomyTypes" :key="type" :value="type">
            {{ type }}
          </option>
        </select>
      </label>
      <label v-if="view === 'rare_drops' || view === 'normal_drops'">
        Item degree
        <select v-model="itemDegree" :disabled="taxonomyDegrees.length === 0">
          <option value="">All known degrees</option>
          <option
            v-for="degree in taxonomyDegrees"
            :key="degree"
            :value="degree"
          >
            {{ degree }}
          </option>
        </select>
      </label>
      <span class="analytics-range"
        >{{ rangeLabel }} · {{ filterTimezone }}</span
      >
      <button
        class="compact-button"
        type="button"
        @click="refreshLiveData([feedID])"
      >
        <UIcon name="i-lucide-refresh-cw" /> Refresh
      </button>
    </section>

    <p v-if="invalidDateRange" class="analytics-inline-state" role="alert">
      Choose a From date on or before the To date.
    </p>
    <p
      v-else-if="statusText"
      class="analytics-inline-state"
      :class="{ 'is-error': feedStatus === 'unavailable' }"
      role="status"
    >
      {{ statusText }}
    </p>

    <div class="analytics-workspace">
      <div class="analytics-charts">
        <AnalyticsChart
          :title="leftChartTitle"
          :description="
            view === 'deaths' && groupBy === 'group'
              ? 'Current group membership can overlap; these bars are not additive.'
              : 'Recorded occurrences in the selected scope.'
          "
          :points="leftPoints"
          :empty-label="
            snapshot?.status === 'unsupported' ? snapshot.reason : undefined
          "
        />
        <AnalyticsChart
          :title="rightChartTitle"
          :description="`Local ${snapshot?.filter.bucket || 'day'} buckets · ${snapshot?.filter.timezone || filterTimezone}`"
          :points="rightPoints"
          :empty-label="
            snapshot?.status === 'unsupported' ? snapshot.reason : undefined
          "
        />
      </div>

      <aside class="panel analytics-summary" aria-label="Analytics summary">
        <header class="panel-header compact">
          <div>
            <h2>Summary</h2>
            <p>
              {{
                snapshot?.as_of
                  ? `As of ${new Date(snapshot.as_of).toLocaleString()}`
                  : 'Selected range'
              }}
            </p>
          </div>
        </header>
        <template v-if="snapshot?.summary.length">
          <article
            v-for="metric in snapshot.summary"
            :key="metric.key"
            class="analytics-metric"
          >
            <span>{{ metric.label }}</span>
            <NuxtLink
              v-if="isLinkMetric(metric)"
              :to="metric.href!"
              class="analytics-metric-value"
              >{{ metric.value || 'Open details' }}</NuxtLink
            >
            <strong v-else
              >{{ metricValue(metric)
              }}<small v-if="metric.unit"> {{ metric.unit }}</small></strong
            >
            <small v-if="metric.reason" class="analytics-reason">{{
              metric.reason
            }}</small>
          </article>
        </template>
        <p v-else-if="feedStatus === 'loading'" class="analytics-muted">
          Loading summary…
        </p>
        <p v-else class="analytics-muted">
          No summary values are available for this range.
        </p>
        <div v-if="snapshot?.coverage" class="analytics-coverage">
          <strong>Source coverage · {{ snapshot.coverage.status }}</strong>
          <span>{{ snapshot.coverage.reason }}</span>
          <span v-if="snapshot.coverage.oldest_sample"
            >History begins
            {{
              new Date(snapshot.coverage.oldest_sample).toLocaleDateString()
            }}</span
          >
        </div>
      </aside>
    </div>

    <section v-if="view !== 'economy'" class="panel analytics-occurrences">
      <header class="panel-header compact">
        <div>
          <h2>Recorded occurrences</h2>
          <p>
            {{ snapshot?.total || '0' }} matching records · totals do not change
            with table paging
          </p>
        </div>
      </header>
      <div class="analytics-table-wrap">
        <table>
          <thead>
            <tr>
              <th>Time</th>
              <th>Character</th>
              <th>Detail</th>
              <th>Location</th>
              <th>Open</th>
            </tr>
          </thead>
          <tbody>
            <tr
              v-for="item in snapshot?.occurrences || []"
              :key="item.event_id"
            >
              <td>{{ new Date(item.occurred_at).toLocaleString() }}</td>
              <td>
                <NuxtLink :to="eventHref(item)">{{ item.character }}</NuxtLink>
              </td>
              <td>
                <ItemDetailPopup
                  v-if="itemRecordFromActivityEvent(item)"
                  :item="itemRecordFromActivityEvent(item)!"
                  :to="eventHref(item)"
                />
                <template v-else>
                  {{ item.detail || item.kind
                  }}<span v-if="item.plus !== undefined">
                    · +{{ item.plus }}</span
                  ><span v-if="item.success !== undefined">
                    · {{ item.success ? 'Success' : 'Failure' }}</span
                  >
                </template>
              </td>
              <td>{{ item.location || 'Unknown location' }}</td>
              <td>
                <NuxtLink v-if="item.region !== undefined" :to="mapHref(item)"
                  >Map</NuxtLink
                ><span v-else>—</span>
              </td>
            </tr>
            <tr v-if="!snapshot?.occurrences.length">
              <td colspan="5" class="analytics-table-empty">
                {{
                  feedStatus === 'loading'
                    ? 'Loading records…'
                    : 'No recorded occurrences in this range.'
                }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div class="analytics-pagination">
        <span
          >Page {{ pageNumber }} ·
          {{ snapshot?.occurrences.length || 0 }} rows</span
        >
        <div>
          <button
            class="compact-button"
            type="button"
            :disabled="pageNumber <= 1"
            @click="previousPage"
          >
            Previous</button
          ><button
            class="compact-button"
            type="button"
            :disabled="!snapshot?.next_cursor"
            @click="nextPage"
          >
            Next
          </button>
        </div>
      </div>
    </section>
  </div>
</template>

<style scoped>
.analytics-page {
  display: grid;
  gap: 14px;
  min-width: 0;
}
.analytics-version {
  color: #8290a4;
  font-size: 11px;
  margin-left: auto;
}
.analytics-tabs {
  display: flex;
  gap: 6px;
  border-bottom: 1px solid #253345;
  padding: 0 2px;
  overflow-x: auto;
}
.analytics-tabs button {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  padding: 9px 12px;
  color: #aab7c9;
  background: transparent;
  border: 1px solid transparent;
  border-radius: 4px 4px 0 0;
  white-space: nowrap;
  cursor: pointer;
}
.analytics-tabs button.active {
  color: #fef6c3;
  border-color: #34445a;
  border-bottom-color: #0d131d;
  background: #151e2b;
}
.analytics-filterbar {
  display: flex;
  align-items: end;
  flex-wrap: wrap;
  gap: 10px;
  padding: 10px 12px;
  border: 1px solid #27364a;
  border-radius: 5px;
  background: rgba(13, 19, 29, 0.86);
}
.analytics-filterbar label {
  display: grid;
  gap: 4px;
  color: #aab7c9;
  font-size: 12px;
}
.analytics-filterbar input,
.analytics-filterbar select {
  min-height: 32px;
  min-width: 132px;
  padding: 5px 8px;
  color: #eaf1ff;
  background: #101925;
  border: 1px solid #35465c;
  border-radius: 4px;
}
.analytics-range {
  margin: 0 auto 8px 4px;
  color: #8290a4;
  font-size: 11px;
}
.compact-button {
  display: inline-flex;
  align-items: center;
  justify-content: center;
  gap: 6px;
  min-height: 32px;
  padding: 5px 10px;
  color: #d9e4f4;
  background: #182435;
  border: 1px solid #34465d;
  border-radius: 4px;
  cursor: pointer;
}
.compact-button:hover:not(:disabled) {
  background: #21334b;
}
.compact-button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.analytics-inline-state {
  margin: 0;
  padding: 9px 12px;
  border: 1px solid #32455e;
  border-radius: 4px;
  color: #c8d4e5;
  background: #121d2a;
  font-size: 12px;
}
.analytics-inline-state.is-error {
  border-color: #71494c;
  color: #e9b8b3;
}
.analytics-workspace {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 286px;
  gap: 12px;
  align-items: stretch;
}
.analytics-charts {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 12px;
  min-width: 0;
}
.analytics-summary {
  min-width: 0;
  padding: 12px;
}
.analytics-metric {
  display: grid;
  gap: 4px;
  padding: 11px 10px;
  margin-top: 8px;
  border: 1px solid #27384c;
  border-radius: 4px;
  background: rgba(20, 30, 44, 0.82);
}
.analytics-metric > span {
  color: #9baabe;
  font-size: 11px;
}
.analytics-metric > strong,
.analytics-metric-value {
  color: #f4efcf;
  font-size: 17px;
  font-weight: 600;
  text-decoration: none;
  overflow-wrap: anywhere;
}
.analytics-metric > strong small {
  color: #8795a8;
  font-size: 10px;
  font-weight: 400;
}
.analytics-metric-value:hover {
  text-decoration: underline;
}
.analytics-reason {
  color: #8795a8;
  font-size: 10px;
  line-height: 1.4;
}
.analytics-coverage {
  display: grid;
  gap: 5px;
  padding: 11px 3px 2px;
  color: #8f9caf;
  font-size: 10px;
  line-height: 1.4;
}
.analytics-coverage strong {
  color: #c4d0e0;
  font-size: 11px;
  font-weight: 500;
}
.analytics-muted {
  color: #8795a8;
  font-size: 12px;
}
.analytics-occurrences {
  min-width: 0;
  padding: 12px;
}
.analytics-table-wrap {
  max-width: 100%;
  overflow-x: auto;
}
.analytics-occurrences table {
  width: 100%;
  border-collapse: collapse;
  font-size: 12px;
}
.analytics-occurrences th,
.analytics-occurrences td {
  padding: 8px 9px;
  border-bottom: 1px solid #263448;
  text-align: left;
  vertical-align: top;
  white-space: nowrap;
}
.analytics-occurrences th {
  color: #9baabe;
  font-weight: 500;
}
.analytics-occurrences td {
  color: #dce5f2;
}
.analytics-occurrences a {
  color: #b5cdec;
  text-decoration: none;
}
.analytics-occurrences a:hover {
  color: #fef6c3;
  text-decoration: underline;
}
.analytics-table-empty {
  text-align: center !important;
  color: #8795a8 !important;
  padding: 30px !important;
}
.analytics-pagination {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
  padding-top: 10px;
  color: #8f9caf;
  font-size: 11px;
}
.analytics-pagination > div {
  display: flex;
  gap: 6px;
}
@media (max-width: 1180px) {
  .analytics-workspace {
    grid-template-columns: minmax(0, 1fr) 250px;
  }
  .analytics-charts {
    grid-template-columns: 1fr;
  }
}
@media (max-width: 760px) {
  .analytics-workspace {
    grid-template-columns: 1fr;
  }
  .analytics-summary {
    order: -1;
  }
  .analytics-filterbar {
    align-items: stretch;
  }
  .analytics-range {
    width: 100%;
    margin: 2px 0;
  }
}
@media (max-width: 420px) {
  .analytics-tabs button {
    padding-inline: 9px;
    font-size: 12px;
  }
  .analytics-filterbar label {
    flex: 1;
    min-width: 42%;
  }
  .analytics-filterbar input,
  .analytics-filterbar select {
    width: 100%;
    min-width: 0;
  }
  .analytics-pagination {
    align-items: flex-start;
    flex-direction: column;
  }
}
</style>
