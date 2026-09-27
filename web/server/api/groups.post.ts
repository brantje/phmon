import { defineEventHandler, setHeader } from 'h3'
import {
  forwardProxyError,
  forwardProxyResponse,
  readBoundedJSON,
} from '../utils/proxy'
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    const body = await readBoundedJSON(event)
    const response = await $fetch.raw<Record<string, unknown>>('/api/groups', {
      baseURL: useRuntimeConfig(event).backendUrl,
      headers: backendAuthHeaders(event),
      method: 'POST',
      body,
      timeout: 3000,
      retry: 0,
    })
    return forwardProxyResponse(event, response)
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
