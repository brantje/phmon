import assert from 'node:assert/strict'
import test from 'node:test'
import type { CharacterView, MapNpc } from '../shared/types/live.ts'
import { prepareCommandFanOut } from '../app/utils/commandFanOut.ts'
import {
  createMapRecallPointIntent,
  mapRecallPointCommand,
} from '../app/utils/mapRecallPointAction.ts'

const gate: MapNpc = {
  id: 'gate-hotan',
  name: 'Hotan',
  servername: 'GATE_KT',
  model_id: 2094,
  role: 'teleporter',
  region: 25000,
  x: 30,
  y: 40,
  observed_at: '2026-10-04T12:00:00Z',
  observers: [{ character_id: 'char-a', session_id: 'sess-a', name: 'Alpha' }],
}
const character: CharacterView = {
  character_id: 'char-a',
  session_id: 'sess-a',
  name: 'Alpha',
  server: 'Greatest',
  online: true,
}

function setup(supported = true) {
  const intent = createMapRecallPointIntent({
    server: 'Greatest',
    gate,
    targetIDs: ['char-a'],
  })
  assert.ok(intent)
  let currentGate: MapNpc | null = gate
  let currentCharacter = character
  const controls = {
    character_id: 'char-a',
    session_id: 'sess-a',
    capabilities: {
      'character.recall_point.designate': {
        supported,
        reason: supported ? undefined : 'recall_point_unverified',
      },
    },
  }
  const command = mapRecallPointCommand({
    getIntent: () => intent,
    getCurrentGate: () => currentGate,
    getCharacter: () => currentCharacter,
    getControls: () => controls,
    mapFeedCurrent: () => true,
  })
  return {
    command,
    controls,
    setGate: (value: MapNpc | null) => (currentGate = value),
    setCharacter: (value: CharacterView) => (currentCharacter = value),
  }
}

test('recall point intent requires a bounded live teleporter and deduplicates targets', () => {
  assert.deepEqual(
    createMapRecallPointIntent({
      server: 'Greatest',
      gate,
      targetIDs: ['char-a', 'char-a'],
    })?.targetIDs,
    ['char-a'],
  )
  assert.equal(
    createMapRecallPointIntent({
      server: 'Greatest',
      gate: { ...gate, servername: 'NPC_KT' },
      targetIDs: ['char-a'],
    }),
    null,
  )
})

test('recall point request uses fixed gate fields and requires explicit confirmation', () => {
  const { command, controls } = setup()
  const operation = prepareCommandFanOut({
    operationID: 'recall-1',
    command,
    characterIDs: ['char-a'],
    targets: {
      'char-a': { character, controls, scopeKey: 'greatest:gate-hotan' },
    },
    scopeKey: 'greatest:gate-hotan',
    liveCurrent: true,
    idempotencyKey: () => 'recall-key-1',
  })
  const child = operation.children[0]!
  assert.equal(child.submission, 'ready')
  assert.deepEqual(child.request?.args, {
    gate_servername: 'GATE_KT',
    region: 25000,
    x: 30,
    y: 40,
    model_id: 2094,
  })
  assert.equal(child.request?.confirmation, true)
})

test('unverified capability is skipped and sends no request', () => {
  const { command, controls } = setup(false)
  const operation = prepareCommandFanOut({
    operationID: 'recall-2',
    command,
    characterIDs: ['char-a'],
    targets: {
      'char-a': { character, controls, scopeKey: 'greatest:gate-hotan' },
    },
    scopeKey: 'greatest:gate-hotan',
    liveCurrent: true,
    idempotencyKey: () => 'recall-key-2',
  })
  assert.equal(operation.children[0]?.submission, 'skipped')
  assert.equal(operation.children[0]?.request, undefined)
  assert.match(
    operation.children[0]?.skipReason?.message || '',
    /awaiting validation/i,
  )
})

test('admission rejects changed gate, character session, and injected arguments', async () => {
  const state = setup()
  const args = state.command.buildArgs(character)
  const child = {
    characterID: 'char-a',
    characterName: 'Alpha',
    sessionID: 'sess-a',
    submission: 'ready' as const,
  }
  const request = {
    character_id: 'char-a',
    expected_session_id: 'sess-a',
    args,
  }
  assert.equal(await state.command.admissionGuard?.(child, request, {}), null)
  assert.equal(
    (
      await state.command.admissionGuard?.(
        child,
        { ...request, args: { ...args, opcode: 28761 } },
        {},
      )
    )?.code,
    'arguments_changed',
  )
  state.setGate({ ...gate, x: 31 })
  assert.equal(
    (await state.command.admissionGuard?.(child, request, {}))?.code,
    'gate_changed',
  )
  state.setGate(gate)
  state.setCharacter({ ...character, session_id: 'sess-b' })
  assert.equal(
    (await state.command.admissionGuard?.(child, request, {}))?.code,
    'session_changed',
  )
})
