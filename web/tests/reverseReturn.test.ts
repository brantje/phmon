import assert from 'node:assert/strict'
import test from 'node:test'
import type { CharacterView, ControlsSnapshot } from '../shared/types/live.ts'
import {
  freshReverseReturnParty,
  reverseReturnPartyNames,
  reverseReturnEligibility,
  reverseReturnSummary,
} from '../app/utils/reverseReturn.ts'
import {
  remoteControlDefinition,
  requiresRemoteControlConfirmation,
  validateRemoteControlArgs,
} from '../app/utils/remoteControlActions.ts'
import {
  prepareCommandFanOut,
  submitCommandFanOut,
  fanOutCounts,
  cancelCommandFanOut,
} from '../app/utils/commandFanOut.ts'
const character: CharacterView = {
  character_id: 'a',
  session_id: 'sa',
  server: 'Fixture',
  name: 'Alpha',
  online: true,
}
function controls(id = 'a', names = ['Member']): ControlsSnapshot {
  return {
    character_id: id,
    session_id: `s${id}`,
    capabilities: {
      'character.reverse_return': {
        supported: true,
        modes: ['last_return', 'last_death', 'party_member'],
      },
    },
    reverse_return: {
      session_id: `s${id}`,
      party_status: 'observed',
      party_names: names,
      party_checked_at: new Date().toISOString(),
      scroll_observed: null,
    },
  }
}

test('Reverse return validates modes and requires explicit confirmation', () => {
  assert.ok(
    requiresRemoteControlConfirmation('character.reverse_return', false),
  )
  assert.deepEqual(
    validateRemoteControlArgs('character.reverse_return', {
      reverseReturnType: 2,
      reverseReturnName: ' Member ',
    }),
    { reverseReturnType: 2, reverseReturnName: 'Member' },
  )
  for (const input of [
    { reverseReturnType: 3 },
    { reverseReturnType: 0.5 },
    { reverseReturnType: 2 },
    { reverseReturnType: 0, reverseReturnName: 'Member' },
    { reverseReturnType: 0, reverseReturnName: ' ' },
    { reverseReturnType: 2, reverseReturnName: 'Member\n' },
    { reverseReturnType: 2, reverseReturnName: 'é'.repeat(51) },
  ])
    assert.equal(
      validateRemoteControlArgs('character.reverse_return', input),
      null,
    )
})
test('party names use fresh per-session resources without coordinate filtering', () => {
  const a = controls('a', ['Member', 'Other']),
    b = controls('b', ['member', 'Bravo'])
  assert.deepEqual(reverseReturnPartyNames([a, b]), [
    'Bravo',
    'member',
    'Other',
  ])
  a.reverse_return!.party_checked_at = new Date(
    Date.now() - 36000,
  ).toISOString()
  assert.equal(freshReverseReturnParty(a), false)
  b.reverse_return!.session_id = 'old-session'
  assert.deepEqual(reverseReturnPartyNames([a, b]), [])
})
test('party membership is evaluated independently and unknown inventory stays eligible', () => {
  const control = controls()
  assert.equal(reverseReturnEligibility(character, control, { type: 0 }), null)
  assert.equal(reverseReturnEligibility(character, control, { type: 1 }), null)
  control.reverse_return!.scroll_observed = true
  control.reverse_return!.inventory_checked_at = new Date(
    Date.now() - 36000,
  ).toISOString()
  assert.match(
    reverseReturnSummary({ type: 0 }, control),
    /will be checked by phBot/,
  )
  control.reverse_return!.inventory_checked_at = new Date().toISOString()
  assert.match(
    reverseReturnSummary({ type: 0 }, control),
    /Scroll observed in inventory/,
  )
  assert.equal(
    reverseReturnEligibility(character, control, { type: 2, name: 'Member' }),
    null,
  )
  assert.equal(
    reverseReturnEligibility(character, control, { type: 2, name: 'Alpha' })
      ?.code,
    'party_self_target',
  )
  assert.equal(
    reverseReturnEligibility(character, control, { type: 2, name: 'Missing' })
      ?.code,
    'party_member_not_found',
  )
  control.reverse_return!.party_status = 'stale'
  assert.equal(
    reverseReturnEligibility(character, control, { type: 2, name: 'Member' })
      ?.code,
    'party_unavailable',
  )
})
test('group Reverse return admits all eligible siblings concurrently and cancellation submits nothing', async () => {
  let key = 0
  const command = remoteControlDefinition('character.reverse_return', {
    reverseReturnType: 2,
    reverseReturnName: 'Member',
  })
  const targets = Object.fromEntries(
    ['a', 'b', 'c'].map((id) => [
      id,
      {
        character: {
          ...character,
          character_id: id,
          session_id: `s${id}`,
          name: `Target ${id}`,
        },
        controls: controls(id, id === 'c' ? ['Else'] : ['Member']),
        scopeKey: 'Fixture',
      },
    ]),
  )
  const operation = prepareCommandFanOut({
    operationID: 'reverse',
    command,
    characterIDs: ['a', 'b', 'a', 'c'],
    targets,
    scopeKey: 'Fixture',
    liveCurrent: true,
    idempotencyKey: () => `k${++key}`,
  })
  assert.equal(fanOutCounts(operation).selected, 3)
  assert.equal(fanOutCounts(operation).eligible, 2)
  assert.equal(fanOutCounts(operation).skipped, 1)
  assert.equal(operation.children[0]?.request?.confirmation, true)
  assert.deepEqual(operation.children[0]?.request?.args, {
    type: 2,
    name: 'Member',
  })
  const cancelled = prepareCommandFanOut({
    operationID: 'cancel',
    command,
    characterIDs: ['a'],
    targets,
    scopeKey: 'Fixture',
    liveCurrent: true,
  })
  cancelCommandFanOut(cancelled)
  let calls = 0
  const sent: string[] = []
  let release: () => void = () => {}
  const gate = new Promise<void>((resolve) => {
    release = resolve
  })
  const deps = {
    currentScopeKey: () => 'Fixture',
    currentCharacter: (id: string) => targets[id]?.character,
    changed: () => {},
    post: async (request: { character_id: string }) => {
      calls++
      sent.push(request.character_id)
      await gate
      return {
        command_id: `cmd_${request.character_id}`,
        state: 'queued' as const,
      }
    },
  }
  await submitCommandFanOut(cancelled, deps)
  assert.equal(calls, 0)
  const submitting = submitCommandFanOut(operation, deps)
  await new Promise((resolve) => setTimeout(resolve, 0))
  assert.deepEqual(sent, ['a', 'b'])
  release()
  await submitting
})
