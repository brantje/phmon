<script setup lang="ts">
import { useId } from 'vue'
import type { CharacterView, PetInventoryRecord } from '~~/shared/types/live'
import { characterDeathState } from '../utils/characterDeath'

const props = defineProps<{
  character: CharacterView
  stale?: boolean
  groupName?: string
}>()
const {
  characterResources,
  setCharacterResources,
  clearCharacterResources,
  connectionState,
  freshnessNow,
} = useLiveData()
const deathState = computed(() =>
  characterDeathState(props.character, false, freshnessNow.value),
)
const selectedTab = ref('Overview')
const selectedContainer = ref('storage')
const cardElement = ref<HTMLElement | null>(null)
const cardVisible = ref(false)
const resourceSubscriptionID = `resources-${useId()
  .replace(/[^a-zA-Z0-9_-]/g, '')
  .slice(-24)}`
const resources = computed(
  () => characterResources.value[props.character.character_id],
)
const resourceMap = computed(() => resources.value?.resources || {})
const observation = (key: string) => resourceMap.value[key]
const payload = (key: string) => observation(key)?.payload || {}
const containerObservation = (key: string) => ({
  ...payload(key),
  availability: observation(key)?.availability || 'unavailable',
  observed_at: observation(key)?.observed_at,
  checked_at: observation(key)?.checked_at,
})
const bag = computed(() => payload('inventory'))
const bagContainer = computed(() => containerObservation('inventory'))
const bagAvailability = computed(() => observation('inventory')?.availability)
const equipment = computed(() => payload('equipment'))
const equipmentObservation = computed(() => observation('equipment'))
const equipmentAvailability = computed(
  () => observation('equipment')?.availability,
)
const freshness = computed(() => {
  const values = Object.values(resourceMap.value)
    .map((entry) => entry.observed_at)
    .filter((value): value is string => !!value)
  if (!values.length) return 'Waiting for resource baseline'
  return `Observed ${formatTimestamp(values.sort().at(-1))}`
})
const pets = computed(() => {
  const list = payload('pets').pets
  return Array.isArray(list) ? (list as PetInventoryRecord[]) : []
})
const partyMembers = computed(() => {
  const list = payload('party').members
  return Array.isArray(list) ? (list as Record<string, unknown>[]) : []
})
const partySetup = computed(() => payload('party_setup'))
const academy = computed(() => {
  const value = payload('academy').value
  return value && typeof value === 'object' && !Array.isArray(value)
    ? (value as Record<string, unknown>)
    : null
})
const academyMembers = computed(() => {
  const members = academy.value?.members
  if (!Array.isArray(members)) return []
  return members.flatMap((value) => {
    if (!value || typeof value !== 'object' || Array.isArray(value)) return []
    const data = value as Record<string, unknown>
    return [{ id: String(data.member_id || 'unknown'), data }]
  })
})
const academyHasMembers = computed(() => academyMembers.value.length > 0)
const activeResourceKeys = computed(() => {
  if (selectedTab.value === 'Overview') return ['pets', 'party']
  if (selectedTab.value === 'Info') return ['equipment']
  if (selectedTab.value === 'Inventory') return ['inventory']
  if (selectedTab.value === 'Storage') return [selectedContainer.value]
  if (selectedTab.value === 'Pet') return ['pets', 'item_enrichment']
  if (selectedTab.value === 'Party') return ['party', 'party_setup']
  if (selectedTab.value === 'Academy') return ['academy']
  return []
})

