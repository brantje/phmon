import type { Health } from '~~/shared/types/health'

export function useBackendHealth() {
  return useState<boolean>('backend-ready', () => false)
}

export function useBackendHealthMonitor() {
  const backendReady = useBackendHealth()
  const health = useFetch<Health>('/api/health', {
    retry: 0,
  })
  const { data, error, refresh } = health
  watch(
    [data, error],
    () => {
      backendReady.value = !error.value && data.value?.status === 'ok'
    },
    { immediate: true },
  )
  let timer: ReturnType<typeof setInterval> | undefined
  onMounted(() => {
    timer = setInterval(() => void refresh(), 10000)
  })
  onUnmounted(() => {
    if (timer) clearInterval(timer)
  })
  return health
}
