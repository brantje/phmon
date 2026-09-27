<script setup lang="ts">
const credentialPanelOpen = ref(false)
const {
  agents: lastAgents,
  connectionState: liveConnectionState,
  liveStale,
  refreshLiveData,
} = useLiveData()
const now = ref<number | null>(null)
let timer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  now.value = Date.now()
  timer = setInterval(() => {
    now.value = Date.now()
  }, 1000)
})
onUnmounted(() => {
  if (timer) clearInterval(timer)
})
const agentsUnavailable = computed(
  () => liveStale.value || liveConnectionState.value === 'stale',
)
const agentsStatus = computed(() =>
  liveConnectionState.value !== 'current' && lastAgents.value.length === 0
    ? 'pending'
    : 'success',
)
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
</script>

<template>
  <section class="panel agent-panel">
    <div class="panel-header">
      <div>
        <h2>phBot agents</h2>
        <p>
          Live connection state, plugin version and phBot version. Credentials
          are never exposed here.
        </p>
      </div>
      <div class="panel-actions">
        <button
          class="compact-button"
          type="button"
          @click="credentialPanelOpen = !credentialPanelOpen"
        >
          <UIcon name="i-lucide-key-round" />
          {{ credentialPanelOpen ? 'Close credential' : 'Create credential' }}
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

    <div v-if="agentsUnavailable" class="status-banner warning" role="status">
      <UIcon name="i-lucide-triangle-alert" />
      <div>
        <strong>Agent service unavailable</strong>
        <span v-if="lastAgents.length">
          Showing the last successfully loaded agent list.
        </span>
        <span v-else>PhMon will retry automatically.</span>
      </div>
    </div>

    <AgentCredentialPanel v-model:open="credentialPanelOpen" />

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
        <code>phmonctl agent create</code>), then configure the matching profile
        in phBot's PhMon plugin tab.
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
              {{ agent.protocol_version ? 'v' + agent.protocol_version : '—' }}
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
                  agent.protocol_version ? 'v' + agent.protocol_version : '—'
                }}
              </dd>
            </div>
          </dl>
        </article>
      </div>
    </div>
  </section>
</template>