function available(key: string) {
  return observation(key)?.availability || 'unavailable'
}
function itemSlots(value: unknown) {
  return Array.isArray(value) ? value : []
}
function petLabel(pet: PetInventoryRecord) {
  const type = typeof pet.type === 'string' ? pet.type : 'unknown'
  const labels: Record<string, string> = {
    wolf: 'Attack',
    fellow: 'Fellow',
    pick: 'Pick',
    transport: 'Transport',
  }
  return (
    labels[type.toLowerCase()] ||
    (type === 'horse' ? 'Horse' : `Other · ${type}`)
  )
}
function petDetailStatus(pet: PetInventoryRecord) {
  if (pet.inventory_available !== true || !Array.isArray(pet.slots))
    return 'No pet inventory was supplied in this observation.'
  const items = (pet.slots as unknown[]).flatMap((row) => {
    if (!row || typeof row !== 'object' || !('item' in row)) return []
    const item = (row as { item?: unknown }).item
    return item && typeof item === 'object'
      ? [item as Record<string, unknown>]
      : []
  })
  if (items.length === 0)
    return 'No items currently occupy this supplied inventory.'
  const detailed = items.filter((item) => {
    const details = item.instance_details
    return (
      !!details &&
      typeof details === 'object' &&
      (details as Record<string, unknown>).status === 'partial'
    )
  }).length
  if (detailed)
    return `${detailed} item${detailed === 1 ? '' : 's'} include interpreted instance details.`
  const raw = items.some((item) => item.api_fields || item.instance)
  return raw
    ? 'Instance evidence is present, but its detail interpretation is still pending.'
    : 'Current slots show basic item facts; detailed attributes await verified API or packet evidence.'
}
const petPacketProbe = computed(() => {
  const value = payload('item_enrichment').pet_packet_probe
  if (!value || typeof value !== 'object' || Array.isArray(value)) return null
  const probe = value as Record<string, unknown>
  if (probe.mode !== 'presence_and_size_only') return null
  const opcodes = probe.opcodes
  if (!opcodes || typeof opcodes !== 'object' || Array.isArray(opcodes))
    return null
  return opcodes as Record<string, unknown>
})
function petPacketProbeSummary(opcode: string) {
  const value = petPacketProbe.value?.[opcode]
  if (!value || typeof value !== 'object' || Array.isArray(value))
    return `${opcode}: waiting`
  const detail = value as Record<string, unknown>
  const count = typeof detail.count === 'number' ? detail.count : 0
  const minimum = typeof detail.min_bytes === 'number' ? detail.min_bytes : null
  const maximum = typeof detail.max_bytes === 'number' ? detail.max_bytes : null
  const sizeRange = minimum === null ? '' : ` · ${minimum}–${maximum} bytes`
  return `${opcode}: ${count} packet${count === 1 ? '' : 's'}${sizeRange}`
}
function memberNumber(member: Record<string, unknown>, key: string) {
  const value = member[key]
  return typeof value === 'number' && Number.isFinite(value) ? value : null
}
let resourceObserver: IntersectionObserver | undefined
watch(
  [cardVisible, activeResourceKeys, () => props.character.character_id],
  ([visible, keys, characterID], [wasVisible, , previousCharacterID]) => {
    if (previousCharacterID && previousCharacterID !== characterID) {
      clearCharacterResources(previousCharacterID, resourceSubscriptionID)
    }
    if (visible && keys.length) {
      setCharacterResources(characterID, keys, resourceSubscriptionID)
    } else if (wasVisible || previousCharacterID !== characterID) {
      clearCharacterResources(characterID, resourceSubscriptionID)
    }
  },
  { deep: true },
)
onMounted(() => {
  if ('IntersectionObserver' in window && cardElement.value) {
    resourceObserver = new IntersectionObserver(
      (entries) => {
        cardVisible.value = entries.some((entry) => entry.isIntersecting)
      },
      { rootMargin: '160px' },
    )
    resourceObserver.observe(cardElement.value)
  } else {
    cardVisible.value = true
  }
})
onBeforeUnmount(() => {
  resourceObserver?.disconnect()
  clearCharacterResources(props.character.character_id, resourceSubscriptionID)
})
</script>

