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
const deleteDialogOpen = ref(false)
const deleteDialogElement = ref<HTMLDialogElement | null>(null)
const deleteConfirmation = ref('')
const deleting = ref(false)
const deleteError = ref('')
const deleteResult = ref('')
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
let isMounted = false

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

function openDeleteDialog() {
  deleteConfirmation.value = ''
  deleteError.value = ''
  deleteDialogOpen.value = true
}

function closeDeleteDialog() {
  if (deleting.value) return
  deleteDialogOpen.value = false
}

watch(deleteDialogOpen, async (open) => {
  await nextTick()
  const dialog = deleteDialogElement.value
  if (!dialog) return
  if (open && !dialog.open) dialog.showModal()
  if (!open && dialog.open) dialog.close()
})

async function deleteGuildStorage() {
  const scope = selectedScope.value
  if (!scope || deleteConfirmation.value !== scope.guild || deleting.value)
    return
  requestRevision += 1
  loading.value = false
  deleting.value = true
  deleteError.value = ''
  try {
    const result = await $fetch<{
      deleted_observations: number
      deleted_items: number
    }>('/api/guild-storage', {
      method: 'DELETE',
      body: {
        server: scope.server,
        guild: scope.guild,
        confirmation: deleteConfirmation.value,
      },
    })
    deleteDialogOpen.value = false
    snapshot.value = null
    requestFailed.value = false
    deleteResult.value = `Removed ${result.deleted_observations} saved observation(s) and ${result.deleted_items} item row(s) for ${scope.server} · ${scope.guild}. A later phBot observation may create a new saved snapshot; in-game contents were not changed.`
    await loadGuildStorage()
  } catch {
    deleteError.value =
      'Could not remove the saved guild storage records. No in-game contents were changed.'
  } finally {
    deleting.value = false
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
    if (isMounted) void loadGuildStorage()
  },
  { immediate: true },
)
let refreshTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  isMounted = true
  void loadGuildStorage()
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
        <button
          v-if="resource"
          class="compact-button danger-button"
          type="button"
          :disabled="loading || deleting"
          @click="openDeleteDialog"
        >
          Remove saved records
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
      <div v-if="deleteResult" class="inventory-empty" role="status">
        <span>{{ deleteResult }}</span>
        <button class="compact-button" type="button" @click="deleteResult = ''">
          Dismiss
        </button>
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
    <dialog
      ref="deleteDialogElement"
      class="record-delete-dialog"
      aria-labelledby="guild-storage-delete-title"
      @cancel.prevent="closeDeleteDialog"
    >
      <form method="dialog" @submit.prevent="deleteGuildStorage">
        <h2 id="guild-storage-delete-title">
          Remove saved guild storage records?
        </h2>
        <p>
          This removes all persisted guild-storage snapshots and item rows for
          <strong
            >{{ selectedScope?.server }} · {{ selectedScope?.guild }}</strong
          >
          from PhMon. It does not remove or change items in the game. Later
          phBot observations may create new saved records.
        </p>
        <label for="guild-storage-confirmation">
          Type the exact guild name to confirm
        </label>
        <input
          id="guild-storage-confirmation"
          v-model="deleteConfirmation"
          :disabled="deleting"
          autocomplete="off"
          maxlength="100"
          :placeholder="selectedScope?.guild || ''"
        />
        <p v-if="deleteError" class="dialog-error" role="alert">
          {{ deleteError }}
        </p>
        <div class="record-delete-actions">
          <button
            class="compact-button"
            type="button"
            :disabled="deleting"
            @click="closeDeleteDialog"
          >
            Cancel
          </button>
          <button
            class="compact-button danger-button"
            type="submit"
            :disabled="deleting || deleteConfirmation !== selectedScope?.guild"
          >
            {{ deleting ? 'Removing…' : 'Remove saved records' }}
          </button>
        </div>
      </form>
    </dialog>
  </div>
</template>
