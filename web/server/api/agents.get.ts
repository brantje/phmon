import { defineEventHandler, setHeader, setResponseStatus } from 'h3'
import type { AgentListResponse, AgentView } from '../../shared/types/agent'

export default defineEventHandler(async (event): Promise<AgentListResponse> => {
  setHeader(event, 'Cache-Control', 'no-store')
  const { backendUrl } = useRuntimeConfig(event)

  try {
    const agents = await $fetch<AgentView[]>('/api/agents', {
      baseURL: backendUrl,
      headers: backendAuthHeaders(event),
      timeout: 3000,
      retry: 0,
    })
    return { status: 'ok', agents }
  } catch {
    setResponseStatus(event, 503)
    return { status: 'unavailable', agents: [] }
  }
})
