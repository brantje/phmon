<script setup lang="ts">
import type { ActivityEvent } from '~~/shared/types/live'
const { eventFeeds, connectionState, liveStale, setEventFeed, clearEventFeed } =
  useLiveData()
const { serverScope } = useServerScope()
const fromDate = ref(dateInput(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000)))
const toDate = ref(dateInput(new Date()))
const characterInput = ref('')
const characterQuery = ref('')
const pageSize = ref(10)
const cursor = ref('')
const previousCursors = ref<string[]>([])
let searchTimer: ReturnType<typeof setTimeout> | undefined
const feedID = 'death-history'
const page = computed(() => eventFeeds.value[feedID])
const invalidDateRange = computed(
  () => !!fromDate.value && !!toDate.value && fromDate.value > toDate.value,
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
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    characterQuery.value = value.trim()
  }, 250)
})

watch(
  [serverScope, fromDate, toDate, characterQuery, cursor, pageSize],
  ([server, from, to, character, pageCursor, size]) => {
    if (from && to && from > to) {
      clearEventFeed(feedID)
      return
    }
    setEventFeed(feedID, {
      server: server === 'all' ? undefined : server,
      from: from ? localDateBoundary(from, 0) : undefined,
      to: to ? localDateBoundary(to, 1) : undefined,
      q: character || undefined,
      kind: 'character.died',
      cursor: pageCursor || undefined,
      limit: size,
    })
  },
  { immediate: true },
)
watch([serverScope, fromDate, toDate, characterQuery, pageSize], () => {
  cursor.value = ''
  previousCursors.value = []
})
onBeforeUnmount(() => {
  if (searchTimer) clearTimeout(searchTimer)
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
  fromDate.value = dateInput(new Date(Date.now() - 6 * 24 * 60 * 60 * 1000))
  toDate.value = dateInput(new Date())
  cursor.value = ''
  previousCursors.value = []
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
      title="Deaths"
      icon="i-lucide-circle-user-round"
      description="Recorded character death events. Cause is shown only when phBot supplies verified evidence."
    />
    <section class="panel events-panel" aria-label="Death event history">
      <div class="events-tabs" role="tablist" aria-label="Event types">
        <button class="compact-button" type="button" disabled>All</button>
        <button class="compact-button" type="button" disabled>Level Ups</button>
        <button class="compact-button" type="button" disabled>Custom</button>
        <button
          class="compact-button selected"
          type="button"
          role="tab"
          aria-selected="true"
        >
          Deaths
        </button>
        <button class="compact-button" type="button" disabled>
          Rare Drops
        </button>
        <button class="compact-button" type="button" disabled>
          Normal Drops
        </button>
        <button class="compact-button" type="button" disabled>Uniques</button>
        <span class="event-count">{{ page?.total ?? '—' }}</span>
      </div>

      <div class="events-filters">
        <label>
          <span>Character</span>
          <input
            v-model="characterInput"
            maxlength="64"
            placeholder="Filter by character"
            aria-label="Filter deaths by character"
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
              <th>Character</th>
              <th>Reason</th>
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
              <td>
                <NuxtLink
                  class="event-character-link"
                  :to="`/characters/${item.character_id}`"
                >
                  <span class="event-avatar">{{
                    item.character.slice(0, 1).toUpperCase()
                  }}</span>
                  {{ item.character }}
                </NuxtLink>
              </td>
              <td>
                {{
                  typeof item.payload.cause === 'string'
                    ? item.payload.cause
                    : 'Cause unknown'
                }}
              </td>
              <td>{{ locationText(item) }}</td>
              <td>
                <NuxtLink
                  v-if="hasReliableMapLocation(item)"
                  class="compact-button map-event-link"
                  :to="`/map?region=${item.region}&x=${item.x}&y=${item.y}`"
                  aria-label="Open death location on map"
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
              ? 'No deaths found'
              : 'Loading death history'
          }}</strong>
          <span>{{
            connectionState === 'current'
              ? 'No death events match this server, character and date range.'
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
