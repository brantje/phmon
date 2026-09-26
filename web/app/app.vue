<script setup lang="ts">
import QrcodeVue from 'qrcode.vue'
import type { AgentCredential } from '~~/shared/types/agent'
import type { CharacterView as Character } from '~~/shared/types/live'
import type { Health } from '~~/shared/types/health'

const mode = useCookie<'easy' | 'advanced'>('phmon-mode', {
  default: () => 'easy',
  sameSite: 'lax',
})
const advancedMode = computed({
  get: () => mode.value === 'advanced',
  set: (value: boolean) => {
    mode.value = value ? 'advanced' : 'easy'
  },
})

const sidebarCollapsed = useCookie<boolean>('phmon-sidebar-collapsed', {
  default: () => false,
  sameSite: 'lax',
})
const mobileNavigationOpen = ref(false)
const mobileAccessOpen = ref(false)
const credentialPanelOpen = ref(false)
const credentialCreating = ref(false)
const createdCredential = ref<AgentCredential | null>(null)
const credentialError = ref('')
const credentialCopied = ref<'agent_id' | 'agent_token' | null>(null)
const credentialCopyFallback = ref<'agent_id' | 'agent_token' | null>(null)
const copied = ref(false)
const copyFallbackNeeded = ref(false)
const now = ref<number | null>(null)
const mobileAccessTrigger = ref<HTMLButtonElement | null>(null)
const accessDialog = ref<HTMLElement | null>(null)
const accessCloseButton = ref<HTMLButtonElement | null>(null)
const runtimeConfig = useRuntimeConfig()
const requestURL = useRequestURL()
const route = useRoute()

const {
  agents: liveAgents,
  characters: liveCharacters,
  fleetCharacters: liveFleetCharacters,
  groups: liveGroups,
  characterDetail: detailCharacter,
  connectionState: liveConnectionState,
  liveStale,
  setCharacterListFilter,
  setCharacterDetail,
  refreshLiveData,
} = useLiveData()

const characterDetailID = computed(() => {
  const match = route.path.match(/^\/characters\/([0-9a-f-]{36})\/?$/i)
  return match?.[1] || ''
})
const isStatsPage = computed(() => route.path.replace(/\/$/, '') === '/stats')
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

watch(
  [debouncedCharacterSearch, selectedGroup, manageGroupMembers],
  ([query, groupID, managing]) => {
    setCharacterListFilter(
      query,
      groupID && !managing ? String(groupID) : undefined,
    )
  },
  { immediate: true },
)
watch(
  characterDetailID,
  (characterID) => {
    setCharacterDetail(characterID)
  },
  { immediate: true },
)

const lastCharacters = liveCharacters
const lastFleetCharacters = liveFleetCharacters
const lastGroups = liveGroups
const lastAgents = liveAgents
const characterError = computed(
  () => liveStale.value || liveConnectionState.value === 'stale',
)
const agentsUnavailable = computed(
  () => liveStale.value || liveConnectionState.value === 'stale',
)
const agentsStatus = computed(() =>
  liveConnectionState.value !== 'current' && lastAgents.value.length === 0
    ? 'pending'
    : 'success',
)
const charactersLoading = computed(
  () =>
    liveConnectionState.value !== 'current' &&
    !liveStale.value &&
    lastCharacters.value.length === 0,
)
const detailLoading = computed(
  () => liveConnectionState.value !== 'current' && !liveStale.value,
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
const onlineCharacterCount = computed(
  () =>
    lastFleetCharacters.value.filter((character) => character.online).length,
)
const offlineCharacterCount = computed(
  () =>
    lastFleetCharacters.value.filter((character) => !character.online).length,
)
const characterCountsUnknown = computed(
  () =>
    liveConnectionState.value !== 'current' &&
    lastFleetCharacters.value.length === 0,
)
function displayCharacterCount(value: number) {
  return characterCountsUnknown.value ? '—' : String(value)
}
const combinedVitals = computed(() => {
  const withHP = lastFleetCharacters.value.filter(
    (character) =>
      character.hp != null && character.hp_max != null && character.hp_max > 0,
  )
  const withMP = lastFleetCharacters.value.filter(
    (character) =>
      character.mp != null && character.mp_max != null && character.mp_max > 0,
  )
  const ratio = (
    items: Character[],
    current: 'hp' | 'mp',
    max: 'hp_max' | 'mp_max',
  ) =>
    items.length
      ? `${(
          (items.reduce(
            (sum, character) => sum + (character[current] || 0),
            0,
          ) /
            items.reduce(
              (sum, character) => sum + (character[max] || 0),
              0,
            )) *
          100
        ).toFixed(1)}%`
      : '—'
  return {
    hp: ratio(withHP, 'hp', 'hp_max'),
    mp: ratio(withMP, 'mp', 'mp_max'),
  }
})
const observedGold = computed(() => {
  const values = lastFleetCharacters.value
    .map((character) => character.gold)
    .filter((value): value is number => value != null)
  return values.length
    ? new Intl.NumberFormat('en', {
        notation: 'compact',
        maximumFractionDigits: 1,
      }).format(values.reduce((sum, value) => sum + value, 0))
    : '—'
})

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

const configuredInstanceUrl = normalizeInstanceUrl(
  String(runtimeConfig.public.instanceUrl || ''),
)
const instanceUrl = ref(
  configuredInstanceUrl ||
    normalizeInstanceUrl(requestURL.origin) ||
    requestURL.origin,
)
const instanceUrlIsLoopback = computed(() => isLoopbackUrl(instanceUrl.value))

const {
  data: health,
  error: healthError,
  refresh: refreshHealth,
} = await useFetch<Health>('/api/health', { retry: 0 })

const backendReady = computed(
  () => !healthError.value && health.value?.status === 'ok',
)
const connectedAgents = computed<number | null>(() =>
  agentsUnavailable.value
    ? null
    : lastAgents.value.filter((agent) => agent.connected).length,
)
const fleetStatus = computed(() => {
  if (liveStale.value) return 'Live data stale'
  if (liveConnectionState.value !== 'current') return 'Connecting live data'
  if (lastAgents.value.length === 0) return 'Waiting for agents'
  return (connectedAgents.value ?? 0) > 0 ? 'Agents connected' : 'Fleet offline'
})

let healthTimer: ReturnType<typeof setInterval> | undefined
let clockTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  if (!configuredInstanceUrl) {
    instanceUrl.value = window.location.origin
  }
  now.value = Date.now()
  healthTimer = setInterval(() => void refreshHealth(), 10000)
  clockTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (healthTimer) clearInterval(healthTimer)
  if (clockTimer) clearInterval(clockTimer)
  if (characterSearchTimer) clearTimeout(characterSearchTimer)
})

