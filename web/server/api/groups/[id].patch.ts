import { defineEventHandler, getRouterParam, setHeader } from 'h3'
import {
  forwardProxyError,
  forwardProxyResponse,
  readBoundedJSON,
} from '../../utils/proxy'
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    const response = await $fetch.raw(
      '/api/groups/' + encodeURIComponent(getRouterParam(event, 'id') || ''),
      {
        baseURL: useRuntimeConfig(event).backendUrl,
        method: 'PATCH',
        body: await readBoundedJSON(event),
        timeout: 3000,
        retry: 0,
      },
    )
    return forwardProxyResponse(event, response)
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
