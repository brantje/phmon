import type {
  CharacterView,
  ControlsSnapshot,
  RemoteCommand,
} from '~~/shared/types/live'

export type FanOutCommandName =
  | 'bot.start'
  | 'bot.stop'
  | 'trace.start'
  | 'trace.stop'
  | 'training.area.set'
  | 'training.radius.set'
  | 'character.walk'
  | 'character.navigate'
  | 'character.return'
  | 'character.disconnect'
  | 'client.clientless'

export type FanOutImpact = 'routine' | 'movement' | 'disruptive'
export type FanOutSubmissionState =
  'ready' | 'submitting' | 'accepted' | 'rejected' | 'uncertain' | 'skipped'

export interface FanOutSkipReason {
  code: string
  message: string
}

export interface FanOutTargetData {
  character: CharacterView | null
  controls: ControlsSnapshot | null
  scopeKey: string
  unavailableReason?: string
}

export interface FanOutCommandDefinition {
  name: FanOutCommandName
  label: string
  impact: FanOutImpact
  buildArgs(character: CharacterView): Record<string, unknown>
  summarizeArgs?(args: Record<string, unknown>): string
  preEligibility?(character: CharacterView): FanOutSkipReason | null
  eligibility?(
    character: CharacterView,
    args: Record<string, unknown>,
  ): FanOutSkipReason | null
  admissionGuard?(
    child: FanOutChild,
    request: FanOutCommandRequest,
    context?: { exactRetry: boolean },
  ): FanOutSkipReason | null | Promise<FanOutSkipReason | null>
}

export interface FanOutCommandRequest {
  readonly character_id: string
  readonly expected_session_id: string
  readonly name: FanOutCommandName
  readonly args: Readonly<Record<string, unknown>>
  readonly idempotency_key: string
  readonly confirmation: boolean
}

export interface FanOutChild {
  readonly characterID: string
  readonly characterName: string
  readonly server: string
  readonly sessionID: string
  readonly scopeKey: string
  readonly args?: Readonly<Record<string, unknown>>
  argsSummary?: string
  readonly idempotencyKey?: string
  readonly request?: FanOutCommandRequest
  submission: FanOutSubmissionState
  skipReason?: FanOutSkipReason
  message?: string
  commandID?: string
  executionState?: RemoteCommand['state']
  resultCode?: string
  finishedAt?: string
  verification?: RemoteCommand['verification']
  apiReturn?: unknown
  effectiveArgs?: RemoteCommand['effective_args']
  observedAfter?: RemoteCommand['observed_after']
}

export interface FanOutOperation {
  operationID: string
  command: FanOutCommandDefinition
  scopeKey: string
  selectedCount: number
  children: FanOutChild[]
  state: 'prepared' | 'submitting' | 'tracking' | 'settled' | 'cancelled'
}

export interface PrepareFanOutInput {
  operationID: string
  command: FanOutCommandDefinition
  characterIDs: string[]
  targets: Record<string, FanOutTargetData>
  scopeKey: string
  liveCurrent: boolean
  idempotencyKey(): string
}

function deepFreeze<T>(value: T): T {
  if (!value || typeof value !== 'object' || Object.isFrozen(value))
    return value
  for (const nested of Object.values(value as Record<string, unknown>))
    deepFreeze(nested)
  return Object.freeze(value)
}

const notFoundReason: FanOutSkipReason = {
  code: 'not_found',
  message: 'Character record was not found.',
}
const offlineReason: FanOutSkipReason = {
  code: 'offline',
  message: 'Character is offline.',
}
const sessionChangedReason: FanOutSkipReason = {
  code: 'session_changed',
  message: 'Character session changed during preparation.',
}
const capabilityMessages: Record<string, string> = {
  plugin_upgrade_required:
    'Reload the current PhMon plugin to report command capabilities.',
  capabilities_pending:
    'Waiting for this session to report command capabilities.',
  unsupported_runtime_primitive:
    'This phBot runtime does not expose the required action.',
  unsupported: 'This session does not report support for the action.',
}

