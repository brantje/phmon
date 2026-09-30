import assert from 'node:assert/strict'
import test from 'node:test'
import {
  chunkFanOutValues,
  fanOutCounts,
  mergeFanOutCommand,
  prepareCommandFanOut,
  refreshPreparedCommandFanOut,
  retryFanOutSubmission,
  submitCommandFanOut,
  type FanOutCommandDefinition,
  type FanOutOperation,
  type FanOutTargetData,
} from '../app/utils/commandFanOut.ts'
import type {
  CharacterView,
  ControlsSnapshot,
  RemoteCommand,
} from '../shared/types/live.ts'

const command: FanOutCommandDefinition = {
  name: 'bot.start',
  label: 'Start bot',
  impact: 'routine',
  buildArgs: (character) => ({ region: character.region }),
}

function character(id: string, session = `${id}-session`): CharacterView {
  return {
    character_id: id,
    server: 'Greatest',
    name: `Character ${id}`,
    online: true,
    session_id: session,
    region: 42,
  }
}

function target(
  id: string,
  options: {
    current?: boolean
    supported?: boolean
    reason?: string
    scope?: string
  } = {},
): FanOutTargetData {
  const row = character(id)
  const controls: ControlsSnapshot | null =
    options.current === false
      ? null
      : {
          character_id: id,
          session_id: row.session_id!,
          capabilities: {
            'bot.start': {
              supported: options.supported ?? true,
              reason: options.reason,
            },
          },
        }
  return {
    character: row,
    controls,
    scopeKey: options.scope ?? 'Greatest:world',
    unavailableReason: options.reason,
  }
}

function prepare(
  ids: string[],
  targets: Record<string, FanOutTargetData>,
): FanOutOperation {
  let key = 0
  return prepareCommandFanOut({
    operationID: 'operation-1',
    command,
    characterIDs: ids,
    targets,
    scopeKey: 'Greatest:world',
    liveCurrent: true,
    idempotencyKey: () => `key-${++key}`,
  })
}

function dependencies(
  post: (
    request: NonNullable<FanOutOperation['children'][number]['request']>,
  ) => Promise<{ command_id: string }>,
  currentByID: Record<string, CharacterView>,
) {
  return {
    post,
    currentCharacter: (id: string) => currentByID[id],
    currentScopeKey: () => 'Greatest:world',
    changed() {},
  }
}

test('deduplicates overlapping selections in order and freezes one child request per character', () => {
  const operation = prepare(['b', 'a', 'b'], { a: target('a'), b: target('b') })
  assert.deepEqual(
    operation.children.map((child) => child.characterID),
    ['b', 'a'],
  )
  assert.deepEqual(
    operation.children.map((child) => child.idempotencyKey),
    ['key-1', 'key-2'],
  )
  assert.deepEqual(operation.children[0]?.request, {
    character_id: 'b',
    expected_session_id: 'b-session',
    name: 'bot.start',
    args: { region: 42 },
    idempotency_key: 'key-1',
    confirmation: false,
  })
  assert.equal(
    operation.children[0]?.request?.args,
    operation.children[0]?.args,
  )
  assert.equal(
    Reflect.set(
      operation.children[0]!.request!,
      'expected_session_id',
      'replacement',
    ),
    false,
  )
  assert.equal(
    Reflect.set(operation.children[0]!.request!.args, 'region', 900),
    false,
  )
})

test('projection chunking keeps every target and exact key in ordered groups of at most 100', () => {
  const values = Array.from({ length: 237 }, (_, index) => `target-${index}`)
  const chunks = chunkFanOutValues(values)
  assert.deepEqual(
    chunks.map((chunk) => chunk.length),
    [100, 100, 37],
  )
  assert.deepEqual(chunks.flat(), values)
  assert.equal(chunkFanOutValues(values.slice(0, 1)).length, 1)
  assert.deepEqual(chunkFanOutValues([]), [])
})

test('preparation gives explicit reasons for missing, stale, offline, unsupported and out-of-scope targets', () => {
  const offline = character('offline')
  offline.online = false
  const operation = prepare(
    ['missing', 'offline', 'stale', 'unsupported', 'wrong-scope'],
    {
      offline: { ...target('offline'), character: offline },
      stale: target('stale', { current: false }),
      unsupported: target('unsupported', { supported: false }),
      'wrong-scope': target('wrong-scope', { scope: 'Other:world' }),
    },
  )
  assert.deepEqual(
    operation.children.map((child) => child.skipReason?.code),
    ['not_found', 'offline', 'session_changed', 'unsupported', 'scope_changed'],
  )
  assert.equal(fanOutCounts(operation).skipped, 5)
})

