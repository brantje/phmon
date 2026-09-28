import assert from 'node:assert/strict'
import test from 'node:test'
import { pruneSeenChatMessageIDs } from '../app/utils/pruneSeenChatMessageIDs.ts'

test('keeps IDs in current chat snapshots while pruning older IDs', () => {
  const seenIDs = new Set(['visible', 'old-one', 'old-two', 'old-three'])

  pruneSeenChatMessageIDs(seenIDs, new Set(['visible']), 2)

  assert.deepEqual([...seenIDs], ['visible', 'old-three'])
})

test('preserves all current snapshot IDs when they exceed the cache limit', () => {
  const seenIDs = new Set(['active-one', 'active-two', 'old'])

  pruneSeenChatMessageIDs(seenIDs, new Set(['active-one', 'active-two']), 1)

  assert.deepEqual([...seenIDs], ['active-one', 'active-two'])
})
