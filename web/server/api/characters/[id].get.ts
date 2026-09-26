import { defineEventHandler, getRouterParam, setHeader } from 'h3'
import { forwardProxyError } from '../../utils/proxy'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  try {
    return await $fetch(
      '/api/characters/' +
        encodeURIComponent(getRouterParam(event, 'id') || ''),
      {
        baseURL: useRuntimeConfig(event).backendUrl,
        timeout: 3000,
        retry: 0,
      },
    )
  } catch (error) {
    return forwardProxyError(event, error)
  }
})