function cloneArgs(value: Record<string, unknown>) {
  const json = JSON.stringify(value)
  if (!json) throw new TypeError('Command arguments must be a JSON object.')
  const copy: unknown = JSON.parse(json)
  if (!copy || typeof copy !== 'object' || Array.isArray(copy))
    throw new TypeError('Command arguments must be a JSON object.')
  return copy as Record<string, unknown>
}

function missingTarget(characterID: string, scopeKey: string): FanOutChild {
  return {
    characterID,
    characterName: characterID,
    server: '',
    sessionID: '',
    scopeKey,
    submission: 'skipped',
    skipReason: notFoundReason,
  }
}

function skip(
  character: CharacterView,
  scopeKey: string,
  reason: FanOutSkipReason,
): FanOutChild {
  return {
    characterID: character.character_id,
    characterName: character.name,
    server: character.server,
    sessionID: character.session_id || '',
    scopeKey,
    submission: 'skipped',
    skipReason: reason,
  }
}

function needsIntentFlag(name: FanOutCommandName) {
  return [
    'character.return',
    'character.disconnect',
    'client.clientless',
  ].includes(name)
}

function freezeChildTarget(child: FanOutChild) {
  if (child.args) deepFreeze(child.args)
  if (child.request) deepFreeze(child.request)
  for (const key of [
    'characterID',
    'characterName',
    'server',
    'sessionID',
    'scopeKey',
    'args',
    'idempotencyKey',
    'request',
  ]) {
    Object.defineProperty(child, key, { writable: false, configurable: false })
  }
  return child
}

export function prepareCommandFanOut(
  input: PrepareFanOutInput,
): FanOutOperation {
  const characterIDs = [...new Set(input.characterIDs)]
  const children: FanOutChild[] = []
  const command = Object.freeze({ ...input.command })

  for (const characterID of characterIDs) {
    const target = input.targets[characterID]
    const character = target?.character
    if (!character) {
      const absent = missingTarget(characterID, input.scopeKey)
      if (target?.unavailableReason) {
        absent.skipReason = {
          code: target.unavailableReason,
          message:
            target.unavailableReason === 'projection_unavailable'
              ? 'Current eligibility data is unavailable. No command was submitted.'
              : target.unavailableReason === 'offline'
                ? offlineReason.message
                : target.unavailableReason === 'session_changed'
                  ? sessionChangedReason.message
                  : target.unavailableReason === 'not_found'
                    ? notFoundReason.message
                    : 'Character data is unavailable for this action.',
        }
      }
      children.push(freezeChildTarget(absent))
      continue
    }
    if (character.character_id !== characterID) {
      children.push(
        freezeChildTarget(
          skip(character, input.scopeKey, {
            code: 'target_identity_mismatch',
            message: 'Live target data did not match the selected character.',
          }),
        ),
      )
      continue
    }

    const reject = (reason: FanOutSkipReason) =>
      children.push(freezeChildTarget(skip(character, input.scopeKey, reason)))
    if (target.scopeKey !== input.scopeKey) {
      reject({
        code: 'scope_changed',
        message: 'Character is outside the current action scope.',
      })
      continue
    }
    const sessionID = character.session_id
    if (!character.online || !sessionID) {
      reject(offlineReason)
      continue
    }
    if (!input.liveCurrent) {
      reject({
        code: 'stale_live_data',
        message: 'Live target or capability data is stale.',
      })
      continue
    }
    const controls = target.controls
    if (
      !controls ||
      controls.character_id !== character.character_id ||
      controls.session_id !== character.session_id
    ) {
      reject({
        code: target.unavailableReason || 'session_changed',
        message:
          target.unavailableReason === 'offline'
            ? offlineReason.message
            : 'Current capabilities for this character session are unavailable.',
      })
      continue
    }
    const capability = controls.capabilities[input.command.name]
    if (!capability?.supported) {
      reject({
        code: capability?.reason || 'unsupported',
        message:
          capabilityMessages[capability?.reason || ''] ||
          capability?.reason ||
          'This session does not report support for the action.',
      })
      continue
    }

    const beforeArguments = command.preEligibility?.(character)
    if (beforeArguments) {
      reject(beforeArguments)
      continue
    }

    let args: Record<string, unknown>
    const invocationCharacter = Object.freeze({ ...character })
    try {
      args = cloneArgs(command.buildArgs(invocationCharacter))
    } catch (error) {
      reject({
        code: 'invalid_arguments',
        message:
          error instanceof Error
            ? error.message
            : 'Command arguments are invalid.',
      })
      continue
    }

    const requestedMode =
      command.name === 'training.area.set' ? args.mode : undefined
    if (
      typeof requestedMode === 'string' &&
      !capability.modes?.includes(requestedMode)
    ) {
      reject({
        code: 'unsupported_argument_mode',
        message: `This session does not support the ${requestedMode} mode.`,
      })
      continue
    }

    const actionReason = command.eligibility?.(invocationCharacter, args)
    if (actionReason) {
      reject(actionReason)
      continue
    }

    const idempotencyKey = input.idempotencyKey()
    const request: FanOutCommandRequest = {
      character_id: character.character_id,
      expected_session_id: sessionID,
      name: command.name,
      args,
      idempotency_key: idempotencyKey,
      confirmation: needsIntentFlag(command.name),
    }
    children.push(
      freezeChildTarget({
        characterID: character.character_id,
        characterName: character.name,
        server: character.server,
        sessionID: character.session_id,
        scopeKey: input.scopeKey,
        args,
        argsSummary: command.summarizeArgs?.(args),
        idempotencyKey,
        request,
        submission: 'ready',
      }),
    )
  }

  return {
    operationID: input.operationID,
    command,
    scopeKey: input.scopeKey,
    selectedCount: characterIDs.length,
    children,
    state: 'prepared',
  }
}

