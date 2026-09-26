const MAX_CLIENT_MESSAGE_BYTES = 16 * 1024
const MAX_SERVER_MESSAGE_BYTES = 512 * 1024
const MAX_PENDING_MESSAGES = 32
const MAX_BROWSER_BUFFERED_BYTES = 512 * 1024

type RelayState = {
  upstream: WebSocket
  pending: string[]
  closed: boolean
}

const relays = new Map<string, RelayState>()

function byteLength(value: string) {
  return Buffer.byteLength(value, 'utf8')
}

function closeRelay(peer: Parameters<NonNullable<Parameters<typeof defineWebSocketHandler>[0]['close']>>[0], code: number, reason: string) {
  const state = relays.get(peer.id)
  if (state) {
    state.closed = true
    relays.delete(peer.id)
    if (state.upstream.readyState === WebSocket.CONNECTING || state.upstream.readyState === WebSocket.OPEN) {
      state.upstream.close(code, reason)
    }
  }
  peer.close(code, reason)
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
  const forwardedHost =
    request.headers.get('x-forwarded-host')?.split(',')[0]?.trim() ||
    request.headers.get('host') ||
    requestURL.host
  const forwardedProto =
    request.headers.get('x-forwarded-proto')?.split(',')[0]?.trim() ||
    requestURL.protocol.replace(':', '')
  if (forwardedProto !== 'http' && forwardedProto !== 'https') return false

  return parsedOrigin.origin === `${forwardedProto}://${forwardedHost}`
}

export default defineWebSocketHandler({
  upgrade(request) {
    if (!sameOriginUpgrade(request)) {
      return new Response('cross-origin WebSocket rejected', { status: 403 })
    }
  },

  open(peer) {
    let upstream: WebSocket
    try {
      upstream = new WebSocket(backendLiveURL())
    } catch {
      peer.close(1011, 'live backend unavailable')
      return
    }

    const state: RelayState = { upstream, pending: [], closed: false }
    relays.set(peer.id, state)

    upstream.addEventListener('open', () => {
      if (state.closed) return
      for (const message of state.pending) upstream.send(message)
      state.pending.length = 0
    })

    upstream.addEventListener('message', (event) => {
      if (state.closed) return
      if (typeof event.data !== 'string') {
        closeRelay(peer, 1003, 'text live protocol required')
        return
      }
      if (byteLength(event.data) > MAX_SERVER_MESSAGE_BYTES) {
        closeRelay(peer, 1009, 'live snapshot too large')
        return
      }
      if (peer.bufferedAmount > MAX_BROWSER_BUFFERED_BYTES) {
        closeRelay(peer, 1013, 'slow live consumer')
        return
      }
      peer.send(event.data)
    })

    upstream.addEventListener('close', (event) => {
      if (state.closed) return
      state.closed = true
      relays.delete(peer.id)
      peer.close(
        event.code >= 1000 && event.code <= 4999 ? event.code : 1011,
        'live backend disconnected',
      )
    })

    upstream.addEventListener('error', () => {
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
    if (state.upstream.readyState === WebSocket.OPEN) {
      state.upstream.send(text)
      return
    }
    if (state.upstream.readyState !== WebSocket.CONNECTING) {
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
    if (state.upstream.readyState === WebSocket.CONNECTING || state.upstream.readyState === WebSocket.OPEN) {
      state.upstream.close(1000, 'browser disconnected')
    }
  },

  error(peer) {
    closeRelay(peer, 1011, 'live relay error')
  },
})
