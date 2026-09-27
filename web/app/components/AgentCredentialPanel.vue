<script setup lang="ts">
import type { AgentCredential } from '~~/shared/types/agent'
const credentialPanelOpen = defineModel<boolean>('open', { required: true })
const credentialCreating = ref(false)
const createdCredential = ref<AgentCredential | null>(null)
const credentialError = ref('')
const credentialCopied = ref<'agent_id' | 'agent_token' | null>(null)
const credentialCopyFallback = ref<'agent_id' | 'agent_token' | null>(null)
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

watch(credentialPanelOpen, (open) => {
  if (!open) dismissCredential()
})
</script>

<template>
  <section
    v-if="credentialPanelOpen"
    class="credential-panel"
    aria-label="Create agent credential"
  >
    <div class="credential-panel-head">
      <div>
        <strong>Create agent credential</strong>
        <p>
          An agent ID/token identifies one logical PhMon agent. Reuse it across
          phBot connections that belong together, or create separate credentials
          for separate agents. The token can only be recovered from this
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
          {{ credentialCopied === 'agent_token' ? 'Copied' : 'Copy token' }}
        </button>
      </div>
      <p
        v-if="credentialCopyFallback"
        class="credential-message warning"
        role="status"
      >
        Clipboard access is unavailable. Select the value above and copy it
        manually.
      </p>
      <p class="credential-message" role="status">
        Save this token in each phBot PhMon profile that should use this logical
        agent now. PostgreSQL stores only its SHA-256 hash, so PhMon cannot show
        this token again.
      </p>
      <div class="credential-actions">
        <button class="compact-button" type="button" @click="dismissCredential">
          Done
        </button>
      </div>
    </template>

    <template v-else>
      <p class="credential-risk">
        PhMon user authentication is not implemented yet. Until it is, anyone
        who can access this web UI can create an agent credential. Keep this
        instance on a trusted network.
      </p>
      <p v-if="credentialError" class="credential-message warning" role="alert">
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
            credentialCreating ? 'Creating…' : 'Generate one-time credential'
          }}
        </button>
      </div>
    </template>
  </section>
</template>
