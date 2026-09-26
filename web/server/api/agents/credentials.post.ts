import { createError, defineEventHandler, setHeader } from 'h3'
import type { AgentCredential } from '../../../shared/types/agent'

export default defineEventHandler(async (event): Promise<AgentCredential> => {
  setHeader(event, 'Cache-Control', 'no-store')
  const { backendUrl } = useRuntimeConfig(event)

  try {
    return await $fetch<AgentCredential>('/api/agents/credentials', {
      baseURL: backendUrl,
      method: 'POST',
      timeout: 3000,
      retry: 0,
    })
  } catch {
    throw createError({
      statusCode: 503,
      statusMessage: 'Credential service unavailable',
    })
  }
})