<template>
  <article
    ref="cardElement"
    class="character-card"
    :class="{ 'is-stale': stale }"
  >
    <header class="character-card-header">
      <CharacterPortrait
        :name="character.name"
        :portrait-url="character.portrait_url"
        size="card"
        :to="`/characters/${character.character_id}`"
      />
      <div class="character-card-identity">
        <NuxtLink
          class="character-card-name"
          :to="`/characters/${character.character_id}`"
          >{{ character.name }}</NuxtLink
        >
        <strong>Lv. {{ character.level ?? '—' }}</strong>
        <span>{{ character.guild || 'Guild unknown' }}</span>
      </div>
      <div class="character-presence">
        <span class="death-chip" :class="`death-${deathState}`">
          {{
            deathState === 'unknown'
              ? 'Unknown'
              : deathState === 'dead'
                ? 'Dead'
                : 'Alive'
          }}
        </span>
        <span
          class="status-chip"
          :class="stale ? 'stale' : character.online ? 'online' : 'offline'"
          ><span />{{
            stale ? 'Stale' : character.online ? 'Online' : 'Offline'
          }}</span
        >
        <span class="character-server">{{ character.server }}</span>
      </div>
    </header>

    <nav class="character-card-tabs" aria-label="Character information tabs">
      <button
        v-for="tab in [
          'Overview',
          'Info',
          'Progress',
          'Inventory',
          'Storage',
          'Pet',
          'Party',
          'Academy',
          'Actions',
        ]"
        :key="tab"
        type="button"
        :class="{ active: selectedTab === tab }"
        @click="selectedTab = tab"
      >
        {{ tab }}
      </button>
    </nav>

    <div
      v-if="stale || connectionState === 'stale'"
      class="character-resource-stale"
      role="status"
    >
      Showing the last received resource observations as stale.
    </div>
    <div v-if="selectedTab === 'Overview'" class="character-overview">
      <div class="character-overview-main">
        <dl class="character-stats">
          <div>
            <dt>HP</dt>
            <dd>
              {{ character.hp?.toLocaleString() ?? '—' }} /
              {{ character.hp_max?.toLocaleString() ?? '—' }}
            </dd>
          </div>
          <div>
            <dt>MP</dt>
            <dd>
              {{ character.mp?.toLocaleString() ?? '—' }} /
              {{ character.mp_max?.toLocaleString() ?? '—' }}
            </dd>
          </div>
          <div>
            <dt>XP</dt>
            <dd>{{ formatProgress(character) }}</dd>
          </div>
          <div>
            <dt>SP</dt>
            <dd>{{ character.sp?.toLocaleString() ?? '—' }}</dd>
          </div>
          <div>
            <dt>Gold</dt>
            <dd>{{ character.gold?.toLocaleString() ?? '—' }}</dd>
          </div>
          <div>
            <dt>Location</dt>
            <dd>{{ character.zone || 'Unknown zone' }}</dd>
          </div>
          <div>
            <dt>Training</dt>
            <dd>
              {{
                character.botting == null
                  ? 'Unknown'
                  : character.botting
                    ? 'Training'
                    : 'Idle'
              }}
            </dd>
          </div>
          <div>
            <dt>Resource freshness</dt>
            <dd>{{ freshness }}</dd>
          </div>
        </dl>
        <MapPreview
          class="character-card-preview"
          :title="`${character.name} map`"
          :server="character.server"
          :members="[character]"
          character-card
        />
      </div>
      <div class="character-mini-summary">
        <span
          >{{ pets.length }} summoned
          {{ pets.length === 1 ? 'pet' : 'pets' }}</span
        >
        <span>{{ partyMembers.length }} party members reported</span>
      </div>
    </div>

    <div v-else-if="selectedTab === 'Info'" class="character-card-content">
      <section class="equipment-view">
        <div class="inventory-topline">
          <h3>Character Set</h3>
          <span
            >{{ Number(equipment.used_slots || 0) }}/{{
              Number(equipment.capacity || 0)
            }}
            equipped</span
          >
        </div>
        <div
          class="inventory-pages"
          role="group"
          aria-label="Character set pages"
        >
          <button class="compact-button selected" type="button" disabled>
            Page 1
          </button>
        </div>
        <div v-if="Array.isArray(equipment.slots)" class="equipment-layout">
          <div
            v-if="equipmentAvailability !== 'observed'"
            class="inventory-stale"
            role="status"
          >
            Showing the last confirmed equipment observation from
            {{
              equipmentObservation?.observed_at
                ? formatTimestamp(equipmentObservation.observed_at)
                : 'an unknown time'
            }}.
          </div>
          <div class="equipment-center">
            <CharacterPortrait
              :name="character.name"
              :portrait-url="character.portrait_url"
              size="large"
            />
            <strong>{{ character.name }}</strong
            ><span>{{
              character.level == null
                ? 'Level unavailable'
                : `Level ${character.level}`
            }}</span>
          </div>
          <div class="equipment-slots">
            <div
              v-for="(slot, index) in itemSlots(equipment.slots)"
              :key="index"
              class="equipment-slot"
            >
              <ItemSlot
                :item="
                  slot && typeof slot === 'object' && 'item' in slot
                    ? (slot.item as Record<string, unknown>)
                    : null
                "
                :slot-number="index"
                :label="`Equipment source slot ${index + 1}`"
              />
              <small>Slot {{ index + 1 }}</small>
            </div>
          </div>
        </div>
        <div v-else class="inventory-unavailable">
          <strong>{{
            equipmentAvailability === 'not_observed'
              ? 'Not observed yet'
              : 'Unavailable'
          }}</strong
          ><span
            >Equipment data is unavailable until phBot reports the inventory
            slots.</span
          >
        </div>
        <p class="mapping-note">
          phBot documents a flat item list and its capacity, but not which
          entries are equipment. The current first-13 split comes from an
          unverified adapter lead; equipment labels and remaining bag capacity
          may be wrong. Mapping evidence:
          {{
            String(
              equipment.mapping_evidence || 'adapter_lead_runtime_unverified',
            )
          }}.
        </p>
      </section>
    </div>

    <div v-else-if="selectedTab === 'Progress'" class="character-card-content">
      <div class="inventory-topline"><h3>Progress</h3></div>
      <dl class="character-stats">
        <div>
          <dt>Level</dt>
          <dd>{{ character.level ?? '—' }}</dd>
        </div>
        <div>
          <dt>Current XP</dt>
          <dd>{{ formatProgress(character) }}</dd>
        </div>
        <div>
          <dt>Current SP</dt>
          <dd>{{ character.sp?.toLocaleString() ?? '—' }}</dd>
        </div>
        <div>
          <dt>Current gold</dt>
          <dd>{{ character.gold?.toLocaleString() ?? '—' }}</dd>
        </div>
      </dl>
      <div class="inventory-unavailable progress-history-note" role="status">
        <strong>Historical metrics unavailable</strong>
        <span
          >XP/hour, SP/hour, gold/hour and drop rates appear when historical
          tracking is available.</span
        >
      </div>
    </div>

    <div v-else-if="selectedTab === 'Inventory'" class="character-card-content">
      <InventoryGrid
        title="Inventory"
        :observation="bagContainer"
        :availability="bagAvailability"
        :gold="typeof bag.gold === 'number' ? bag.gold : null"
      />
    </div>

    <div
      v-else-if="selectedTab === 'Storage'"
      class="character-card-content storage-tabs"
    >
      <div
        class="container-switcher"
        role="group"
        aria-label="Storage container"
      >
        <button
          v-for="key in ['storage', 'guild_storage', 'job_pouch']"
          :key="key"
          class="compact-button"
          :class="{ selected: selectedContainer === key }"
          type="button"
          @click="selectedContainer = key"
        >
          {{
            key === 'guild_storage'
              ? 'Guild'
              : key === 'job_pouch'
                ? 'Job pouch'
                : 'Personal'
          }}
        </button>
      </div>
      <InventoryGrid
        :title="
          selectedContainer === 'guild_storage'
            ? 'Guild storage'
            : selectedContainer === 'job_pouch'
              ? 'Job pouch'
              : 'Personal storage'
        "
        :observation="containerObservation(selectedContainer)"
        :availability="available(selectedContainer)"
      />
    </div>

    <div
      v-else-if="selectedTab === 'Pet'"
      class="character-card-content pet-list"
    >
      <div
        v-if="available('pets') !== 'observed'"
        :class="
          Array.isArray(payload('pets').pets)
            ? 'inventory-stale'
            : 'inventory-unavailable'
        "
      >
        <strong>{{
          Array.isArray(payload('pets').pets)
            ? 'Last reported pet state'
            : 'Pet state unavailable'
        }}</strong
        ><span>{{
          Array.isArray(payload('pets').pets) &&
          observation('pets')?.observed_at
            ? `Last contents observed ${formatTimestamp(observation('pets')?.observed_at)}`
            : 'phBot has not supplied a current pet observation.'
        }}</span>
      </div>
      <div v-if="petPacketProbe" class="pet-packet-probe" role="status">
        <strong>Passive packet probe · V14</strong>
        <span>{{ petPacketProbeSummary('0x30C8') }}</span>
        <span>{{ petPacketProbeSummary('0xB034') }}</span>
        <small
          >This build reports packet counts and sizes only. It does not decode
          packet contents or change pet items.</small
        >
      </div>
      <article v-for="pet in pets" :key="String(pet.pet_id)" class="pet-card">
        <div class="pet-card-heading">
          <strong>{{
            String(pet.name || pet.servername || pet.pet_id || 'Unknown pet')
          }}</strong
          ><span>{{ petLabel(pet) }}</span
          ><span v-if="pet.mounted === true">Mounted</span>
        </div>
        <p>
          Owner: {{ character.name }} · Pet ID:
          {{ String(pet.pet_id || 'unknown') }} ·
          {{
            pet.inventory_available === true && Array.isArray(pet.slots)
              ? `${pet.slots.length} inventory slots from get_pets()`
              : 'inventory not supplied'
          }}
        </p>
        <p>
          Reported type: {{ String(pet.type || 'unknown') }} · HP
          {{ pet.hp == null ? '—' : String(pet.hp) }}
        </p>
        <p class="pet-detail-status" role="status">
          {{ petDetailStatus(pet) }}
        </p>
        <InventoryGrid
          v-if="pet.inventory_available === true && Array.isArray(pet.slots)"
          title="Pet inventory"
          :observation="{
            availability: available('pets'),
            observed_at: observation('pets')?.observed_at,
            slots: pet.slots,
            capacity: pet.slots.length,
            used_slots: pet.slots.filter(Boolean).length,
          }"
          :availability="available('pets')"
        />
        <div v-else class="inventory-unavailable">
          <strong>Pet inventory not supplied</strong
          ><span
            >The current API observation includes no inventory slots for this
            pet.</span
          >
        </div>
      </article>
      <div
        v-if="available('pets') === 'observed' && pets.length === 0"
        class="inventory-empty"
      >
        No summoned pets reported.
      </div>
    </div>

    <div
      v-else-if="selectedTab === 'Party'"
      class="character-card-content party-view"
    >
      <div
        v-if="available('party') !== 'observed'"
        :class="
          Array.isArray(payload('party').members)
            ? 'inventory-stale'
            : 'inventory-unavailable'
        "
      >
        <strong>{{
          Array.isArray(payload('party').members)
            ? 'Last reported party state'
            : 'Party state unavailable'
        }}</strong
        ><span>{{
          Array.isArray(payload('party').members) &&
          observation('party')?.observed_at
            ? `Last contents observed ${formatTimestamp(observation('party')?.observed_at)}`
            : 'Party membership and Party Setup are separate observations.'
        }}</span>
      </div>
      <section
        class="party-membership"
        aria-labelledby="party-membership-title"
      >
        <h3 id="party-membership-title">Current party membership</h3>
        <div
          v-if="available('party') === 'observed' && partyMembers.length === 0"
          class="inventory-empty"
        >
          No party members reported.
        </div>
        <div
          v-for="member in partyMembers"
          :key="String(member.party_id)"
          class="party-member"
        >
          <div>
            <strong>{{ String(member.name || 'Unknown member') }}</strong
            ><small
              >{{ String(member.guild || 'No guild') }} · Lv.
              {{ String(member.level ?? '—') }}</small
            >
          </div>
          <span>HP {{ memberNumber(member, 'hp_percent') ?? '—' }}%</span
          ><span>MP {{ memberNumber(member, 'mp_percent') ?? '—' }}%</span>
        </div>
      </section>
      <section class="party-setup-panel" aria-labelledby="party-setup-title">
        <div class="party-setup-heading">
          <div>
            <h3 id="party-setup-title">Party Setup</h3>
            <span class="status-chip stale">Configuration unavailable</span>
          </div>
          <button
            class="compact-button"
            type="button"
            disabled
            aria-describedby="party-setup-blocker"
          >
            Edit settings
          </button>
        </div>
        <p id="party-setup-blocker">
          No supported Party Setup fields or phBot write API are documented.
          Direct edits to the active JSON file are explicitly documented as
          overwrite-prone; profile reload and effective-state readback are
          unverified. Editing stays disabled until those behaviors are verified
          on the installed phBot version.
        </p>
        <small
          >Observed mode:
          {{ String(partySetup.mode || 'read_only_unverified') }} ·
          {{
            String(
              partySetup.reason || 'configuration_reload_contract_not_verified',
            )
          }}</small
        >
      </section>
    </div>

    <div
      v-else-if="selectedTab === 'Academy'"
      class="character-card-content academy-view"
    >
      <div
        v-if="available('academy') !== 'observed'"
        :class="academy ? 'inventory-stale' : 'inventory-unavailable'"
      >
        <strong>{{
          academy ? 'Last reported academy state' : 'Academy state unavailable'
        }}</strong
        ><span>{{
          academy && observation('academy')?.observed_at
            ? `Last contents observed ${formatTimestamp(observation('academy')?.observed_at)}`
            : 'Waiting for the phBot academy getter.'
        }}</span>
      </div>
      <template v-else-if="academyHasMembers">
        <div class="academy-summary">
          <span>Academy ID</span
          ><strong>{{ String(academy?.id ?? 'Unknown') }}</strong>
        </div>
        <div
          v-for="entry in academyMembers"
          :key="entry.id"
          class="party-member"
        >
          <div>
            <strong>{{ String(entry.data.name || entry.id) }}</strong
            ><small
              >Type {{ String(entry.data.type ?? '—') }} · Lv.
              {{ String(entry.data.level ?? '—') }}</small
            >
          </div>
          <span>{{ Number(entry.data.online) ? 'Online' : 'Offline' }}</span>
        </div>
      </template>
      <div v-else class="inventory-empty">
        No current academy membership reported.
      </div>
      <p class="mapping-note">
        Historical academy activity remains outside Slice 4.
      </p>
    </div>

    <div v-else class="character-card-content action-view">
      <RemoteCommandActions :character="character" />
    </div>
  </article>
</template>
