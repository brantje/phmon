import {
  defineEventHandler,
  getMethod,
  getQuery,
  getRouterParam,
  setHeader,
} from 'h3'
import { backendAuthHeaders } from '../../utils/operatorAuth'
import {
  forwardProxyError,
  forwardProxyResponse,
  readBoundedJSON,
} from '../../utils/proxy'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const path = getRouterParam(event, 'path') || ''
  if (
    !/^(?:servers|match-candidates|links(?:\/[0-9a-f-]{36})?|[0-9a-f-]{36}(?:\/(?:aliases|observations|equipment(?:\/history)?))?)?$/i.test(
      path,
    )
  )
    return forwardProxyError(event, { statusCode: 404 })
  const method = getMethod(event)
  if (method !== 'GET' && method !== 'POST' && method !== 'DELETE')
    return forwardProxyError(event, { statusCode: 405 })
  try {
    const response = await $fetch.raw<Record<string, unknown>>(
      `/api/players${path ? `/${path}` : ''}`,
      {
        baseURL: useRuntimeConfig(event).backendUrl,
        headers: backendAuthHeaders(event),
        method,
        query: getQuery(event),
        body: method === 'GET' ? undefined : await readBoundedJSON(event),
        timeout: 5000,
        retry: 0,
      },
    )
    return forwardProxyResponse(event, response)
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
