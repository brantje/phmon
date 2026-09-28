import assert from 'node:assert/strict'
import test from 'node:test'
import { characterDeathState } from '../app/utils/characterDeath.ts'

const now = Date.parse('2026-09-28T10:00:00Z')

test('death state is distinct from presence and requires a fresh boolean sample', () => {
  const character = {
    online: true,
    dead: true,
    state_updated_at: '2026-09-28T09:59:55Z',
  }
  assert.equal(characterDeathState(character, false, now), 'dead')
  assert.equal(
    characterDeathState({ ...character, dead: false }, false, now),
    'alive',
  )
  assert.equal(
    characterDeathState({ ...character, online: false }, false, now),
    'unknown',
  )
  assert.equal(
    characterDeathState({ ...character, dead: null }, false, now),
    'unknown',
  )
  assert.equal(characterDeathState(character, true, now), 'unknown')
  assert.equal(
    characterDeathState(
      { ...character, state_updated_at: '2026-09-28T09:59:00Z' },
      false,
      now,
    ),
    'unknown',
  )
})
