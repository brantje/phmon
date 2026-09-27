import { defineEventHandler, getRouterParam, setHeader } from 'h3'
import {
  forwardProxyError,
  forwardProxyResponse,
} from '../../../../utils/proxy'
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const url =
    '/api/groups/' +
    encodeURIComponent(getRouterParam(event, 'id') || '') +
    '/members/' +
    encodeURIComponent(getRouterParam(event, 'characterID') || '')
  try {
    const response = await $fetch.raw(url, {
      baseURL: useRuntimeConfig(event).backendUrl,
      headers: backendAuthHeaders(event),
      method: 'DELETE',
      timeout: 3000,
      retry: 0,
    })
    return forwardProxyResponse(event, response)
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
