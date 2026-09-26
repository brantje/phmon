import type { H3Event } from 'h3'
import { createError, setResponseStatus } from 'h3'

export async function readBoundedJSON(
  event: H3Event,
  limit = 4096,
): Promise<Record<string, unknown>> {
  const declaredLength = Number(event.node.req.headers['content-length'] || 0)
  if (declaredLength > limit) {
    throw createError({ statusCode: 413, statusMessage: 'request too large' })
  }
  const chunks: Buffer[] = []
  let size = 0
  for await (const chunk of event.node.req) {
    const data = Buffer.isBuffer(chunk) ? chunk : Buffer.from(chunk)
    size += data.length
    if (size > limit) {
      throw createError({ statusCode: 413, statusMessage: 'request too large' })
    }
    chunks.push(data)
  }
  try {
    const body: unknown = JSON.parse(Buffer.concat(chunks).toString('utf8'))
    if (!body || typeof body !== 'object' || Array.isArray(body)) {
      throw createError({
        statusCode: 400,
        statusMessage: 'invalid JSON object',
      })
    }
    return body as Record<string, unknown>
  } catch {
    throw createError({ statusCode: 400, statusMessage: 'invalid JSON' })
  }
}

export function forwardProxyError(
  event: H3Event,
  error: unknown,
  fallback = 503,
): Record<string, unknown> {
  const upstream = error as {
    response?: { status?: number; _data?: unknown }
    statusCode?: number
    data?: unknown
  }
  const status = upstream.response?.status || upstream.statusCode || fallback
  setResponseStatus(event, status)
  const body = upstream.response?._data || upstream.data
  if (body && typeof body === 'object' && !Array.isArray(body)) {
    return body as Record<string, unknown>
  }
  return { error: status === 503 ? 'service unavailable' : 'request failed' }
}

export function forwardProxyResponse(
  event: H3Event,
  response: { status: number; _data?: unknown },
) {
  setResponseStatus(event, response.status)
  return response.status === 204 ? undefined : response._data
}
