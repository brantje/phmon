import assert from 'node:assert/strict'
import test from 'node:test'
import { analyticsGoldOrigin } from '../app/utils/analyticsGoldScale.ts'

test('gold bars retain the full balance for a single character and the smallest fleet member', () => {
  for (const values of [['884736012'], ['884736012', '41877014860']]) {
    const origin = analyticsGoldOrigin(values, 'bar')
    assert.equal(origin, 0n)
    assert.ok(values.every((value) => Number(BigInt(value) - origin) > 0))
  }
})

test('gold lines preserve one-unit changes above the JavaScript safe integer limit', () => {
  const values = ['9007199254740993', '9007199254740994']
  const origin = analyticsGoldOrigin(values, 'line')
  assert.deepEqual(
    values.map((value) => Number(BigInt(value) - origin)),
    [0, 1],
  )
})

test('zero balances and absent or malformed values keep a valid origin', () => {
  assert.equal(analyticsGoldOrigin(['0', '100'], 'bar'), 0n)
  assert.equal(analyticsGoldOrigin([], 'line'), 0n)
  assert.equal(analyticsGoldOrigin(['unknown', '1.5'], 'line'), 0n)
})
