import {
  defineEventHandler,
  getRouterParam,
  setHeader,
  setResponseStatus,
} from 'h3'
export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const url =
    '/api/groups/' +
    encodeURIComponent(getRouterParam(event, 'id') || '') +
    '/members/' +
    encodeURIComponent(getRouterParam(event, 'characterID') || '')
  try {
    return await $fetch(url, {
      baseURL: useRuntimeConfig(event).backendUrl,
      method: 'PUT',
      timeout: 3000,
      retry: 0,
    })
  } catch {
    setResponseStatus(event, 400)
    return { error: 'membership update failed' }
  }
})