test('an all-ineligible operation makes no admission requests', async () => {
  const operation = prepare(['absent'], {})
  let posts = 0
  await submitCommandFanOut(
    operation,
    dependencies(async () => {
      posts++
      return { command_id: 'unexpected' }
    }, {}),
  )
  assert.equal(posts, 0)
  assert.equal(operation.state, 'settled')
  assert.equal(fanOutCounts(operation).eligible, 0)
})

test('session replacement before submission is skipped without substituting arguments or session', async () => {
  const operation = prepare(['a'], { a: target('a') })
  let posts = 0
  await submitCommandFanOut(
    operation,
    dependencies(
      async () => {
        posts++
        return { command_id: 'unexpected' }
      },
      { a: character('a', 'replacement-session') },
    ),
  )
  assert.equal(posts, 0)
  assert.equal(operation.children[0]?.submission, 'skipped')
  assert.equal(operation.children[0]?.skipReason?.code, 'session_changed')
  assert.equal(operation.children[0]?.request?.expected_session_id, 'a-session')
})

test('review refresh retains the exact request when the frozen eligible plan is unchanged', () => {
  const operation = prepare(['a'], { a: target('a') })
  const request = operation.children[0]!.request
  const refreshed = prepare(['a'], { a: target('a') })

  assert.equal(refreshPreparedCommandFanOut(operation, refreshed), false)
  assert.equal(operation.children[0]?.request, request)
  assert.equal(operation.children[0]?.idempotencyKey, 'key-1')
  assert.equal(operation.children[0]?.submission, 'ready')
})

test('review refresh skips a replaced session and never adopts its new request', () => {
  const operation = prepare(['a'], { a: target('a') })
  const request = operation.children[0]!.request
  const replacement = character('a', 'replacement-session')
  const replacementControls = target('a').controls!
  const refreshed = prepareCommandFanOut({
    operationID: 'operation-1',
    command,
    characterIDs: ['a'],
    targets: {
      a: {
        character: replacement,
        controls: {
          ...replacementControls,
          session_id: 'replacement-session',
        },
        scopeKey: 'Greatest:world',
      },
    },
    scopeKey: 'Greatest:world',
    liveCurrent: true,
    idempotencyKey: () => 'replacement-key',
  })

  assert.equal(refreshPreparedCommandFanOut(operation, refreshed), true)
  assert.equal(operation.children[0]?.submission, 'skipped')
  assert.equal(operation.children[0]?.skipReason?.code, 'session_changed')
  assert.equal(operation.children[0]?.request, request)
  assert.equal(operation.children[0]?.request?.expected_session_id, 'a-session')
  assert.equal(operation.children[0]?.idempotencyKey, 'key-1')
  assert.equal(fanOutCounts(operation).eligible, 0)
})

test('review refresh holds a child when its frozen arguments have changed', () => {
  const operation = prepare(['a'], { a: target('a') })
  const request = operation.children[0]!.request
  const refreshed = prepareCommandFanOut({
    operationID: 'operation-1',
    command: { ...command, buildArgs: () => ({ region: 99 }) },
    characterIDs: ['a'],
    targets: { a: target('a') },
    scopeKey: 'Greatest:world',
    liveCurrent: true,
    idempotencyKey: () => 'changed-arguments-key',
  })

  assert.equal(refreshPreparedCommandFanOut(operation, refreshed), true)
  assert.equal(operation.children[0]?.submission, 'skipped')
  assert.equal(operation.children[0]?.skipReason?.code, 'arguments_changed')
  assert.equal(operation.children[0]?.request, request)
  assert.deepEqual(operation.children[0]?.args, { region: 42 })
})

test('a requested training mode requires that exact mode in current capabilities', () => {
  const trainingCommand: FanOutCommandDefinition = {
    name: 'training.area.set',
    label: 'Set training area',
    impact: 'movement',
    buildArgs: () => ({ mode: 'named', name: 'Jangan' }),
  }
  const prepared = prepareCommandFanOut({
    operationID: 'training-mode',
    command: trainingCommand,
    characterIDs: ['a'],
    targets: {
      a: {
        ...target('a'),
        controls: {
          ...target('a').controls!,
          capabilities: {
            'training.area.set': {
              supported: true,
              modes: ['current_position'],
            },
          },
        },
      },
    },
    scopeKey: 'Greatest:world',
    liveCurrent: true,
    idempotencyKey: () => 'unused',
  })
  assert.equal(prepared.children[0]?.submission, 'skipped')
  assert.equal(
    prepared.children[0]?.skipReason?.code,
    'unsupported_argument_mode',
  )
})

