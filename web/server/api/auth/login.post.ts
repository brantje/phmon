import {
  appendResponseHeader,
  createError,
  defineEventHandler,
  getRequestHeader,
  getRequestURL,
  readBody,
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
      statusMessage: 'Login must be same-origin',
    })
  }
  const body = await readBody<{ secret?: unknown }>(event)
  if (!body || typeof body.secret !== 'string' || body.secret.length > 1024) {
    throw createError({
      statusCode: 400,
      statusMessage: 'Invalid login request',
    })
  }
  const { backendUrl } = useRuntimeConfig(event)
  try {
    const upstream = await fetch(
      new URL('/api/auth/login', String(backendUrl)),
      {
        method: 'POST',
        headers: { 'content-type': 'application/json', origin },
        body: JSON.stringify({ secret: body.secret }),
      },
    )
    const setCookie = upstream.headers.get('set-cookie')
    if (setCookie) appendResponseHeader(event, 'set-cookie', setCookie)
    setResponseStatus(event, upstream.status)
    return await upstream.json().catch(() => ({ error: 'service_unavailable' }))
  } catch {
    throw createError({
      statusCode: 503,
      statusMessage: 'Operator authentication unavailable',
    })
  }
})
