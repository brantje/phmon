import type { FoundItem, ItemSearchPage } from '~/utils/itemSearch'
import { readItemSearchItems } from '~/utils/itemSearch'

export function useItemSearch(server: Ref<string>) {
  const items = shallowRef<FoundItem[]>([])
  const loading = ref(true)
  const error = ref('')
  let controller: AbortController | undefined
  let generation = 0

  async function reload(clear = false) {
    const current = ++generation
    controller?.abort()
    controller = new AbortController()
    const signal = controller.signal
    if (clear) items.value = []
    loading.value = true
    error.value = ''
    const scope = server.value
    try {
      const result = await readItemSearchItems((cursor) =>
        $fetch<ItemSearchPage>('/api/items', {
          query: {
            server: scope === 'all' ? undefined : scope,
            include_sets: 'true',
            // One page reads every row from one snapshot; paging across
            // requests almost always saw the total change midway.
            limit: 5000,
            cursor,
          },
          signal,
          retry: 0,
          timeout: 15_000,
        }),
      )
      if (current === generation) items.value = result
    } catch {
      if (current === generation)
        error.value = 'Item Search could not be loaded.'
    } finally {
      if (current === generation) loading.value = false
    }
  }
  watch(server, () => {
    void reload(true)
  })
  onMounted(() => {
    void reload()
  })
  onBeforeUnmount(() => {
    generation++
    controller?.abort()
  })
  return { items, loading, error, reload }
}
