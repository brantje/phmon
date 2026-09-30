import assert from 'node:assert/strict'
import test from 'node:test'
import {
  applyMapActionTargetGroup,
  clearMapActionTargets,
  mapActionTargetGroupState,
  mapActionTargetScopeKey,
  reconcileMapActionTargets,
  selectAllMapActionTargets,
  toggleMapActionTarget,
} from '../app/utils/mapActionTargets.ts'

test('individual selection toggles one character without duplicating IDs', () => {
  const first = toggleMapActionTarget(new Set(['a']), 'b')
  assert.deepEqual([...first].sort(), ['a', 'b'])
  const removed = toggleMapActionTarget(first, 'a')
  assert.deepEqual([...removed], ['b'])
})

test('All selects only applicable characters and None clears every target', () => {
  assert.deepEqual(
    [...selectAllMapActionTargets(['online', 'offline', 'stale'])].sort(),
    ['offline', 'online', 'stale'],
  )
  assert.deepEqual([...clearMapActionTargets()], [])
})

test('group controls represent empty, unchecked, partial and checked states', () => {
  const none = new Set<string>()
  assert.equal(mapActionTargetGroupState(none, []), 'unchecked')
  assert.equal(mapActionTargetGroupState(none, ['a', 'b']), 'unchecked')
  assert.equal(
    mapActionTargetGroupState(new Set(['a']), ['a', 'b']),
    'indeterminate',
  )
  assert.equal(
    mapActionTargetGroupState(new Set(['a', 'b']), ['a', 'b']),
    'checked',
  )
})

test('applying a partial group selects all members; applying a full group clears them', () => {
  const partial = applyMapActionTargetGroup(new Set(['a']), ['a', 'b'])
  assert.deepEqual([...partial].sort(), ['a', 'b'])
  const cleared = applyMapActionTargetGroup(partial, ['a', 'b'])
  assert.deepEqual([...cleared], [])
})

test('overlapping groups share IDs without duplicates and remain independently tri-state', () => {
  const firstGroup = applyMapActionTargetGroup(new Set(), ['a', 'b'])
  const bothGroups = applyMapActionTargetGroup(firstGroup, ['b', 'c'])
  assert.deepEqual([...bothGroups].sort(), ['a', 'b', 'c'])

  const removeFirst = applyMapActionTargetGroup(bothGroups, ['a', 'b'])
  assert.deepEqual([...removeFirst], ['c'])
  assert.equal(
    mapActionTargetGroupState(removeFirst, ['b', 'c']),
    'indeterminate',
  )
})

test('group membership changes recalculate tri-state without retargeting by group identity', () => {
  const targets = new Set(['a', 'b'])
  const oldMembership = ['a', 'b']
  const newMembership = ['a', 'c']
  assert.equal(mapActionTargetGroupState(targets, oldMembership), 'checked')
  assert.equal(
    mapActionTargetGroupState(targets, newMembership),
    'indeterminate',
  )

  const selectedNewMembers = applyMapActionTargetGroup(targets, newMembership)
  assert.deepEqual([...selectedNewMembers].sort(), ['a', 'b', 'c'])
})

test('confirmed membership updates prune departed IDs while stale refreshes preserve targets', () => {
  const selected = new Set(['online', 'offline', 'left-scope'])
  const stale = reconcileMapActionTargets(selected, ['online'], false)
  assert.deepEqual([...stale].sort(), [...selected].sort())

  const current = reconcileMapActionTargets(
    selected,
    ['online', 'offline'],
    true,
  )
  assert.deepEqual([...current].sort(), ['offline', 'online'])
})

test('server, area, floor and region changes produce different target scopes', () => {
  const base = {
    server: 'Greatest',
    area: 'world',
    floor: 'world',
    region: 25273,
  }
  const baseKey = mapActionTargetScopeKey(base)
  for (const changed of [
    { ...base, server: 'Silkroad' },
    { ...base, area: 'donwhang-cave' },
    { ...base, floor: '2F' },
    { ...base, region: -32767 },
  ]) {
    assert.notEqual(mapActionTargetScopeKey(changed), baseKey)
  }
  assert.equal(
    mapActionTargetScopeKey({ ...base, server: 'greatest' }),
    baseKey,
  )
})
