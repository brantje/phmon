<script setup lang="ts">
import {
  createLatestRequestGate,
  itemHistoryTarget,
  itemOwnerHref,
  itemSearchQuery,
} from '~/utils/searchPage'
type FoundItem = {
  server: string
  container: string
  owner_kind: 'character' | 'guild' | 'pet'
  owner_id: string
  owner_name?: string
  guild?: string
  observer_character_id: string
  observer_name: string
  pet_id?: string
  slot: number
  revision: number
  observed_at: string
  checked_at: string
  item: Record<string, unknown>
  quantity_exact?: string
  metadata?: Record<string, unknown>
}
type ItemSearchPage = {
  items: FoundItem[]
  total: number
  next_cursor?: string
}
const { serverScope } = useServerScope()
const { liveStale, connectionState } = useLiveData()
const queryInput = ref('')
const query = ref('')
const container = ref('')
const owner = ref('')
const itemType = ref('')
const subcategory = ref('')
const degree = ref('')
const includeSets = ref(true)
const cursor = ref('')
const previous = ref<string[]>([])
const page = shallowRef<ItemSearchPage | null>(null)
const loading = ref(false)
const error = ref('')
const requestGate = createLatestRequestGate()
let searchTimer: ReturnType<typeof setTimeout> | undefined
const stale = computed(
  () => liveStale.value || connectionState.value === 'stale',
)
const needsServerForTaxonomy = computed(
  () =>
    serverScope.value === 'all' &&
    Boolean(itemType.value || subcategory.value || degree.value),
)
function formatTime(value: string) {
  const date = new Date(value)
  return Number.isNaN(date.getTime()) ? 'Unknown' : date.toLocaleString()
}
function itemIdentity(item: FoundItem) {
  return (
    String(item.item.name ?? '') ||
    String(item.item.servername ?? '') ||
    (item.item.model != null
      ? `Model ${String(item.item.model)}`
      : 'Unidentified item')
  )
}
function load() {
  const id = requestGate.next()
  if (needsServerForTaxonomy.value) {
    error.value = 'Select a server before applying item taxonomy filters.'
    page.value = null
    loading.value = false
    return
  }
  loading.value = true
  error.value = ''
  page.value = null
  void $fetch<ItemSearchPage>('/api/items', {
    query: itemSearchQuery({
      server: serverScope.value,
      query: query.value,
      container: container.value,
      owner: owner.value,
      itemType: itemType.value,
      subcategory: subcategory.value,
      degree: degree.value,
      includeSets: includeSets.value,
      cursor: cursor.value,
    }),
  })
    .then((result) => {
      if (requestGate.isCurrent(id)) page.value = result
    })
    .catch(() => {
      if (requestGate.isCurrent(id))
        error.value = 'Item Search could not be loaded.'
    })
    .finally(() => {
      if (requestGate.isCurrent(id)) loading.value = false
    })
}
watch(queryInput, (value) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    query.value = value.trim()
  }, 250)
})
watch(
  [
    serverScope,
    query,
    container,
    owner,
    itemType,
    subcategory,
    degree,
    includeSets,
  ],
  () => {
    cursor.value = ''
    previous.value = []
    if (import.meta.client) load()
  },
)
watch(cursor, () => {
  if (import.meta.client) load()
})
onMounted(load)
onUnmounted(() => {
  requestGate.invalidate()
  if (searchTimer) clearTimeout(searchTimer)
})
function resetFilters() {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = undefined
  queryInput.value = ''
  query.value = ''
  container.value = ''
  owner.value = ''
  itemType.value = ''
  subcategory.value = ''
  degree.value = ''
  includeSets.value = true
  cursor.value = ''
  previous.value = []
}
function nextPage() {
  if (!page.value?.next_cursor) return
  previous.value.push(cursor.value)
  cursor.value = page.value.next_cursor
}
function previousPage() {
  if (!previous.value.length) return
  cursor.value = previous.value.pop() || ''
}
</script>

