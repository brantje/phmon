import {
  defineEventHandler,
  getRouterParam,
  setHeader,
  setResponseStatus,
} from 'h3'

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
  } catch {
    setResponseStatus(event, 503)
    return { error: 'unavailable' }
  }
})
