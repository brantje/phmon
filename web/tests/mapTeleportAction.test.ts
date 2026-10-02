import assert from 'node:assert/strict'
import test from 'node:test'
import type { CharacterView, MapNpc } from '../shared/types/live.ts'
import {
  characterObservesGate,
  createMapTeleportIntent,
  gateSourceLabel,
  mapTeleportCommand,
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
