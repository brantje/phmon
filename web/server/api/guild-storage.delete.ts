import { defineEventHandler, setHeader } from 'h3'
import {
  forwardProxyError,
  forwardProxyResponse,
  readBoundedJSON,
} from '../utils/proxy'
import { backendAuthHeaders } from '../utils/operatorAuth'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    const body = await readBoundedJSON(event, 4096)
    const response = await $fetch.raw<Record<string, unknown>>(
      '/api/guild-storage',
      {
        baseURL: useRuntimeConfig(event).backendUrl,
        headers: backendAuthHeaders(event),
        method: 'DELETE',
        body,
        timeout: 5000,
        retry: 0,
      },
    )
    return forwardProxyResponse(event, response)
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
