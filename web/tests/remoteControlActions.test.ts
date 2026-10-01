import assert from 'node:assert/strict'
import test from 'node:test'
import {
  remoteControlDefinition,
  validateRemoteControlArgs,
  type RemoteControlActionName,
} from '../app/utils/remoteControlActions.ts'
import type { CharacterView, ControlsSnapshot } from '../shared/types/live.ts'
import { prepareCommandFanOut } from '../app/utils/commandFanOut.ts'

const actionNames: RemoteControlActionName[] = [
  'bot.start',
  'bot.stop',
  'trace.start',
  'trace.stop',
  'character.return',
  'character.disconnect',
  'client.clientless',
  'training.area.set',
  'training.radius.set',
]

const character: CharacterView = {
  character_id: 'character-one',
  name: 'Alpha',
  server: 'Greatest',
  online: true,
  session_id: 'session-one',
  region: 25273,
  x: 10,
  y: 20,
  z: 30,
}

function controls(
  name: RemoteControlActionName,
  modes?: string[],
  training?: ControlsSnapshot['training'],
): ControlsSnapshot {
  return {
    character_id: character.character_id,
    session_id: character.session_id!,
    capabilities: {
      [name]: { supported: true, modes },
      ...(name === 'training.area.set'
        ? { 'training.radius.set': { supported: true } }
        : {}),
    },
    training,
  }
}

function prepare(
  name: RemoteControlActionName,
  args: Parameters<typeof remoteControlDefinition>[1] = {},
  targetControls = controls(name),
) {
  const definition = remoteControlDefinition(name, args)
  return prepareCommandFanOut({
    operationID: 'remote-action-test',
    command: definition,
    characterIDs: [character.character_id],
    targets: {
      [character.character_id]: {
        character,
        controls: targetControls,
        scopeKey: 'Greatest',
      },
    },
    scopeKey: 'Greatest',
    liveCurrent: true,
    idempotencyKey: () => 'key-one',
  })
}

test('remote-control catalog contains only the nine intended existing commands', () => {
  assert.deepEqual(
    actionNames.map((name) => remoteControlDefinition(name).name),
    actionNames,
  )
  assert.deepEqual(
    actionNames.map((name) => remoteControlDefinition(name).label),
    [
      'Start Training',
      'Stop Training',
      'Start Trace',
      'Stop Trace',
      'Return Scroll',
      'Disconnect',
      'Go Clientless',
      'Set Training Area',
      'Set Training Radius',
    ],
  )
})

test('intent flags remain true for return, disconnect and clientless', () => {
  for (const name of [
    'character.return',
    'character.disconnect',
    'client.clientless',
  ] as const) {
    assert.equal(prepare(name).children[0]?.request?.confirmation, true)
  }
  for (const name of ['bot.start', 'bot.stop', 'trace.stop'] as const) {
    assert.equal(prepare(name).children[0]?.request?.confirmation, false)
  }
})

test('trace and named-area arguments use trimmed UTF-8 byte bounds and reject NUL', () => {
  assert.deepEqual(
    validateRemoteControlArgs('trace.start', { traceName: '  Target  ' }),
    { traceName: 'Target' },
  )
  assert.equal(
    validateRemoteControlArgs('trace.start', { traceName: 'é'.repeat(32) })
      ?.traceName?.length,
    32,
  )
  assert.equal(
    validateRemoteControlArgs('trace.start', { traceName: 'é'.repeat(33) }),
    null,
  )
  assert.equal(
    validateRemoteControlArgs('trace.start', { traceName: ' bad\0name ' }),
    null,
  )
  assert.deepEqual(
    validateRemoteControlArgs('training.area.set', {
      trainingAreaMode: 'named',
      trainingAreaName: '  Jangan  ',
    }),
    { trainingAreaMode: 'named', trainingAreaName: 'Jangan' },
  )
  assert.equal(
    validateRemoteControlArgs('training.area.set', {
      trainingAreaMode: 'named',
      trainingAreaName: 'é'.repeat(51),
    }),
    null,
  )
  assert.equal(
    validateRemoteControlArgs('training.area.set', {
      trainingAreaMode: 'named',
      trainingAreaName: 'bad\0name',
    }),
    null,
  )
})

