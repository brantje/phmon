import { defineEventHandler, setHeader, setResponseStatus } from 'h3'

export default defineEventHandler(
  async (event): Promise<Record<string, unknown>> => {
    setHeader(event, 'Cache-Control', 'no-store')
    try {
      return await $fetch<Record<string, unknown>>('/api/groups', {
        baseURL: useRuntimeConfig(event).backendUrl,
        timeout: 3000,
        retry: 0,
      })
    } catch {
      setResponseStatus(event, 503)
      return { groups: [] }
    }
  },
)