test('submission is bounded to four requests and rejection or uncertain siblings do not stop progress', async () => {
  const ids = Array.from({ length: 7 }, (_, index) => `c${index}`)
  const targets = Object.fromEntries(ids.map((id) => [id, target(id)]))
  const operation = prepare(ids, targets)
  const current = Object.fromEntries(ids.map((id) => [id, character(id)]))
  let active = 0
  let peak = 0
  const deps = dependencies(async (request) => {
    active++
    peak = Math.max(peak, active)
    await new Promise((resolve) => setTimeout(resolve, 2))
    active--
    if (request.character_id === 'c0')
      throw { status: 429, data: { message: 'rate limited' } }
    if (request.character_id === 'c1') throw new Error('connection lost')
    return { command_id: `cmd-${request.character_id}` }
  }, current)
  await submitCommandFanOut(operation, deps)
  assert.equal(peak, 4)
  assert.equal(operation.children[0]?.submission, 'rejected')
  assert.equal(operation.children[1]?.submission, 'uncertain')
  assert.equal(
    operation.children.filter((child) => child.submission === 'accepted')
      .length,
    5,
  )
})

test('uncertain retry reuses the exact frozen idempotency key and body', async () => {
  const operation = prepare(['a'], { a: target('a') })
  const child = operation.children[0]!
  const firstRequest = child.request!
  let calls = 0
  const current = { a: character('a') }
  await submitCommandFanOut(
    operation,
    dependencies(async () => {
      calls++
      throw new Error('response lost')
    }, current),
  )
  assert.equal(child.submission, 'uncertain')
  await retryFanOutSubmission(
    child,
    dependencies(async (request) => {
      calls++
      assert.equal(request, firstRequest)
      return { command_id: 'command-a' }
    }, current),
  )
  assert.equal(calls, 2)
  assert.equal(child.idempotencyKey, 'key-1')
  assert.equal(child.request, firstRequest)
  assert.equal(child.commandID, 'command-a')
  assert.equal(child.submission, 'accepted')
})

test('authoritative exact-key result arriving before a lost HTTP response is retained', async () => {
  const operation = prepare(['a'], { a: target('a') })
  const child = operation.children[0]!
  await submitCommandFanOut(
    operation,
    dependencies(
      async () => {
        mergeFanOutCommand(child, {
          command_id: 'command-a',
          character_id: 'a',
          session_id: 'a-session',
          idempotency_key: child.idempotencyKey,
          name: 'bot.start',
          args: { region: 42 },
          state: 'completed',
          created_at: '2026-01-01T00:00:00Z',
          expires_at: '2026-01-01T00:01:00Z',
          finished_at: '2026-01-01T00:00:01Z',
          result_code: 'api_returned',
          verification: 'api_confirmed',
          api_return: true,
        })
        throw new Error('lost response')
      },
      { a: character('a') },
    ),
  )
  assert.equal(child.submission, 'accepted')
  assert.equal(child.commandID, 'command-a')
  assert.equal(child.executionState, 'completed')
  assert.equal(child.resultCode, 'api_returned')
})

test('exact result merge rejects other sessions and does not regress a terminal result', () => {
  const operation = prepare(['a'], { a: target('a') })
  const child = operation.children[0]!
  const result = (
    session: string,
    state: RemoteCommand['state'],
  ): RemoteCommand => ({
    command_id: 'command-a',
    character_id: 'a',
    session_id: session,
    idempotency_key: child.idempotencyKey,
    name: 'bot.start',
    args: {},
    state,
    created_at: '2026-01-01T00:00:00Z',
    expires_at: '2026-01-01T00:01:00Z',
  })
  assert.equal(
    mergeFanOutCommand(child, result('replacement-session', 'completed')),
    false,
  )
  assert.equal(
    mergeFanOutCommand(child, result('a-session', 'completed')),
    true,
  )
  assert.equal(mergeFanOutCommand(child, result('a-session', 'queued')), true)
  assert.equal(child.executionState, 'completed')
})
