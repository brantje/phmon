import assert from 'node:assert/strict'
import test from 'node:test'
import type {
  RealtimePosition,
  RealtimePositionDelta,
} from '../shared/types/live.ts'
import {
  applyRealtimePositionDeltaState,
  realtimePositionSnapshotState,
  validRealtimePosition,
} from '../app/utils/realtimePositions.ts'

function position(
  session_id = 'session-a',
  sequence = 1,
  x = 10,
): RealtimePosition {
  return {
    character_id: 'character-a',
    session_id,
    sequence,
    region: 25000,
    x,
    y: 20,
    z: 3,
    observed_at: '2026-10-05T20:00:00Z',
  }
}

test('snapshot keeps only newest position per character', () => {
  const state = realtimePositionSnapshotState([
    position('session-a', 1, 1),
    position('session-a', 3, 3),
    position('session-a', 2, 2),
  ])
  assert.equal(state['character-a']?.sequence, 3)
  assert.equal(state['character-a']?.x, 3)
})

test('older same-session snapshot cannot rewind a newer delta', () => {
  const current = { 'character-a': position('session-a', 8, 80) }
  const state = realtimePositionSnapshotState(
    [position('session-a', 6, 60)],
    current,
  )
  assert.equal(state['character-a']?.sequence, 8)
  assert.equal(state['character-a']?.x, 80)
})

test('lower or equal sequence is ignored', () => {
  const current = { 'character-a': position('session-a', 5, 5) }
  for (const sequence of [4, 5]) {
    const next = applyRealtimePositionDeltaState(current, {
      positions: [position('session-a', sequence, sequence)],
    })
    assert.equal(next['character-a']?.sequence, 5)
    assert.equal(next['character-a']?.x, 5)
  }
})

test('different session cannot overwrite current overlay without matching removal', () => {
  const current = { 'character-a': position('session-new', 2, 20) }
  const next = applyRealtimePositionDeltaState(current, {
    positions: [position('session-old', 99, 99)],
  })
  assert.equal(next['character-a']?.session_id, 'session-new')
  assert.equal(next['character-a']?.x, 20)
})

test('session-aware removal permits same-batch replacement', () => {
  const current = { 'character-a': position('session-old', 9, 9) }
  const delta: RealtimePositionDelta = {
    removed: [{ character_id: 'character-a', session_id: 'session-old' }],
    positions: [position('session-new', 1, 10)],
  }
  const next = applyRealtimePositionDeltaState(current, delta)
  assert.equal(next['character-a']?.session_id, 'session-new')
  assert.equal(next['character-a']?.sequence, 1)
})

test('delayed old-session removal cannot remove newer overlay', () => {
  const current = { 'character-a': position('session-new', 2, 20) }
  const next = applyRealtimePositionDeltaState(current, {
    removed: [{ character_id: 'character-a', session_id: 'session-old' }],
  })
  assert.deepEqual(next, current)
})

test('malformed coordinates are ignored', () => {
  const malformed = { ...position(), x: Number.NaN }
  assert.equal(validRealtimePosition(malformed), false)
  const next = applyRealtimePositionDeltaState({}, {
    positions: [malformed],
  })
  assert.deepEqual(next, {})
})
