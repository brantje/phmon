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

test('freshness includes both 35-second clock boundaries', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  for (const offset of [-35_000, 35_000]) {
    assert.equal(
      characterPositionIsFresh(
        {
          online: true,
          state_updated_at: new Date(now + offset).toISOString(),
        },
        now,
      ),
      true,
    )
  }
})

test('positions outside the bounded past or future window are stale', () => {
  const now = Date.parse('2026-09-29T12:00:00Z')
  for (const offset of [-35_001, 35_001, 86_400_000]) {
    assert.equal(
      characterPositionIsFresh(
        {
          online: true,
          state_updated_at: new Date(now + offset).toISOString(),
        },
        now,
      ),
      false,
    )
  }
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
