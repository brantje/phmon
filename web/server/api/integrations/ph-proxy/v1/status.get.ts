import { defineEventHandler, getQuery, setHeader } from 'h3'
import { forwardProxyError } from '../../../../utils/proxy'
import { backendAuthHeaders } from '../../../../utils/operatorAuth'

type PhProxyStatusResponse = Record<string, unknown>

export default defineEventHandler(
  async (event): Promise<PhProxyStatusResponse> => {
    setHeader(event, 'Cache-Control', 'no-store')
    try {
      const response = await $fetch<PhProxyStatusResponse>(
        '/api/integrations/ph-proxy/v1/status',
        {
          baseURL: useRuntimeConfig(event).backendUrl,
          headers: backendAuthHeaders(event),
          query: getQuery(event),
          timeout: 3000,
          retry: 0,
        },
      )
      return response
    } catch (error) {
      return forwardProxyError(event, error)
    }
  },
)
