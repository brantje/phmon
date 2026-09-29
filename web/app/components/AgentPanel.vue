<script setup lang="ts">
const credentialPanelOpen = ref(false)
const credentialCreating = ref(false)
const removingAgentID = ref('')
const removeError = ref('')
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
const agentMutationsUnavailable = computed(
  () => agentsUnavailable.value || liveConnectionState.value !== 'current',
)
const agentsStatus = computed(() => {
  if (agentsUnavailable.value || liveConnectionState.value === 'reconnecting')
    return 'unavailable'
  return liveConnectionState.value !== 'current' &&
    lastAgents.value.length === 0
    ? 'pending'
    : 'success'
})
async function removeAgent(agentID: string) {
  if (
    removingAgentID.value ||
    agentMutationsUnavailable.value ||
    !import.meta.client
  )
    return
  if (
    !window.confirm(
      `Remove agent ${agentID}? Its credential will be revoked and cannot be recovered.`,
    )
  ) {
    return
  }

  removingAgentID.value = agentID
  removeError.value = ''
  try {
    await $fetch('/api/agents/' + encodeURIComponent(agentID), {
      method: 'DELETE',
      retry: 0,
    })
    refreshLiveData(['agents'])
  } catch (error) {
    const failure = error as {
      statusCode?: number
      data?: { error?: string; message?: string }
    }
    removeError.value =
      failure.statusCode === 409
        ? 'Disconnect all phBot sessions for this agent before removing it.'
        : failure.data?.error ||
          failure.data?.message ||
          'Could not remove the agent. Try again.'
  } finally {
    removingAgentID.value = ''
  }
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
</script>

<template>
  <section class="panel agent-panel">
    <div class="panel-header">
      <div>
        <h2>phBot agents</h2>
        <p>
          Create, inspect and revoke phBot agent credentials. Plaintext tokens
          are only shown once when a credential is created.
        </p>
      </div>
      <div class="panel-actions">
        <button
          class="compact-button"
          type="button"
          :disabled="credentialCreating"
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

    <AgentCredentialPanel
      v-model:open="credentialPanelOpen"
      v-model:creating="credentialCreating"
      @created="refreshLiveData(['agents'])"
    />

    <p v-if="removeError" class="agent-remove-error" role="alert">
      {{ removeError }}
    </p>

    <div
      v-if="agentsStatus === 'pending' && lastAgents.length === 0"
      class="empty-state"
    >
      <UIcon name="i-lucide-loader-circle" class="spinning" />
      <strong>Loading agents…</strong>
    </div>

    <div
      v-else-if="agentsStatus === 'unavailable' && lastAgents.length === 0"
      class="empty-state"
    >
      <UIcon name="i-lucide-cloud-off" />
      <strong>Agent data unavailable</strong>
      <p>
        PhMon will retry automatically. Use Refresh to request the latest state.
      </p>
    </div>

    <div v-else-if="lastAgents.length === 0" class="empty-state">
      <UIcon name="i-lucide-plug-zap" />
      <strong>No active agent credentials</strong>
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
            <th>Actions</th>
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
                      : agent.first_seen_at
                        ? 'Offline'
                        : 'Never connected'
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
            <td>
              <button
                class="compact-button danger-button"
                type="button"
                :disabled="
                  agentMutationsUnavailable ||
                  agent.connected ||
                  removingAgentID === agent.agent_id
                "
                :title="
                  agent.connected
                    ? 'Disconnect this agent before removing it'
                    : agentMutationsUnavailable
                      ? 'Wait for current agent state before removing'
                      : 'Revoke this agent credential'
                "
                @click="removeAgent(agent.agent_id)"
              >
                <UIcon
                  :name="
                    removingAgentID === agent.agent_id
                      ? 'i-lucide-loader-circle'
                      : 'i-lucide-trash-2'
                  "
                  :class="{ spinning: removingAgentID === agent.agent_id }"
                />
                {{
                  removingAgentID === agent.agent_id ? 'Removing…' : 'Remove'
                }}
              </button>
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
                    : agent.first_seen_at
                      ? 'Offline'
                      : 'Never connected'
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
          <div class="agent-card-actions">
            <button
              class="compact-button danger-button"
              type="button"
              :disabled="
                agentMutationsUnavailable ||
                agent.connected ||
                removingAgentID === agent.agent_id
              "
              @click="removeAgent(agent.agent_id)"
            >
              <UIcon
                :name="
                  removingAgentID === agent.agent_id
                    ? 'i-lucide-loader-circle'
                    : 'i-lucide-trash-2'
                "
                :class="{ spinning: removingAgentID === agent.agent_id }"
              />
              {{
                removingAgentID === agent.agent_id
                  ? 'Removing…'
                  : 'Remove agent'
              }}
            </button>
          </div>
        </article>
      </div>
    </div>
  </section>
</template>

<style scoped>
.agent-remove-error {
  margin: 10px;
  border: 1px solid rgba(217, 84, 104, 0.35);
  border-radius: 4px;
  background: rgba(84, 25, 39, 0.22);
  padding: 8px 10px;
  color: #ff9eaa;
  font-size: 11px;
}

.danger-button {
  border-color: rgba(191, 80, 99, 0.55);
  color: #ef9aa8;
}

.danger-button:hover:not(:disabled) {
  border-color: #bf5063;
  background: rgba(86, 34, 47, 0.42);
}

.agent-card-actions {
  display: flex;
  justify-content: flex-end;
  margin-top: 10px;
}
</style>