<template>
  <div class="items-page">
    <PageHeader
      title="Item Search"
      icon="i-lucide-search"
      description="Search last confirmed item observations across characters, pets, personal storage and guild storage. Item contents may be stale; guild rows use one observer snapshot."
    />
    <section class="panel p-4" aria-label="Item search filters">
      <form
        class="grid gap-3 sm:grid-cols-2 lg:grid-cols-4"
        @submit.prevent="load"
      >
        <label class="grid gap-1">
          <span>Item name, code or model</span>
          <input
            v-model="queryInput"
            class="min-w-0 rounded border p-2"
            maxlength="128"
            placeholder="Search items"
            aria-label="Search item name, code or model"
          />
        </label>
        <label class="grid gap-1">
          <span>Container</span>
          <select v-model="container" class="rounded border p-2">
            <option value="">All containers</option>
            <option value="inventory">Bag inventory</option>
            <option value="equipment">Equipment</option>
            <option value="storage">Personal storage</option>
            <option value="guild_storage">Guild storage</option>
            <option value="pets">Pet inventory</option>
            <option value="job_pouch">Job pouch</option>
          </select>
        </label>
        <label class="grid gap-1">
          <span>Character, guild or pet owner</span>
          <input
            v-model="owner"
            class="min-w-0 rounded border p-2"
            maxlength="100"
            placeholder="Owner"
            aria-label="Filter item owner"
          />
        </label>
        <label class="grid gap-1">
          <span>Item type</span>
          <input
            v-model="itemType"
            class="min-w-0 rounded border p-2"
            maxlength="64"
            placeholder="Verified catalog type"
          />
        </label>
        <label class="grid gap-1">
          <span>Subcategory</span>
          <input
            v-model="subcategory"
            class="min-w-0 rounded border p-2"
            maxlength="64"
            placeholder="Verified catalog subcategory"
          />
        </label>
        <label class="grid gap-1">
          <span>Degree</span>
          <select v-model="degree" class="rounded border p-2">
            <option value="">Any degree</option>
            <option v-for="n in 20" :key="n" :value="String(n)">{{ n }}</option>
          </select>
        </label>
        <label class="flex items-center gap-2 self-end p-2">
          <input v-model="includeSets" type="checkbox" />
          <span>Include equipped items / character sets</span>
        </label>
        <button class="compact-button self-end" type="submit">Refresh</button>
        <button
          class="compact-button self-end"
          type="button"
          @click="resetFilters"
        >
          Reset filters
        </button>
      </form>
    </section>
    <p v-if="stale" role="status" class="status-banner warning">
      Agent data may be stale or disconnected. Results show the most recently
      saved observed snapshots.
    </p>
    <p v-if="error" role="alert" class="status-banner warning">{{ error }}</p>
    <section class="panel p-4" aria-label="Item search results">
      <div class="mb-3 flex items-center justify-between gap-3">
        <strong>{{
          page ? `${page.total} matching slots` : 'Item results'
        }}</strong>
        <span v-if="loading" role="status">Loading items…</span>
      </div>
      <div
        v-if="!loading && !error && page && !page.items.length"
        class="event-empty-state"
      >
        <strong>No matching observed items</strong>
        <span
          >Storage that is closed or unavailable is not assumed empty and is not
          included as current evidence.</span
        >
      </div>
      <div class="grid gap-3">
        <article
          v-for="row in page?.items || []"
          :key="`${row.server}/${row.observer_character_id}/${row.container}/${row.pet_id || ''}/${row.slot}`"
          class="rounded border p-3"
          :aria-label="`${itemIdentity(row)} in ${row.container}`"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <ItemDetailPopup :item="row.item" />
            <span
              >{{ row.server }} · {{ row.container.replaceAll('_', ' ') }} ·
              slot {{ row.slot }}</span
            >
          </div>
          <div class="mt-2 flex flex-wrap gap-x-4 gap-y-1 text-sm">
            <span
              >Owner:
              <NuxtLink :to="itemOwnerHref(row)">{{
                row.owner_name || row.owner_id
              }}</NuxtLink></span
            >
            <span
              >Observed by:
              <NuxtLink :to="`/characters/${row.observer_character_id}`">{{
                row.observer_name
              }}</NuxtLink></span
            >
            <span v-if="row.pet_id">Pet ID: {{ row.pet_id }}</span>
            <NuxtLink :to="itemHistoryTarget(row)">
              View canonical item events
            </NuxtLink>
            <span
              >Quantity:
              {{ row.quantity_exact || row.item.quantity || 'unknown' }}</span
            >
            <span>+{{ row.item.plus ?? '?' }}</span>
            <span
              >Observed:
              <time :datetime="row.observed_at">{{
                formatTime(row.observed_at)
              }}</time></span
            >
            <span
              >Checked:
              <time :datetime="row.checked_at">{{
                formatTime(row.checked_at)
              }}</time></span
            >
          </div>
        </article>
      </div>
      <nav
        class="mt-4 flex items-center justify-between gap-3"
        aria-label="Item search pages"
      >
        <button
          type="button"
          class="compact-button"
          :disabled="!previous.length || loading"
          @click="previousPage"
        >
          Previous
        </button>
        <span>{{ page?.items.length ?? 0 }} results on this page</span>
        <button
          type="button"
          class="compact-button"
          :disabled="!page?.next_cursor || loading"
          @click="nextPage"
        >
          Next
        </button>
      </nav>
    </section>
  </div>
</template>
