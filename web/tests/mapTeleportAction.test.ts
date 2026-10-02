import assert from 'node:assert/strict'
import test from 'node:test'
import type { CharacterView, MapNpc } from '../shared/types/live.ts'
import {
  fanOutCounts,
  prepareCommandFanOut,
  submitCommandFanOut,
} from '../app/utils/commandFanOut.ts'
import {
  characterObservesGate,
  createMapTeleportIntent,
  gateSourceLabel,
  mapTeleportCommand,
  teleportEligibleActionLabel,
} from '../app/utils/mapTeleportAction.ts'

const teleporter: MapNpc = {
  id: 'npc:greatest:25000:GATE_KT:none:4:30.0:40.0',
  name: 'Hotan',
  servername: 'GATE_KT',
  role: 'teleporter',
  region: 25000,
  x: 30,
  y: 40,
  observed_at: '2026-10-02T10:00:00Z',
  observers: [
    {
      character_id: 'char-a',
      session_id: 'sess-a',
      name: 'Alpha',
      teleport_routes: [{ destination: 'Jangan' }],
    },
  ],
}

test('gateSourceLabel prefers display name', () => {
  assert.equal(gateSourceLabel(teleporter), 'Hotan')
})

test('createMapTeleportIntent rejects invalid destinations', () => {
  assert.equal(
    createMapTeleportIntent({
      server: 'Greatest',
      npc: teleporter,
      destination: 'Hotan,Jangan',
      targetIDs: ['char-a'],
    }),
    null,
  )
})

test('map teleport eligibility requires gate observer and capability', () => {
  const intent = createMapTeleportIntent({
    server: 'Greatest',
    npc: teleporter,
    destination: 'Jangan',
    targetIDs: ['char-a'],
  })
  assert.ok(intent)
  const character: CharacterView = {
    character_id: 'char-a',
    name: 'Alpha',
    server: 'Greatest',
    online: true,
    session_id: 'sess-a',
  }
  const definition = mapTeleportCommand({
    getIntent: () => intent,
    getCharacter: (id) => (id === 'char-a' ? character : undefined),
    getControls: () => ({
      character_id: 'char-a',
      session_id: 'sess-a',
      capabilities: {
        'character.teleport': { supported: true },
      },
    }),
    mapFeedCurrent: () => true,
  })
  assert.equal(definition.preEligibility?.(character)?.code, undefined)
  assert.ok(characterObservesGate('char-a', teleporter))
  assert.equal(
    definition.eligibility?.(character, definition.buildArgs(character), {
      character_id: 'char-a',
      session_id: 'sess-a',
      capabilities: { 'character.teleport': { supported: false } },
    })?.code,
    'unsupported',
  )
  assert.deepEqual(definition.buildArgs(character), {
    source: 'Hotan',
    destination: 'Jangan',
    gate_servername: 'GATE_KT',
  })
})

test('map teleport admission rejects a stale map feed', async () => {
  const intent = createMapTeleportIntent({
    server: 'Greatest',
    npc: teleporter,
    destination: 'Jangan',
    targetIDs: ['char-a'],
  })
  assert.ok(intent)
  const character: CharacterView = {
    character_id: 'char-a',
    name: 'Alpha',
    server: 'Greatest',
    online: true,
    session_id: 'sess-a',
  }
  const definition = mapTeleportCommand({
    getIntent: () => intent,
    getCharacter: () => character,
    getControls: () => ({
      character_id: 'char-a',
      session_id: 'sess-a',
      capabilities: {
        'character.teleport': { supported: true },
      },
    }),
    mapFeedCurrent: () => false,
  })
  const reason = await definition.admissionGuard?.(
    {
      characterID: 'char-a',
      characterName: 'Alpha',
      sessionID: 'sess-a',
      submission: 'ready',
    },
    {
      character_id: 'char-a',
      expected_session_id: 'sess-a',
      args: {
        source: 'Hotan',
        destination: 'Jangan',
        gate_servername: 'GATE_KT',
      },
    },
    {},
  )
  assert.equal(reason?.code, 'stale_map_scope')
})

