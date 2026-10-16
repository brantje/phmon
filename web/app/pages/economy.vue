<script setup lang="ts">
import { createLatestRequestGate, economySearchQuery } from '~/utils/searchPage'
import {
  offerDegrees,
  offerItemChips,
  offerItemTypes,
  offerSubtypeOptions,
} from '~/utils/offerTaxonomy'
import type { GlobalOffer, GlobalOffersPage } from '~~/shared/types/live'

type Offer = GlobalOffer
type OfferPage = GlobalOffersPage
const { serverScope } = useServerScope()
const type = ref('')
const itemType = ref('')
const subtype = ref('')
const degree = ref('')
const seller = ref('')
const search = ref('')
const from = ref('')
const to = ref('')
const page = shallowRef<OfferPage | null>(null)
// Tab counts follow every filter except the offer type, so keep the last
// complete set while the next page loads.
const counts = ref<Record<string, number>>({})
const error = ref('')
const loading = ref(false)
const cursor = ref('')
const previous = ref<string[]>([])
const requestGate = createLatestRequestGate()
let searchTimer: ReturnType<typeof setTimeout> | undefined
const searchInput = ref('')
const invalidRange = computed(
  () => !!from.value && !!to.value && from.value >= to.value,
)
const { liveStale, connectionState } = useLiveData()
const stale = computed(
  () => liveStale.value || connectionState.value === 'stale',
)
const subtypeOptions = computed(() => offerSubtypeOptions(itemType.value))
const tabs = [
  { key: '', label: 'All' },
  { key: 'WTB', label: 'WTB' },
  { key: 'WTS', label: 'WTS' },
  { key: 'WTT', label: 'WTT' },
  { key: 'unclassified', label: 'Other' },
]
function tabCount(key: string) {
  if (key === 'unclassified') return null
  if (key === '')
    return ['WTB', 'WTS', 'WTT'].reduce(
      (sum, k) => sum + (counts.value[k] || 0),
      0,
    )
  return counts.value[key] ?? null
}
function load() {
  const id = requestGate.next()
  if (invalidRange.value) {
    page.value = null
    error.value = 'The start date must be before the end date.'
    loading.value = false
    return
  }
  error.value = ''
  loading.value = true
  page.value = null
  void $fetch<OfferPage>('/api/economy/offers', {
    query: economySearchQuery({
      server: serverScope.value,
      type: type.value,
      sender: seller.value,
      query: search.value,
      from: from.value,
      to: to.value,
      cursor: cursor.value,
      itemType: itemType.value,
      subtype: subtype.value,
      degree: degree.value,
    }),
  })
    .then((response) => {
      if (!requestGate.isCurrent(id)) return
      page.value = response
      if (response.counts) counts.value = response.counts
    })
    .catch(() => {
      if (requestGate.isCurrent(id))
        error.value = 'Global offer history is unavailable.'
    })
    .finally(() => {
      if (requestGate.isCurrent(id)) loading.value = false
    })
}
const pmLink = (offer: Offer) => ({
  path: '/chat',
  query: {
    channel: 'private',
    character_id: offer.observer_character_id,
    peer: offer.sender,
  },
})
function offerIcons(offer: Offer) {
  const urls: string[] = []
  for (const item of offer.items || [])
    if (item.icon_url && !urls.includes(item.icon_url) && urls.length < 2)
      urls.push(item.icon_url)
  return urls
}
function offerChipRows(offer: Offer) {
  return (offer.items || []).map((item) => offerItemChips(item))
}
watch(searchInput, (value) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    search.value = value.trim()
  }, 250)
})
watch(itemType, () => {
  subtype.value = ''
})
watch(
  [serverScope, type, seller, search, from, to, itemType, subtype, degree],
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
  searchInput.value = ''
  search.value = ''
  type.value = ''
  itemType.value = ''
  subtype.value = ''
  degree.value = ''
  seller.value = ''
  from.value = ''
  to.value = ''
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
  <div class="economy-page">
    <PageHeader
      title="Global Economy"
      icon="i-lucide-coins"
      description="WTB, WTS and WTT advertisements from preserved Global chat observations. Offers are not completed trades."
    />
    <div class="economy-tabs" role="tablist" aria-label="Offer type">
      <button
        v-for="tab in tabs"
        :key="tab.key"
        type="button"
        role="tab"
        class="economy-tab"
        :class="[`kind-${tab.key || 'all'}`, { active: type === tab.key }]"
        :aria-selected="type === tab.key"
        @click="type = tab.key"
      >
        {{ tab.label }}
        <span v-if="tabCount(tab.key) !== null" class="economy-count">{{
          tabCount(tab.key)
        }}</span>
      </button>
    </div>
    <section class="panel economy-filters" aria-label="Global economy filters">
      <form class="economy-filter-row" @submit.prevent="load">
        <label>
          <span>Search</span>
          <input
            v-model="searchInput"
            maxlength="128"
            placeholder="Search offer text, item or notes"
            aria-label="Search Global offer text"
          />
        </label>
        <label>
          <span>Buyer / seller</span>
          <input
            v-model="seller"
            maxlength="64"
            placeholder="Filter by player"
          />
        </label>
        <label>
          <span>Item type</span>
          <select v-model="itemType">
            <option value="">Any type</option>
            <option v-for="t in offerItemTypes" :key="t.key" :value="t.key">
              {{ t.label }}
            </option>
          </select>
        </label>
        <label>
          <span>Subcategory</span>
          <select v-model="subtype" :disabled="!subtypeOptions.length">
            <option value="">Any subcategory</option>
            <option v-for="t in subtypeOptions" :key="t.key" :value="t.key">
              {{ t.label }}
            </option>
          </select>
        </label>
        <label>
          <span>Degree</span>
          <select v-model="degree">
            <option value="">Any degree</option>
            <option v-for="d in offerDegrees" :key="d" :value="String(d)">
              D{{ d }}
            </option>
          </select>
        </label>
        <label>
          <span>From</span>
          <input v-model="from" type="date" />
        </label>
        <label>
          <span>To (exclusive)</span>
          <input v-model="to" type="date" />
        </label>
        <button type="button" class="compact-button" @click="resetFilters">
          Reset filters
        </button>
      </form>
    </section>
    <p v-if="stale" class="status-banner warning" role="status">
      The live connection is stale. Saved Global messages remain available.
    </p>
    <p v-if="error" class="status-banner warning" role="alert">{{ error }}</p>
    <section aria-label="Classified Global offers">
      <div class="economy-list-head">
        <strong>{{ page ? `${page.total} offers` : 'Offers' }}</strong>
        <span>Newest first</span>
        <span v-if="loading" role="status">Loading offers…</span>
      </div>
      <div
        v-if="!loading && !error && page && !page.offers.length"
        class="event-empty-state"
      >
        <strong>No matching Global messages</strong>
        <span
          >No observed advertisements match the selected source and
          filters.</span
        >
      </div>
      <div class="economy-grid">
        <article
          v-for="offer in page?.offers || []"
          :key="offer.message_id"
          class="economy-card panel"
        >
          <header>
            <span class="economy-kind" :class="`kind-${offer.offer_type}`">{{
              offer.offer_type
            }}</span>
            <FormattedDate :value="offer.occurred_at" />
          </header>
          <div class="economy-body">
            <div v-if="offerIcons(offer).length" class="economy-icons">
              <span
                v-for="url in offerIcons(offer)"
                :key="url"
                class="economy-icon"
              >
                <img :src="url" alt="" loading="lazy" />
              </span>
            </div>
            <p class="economy-text">{{ offer.message }}</p>
          </div>
          <div
            v-for="(chips, index) in offerChipRows(offer)"
            :key="index"
            class="economy-chips"
          >
            <span v-for="chip in chips" :key="chip" class="economy-chip">{{
              chip
            }}</span>
          </div>
          <p class="economy-meta">
            <strong>{{ offer.sender }}</strong>
            <span>{{ offer.server }}</span>
          </p>
          <footer>
            <NuxtLink class="compact-button" :to="pmLink(offer)"
              >Write PM</NuxtLink
            >
            <NuxtLink
              class="economy-source"
              :to="{
                path: '/events',
                query: { event_id: offer.source_event_id },
              }"
              :title="`Seen by ${offer.observer}`"
              >Source event</NuxtLink
            >
          </footer>
        </article>
      </div>
      <nav
        class="mt-4 flex items-center justify-between gap-3"
        aria-label="Global offers pages"
      >
        <button
          type="button"
          class="compact-button"
          :disabled="loading || !previous.length"
          @click="previousPage"
        >
          Previous
        </button>
        <span>{{ page?.offers.length ?? 0 }} results on this page</span>
        <button
          type="button"
          class="compact-button"
          :disabled="loading || !page?.next_cursor"
          @click="nextPage"
        >
          Next
        </button>
      </nav>
      <p class="mt-4 text-sm">
        Prices mentioned in advertisements are asking prices only. Confirmed
        stall sales, realized prices and price history are unsupported without
        verified transaction evidence.
      </p>
    </section>
  </div>
</template>
