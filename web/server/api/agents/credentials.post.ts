import {
  createError,
  defineEventHandler,
  getRequestHeader,
  getRequestURL,
  setHeader,
} from 'h3'
import type { AgentCredential } from '../../../shared/types/agent'

export default defineEventHandler(async (event): Promise<AgentCredential> => {
  setHeader(event, 'Cache-Control', 'no-store')
  const requestURL = getRequestURL(event)
  const origin = getRequestHeader(event, 'origin')
  const fetchSite = getRequestHeader(event, 'sec-fetch-site')
  const contentType = getRequestHeader(event, 'content-type')
  const mediaType = contentType?.split(';', 1)[0]?.trim().toLowerCase()
  if (
    fetchSite?.toLowerCase() === 'cross-site' ||
    (origin !== undefined && origin !== requestURL.origin) ||
    mediaType !== 'application/json'
  ) {
    throw createError({
      statusCode: 403,
      statusMessage: 'Credential requests must be same-origin JSON',
    })
  }

  const { backendUrl } = useRuntimeConfig(event)

  try {
    return await $fetch<AgentCredential>('/api/agents/credentials', {
      baseURL: backendUrl,
      headers: backendAuthHeaders(event),
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
