import type { AgentView } from '~~/shared/types/agent'
import {
  LIVE_PROTOCOL_VERSION,
  type AgentsSnapshot,
  type CharacterGroup,
  type CharacterSnapshot,
  type CharactersSnapshot,
  type CharacterView,
  type GroupsSnapshot,
  type LiveClientFrame,
  type LiveConnectionState,
  type LiveFilter,
  type LiveServerFrame,
  type LiveStream,
} from '~~/shared/types/live'

type Subscription = {
  id: string
  revision: number
  stream: LiveStream
  filter: LiveFilter
  current: boolean
  unavailable: boolean
}

const agents = ref<AgentView[]>([])
const characters = ref<CharacterView[]>([])
const fleetCharacters = ref<CharacterView[]>([])
const groups = ref<CharacterGroup[]>([])
const characterDetail = ref<CharacterView | null>(null)
const connectionState = ref<LiveConnectionState>('idle')
const hasSnapshot = ref(false)
const staleCycle = ref(false)

const subscriptions = new Map<string, Subscription>()
const subscriptionRevisions = new Map<string, number>()
let socket: WebSocket | null = null
let reconnectTimer: number | undefined
let watchdogTimer: number | undefined
let reconnectAttempt = 0
let lastMessageAt = 0
let users = 0

const liveStale = computed(() => hasSnapshot.value && staleCycle.value)
const liveLoading = computed(
  () =>
    !hasSnapshot.value &&
    connectionState.value !== 'current' &&
    connectionState.value !== 'idle',
)

function sameFilter(left: LiveFilter, right: LiveFilter) {
  return (
    (left.q || '') === (right.q || '') &&
    (left.group_id || '') === (right.group_id || '') &&
    (left.character_id || '') === (right.character_id || '')
  )
}

function nextSubscriptionRevision(id: string, current?: Subscription) {
  const previous = Math.max(
    subscriptionRevisions.get(id) || 0,
    current?.revision || 0,
  )
  const next = previous + 1
  subscriptionRevisions.set(id, next)
  return next
}

function send(frame: LiveClientFrame) {
  if (!import.meta.client || !socket || socket.readyState !== WebSocket.OPEN)
    return false
  socket.send(JSON.stringify(frame))
  return true
}

function subscribe(subscription: Subscription) {
  send({
    type: 'subscribe',
    protocol_version: LIVE_PROTOCOL_VERSION,
    subscription_id: subscription.id,
    revision: subscription.revision,
    stream: subscription.stream,
    filter: subscription.filter,
  })
}

function ensureSubscription(
  id: string,
  stream: LiveStream,
  filter: LiveFilter = {},
  clear?: () => void,
) {
  if (!import.meta.client) return
  const current = subscriptions.get(id)
  if (
    current &&
    current.stream === stream &&
    sameFilter(current.filter, filter)
  ) {
    return
  }
  const subscription: Subscription = {
    id,
    stream,
    filter,
    revision: nextSubscriptionRevision(id, current),
    current: false,
    unavailable: false,
  }
  subscriptions.set(id, subscription)
  clear?.()
  if (hasSnapshot.value) connectionState.value = 'syncing'
  if (
    !send({
      type: 'subscribe',
      protocol_version: LIVE_PROTOCOL_VERSION,
      subscription_id: id,
      revision: subscription.revision,
      stream,
      filter,
    })
  ) {
    ensureConnection()
  }
}

function removeSubscription(id: string, clear?: () => void) {
  if (!import.meta.client) return
  const current = subscriptions.get(id)
  if (!current) {
    clear?.()
    return
  }
  send({
    type: 'unsubscribe',
    protocol_version: LIVE_PROTOCOL_VERSION,
    subscription_id: current.id,
    revision: current.revision,
  })
  subscriptions.delete(id)
  clear?.()
  updateCurrentState()
}

function ensureBaseSubscriptions() {
  ensureSubscription('agents', 'agents')
  ensureSubscription('fleet-characters', 'characters')
  ensureSubscription('groups', 'groups')
}

function clearCharacterListFilter() {
  removeSubscription('character-list', () => {
    characters.value = []
  })
}