test('radius rejects blanks, non-finite values and values outside the documented safety range', () => {
  for (const trainingRadius of ['', ' ', '0', '10000.01', 'NaN', 'Infinity'])
    assert.equal(
      validateRemoteControlArgs('training.radius.set', { trainingRadius }),
      null,
    )
  assert.deepEqual(
    validateRemoteControlArgs('training.radius.set', {
      trainingRadius: ' 50.5 ',
    }),
    { trainingRadius: '50.5' },
  )
  assert.deepEqual(
    validateRemoteControlArgs('training.radius.set', { trainingRadius: '1' }),
    { trainingRadius: '1' },
  )
  assert.deepEqual(
    validateRemoteControlArgs('training.radius.set', {
      trainingRadius: '10000',
    }),
    { trainingRadius: '10000' },
  )
})

test('current-position mode sends no focused-character coordinates', () => {
  const result = prepare(
    'training.area.set',
    { trainingAreaMode: 'current_position' },
    controls('training.area.set', ['current_position']),
  )
  assert.deepEqual(result.children[0]?.request?.args, {
    mode: 'current_position',
  })
})

test('training-area readback blocks radius/current-position only when the getter confirms no area', () => {
  const noArea = {
    session_id: character.session_id!,
    training_available: false,
    observed_at: '2026-10-01T00:00:00Z',
  }
  const radius = prepare(
    'training.radius.set',
    { trainingRadius: '50' },
    controls('training.radius.set', undefined, noArea),
  )
  assert.equal(radius.children[0]?.skipReason?.code, 'no_training_area')

  const currentPosition = prepare(
    'training.area.set',
    { trainingAreaMode: 'current_position' },
    controls('training.area.set', ['current_position'], noArea),
  )
  assert.equal(
    currentPosition.children[0]?.skipReason?.code,
    'no_training_area',
  )

  const named = prepare(
    'training.area.set',
    { trainingAreaMode: 'named', trainingAreaName: 'Jangan' },
    controls('training.area.set', ['named'], noArea),
  )
  assert.equal(named.children[0]?.skipReason, undefined)
  assert.deepEqual(named.children[0]?.request?.args, {
    mode: 'named',
    name: 'Jangan',
  })
})

test('unavailable active areas require matching observed readback; missing readback stays explicitly unconfirmed', () => {
  const withoutRadiusSetter = controls(
    'training.area.set',
    ['current_position'],
    {
      session_id: character.session_id!,
      training_available: false,
      observed_at: '2026-10-01T00:00:00Z',
    },
  )
  delete withoutRadiusSetter.capabilities['training.radius.set']
  const unavailable = prepare(
    'training.area.set',
    { trainingAreaMode: 'current_position' },
    withoutRadiusSetter,
  )
  assert.equal(unavailable.children[0]?.skipReason?.code, 'no_training_area')

  const missingReadback = prepare(
    'training.area.set',
    { trainingAreaMode: 'current_position' },
    controls('training.area.set', ['current_position'], {
      session_id: character.session_id!,
      training_available: false,
    }),
  )
  assert.equal(missingReadback.children[0]?.skipReason, undefined)
  assert.match(
    missingReadback.children[0]?.argsSummary || '',
    /readback unavailable; active area is unconfirmed/,
  )

  const radiusMissingReadback = prepare(
    'training.radius.set',
    { trainingRadius: '50' },
    controls('training.radius.set'),
  )
  assert.equal(radiusMissingReadback.children[0]?.skipReason, undefined)
  assert.match(
    radiusMissingReadback.children[0]?.argsSummary || '',
    /readback unavailable; active area is unconfirmed/,
  )
})

test('clientless remains capability-gated and does not invent runtime support', () => {
  const unsupported = controls('client.clientless')
  unsupported.capabilities['client.clientless'] = {
    supported: false,
    reason: 'unsupported_runtime_primitive',
  }
  const result = prepare('client.clientless', {}, unsupported)
  assert.equal(
    result.children[0]?.skipReason?.code,
    'unsupported_runtime_primitive',
  )
  assert.equal(result.children[0]?.request, undefined)
})
