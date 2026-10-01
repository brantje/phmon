import assert from 'node:assert/strict'
import test from 'node:test'
import {
  actionTargetGroupState,
  applyActionTargetGroup,
  reconcileActionTargets,
  selectAllActionTargets,
  toggleActionTarget,
} from '../app/utils/actionTargets.ts'

test('client target selection deduplicates overlapping group membership', () => {
  const selected = applyActionTargetGroup(
    applyActionTargetGroup(new Set(), ['alpha', 'bravo']),
    ['bravo', 'charlie'],
  )
  assert.deepEqual([...selected], ['alpha', 'bravo', 'charlie'])
  assert.equal(actionTargetGroupState(selected, ['alpha', 'bravo']), 'checked')
  assert.equal(
    actionTargetGroupState(selected, ['bravo', 'charlie', 'delta']),
    'indeterminate',
  )
})

test('client group updates do not select newly added members and stale lists retain targets', () => {
  const selected = new Set(['alpha', 'bravo'])
  const changedMembership = ['alpha', 'bravo', 'charlie']
  assert.equal(
    actionTargetGroupState(selected, changedMembership),
    'indeterminate',
  )
  assert.deepEqual(
    [...reconcileActionTargets(selected, ['alpha'], false)],
    ['alpha', 'bravo'],
  )
  assert.deepEqual(
    [...reconcileActionTargets(selected, ['alpha'], true)],
    ['alpha'],
  )
  assert.deepEqual(
    [...selectAllActionTargets(['online', 'offline', 'online'])],
    ['online', 'offline'],
  )
  assert.deepEqual(
    [...toggleActionTarget(selected, 'charlie')],
    ['alpha', 'bravo', 'charlie'],
  )
})
