<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'

const props = defineProps<{ character: CharacterView }>()
const {
  commandHistory,
  characterControls,
  liveStale,
  setCharacterCommands,
  setCharacterControls,
  clearCharacterCommandSubscriptions,
} = useLiveData()

const dialog = ref('')
const traceName = ref('')
const areaMode = ref<'current_position' | 'position' | 'named'>(
  'current_position',
)
const areaName = ref('')
const radius = ref('')
const walk = reactive({ x: '', y: '', z: '' })
const working = ref(false)
const notice = ref('')
const errorText = ref('')
const historyState = ref('')
const historyName = ref('')
const requestTarget = ref<{ characterID: string; sessionID: string } | null>(
  null,
)
const actions = [
  { name: 'bot.start', label: 'Start Training', icon: 'i-lucide-play' },
  { name: 'bot.stop', label: 'Stop Training', icon: 'i-lucide-square' },
  { name: 'trace.start', label: 'Start Trace', icon: 'i-lucide-route' },
  { name: 'trace.stop', label: 'Stop Trace', icon: 'i-lucide-route-off' },
  {
    name: 'training.area.set',
    label: 'Set Training Area',
    icon: 'i-lucide-crosshair',
  },
  {
    name: 'training.radius.set',
    label: 'Set Training Radius',
    icon: 'i-lucide-circle-dot',
  },
  { name: 'character.return', label: 'Return Scroll', icon: 'i-lucide-scroll' },
  { name: 'character.walk', label: 'Walk', icon: 'i-lucide-footprints' },
  {
    name: 'character.disconnect',
    label: 'Disconnect',
    icon: 'i-lucide-unplug',
  },
] as const

const commandFilterKey = computed(
  () => `${historyName.value}:${historyState.value}`,
)
let stopWatch: (() => void) | undefined
onMounted(() => {
  stopWatch = watch(
    () => [props.character.character_id, commandFilterKey.value] as const,
    ([id]) => {
      setCharacterControls(id)
      setCharacterCommands(id, historyName.value, historyState.value)
    },
    { immediate: true },
  )
})
onBeforeUnmount(() => {
  stopWatch?.()
  clearCharacterCommandSubscriptions()
})

const currentSession = computed(() =>
  Boolean(
    props.character.online &&
    props.character.session_id &&
    characterControls.value?.character_id === props.character.character_id &&
    characterControls.value.session_id === props.character.session_id,
  ),
)

const sessionCapabilityNotice = computed(() => {
  if (liveStale.value || !currentSession.value) return ''
  const capabilities = characterControls.value?.capabilities || {}
  if (
    Object.entries(capabilities).some(
      ([name, capability]) =>
        name !== 'character.walk' &&
        capability.reason === 'plugin_upgrade_required',
    )
  )
    return 'This character session is using the older agent protocol. Reload the current PhMon plugin in this phBot profile so it reconnects with protocol v3 and reports its capabilities.'
  return ''
})

function capabilityReason(name: string) {
  if (liveStale.value) return 'Live connection is stale.'
  if (!props.character.online || !props.character.session_id)
    return 'Character is offline.'
  if (!currentSession.value)
    return 'Waiting for fresh capabilities for this session.'
  const capability = characterControls.value?.capabilities[name]
  if (!capability) return 'Capability report is unavailable.'
  if (
    capability.reason === 'plugin_upgrade_required' &&
    name === 'character.walk'
  )
    return 'Pathfinding Walk requires PhMon plugin 1.1.2 or newer in this phBot profile.'
  if (capability.reason === 'plugin_upgrade_required')
    return 'This character session uses the older agent protocol. Reload the PhMon plugin for this phBot profile to reconnect with protocol v3.'
  if (capability.reason === 'capabilities_pending')
    return 'Waiting for this session to report command capabilities.'
  if (
    name === 'training.area.set' &&
    capability.modes?.length &&
    !capability.modes.some(
      (mode) =>
        mode === 'current_position' || mode === 'position' || mode === 'named',
    )
  )
    return 'No supported training-area mode is available in this runtime.'
  return capability.supported
    ? ''
    : capability.reason || 'Unsupported by this runtime.'
}