const primaryNavigation = [
  { label: 'Dashboard', icon: 'i-lucide-layout-dashboard', href: '/' },
  { label: 'Stats', icon: 'i-lucide-chart-no-axes-combined', href: '/stats' },
  { label: 'Events', icon: 'i-lucide-activity' },
  { label: 'Chat', icon: 'i-lucide-messages-square' },
  { label: 'Economy', icon: 'i-lucide-coins' },
  { label: 'Alchemy', icon: 'i-lucide-flask-conical' },
  { label: 'Academy', icon: 'i-lucide-graduation-cap' },
  { label: 'Guild Storage', icon: 'i-lucide-warehouse' },
  { label: 'phBot', icon: 'i-lucide-bot' },
]

const advancedNavigation = [
  { label: 'Analytics', icon: 'i-lucide-chart-no-axes-column-increasing' },
  { label: 'Map', icon: 'i-lucide-map' },
  { label: 'Item Search', icon: 'i-lucide-search' },
  { label: 'Skill Builder', icon: 'i-lucide-git-branch' },
  { label: 'Automations', icon: 'i-lucide-zap' },
]

function formatTimestamp(value?: string) {
  if (!value) return 'Never'
  const date = new Date(value)
  if (Number.isNaN(date.getTime())) return 'Unknown'
  return date.toISOString().replace('T', ' ').replace('.000Z', 'Z')
}

function formatConnectionAge(value?: string) {
  if (!value || now.value === null) return 'Connected'
  const elapsed = Math.max(
    0,
    Math.floor((now.value - new Date(value).getTime()) / 1000),
  )
  if (elapsed < 60) return String(elapsed) + 's'
  if (elapsed < 3600) {
    return String(Math.floor(elapsed / 60)) + 'm ' + String(elapsed % 60) + 's'
  }
  const hours = Math.floor(elapsed / 3600)
  return String(hours) + 'h ' + String(Math.floor((elapsed % 3600) / 60)) + 'm'
}

function formatHealthMana(character: Character) {
  return `HP ${character.hp?.toLocaleString() ?? '—'} / ${character.hp_max?.toLocaleString() ?? '—'} · MP ${character.mp?.toLocaleString() ?? '—'} / ${character.mp_max?.toLocaleString() ?? '—'}`
}

function formatProgress(character: Character) {
  const xp =
    character.current_exp == null
      ? '—'
      : `${character.current_exp.toLocaleString()} / ${character.max_exp?.toLocaleString() ?? '—'} XP`
  const ratio =
    character.current_exp != null &&
    character.max_exp != null &&
    character.max_exp > 0
      ? ` (${Math.min(100, Math.round((character.current_exp / character.max_exp) * 100))}%)`
      : ''
  return `${xp}${ratio} · ${character.sp?.toLocaleString() ?? '—'} SP`
}

function normalizeInstanceUrl(value: string) {
  const candidate = value.trim()
  if (!candidate) return ''
  try {
    const url = new URL(candidate)
    if (url.username || url.password) return ''
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return ''
    return url.origin
  } catch {
    return ''
  }
}

function isLoopbackUrl(value: string) {
  try {
    const hostname = new URL(value).hostname.toLowerCase()
    return (
      hostname === 'localhost' ||
      hostname.endsWith('.localhost') ||
      hostname === '127.0.0.1' ||
      hostname === '[::1]'
    )
  } catch {
    return true
  }
}

