import { defineEventHandler, setHeader } from 'h3'
import {
  forwardProxyError,
  forwardProxyResponse,
  readBoundedJSON,
} from '../../utils/proxy'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    const body = await readBoundedJSON(event, 8 * 1024)
    const response = await $fetch.raw<Record<string, unknown>>(
      '/api/chat/read',
      {
        baseURL: useRuntimeConfig(event).backendUrl,
        headers: backendAuthHeaders(event),
        method: 'POST',
        body,
        timeout: 4000,
        retry: 0,
      },
    )
    return forwardProxyResponse(event, response)
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
