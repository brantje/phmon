import {
  appendResponseHeader,
  createError,
  defineEventHandler,
  getRequestHeader,
  getRequestURL,
  setHeader,
  setResponseStatus,
} from 'h3'

export default defineEventHandler(async (event) => {
  setHeader(event, 'Cache-Control', 'no-store')
  const requestURL = getRequestURL(event)
  const origin = getRequestHeader(event, 'origin')
  const fetchSite = getRequestHeader(event, 'sec-fetch-site')
  if (
    fetchSite?.toLowerCase() === 'cross-site' ||
    !origin ||
    origin !== requestURL.origin
  ) {
    throw createError({
      statusCode: 403,
      statusMessage: 'Logout must be same-origin',
    })
  }
  const { backendUrl } = useRuntimeConfig(event)
  try {
    const upstream = await fetch(
      new URL('/api/auth/logout', String(backendUrl)),
      {
        method: 'POST',
        headers: backendAuthHeaders(event),
      },
    )
    const setCookie = upstream.headers.get('set-cookie')
    if (setCookie) appendResponseHeader(event, 'set-cookie', setCookie)
    setResponseStatus(event, upstream.status)
    return await upstream.json().catch(() => ({ authenticated: false }))
  } catch {
    throw createError({
      statusCode: 503,
      statusMessage: 'Operator authentication unavailable',
    })
  }
})
