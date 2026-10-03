<script setup lang="ts">
import type { ActivityEvent } from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import { mapEventLocation, mapEventRoute } from '~/utils/mapNavigation'
import { eventLocationText } from '~/utils/event-location'
import { itemRecordFromActivityEvent } from '~/utils/itemDetailPopup'
import { uniqueEventDetails, uniqueEventHeadline } from '~/utils/uniqueEvent'
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
  {
    label: 'Rare Drops',
    key: 'drop.rare',
    query: { kind: 'drop.rare', include_pet_pickups: 'true' },
  },
  {
    label: 'Normal Drops',
    key: 'drop.item',
    query: { kind: 'drop.item', include_pet_pickups: 'true' },
  },
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
const includePetPickups = computed(
  () =>
    !filterCategory.value &&
    (filterKind.value === 'drop.item' || filterKind.value === 'drop.rare') &&
    route.query.include_pet_pickups !== 'false',
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
const eventMapProfiles = ref<Record<string, MapProfile>>({})
const mapProfileRequests = new Map<string, Promise<MapProfile>>()
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
const dropFeed = computed(
  () => filterKind.value === 'drop.item' || filterKind.value === 'drop.rare',
)
const rareDropFeed = computed(() => filterKind.value === 'drop.rare')
const hasReliableMapLocation = (event: ActivityEvent) => {
  const profile = eventMapProfiles.value[event.server.toLowerCase()]
  return Boolean(
    profile && mapEventLocation(profile, event).status === 'mapped',
  )
}

const eventMapTarget = (event: ActivityEvent) => {
  const profile = eventMapProfiles.value[event.server.toLowerCase()]
  const location = profile ? mapEventLocation(profile, event) : undefined
  return mapEventRoute(
    event.server,
    event.event_id,
    event.character_id,
    event.region,
    location,
  )
}

watch(
  page,
  async (eventPage) => {
    const serverNames = [
      ...new Set((eventPage?.events || []).map((event) => event.server)),
    ]
    await Promise.all(
      serverNames.map(async (server) => {
        const key = server.toLowerCase()
        let request = mapProfileRequests.get(key)
        if (!request) {
          request = $fetch<MapProfile>(
            `/api/map/profile?server=${encodeURIComponent(server)}`,
          )
          mapProfileRequests.set(key, request)
        }
        try {
          const profile = await request
          eventMapProfiles.value = {
            ...eventMapProfiles.value,
            [key]: profile,
          }
        } catch {
          if (mapProfileRequests.get(key) === request)
            mapProfileRequests.delete(key)
          // An unavailable profile leaves the map action disabled for this server.
        }
      }),
    )
  },
  { immediate: true },
)

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
    includePetPickups,
    cursor,
    pageSize,
  ],
  ([
    server,
    from,
    to,
    character,
    item,
    kind,
    category,
    includePickups,
    pageCursor,
    size,
  ]) => {
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
      include_pet_pickups: includePickups || undefined,
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
      return 'Rare drop'
    case 'drop.item':
      return 'Normal drop'
    case 'world.unique_spawned':
      return uniqueEventHeadline(item)
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
function eventSourceLabel(item: ActivityEvent) {
  if (item.kind === 'drop.item' || item.kind === 'drop.rare')
    return 'Drop observed'
  if (item.kind === 'item.acquired' && item.source === 'joymax.pet_inventory')
    return 'Pet pickup'
  return ''
}
function eventContainerDetail(item: ActivityEvent) {
  if (item.source !== 'joymax.pet_inventory') return ''
  const payload = record(item.payload)
  const destination = record(payload.destination_container)
  if (destination.type !== 'pets') return ''
  const petID = typeof destination.id === 'string' ? destination.id : 'unknown'
  const slot =
    typeof destination.slot === 'number' ? destination.slot + 1 : null
  const quantity =
    typeof payload.quantity_delta === 'number' ? payload.quantity_delta : null
  return [
    `Pet ${petID}`,
    slot === null ? '' : `slot ${slot}`,
    quantity === null ? '' : `+${quantity}`,
  ]
    .filter(Boolean)
    .join(' · ')
}
function eventItemName(item: ActivityEvent) {
  const payload = record(item.payload)
  const snapshot = record(payload.item)
  const metadata = record(item.item_metadata)
  if (
    (item.kind === 'drop.rare' || item.kind === 'drop.item') &&
    typeof metadata.name === 'string' &&
    metadata.name
  )
    return metadata.name
  if (typeof snapshot.name === 'string' && snapshot.name) return snapshot.name
  if (typeof payload.item_name === 'string' && payload.item_name)
    return payload.item_name
  if (typeof metadata.name === 'string' && metadata.name) return metadata.name
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
function eventItemTarget(item: ActivityEvent) {
  return {
    path: '/events',
    query: {
      kind: item.kind,
      item: itemFilterKey(item) || undefined,
    },
  }
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
      <div
        v-if="filterKind === 'world.unique_spawned'"
        class="unique-event-list"
      >
        <article
          v-for="item in page?.events || []"
          :key="item.event_id"
          class="unique-event-card"
        >
          <img
            v-if="uniqueEventDetails(item)?.imageUrl"
            class="unique-event-art"
            :src="uniqueEventDetails(item)?.imageUrl"
            :alt="uniqueEventDetails(item)?.name || 'Unique'"
          />
          <div>
            <strong>{{ uniqueEventHeadline(item) }}</strong>
            <p>{{ eventLocationText(item) }}</p>
            <p>
              Seen by
              <NuxtLink
                v-if="item.character_id"
                :to="`/characters/${item.character_id}`"
                >{{ item.character }}</NuxtLink
              >
              <template v-else>{{ item.character || 'unknown' }}</template>
            </p>
            <time :datetime="item.occurred_at">{{
              formatTimestamp(item.occurred_at)
            }}</time>
          </div>
        </article>
        <div v-if="!page?.events.length" class="event-empty-state">
          <strong>{{
            connectionState === 'current'
              ? 'No unique spawns found'
              : 'Loading event history'
          }}</strong>
          <span>{{
            connectionState === 'current'
              ? 'No unique notices match this server, character and date range.'
              : 'Waiting for a current event snapshot.'
          }}</span>
        </div>
      </div>
      <div v-else-if="dropFeed" class="event-table-scroll">
        <table class="event-table drop-event-table">
          <thead>
            <tr>
              <th>Item</th>
              <th>Time</th>
              <th>Character</th>
              <th>Location</th>
              <th v-if="rareDropFeed">Map</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="item in page?.events || []" :key="item.event_id">
              <td :class="{ 'rare-drop-event': item.kind === 'drop.rare' }">
                <ItemDetailPopup
                  v-if="itemRecordFromActivityEvent(item)"
                  :item="itemRecordFromActivityEvent(item)!"
                  :to="eventItemTarget(item)"
                />
                <span v-else>—</span>
                <small v-if="eventSourceLabel(item)" class="event-origin">
                  {{ eventSourceLabel(item) }}
                  <template v-if="eventContainerDetail(item)">
                    · {{ eventContainerDetail(item) }}
                  </template>
                </small>
              </td>
              <td>
                <time :datetime="item.occurred_at">{{
                  formatTimestamp(item.occurred_at)
                }}</time>
              </td>
              <td>
                <NuxtLink
                  v-if="item.character_id"
                  class="event-character-link"
                  :to="`/characters/${item.character_id}`"
                >
                  <CharacterPortrait
                    :name="item.character"
                    :portrait-url="item.portrait_url"
                    size="small"
                  />
                  {{ item.character }}
                </NuxtLink>
                <span v-else>{{ item.character || '—' }}</span>
              </td>
              <td>
                {{ eventLocationText(item) }}
                <NuxtLink
                  v-if="!rareDropFeed && hasReliableMapLocation(item)"
                  class="compact-button map-event-link"
                  :to="eventMapTarget(item)"
                  aria-label="Open event location on map"
                >
                  <UIcon name="i-lucide-map-pin" />
                </NuxtLink>
              </td>
              <td v-if="rareDropFeed">
                <NuxtLink
                  v-if="hasReliableMapLocation(item)"
                  class="compact-button map-event-link"
                  :to="eventMapTarget(item)"
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
      <div v-else class="event-table-scroll">
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
              <td :class="{ 'rare-drop-event': item.kind === 'drop.rare' }">
                <span>{{ eventSummary(item) }}</span>
                <small v-if="eventSourceLabel(item)" class="event-origin">
                  {{ eventSourceLabel(item) }}
                  <template v-if="eventContainerDetail(item)">
                    · {{ eventContainerDetail(item) }}
                  </template>
                </small>
              </td>
              <td>
                <NuxtLink
                  v-if="item.character_id"
                  class="event-character-link"
                  :to="`/characters/${item.character_id}`"
                >
                  <CharacterPortrait
                    :name="item.character"
                    :portrait-url="item.portrait_url"
                    size="small"
                  />
                  {{ item.character }}
                </NuxtLink>
                <span v-else>{{ item.character || '—' }}</span>
              </td>
              <td>
                <ItemDetailPopup
                  v-if="itemRecordFromActivityEvent(item)"
                  :item="itemRecordFromActivityEvent(item)!"
                  :to="eventItemTarget(item)"
                />
                <span v-else>—</span>
              </td>
              <td>{{ eventLocationText(item) }}</td>
              <td>
                <NuxtLink
                  v-if="hasReliableMapLocation(item)"
                  class="compact-button map-event-link"
                  :to="eventMapTarget(item)"
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
