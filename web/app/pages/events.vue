<script setup lang="ts">
import type {
  ActivityEvent,
  ThiefSighting,
  ThiefSightingPage,
} from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import { mapEventLocation, mapEventRoute } from '~/utils/mapNavigation'
import { eventLocationText, eventRowLocationText } from '~/utils/event-location'
import { deathCauseLabel } from '~/utils/deathCause'
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
    query: {
      kind: 'drop.rare',
      include_pet_pickups: 'true',
      include_owned_gains: 'true',
    },
  },
  {
    label: 'Normal Drops',
    key: 'drop.item',
    query: {
      kind: 'drop.item',
      include_pet_pickups: 'true',
      include_owned_gains: 'true',
    },
  },
  {
    label: 'Uniques',
    key: 'world.unique_spawned',
    query: { kind: 'world.unique_spawned' },
  },
  { label: 'Thieves', key: 'thieves', query: { view: 'thieves' } },
]
const selectedTab = computed(() => {
  if (route.query.view === 'thieves') return 'thieves'
  if (typeof route.query.kind === 'string') return route.query.kind
  if (route.query.category === 'custom') return 'custom'
  return 'all'
})
const thievesView = computed(() => selectedTab.value === 'thieves')
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
const includeOwnedGains = computed(
  () =>
    !filterCategory.value &&
    (filterKind.value === 'drop.item' || filterKind.value === 'drop.rare') &&
    route.query.include_owned_gains !== 'false',
)
const activeTabLabel = computed(() => activeTab.value?.label ?? 'All')
const pageTitle = computed(() =>
  activeTabLabel.value === 'All'
    ? 'History · All'
    : `History · ${activeTabLabel.value}`,
)
const description = computed(() =>
  thievesView.value
    ? 'Thief sightings reported by PhMon characters and AdvancedAutoTrade.'
    : 'Recorded activity from phBot callbacks and reliable state observations.',
)
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
const sightingPage = ref<ThiefSightingPage | null>(null)
const sightingStatus = ref<'loading' | 'current' | 'error'>('loading')
const sightingCursor = ref('')
const sightingPrevious = ref<string[]>([])
let sightingTimer: ReturnType<typeof setInterval> | undefined
let sightingRequest = 0
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

async function rememberMapProfiles(servers: string[]) {
  await Promise.all(
    [...new Set(servers.filter(Boolean))].map(async (server) => {
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
        eventMapProfiles.value = { ...eventMapProfiles.value, [key]: profile }
      } catch {
        if (mapProfileRequests.get(key) === request)
          mapProfileRequests.delete(key)
      }
    }),
  )
}

watch(
  page,
  (eventPage) => {
    void rememberMapProfiles(
      (eventPage?.events || []).map((event) => event.server),
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
    includeOwnedGains,
    thievesView,
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
    includeGains,
    _thieves,
    pageCursor,
    size,
  ]) => {
    if (_thieves) {
      clearEventFeed(feedID)
      return
    }
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
      include_owned_gains: includeGains || undefined,
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
    includePetPickups,
    includeOwnedGains,
    pageSize,
  ],
  () => {
    cursor.value = ''
    previousCursors.value = []
  },
)
watch(
  [
    thievesView,
    serverScope,
    fromDate,
    toDate,
    characterQuery,
    sightingCursor,
    pageSize,
  ],
  () => {
    if (sightingTimer) clearInterval(sightingTimer)
    sightingTimer = undefined
    if (!thievesView.value) return
    void loadSightings()
    sightingTimer = setInterval(() => void loadSightings(), 5000)
  },
  { immediate: true },
)
watch(
  [thievesView, serverScope, fromDate, toDate, characterQuery, pageSize],
  () => {
    sightingCursor.value = ''
    sightingPrevious.value = []
  },
)
watch(sightingPage, (value) => {
  void rememberMapProfiles((value?.sightings || []).map((row) => row.server))
})
onBeforeUnmount(() => {
  if (characterSearchTimer) clearTimeout(characterSearchTimer)
  if (itemSearchTimer) clearTimeout(itemSearchTimer)
  if (sightingTimer) clearInterval(sightingTimer)
  clearEventFeed(feedID)
})

