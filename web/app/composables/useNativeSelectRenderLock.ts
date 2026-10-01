import { computed, ref, type Ref } from 'vue'

/** Freeze rendered <option> rows while a native select menu is open. */
export function useNativeSelectRenderLock<T>(source: () => T): {
  display: Ref<T>
  lock: () => void
  unlock: () => void
  onPointerDown: () => void
  onFocus: () => void
  onChange: () => void
  onBlur: () => void
  selectRef: Ref<HTMLSelectElement | null>
} {
  const selectRef = ref<HTMLSelectElement | null>(null)
  const locked = ref(false)
  const snapshot = ref<T | null>(null)
  const display = computed(() =>
    locked.value && snapshot.value != null ? snapshot.value : source(),
  ) as Ref<T>

  function lock() {
    if (locked.value) return
    locked.value = true
    snapshot.value = structuredClone(source())
  }

  function unlock() {
    locked.value = false
    snapshot.value = null
  }

  function onPointerDown() {
    lock()
  }

  function onFocus() {
    lock()
  }

  function onChange() {
    unlock()
  }

  function onBlur() {
    queueMicrotask(() => {
      if (selectRef.value?.matches(':focus')) return
      unlock()
    })
  }

  return {
    display,
    lock,
    unlock,
    onPointerDown,
    onFocus,
    onChange,
    onBlur,
    selectRef,
  }
}
