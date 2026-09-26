<script setup lang="ts">
import QrcodeVue from 'qrcode.vue'
import type {
  AgentCredential,
  AgentListResponse,
  AgentView,
} from '../shared/types/agent'
import type { Health } from '../shared/types/health'

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
  data: agentResponse,
  status: agentsStatus,
  error: agentsError,
  refresh: refreshAgents,
} = await useFetch<AgentListResponse>('/api/agents', {
  retry: 0,
})
const lastAgents = ref<AgentView[]>(
  agentResponse.value?.status === 'ok' ? agentResponse.value.agents : [],
)
watch(agentResponse, (value) => {
  if (value?.status === 'ok') lastAgents.value = value.agents
})

const {
  data: health,
  status: healthStatus,
  error: healthError,
  refresh: refreshHealth,
} = await useFetch<Health>('/api/health', { retry: 0 })

const agentsUnavailable = computed(
  () =>
    Boolean(agentsError.value) || agentResponse.value?.status === 'unavailable',
)
const backendReady = computed(
  () => !healthError.value && health.value?.status === 'ok',
)
const connectedAgents = computed<number | null>(() =>
  agentsUnavailable.value
    ? null
    : lastAgents.value.filter((agent) => agent.connected).length,
)
const disconnectedAgents = computed<number | null>(() =>
  agentsUnavailable.value || connectedAgents.value === null
    ? null
    : lastAgents.value.length - connectedAgents.value,
)
const fleetStatus = computed(() => {
  if (agentsUnavailable.value) return 'Backend unavailable'
  if (lastAgents.value.length === 0) return 'Waiting for agents'
  return (connectedAgents.value ?? 0) > 0 ? 'Agents connected' : 'Fleet offline'
})

let agentTimer: ReturnType<typeof setInterval> | undefined
let healthTimer: ReturnType<typeof setInterval> | undefined
let clockTimer: ReturnType<typeof setInterval> | undefined

onMounted(() => {
  if (!configuredInstanceUrl) {
    instanceUrl.value = window.location.origin
  }
  now.value = Date.now()
  agentTimer = setInterval(() => void refreshAgents(), 3000)
  healthTimer = setInterval(() => void refreshHealth(), 10000)
  clockTimer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})

onUnmounted(() => {
  if (agentTimer) clearInterval(agentTimer)
  if (healthTimer) clearInterval(healthTimer)
  if (clockTimer) clearInterval(clockTimer)
})

const primaryNavigation = [
  { label: 'Dashboard', icon: 'i-lucide-layout-dashboard', active: true },
  { label: 'Stats', icon: 'i-lucide-chart-no-axes-combined' },
  { label: 'Chat', icon: 'i-lucide-messages-square' },
  { label: 'Economy', icon: 'i-lucide-coins' },
  { label: 'Alchemy', icon: 'i-lucide-flask-conical' },
  { label: 'Academy', icon: 'i-lucide-graduation-cap' },
  { label: 'Map', icon: 'i-lucide-map' },
]

const advancedNavigation = [
  { label: 'Item Search', icon: 'i-lucide-search' },
  { label: 'Skill Builder', icon: 'i-lucide-git-branch' },
  { label: 'Automations', icon: 'i-lucide-zap' },
  { label: 'Server List', icon: 'i-lucide-server' },
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
            <span>self-hosted · slice 1</span>
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
          <span>Server scope</span>
          <button type="button" disabled>
            <UIcon name="i-lucide-layers-3" />
            <span>All servers</span>
            <UIcon name="i-lucide-chevron-down" />
          </button>
        </div>

        <nav class="navigation" aria-label="Primary navigation">
          <button
            v-for="item in primaryNavigation"
            :key="item.label"
            type="button"
            class="nav-item"
            :class="{ active: item.active }"
            :disabled="!item.active"
            :title="sidebarCollapsed ? item.label : undefined"
          >
            <UIcon :name="item.icon" />
            <span>{{ item.label }}</span>
            <span v-if="!item.active" class="nav-soon">later</span>
          </button>

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

          <p class="nav-heading">System</p>
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
          <div class="instance-url" :title="instanceUrl">
            {{ instanceUrl }}
          </div>
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
            <span
              class="diagnostic"
              :class="backendReady ? 'is-ok' : 'is-warning'"
            >
              <span class="diagnostic-dot" />
              {{
                healthStatus === 'pending'
                  ? 'Checking backend'
                  : backendReady
                    ? 'Backend ready'
                    : 'Backend unavailable'
              }}
            </span>
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
          <header class="page-header">
            <div class="page-icon">
              <UIcon name="i-lucide-radio-tower" />
            </div>
            <div>
              <h1>Agent connections</h1>
              <p>
                Authenticated phBot instances connected to this PhMon server.
              </p>
            </div>
          </header>

          <section class="summary-grid" aria-label="Fleet summary">
            <article class="summary-card">
              <span>Registered</span>
              <strong>{{ lastAgents.length }}</strong>
              <small>agents seen</small>
            </article>
            <article class="summary-card">
              <span>Online</span>
              <strong>{{ connectedAgents ?? '—' }}</strong>
              <small>{{
                agentsUnavailable
                  ? 'last loaded state retained'
                  : 'active sockets'
              }}</small>
            </article>
            <article class="summary-card">
              <span>Offline</span>
              <strong>{{ disconnectedAgents ?? '—' }}</strong>
              <small>{{
                agentsUnavailable
                  ? 'last loaded state retained'
                  : 'last known agents'
              }}</small>
            </article>
            <article class="summary-card summary-wide">
              <span>Connection state</span>
              <strong class="summary-state">{{ fleetStatus }}</strong>
              <small>protocol v1 · automatic reconnect</small>
            </article>
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
                  @click="refreshAgents()"
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
                    Generate one identity/token pair for one phBot
                    account/profile. The token can only be recovered from this
                    response.
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
                    {{ credentialCopied === 'agent_id' ? 'Copied' : 'Copy ID' }}
                  </button>
                </div>
                <div class="credential-row">
                  <span>Agent token</span>
                  <code tabindex="0">{{ createdCredential.agent_token }}</code>
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
                  Save this token in the matching phBot PhMon profile now.
                  PostgreSQL stores only its SHA-256 hash, so PhMon cannot show
                  this token again.
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
                  PhMon user authentication is not implemented yet. Until it is,
                  anyone who can access this web UI can create an agent
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
                <code>phmonctl agent create</code>), then configure the matching
                profile in phBot's PhMon plugin tab.
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
                    Slice 0 health stays available as an operational diagnostic.
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
                  <dd>v1</dd>
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
                  <h2>Next data layer</h2>
                  <p>
                    Character identity and live statistics belong to Slice 2.
                  </p>
                </div>
              </div>
              <div class="placeholder-lines" aria-hidden="true">
                <span /><span /><span />
              </div>
            </article>
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
