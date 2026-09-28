export function pruneSeenChatMessageIDs(
  seenIDs: Set<string>,
  currentSnapshotIDs: ReadonlySet<string>,
  maxSize = 1000,
) {
  if (seenIDs.size <= maxSize) return
  for (const id of seenIDs) {
    if (!currentSnapshotIDs.has(id)) seenIDs.delete(id)
    if (seenIDs.size <= maxSize) return
  }
}
