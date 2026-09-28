import { defineEventHandler, getQuery, setHeader } from 'h3'
import { forwardProxyError } from '../utils/proxy'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    return await $fetch<Record<string, unknown>>('/api/guild-storage', {
      baseURL: useRuntimeConfig(event).backendUrl,
      headers: backendAuthHeaders(event),
      query: getQuery(event),
      timeout: 3000,
      retry: 0,
    })
  } catch (error) {
    const response = forwardProxyError(event, error)
    return (event.node.res.statusCode || 503) === 503
      ? { server: '', guild: '', items: [], status: 'unavailable' }
      : response
  }
})