async function loadSightings() {
  const request = ++sightingRequest
  if (!thievesView.value || invalidDateRange.value) {
    sightingPage.value = null
    return
  }
  if (!sightingPage.value) sightingStatus.value = 'loading'
  try {
    const next = await $fetch<ThiefSightingPage>('/api/thief-sightings', {
      query: {
        server: serverScope.value === 'all' ? undefined : serverScope.value,
        q: characterQuery.value || undefined,
        from: fromDate.value ? localDateBoundary(fromDate.value, 0) : undefined,
        to: toDate.value ? localDateBoundary(toDate.value, 1) : undefined,
        cursor: sightingCursor.value || undefined,
        limit: pageSize.value,
      },
    })
    if (request !== sightingRequest) return
    sightingPage.value = next
    sightingStatus.value = 'current'
  } catch {
    if (request !== sightingRequest) return
    sightingStatus.value = 'error'
  }
}

function nextSightingPage() {
  if (!sightingPage.value?.next_cursor) return
  sightingPrevious.value.push(sightingCursor.value)
  sightingCursor.value = sightingPage.value.next_cursor
}
function previousSightingPage() {
  if (!sightingPrevious.value.length) return
  sightingCursor.value = sightingPrevious.value.pop() || ''
}
function sightingSource(row: ThiefSighting) {
  if (row.origin === 'phmon') return 'PhMon'
  return row.reporter.app?.trim() || 'AdvancedAutoTrade'
}
function sightingMapTarget(row: ThiefSighting) {
  const position = row.position
  if (!position) return undefined
  const profile = eventMapProfiles.value[row.server.toLowerCase()]
  const location = profile ? mapEventLocation(profile, position) : undefined
  if (!location || location.status !== 'mapped') return undefined
  return {
    path: '/map',
    query: {
      server: row.server,
      area: location.areaID,
      floor: location.floorID,
      ...(location.areaID === 'world'
        ? { region: String(position.region) }
        : {}),
    },
  }
}
function sightingLocation(row: ThiefSighting) {
  if (!row.position) return 'Position unavailable'
  return `Region ${row.position.region} · ${row.position.x.toFixed(1)}, ${row.position.y.toFixed(1)}`
}

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
function eventRowLayout(item: ActivityEvent) {
  if (
    item.kind === 'world.unique_spawned' ||
    item.kind === 'character.level_up'
  )
    return 'featured'
  if (item.kind === 'character.died') return 'death'
  if (item.kind === 'drop.rare' || item.kind === 'drop.item') return 'drop'
  return 'standard'
}
function isDropEvent(item: ActivityEvent) {
  return item.kind === 'drop.rare' || item.kind === 'drop.item'
}
function eventRowTitle(item: ActivityEvent) {
  if (item.kind === 'character.level_up') {
    const level = Number(record(item.payload).level)
    if (Number.isInteger(level) && level > 0)
      return `${item.character || 'Character'} leveled up ${level - 1} -> ${level}`
  }
  return eventSummary(item)
}
function eventRowSubtitle(item: ActivityEvent) {
  if (item.kind === 'world.unique_spawned')
    return item.server || 'Unknown server'
  const level = record(item.payload).level
  const levelLabel = level == null ? '' : `Lv ${String(level)}`
  return [
    item.character || 'Unknown character',
    item.zone || eventLocationText(item),
    levelLabel,
  ]
    .filter(Boolean)
    .join(' | ')
}
function eventDeathCause(item: ActivityEvent) {
  return deathCauseLabel(item.payload)
}
function eventRowDetail(item: ActivityEvent) {
  if (item.kind === 'character.died') return eventDeathCause(item)
  if (item.kind === 'drop.item' || item.kind === 'drop.rare') return ''
  return eventSourceLabel(item) || item.category.replaceAll('.', ' ')
}
function eventRowLocation(item: ActivityEvent) {
  return eventRowLocationText(item)
}
function eventSourceLabel(item: ActivityEvent) {
  if (item.kind === 'drop.item' || item.kind === 'drop.rare')
    return 'Drop observed'
  if (item.kind === 'item.acquired' && item.source === 'joymax.pet_inventory')
    return 'Pet pickup'
  if (
    (item.kind === 'item.acquired' ||
      item.kind === 'item.quantity_increased') &&
    item.source === 'phbot.state_diff'
  )
    return 'Owned item gain'
  return ''
}
function eventContainerDetail(item: ActivityEvent) {
  const payload = record(item.payload)
  const destination = record(payload.destination_container)
  if (
    item.source !== 'joymax.pet_inventory' &&
    item.source !== 'phbot.state_diff'
  )
    return ''
  if (typeof destination.type !== 'string') return ''
  const container =
    destination.type === 'pets'
      ? `Pet ${typeof destination.id === 'string' ? destination.id : 'unknown'}`
      : destination.type.replaceAll('_', ' ')
  const slot =
    typeof destination.slot === 'number' ? destination.slot + 1 : null
  const quantity =
    typeof payload.quantity_delta === 'number' ? payload.quantity_delta : null
  return [
    container,
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
        <span class="event-count">{{
          thievesView ? (sightingPage?.total ?? '—') : (page?.total ?? '—')
        }}</span>
      </div>

      <div class="events-filters">
        <label>
          <span>{{ thievesView ? 'Name' : 'Character' }}</span>
          <input
            v-model="characterInput"
            maxlength="64"
            :placeholder="
              thievesView
                ? 'Filter by thief or reporter'
                : 'Filter by character'
            "
            :aria-label="
              thievesView
                ? 'Filter thief sightings by name'
                : 'Filter events by character'
            "
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

      <div
        v-if="liveStale && !thievesView"
        class="status-banner warning"
        role="status"
      >
        Showing the last received event page as stale while PhMon reconnects.
      </div>
      <div
        v-if="thievesView && sightingStatus === 'error'"
        class="status-banner warning"
        role="alert"
      >
        Thief sightings could not be loaded.
      </div>
      <div class="event-rows-scroll">
        <div v-if="thievesView" class="event-history-rows">
          <article
            v-for="row in sightingPage?.sightings || []"
            :key="row.sighting_id"
            class="event-row-grid event-row-thief"
            :aria-label="`${row.thief_name} reported by ${row.reporter.name || 'an unknown reporter'}`"
          >
            <div class="event-row-primary">
              <strong>{{ row.thief_name }}</strong>
            </div>
            <time :datetime="row.received_at">{{
              formatTimestamp(row.received_at)
            }}</time>
            <div class="event-row-character">
              {{ row.reporter.name || '—' }}
            </div>
            <div class="event-row-detail">{{ sightingSource(row) }}</div>
            <div class="event-row-location">{{ sightingLocation(row) }}</div>
            <div class="event-row-map">
              <NuxtLink
                v-if="sightingMapTarget(row)"
                class="compact-button map-event-link"
                :to="sightingMapTarget(row) || '/map'"
                aria-label="Open thief sighting on map"
              >
                <UIcon name="i-lucide-map-pin" />
              </NuxtLink>
            </div>
          </article>
          <div v-if="!sightingPage?.sightings.length" class="event-empty-state">
            <strong>{{
              sightingStatus === 'current'
                ? 'No thief sightings found'
                : sightingStatus === 'error'
                  ? 'Thief sightings unavailable'
                  : 'Loading thief sightings'
            }}</strong>
            <span>{{
              sightingStatus === 'current'
                ? 'No sightings match this server, name and date range.'
                : 'Waiting for the thief sighting history.'
            }}</span>
          </div>
        </div>
        <div v-else class="event-history-rows">
          <template v-for="item in page?.events || []" :key="item.event_id">
            <article
              v-if="eventRowLayout(item) === 'featured'"
              class="event-row-featured"
              :class="{
                'event-row-unique': item.kind === 'world.unique_spawned',
                'event-row-level': item.kind === 'character.level_up',
              }"
            >
              <NuxtLink
                v-if="
                  item.kind === 'world.unique_spawned' &&
                  uniqueEventDetails(item)?.imageUrl &&
                  item.character_id
                "
                class="event-row-art-link"
                :to="`/characters/${item.character_id}`"
                :aria-label="`Open ${item.character} details`"
              >
                <img
                  class="event-row-art"
                  :src="uniqueEventDetails(item)?.imageUrl"
                  :alt="uniqueEventDetails(item)?.name || 'Unique'"
                />
              </NuxtLink>
              <img
                v-else-if="
                  item.kind === 'world.unique_spawned' &&
                  uniqueEventDetails(item)?.imageUrl
                "
                class="event-row-art"
                :src="uniqueEventDetails(item)?.imageUrl"
                :alt="uniqueEventDetails(item)?.name || 'Unique'"
              />
              <CharacterPortrait
                v-else
                :name="item.character || 'Unknown character'"
                :portrait-url="item.portrait_url"
                size="small"
                :to="
                  item.character_id
                    ? `/characters/${item.character_id}`
                    : undefined
                "
              />
              <div class="event-row-featured-copy">
                <strong>{{ eventRowTitle(item) }}</strong>
                <span>{{ eventRowSubtitle(item) }}</span>
              </div>
              <time :datetime="item.occurred_at">{{
                formatTimestamp(item.occurred_at)
              }}</time>
              <NuxtLink
                v-if="hasReliableMapLocation(item)"
                class="compact-button map-event-link featured-event-map-link"
                :to="eventMapTarget(item)"
                aria-label="Open event location on map"
              >
                <UIcon name="i-lucide-map-pin" />
              </NuxtLink>
            </article>

            <article
              v-else
              class="event-row-grid"
              :class="[
                `event-row-${eventRowLayout(item)}`,
                { 'event-row-rare': item.kind === 'drop.rare' },
              ]"
              :aria-label="eventSummary(item)"
            >
              <div class="event-row-primary">
                <template v-if="item.kind === 'character.died'">
                  <CharacterPortrait
                    :name="item.character || 'Unknown character'"
                    :portrait-url="item.portrait_url"
                    size="small"
                  />
                </template>
                <ItemDetailPopup
                  v-else-if="itemRecordFromActivityEvent(item)"
                  :item="itemRecordFromActivityEvent(item)!"
                  :to="eventItemTarget(item)"
                />
                <NuxtLink
                  v-else-if="item.character_id"
                  class="event-row-character-primary"
                  :to="`/characters/${item.character_id}`"
                >
                  <CharacterPortrait
                    :name="item.character"
                    :portrait-url="item.portrait_url"
                    size="small"
                  />
                  <strong>{{ eventSummary(item) }}</strong>
                </NuxtLink>
                <div v-else class="event-row-generic-primary">
                  <UIcon name="i-lucide-activity" />
                  <strong>{{ eventSummary(item) }}</strong>
                </div>
                <small
                  v-if="eventSourceLabel(item) && !isDropEvent(item)"
                  class="event-origin"
                >
                  {{ eventSourceLabel(item) }}
                  <template v-if="eventContainerDetail(item)">
                    · {{ eventContainerDetail(item) }}
                  </template>
                </small>
              </div>
              <time :datetime="item.occurred_at">{{
                formatTimestamp(item.occurred_at)
              }}</time>
              <div class="event-row-character">
                <NuxtLink
                  v-if="item.character_id"
                  :to="`/characters/${item.character_id}`"
                  >{{ item.character }}</NuxtLink
                >
                <span v-else>{{ item.character || '—' }}</span>
              </div>
              <div
                v-if="item.kind === 'character.died'"
                class="event-row-detail"
              >
                {{ eventDeathCause(item) }}
              </div>
              <div v-else-if="!isDropEvent(item)" class="event-row-detail">
                {{ eventRowDetail(item) || '—' }}
              </div>
              <div class="event-row-location">
                {{ eventRowLocation(item) }}
              </div>
              <div class="event-row-map">
                <NuxtLink
                  v-if="hasReliableMapLocation(item)"
                  class="compact-button map-event-link"
                  :to="eventMapTarget(item)"
                  aria-label="Open event location on map"
                >
                  <UIcon name="i-lucide-map-pin" />
                </NuxtLink>
                <button
                  v-else-if="
                    item.kind === 'character.died' || item.kind === 'drop.rare'
                  "
                  class="compact-button map-event-link"
                  type="button"
                  disabled
                  title="Map transforms are not yet validated for this region"
                >
                  <UIcon name="i-lucide-map-pin" />
                </button>
              </div>
            </article>
          </template>
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
            :disabled="
              thievesView ? !sightingPrevious.length : !previousCursors.length
            "
            @click="thievesView ? previousSightingPage() : previousPage()"
          >
            Previous
          </button>
          <span>
            Page
            {{
              (thievesView ? sightingPrevious.length : previousCursors.length) +
              1
            }}
            /
            {{
              Math.max(
                1,
                Math.ceil(
                  ((thievesView ? sightingPage?.total : page?.total) || 0) /
                    pageSize,
                ),
              )
            }}
            ·
            {{ (thievesView ? sightingPage?.total : page?.total) || 0 }}
            entries
          </span>
          <button
            class="compact-button"
            type="button"
            :disabled="
              thievesView ? !sightingPage?.next_cursor : !page?.next_cursor
            "
            @click="thievesView ? nextSightingPage() : nextPage()"
          >
            Next
          </button>
        </div>
      </footer>
    </section>
  </div>
</template>
