<script setup lang="ts">
import {
  filterSearchItems,
  groupSearchItems,
  itemSearchRowKey,
  itemSearchSlot,
  itemSearchTaxonomy,
  itemSearchTypes,
} from '~/utils/itemSearch'
import type { FoundItem } from '~/utils/itemSearch'
import type { ContainerDialogTarget } from '~/composables/useContainerDialog'
const { serverScope } = useServerScope()
const { liveStale, connectionState } = useLiveData()
const { items, loading, error, reload } = useItemSearch(serverScope)
const dialogTarget = shallowRef<ContainerDialogTarget | null>(null)
let dialogOpener: HTMLElement | null = null
const search = ref('')
const itemType = ref('')
const subcategory = ref('')
const degree = ref('')
const includeSets = ref(false)
const stale = computed(
  () => liveStale.value || connectionState.value === 'stale',
)
const filters = computed(() => ({
  query: search.value,
  type: itemType.value,
  subcategory: subcategory.value,
  degree: degree.value,
  includeSets: includeSets.value,
}))
const matches = computed(() => filterSearchItems(items.value, filters.value))
const groups = computed(() => groupSearchItems(matches.value))
const categoryItems = computed(() =>
  items.value.filter(
    (row) =>
      (!itemType.value || itemSearchTaxonomy(row).type === itemType.value) &&
      (includeSets.value || row.container !== 'equipment'),
  ),
)
const subcategories = computed(() =>
  [
    ...new Set(
      categoryItems.value
        .map((row) => itemSearchTaxonomy(row).subcategory)
        .filter(Boolean),
    ),
  ].sort(),
)
const degrees = computed(() =>
  [
    ...new Set(
      categoryItems.value
        .filter(
          (row) =>
            !subcategory.value ||
            itemSearchTaxonomy(row).subcategory === subcategory.value,
        )
        .map((row) => itemSearchTaxonomy(row).degree)
        .filter(Boolean),
    ),
  ].sort((a, b) => Number(a) - Number(b)),
)
watch(itemType, () => {
  subcategory.value = ''
  degree.value = ''
})
watch(subcategory, () => {
  degree.value = ''
})
watch(serverScope, resetFilters)
function resetFilters() {
  search.value = ''
  itemType.value = ''
  subcategory.value = ''
  degree.value = ''
  includeSets.value = false
}
function dialogTargetFor(row: FoundItem): ContainerDialogTarget | null {
  if (row.container === 'guild_storage') {
    const guild = row.guild || row.owner_id
    return guild ? { kind: 'guild', server: row.server, guild } : null
  }
  if (row.container === 'inventory' || row.container === 'storage')
    return {
      kind: 'character',
      server: row.server,
      characterID: row.observer_character_id,
      characterName: row.observer_name,
      container: row.container,
    }
  return null
}
function openDialog(row: FoundItem, event: MouseEvent) {
  const next = dialogTargetFor(row)
  if (!next) return
  dialogOpener =
    event.currentTarget instanceof HTMLElement ? event.currentTarget : null
  dialogTarget.value = next
}
function closeDialog() {
  dialogTarget.value = null
  const opener = dialogOpener
  dialogOpener = null
  void nextTick(() => opener?.focus())
}
watch(serverScope, () => {
  dialogTarget.value = null
})
function sourceLabel(container: string) {
  return (
    (
      {
        inventory: 'Open Char Inventory',
        storage: 'Open Char Storage',
        equipment: 'Open Char Set',
        guild_storage: 'Open Guild Storage',
        pets: 'Open Pet Inventory',
        job_pouch: 'Open Job Pouch',
      } as Record<string, string>
    )[container] || 'Open Container'
  )
}
</script>

