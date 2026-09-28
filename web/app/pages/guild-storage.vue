<script setup lang="ts">
type Scope = {
  key: string
  server: string
  guild: string
  label: string
}
type GuildResource = {
  resource_key: string
  availability: string
  payload: Record<string, unknown>
  observed_at?: string
  checked_at?: string
  observer_character_id?: string
  observer_name?: string
}
type GuildStorageSnapshot = {
  server: string
  guild: string
  items: GuildResource[]
  status?: string
}
type SlotItem = {
  source_slot: number
  displayed_slot?: number
  item: Record<string, unknown>
}

const { fleetCharacters, liveStale, connectionState } = useLiveData()
const { matchesServer } = useServerScope()
const scopes = computed<Scope[]>(() => {
  const unique = new Map<string, Scope>()
  for (const character of fleetCharacters.value) {
    if (!matchesServer(character.server)) continue
    const guild = character.guild?.trim()
    if (!guild) continue
    const key = `${character.server.toLocaleLowerCase()}\u0000${guild.toLocaleLowerCase()}`
    if (!unique.has(key)) {
      unique.set(key, {
        key,
        server: character.server,
        guild,
        label: `${character.server} · ${guild}`,
      })
    }
  }
  return [...unique.values()].sort((left, right) =>
    left.label.localeCompare(right.label),
  )
})
const selectedScopeKey = ref('')
const selectedScope = computed(() =>
  scopes.value.find((scope) => scope.key === selectedScopeKey.value),
)
const snapshot = shallowRef<GuildStorageSnapshot | null>(null)
const loading = ref(false)
const requestFailed = ref(false)
const search = ref('')
const resource = computed(() =>
  snapshot.value?.items.find((item) => item.resource_key === 'guild_storage'),
)
const payload = computed(() => resource.value?.payload || {})
const hasSlots = computed(() => Array.isArray(payload.value.slots))
const effectiveAvailability = computed(() =>
  requestFailed.value
    ? 'unavailable'
    : resource.value?.availability || 'unavailable',
)
const observation = computed(() => ({
  ...payload.value,
  availability: resource.value?.availability || 'unavailable',
  observed_at: resource.value?.observed_at,
  checked_at: resource.value?.checked_at,
}))
const occupiedSlots = computed<SlotItem[]>(() => {
  const slots = payload.value.slots
  if (!Array.isArray(slots)) return []
  return slots.filter(
    (slot): slot is SlotItem =>
      !!slot &&
      typeof slot === 'object' &&
      'item' in slot &&
      !!slot.item &&
      typeof slot.item === 'object',
  )
})
const matchingSlots = computed(() => {
  const query = search.value.trim().toLocaleLowerCase()
  if (!query) return occupiedSlots.value
  return occupiedSlots.value.filter(({ item }) =>
    [item.name, item.servername, item.model]
      .map((value) => String(value ?? '').toLocaleLowerCase())
      .some((value) => value.includes(query)),
  )
})
const hasObservation = computed(() => !!resource.value)
let requestRevision = 0

async function loadGuildStorage() {
  const scope = selectedScope.value
  if (!scope || loading.value) return
  const revision = ++requestRevision
  loading.value = true
  try {
    const result = await $fetch<GuildStorageSnapshot>('/api/guild-storage', {
      query: { server: scope.server, guild: scope.guild },
    })
    if (revision !== requestRevision) return
    snapshot.value = result
    requestFailed.value = false
  } catch {
    if (revision === requestRevision) requestFailed.value = true
  } finally {
    if (revision === requestRevision) loading.value = false
  }
}

watch(
  scopes,
  (values) => {
    if (!values.some((scope) => scope.key === selectedScopeKey.value)) {
      selectedScopeKey.value = values[0]?.key || ''
    }
  },
  { immediate: true },
)
watch(
  selectedScopeKey,
  (next, previous) => {
    requestRevision += 1
    loading.value = false
    if (next !== previous) {
      snapshot.value = null
      requestFailed.value = false
    }
    void loadGuildStorage()
  },
  { immediate: true },
)
let refreshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  refreshTimer = setInterval(() => void loadGuildStorage(), 15_000)
})
onBeforeUnmount(() => {
  if (refreshTimer) clearInterval(refreshTimer)
})
</script>

