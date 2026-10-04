import assert from 'node:assert/strict'
import test from 'node:test'
import { deathCauseLabel } from '../app/utils/deathCause.ts'

test('recent player attack is labeled as an observation, not a confirmed killer', () => {
  assert.equal(
    deathCauseLabel({
      cause: 'Rival',
      reason_type: 'attacker',
      reason_value: 'Rival',
    }),
    'Recent attacker: Rival',
  )
})

test('fallback and historical death payloads remain readable', () => {
  assert.equal(
    deathCauseLabel({
      cause: 'monster_environment',
      reason_type: 'monster_or_environment',
    }),
    'Monster / environment',
  )
  assert.equal(deathCauseLabel({ cause: 'unknown' }), 'Unknown cause')
  assert.equal(deathCauseLabel({ cause: 'Rival' }), 'Rival')
})
