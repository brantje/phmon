import assert from 'node:assert/strict'
import { readFileSync, readdirSync } from 'node:fs'
import test from 'node:test'

const mapPageURL = new URL('../app/pages/map.vue', import.meta.url)
const pagesURL = new URL('../app/pages/', import.meta.url)

test('/map owns the realtime positions subscription lifecycle', () => {
  const source = readFileSync(mapPageURL, 'utf8')
  assert.match(source, /const positionSubscriptionID = 'map-positions'/)
  assert.match(
    source,
    /setMapPositionFeed\(positionSubscriptionID, selectedServer\)/,
  )
  assert.match(source, /clearMapPositionFeed\(positionSubscriptionID\)/)
  assert.match(source, /onBeforeUnmount\(\(\) => \{/)
})

test('other pages do not create the map positions subscription', () => {
  for (const relativePath of readdirSync(pagesURL, { recursive: true })) {
    if (typeof relativePath !== 'string' || !relativePath.endsWith('.vue'))
      continue
    if (relativePath === 'map.vue') continue
    const source = readFileSync(new URL(relativePath, pagesURL), 'utf8')
    assert.doesNotMatch(source, /setMapPositionFeed\(/, relativePath)
    assert.doesNotMatch(source, /map-positions/, relativePath)
  }
})
