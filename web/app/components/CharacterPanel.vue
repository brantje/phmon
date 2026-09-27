<script setup lang="ts">
import type { CharacterView as Character } from '~~/shared/types/live'
const {
  characters: lastCharacters,
  groups: lastGroups,
  connectionState: liveConnectionState,
  liveStale,
  refreshLiveData,
  setCharacterListFilter,
  clearCharacterListFilter,
} = useLiveData()
const characterSearch = ref('')
const debouncedCharacterSearch = ref('')
let characterSearchTimer: ReturnType<typeof setTimeout> | undefined
watch(characterSearch, (value) => {
  if (characterSearchTimer) clearTimeout(characterSearchTimer)
  characterSearchTimer = setTimeout(() => {
    debouncedCharacterSearch.value = value.trim()
  }, 250)
})

const selectedGroup = ref('')
const manageGroupMembers = ref(false)
const groupName = ref('')
const groupActionError = ref('')

// Nuxt mounts the incoming page after disposing the outgoing page. Install the
// subscription here so the old panel's cleanup cannot clear the new filter.
let stopFilterWatch: (() => void) | undefined
onMounted(() => {
  stopFilterWatch = watch(
    [debouncedCharacterSearch, selectedGroup, manageGroupMembers],
    ([query, groupID, managing]) => {
      setCharacterListFilter(
        query,
        groupID && !managing ? String(groupID) : undefined,
      )
    },
    { immediate: true },
  )
})
onBeforeUnmount(() => {
  stopFilterWatch?.()
  clearCharacterListFilter()
})
const characterError = computed(
  () => liveStale.value || liveConnectionState.value === 'stale',
)
const charactersLoading = computed(
  () =>
    liveConnectionState.value !== 'current' &&
    !liveStale.value &&
    lastCharacters.value.length === 0,
)
const visibleCharacters = computed(() =>
  lastCharacters.value.filter((character) => {
    const groupMatches =
      !selectedGroup.value ||
      manageGroupMembers.value ||
      lastGroups.value
        .find((group) => group.group_id === selectedGroup.value)
        ?.members.some(
          (member) => member.character_id === character.character_id,
        )
    return groupMatches
  }),
)
async function createCharacterGroup() {
  groupActionError.value = ''
  try {
    await $fetch('/api/groups', {
      method: 'POST',
      body: { name: groupName.value },
    })
    groupName.value = ''
  } catch {
    groupActionError.value = 'Could not create this group.'
  }
}
async function renameCharacterGroup() {
  const group = lastGroups.value.find(
    (item) => item.group_id === selectedGroup.value,
  )
  if (!group) return
  const name = groupName.value.trim()
  if (!name) return
  groupActionError.value = ''
  try {
    await $fetch('/api/groups/' + group.group_id, {
      method: 'PATCH',
      body: { name },
    })
    groupName.value = ''
  } catch {
    groupActionError.value = 'Could not rename this group.'
  }
}
async function deleteCharacterGroup() {
  const group = lastGroups.value.find(
    (item) => item.group_id === selectedGroup.value,
  )
  if (
    !group ||
    !window.confirm(
      'Delete group "' + group.name + '"? Characters will be kept.',
    )
  )
    return
  groupActionError.value = ''
  try {
    await $fetch('/api/groups/' + group.group_id, { method: 'DELETE' })
    selectedGroup.value = ''
  } catch {
    groupActionError.value = 'Could not delete this group.'
  }
}
async function toggleGroupMember(character: Character) {
  const group = lastGroups.value.find(
    (item) => item.group_id === selectedGroup.value,
  )
  if (!group) return
  const isMember = group.members.some(
    (item) => item.character_id === character.character_id,
  )
  groupActionError.value = ''
  try {
    await $fetch(
      '/api/groups/' + group.group_id + '/members/' + character.character_id,
      { method: isMember ? 'DELETE' : 'PUT' },
    )
  } catch {
    groupActionError.value = 'Could not update group membership.'
  }
}
function refreshCharacterData() {
  refreshLiveData(['character-list', 'fleet-characters', 'groups'])
}

onUnmounted(() => {
  if (characterSearchTimer) clearTimeout(characterSearchTimer)
})
</script>