/**
 * Reconcile an open review against a newly read controls snapshot. Existing
 * eligible children keep their exact frozen request. A changed session,
 * capability or argument plan becomes skipped and must be reviewed again;
 * targets that were previously unavailable can become eligible only if their
 * previously observed session still matches.
 */
export function refreshPreparedCommandFanOut(
  previous: FanOutOperation,
  refreshed: FanOutOperation,
) {
  const before = JSON.stringify(
    previous.children.map((child) => [
      child.characterID,
      child.characterName,
      child.sessionID,
      child.scopeKey,
      child.submission,
      child.skipReason?.code,
      child.args,
    ]),
  )
  const freshByID = new Map(
    refreshed.children.map((child) => [child.characterID, child]),
  )
  const children = previous.children.map((child) => {
    const fresh = freshByID.get(child.characterID)
    if (!fresh) return child

    if (child.sessionID && child.sessionID !== fresh.sessionID) {
      if (child.submission === 'ready' || child.submission === 'skipped') {
        child.submission = 'skipped'
        child.skipReason = sessionChangedReason
      }
      return child
    }

    if (child.scopeKey !== fresh.scopeKey) {
      if (child.submission === 'ready' || child.submission === 'skipped') {
        child.submission = 'skipped'
        child.skipReason = {
          code: 'scope_changed',
          message:
            'The map server, area, floor, region or dataset changed after preparation.',
        }
      }
      return child
    }

    if (child.request) {
      const sameArguments =
        JSON.stringify(child.args) === JSON.stringify(fresh.args)
      if (
        fresh.request &&
        fresh.sessionID === child.sessionID &&
        sameArguments
      ) {
        child.submission = 'ready'
        child.skipReason = undefined
        child.message = undefined
      } else {
        child.submission = 'skipped'
        child.skipReason = fresh.request
          ? {
              code: 'arguments_changed',
              message:
                'Command arguments changed during review. Start a new action to review the updated arguments.',
            }
          : (fresh.skipReason ?? {
              code: 'eligibility_changed',
              message: 'Eligibility changed during review.',
            })
      }
      return child
    }

    if (fresh.request) return fresh
    child.skipReason = fresh.skipReason
    child.message = fresh.message
    return child
  })
  const after = JSON.stringify(
    children.map((child) => [
      child.characterID,
      child.characterName,
      child.sessionID,
      child.scopeKey,
      child.submission,
      child.skipReason?.code,
      child.args,
    ]),
  )
  previous.children = children
  previous.state = 'prepared'
  return before !== after
}