function setCharacterListFilter(query: string, groupID?: string) {
  ensureSubscription(
    'character-list',
    'characters',
    {
      q: query.trim() || undefined,
      group_id: groupID || undefined,
    },
    () => {
      characters.value = []
    },
  )
}

function setCharacterDetail(characterID: string) {
  if (!characterID) {
    removeSubscription('character-detail', () => {
      characterDetail.value = null
    })
    return
  }
  ensureSubscription(
    'character-detail',
    'character',
    { character_id: characterID },
    () => {
      characterDetail.value = null
    },
  )
}

function refreshLiveData(ids?: string[]) {
  if (!import.meta.client) return
  const wanted = ids ? new Set(ids) : null
  let requested = false
  for (const subscription of subscriptions.values()) {
    if (wanted && !wanted.has(subscription.id)) continue
    subscription.current = false
    subscription.unavailable = false
    requested = true
    send({
      type: 'refresh',
      protocol_version: LIVE_PROTOCOL_VERSION,
      subscription_id: subscription.id,
      revision: subscription.revision,
    })
  }
  if (requested) {
    connectionState.value = hasSnapshot.value ? 'syncing' : 'connecting'
  }
  ensureConnection()
}

function browserLiveURL() {
  const url = new URL('/api/live', window.location.origin)
  url.protocol = window.location.protocol === 'https:' ? 'wss:' : 'ws:'
  return url.toString()
}

function ensureConnection() {
  if (!import.meta.client || users < 1) return
  if (
    socket &&
    (socket.readyState === WebSocket.OPEN ||
      socket.readyState === WebSocket.CONNECTING)
  )
    return

  clearTimeout(reconnectTimer)
  reconnectTimer = undefined
  connectionState.value =
    hasSnapshot.value || reconnectAttempt > 0 ? 'reconnecting' : 'connecting'

  const next = new WebSocket(browserLiveURL())
  socket = next

  next.addEventListener('open', () => {
    if (socket !== next) return
    lastMessageAt = Date.now()
    connectionState.value = 'syncing'
    for (const subscription of subscriptions.values()) {
      subscription.current = false
      subscription.unavailable = false
      subscribe(subscription)
    }
    startWatchdog()
  })

  next.addEventListener('message', (event) => {
    if (socket !== next || typeof event.data !== 'string') {
      next.close(1003, 'text live protocol required')
      return
    }
    lastMessageAt = Date.now()
    let frame: LiveServerFrame
    try {
      frame = JSON.parse(event.data) as LiveServerFrame
    } catch {
      next.close(1002, 'invalid live protocol')
      return
    }
    handleFrame(frame)
  })

  next.addEventListener('close', () => {
    if (socket !== next) return
    socket = null
    stopWatchdog()
    markSubscriptionsStale()
    scheduleReconnect()
  })

  next.addEventListener('error', () => {
    if (socket === next && next.readyState < WebSocket.CLOSING) {
      next.close()
    }
  })
}

function handleFrame(frame: LiveServerFrame) {
  if (frame.protocol_version !== LIVE_PROTOCOL_VERSION) {
    socket?.close(1002, 'unsupported live protocol')
    return
  }
  if (frame.type === 'heartbeat') {
    send({ type: 'heartbeat', protocol_version: LIVE_PROTOCOL_VERSION })
    return
  }

  if (!frame.subscription_id || !frame.revision || !frame.stream) return
  const subscription = subscriptions.get(frame.subscription_id)
  if (
    !subscription ||
    subscription.revision !== frame.revision ||
    subscription.stream !== frame.stream
  ) {
    // The response belongs to a superseded filter/detail revision.
    return
  }

  if (frame.type === 'subscription.unavailable') {
    subscription.current = false
    subscription.unavailable = true
    staleCycle.value = true
    connectionState.value = 'stale'
    return
  }
  if (frame.type === 'subscription.rejected') {
    subscription.current = false
    if (frame.reason !== 'obsolete_revision') {
      subscription.unavailable = true
      staleCycle.value = true
      connectionState.value = 'stale'
    }
    return
  }
  if (frame.type !== 'snapshot') return

  if (!applySnapshot(subscription, frame.data)) {
    socket?.close(1002, 'invalid live snapshot')
    return
  }
  subscription.current = true
  subscription.unavailable = false
  hasSnapshot.value = true
  updateCurrentState()
}

