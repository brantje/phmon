import assert from 'node:assert/strict'
import test from 'node:test'
import { characterPositionIsFresh } from '../app/utils/characterPositionFreshness.ts'

test('fresh samples stay live when the UI clock lags behind the sample', () => {
  const laggedNow = Date.parse('2026-09-29T12:00:00Z')
  const character = {
    online: true,
    state_updated_at: '2026-09-29T12:00:20Z',
  }
  assert.equal(characterPositionIsFresh(character, laggedNow), true)
})

test('positions older than the live window are stale', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  const character = {
    online: true,
    state_updated_at: '2026-09-29T11:59:24Z',
  }
  assert.equal(characterPositionIsFresh(character, now), false)
})

test('offline characters are not treated as live positions', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  assert.equal(
    characterPositionIsFresh(
      { online: false, state_updated_at: '2026-09-29T12:00:00Z' },
      now,
    ),
    false,
  )
})
