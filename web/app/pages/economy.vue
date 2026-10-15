<script setup lang="ts">
type Offer = {
  message_id: string
  source_event_id: string
  server: string
  observer_character_id: string
  observer: string
  sender: string
  channel: string
  raw_type: string
  message: string
  offer_type: 'WTB' | 'WTS' | 'WTT' | 'unclassified'
  occurred_at: string
}
type OfferPage = {
  offers: Offer[]
  total: number
  next_cursor?: string
}
const { serverScope } = useServerScope()
const type = ref('')
const seller = ref('')
const search = ref('')
const from = ref('')
const to = ref('')
const page = shallowRef<OfferPage | null>(null)
const error = ref('')
const loading = ref(false)
const cursor = ref('')
const previous = ref<string[]>([])
let requestID = 0
let searchTimer: ReturnType<typeof setTimeout> | undefined
const searchInput = ref('')
const invalidRange = computed(
  () => !!from.value && !!to.value && from.value > to.value,
)
const { liveStale, connectionState } = useLiveData()
const stale = computed(
  () => liveStale.value || connectionState.value === 'stale',
)
function load() {
  const id = ++requestID
  if (invalidRange.value) {
    page.value = null
    error.value = 'The start date must be before the end date.'
    return
  }
  error.value = ''
  loading.value = true
  void $fetch<OfferPage>('/api/economy/offers', {
    query: {
      server: serverScope.value === 'all' ? undefined : serverScope.value,
      offer_type: type.value || undefined,
      sender: seller.value.trim() || undefined,
      q: search.value || undefined,
      from: from.value || undefined,
      to: to.value || undefined,
      cursor: cursor.value || undefined,
      limit: 30,
    },
  })
    .then((response) => {
      if (id === requestID) page.value = response
    })
    .catch(() => {
      if (id === requestID) error.value = 'Global offer history is unavailable.'
    })
    .finally(() => {
      if (id === requestID) loading.value = false
    })
}
watch(searchInput, (value) => {
  if (searchTimer) clearTimeout(searchTimer)
  searchTimer = setTimeout(() => {
    search.value = value.trim()
  }, 250)
})
watch([serverScope, type, seller, search, from, to], () => {
  cursor.value = ''
  previous.value = []
  if (import.meta.client) load()
})
watch(cursor, () => {
  if (import.meta.client) load()
})
onMounted(load)
onUnmounted(() => {
  requestID++
  if (searchTimer) clearTimeout(searchTimer)
})
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
    <section class="panel p-4" aria-label="Global economy filters">
      <form
        class="grid gap-3 sm:grid-cols-2 lg:grid-cols-3"
        @submit.prevent="load"
      >
        <label class="grid gap-1">
          <span>Offer type</span>
          <select v-model="type" class="rounded border p-2">
            <option value="">All classified offers</option>
            <option value="WTB">WTB · Want to buy</option>
            <option value="WTS">WTS · Want to sell</option>
            <option value="WTT">WTT · Want to trade</option>
            <option value="unclassified">Unclassified Global messages</option>
          </select>
        </label>
        <label class="grid gap-1">
          <span>Item, offer text or notes</span>
          <input
            v-model="searchInput"
            maxlength="128"
            placeholder="Search original messages"
            aria-label="Search Global offer text"
            class="min-w-0 rounded border p-2"
          />
        </label>
        <label class="grid gap-1">
          <span>Buyer or seller name</span>
          <input
            v-model="seller"
            maxlength="64"
            class="min-w-0 rounded border p-2"
            placeholder="Sender name"
          />
        </label>
        <label class="grid gap-1">
          <span>From</span>
          <input v-model="from" type="date" class="rounded border p-2" />
        </label>
        <label class="grid gap-1">
          <span>To (exclusive)</span>
          <input v-model="to" type="date" class="rounded border p-2" />
        </label>
        <button type="submit" class="compact-button self-end">
          Refresh offers
        </button>
      </form>
    </section>
    <p v-if="stale" class="status-banner warning" role="status">
      The live connection is stale. Saved Global messages remain available.
    </p>
    <p v-if="error" class="status-banner warning" role="alert">{{ error }}</p>
    <section class="panel p-4" aria-label="Classified Global offers">
      <div class="mb-3 flex flex-wrap items-center justify-between gap-3">
        <strong>{{ page ? `${page.total} messages` : 'Global offers' }}</strong>
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
      <div class="grid gap-3">
        <article
          v-for="offer in page?.offers || []"
          :key="offer.message_id"
          class="rounded border p-3"
        >
          <div class="flex flex-wrap items-center justify-between gap-3">
            <strong>{{ offer.offer_type }}</strong>
            <time :datetime="offer.occurred_at">
              {{ new Date(offer.occurred_at).toLocaleString() }}
            </time>
          </div>
          <p class="my-2 whitespace-pre-wrap break-words">
            {{ offer.message }}
          </p>
          <div class="flex flex-wrap gap-3 text-sm">
            <span>{{ offer.server }} · Global</span>
            <span>Sender: {{ offer.sender }}</span>
            <NuxtLink
              :to="`/chat?channel=global&character_id=${offer.observer_character_id}`"
            >
              View Global chat
            </NuxtLink>
            <span>Observed by {{ offer.observer }}</span>
          </div>
          <details class="mt-2 text-sm">
            <summary>Source observation</summary>
            <p>Message ID: {{ offer.message_id }}</p>
            <NuxtLink
              :to="{
                path: '/events',
                query: { event_id: offer.source_event_id },
              }"
            >
              Inspect canonical chat event {{ offer.source_event_id }}
            </NuxtLink>
          </details>
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
