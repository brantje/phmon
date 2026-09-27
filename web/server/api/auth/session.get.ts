import { defineEventHandler, setHeader, setResponseStatus } from 'h3'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const { backendUrl } = useRuntimeConfig(event)
  try {
    const upstream = await fetch(
      new URL('/api/auth/session', String(backendUrl)),
      {
        headers: backendAuthHeaders(event),
      },
    )
    setResponseStatus(event, upstream.status)
    return await upstream.json().catch(() => ({ authenticated: false }))
  } catch {
    setResponseStatus(event, 503)
    return { authenticated: false }
  }
})
