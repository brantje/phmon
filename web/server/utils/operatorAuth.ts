import type { H3Event } from 'h3'
import { getCookie, getRequestHeader } from 'h3'

export function backendAuthHeaders(event: H3Event) {
  const config = useRuntimeConfig(event)
  const cookieName = String(config.operatorCookieName || 'phmon_operator')
  const headers: Record<string, string> = {}
  const token = getCookie(event, cookieName)
  if (token) headers.cookie = `${cookieName}=${encodeURIComponent(token)}`
  const origin = getRequestHeader(event, 'origin')
  if (origin) headers.origin = origin
  return headers
}
