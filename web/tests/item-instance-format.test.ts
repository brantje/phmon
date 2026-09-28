import assert from 'node:assert/strict'
import test from 'node:test'
import {
  formatScaledUnsigned,
  formatUnsignedCount,
} from '../app/utils/item-instance-format.ts'

test('keeps unsigned 64-bit values exact', () => {
  assert.equal(
    formatUnsignedCount('18446744073709551615'),
    '18,446,744,073,709,551,615',
  )
  assert.equal(
    formatScaledUnsigned('18446744073709551615', 1, 0),
    '18,446,744,073,709,551,615',
  )
})

test('formats scaled values without stripping integer zeroes', () => {
  assert.equal(formatScaledUnsigned('20', 1, 0), '20')
  assert.equal(formatScaledUnsigned('12345', 100, 2), '123.45')
  assert.equal(formatScaledUnsigned('1200', 100, 2), '12')
  assert.equal(formatScaledUnsigned('1999', 1000, 2), '2')
})

test('rejects malformed or unsafe formatting inputs', () => {
  assert.equal(formatUnsignedCount(Number.MAX_SAFE_INTEGER + 1), null)
  assert.equal(formatUnsignedCount('-1'), null)
  assert.equal(formatScaledUnsigned(3, 1, 0), null)
  assert.equal(formatScaledUnsigned('3', 0, 0), null)
  assert.equal(formatScaledUnsigned('3', 1, 7), null)
})
