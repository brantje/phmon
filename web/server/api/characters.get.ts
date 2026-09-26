import { defineEventHandler, getQuery, setHeader } from 'h3'
import { forwardProxyError } from '../utils/proxy'

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
    } catch (error) {
      const response = forwardProxyError(event, error)
      if ((event.node.res.statusCode || 503) === 503) {
        return { characters: [], status: 'unavailable' }
      }
      return response
    }
  },
)