async function copyInstanceUrl() {
  if (!import.meta.client) return
  copyFallbackNeeded.value = false
  try {
    await navigator.clipboard.writeText(instanceUrl.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1800)
  } catch {
    copied.value = false
    copyFallbackNeeded.value = true
  }
}

function toggleCredentialPanel() {
  if (credentialPanelOpen.value) {
    dismissCredential()
    return
  }
  credentialError.value = ''
  credentialCopied.value = null
  credentialCopyFallback.value = null
  credentialPanelOpen.value = true
}

function dismissCredential() {
  credentialPanelOpen.value = false
  createdCredential.value = null
  credentialError.value = ''
  credentialCopied.value = null
  credentialCopyFallback.value = null
}

async function createAgentCredential() {
  credentialCreating.value = true
  credentialError.value = ''
  createdCredential.value = null
  credentialCopied.value = null
  credentialCopyFallback.value = null
  try {
    createdCredential.value = await $fetch<AgentCredential>(
      '/api/agents/credentials',
      {
        method: 'POST',
        body: {},
        retry: 0,
      },
    )
  } catch {
    credentialError.value =
      'Could not create a credential. Check backend/database readiness and try again.'
  } finally {
    credentialCreating.value = false
  }
}

async function copyCredential(field: 'agent_id' | 'agent_token') {
  if (!import.meta.client || !createdCredential.value) return
  credentialCopied.value = null
  credentialCopyFallback.value = null
  const value = createdCredential.value[field]
  try {
    await navigator.clipboard.writeText(value)
    credentialCopied.value = field
    window.setTimeout(() => {
      if (credentialCopied.value === field) credentialCopied.value = null
    }, 1800)
  } catch {
    credentialCopyFallback.value = field
  }
}

async function openMobileAccess() {
  copyFallbackNeeded.value = false
  mobileAccessOpen.value = true
  await nextTick()
  accessCloseButton.value?.focus()
}

async function closeMobileAccess() {
  mobileAccessOpen.value = false
  await nextTick()
  mobileAccessTrigger.value?.focus()
}

function handleAccessDialogKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    void closeMobileAccess()
    return
  }
  if (event.key !== 'Tab' || !accessDialog.value) return

  const focusable = Array.from(
    accessDialog.value.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  )
  if (focusable.length === 0) {
    event.preventDefault()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
</script>

<template>
  <UApp>
    <div class="phmon-shell" :class="{ 'is-collapsed': sidebarCollapsed }">
      <button
        v-if="mobileNavigationOpen"
        class="mobile-backdrop"
        type="button"
        aria-label="Close navigation"
        @click="mobileNavigationOpen = false"
      />

      <aside
        class="app-sidebar"
        :class="{ 'is-mobile-open': mobileNavigationOpen }"
      >
        <div class="brand-block">
          <div class="brand-mark" aria-hidden="true">P</div>
          <div class="brand-copy">
            <strong>PhMon</strong>
            <span>self-hosted · slice 2</span>
            <span
              class="brand-connect"
              :class="
                !agentsUnavailable && (connectedAgents ?? 0) > 0
                  ? 'is-online'
                  : 'is-offline'
              "
              >{{ fleetStatus }}</span
            >
          </div>
          <span
            class="connection-dot"
            :class="
              !agentsUnavailable && (connectedAgents ?? 0) > 0
                ? 'is-online'
                : 'is-offline'
            "
            :title="fleetStatus"
          />
        </div>

        <div class="scope-block">
          <span>Server scope · LATER</span>
          <button type="button" disabled>
            <UIcon name="i-lucide-layers-3" />
            <span>All</span>
            <span class="nav-soon">LATER</span>
            <UIcon name="i-lucide-chevron-down" />
          </button>
        </div>

        <nav class="navigation" aria-label="Primary navigation">
          <template v-for="item in primaryNavigation" :key="item.label">
            <NuxtLink
              v-if="item.href"
              :to="item.href"
              class="nav-item"
              :class="{ active: (isStatsPage ? '/stats' : '/') === item.href }"
              :aria-current="
                (isStatsPage ? '/stats' : '/') === item.href
                  ? 'page'
                  : undefined
              "
            >
              <UIcon :name="item.icon" />
              <span>{{ item.label }}</span>
            </NuxtLink>
            <button
              v-else
              type="button"
              class="nav-item"
              disabled
              :title="sidebarCollapsed ? item.label : undefined"
            >
              <UIcon :name="item.icon" />
              <span>{{ item.label }}</span>
              <span class="nav-soon">later</span>
            </button>
          </template>

          <template v-if="advancedMode">
            <p class="nav-heading">Tools</p>
            <button
              v-for="item in advancedNavigation"
              :key="item.label"
              type="button"
              class="nav-item"
              disabled
              :title="sidebarCollapsed ? item.label : undefined"
            >
              <UIcon :name="item.icon" />
              <span>{{ item.label }}</span>
              <span class="nav-soon">later</span>
            </button>
          </template>

          <p class="nav-heading">{{ advancedMode ? 'Misc' : 'System' }}</p>
          <button
            class="nav-item"
            type="button"
            disabled
            title="Settings arrive progressively"
          >
            <UIcon name="i-lucide-settings" />
            <span>Settings</span>
            <span class="nav-soon">later</span>
          </button>
          <button
            v-if="advancedMode"
            class="nav-item"
            type="button"
            disabled
            title="Server List arrives later"
          >
            <UIcon name="i-lucide-server" />
            <span>Server List</span>
            <span class="nav-soon">later</span>
          </button>
        </nav>

        <div class="sidebar-footer">
          <button
            ref="mobileAccessTrigger"
            class="instance-button"
            type="button"
            :title="sidebarCollapsed ? 'Mobile access' : undefined"
            @click="openMobileAccess"
          >
            <UIcon name="i-lucide-qr-code" />
            <span>Mobile access</span>
          </button>
          <div class="sidebar-qr" aria-label="PhMon instance QR code">
            <QrcodeVue
              :value="instanceUrl"
              :size="78"
              level="M"
              render-as="svg"
            />
          </div>
          <div class="instance-url" :title="instanceUrl">
            {{ instanceUrl }}
          </div>
          <button class="sidebar-copy" type="button" @click="copyInstanceUrl">
            {{ copied ? 'Copied' : 'Copy link' }}
          </button>
        </div>
      </aside>

      <div class="workspace">
        <div class="top-strip">
          <div class="top-left">
            <button
              class="icon-button mobile-menu"
              type="button"
              aria-label="Open navigation"
              @click="mobileNavigationOpen = true"
            >
              <UIcon name="i-lucide-menu" />
            </button>
            <button
              class="icon-button collapse-button"
              type="button"
              :aria-label="
                sidebarCollapsed ? 'Expand sidebar' : 'Collapse sidebar'
              "
              @click="sidebarCollapsed = !sidebarCollapsed"
            >
              <UIcon
                :name="
                  sidebarCollapsed
                    ? 'i-lucide-panel-left-open'
                    : 'i-lucide-panel-left-close'
                "
              />
            </button>
            <div class="top-summary" aria-label="Live character summary">
              <span class="top-summary-item"
                ><UIcon name="i-lucide-users-round" /> Chars:
                <strong
                  >{{
                    displayCharacterCount(onlineCharacterCount)
                  }}
                  online</strong
                ><i>|</i
                ><span
                  >{{
                    displayCharacterCount(offlineCharacterCount)
                  }}
                  offline</span
                ></span
              >
              <span class="top-summary-item"
                ><UIcon name="i-lucide-heart-pulse" /> Combined stats:
                <strong>HP {{ combinedVitals.hp }}</strong
                ><i>|</i><strong>MP {{ combinedVitals.mp }}</strong></span
              >
              <span class="top-summary-item"
                ><UIcon name="i-lucide-coins" /> Total Gold:
                <strong>{{ observedGold }}</strong></span
              >
            </div>
            <span class="sr-only" role="status">{{
              backendReady ? 'Backend ready' : 'Backend unavailable'
            }}</span>
          </div>

          <label class="mode-toggle">
            <span>Easy</span>
            <input
              v-model="advancedMode"
              type="checkbox"
              aria-label="Toggle advanced mode"
            />
            <span>Advanced</span>
          </label>
        </div>

        <main class="workspace-content">
          <template v-if="!characterDetailID">
            <header class="page-header">
              <div class="page-icon">
                <UIcon
                  :name="
                    isStatsPage
                      ? 'i-lucide-chart-no-axes-combined'
                      : 'i-lucide-layout-dashboard'
                  "
                />
              </div>
              <div>
                <h1>{{ isStatsPage ? 'Stats' : 'Dashboard' }}</h1>
                <p>
                  {{
                    isStatsPage
                      ? 'Live character state and saved character groups.'
                      : 'Live overview of your characters, recent activity and server.'
                  }}
                </p>
              </div>
            </header>

            <section
              v-if="!isStatsPage"
              class="dashboard-grid"
              aria-label="Dashboard overview"
            >
              <article class="panel dashboard-characters">
                <div class="panel-header compact">
                  <div>
                    <h2>Characters</h2>
                    <p>Current fleet presence and observed gold</p>
                  </div>
                  <a class="panel-link" href="/stats#characters"
                    >Show character stats <UIcon name="i-lucide-arrow-up-right"
                  /></a>
                </div>
                <div class="dashboard-counts">
                  <div class="dashboard-count online-count">
                    <span>Online</span
                    ><strong>{{
                      displayCharacterCount(onlineCharacterCount)
                    }}</strong>
                  </div>
                  <div class="dashboard-count">
                    <span>Offline</span
                    ><strong>{{
                      displayCharacterCount(offlineCharacterCount)
                    }}</strong>
                  </div>
                  <div class="dashboard-count later-count">
                    <span>Alive</span><strong>LATER</strong>
                  </div>
                  <div class="dashboard-count later-count">
                    <span>Dead</span><strong>LATER</strong>
                  </div>
                </div>
                <div class="dashboard-gold">
                  <UIcon name="i-lucide-coins" /><span>Total Gold</span
                  ><strong>{{ observedGold }}</strong>
                </div>
              </article>

              <article class="panel dashboard-later dashboard-deaths">
                <div class="panel-header compact">
                  <div>
                    <h2>Last Deaths</h2>
                    <p>Recent character deaths</p>
                  </div>
                  <span class="later-badge">LATER</span>
                </div>
                <div class="later-content">
                  <UIcon name="i-lucide-skull" /><strong>LATER</strong
                  ><span>Event history arrives in a later slice.</span>
                </div>
              </article>

              <article class="panel dashboard-later dashboard-server">
                <div class="panel-header compact">
                  <div>
                    <h2>Server Information</h2>
                    <p>Operator managed</p>
                  </div>
                  <span class="later-badge">LATER</span>
                </div>
                <div class="later-content">
                  <UIcon name="i-lucide-server" /><strong>LATER</strong
                  ><span
                    >Server artwork and metadata arrive in a later slice.</span
                  >
                </div>
              </article>

              <article class="panel dashboard-later dashboard-events">
                <div class="panel-header compact">
                  <div>
                    <h2>Recent Events</h2>
                    <p>Latest activity across your characters</p>
                  </div>
                  <span class="later-badge">LATER</span>
                </div>
                <div class="later-content">
                  <UIcon name="i-lucide-clock-3" /><strong>LATER</strong
                  ><span>Timeline data arrives in a later slice.</span>
                </div>
              </article>

              <div class="dashboard-stack">
                <article class="panel dashboard-later">
                  <div class="panel-header compact">
                    <div><h2>Last Rare Drop</h2></div>
                    <span class="later-badge">LATER</span>
                  </div>
                  <div class="later-content compact-later">
                    <UIcon name="i-lucide-gem" /><strong>LATER</strong>
                  </div>
                </article>
                <article class="panel dashboard-later">
                  <div class="panel-header compact">
                    <div><h2>Chat Messages</h2></div>
                    <span class="later-badge">LATER</span>
                  </div>
                  <div class="later-content compact-later">
                    <UIcon name="i-lucide-messages-square" /><strong
                      >LATER</strong
                    >
                  </div>
                </article>
              </div>

              <article class="panel dashboard-later dashboard-offers">
                <div class="panel-header compact">
                  <div>
                    <h2>Global Offers</h2>
                    <p>Recent buy, sell and trade offers</p>
                  </div>
                  <span class="later-badge">LATER</span>
                </div>
                <div class="later-content">
                  <UIcon name="i-lucide-store" /><strong>LATER</strong
                  ><span>Economy data arrives in a later slice.</span>
                </div>
              </article>
            </section>

            <section id="characters" class="panel character-panel">
              <div class="panel-header">
                <div>
                  <h2>Characters</h2>
                  <p>
                    Identity is scoped by game server. State refreshes
                    automatically.
                  </p>
                </div>
                <div class="panel-actions character-filters">
                  <input
                    v-model="characterSearch"
                    maxlength="100"
                    aria-label="Search characters, guild, server or zone"
                    placeholder="Search characters, guild, server, zone"
                  />
                  <select
                    v-model="selectedGroup"
                    aria-label="Filter by character group"
                  >
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
                    {{
                      manageGroupMembers ? 'Filter members' : 'Manage members'
                    }}
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
              <div
                v-if="groupActionError"
                class="status-banner warning"
                role="alert"
              >
                {{ groupActionError }}
              </div>
              <div
                v-if="characterError"
                class="status-banner warning"
                role="status"
              >
                <UIcon name="i-lucide-triangle-alert" /> Character service
                unavailable. Showing the last received records as stale.
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
                        <a
                          class="character-link"
                          :href="'/characters/' + character.character_id"
                          >{{ character.name }}</a
                        ><small v-if="character.guild">{{
                          character.guild
                        }}</small>
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
                          character.gold == null
                            ? '—'
                            : character.gold.toLocaleString()
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
                            character.last_activity_at ||
                              character.state_updated_at,
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
                                  member.character_id ===
                                  character.character_id,
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

            <section class="panel agent-panel">
              <div class="panel-header">
                <div>
                  <h2>phBot agents</h2>
                  <p>
                    Live connection state, plugin version and phBot version.
                    Credentials are never exposed here.
                  </p>
                </div>
                <div class="panel-actions">
                  <button
                    class="compact-button"
                    type="button"
                    @click="toggleCredentialPanel"
                  >
                    <UIcon name="i-lucide-key-round" />
                    {{
                      credentialPanelOpen
                        ? 'Close credential'
                        : 'Create credential'
                    }}
                  </button>
                  <button
                    class="compact-button"
                    type="button"
                    :disabled="agentsStatus === 'pending'"
                    @click="refreshLiveData(['agents'])"
                  >
                    <UIcon
                      name="i-lucide-refresh-cw"
                      :class="{ spinning: agentsStatus === 'pending' }"
                    />
                    Refresh
                  </button>
                </div>
              </div>

              <div
                v-if="agentsUnavailable"
                class="status-banner warning"
                role="status"
              >
                <UIcon name="i-lucide-triangle-alert" />
                <div>
                  <strong>Agent service unavailable</strong>
                  <span v-if="lastAgents.length">
                    Showing the last successfully loaded agent list.
                  </span>
                  <span v-else>PhMon will retry automatically.</span>
                </div>
              </div>

              <section
                v-if="credentialPanelOpen"
                class="credential-panel"
                aria-label="Create agent credential"
              >
                <div class="credential-panel-head">
                  <div>
                    <strong>Create agent credential</strong>
                    <p>
                      An agent ID/token identifies one logical PhMon agent.
                      Reuse it across phBot connections that belong together, or
                      create separate credentials for separate agents. The token
                      can only be recovered from this response.
                    </p>
                  </div>
                </div>

                <template v-if="createdCredential">
                  <div class="credential-row">
                    <span>Agent ID</span>
                    <code tabindex="0">{{ createdCredential.agent_id }}</code>
                    <button
                      class="compact-button"
                      type="button"
                      @click="copyCredential('agent_id')"
                    >
                      <UIcon
                        :name="
                          credentialCopied === 'agent_id'
                            ? 'i-lucide-check'
                            : 'i-lucide-copy'
                        "
                      />
                      {{
                        credentialCopied === 'agent_id' ? 'Copied' : 'Copy ID'
                      }}
                    </button>
                  </div>
                  <div class="credential-row">
                    <span>Agent token</span>
                    <code tabindex="0">{{
                      createdCredential.agent_token
                    }}</code>
                    <button
                      class="compact-button"
                      type="button"
                      @click="copyCredential('agent_token')"
                    >
                      <UIcon
                        :name="
                          credentialCopied === 'agent_token'
                            ? 'i-lucide-check'
                            : 'i-lucide-copy'
                        "
                      />
                      {{
                        credentialCopied === 'agent_token'
                          ? 'Copied'
                          : 'Copy token'
                      }}
                    </button>
                  </div>
                  <p
                    v-if="credentialCopyFallback"
                    class="credential-message warning"
                    role="status"
                  >
                    Clipboard access is unavailable. Select the value above and
                    copy it manually.
                  </p>
                  <p class="credential-message" role="status">
                    Save this token in each phBot PhMon profile that should use
                    this logical agent now. PostgreSQL stores only its SHA-256
                    hash, so PhMon cannot show this token again.
                  </p>
                  <div class="credential-actions">
                    <button
                      class="compact-button"
                      type="button"
                      @click="dismissCredential"
                    >
                      Done
                    </button>
                  </div>
                </template>

                <template v-else>
                  <p class="credential-risk">
                    PhMon user authentication is not implemented yet. Until it
                    is, anyone who can access this web UI can create an agent
                    credential. Keep this instance on a trusted network.
                  </p>
                  <p
                    v-if="credentialError"
                    class="credential-message warning"
                    role="alert"
                  >
                    {{ credentialError }}
                  </p>
                  <div class="credential-actions">
                    <button
                      class="compact-button"
                      type="button"
                      :disabled="credentialCreating"
                      @click="createAgentCredential"
                    >
                      <UIcon
                        :name="
                          credentialCreating
                            ? 'i-lucide-loader-circle'
                            : 'i-lucide-key-round'
                        "
                        :class="{ spinning: credentialCreating }"
                      />
                      {{
                        credentialCreating
                          ? 'Creating…'
                          : 'Generate one-time credential'
                      }}
                    </button>
                  </div>
                </template>
              </section>

              <div
                v-if="agentsStatus === 'pending' && lastAgents.length === 0"
                class="empty-state"
              >
                <UIcon name="i-lucide-loader-circle" class="spinning" />
                <strong>Loading agents…</strong>
              </div>

              <div v-else-if="lastAgents.length === 0" class="empty-state">
                <UIcon name="i-lucide-plug-zap" />
                <strong>No agents have connected yet</strong>
                <p>
                  Create a credential above (or use
                  <code>phmonctl agent create</code>), then configure the
                  matching profile in phBot's PhMon plugin tab.
                </p>
              </div>

              <div v-else class="agent-table-wrap">
                <table class="agent-table">
                  <thead>
                    <tr>
                      <th>Status</th>
                      <th>Agent</th>
                      <th>Plugin</th>
                      <th>phBot</th>
                      <th>Protocol</th>
                      <th>Connected</th>
                      <th>Last seen</th>
                    </tr>
                  </thead>
                  <tbody>
                    <tr v-for="agent in lastAgents" :key="agent.agent_id">
                      <td>
                        <span
                          class="status-chip"
                          :class="
                            agentsUnavailable
                              ? 'stale'
                              : agent.connected
                                ? 'online'
                                : 'offline'
                          "
                        >
                          <span />{{
                            agentsUnavailable
                              ? agent.connected
                                ? 'Last known online'
                                : 'Last known offline'
                              : agent.connected
                                ? 'Online'
                                : 'Offline'
                          }}
                        </span>
                      </td>
                      <td>
                        <code class="agent-id">{{ agent.agent_id }}</code>
                      </td>
                      <td>{{ agent.plugin_version || '—' }}</td>
                      <td>{{ agent.phbot_version || '—' }}</td>
                      <td>
                        {{
                          agent.protocol_version
                            ? 'v' + agent.protocol_version
                            : '—'
                        }}
                      </td>
                      <td>
                        {{
                          agentsUnavailable
                            ? '—'
                            : agent.connected
                              ? formatConnectionAge(agent.connected_at)
                              : '—'
                        }}
                      </td>
                      <td>
                        <time :datetime="agent.last_seen_at">
                          {{ formatTimestamp(agent.last_seen_at) }}
                        </time>
                      </td>
                    </tr>
                  </tbody>
                </table>

                <div class="agent-cards">
                  <article
                    v-for="agent in lastAgents"
                    :key="'mobile-' + agent.agent_id"
                    class="agent-card"
                  >
                    <div class="agent-card-head">
                      <span
                        class="status-chip"
                        :class="
                          agentsUnavailable
                            ? 'stale'
                            : agent.connected
                              ? 'online'
                              : 'offline'
                        "
                      >
                        <span />{{
                          agentsUnavailable
                            ? agent.connected
                              ? 'Last known online'
                              : 'Last known offline'
                            : agent.connected
                              ? 'Online'
                              : 'Offline'
                        }}
                      </span>
                      <span>
                        {{
                          agentsUnavailable
                            ? formatTimestamp(agent.last_seen_at)
                            : agent.connected
                              ? formatConnectionAge(agent.connected_at)
                              : formatTimestamp(agent.last_seen_at)
                        }}
                      </span>
                    </div>
                    <code>{{ agent.agent_id }}</code>
                    <dl>
                      <div>
                        <dt>Plugin</dt>
                        <dd>{{ agent.plugin_version || '—' }}</dd>
                      </div>
                      <div>
                        <dt>phBot</dt>
                        <dd>{{ agent.phbot_version || '—' }}</dd>
                      </div>
                      <div>
                        <dt>Protocol</dt>
                        <dd>
                          {{
                            agent.protocol_version
                              ? 'v' + agent.protocol_version
                              : '—'
                          }}
                        </dd>
                      </div>
                    </dl>
                  </article>
                </div>
              </div>
            </section>

            <section class="lower-grid">
              <article class="panel operations-panel">
                <div class="panel-header compact">
                  <div>
                    <h2>Operations</h2>
                    <p>
                      Slice 0 health stays available as an operational
                      diagnostic.
                    </p>
                  </div>
                </div>
                <dl class="operation-list">
                  <div>
                    <dt>API / database</dt>
                    <dd :class="backendReady ? 'text-ok' : 'text-warning'">
                      {{ backendReady ? 'Ready' : 'Unavailable' }}
                    </dd>
                  </div>
                  <div>
                    <dt>Agent protocol</dt>
                    <dd>v2</dd>
                  </div>
                  <div>
                    <dt>Polling</dt>
                    <dd>3 seconds</dd>
                  </div>
                </dl>
              </article>

              <article class="panel next-panel">
                <div class="panel-header compact">
                  <div>
                    <h2>Presence model</h2>
                    <p>
                      Agent connection and joined character sessions are tracked
                      separately.
                    </p>
                  </div>
                </div>
                <dl class="operation-list">
                  <div>
                    <dt>Character identity</dt>
                    <dd>Server + character name</dd>
                  </div>
                  <div>
                    <dt>Session recovery</dt>
                    <dd>Full snapshot on reconnect</dd>
                  </div>
                  <div>
                    <dt>Training state</dt>
                    <dd>Unknown until API getter is verified</dd>
                  </div>
                </dl>
              </article>
            </section>
          </template>
          <section v-else class="character-detail-view">
            <header class="page-header">
              <div class="page-icon"><UIcon name="i-lucide-user-round" /></div>
              <div>
                <h1>{{ detailCharacter?.name || 'Character detail' }}</h1>
                <p>
                  {{ detailCharacter?.server || 'Loading identity' }} · stable
                  character record
                </p>
              </div>
              <a class="compact-button" href="/">Back to overview</a>
            </header>
            <div
              v-if="liveStale"
              class="status-banner warning"
              role="status"
            >
              <UIcon name="i-lucide-triangle-alert" />
              Live character data is stale. PhMon is retrying the WebSocket
              connection; HTTP fallback is disabled.
            </div>
            <div v-if="detailCharacter" class="detail-grid">
              <article class="panel detail-identity">
                <div class="panel-header compact">
                  <div>
                    <h2>Current status</h2>
                    <p>{{ detailCharacter.guild || 'Guild unknown' }}</p>
                  </div>
                  <span
                    class="status-chip"
                    :class="detailCharacter.online ? 'online' : 'offline'"
                    ><span />{{
                      detailCharacter.online ? 'Online' : 'Offline'
                    }}</span
                  >
                </div>
                <dl class="operation-list">
                  <div>
                    <dt>Serving agent</dt>
                    <dd>{{ detailCharacter.agent_id || 'None' }}</dd>
                  </div>
                  <div>
                    <dt>Level</dt>
                    <dd>{{ detailCharacter.level ?? '—' }}</dd>
                  </div>
                  <div>
                    <dt>HP / MP</dt>
                    <dd>
                      {{ formatHealthMana(detailCharacter) }}
                    </dd>
                  </div>
                  <div>
                    <dt>XP / SP</dt>
                    <dd>
                      {{ formatProgress(detailCharacter) }}
                    </dd>
                  </div>
                  <div>
                    <dt>Gold</dt>
                    <dd>{{ detailCharacter.gold?.toLocaleString() ?? '—' }}</dd>
                  </div>
                  <div>
                    <dt>Location</dt>
                    <dd>
                      {{ detailCharacter.zone || 'Unknown zone' }} ·
                      {{ detailCharacter.x ?? '—' }},
                      {{ detailCharacter.y ?? '—' }},
                      {{ detailCharacter.z ?? '—' }} (region
                      {{ detailCharacter.region ?? '—' }})
                    </dd>
                  </div>
                  <div>
                    <dt>Training state</dt>
                    <dd>
                      {{
                        detailCharacter.botting == null
                          ? 'Not reported by documented phBot API'
                          : detailCharacter.botting
                            ? 'Training'
                            : 'Idle'
                      }}
                    </dd>
                  </div>
                  <div>
                    <dt>Session started</dt>
                    <dd>
                      {{ formatTimestamp(detailCharacter.session_started_at) }}
                    </dd>
                  </div>
                  <div>
                    <dt>Last activity</dt>
                    <dd>
                      {{ formatTimestamp(detailCharacter.last_activity_at) }}
                    </dd>
                  </div>
                </dl>
              </article>
              <article class="panel detail-future">
                <div class="panel-header compact">
                  <div>
                    <h2>Character tools</h2>
                    <p>These panels will use this stable character ID.</p>
                  </div>
                </div>
                <div class="detail-links">
                  <span>Inventory / equipment · Slice 4</span
                  ><span>Pets and party · Slice 4</span
                  ><span>Map position · Slice 7</span
                  ><span>Verified actions · Slice 3</span>
                </div>
              </article>
            </div>
            <div v-else class="panel empty-state">
              <strong>{{
                detailLoading
                  ? 'Loading live character'
                  : liveStale
                    ? 'Last character snapshot unavailable'
                    : 'Character unavailable'
              }}</strong>
              <p>
                {{
                  detailLoading
                    ? 'Waiting for the initial WebSocket detail snapshot.'
                    : liveStale
                      ? 'The WebSocket will retry and resynchronize without an HTTP fallback.'
                      : 'The character ID is not present in the current live snapshot.'
                }}
              </p>
            </div>
          </section>
        </main>
      </div>

      <div
        v-if="mobileAccessOpen"
        class="dialog-layer"
        role="presentation"
        @click.self="closeMobileAccess"
      >
        <section
          ref="accessDialog"
          class="access-dialog"
          role="dialog"
          aria-modal="true"
          aria-labelledby="mobile-access-title"
          @keydown="handleAccessDialogKeydown"
        >
          <div class="dialog-header">
            <div>
              <h2 id="mobile-access-title">Open PhMon on another device</h2>
              <p>
                The QR code contains only this instance URL. Agent credentials
                are never included.
              </p>
            </div>
            <button
              ref="accessCloseButton"
              class="icon-button"
              type="button"
              aria-label="Close"
              @click="closeMobileAccess"
            >
              <UIcon name="i-lucide-x" />
            </button>
          </div>
          <div
            v-if="instanceUrlIsLoopback"
            class="access-warning"
            role="status"
          >
            This URL points back to the device opening PhMon and cannot be used
            from another device. Set NUXT_PUBLIC_INSTANCE_URL to a reachable
            HTTPS or LAN URL.
          </div>
          <div v-else class="qr-frame">
            <QrcodeVue
              id="phmon-instance-qr"
              :value="instanceUrl"
              :size="184"
              level="M"
              render-as="svg"
            />
          </div>
          <code class="dialog-url" tabindex="0">{{ instanceUrl }}</code>
          <p v-if="copyFallbackNeeded" class="copy-fallback" role="status">
            Clipboard access is unavailable here. Select the URL above and copy
            it manually.
          </p>
          <button
            class="compact-button dialog-copy"
            type="button"
            @click="copyInstanceUrl"
          >
            <UIcon :name="copied ? 'i-lucide-check' : 'i-lucide-copy'" />
            {{ copied ? 'Copied' : 'Copy instance URL' }}
          </button>
        </section>
      </div>
    </div>
  </UApp>
</template>
