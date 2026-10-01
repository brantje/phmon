import { ref, watch, type Ref } from 'vue'

function snapshotEqual<T>(left: T, right: T): boolean {
  if (Object.is(left, right)) return true
  if (
    typeof left !== 'object' ||
    left === null ||
    typeof right !== 'object' ||
    right === null
  ) {
    return false
  }
  try {
    return JSON.stringify(left) === JSON.stringify(right)
  } catch {
    return false
  }
}

/** Keep the last snapshot while a native select is focused so live updates do not reset it. */
export function useFrozenWhileFocused<T>(source: () => T): {
  frozen: Ref<T>
  onFocus: () => void
  onBlur: () => void
} {
  const focused = ref(false)
  const frozen = ref(source()) as Ref<T>
  watch(
    source,
    (next) => {
      if (focused.value) return
      if (snapshotEqual(frozen.value, next)) return
      frozen.value = next
    },
    { deep: true },
  )
  return {
    frozen,
    onFocus: () => {
      focused.value = true
    },
    onBlur: () => {
      focused.value = false
      const next = source()
      if (!snapshotEqual(frozen.value, next)) frozen.value = next
    },
  }
}
