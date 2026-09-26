import { defineEventHandler, getQuery, setHeader, setResponseStatus } from 'h3'

export default defineEventHandler(
  async (event): Promise<Record<string, unknown>> => {
    setHeader(event, 'Cache-Control', 'no-store')
    const { backendUrl } = useRuntimeConfig(event)
    try {
      return await $fetch<Record<string, unknown>>('/api/characters', {
        baseURL: backendUrl,
        query: getQuery(event),
        timeout: 3000,
        retry: 0,
      })
    } catch {
      setResponseStatus(event, 503)
      return { characters: [], status: 'unavailable' }
    }
  },
)
