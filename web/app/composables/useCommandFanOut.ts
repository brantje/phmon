import type { CharacterView, ControlsSnapshot } from '~~/shared/types/live'
import { createIdempotencyKey } from '~/utils/createIdempotencyKey'
import {
  cancelCommandFanOut,
  fanOutCounts,
  mergeFanOutCommand,
  prepareCommandFanOut,
  refreshPreparedCommandFanOut,
  retryFanOutSubmission,
  submitCommandFanOut,
  type FanOutCommandDefinition,
  type FanOutOperation,
  type FanOutTargetData,
} from '~/utils/commandFanOut'

let nextOwnerNumber = 0

export interface UseCommandFanOutOptions {
  command: FanOutCommandDefinition
  scopeKey: string
  scopeKeyForCharacter(character: CharacterView, scopeKey?: string): string
  currentScopeKey(characterID: string): string
  currentCharacter(characterID: string): CharacterView | undefined
}

export function useCommandFanOut(options: UseCommandFanOutOptions) {
  const live = useLiveData()
  const ownerID = `fanout-owner-${(++nextOwnerNumber).toString(36)}`
  const operations = ref<FanOutOperation[]>([])
  const error = ref('')
  const preparing = ref(false)
  const submitting = ref(false)
  let disposed = false

  const feed = computed(() => live.commandFanOutFeeds.value[ownerID])
  const stale = computed(
    () =>
      live.liveStale.value ||
      !feed.value?.commands_current ||
      feed.value.commands_unavailable,
  )

  function notify() {
    operations.value = [...operations.value]
  }

  const stopCommandWatch = watch(
    () => feed.value?.commands,
    (commands) => {
      if (!commands) return
      for (const operation of operations.value) {
        for (const child of operation.children) {
          if (!child.idempotencyKey) continue
          const command = commands[child.idempotencyKey]
          if (command && mergeFanOutCommand(child, command)) notify()
        }
        if (
          operation.children.length > 0 &&
          operation.children.every(
            (child) =>
              child.submission === 'skipped' ||
              child.submission === 'rejected' ||
              (child.submission === 'accepted' &&
                ['completed', 'failed', 'expired'].includes(
                  child.executionState || '',
                )),
          )
        )
          operation.state = 'settled'
      }
    },
    { deep: true },
  )

  async function waitForControls(timeoutMs = 12_000) {
    if (feed.value?.controls_current || feed.value?.controls_unavailable) return
    await new Promise<void>((resolve, reject) => {
      const timer = setTimeout(() => {
        stop()
        reject(new Error('Timed out waiting for current character controls.'))
      }, timeoutMs)
      const stop = watch(
        () =>
          [
            feed.value?.controls_current,
            feed.value?.controls_unavailable,
          ] as const,
        ([current, unavailable]) => {
          if (!current && !unavailable) return
          clearTimeout(timer)
          stop()
          resolve()
        },
        { immediate: true },
      )
    })
  }

  function makePreparedOperation(
    selected: string[],
    scopeKey: string,
    operationID: string,
    command: FanOutCommandDefinition = options.command,
    operationScopeKey = scopeKey,
  ) {
    const snapshot = feed.value
    const targets: Record<string, FanOutTargetData> = {}
    for (const characterID of selected) {
      const target = snapshot?.targets[characterID]
      const character = target?.character || null
      const projectedControls = target?.controls
      const controls: ControlsSnapshot | null = projectedControls
        ? {
            ...projectedControls,
            capabilities: Object.fromEntries(
              Object.entries(projectedControls.capabilities).map(
                ([name, capability]) => [
                  name,
                  {
                    ...capability,
                    modes: capability.modes ? [...capability.modes] : undefined,
                  },
                ],
              ),
            ),
          }
        : null
      targets[characterID] = {
        character,
        controls,
        scopeKey: character
          ? options.scopeKeyForCharacter(character, operationScopeKey)
          : scopeKey,
        unavailableReason:
          target?.unavailable_reason ||
          (snapshot?.controls_unavailable
            ? 'projection_unavailable'
            : undefined),
      }
    }
    return prepareCommandFanOut({
      operationID,
      command,
      characterIDs: selected,
      targets,
      scopeKey,
      liveCurrent:
        Boolean(snapshot?.controls_current) &&
        live.connectionState.value === 'current' &&
        !live.liveStale.value,
      idempotencyKey: createIdempotencyKey,
    })
  }

  async function prepare(
    characterIDs: string[],
    command: FanOutCommandDefinition = options.command,
    operationScopeKey = options.scopeKey,
  ) {
    if (disposed || preparing.value || submitting.value) return null
    error.value = ''
    preparing.value = true
    const selected = [...new Set(characterIDs)]
    live.setCommandFanOutTargets(ownerID, selected)
    try {
      await waitForControls()
      if (disposed) return null
      const operation = makePreparedOperation(
        selected,
        operationScopeKey,
        `${ownerID}-op-${Date.now().toString(36)}-${operations.value.length.toString(36)}`,
        command,
        operationScopeKey,
      )
      operations.value = [operation, ...operations.value]
      notify()
      return operation
    } catch (failure) {
      error.value =
        failure instanceof Error
          ? failure.message
          : 'Could not prepare character actions.'
      return null
    } finally {
      preparing.value = false
    }
  }

  async function refreshPreview(operation: FanOutOperation) {
    if (
      disposed ||
      preparing.value ||
      submitting.value ||
      operation.state !== 'prepared'
    )
      return false
    preparing.value = true
    error.value = ''
    try {
      live.refreshCommandFanOutTargets(ownerID)
      await waitForControls()
      if (disposed) return false
      const refreshed = makePreparedOperation(
        operation.children.map((child) => child.characterID),
        operation.scopeKey,
        operation.operationID,
        operation.command,
        options.scopeKey,
      )
      const changed = refreshPreparedCommandFanOut(operation, refreshed)
      notify()
      return changed
    } catch (failure) {
      error.value =
        failure instanceof Error
          ? failure.message
          : 'Could not refresh current target eligibility.'
      return true
    } finally {
      preparing.value = false
    }
  }

  async function submit(operation: FanOutOperation) {
    if (disposed || submitting.value || operation.state !== 'prepared') return
    submitting.value = true
    live.trackCommandFanOut(
      ownerID,
      operations.value.flatMap((item) =>
        item.children.flatMap((child) =>
          child.idempotencyKey ? [child.idempotencyKey] : [],
        ),
      ),
    )
    const dependencies = {
      post: (
        request: NonNullable<FanOutOperation['children'][number]['request']>,
      ) =>
        $fetch<{ command_id: string; state?: string; duplicate?: boolean }>(
          '/api/commands',
          { method: 'POST', body: request, retry: 0, timeout: 4_000 },
        ),
      currentCharacter: options.currentCharacter,
      currentScopeKey: options.currentScopeKey,
      changed: notify,
    }
    try {
      await submitCommandFanOut(operation, dependencies)
    } finally {
      submitting.value = false
      notify()
    }
  }

  async function retrySubmission(
    operation: FanOutOperation,
    characterID: string,
  ) {
    if (disposed || submitting.value) return
    const child = operation.children.find(
      (item) => item.characterID === characterID,
    )
    if (!child) return
    submitting.value = true
    const dependencies = {
      post: (request: NonNullable<typeof child.request>) =>
        $fetch<{ command_id: string; state?: string; duplicate?: boolean }>(
          '/api/commands',
          { method: 'POST', body: request, retry: 0, timeout: 4_000 },
        ),
      currentCharacter: options.currentCharacter,
      currentScopeKey: options.currentScopeKey,
      changed: notify,
    }
    try {
      await retryFanOutSubmission(child, dependencies, operation.command)
      operation.state =
        child.submission === 'accepted' ? 'tracking' : operation.state
    } finally {
      submitting.value = false
      notify()
    }
  }

  function cancel(operation: FanOutOperation) {
    const cancelled = cancelCommandFanOut(operation)
    if (cancelled) notify()
    return cancelled
  }

  function dismiss(operation: FanOutOperation) {
    if (operation.state === 'submitting') return
    operations.value = operations.value.filter((item) => item !== operation)
    live.trackCommandFanOut(
      ownerID,
      operations.value.flatMap((item) =>
        item.children.flatMap((child) =>
          child.idempotencyKey ? [child.idempotencyKey] : [],
        ),
      ),
    )
  }

  function dispose() {
    if (disposed) return
    disposed = true
    stopCommandWatch()
    live.clearCommandFanOutOwner(ownerID)
  }

  return {
    ownerID,
    operations: computed(() => operations.value),
    feed,
    stale,
    error: readonly(error),
    preparing: readonly(preparing),
    submitting: readonly(submitting),
    prepare,
    refreshPreview,
    submit,
    cancel,
    retrySubmission,
    dismiss,
    counts: fanOutCounts,
    dispose,
  }
}
