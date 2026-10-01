<script setup lang="ts">
import type { CharacterGroup, CharacterView } from '~~/shared/types/live'
import {
  actionTargetGroupState,
  applyActionTargetGroup,
  clearActionTargets,
  reconcileActionTargets,
  selectAllActionTargets,
  toggleActionTarget,
} from '~/utils/actionTargets'

const props = defineProps<{
  characters: readonly CharacterView[]
  groups: readonly (Omit<CharacterGroup, 'members'> & {
    members: readonly CharacterView[]
  })[]
  selectedIds: string[]
  snapshotCurrent: boolean
}>()
const emit = defineEmits<{
  'update:selectedIds': [ids: string[]]
  inspect: [characterID: string]
}>()

const selected = computed(() => new Set(props.selectedIds))
const visibleIDs = computed(
  () => new Set(props.characters.map((character) => character.character_id)),
)
const applicableGroups = computed(() =>
  props.groups
    .map((group) => ({
      ...group,
      memberIDs: [
        ...new Set(
          group.members
            .filter((member) => visibleIDs.value.has(member.character_id))
            .map((member) => member.character_id),
        ),
      ],
    }))
    .filter((group) => group.memberIDs.length),
)

watch(
  () => [props.snapshotCurrent, props.characters] as const,
  ([current]) => {
    const reconciled = reconcileActionTargets(
      selected.value,
      visibleIDs.value,
      current,
    )
    if (
      reconciled.size !== selected.value.size ||
      [...reconciled].some((id) => !selected.value.has(id))
    )
      emit('update:selectedIds', [...reconciled])
  },
)

function update(next: Set<string>) {
  emit('update:selectedIds', [...next])
}
</script>

<template>
  <section class="panel action-target-selector" aria-labelledby="targets-title">
    <header class="action-target-heading">
      <div>
        <h2 id="targets-title">Action targets</h2>
        <p>Select characters or a saved group.</p>
      </div>
      <span class="status-chip" :class="snapshotCurrent ? 'online' : 'stale'">
        <span />{{ snapshotCurrent ? 'Live list' : 'List stale' }}
      </span>
    </header>
    <div class="action-target-toolbar">
      <button
        type="button"
        class="compact-button"
        :disabled="!props.characters.length"
        @click="
          update(
            selectAllActionTargets(
              props.characters.map((item) => item.character_id),
            ),
          )
        "
      >
        All
      </button>
      <button
        type="button"
        class="compact-button"
        :disabled="!props.selectedIds.length"
        @click="update(clearActionTargets())"
      >
        None
      </button>
      <span role="status">{{ props.selectedIds.length }} selected</span>
    </div>

    <div v-if="applicableGroups.length" class="action-target-groups">
      <label
        v-for="group in applicableGroups"
        :key="group.group_id"
        class="action-target-group"
      >
        <input
          type="checkbox"
          :checked="
            actionTargetGroupState(selected, group.memberIDs) === 'checked'
          "
          :indeterminate="
            actionTargetGroupState(selected, group.memberIDs) ===
            'indeterminate'
          "
          :aria-checked="
            actionTargetGroupState(selected, group.memberIDs) ===
            'indeterminate'
              ? 'mixed'
              : actionTargetGroupState(selected, group.memberIDs) === 'checked'
                ? 'true'
                : 'false'
          "
          :aria-label="`Select group ${group.name} for actions`"
          @change="update(applyActionTargetGroup(selected, group.memberIDs))"
        />
        <span>{{ group.name }}</span>
        <small>{{ group.memberIDs.length }}</small>
      </label>
    </div>

    <div class="action-target-list">
      <div
        v-for="character in props.characters"
        :key="character.character_id"
        class="action-target-row"
      >
        <label>
          <input
            type="checkbox"
            :checked="selected.has(character.character_id)"
            :aria-label="`Target ${character.name} on ${character.server}`"
            @change="
              update(toggleActionTarget(selected, character.character_id))
            "
          />
          <span class="action-target-name">{{ character.name }}</span>
          <span class="action-target-server">{{ character.server }}</span>
          <span
            class="status-chip"
            :class="character.online ? 'online' : 'stale'"
            ><span />{{ character.online ? 'Online' : 'Offline' }}</span
          >
        </label>
        <button
          type="button"
          class="compact-button action-target-inspect"
          :aria-label="`Inspect ${character.name}`"
          @click="emit('inspect', character.character_id)"
        >
          Inspect
        </button>
      </div>
      <p v-if="!props.characters.length" class="action-target-empty">
        No character records are available in this server scope.
      </p>
    </div>
  </section>
</template>

<style scoped>
.action-target-selector {
  display: grid;
  min-width: 0;
  gap: 9px;
  padding: 11px;
}
.action-target-heading,
.action-target-toolbar,
.action-target-row,
.action-target-row label {
  display: flex;
  align-items: center;
  gap: 8px;
  min-width: 0;
}
.action-target-heading,
.action-target-row {
  justify-content: space-between;
}
.action-target-heading h2,
.action-target-heading p {
  margin: 0;
}
.action-target-heading h2 {
  color: var(--ph-primary);
  font-size: 14px;
}
.action-target-heading p,
.action-target-toolbar span,
.action-target-server,
.action-target-empty {
  color: var(--ph-muted);
  font-size: 12px;
}
.action-target-toolbar {
  flex-wrap: wrap;
}
.action-target-toolbar span {
  margin-left: auto;
}
.action-target-groups,
.action-target-list {
  display: grid;
  gap: 5px;
  max-height: 230px;
  overflow: auto;
}
.action-target-group,
.action-target-row {
  padding: 5px 7px;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  background: rgb(13 19 29 / 62%);
}
.action-target-group {
  display: flex;
  align-items: center;
  gap: 7px;
  min-width: 0;
  color: var(--ph-text);
  font-size: 12px;
}
.action-target-group span,
.action-target-name {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.action-target-group small {
  margin-left: auto;
  color: var(--ph-muted);
}
.action-target-row label {
  flex: 1;
}
.action-target-name {
  color: var(--ph-text);
  font-size: 12px;
}
.action-target-server {
  overflow: hidden;
  text-overflow: ellipsis;
  white-space: nowrap;
}
.action-target-inspect {
  min-height: 27px;
  padding: 3px 7px;
  font-size: 11px;
}
.action-target-empty {
  margin: 0;
  padding: 8px;
}
input:focus-visible,
button:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}
</style>
