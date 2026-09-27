<script setup lang="ts">
const {
  fleetCharacters,
  characterControls,
  commandHistory,
  setCharacterCommands,
  setCharacterControls,
  clearCharacterCommandSubscriptions,
  liveStale,
} = useLiveData()
const targetID = ref('')
const resultMessage = ref('No command has been submitted from this panel.')
const errorMessage = ref('')
const sending = ref(false)
const selectedCharacter = computed(
  () =>
    fleetCharacters.value.find(
      (item) => item.character_id === targetID.value,
    ) || null,
)
let stopTargetWatch: (() => void) | undefined
onMounted(() => {
  stopTargetWatch = watch(
    targetID,
    (id) => {
      if (!id) {
        clearCharacterCommandSubscriptions()
        return
      }
      setCharacterControls(id)
      setCharacterCommands(id)
    },
    { immediate: true },
  )
})
onBeforeUnmount(() => {
  stopTargetWatch?.()
  clearCharacterCommandSubscriptions()
})
const clientCapability = computed(() =>
  characterControls.value?.character_id === targetID.value
    ? characterControls.value.capabilities['client.clientless']
    : undefined,
)
const disabledReason = computed(() => {
  if (!selectedCharacter.value) return 'Choose a character first.'
  if (liveStale.value) return 'Live controls are stale.'
  if (!selectedCharacter.value.online || !selectedCharacter.value.session_id)
    return 'Character is offline.'
  if (
    characterControls.value?.session_id !== selectedCharacter.value.session_id
  )
    return 'Waiting for capabilities for the current session.'
  return clientCapability.value?.supported
    ? ''
    : clientCapability.value?.reason ||
        'This phBot runtime has not reported a safe clientless primitive.'
})
async function goClientless() {
  const target = selectedCharacter.value
  if (!target?.session_id || disabledReason.value) return
  if (
    !window.confirm(
      `Go clientless for ${target.name} on ${target.server}? This uses only the selected character's documented runtime capability.`,
    )
  )
    return
  sending.value = true
  errorMessage.value = ''
  try {
    const response = await $fetch<{ command_id: string }>('/api/commands', {
      method: 'POST',
      body: {
        character_id: target.character_id,
        expected_session_id: target.session_id,
        name: 'client.clientless',
        args: {},
        confirmation: true,
        idempotency_key: createIdempotencyKey(),
      },
    })
    resultMessage.value = `Accepted ${response.command_id}. Awaiting result over the live connection.`
  } catch (error) {
    const failure = error as { data?: { message?: string; error?: string } }
    errorMessage.value =
      failure.data?.message ||
      failure.data?.error ||
      'Command could not be accepted.'
  } finally {
    sending.value = false
  }
}
</script>

<template>
  <div class="client-tool-page">
    <PageHeader
      title="phBot | Client"
      icon="i-lucide-bot"
      description="Session-scoped controls for the selected phBot character."
    />
    <section class="panel client-intro">
      <small>LOCAL CONTROL</small>
      <h2>Manage a selected phBot character</h2>
      <p>
        Actions use the live session assigned to this character. Commands never
        target every client process on the machine.
      </p>
      <label
        >Character
        <select
          v-model="targetID"
          aria-label="Select character for Client actions"
        >
          <option value="">Select a character</option>
          <option
            v-for="character in fleetCharacters"
            :key="character.character_id"
            :value="character.character_id"
          >
            {{ character.name }} · {{ character.server }} ·
            {{ character.online ? 'Online' : 'Offline' }}
          </option>
        </select>
      </label>
    </section>
    <div class="client-tool-grid">
      <nav class="panel client-tool-menu" aria-label="phBot tools">
        <NuxtLink class="nav-item active" to="/phbot/client" aria-current="page"
          ><UIcon name="i-lucide-monitor" /><span>Client</span></NuxtLink
        >
        <span class="nav-item disabled"
          ><UIcon name="i-lucide-users" /><span>Party</span
          ><small>Slice 4</small></span
        >
        <span class="nav-item disabled"
          ><UIcon name="i-lucide-scroll-text" /><span>Scripts</span
          ><small>Later</small></span
        >
        <span class="nav-item disabled"
          ><UIcon name="i-lucide-list-checks" /><span>Quest</span
          ><small>Later</small></span
        >
      </nav>
      <section class="panel clientless-panel">
        <div class="panel-header compact">
          <div>
            <h2>Go Clientless</h2>
            <p>Requires a verified, per-session phBot API capability.</p>
          </div>
        </div>
        <p class="clientless-explanation">
          No supported clientless mutation was found in the official public API
          docs or installed runtime evidence. The demo describes killing all
          <code>sro_client.exe</code> processes; PhMon does not reproduce that
          machine-wide behavior.
        </p>
        <button
          class="compact-button primary"
          type="button"
          :disabled="Boolean(disabledReason) || sending"
          :title="disabledReason"
          @click="goClientless"
        >
          {{ sending ? 'Submitting…' : 'Go Clientless' }}
        </button>
        <p v-if="disabledReason" class="capability-reason" role="status">
          {{ disabledReason }}
        </p>
        <p v-if="errorMessage" class="status-banner warning" role="alert">
          {{ errorMessage }}
        </p>
        <div class="client-result-inset">
          <strong>LATEST RESULT</strong>
          <p>{{ resultMessage }}</p>
          <p v-if="commandHistory[0]">
            {{ commandHistory[0].name }} · {{ commandHistory[0].state }} ·
            {{
              commandHistory[0].message ||
              commandHistory[0].verification ||
              'No execution evidence yet'
            }}
          </p>
        </div>
      </section>
    </div>
  </div>
</template>