test('teleport action label uses the eligible count, not the selected count', () => {
  assert.equal(
    teleportEligibleActionLabel({
      destination: 'Jangan',
      eligible: 6,
    }),
    'Teleport 6 characters to Jangan',
  )
  assert.equal(
    teleportEligibleActionLabel({
      destination: 'Jangan',
      eligible: 1,
      eligibleName: 'Alpha',
    }),
    'Teleport Alpha to Jangan',
  )
  assert.equal(
    teleportEligibleActionLabel({ destination: 'Jangan', eligible: 0 }),
    'Teleport to Jangan',
  )
})

test('destination eligibility is observer-specific and does not submit the unverified character', async () => {
  const sharedGate: MapNpc = {
    ...teleporter,
    teleport_routes: [{ destination: 'Jangan' }, { destination: 'Donwhang' }],
    observers: [
      {
        character_id: 'char-a',
        session_id: 'sess-a',
        name: 'Alpha',
        teleport_routes: [{ destination: 'Jangan' }],
      },
      {
        character_id: 'char-b',
        session_id: 'sess-b',
        name: 'Bravo',
        teleport_routes: [{ destination: 'Donwhang' }],
      },
    ],
  }
  const characters: Record<string, CharacterView> = {
    'char-a': {
      character_id: 'char-a',
      name: 'Alpha',
      server: 'Greatest',
      online: true,
      session_id: 'sess-a',
    },
    'char-b': {
      character_id: 'char-b',
      name: 'Bravo',
      server: 'Greatest',
      online: true,
      session_id: 'sess-b',
    },
  }
  const intent = createMapTeleportIntent({
    server: 'Greatest',
    npc: sharedGate,
    destination: 'Jangan',
    targetIDs: ['char-a', 'char-b'],
  })
  assert.ok(intent)
  const definition = mapTeleportCommand({
    getIntent: () => intent,
    getCharacter: (id) => characters[id],
    getControls: (id) => ({
      character_id: id,
      session_id: characters[id]?.session_id || '',
      capabilities: { 'character.teleport': { supported: true } },
    }),
    mapFeedCurrent: () => true,
  })
  const scopeKey = 'teleport:greatest:GATE_KT'
  const controlsFor = (id: string) => ({
    character_id: id,
    session_id: characters[id]?.session_id || '',
    capabilities: { 'character.teleport': { supported: true } },
  })
  const operation = prepareCommandFanOut({
    operationID: 'teleport-op',
    command: definition,
    characterIDs: ['char-a', 'char-b'],
    scopeKey,
    liveCurrent: true,
    idempotencyKey: () => 'idem-a',
    targets: {
      'char-a': {
        character: characters['char-a']!,
        controls: controlsFor('char-a'),
        scopeKey,
      },
      'char-b': {
        character: characters['char-b']!,
        controls: controlsFor('char-b'),
        scopeKey,
      },
    },
  })
  const counts = fanOutCounts(operation)
  assert.deepEqual(
    {
      selected: counts.selected,
      eligible: counts.eligible,
      skipped: counts.skipped,
    },
    { selected: 2, eligible: 1, skipped: 1 },
  )
  const skipped = operation.children.find(
    (child) => child.characterID === 'char-b',
  )
  assert.equal(skipped?.skipReason?.code, 'destination_unverified')
  const posted: string[] = []
  await submitCommandFanOut(operation, {
    post: async (request) => {
      posted.push(request.character_id)
      return { command_id: 'cmd-a' }
    },
    currentCharacter: (id) => characters[id],
    currentScopeKey: () => scopeKey,
    changed: () => undefined,
  })
  assert.deepEqual(posted, ['char-a'])
  characters['char-a'] = {
    ...characters['char-a']!,
    session_id: 'sess-replaced',
  }
  const ready = operation.children.find(
    (child) => child.characterID === 'char-a',
  )
  assert.ok(ready?.request)
  const reason = await definition.admissionGuard?.(ready, ready.request, {})
  assert.equal(reason?.code, 'session_changed')
})
