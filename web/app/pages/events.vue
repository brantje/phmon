<script setup lang="ts">
import type { ActivityEvent } from '~~/shared/types/live'
const { eventFeeds, connectionState, liveStale, setEventFeed, clearEventFeed } =
  useLiveData()
const { serverScope } = useServerScope()
const route = useRoute()
const tabs = [
  { label: 'All', key: 'all', query: {} },
  {
    label: 'Level Ups',
    key: 'character.level_up',
    query: { kind: 'character.level_up' },
  },
  { label: 'Custom', key: 'custom', query: { category: 'custom' } },
  { label: 'Deaths', key: 'character.died', query: { kind: 'character.died' } },
  { label: 'Rare Drops', key: 'drop.rare', query: { kind: 'drop.rare' } },
  { label: 'Normal Drops', key: 'drop.item', query: { kind: 'drop.item' } },
  {
    label: 'Uniques',
    key: 'world.unique_spawned',
    query: { kind: 'world.unique_spawned' },
  },
]
const selectedTab = computed(() => {
  if (typeof route.query.kind === 'string') return route.query.kind
  if (route.query.category === 'custom') return 'custom'
  return 'all'
})
const activeTab = computed(
  () => tabs.find((tab) => tab.key === selectedTab.value) || tabs[0],
)
const filterKind = computed(() =>
  typeof route.query.kind === 'string' ? route.query.kind : undefined,
)
const filterCategory = computed(() =>
  route.query.category === 'custom' ? 'custom' : undefined,
)
const activeTabLabel = computed(() => activeTab.value?.label ?? 'All')
const pageTitle = computed(() =>
  activeTabLabel.value === 'All'
    ? 'History · All'
    : `History · ${activeTabLabel.value}`,
)
const description =
  'Recorded activity from phBot callbacks and reliable state observations.'
const fromDate = ref(dateInput(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000)))
const toDate = ref(dateInput(new Date()))
const characterInput = ref('')
const characterQuery = ref('')
const itemInput = ref(
  typeof route.query.item === 'string' ? route.query.item : '',
)
const itemQuery = ref(itemInput.value.trim())
const pageSize = ref(10)
const cursor = ref('')
const previousCursors = ref<string[]>([])
let characterSearchTimer: ReturnType<typeof setTimeout> | undefined
let itemSearchTimer: ReturnType<typeof setTimeout> | undefined
const feedID = 'activity-history'
const page = computed(() => eventFeeds.value[feedID])
const invalidDateRange = computed(
  () => !!fromDate.value && !!toDate.value && fromDate.value > toDate.value,
)
const itemTab = computed(
  () =>
    [
      'drop.item',
      'drop.rare',
      'alchemy.attempt',
      'item.acquired',
      'item.transferred',
    ].includes(filterKind.value || '') || activeTabLabel.value === 'All',
)
const locationText = (event: {
  region?: number
  x?: number
  y?: number
  z?: number
}) => {
  if (event.region == null || event.x == null || event.y == null)
    return 'Location unknown'
  return `Region ${event.region} · ${event.x.toFixed(1)}, ${event.y.toFixed(1)}, ${event.z?.toFixed(1) ?? '—'}`
}
const hasReliableMapLocation = (_event: ActivityEvent) => false

watch(characterInput, (value) => {
  if (characterSearchTimer) clearTimeout(characterSearchTimer)
  characterSearchTimer = setTimeout(() => {
    characterQuery.value = value.trim()
  }, 250)
})
watch(itemInput, (value) => {
  if (itemSearchTimer) clearTimeout(itemSearchTimer)
  itemSearchTimer = setTimeout(() => {
    itemQuery.value = value.trim()
  }, 250)
})
watch(
  () => route.query.item,
  (value) => {
    const next = typeof value === 'string' ? value : ''
    if (itemSearchTimer) clearTimeout(itemSearchTimer)
    itemSearchTimer = undefined
    itemInput.value = next
    itemQuery.value = next.trim()
  },
)

