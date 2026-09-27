import WebSocketClient from 'ws'

const MAX_CLIENT_MESSAGE_BYTES = 16 * 1024
const MAX_SERVER_MESSAGE_BYTES = 512 * 1024
const MAX_PENDING_MESSAGES = 32
const MAX_BROWSER_BUFFERED_BYTES = 512 * 1024
const MAX_UPSTREAM_BUFFERED_BYTES = 512 * 1024

type UpstreamSocket = {
  readyState: number
  bufferedAmount: number
  send(message: string): void
  close(code?: number, reason?: string): void
  on(event: 'open' | 'error', listener: () => void): void
  on(
    event: 'unexpected-response',
    listener: (
      request: { destroy(): void },
      response: { statusCode?: number; resume(): void },
    ) => void,
  ): void
  on(
    event: 'message',
    listener: (data: { toString(): string }, isBinary: boolean) => void,
  ): void
  on(event: 'close', listener: (code: number) => void): void
}

const WS_CONNECTING = 0
const WS_OPEN = 1

type RelayState = {
  upstream: UpstreamSocket
  pending: string[]
  closed: boolean
}

type RelayPeer = {
  id: string
  bufferedAmount: number
  send(message: string): void
  close(code?: number, reason?: string): void
}

const relays = new Map<string, RelayState>()

function byteLength(value: string) {
  return Buffer.byteLength(value, 'utf8')
}

function closeRelay(peer: RelayPeer, code: number, reason: string) {
  const state = relays.get(peer.id)
  if (state) {
    state.closed = true
    relays.delete(peer.id)
    if (
      state.upstream.readyState === WS_CONNECTING ||
      state.upstream.readyState === WS_OPEN
    ) {
      state.upstream.close(code, reason)
    }
  }
  peer.close(code, reason)
}

function upstreamHeaders(request: Request) {
  const config = useRuntimeConfig()
  const cookieName = String(config.operatorCookieName || 'phmon_operator')
  const cookie = (request.headers.get('cookie') || '')
    .split(';')
    .map((value) => value.trim())
    .find((value) => value.startsWith(cookieName + '='))
  const headers: Record<string, string> = {}
  if (cookie) headers.Cookie = cookie
  const origin = request.headers.get('origin')
  if (origin) headers.Origin = origin
  return headers
}

function backendLiveURL() {
  const config = useRuntimeConfig()
  const backend = new URL(String(config.backendUrl))
  if (backend.username || backend.password) {
    throw new Error('backend URL must not contain credentials')
  }
  if (backend.protocol === 'http:') backend.protocol = 'ws:'
  else if (backend.protocol === 'https:') backend.protocol = 'wss:'
  else throw new Error('backend URL must use http or https')
  backend.pathname = '/api/live'
  backend.search = ''
  backend.hash = ''
  return backend.toString()
}

function sameOriginUpgrade(request: Request) {
  const origin = request.headers.get('origin')
  if (!origin) return false
  let parsedOrigin: URL
  try {
    parsedOrigin = new URL(origin)
  } catch {
    return false
  }
  if (parsedOrigin.username || parsedOrigin.password) return false

  const requestURL = new URL(request.url)
  const hosts = [
    request.headers.get('host')?.trim(),
    request.headers.get('x-forwarded-host')?.split(',')[0]?.trim(),
    requestURL.host,
  ].filter((value): value is string => Boolean(value))
  const protocols = [
    request.headers.get('x-forwarded-proto')?.split(',')[0]?.trim(),
    requestURL.protocol.replace(':', ''),
  ].filter(
    (value): value is 'http' | 'https' => value === 'http' || value === 'https',
  )

  return (
    hosts.includes(parsedOrigin.host) &&
    protocols.includes(
      parsedOrigin.protocol.replace(':', '') as 'http' | 'https',
    )
  )
}

export default defineWebSocketHandler({
  upgrade(request) {
    if (!sameOriginUpgrade(request)) {
      return new Response('cross-origin WebSocket rejected', { status: 403 })
    }
  },

  open(peer) {
    let upstream: UpstreamSocket
    try {
      const request = (peer as typeof peer & { request?: Request }).request
      if (!request) {
        peer.close(1011, 'live request unavailable')
        return
      }
      upstream = new WebSocketClient(backendLiveURL(), {
        headers: upstreamHeaders(request),
      }) as UpstreamSocket
    } catch {
      peer.close(1011, 'live backend unavailable')
      return
    }

    const state: RelayState = { upstream, pending: [], closed: false }
    relays.set(peer.id, state)

    upstream.on('open', () => {
      if (state.closed) return
      for (const message of state.pending) upstream.send(message)
      state.pending.length = 0
    })

    upstream.on('message', (data, isBinary) => {
      if (state.closed) return
      if (isBinary) {
        closeRelay(peer, 1003, 'text live protocol required')
        return
      }
      const message = data.toString()
      if (byteLength(message) > MAX_SERVER_MESSAGE_BYTES) {
        closeRelay(peer, 1009, 'live snapshot too large')
        return
      }
      if (peer.bufferedAmount > MAX_BROWSER_BUFFERED_BYTES) {
        closeRelay(peer, 1013, 'slow live consumer')
        return
      }
      peer.send(message)
    })

    upstream.on('close', (code) => {
      if (state.closed) return
      state.closed = true
      relays.delete(peer.id)
      peer.close(
        code >= 1000 && code <= 4999 ? code : 1011,
        'live backend disconnected',
      )
    })

    upstream.on('unexpected-response', (request, response) => {
      response.resume()
      request.destroy()
      if (state.closed) return
      state.closed = true
      relays.delete(peer.id)
      const unauthorized = response.statusCode === 401
      peer.close(
        unauthorized ? 4401 : 1011,
        unauthorized
          ? 'operator authentication required'
          : 'live backend unavailable',
      )
    })

    upstream.on('error', () => {
      if (state.closed) return
      state.closed = true
      relays.delete(peer.id)
      peer.close(1011, 'live backend unavailable')
    })
  },

  message(peer, message) {
    const state = relays.get(peer.id)
    if (!state || state.closed) {
      peer.close(1011, 'live backend unavailable')
      return
    }
    const text = message.text()
    if (byteLength(text) > MAX_CLIENT_MESSAGE_BYTES) {
      closeRelay(peer, 1009, 'live request too large')
      return
    }
    if (state.upstream.readyState === WS_OPEN) {
      if (state.upstream.bufferedAmount > MAX_UPSTREAM_BUFFERED_BYTES) {
        closeRelay(peer, 1013, 'live backend is not consuming')
        return
      }
      state.upstream.send(text)
      return
    }
    if (state.upstream.readyState !== WS_CONNECTING) {
      closeRelay(peer, 1011, 'live backend unavailable')
      return
    }
    if (state.pending.length >= MAX_PENDING_MESSAGES) {
      closeRelay(peer, 1013, 'live relay queue full')
      return
    }
    state.pending.push(text)
  },

  close(peer) {
    const state = relays.get(peer.id)
    if (!state) return
    state.closed = true
    relays.delete(peer.id)
    if (
      state.upstream.readyState === WS_CONNECTING ||
      state.upstream.readyState === WS_OPEN
    ) {
      state.upstream.close(1000, 'browser disconnected')
    }
  },

  error(peer) {
    closeRelay(peer, 1011, 'live relay error')
  },
})