<template>
  <section id="characters" class="panel character-panel">
    <div class="panel-header">
      <div>
        <h2>Characters</h2>
        <p>Identity is scoped by game server. State refreshes automatically.</p>
      </div>
      <div class="panel-actions character-filters">
        <input
          v-model="characterSearch"
          maxlength="100"
          aria-label="Search characters, guild, server or zone"
          placeholder="Search characters, guild, server, zone"
        />
        <select v-model="selectedGroup" aria-label="Filter by character group">
          <option value="">All groups</option>
          <option
            v-for="group in lastGroups"
            :key="group.group_id"
            :value="group.group_id"
          >
            {{ group.name }}
          </option>
        </select>
        <button
          v-if="selectedGroup"
          class="compact-button"
          type="button"
          @click="manageGroupMembers = !manageGroupMembers"
        >
          {{ manageGroupMembers ? 'Filter members' : 'Manage members' }}
        </button>
        <input
          v-model="groupName"
          aria-label="Character group name"
          placeholder="Group name"
        />
        <button
          class="compact-button"
          type="button"
          @click="createCharacterGroup"
        >
          New group
        </button>
        <button
          v-if="selectedGroup"
          class="compact-button"
          type="button"
          @click="renameCharacterGroup"
        >
          Rename
        </button>
        <button
          v-if="selectedGroup"
          class="compact-button"
          type="button"
          @click="deleteCharacterGroup"
        >
          Delete
        </button>
        <button
          class="compact-button"
          type="button"
          @click="refreshCharacterData"
        >
          Refresh
        </button>
      </div>
    </div>
    <div v-if="groupActionError" class="status-banner warning" role="alert">
      {{ groupActionError }}
    </div>
    <div v-if="characterError" class="status-banner warning" role="status">
      <UIcon name="i-lucide-triangle-alert" /> Character service unavailable.
      Showing the last received records as stale.
    </div>
    <div v-if="visibleCharacters.length" class="agent-table-wrap">
      <table class="agent-table character-table">
        <thead>
          <tr>
            <th>Character</th>
            <th>Presence</th>
            <th>Level</th>
            <th>HP / MP</th>
            <th>Progress</th>
            <th>Gold</th>
            <th>Server · Zone</th>
            <th>Training</th>
            <th>Freshness</th>
            <th>Group</th>
          </tr>
        </thead>
        <tbody>
          <tr
            v-for="character in visibleCharacters"
            :key="character.character_id"
          >
            <td>
              <NuxtLink
                class="character-link"
                :to="'/characters/' + character.character_id"
                >{{ character.name }}</NuxtLink
              ><small v-if="character.guild">{{ character.guild }}</small>
            </td>
            <td>
              <span
                class="status-chip"
                :class="
                  characterError
                    ? 'stale'
                    : character.online
                      ? 'online'
                      : 'offline'
                "
                ><span />{{
                  characterError
                    ? 'Stale'
                    : character.online
                      ? 'Online'
                      : 'Offline'
                }}</span
              >
            </td>
            <td>{{ character.level ?? '—' }}</td>
            <td>{{ formatHealthMana(character) }}</td>
            <td>{{ formatProgress(character) }}</td>
            <td>
              {{
                character.gold == null ? '—' : character.gold.toLocaleString()
              }}
            </td>
            <td>
              {{ character.server
              }}<small>{{ character.zone || 'Zone unknown' }}</small>
            </td>
            <td>
              {{
                character.botting == null
                  ? 'Unknown'
                  : character.botting
                    ? 'Training'
                    : 'Idle'
              }}
            </td>
            <td>
              {{
                formatTimestamp(
                  character.last_activity_at || character.state_updated_at,
                )
              }}
            </td>
            <td>
              <button
                v-if="selectedGroup && manageGroupMembers"
                class="compact-button"
                type="button"
                @click="toggleGroupMember(character)"
              >
                {{
                  lastGroups
                    .find((group) => group.group_id === selectedGroup)
                    ?.members.some(
                      (member) =>
                        member.character_id === character.character_id,
                    )
                    ? 'Remove'
                    : 'Add'
                }}</button
              ><span v-else>—</span>
            </td>
          </tr>
        </tbody>
      </table>
    </div>
    <div v-else class="empty-state character-empty">
      <UIcon
        :name="
          characterError
            ? 'i-lucide-cloud-off'
            : charactersLoading
              ? 'i-lucide-loader-circle'
              : 'i-lucide-user-round-search'
        "
      />
      <strong>{{
        characterError
          ? 'Character data stale'
          : charactersLoading
            ? 'Loading live characters'
            : 'No characters observed yet'
      }}</strong>
      <p>
        {{
          characterError
            ? 'The last received records remain visible and will be replaced after WebSocket recovery.'
            : charactersLoading
              ? 'Waiting for the initial WebSocket snapshot.'
              : 'Join a character in phBot. PhMon will register its server-scoped identity automatically.'
        }}
      </p>
    </div>
  </section>
</template>
