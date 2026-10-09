import { defineEventHandler, getQuery, getRouterParam, setHeader } from 'h3'
import { forwardProxyError } from '../../utils/proxy'
import { backendAuthHeaders } from '../../utils/operatorAuth'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    return await $fetch(
      '/api/players/' + encodeURIComponent(getRouterParam(event, 'id') || ''),
      {
        baseURL: useRuntimeConfig(event).backendUrl,
        headers: backendAuthHeaders(event),
        query: getQuery(event),
        timeout: 3000,
        retry: 0,
      },
    )
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
