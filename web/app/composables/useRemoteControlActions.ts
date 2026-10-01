import type { CharacterView, ControlsSnapshot } from '~~/shared/types/live'
import { useCommandFanOut } from '~/composables/useCommandFanOut'
import type {
  FanOutChild,
  FanOutCommandRequest,
  FanOutSkipReason,
} from '~/utils/commandFanOut'
import {
  remoteControlDefinition,
  type RemoteControlActionName,
  type RemoteControlArgs,
} from '~/utils/remoteControlActions'

export function useRemoteControlActions(options: {
  scopeKey(): string
  scopeKeyForCharacter(character: CharacterView, scopeKey: string): string
  currentScopeKey(characterID: string): string
  currentCharacter(characterID: string): CharacterView | undefined
  mapSnapshotCurrent?(): boolean
}) {
  const live = useLiveData()
  const fanOut = useCommandFanOut({
    command: remoteControlDefinition('bot.start'),
    get scopeKey() {
      return options.scopeKey()
    },
    scopeKeyForCharacter: (character, operationScope) =>
      options.scopeKeyForCharacter(
        character,
        operationScope || options.scopeKey(),
      ),
    currentScopeKey: options.currentScopeKey,
    currentCharacter: options.currentCharacter,
  })
  const controlsCurrent = computed(
    () =>
      Boolean(fanOut.feed.value?.controls_current) &&
      live.connectionState.value === 'current' &&
      !live.liveStale.value,
  )

  function definitionFor(
    name: RemoteControlActionName,
    args: RemoteControlArgs,
  ) {
    const definition = remoteControlDefinition(
      name,
      args,
      options.mapSnapshotCurrent?.() ?? true,
    )
    return {
      ...definition,
      admissionGuard: (
        child: FanOutChild,
        request: FanOutCommandRequest,
        context?: { exactRetry: boolean },
      ): FanOutSkipReason | null => {
        if (context?.exactRetry) return null
        if (options.mapSnapshotCurrent?.() === false)
          return {
            code: 'stale_map_scope',
            message:
              'The current map scope is stale. Refresh it before acting.',
          }
        if (!controlsCurrent.value)
          return {
            code: 'stale_live_data',
            message: 'Current target eligibility is unavailable.',
          }
        const controls = fanOut.feed.value?.targets[child.characterID]?.controls
        if (
          !controls ||
          controls.character_id !== child.characterID ||
          controls.session_id !== child.sessionID
        )
          return {
            code: 'session_changed',
            message:
              'Current capabilities for this character session are unavailable.',
          }
        const capability = controls.capabilities[request.name]
        if (!capability?.supported)
          return {
            code: capability?.reason || 'unsupported',
            message:
              capability?.reason ||
              'This session does not report support for the action.',
          }
        const mode =
          request.name === 'training.area.set' &&
          typeof request.args.mode === 'string'
            ? request.args.mode
            : ''
        if (mode && !capability.modes?.includes(mode))
          return {
            code: 'unsupported_argument_mode',
            message: `This session does not support the ${mode} mode.`,
          }
        const current = options.currentCharacter(child.characterID)
        if (!current || current.session_id !== child.sessionID)
          return {
            code: 'session_changed',
            message: 'The character session changed after preparation.',
          }
        return (
          definition.eligibility?.(
            current,
            { ...request.args },
            controls as ControlsSnapshot,
          ) || null
        )
      },
    }
  }

  function setTargets(targetIDs: string[]) {
    fanOut.setTargets([...new Set(targetIDs)])
  }

  function preview(
    targetIDs: string[],
    name: RemoteControlActionName,
    args: RemoteControlArgs = {},
    scopeKey = options.scopeKey(),
  ) {
    if (!controlsCurrent.value) return null
    const definition = definitionFor(name, args)
    return fanOut.preview(targetIDs, definition, scopeKey)
  }

  async function run(
    targetIDs: string[],
    name: RemoteControlActionName,
    args: RemoteControlArgs = {},
    scopeKey = options.scopeKey(),
  ) {
    const definition = definitionFor(name, args)
    return fanOut.prepare(targetIDs, definition, scopeKey)
  }

  async function submit(operation: Parameters<typeof fanOut.submit>[0]) {
    await fanOut.submit(operation)
  }

  async function refreshPreview(
    operation: Parameters<typeof fanOut.refreshPreview>[0],
  ) {
    return fanOut.refreshPreview(operation)
  }

  onBeforeUnmount(fanOut.dispose)
  return {
    feed: fanOut.feed,
    controlsCurrent,
    operations: fanOut.operations,
    preparing: fanOut.preparing,
    submitting: fanOut.submitting,
    error: fanOut.error,
    setTargets,
    preview,
    run,
    submit,
    refreshPreview,
    cancel: fanOut.cancel,
    dismiss: fanOut.dismiss,
    retrySubmission: fanOut.retrySubmission,
    stale: fanOut.stale,
  }
}