<template>
  <div class="items-page">
    <PageHeader title="Item Search" icon="i-lucide-search" description="" />
    <section
      class="panel item-search-panel"
      aria-label="Item search results"
      :aria-busy="loading"
    >
      <div class="item-search-summary">
        <button
          class="compact-button item-search-refresh"
          type="button"
          aria-label="Refresh items"
          :disabled="loading"
          @click="reload()"
        >
          <UIcon name="i-lucide-refresh-cw" />
        </button>
        <span
          v-if="!loading"
          class="item-search-count"
          aria-label="Total matching items"
          >{{ matches.length }}</span
        >
      </div>
      <form
        class="item-search-filters"
        aria-label="Item search filters"
        @submit.prevent
      >
        <label class="item-search-text"
          ><span>Search</span
          ><input
            v-model="search"
            maxlength="128"
            aria-label="Search item name, code or model"
            type="search"
        /></label>
        <label
          ><span>Item Type</span
          ><select v-model="itemType" aria-label="Item Type">
            <option value="">Any type</option>
            <option
              v-for="[value, label] in itemSearchTypes"
              :key="value"
              :value="value"
            >
              {{ label }}
            </option>
          </select></label
        >
        <label
          ><span>Subcategory</span
          ><select
            v-model="subcategory"
            aria-label="Subcategory"
            :disabled="!itemType || !subcategories.length"
          >
            <option value="">Subcategory</option>
            <option v-for="value in subcategories" :key="value" :value="value">
              {{ value }}
            </option>
          </select></label
        >
        <label
          ><span>Degree</span
          ><select
            v-model="degree"
            aria-label="Degree"
            :disabled="!itemType || !degrees.length"
          >
            <option value="">Degree</option>
            <option v-for="value in degrees" :key="value" :value="value">
              {{ value }}
            </option>
          </select></label
        >
        <label class="item-search-sets"
          ><input
            v-model="includeSets"
            type="checkbox"
            aria-label="Include char sets"
          /><span>Include char sets</span></label
        >
        <button
          class="compact-button item-search-reset"
          type="button"
          @click="resetFilters"
        >
          Reset Filters
        </button>
      </form>
      <p v-if="loading" class="item-search-status" role="status">
        Loading items…
      </p>
      <p v-if="error" class="status-banner warning" role="alert">
        {{ error }}
        <button type="button" class="compact-button" @click="reload()">
          Retry
        </button>
      </p>
      <p v-if="stale" class="status-banner warning" role="status">
        Disconnected · showing last synced items.
      </p>
      <div
        v-if="!loading && !error && !matches.length"
        class="event-empty-state"
      >
        <strong>No matching items</strong>
      </div>
      <article
        v-for="group in groups"
        :key="group.key"
        class="item-search-owner"
        :aria-label="`${group.name} items`"
      >
        <div class="item-search-owner-heading">
          <h2>{{ group.name }}</h2>
          <span class="item-search-count">{{ group.count }} matches</span
          ><span v-if="serverScope === 'all'" class="item-search-server">{{
            group.server
          }}</span>
        </div>
        <p class="item-search-meta">Owner: {{ group.name }}</p>
        <section
          v-for="container in group.containers"
          :key="container.key"
          class="item-search-container"
          :aria-label="`${container.name} | ${group.name}`"
        >
          <div class="item-search-container-heading">
            <div>
              <div class="item-search-owner-heading">
                <h3>{{ container.name }} | {{ group.name }}</h3>
                <span class="item-search-count"
                  >{{ container.items.length }} matches</span
                >
              </div>
              <p class="item-search-meta">
                Owner: {{ group.name }} | Viewed by:
                {{ container.row.observer_name }} | Last synced:
                {{ formatSyncedTime(container.row.observed_at) }}
              </p>
            </div>
            <button
              v-if="dialogTargetFor(container.row)"
              class="compact-button item-search-open"
              type="button"
              aria-haspopup="dialog"
              @click="openDialog(container.row, $event)"
            >
              {{ sourceLabel(container.row.container) }}
            </button>
            <NuxtLink
              v-else
              class="compact-button item-search-open"
              :to="container.source"
              >{{ sourceLabel(container.row.container) }}</NuxtLink
            >
          </div>
          <div class="item-search-grid">
            <ItemDetailPopup
              v-for="row in container.items"
              :key="itemSearchRowKey(row)"
              :item="row.item"
              :quantity-exact="row.quantity_exact"
              layout="tile"
              :subtitle="itemSearchSlot(row)"
            />
          </div>
        </section>
      </article>
    </section>
    <InventoryDialog
      v-if="dialogTarget"
      :target="dialogTarget"
      @close="closeDialog"
    />
  </div>
</template>