<template>
  <div>
    <!-- eslint-disable vue/html-self-closing -->
    <PageHeader
      title="Guild Storage"
      icon="i-lucide-warehouse"
      description="Server and guild scoped contents from phBot observers."
    />
    <section class="panel guild-storage-panel">
      <div class="guild-storage-toolbar">
        <label>
          <span>Server · Guild</span>
          <select v-model="selectedScopeKey" aria-label="Server and guild">
            <option v-for="scope in scopes" :key="scope.key" :value="scope.key">
              {{ scope.label }}
            </option>
          </select>
        </label>
        <label>
          <span>Search items</span>
          <input
            v-model="search"
            maxlength="100"
            type="search"
            placeholder="Search item name, code or model"
            aria-label="Search guild storage items"
          />
        </label>
        <button
          class="compact-button"
          type="button"
          :disabled="!selectedScope || loading"
          @click="loadGuildStorage"
        >
          Refresh
        </button>
      </div>

      <div
        v-if="liveStale || connectionState === 'stale'"
        class="inventory-stale"
        role="status"
      >
        Character server and guild choices are based on the last live snapshot.
      </div>
      <div
        v-if="requestFailed && snapshot"
        class="inventory-stale"
        role="status"
      >
        Guild storage refresh failed. Showing the last retrieved contents for
        this server and guild.
      </div>
      <div
        v-if="scopes.length === 0"
        class="inventory-unavailable"
        role="status"
      >
        <strong>No guild scope available</strong>
        <span
          >Waiting for a character observation with a server and guild.</span
        >
      </div>
      <div
        v-else-if="loading && !snapshot"
        class="inventory-unavailable"
        role="status"
      >
        Loading guild storage…
      </div>
      <template v-else-if="selectedScope">
        <div v-if="resource" class="guild-storage-observer">
          <span>
            Observer:
            {{
              resource.observer_name ||
              resource.observer_character_id ||
              'Unknown'
            }}
          </span>
          <span>
            Last contents observed
            {{
              resource.observed_at
                ? formatTimestamp(resource.observed_at)
                : 'never'
            }}
          </span>
          <span>
            Checked
            {{
              resource.checked_at ? formatTimestamp(resource.checked_at) : '—'
            }}
          </span>
        </div>
        <div
          v-if="hasObservation && search.trim() && hasSlots"
          class="guild-search-summary"
        >
          {{ matchingSlots.length }} matching items · source slot order retained
        </div>
        <div
          v-if="search.trim() && hasSlots"
          class="inventory-grid guild-search-results"
        >
          <div
            v-if="effectiveAvailability !== 'observed'"
            class="inventory-stale guild-search-stale"
            role="status"
          >
            Search results show last confirmed contents from
            {{
              resource?.observed_at
                ? formatTimestamp(resource.observed_at)
                : 'an unknown time'
            }}.
          </div>
          <ItemSlot
            v-for="slot in matchingSlots"
            :key="slot.source_slot"
            :item="slot.item"
            :slot-number="slot.source_slot"
            :label="`Guild storage · source slot ${slot.source_slot + 1}`"
          />
          <p v-if="matchingSlots.length === 0" class="inventory-empty">
            No items match this search.
          </p>
        </div>
        <InventoryGrid
          v-else
          title="Guild storage"
          :observation="observation"
          :availability="effectiveAvailability"
          :reason="requestFailed ? 'refresh_failed' : undefined"
        />
        <p v-if="resource" class="mapping-note">
          Contents are scoped to {{ snapshot?.server }} · {{ snapshot?.guild }}.
          The observer’s last confirmed view may be stale when phBot has not
          opened guild storage recently.
        </p>
      </template>
    </section>
  </div>
</template>