export function fanOutCounts(operation: FanOutOperation) {
  const counts = {
    selected: operation.selectedCount,
    eligible: operation.children.filter(
      (child) => child.request && child.submission !== 'skipped',
    ).length,
    skipped: 0,
    awaitingSubmission: 0,
    accepted: 0,
    rejected: 0,
    uncertain: 0,
    queued: 0,
    inProgress: 0,
    completed: 0,
    failed: 0,
    expired: 0,
    unknown: 0,
  }
  for (const child of operation.children) {
    if (child.submission === 'skipped') counts.skipped++
    if (child.submission === 'ready' || child.submission === 'submitting')
      counts.awaitingSubmission++
    if (child.submission === 'accepted') counts.accepted++
    if (child.submission === 'rejected') counts.rejected++
    if (child.submission === 'uncertain') counts.uncertain++
    if (child.executionState === 'queued') counts.queued++
    if (
      ['dispatching', 'sent', 'acknowledged'].includes(
        child.executionState || '',
      )
    )
      counts.inProgress++
    if (child.executionState === 'completed') counts.completed++
    if (child.executionState === 'failed') counts.failed++
    if (child.executionState === 'expired') counts.expired++
    if (child.executionState === 'unknown') counts.unknown++
  }
  return counts
}

export function chunkFanOutValues(values: string[], size = 100) {
  if (!Number.isSafeInteger(size) || size < 1)
    throw new RangeError('Fan-out projection chunk size must be positive.')
  const chunks: string[][] = []
  for (let offset = 0; offset < values.length; offset += size)
    chunks.push(values.slice(offset, offset + size))
  return chunks
}

export interface FanOutSubmitResponse {
  command_id: string
  state?: string
  duplicate?: boolean
}

export interface FanOutSubmitDependencies {
  post(request: FanOutCommandRequest): Promise<FanOutSubmitResponse>
  currentCharacter(characterID: string): CharacterView | undefined
  currentScopeKey(characterID: string): string
  changed(): void
  concurrency?: number
}

function currentTargetReason(
  child: FanOutChild,
  dependencies: FanOutSubmitDependencies,
): FanOutSkipReason | null {
  const current = dependencies.currentCharacter(child.characterID)
  if (!current) return notFoundReason
  if (!current.online || !current.session_id) return offlineReason
  if (current.session_id !== child.sessionID) return sessionChangedReason
  if (dependencies.currentScopeKey(child.characterID) !== child.scopeKey)
    return {
      code: 'scope_changed',
      message: 'The action scope changed after preparation.',
    }
  return null
}

async function submitChild(
  child: FanOutChild,
  command: FanOutCommandDefinition | undefined,
  dependencies: FanOutSubmitDependencies,
) {
  const changed = currentTargetReason(child, dependencies)
  if (changed) {
    child.submission = 'skipped'
    child.skipReason = changed
    dependencies.changed()
    return
  }
  if (!child.request) {
    child.submission = 'rejected'
    child.message = 'Frozen command request is unavailable.'
    dependencies.changed()
    return
  }
  let admissionReason: FanOutSkipReason | null | undefined
  try {
    admissionReason = await command?.admissionGuard?.(child, child.request, {
      exactRetry: false,
    })
  } catch (error) {
    child.submission = 'skipped'
    child.skipReason = {
      code: 'admission_check_failed',
      message:
        error instanceof Error
          ? error.message
          : 'Could not verify current action eligibility.',
    }
    dependencies.changed()
    return
  }
  if (admissionReason) {
    child.submission = 'skipped'
    child.skipReason = admissionReason
    dependencies.changed()
    return
  }
  child.submission = 'submitting'
  dependencies.changed()
  try {
    const response = await dependencies.post(child.request)
    if (
      !response ||
      typeof response.command_id !== 'string' ||
      !response.command_id
    ) {
      child.submission = 'uncertain'
      child.message = 'The server response did not include a command ID.'
    } else {
      if (child.commandID && child.commandID !== response.command_id) {
        child.submission = 'uncertain'
        child.message =
          'The accepted command ID did not match the live command result.'
        dependencies.changed()
        return
      }
      child.commandID = response.command_id
      child.submission = 'accepted'
      child.message = undefined
    }
  } catch (error) {
    if (child.commandID || child.executionState) {
      child.submission = 'accepted'
      dependencies.changed()
      return
    }
    const failure = error as {
      status?: number
      statusCode?: number
      response?: { status?: number }
      data?: { message?: string; error?: string }
    }
    const status =
      failure.response?.status || failure.statusCode || failure.status || 0
    child.submission = status >= 400 && status < 500 ? 'rejected' : 'uncertain'
    child.message =
      failure.data?.message ||
      failure.data?.error ||
      (child.submission === 'uncertain'
        ? 'The submission outcome is unknown. Retry this exact request to reconcile safely.'
        : 'The server rejected this character command.')
  }
  dependencies.changed()
}

