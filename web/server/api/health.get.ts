import { defineEventHandler, setHeader, setResponseStatus } from 'h3'
import type { Health } from '../../shared/types/health'

export default defineEventHandler(async (event): Promise<Health> => {
  setHeader(event, 'Cache-Control', 'no-store')
  const { backendUrl } = useRuntimeConfig(event)
  try {
    const health = await $fetch<Health>('/readyz', {
      baseURL: backendUrl,
      timeout: 3000,
      retry: 0,
    })
    if (health.status !== 'ok' || health.database !== 'ok') {
      throw new Error('Backend is not ready')
    }
    return health
  } catch {
    setResponseStatus(event, 503)
    return { status: 'unavailable', database: 'unavailable' }
  }
})