watch(
  [
    serverScope,
    fromDate,
    toDate,
    characterQuery,
    itemQuery,
    filterKind,
    filterCategory,
    cursor,
    pageSize,
  ],
  ([server, from, to, character, item, kind, category, pageCursor, size]) => {
    if (from && to && from > to) {
      clearEventFeed(feedID)
      return
    }
    setEventFeed(feedID, {
      server: server === 'all' ? undefined : server,
      from: from ? localDateBoundary(from, 0) : undefined,
      to: to ? localDateBoundary(to, 1) : undefined,
      q: character || undefined,
      item: itemTab.value ? item || undefined : undefined,
      kind: kind || undefined,
      category: category || undefined,
      cursor: pageCursor || undefined,
      limit: size,
    })
  },
  { immediate: true },
)
watch(
  [
    serverScope,
    fromDate,
    toDate,
    characterQuery,
    itemQuery,
    filterKind,
    filterCategory,
    pageSize,
  ],
  () => {
    cursor.value = ''
    previousCursors.value = []
  },
)
onBeforeUnmount(() => {
  if (characterSearchTimer) clearTimeout(characterSearchTimer)
  if (itemSearchTimer) clearTimeout(itemSearchTimer)
  clearEventFeed(feedID)
})

function nextPage() {
  if (!page.value?.next_cursor) return
  previousCursors.value.push(cursor.value)
  cursor.value = page.value.next_cursor
}
function previousPage() {
  if (!previousCursors.value.length) return
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
function record(value: unknown): Record<string, unknown> {
  return value && typeof value === 'object'
    ? (value as Record<string, unknown>)
    : {}
}
function eventSummary(item: ActivityEvent) {
  const payload = item.payload || {}
  switch (item.kind) {
    case 'character.died':
      return 'Character died'
    case 'character.level_up':
      return `Reached level ${String(payload.level ?? 'unknown')}`
    case 'drop.rare':
      return `Rare drop${eventItemName(item) ? ` · ${eventItemName(item)}` : ''}`
    case 'drop.item':
      return `Normal drop${eventItemName(item) ? ` · ${eventItemName(item)}` : ''}`
    case 'world.unique_spawned':
      return `${String(payload.value || 'Unique')} spawned`
    case 'world.gm_spawned':
      return `GM spawned · ${String(payload.value || 'name unknown')}`
    case 'job.hunter_trader_seen':
      return `Hunter/trader seen · ${String(payload.value || 'name unknown')}`
    case 'job.thief_seen':
      return `Thief seen · ${String(payload.value || 'name unknown')}`
    case 'pet.transport_died':
      return `Transport died · ${String(payload.value || 'identity unknown')}`
    case 'character.attacked':
      return `Attacked · ${String(payload.value || 'attacker unknown')}`
    case 'alchemy.finished':
      return 'Alchemy run finished'
    case 'alchemy.attempt':
      return `Alchemy attempt${payload.plus != null ? ` · +${String(payload.plus)}` : ''}`
    case 'chat.message_received':
      return `${String(payload.sender || 'Message')}: ${String(payload.message || '')}`
    case 'item.acquired':
      return `Item acquired · ${eventItemName(item)}`
    case 'item.transferred':
      return `Item transferred · ${eventItemName(item)}`
    case 'item.quantity_increased':
      return `Item quantity increased · ${eventItemName(item)}`
    case 'item.quantity_decreased':
      return `Item quantity decreased · ${eventItemName(item)}`
    default:
      return item.kind.replaceAll('.', ' ')
  }
}
function eventItemName(item: ActivityEvent) {
  const payload = record(item.payload)
  const snapshot = record(payload.item)
  if (typeof snapshot.name === 'string' && snapshot.name) return snapshot.name
  if (typeof payload.item_name === 'string' && payload.item_name)
    return payload.item_name
  const model = item.item_model ?? payload.model
  return model == null ? '' : `Model ${String(model)}`
}
function itemFilterKey(item: ActivityEvent) {
  const snapshot = record(record(item.payload).item)
  return (
    item.item_code ||
    (typeof snapshot.servername === 'string' ? snapshot.servername : '') ||
    String(item.item_model ?? record(item.payload).model ?? '')
  )
}
function itemDetail(item: ActivityEvent) {
  const payload = record(item.payload)
  const snapshot = record(payload.item)
  return [
    snapshot.plus != null ? `+${String(snapshot.plus)}` : '',
    snapshot.quantity != null ? `Qty ${String(snapshot.quantity)}` : '',
    typeof snapshot.servername === 'string'
      ? snapshot.servername
      : item.item_code || '',
    typeof payload.acquisition_method === 'string'
      ? `Method ${payload.acquisition_method}`
      : '',
  ]
    .filter(Boolean)
    .join(' · ')
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
      :title="pageTitle"
      icon="i-lucide-activity"
      :description="description"
    />
    <section class="panel events-panel" aria-label="Activity event history">
      <div class="events-tabs" role="tablist" aria-label="Event types">
        <NuxtLink
          v-for="tab in tabs"
          :key="tab.key"
          class="compact-button"
          :class="{ selected: selectedTab === tab.key }"
          role="tab"
          :aria-selected="selectedTab === tab.key"
          :to="{ path: '/events', query: tab.query }"
          >{{ tab.label }}</NuxtLink
        >
        <span class="event-count">{{ page?.total ?? '—' }}</span>
      </div>

      <div class="events-filters">
        <label>
          <span>Character</span>
          <input
            v-model="characterInput"
            maxlength="64"
            placeholder="Filter by character"
            aria-label="Filter events by character"
          />
        </label>
        <label v-if="itemTab">
          <span>Item</span>
          <input
            v-model="itemInput"
            maxlength="128"
            placeholder="Filter by item"
            aria-label="Filter events by item"
          />
        </label>
        <label>
          <span>From date</span>
          <input v-model="fromDate" type="date" aria-label="From date" />
        </label>
        <label>
          <span>To date</span>
          <input v-model="toDate" type="date" aria-label="To date" />
        </label>
        <button class="compact-button" type="button" @click="resetFilters">
          Reset
        </button>
      </div>

      <div v-if="invalidDateRange" class="status-banner warning" role="alert">
        The start date must be on or before the end date.
      </div>

      <div v-if="liveStale" class="status-banner warning" role="status">
        Showing the last received event page as stale while PhMon reconnects.
      </div>
      <div class="event-table-scroll">
        <table class="event-table">
          <thead>
            <tr>
              <th>Timestamp</th>
              <th>Event</th>
              <th>Character</th>
              <th>Item</th>
              <th>Location</th>
              <th>Map</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in page?.events || []" :key="item.event_id">
              <td>
                <time :datetime="item.occurred_at">{{
                  formatTimestamp(item.occurred_at)
                }}</time>
              </td>
              <td>{{ eventSummary(item) }}</td>
              <td>
                <NuxtLink
                  v-if="item.character_id"
                  class="event-character-link"
                  :to="`/characters/${item.character_id}`"
                >
                  <span class="event-avatar">{{
                    (item.character || '?').slice(0, 1).toUpperCase()
                  }}</span>
                  {{ item.character }}
                </NuxtLink>
                <span v-else>{{ item.character || '—' }}</span>
              </td>
              <td>
                <NuxtLink
                  v-if="
                    item.item_model != null ||
                    item.item_code ||
                    record(item.payload).item
                  "
                  class="event-item-link"
                  :to="{
                    path: '/events',
                    query: {
                      kind: item.kind,
                      item: itemFilterKey(item) || undefined,
                    },
                  }"
                  :title="itemDetail(item) || 'Show events for this item'"
                  >{{ eventItemName(item) || 'Item details' }}</NuxtLink
                >
                <span v-else>—</span>
                <small v-if="itemDetail(item)" class="event-item-detail">{{
                  itemDetail(item)
                }}</small>
              </td>
              <td>{{ locationText(item) }}</td>
              <td>
                <NuxtLink
                  v-if="hasReliableMapLocation(item)"
                  class="compact-button map-event-link"
                  :to="`/map?region=${item.region}&x=${item.x}&y=${item.y}`"
                  aria-label="Open event location on map"
                >
                  <UIcon name="i-lucide-map-pin" />
                </NuxtLink>
                <button
                  v-else
                  class="compact-button map-event-link"
                  type="button"
                  disabled
                  title="Map transforms are not yet validated for this region"
                >
                  <UIcon name="i-lucide-map-pin" />
                </button>
              </td>
            </tr>
          </tbody>
        </table>
        <div v-if="!page?.events.length" class="event-empty-state">
          <strong>{{
            connectionState === 'current'
              ? 'No events found'
              : 'Loading event history'
          }}</strong>
          <span>{{
            connectionState === 'current'
              ? 'No events match this server, character, item and date range.'
              : 'Waiting for a current event snapshot.'
          }}</span>
        </div>
      </div>

      <footer class="event-pagination">
        <label>
          <span>Entries per page</span>
          <select v-model.number="pageSize" aria-label="Entries per page">
            <option :value="10">10</option>
            <option :value="25">25</option>
            <option :value="50">50</option>
          </select>
        </label>
        <div>
          <button
            class="compact-button"
            type="button"
            :disabled="!previousCursors.length"
            @click="previousPage"
          >
            Previous
          </button>
          <span>
            Page {{ previousCursors.length + 1 }} /
            {{ Math.max(1, Math.ceil((page?.total || 0) / pageSize)) }}
            · {{ page?.total || 0 }} entries
          </span>
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
  </div>
</template>
