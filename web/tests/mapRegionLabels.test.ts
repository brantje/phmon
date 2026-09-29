import assert from 'node:assert/strict'
import test from 'node:test'
import { uniqueRegionOptionLabels } from '../app/utils/mapRegionLabels.ts'

test('map zone options disambiguate missing and duplicate zone names', () => {
  const labels = uniqueRegionOptionLabels(
    [25000, 25001, 25002, 25003],
    (region) => {
      if (region === 25000) return 'Jangan'
      if (region === 25003) return 'Donwhang'
      return 'Unknown zone'
    },
  )

  assert.deepEqual(
    [...labels],
    [
      [25000, 'Jangan'],
      [25001, 'Unknown zone · 25001'],
      [25002, 'Unknown zone · 25002'],
      [25003, 'Donwhang'],
    ],
  )
})

test('map zone options distinguish regions that share a known zone name', () => {
  const labels = uniqueRegionOptionLabels([25735, 25736], () => 'Jangan')

  assert.deepEqual(
    [...labels],
    [
      [25735, 'Jangan · 25735'],
      [25736, 'Jangan · 25736'],
    ],
  )
})
