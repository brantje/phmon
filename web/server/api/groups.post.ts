import { defineEventHandler, readBody, setHeader, setResponseStatus } from 'h3'
export default defineEventHandler(
  async (event): Promise<Record<string, unknown>> => {
    setHeader(event, 'Cache-Control', 'no-store')
    try {
      const body = await readBody(event)
      return await $fetch<Record<string, unknown>>('/api/groups', {
        baseURL: useRuntimeConfig(event).backendUrl,
        method: 'POST',
        body,
        timeout: 3000,
        retry: 0,
      })
    } catch {
      setResponseStatus(event, 400)
      return { error: 'invalid group' }
    }
  },
)
