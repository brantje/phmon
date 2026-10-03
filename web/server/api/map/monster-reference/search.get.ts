import { defineEventHandler, getQuery, setHeader } from 'h3'
import { forwardProxyError } from '../../../utils/proxy'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    return await $fetch<Record<string, unknown>>('/api/map/monster-reference/search', {
      baseURL: useRuntimeConfig(event).backendUrl,
      headers: backendAuthHeaders(event),
      query: getQuery(event),
      timeout: 5000,
      retry: 0,
    })
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