function applySnapshot(subscription: Subscription, data: unknown) {
  if (!data || typeof data !== 'object' || Array.isArray(data)) return false
  switch (subscription.id) {
    case 'agents': {
      const snapshot = data as AgentsSnapshot
      if (!Array.isArray(snapshot.agents)) return false
      agents.value = snapshot.agents
      return true
    }
    case 'fleet-characters': {
      const snapshot = data as CharactersSnapshot
      if (!Array.isArray(snapshot.characters)) return false
      fleetCharacters.value = snapshot.characters
      return true
    }
    case 'character-list': {
      const snapshot = data as CharactersSnapshot
      if (!Array.isArray(snapshot.characters)) return false
      characters.value = snapshot.characters
      return true
    }
    case 'groups': {
      const snapshot = data as GroupsSnapshot
      if (!Array.isArray(snapshot.groups)) return false
      groups.value = snapshot.groups
      return true
    }
    case 'character-detail': {
      const snapshot = data as CharacterSnapshot
      if (
        snapshot.character !== null &&
        (typeof snapshot.character !== 'object' ||
          Array.isArray(snapshot.character))
      )
        return false
      characterDetail.value = snapshot.character
      return true
    }
    default:
      return false
  }
}

function updateCurrentState() {
  if (subscriptions.size === 0) {
    connectionState.value = 'current'
    reconnectAttempt = 0
    return
  }
  const allCurrent = [...subscriptions.values()].every(
    (subscription) => subscription.current,
  )
  if (allCurrent) {
    connectionState.value = 'current'
    reconnectAttempt = 0
    staleCycle.value = false
  } else if (
    [...subscriptions.values()].some((subscription) => subscription.unavailable)
  ) {
    connectionState.value = 'stale'
    staleCycle.value = true
  } else if (hasSnapshot.value && connectionState.value !== 'reconnecting') {
    connectionState.value = 'syncing'
  }
}

function markSubscriptionsStale() {
  for (const subscription of subscriptions.values()) {
    subscription.current = false
  }
  staleCycle.value = true
  connectionState.value = hasSnapshot.value ? 'stale' : 'reconnecting'
}

function scheduleReconnect() {
  if (!import.meta.client || users < 1 || reconnectTimer) return
  const base = Math.min(30_000, 1000 * 2 ** Math.min(reconnectAttempt, 5))
  const delay = Math.min(30_000, base * (0.75 + Math.random() * 0.25))
  reconnectAttempt += 1
  reconnectTimer = window.setTimeout(() => {
    reconnectTimer = undefined
    ensureConnection()
  }, delay)
}

function startWatchdog() {
  stopWatchdog()
  watchdogTimer = window.setInterval(() => {
    if (
      socket?.readyState === WebSocket.OPEN &&
      Date.now() - lastMessageAt > 35_000
    ) {
      socket.close(4000, 'live heartbeat timeout')
    }
  }, 5000)
}

function stopWatchdog() {
  if (watchdogTimer) window.clearInterval(watchdogTimer)
  watchdogTimer = undefined
}

function attach() {
  if (!import.meta.client) return
  users += 1
  ensureBaseSubscriptions()
  ensureConnection()
}

function detach() {
  if (!import.meta.client) return
  users = Math.max(0, users - 1)
  if (users > 0) return
  clearTimeout(reconnectTimer)
  reconnectTimer = undefined
  stopWatchdog()
  const current = socket
  socket = null
  current?.close(1000, 'view closed')
  connectionState.value = 'idle'
}

export function useLiveData() {
  onMounted(attach)
  onUnmounted(detach)

  return {
    agents: readonly(agents),
    characters: readonly(characters),
    fleetCharacters: readonly(fleetCharacters),
    groups: readonly(groups),
    characterDetail: readonly(characterDetail),
    connectionState: readonly(connectionState),
    liveStale,
    liveLoading,
    setCharacterListFilter,
    clearCharacterListFilter,
    setCharacterDetail,
    refreshLiveData,
  }
}