export async function submitCommandFanOut(
  operation: FanOutOperation,
  dependencies: FanOutSubmitDependencies,
) {
  if (operation.state !== 'prepared') return
  operation.state = 'submitting'
  dependencies.changed()
  const ready = operation.children.filter(
    (child) => child.submission === 'ready',
  )
  let next = 0
  const concurrency = Math.max(
    1,
    Math.min(4, Math.floor(dependencies.concurrency || 4)),
  )
  await Promise.all(
    Array.from({ length: Math.min(concurrency, ready.length) }, async () => {
      while (next < ready.length) {
        const child = ready[next++]!
        await submitChild(child, operation.command, dependencies)
      }
    }),
  )
  operation.state = operation.children.some(
    (child) =>
      child.submission === 'accepted' || child.submission === 'uncertain',
  )
    ? 'tracking'
    : 'settled'
  dependencies.changed()
}

export async function retryFanOutSubmission(
  child: FanOutChild,
  dependencies: FanOutSubmitDependencies,
  command?: FanOutCommandDefinition,
) {
  if (child.submission !== 'uncertain' || !child.request) return
  const changed = currentTargetReason(child, dependencies)
  if (changed) {
    child.message = `Retry was not sent. ${changed.message} The original outcome is still being checked.`
    dependencies.changed()
    return
  }
  let admissionReason: FanOutSkipReason | null | undefined
  try {
    admissionReason = command?.admissionGuard
      ? await command.admissionGuard(child, child.request, { exactRetry: true })
      : null
  } catch (error) {
    child.message = `Retry was not sent. ${error instanceof Error ? error.message : 'Current eligibility could not be checked.'} The original outcome is still being checked.`
    dependencies.changed()
    return
  }
  if (admissionReason) {
    child.message = `Retry was not sent. ${admissionReason.message} The original outcome is still being checked.`
    dependencies.changed()
    return
  }
  await submitChild(child, undefined, dependencies)
}

export function cancelCommandFanOut(operation: FanOutOperation) {
  if (operation.state !== 'prepared') return false
  operation.state = 'cancelled'
  for (const child of operation.children) {
    if (child.submission === 'ready') {
      child.submission = 'skipped'
      child.skipReason = {
        code: 'cancelled',
        message: 'The operator cancelled before submission.',
      }
    }
  }
  return true
}

export function mergeFanOutCommand(child: FanOutChild, command: RemoteCommand) {
  if (
    command.character_id !== child.characterID ||
    command.session_id !== child.sessionID ||
    command.idempotency_key !== child.idempotencyKey
  )
    return false
  if (child.commandID && child.commandID !== command.command_id) return false
  child.commandID = command.command_id
  child.submission = 'accepted'
  if (
    !child.executionState ||
    [
      'queued',
      'dispatching',
      'sent',
      'acknowledged',
      'unknown',
      'completed',
      'failed',
      'expired',
    ].indexOf(command.state) >=
      [
        'queued',
        'dispatching',
        'sent',
        'acknowledged',
        'unknown',
        'completed',
        'failed',
        'expired',
      ].indexOf(child.executionState)
  ) {
    child.executionState = command.state
    child.message = command.message
    child.resultCode = command.result_code
    child.finishedAt = command.finished_at
    child.verification = command.verification
    child.apiReturn = command.api_return
    child.effectiveArgs = command.effective_args
    child.observedAfter = command.observed_after
  }
  return true
}
