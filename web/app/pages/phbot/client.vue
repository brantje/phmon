<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'

const {
  fleetCharacters,
  characterControls,
  commandHistory,
  setCharacterCommands,
  setCharacterControls,
  clearCharacterCommandSubscriptions,
  liveStale,
  connectionState,
} = useLiveData()
const { matchesServer, serverScope, scopedGroups } = useServerScope()
const targetIDs = ref<string[]>([])
const inspectedCharacterID = ref('')
const currentList = computed(
  () => connectionState.value === 'current' && !liveStale.value,
)
const selectableCharacters = computed(() =>
  fleetCharacters.value.filter((character) => matchesServer(character.server)),
)
const inspectedCharacter = computed(
  () =>
    selectableCharacters.value.find(
      (character) => character.character_id === inspectedCharacterID.value,
    ) || null,
)
const clientScopeKey = computed(
  () => `client:${serverScope.value.toLocaleLowerCase()}`,
)
function clientScopeKeyForCharacter(
  character: CharacterView,
  scopeKey: string,
) {
  return matchesServer(character.server)
    ? scopeKey
    : `server:${character.server.toLocaleLowerCase()}`
}
function currentClientScopeKey(characterID: string) {
  const character = fleetCharacters.value.find(
    (item) => item.character_id === characterID,
  )
  return character
    ? clientScopeKeyForCharacter(character, clientScopeKey.value)
    : 'unavailable'
}
function findCurrentCharacter(characterID: string) {
  return fleetCharacters.value.find(
    (character) => character.character_id === characterID,
  )
}
function inspectCharacter(characterID: string) {
  inspectedCharacterID.value = characterID
}

watch(serverScope, () => {
  targetIDs.value = []
  inspectedCharacterID.value = ''
})
watch(
  [inspectedCharacterID, currentList],
  ([id, current]) => {
    if (!current) return
    if (
      id &&
      !selectableCharacters.value.some(
        (character) => character.character_id === id,
      )
    ) {
      inspectedCharacterID.value = ''
      return
    }
    if (id) {
      setCharacterControls(id)
      setCharacterCommands(id)
    } else {
      clearCharacterCommandSubscriptions()
    }
  },
  { immediate: true },
)
onBeforeUnmount(clearCharacterCommandSubscriptions)
</script>

<template>
  <div class="client-tool-page">
    <PageHeader
      title="phBot | Client"
      icon="i-lucide-bot"
      description="Session-scoped controls for one character, a saved group or the current server scope."
    />
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

      <main class="client-control-workspace">
        <p v-if="liveStale" class="status-banner warning" role="status">
          Live data is stale. Character actions are disabled until the current
          fleet snapshot returns.
        </p>
        <ActionTargetSelector
          v-model:selected-ids="targetIDs"
          :characters="selectableCharacters"
          :groups="scopedGroups"
          :snapshot-current="currentList"
          @inspect="inspectCharacter"
        />
        <RemoteControlPanel
          :selected-ids="targetIDs"
          :scope-key="clientScopeKey"
          :scope-key-for-character="clientScopeKeyForCharacter"
          :current-scope-key="currentClientScopeKey"
          :current-character="findCurrentCharacter"
        />

        <section class="panel client-inspector">
          <header>
            <div>
              <h2>Character command history</h2>
              <p>
                Inspect a character independently of the action target
                selection.
              </p>
            </div>
            <span v-if="inspectedCharacter" class="status-chip">
              <span />{{ inspectedCharacter.name }} ·
              {{ inspectedCharacter.server }}
            </span>
          </header>
          <p v-if="!inspectedCharacter" class="client-inspector-empty">
            Choose Inspect beside a character to view its recent command
            results.
          </p>
          <p v-else-if="!commandHistory.length" class="client-inspector-empty">
            No recent commands are recorded for this character.
          </p>
          <ul v-else class="client-history-list">
            <li v-for="command in commandHistory" :key="command.command_id">
              <span
                ><strong>{{ command.name }}</strong
                ><small>{{ command.created_at }}</small></span
              >
              <span
                class="status-chip"
                :class="
                  command.state === 'completed'
                    ? 'online'
                    : ['failed', 'expired'].includes(command.state)
                      ? 'stale'
                      : 'pending'
                "
              >
                <span />{{ command.state }}
              </span>
              <small>{{
                command.message ||
                command.verification ||
                'No execution evidence yet'
              }}</small>
            </li>
          </ul>
          <p
            v-if="
              inspectedCharacter &&
              characterControls?.character_id ===
                inspectedCharacter.character_id &&
              characterControls.session_id === inspectedCharacter.session_id
            "
            class="client-inspector-readback"
          >
            <template v-if="characterControls.training?.training_available">
              Training area ·
              {{
                characterControls.training.training_zone ||
                inspectedCharacter.zone ||
                'Unknown zone'
              }}
              · radius
              {{ characterControls.training.training_radius ?? 'unknown' }}
            </template>
            <template v-else
              >Training area readback unavailable for this session.</template
            >
          </p>
        </section>
      </main>
    </div>
  </div>
</template>

<style scoped>
.client-tool-page {
  display: grid;
  gap: 0.9rem;
}
.client-tool-grid {
  display: grid;
  grid-template-columns: minmax(150px, 205px) minmax(0, 1fr);
  align-items: start;
  gap: 0.8rem;
}
.client-tool-menu {
  display: grid;
  gap: 0.3rem;
  padding: 0.55rem;
}
.client-tool-menu .nav-item {
  display: flex;
  align-items: center;
  gap: 0.5rem;
  padding: 0.6rem;
  border: 1px solid #344253;
  border-radius: 4px;
  color: #eaf1ff;
}
.client-control-workspace {
  display: grid;
  min-width: 0;
  gap: 0.7rem;
}
.client-inspector {
  display: grid;
  min-width: 0;
  gap: 8px;
  padding: 11px;
}
.client-inspector header,
.client-inspector header > div {
  display: flex;
  align-items: flex-start;
  justify-content: space-between;
  flex-wrap: wrap;
  gap: 7px;
  min-width: 0;
}
.client-inspector h2,
.client-inspector p {
  margin: 0;
}
.client-inspector h2 {
  color: var(--ph-primary);
  font-size: 14px;
}
.client-inspector header p,
.client-inspector-empty,
.client-inspector-readback {
  color: var(--ph-muted);
  font-size: 12px;
}
.client-history-list {
  display: grid;
  gap: 5px;
  max-height: 250px;
  overflow: auto;
  margin: 0;
  padding: 0;
  list-style: none;
}
.client-history-list li {
  display: grid;
  grid-template-columns: minmax(0, 1fr) auto;
  align-items: center;
  gap: 4px 10px;
  padding: 7px;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  color: var(--ph-muted);
  font-size: 12px;
}
.client-history-list li > span:first-child {
  display: grid;
  min-width: 0;
}
.client-history-list strong {
  color: var(--ph-text);
}
.client-history-list small,
.client-inspector-readback {
  overflow-wrap: anywhere;
}
.client-history-list li > small {
  grid-column: 1 / -1;
}
@media (max-width: 640px) {
  .client-tool-grid {
    grid-template-columns: minmax(0, 1fr);
  }
  .client-tool-menu {
    grid-template-columns: repeat(2, minmax(0, 1fr));
  }
}
</style>
