import { defineEventHandler, getQuery, setHeader } from 'h3'
import { forwardProxyError } from '../utils/proxy'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const { backendUrl } = useRuntimeConfig(event)
  try {
    return await $fetch<Record<string, unknown>>('/api/trade-reports', {
      baseURL: backendUrl,
      headers: backendAuthHeaders(event),
      query: getQuery(event),
      timeout: 3000,
      retry: 0,
    })
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