function actionArgs(name: string): Record<string, unknown> | null {
  if (name === 'trace.start') {
    if (!traceName.value.trim() || traceName.value.trim().length > 64)
      return null
    return { name: traceName.value.trim() }
  }
  if (name === 'training.area.set') {
    if (areaMode.value === 'current_position')
      return { mode: 'current_position' }
    if (areaMode.value === 'named') {
      const name = areaName.value.trim()
      return name && new TextEncoder().encode(name).length <= 100
        ? { mode: 'named', name }
        : null
    }
    const x = Number(walk.x),
      y = Number(walk.y),
      z = Number(walk.z)
    if (![x, y, z].every(Number.isFinite) || props.character.region == null)
      return null
    return { mode: 'position', region: props.character.region, x, y, z }
  }
  if (name === 'training.radius.set') {
    const value = Number(radius.value)
    return Number.isFinite(value) && value >= 1 && value <= 10000
      ? { radius: value }
      : null
  }
  if (name === 'character.walk') {
    const x = Number(walk.x),
      y = Number(walk.y),
      z = Number(walk.z)
    if (![x, y, z].every(Number.isFinite) || props.character.region == null)
      return null
    return { region: props.character.region, x, y, z }
  }
  return {}
}

function openAction(name: string) {
  if (capabilityReason(name)) return
  requestTarget.value = {
    characterID: props.character.character_id,
    sessionID: props.character.session_id || '',
  }
  errorText.value = ''
  notice.value = ''
  if (name === 'trace.start') dialog.value = name
  else if (name === 'training.area.set') {
    areaMode.value = characterControls.value?.capabilities[
      name
    ]?.modes?.includes('current_position')
      ? 'current_position'
      : characterControls.value?.capabilities[name]?.modes?.includes('position')
        ? 'position'
        : 'named'
    areaName.value = ''
    dialog.value = name
  } else if (name === 'training.radius.set') {
    radius.value = ''
    dialog.value = name
  } else if (name === 'character.walk') {
    walk.x = ''
    walk.y = ''
    walk.z = ''
    dialog.value = name
  } else void submit(name)
}

async function submit(name: string) {
  if (!requestTarget.value) {
    requestTarget.value = {
      characterID: props.character.character_id,
      sessionID: props.character.session_id || '',
    }
  }
  if (
    requestTarget.value.characterID !== props.character.character_id ||
    requestTarget.value.sessionID !== props.character.session_id
  ) {
    errorText.value =
      'The character session changed. Close this form and review the new target.'
    return
  }
  const args = actionArgs(name)
  if (!args) {
    errorText.value = 'Enter valid values for this action.'
    return
  }
  if (['character.return', 'character.disconnect'].includes(name)) {
    const action =
      name === 'character.return' ? 'use a Return Scroll' : 'disconnect'
    if (
      !window.confirm(
        `Confirm ${action} for ${props.character.name} on ${props.character.server}?`,
      )
    )
      return
  }
  working.value = true
  errorText.value = ''
  notice.value = ''
  try {
    const accepted = await $fetch<{ command_id: string; state: string }>(
      '/api/commands',
      {
        method: 'POST',
        body: {
          character_id: requestTarget.value.characterID,
          expected_session_id: requestTarget.value.sessionID,
          name,
          args,
          idempotency_key: createIdempotencyKey(),
          confirmation: ['character.return', 'character.disconnect'].includes(
            name,
          ),
        },
      },
    )
    notice.value = `Accepted ${accepted.command_id}. Waiting for its live result; acceptance is not execution success.`
    dialog.value = ''
    setCharacterCommands(
      props.character.character_id,
      historyName.value,
      historyState.value,
    )
  } catch (error) {
    const failure = error as { data?: { error?: string; message?: string } }
    errorText.value =
      failure.data?.message ||
      failure.data?.error ||
      'Command could not be accepted.'
  } finally {
    working.value = false
  }
}
</script>

<template>
  <section class="panel remote-actions">
    <div class="panel-header compact">
      <div>
        <h2>Actions</h2>
        <p>
          {{ character.name }} · {{ character.server }} · session
          {{ character.session_id || 'offline' }}
        </p>
      </div>
      <span class="status-chip" :class="currentSession ? 'online' : 'stale'"
        ><span />{{
          currentSession ? 'Target current' : 'Target unavailable'
        }}</span
      >
    </div>
    <div v-if="liveStale" class="status-banner warning" role="status">
      Live command controls are stale and temporarily disabled.
    </div>
    <div
      v-else-if="sessionCapabilityNotice"
      class="status-banner warning"
      role="status"
    >
      {{ sessionCapabilityNotice }}
    </div>
    <div class="remote-action-grid">
      <button
        v-for="item in actions"
        :key="item.name"
        class="remote-action-button"
        type="button"
        :disabled="Boolean(capabilityReason(item.name)) || working"
        :title="
          capabilityReason(item.name) ||
          `Target ${character.name} on ${character.server}`
        "
        @click="openAction(item.name)"
      >
        <UIcon :name="item.icon" /><span>{{ item.label }}</span>
      </button>
      <button
        class="remote-action-button is-later"
        type="button"
        disabled
        title="Script discovery and execution are later tool-parity work."
      >
        <UIcon name="i-lucide-scroll-text" /><span>Execute Script · later</span>
      </button>
    </div>
    <p v-if="characterControls?.training" class="status-banner">
      Training area
      <template v-if="characterControls.training.training_available">
        · region {{ characterControls.training.training_region ?? 'unknown' }} ·
        position {{ characterControls.training.training_x ?? '—' }},
        {{ characterControls.training.training_y ?? '—' }},
        {{ characterControls.training.training_z ?? '—' }} · radius
        {{ characterControls.training.training_radius ?? 'unknown' }}
      </template>
      <template v-else> unavailable</template>
      · observed {{ characterControls.training.observed_at || 'time unknown' }}
    </p>
    <p v-if="notice" class="status-banner success" role="status">
      {{ notice }}
    </p>
    <p v-if="errorText" class="status-banner warning" role="alert">
      {{ errorText }}
    </p>

    <div
      v-if="dialog"
      class="command-dialog-backdrop"
      @click.self="dialog = ''"
    >
      <form class="panel command-dialog" @submit.prevent="submit(dialog)">
        <h3>{{ actions.find((item) => item.name === dialog)?.label }}</h3>
        <p>
          Target locked to <strong>{{ character.name }}</strong> on
          {{ character.server }} · {{ requestTarget?.sessionID }}
        </p>
        <label v-if="dialog === 'trace.start'"
          >Player name<input
            v-model="traceName"
            maxlength="64"
            required
            autocomplete="off"
        /></label>
        <template v-if="dialog === 'training.area.set'">
          <label
            >Set area from
            <select v-model="areaMode">
              <option
                v-if="
                  !characterControls?.capabilities['training.area.set']?.modes
                    ?.length ||
                  characterControls.capabilities[
                    'training.area.set'
                  ].modes.includes('current_position')
                "
                value="current_position"
              >
                Current live position
              </option>
              <option
                v-if="
                  characterControls?.capabilities[
                    'training.area.set'
                  ]?.modes?.includes('position')
                "
                value="position"
              >
                Coordinates in current region
              </option>
              <option
                v-if="
                  characterControls?.capabilities[
                    'training.area.set'
                  ]?.modes?.includes('named')
                "
                value="named"
              >
                Select named area
              </option>
            </select>
          </label>
          <label v-if="areaMode === 'named'"
            >Area name<input
              v-model="areaName"
              maxlength="100"
              required
              autocomplete="off"
          /></label>
          <div v-if="areaMode === 'position'" class="command-coordinates">
            <label
              >X<input
                v-model="walk.x"
                type="number"
                step="any"
                required /></label
            ><label
              >Y<input
                v-model="walk.y"
                type="number"
                step="any"
                required /></label
            ><label
              >Z<input v-model="walk.z" type="number" step="any" required
            /></label>
          </div>
          <small
            >Region {{ character.region ?? 'unavailable' }}. Named areas must
            exactly match a name configured in phBot. Selection is available
            only when this plugin runtime reports its documented API.</small
          >
        </template>
        <label v-if="dialog === 'training.radius.set'"
          >Radius (application safety range 1–10,000)<input
            v-model="radius"
            type="number"
            min="1"
            max="10000"
            step="any"
            required
        /></label>
        <div v-if="dialog === 'character.walk'" class="command-coordinates">
          <label
            >X<input
              v-model="walk.x"
              type="number"
              step="any"
              required /></label
          ><label
            >Y<input
              v-model="walk.y"
              type="number"
              step="any"
              required /></label
          ><label
            >Z<input v-model="walk.z" type="number" step="any" required
          /></label>
          <small
            >phBot calculates a waypoint route within observed region
            {{ character.region ?? 'unavailable' }}; teleport routes are not
            supported. Arrival is reported only after live position readback
            reaches the destination.</small
          >
        </div>
        <p v-if="errorText" role="alert">{{ errorText }}</p>
        <div class="command-dialog-actions">
          <button type="button" class="compact-button" @click="dialog = ''">
            Cancel</button
          ><button
            class="compact-button primary"
            type="submit"
            :disabled="working"
          >
            {{ working ? 'Submitting…' : 'Submit command' }}
          </button>
        </div>
      </form>
    </div>

    <div class="command-history">
      <div class="panel-header compact">
        <div>
          <h3>Command history</h3>
          <p>Durable results arrive on the shared live connection.</p>
        </div>
      </div>
      <div class="panel-actions">
        <select v-model="historyName" aria-label="Filter command">
          <option value="">All commands</option>
          <option v-for="item in actions" :key="item.name" :value="item.name">
            {{ item.label }}
          </option></select
        ><select v-model="historyState" aria-label="Filter command status">
          <option value="">All states</option>
          <option
            v-for="state in [
              'queued',
              'dispatching',
              'sent',
              'acknowledged',
              'completed',
              'failed',
              'expired',
              'unknown',
            ]"
            :key="state"
            :value="state"
          >
            {{ state }}
          </option>
        </select>
      </div>
      <div class="agent-table-wrap">
        <table class="agent-table">
          <thead>
            <tr>
              <th>Action</th>
              <th>State</th>
              <th>Submitted</th>
              <th>Result / evidence</th>
            </tr>
          </thead>
          <tbody>
            <tr v-for="command in commandHistory" :key="command.command_id">
              <td>
                {{
                  actions.find((item) => item.name === command.name)?.label ||
                  command.name
                }}
              </td>
              <td>
                <span
                  class="status-chip"
                  :class="
                    ['completed', 'failed', 'expired', 'unknown'].includes(
                      command.state,
                    )
                      ? command.state === 'completed'
                        ? 'online'
                        : 'stale'
                      : 'pending'
                  "
                  >{{ command.state }}</span
                >
              </td>
              <td>{{ formatTimestamp(command.created_at) }}</td>
              <td>
                {{
                  command.message ||
                  (command.verification
                    ? `${command.verification}${command.api_return === false ? ' · API returned false' : ''}`
                    : 'Waiting for execution evidence')
                }}
              </td>
            </tr>
          </tbody>
        </table>
      </div>
      <div v-if="!commandHistory.length" class="empty-state compact-empty">
        No matching command history for this character.
      </div>
    </div>
  </section>
</template>
