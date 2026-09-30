import type { AgentView } from '~~/shared/types/agent'
import {
  LIVE_PROTOCOL_VERSION,
  type AgentsSnapshot,
  type CharacterGroup,
  type CharacterSnapshot,
  type CharacterResourcesView,
  type EventPage,
  type CharactersSnapshot,
  type CharacterView,
  type CommandsSnapshot,
  type CommandFanOutLiveFeed,
  type ControlsTargetsSnapshot,
  type ChatSnapshot,
  type ControlsSnapshot,
  type RemoteCommand,
  type GroupsSnapshot,
  type MapSnapshot,
  type LiveClientFrame,
  type LiveConnectionState,
  type LiveFilter,
  type LiveServerFrame,
  type LiveStream,
} from '~~/shared/types/live'
import { mapSnapshotMatchesScope } from '~/utils/mapRefresh'
import { chunkFanOutValues } from '~/utils/commandFanOut'

type Subscription = {
  id: string
  revision: number
  stream: LiveStream
  filter: LiveFilter
  current: boolean
  unavailable: boolean
  countsForConnection: boolean
}
type CommandFanOutOwner = {
  controlChunks: string[][]
  controlIndex: number
  commandChunks: string[][]
  commandIndex: number
  commandFreshChunks: Set<number>
  commandRotationTimer?: number
}
type CachedDeathState = {
  session_id: string
  dead: boolean
  state_updated_at: string
}

const agents = ref<AgentView[]>([])
const characters = ref<CharacterView[]>([])
const fleetCharacters = ref<CharacterView[]>([])
const cachedDeathStates = ref<Record<string, CachedDeathState>>({})
const groups = ref<CharacterGroup[]>([])
const characterDetail = ref<CharacterView | null>(null)
const characterResources = ref<Record<string, CharacterResourcesView>>({})
const commandHistory = ref<RemoteCommand[]>([])
const characterControls = ref<ControlsSnapshot | null>(null)
const eventFeeds = ref<Record<string, EventPage>>({})
const chatFeeds = ref<Record<string, ChatSnapshot>>({})
const chatFeedCurrent = ref<Record<string, boolean>>({})
const mapFeeds = ref<Record<string, MapSnapshot>>({})
const mapFeedCurrent = ref<Record<string, boolean>>({})
const commandFanOutFeeds = ref<Record<string, CommandFanOutLiveFeed>>({})
const connectionState = ref<LiveConnectionState>('idle')
const freshnessNow = ref(Date.now())
const hasSnapshot = ref(false)
const staleCycle = ref(false)

const subscriptions = new Map<string, Subscription>()
const subscriptionRevisions = new Map<string, number>()
const commandFanOutOwners = new Map<string, CommandFanOutOwner>()
let socket: WebSocket | null = null
let reconnectTimer: number | undefined
let watchdogTimer: number | undefined
let reconnectAttempt = 0
let lastMessageAt = 0
let liveDataStarted = false

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
    (left.character_id || '') === (right.character_id || '') &&
    (left.command_name || '') === (right.command_name || '') &&
    (left.command_state || '') === (right.command_state || '') &&
    (left.limit || 0) === (right.limit || 0) &&
    (left.resource_keys || []).join('\u0000') ===
      (right.resource_keys || []).join('\u0000') &&
    (left.server || '') === (right.server || '') &&
    (left.kind || '') === (right.kind || '') &&
    (left.category || '') === (right.category || '') &&
    (left.item || '') === (right.item || '') &&
    (left.from || '') === (right.from || '') &&
    (left.to || '') === (right.to || '') &&
    (left.channel || '') === (right.channel || '') &&
    (left.peer || '') === (right.peer || '') &&
    (left.area || '') === (right.area || '') &&
    (left.floor || '') === (right.floor || '') &&
    (left.region || 0) === (right.region || 0) &&
    (left.event_id || '') === (right.event_id || '') &&
    (left.character_ids || []).join('\u0000') ===
      (right.character_ids || []).join('\u0000') &&
    (left.idempotency_keys || []).join('\u0000') ===
      (right.idempotency_keys || []).join('\u0000') &&
    (left.cursor || '') === (right.cursor || '')
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

function deathStateKey(character: CharacterView) {
  return `${character.server.toLowerCase()}\u0000${character.character_id}`
}

function rememberDeathStates(characters: CharacterView[]) {
  let nextCache = cachedDeathStates.value
  let copiedCache = false
  const mutableCache = () => {
    if (!copiedCache) {
      nextCache = { ...nextCache }
      copiedCache = true
    }
    return nextCache
  }

  const result = characters.map((character) => {
    const sessionID = character.session_id
    if (!sessionID) return character

    const key = deathStateKey(character)
    let cached = nextCache[key]
    if (cached && cached.session_id !== sessionID) {
      Reflect.deleteProperty(mutableCache(), key)
      cached = undefined
    }

    const observedAt = character.state_updated_at
    const observedTime = observedAt ? Date.parse(observedAt) : Number.NaN
    if (typeof character.dead === 'boolean' && Number.isFinite(observedTime)) {
      const cachedTime = cached
        ? Date.parse(cached.state_updated_at)
        : Number.NEGATIVE_INFINITY
      if (!cached || observedTime >= cachedTime) {
        cached = {
          session_id: sessionID,
          dead: character.dead,
          state_updated_at: observedAt!,
        }
        mutableCache()[key] = cached
      }
    }

    if (!character.online || !cached) return character
    const currentTime = Number.isFinite(observedTime)
      ? observedTime
      : Number.NEGATIVE_INFINITY
    const cachedTime = Date.parse(cached.state_updated_at)
    if (typeof character.dead === 'boolean' && currentTime >= cachedTime)
      return character

    // Keep the timestamp of the last boolean sample. A newer partial snapshot
    // must not make an old Alive/Dead value look freshly observed.
    return {
      ...character,
      dead: cached.dead,
      state_updated_at: cached.state_updated_at,
    }
  })

  if (copiedCache) cachedDeathStates.value = nextCache
  return result
}

function rememberDeathState(character: CharacterView) {
  return rememberDeathStates([character])[0]!
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
  countsForConnection = true,
) {
  if (!import.meta.client) return
  const current = subscriptions.get(id)
  if (
    current &&
    current.stream === stream &&
    sameFilter(current.filter, filter) &&
    current.countsForConnection === countsForConnection
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
    countsForConnection,
  }
  subscriptions.set(id, subscription)
  clear?.()
  if (hasSnapshot.value && stream !== 'chat' && countsForConnection)
    connectionState.value = 'syncing'
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
  // Keep the shared group snapshot server-neutral. Individual screens filter
  // members to their own server scope; this also supports map route overrides.
  ensureSubscription('groups', 'groups')
}

function clearCharacterListFilter() {
  removeSubscription('character-list', () => {
    characters.value = []
  })
}

function setCharacterListFilter(
  query: string,
  groupID?: string,
  server?: string,
) {
  ensureSubscription(
    'character-list',
    'characters',
    {
      q: query.trim() || undefined,
      group_id: groupID || undefined,
      server: server || undefined,
    },
    () => {
      characters.value = []
    },
  )
}

function setCharacterDetail(characterID: string, server?: string) {
  if (!characterID) {
    removeSubscription('character-detail', () => {
      characterDetail.value = null
    })
    return
  }
  ensureSubscription(
    'character-detail',
    'character',
    { character_id: characterID, server: server || undefined },
    () => {
      characterDetail.value = null
    },
  )
}

function setCharacterResources(
  characterID: string,
  resourceKeys: string[],
  subscriptionID: string,
) {
  if (!characterID) return
  ensureSubscription(subscriptionID, 'resources', {
    character_id: characterID,
    resource_keys: resourceKeys,
  })
}

function clearCharacterResources(characterID: string, subscriptionID: string) {
  if (!characterID) return
  removeSubscription(subscriptionID)
}

function setCharacterCommands(
  characterID: string,
  commandName = '',
  commandState = '',
) {
  if (!characterID) return
  ensureSubscription(
    'character-commands',
    'commands',
    {
      character_id: characterID,
      command_name: commandName || undefined,
      command_state: commandState || undefined,
      limit: 25,
    },
    () => {
      commandHistory.value = []
    },
  )
}

function setCharacterControls(characterID: string) {
  if (!characterID) return
  ensureSubscription(
    'character-controls',
    'controls',
    { character_id: characterID },
    () => {
      characterControls.value = null
    },
  )
}

function setEventFeed(subscriptionID: string, filter: LiveFilter) {
  ensureSubscription(subscriptionID, 'events', filter, () => {
    removeEventFeedSnapshot(subscriptionID)
  })
}

function clearEventFeed(subscriptionID: string) {
  removeSubscription(subscriptionID, () => {
    removeEventFeedSnapshot(subscriptionID)
  })
}

function setMapFeed(subscriptionID: string, filter: LiveFilter) {
  ensureSubscription(subscriptionID, 'map', filter, () => {
    mapFeedCurrent.value = { ...mapFeedCurrent.value, [subscriptionID]: false }
    const existing = mapFeeds.value[subscriptionID]
    if (!existing || !mapSnapshotMatchesScope(existing, filter)) {
      mapFeeds.value = Object.fromEntries(
        Object.entries(mapFeeds.value).filter(([id]) => id !== subscriptionID),
      )
    }
  })
}

function clearMapFeed(subscriptionID: string) {
  removeSubscription(subscriptionID, () => {
    mapFeeds.value = Object.fromEntries(
      Object.entries(mapFeeds.value).filter(([id]) => id !== subscriptionID),
    )
    mapFeedCurrent.value = Object.fromEntries(
      Object.entries(mapFeedCurrent.value).filter(
        ([id]) => id !== subscriptionID,
      ),
    )
  })
}

function setChatFeed(subscriptionID: string, filter: LiveFilter) {
  const previousFilter = subscriptions.get(subscriptionID)?.filter
  const sameContactScope =
    previousFilter?.server === filter.server &&
    previousFilter?.character_id === filter.character_id
  ensureSubscription(subscriptionID, 'chat', filter, () => {
    chatFeedCurrent.value = {
      ...chatFeedCurrent.value,
      [subscriptionID]: false,
    }
    const previous = chatFeeds.value[subscriptionID]
    if (previous && sameContactScope) {
      chatFeeds.value = {
        ...chatFeeds.value,
        [subscriptionID]: {
          ...previous,
          channel: filter.channel || previous.channel,
          page: { messages: [], has_older: false },
        },
      }
      return
    }
    chatFeeds.value = Object.fromEntries(
      Object.entries(chatFeeds.value).filter(([id]) => id !== subscriptionID),
    )
  })
}

function clearChatFeed(subscriptionID: string) {
  removeSubscription(subscriptionID, () => {
    chatFeeds.value = Object.fromEntries(
      Object.entries(chatFeeds.value).filter(([id]) => id !== subscriptionID),
    )
    chatFeedCurrent.value = Object.fromEntries(
      Object.entries(chatFeedCurrent.value).filter(
        ([id]) => id !== subscriptionID,
      ),
    )
  })
}

function applyChatReadState(
  subscriptionID: string,
  state: Pick<ChatSnapshot, 'contacts' | 'unread_by_channel'>,
) {
  const current = chatFeeds.value[subscriptionID]
  if (!current || !Array.isArray(state.contacts) || !state.unread_by_channel)
    return
  chatFeeds.value = {
    ...chatFeeds.value,
    [subscriptionID]: {
      ...current,
      contacts: state.contacts,
      unread_by_channel: state.unread_by_channel,
    },
  }
}

function removeEventFeedSnapshot(subscriptionID: string) {
  const next: Record<string, EventPage> = {}
  for (const [id, page] of Object.entries(eventFeeds.value)) {
    if (id !== subscriptionID) next[id] = page
  }
  eventFeeds.value = next
}

function clearCharacterCommandSubscriptions() {
  removeSubscription('character-commands', () => {
    commandHistory.value = []
  })
  removeSubscription('character-controls', () => {
    characterControls.value = null
  })
}

function fanOutSubscriptionID(ownerID: string, kind: 'controls' | 'commands') {
  return `fanout-${ownerID}-${kind}`
}

function fanOutOwner(ownerID: string) {
  let owner = commandFanOutOwners.get(ownerID)
  if (!owner) {
    owner = {
      controlChunks: [],
      controlIndex: 0,
      commandChunks: [],
      commandIndex: 0,
      commandFreshChunks: new Set(),
    }
    commandFanOutOwners.set(ownerID, owner)
  }
  if (!commandFanOutFeeds.value[ownerID]) {
    commandFanOutFeeds.value = {
      ...commandFanOutFeeds.value,
      [ownerID]: {
        targets: {},
        controls_current: false,
        controls_unavailable: false,
        commands: {},
        commands_current: false,
        commands_unavailable: false,
      },
    }
  }
  return owner
}

function setCommandFanOutTargets(ownerID: string, characterIDs: string[]) {
  const owner = fanOutOwner(ownerID)
  owner.controlChunks = chunkFanOutValues([...new Set(characterIDs)])
  owner.controlIndex = 0
  const feed = commandFanOutFeeds.value[ownerID]!
  feed.targets = {}
  feed.controls_current = owner.controlChunks.length === 0
  feed.controls_unavailable = false
  commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
  if (!owner.controlChunks.length) {
    removeSubscription(fanOutSubscriptionID(ownerID, 'controls'))
    return
  }
  const id = fanOutSubscriptionID(ownerID, 'controls')
  const previousRevision = subscriptions.get(id)?.revision
  subscribeFanOutChunk(ownerID, 'controls')
  if (subscriptions.get(id)?.revision === previousRevision)
    refreshLiveData([id])
}

function refreshCommandFanOutTargets(ownerID: string) {
  const owner = commandFanOutOwners.get(ownerID)
  const feed = commandFanOutFeeds.value[ownerID]
  if (!owner || !feed) return
  owner.controlIndex = 0
  feed.controls_current = owner.controlChunks.length === 0
  feed.controls_unavailable = false
  commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
  if (!owner.controlChunks.length) return
  const id = fanOutSubscriptionID(ownerID, 'controls')
  subscribeFanOutChunk(ownerID, 'controls')
  refreshLiveData([id])
}

function trackCommandFanOut(ownerID: string, keys: string[]) {
  const owner = fanOutOwner(ownerID)
  const feed = commandFanOutFeeds.value[ownerID]!
  const allKeys = [...new Set(keys)]
  const keySet = new Set(allKeys)
  feed.commands = Object.fromEntries(
    Object.entries(feed.commands).filter(([key]) => keySet.has(key)),
  )
  owner.commandChunks = chunkFanOutValues(allKeys)
  owner.commandIndex = 0
  owner.commandFreshChunks.clear()
  feed.commands_current = owner.commandChunks.length === 0
  feed.commands_unavailable = false
  commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
  if (!owner.commandChunks.length) {
    removeSubscription(fanOutSubscriptionID(ownerID, 'commands'))
    return
  }
  subscribeFanOutChunk(ownerID, 'commands')
}

function subscribeFanOutChunk(ownerID: string, kind: 'controls' | 'commands') {
  const owner = commandFanOutOwners.get(ownerID)
  if (!owner) return
  const chunks = kind === 'controls' ? owner.controlChunks : owner.commandChunks
  const index = kind === 'controls' ? owner.controlIndex : owner.commandIndex
  const values = chunks[index]
  if (!values) return
  const id = fanOutSubscriptionID(ownerID, kind)
  const stream: LiveStream = kind
  const filter: LiveFilter =
    kind === 'controls'
      ? { character_ids: values }
      : { idempotency_keys: values }
  ensureSubscription(id, stream, filter, undefined, false)
}

function commandStateRank(state: RemoteCommand['state']) {
  switch (state) {
    case 'queued':
      return 0
    case 'dispatching':
      return 1
    case 'sent':
      return 2
    case 'acknowledged':
      return 3
    case 'unknown':
      return 4
    case 'completed':
    case 'failed':
    case 'expired':
      return 5
  }
}

function applyCommandFanOutSnapshot(subscription: Subscription, data: unknown) {
  const suffix = subscription.stream === 'controls' ? '-controls' : '-commands'
  const ownerID =
    subscription.id.startsWith('fanout-') && subscription.id.endsWith(suffix)
      ? subscription.id.slice('fanout-'.length, -suffix.length)
      : ''
  const owner = commandFanOutOwners.get(ownerID)
  const feed = commandFanOutFeeds.value[ownerID]
  if (!owner || !feed) return false

  if (subscription.stream === 'controls') {
    const snapshot = data as ControlsTargetsSnapshot
    const expected = owner.controlChunks[owner.controlIndex] || []
    if (
      !Array.isArray(snapshot.targets) ||
      snapshot.targets.length !== expected.length
    )
      return false
    const targets = { ...feed.targets }
    for (let index = 0; index < expected.length; index++) {
      const item = snapshot.targets[index]
      if (!item || item.character_id !== expected[index]) return false
      targets[item.character_id] = item
    }
    feed.targets = targets
    feed.controls_unavailable = false
    if (owner.controlIndex + 1 < owner.controlChunks.length) {
      owner.controlIndex++
      feed.controls_current = false
      commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
      subscribeFanOutChunk(ownerID, 'controls')
    } else {
      feed.controls_current = true
      feed.updated_at = Date.now()
      commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
    }
    return true
  }

  const snapshot = data as CommandsSnapshot
  const expected = owner.commandChunks[owner.commandIndex] || []
  if (!Array.isArray(snapshot.commands)) return false
  const commands = { ...feed.commands }
  for (const command of snapshot.commands) {
    if (
      !command?.command_id ||
      !command.idempotency_key ||
      !expected.includes(command.idempotency_key)
    )
      return false
    const previous = commands[command.idempotency_key]
    if (
      !previous ||
      commandStateRank(command.state) > commandStateRank(previous.state)
    )
      commands[command.idempotency_key] = command
  }
  feed.commands = commands
  owner.commandFreshChunks.add(owner.commandIndex)
  feed.commands_current =
    owner.commandFreshChunks.size === owner.commandChunks.length
  feed.commands_unavailable = false
  feed.updated_at = Date.now()
  commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
  if (owner.commandChunks.length > 1 && !owner.commandRotationTimer) {
    owner.commandRotationTimer = window.setTimeout(() => {
      owner.commandRotationTimer = undefined
      owner.commandIndex = (owner.commandIndex + 1) % owner.commandChunks.length
      if (owner.commandIndex === 0) owner.commandFreshChunks.clear()
      feed.commands_current = false
      commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
      subscribeFanOutChunk(ownerID, 'commands')
    }, 1000)
  }
  return true
}

function markCommandFanOutUnavailable(subscriptionID: string) {
  const suffix = subscriptionID.endsWith('-controls')
    ? '-controls'
    : '-commands'
  const ownerID = subscriptionID.slice('fanout-'.length, -suffix.length)
  const feed = commandFanOutFeeds.value[ownerID]
  if (!feed) return
  if (suffix === '-controls') {
    feed.controls_current = false
    feed.controls_unavailable = true
  } else {
    feed.commands_current = false
    feed.commands_unavailable = true
  }
  commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
}

function clearCommandFanOutOwner(ownerID: string) {
  const owner = commandFanOutOwners.get(ownerID)
  if (import.meta.client && owner?.commandRotationTimer)
    window.clearTimeout(owner.commandRotationTimer)
  removeSubscription(fanOutSubscriptionID(ownerID, 'controls'))
  removeSubscription(fanOutSubscriptionID(ownerID, 'commands'))
  commandFanOutOwners.delete(ownerID)
  const { [ownerID]: _removed, ...remaining } = commandFanOutFeeds.value
  commandFanOutFeeds.value = remaining
}

function refreshLiveData(ids?: string[]) {
  if (!import.meta.client) return
  const wanted = ids ? new Set(ids) : null
  let requested = false
  for (const subscription of subscriptions.values()) {
    if (wanted && !wanted.has(subscription.id)) continue
    subscription.current = false
    subscription.unavailable = false
    requested ||= subscription.countsForConnection
    if (subscription.id.startsWith('fanout-')) {
      const suffix = subscription.id.endsWith('-controls')
        ? '-controls'
        : '-commands'
      const ownerID = subscription.id.slice('fanout-'.length, -suffix.length)
      const feed = commandFanOutFeeds.value[ownerID]
      if (feed) {
        if (suffix === '-controls') {
          feed.controls_current = false
          feed.controls_unavailable = false
        } else {
          feed.commands_current = false
          feed.commands_unavailable = false
        }
        commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
      }
    }
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
  if (!import.meta.client || !liveDataStarted) return
  if (socket && socket.readyState !== WebSocket.CLOSED) return

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
    for (const [ownerID, owner] of commandFanOutOwners) {
      owner.controlIndex = 0
      owner.commandIndex = 0
      owner.commandFreshChunks.clear()
      const feed = commandFanOutFeeds.value[ownerID]
      if (feed) {
        feed.controls_current = owner.controlChunks.length === 0
        feed.commands_current = owner.commandChunks.length === 0
        feed.controls_unavailable = false
        feed.commands_unavailable = false
      }
      subscribeFanOutChunk(ownerID, 'controls')
      subscribeFanOutChunk(ownerID, 'commands')
    }
    commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
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
    if (subscription.id.startsWith('fanout-')) {
      markCommandFanOutUnavailable(subscription.id)
    } else {
      staleCycle.value = true
      connectionState.value = 'stale'
    }
    if (subscription.stream === 'map') {
      mapFeedCurrent.value = {
        ...mapFeedCurrent.value,
        [subscription.id]: false,
      }
    }
    return
  }
  if (frame.type === 'subscription.rejected') {
    subscription.current = false
    if (frame.reason !== 'obsolete_revision') {
      subscription.unavailable = true
      if (subscription.id.startsWith('fanout-')) {
        markCommandFanOutUnavailable(subscription.id)
      } else {
        staleCycle.value = true
        connectionState.value = 'stale'
      }
    }
    if (subscription.stream === 'map') {
      mapFeedCurrent.value = {
        ...mapFeedCurrent.value,
        [subscription.id]: false,
      }
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
  if (subscription.id.startsWith('fanout-'))
    return applyCommandFanOutSnapshot(subscription, data)
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
      fleetCharacters.value = rememberDeathStates(snapshot.characters)
      return true
    }
    case 'character-list': {
      const snapshot = data as CharactersSnapshot
      if (!Array.isArray(snapshot.characters)) return false
      characters.value = rememberDeathStates(snapshot.characters)
      return true
    }
    case 'groups': {
      const snapshot = data as GroupsSnapshot
      if (!Array.isArray(snapshot.groups)) return false
      groups.value = snapshot.groups.map((group) => ({
        ...group,
        members: rememberDeathStates(group.members),
      }))
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
        ? rememberDeathState(snapshot.character)
        : null
      return true
    }
    case 'character-commands': {
      const snapshot = data as CommandsSnapshot
      if (!Array.isArray(snapshot.commands)) return false
      commandHistory.value = snapshot.commands
      return true
    }
    case 'character-controls': {
      const snapshot = data as ControlsSnapshot
      if (
        typeof snapshot.character_id !== 'string' ||
        typeof snapshot.session_id !== 'string' ||
        !snapshot.capabilities ||
        typeof snapshot.capabilities !== 'object'
      )
        return false
      characterControls.value = snapshot
      return true
    }
    default: {
      if (subscription.stream === 'events') {
        const page = data as EventPage
        if (
          !Array.isArray(page.events) ||
          typeof page.total !== 'number' ||
          page.events.some(
            (item) =>
              !item ||
              typeof item !== 'object' ||
              typeof item.event_id !== 'string' ||
              typeof item.character_id !== 'string' ||
              typeof item.occurred_at !== 'string',
          )
        )
          return false
        eventFeeds.value = { ...eventFeeds.value, [subscription.id]: page }
        return true
      }
      if (subscription.stream === 'resources') {
        const snapshot = data as CharacterResourcesView
        if (
          typeof snapshot.character_id !== 'string' ||
          snapshot.character_id !== subscription.filter.character_id ||
          !snapshot.resources ||
          typeof snapshot.resources !== 'object' ||
          Array.isArray(snapshot.resources)
        )
          return false
        const current = characterResources.value[snapshot.character_id]
        characterResources.value = {
          ...characterResources.value,
          [snapshot.character_id]: {
            ...snapshot,
            revision: Math.max(current?.revision || 0, snapshot.revision),
            resources: { ...current?.resources, ...snapshot.resources },
          },
        }
        return true
      }
      if (subscription.stream === 'map') {
        const snapshot = data as MapSnapshot
        if (
          typeof snapshot.server !== 'string' ||
          snapshot.server.toLowerCase() !==
            (subscription.filter.server || '').toLowerCase() ||
          !Array.isArray(snapshot.characters) ||
          !Array.isArray(snapshot.monsters) ||
          !Array.isArray(snapshot.events) ||
          !snapshot.academy ||
          typeof snapshot.academy.status !== 'string' ||
          !Array.isArray(snapshot.academy.members)
        )
          return false
        if (
          snapshot.training_areas &&
          (typeof snapshot.training_areas.status !== 'string' ||
            !Array.isArray(snapshot.training_areas.areas))
        )
          snapshot.training_areas = undefined
        if (
          snapshot.npcs &&
          (typeof snapshot.npcs.status !== 'string' ||
            !Array.isArray(snapshot.npcs.npcs))
        )
          snapshot.npcs = undefined
        mapFeeds.value = { ...mapFeeds.value, [subscription.id]: snapshot }
        mapFeedCurrent.value = {
          ...mapFeedCurrent.value,
          [subscription.id]: true,
        }
        return true
      }
      if (subscription.stream === 'chat') {
        const snapshot = data as ChatSnapshot
        if (
          !Array.isArray(snapshot.contacts) ||
          !snapshot.page ||
          !Array.isArray(snapshot.page.messages) ||
          snapshot.page.messages.some(
            (message) =>
              !message ||
              typeof message.message_id !== 'string' ||
              typeof message.message !== 'string' ||
              typeof message.occurred_at !== 'string',
          ) ||
          typeof snapshot.channel !== 'string' ||
          !snapshot.unread_by_channel ||
          typeof snapshot.unread_by_channel !== 'object'
        )
          return false
        chatFeeds.value = { ...chatFeeds.value, [subscription.id]: snapshot }
        chatFeedCurrent.value = {
          ...chatFeedCurrent.value,
          [subscription.id]: true,
        }
        return true
      }
      return false
    }
  }
}

function updateCurrentState() {
  if (subscriptions.size === 0) {
    connectionState.value = 'current'
    reconnectAttempt = 0
    return
  }
  const connectionSubscriptions = [...subscriptions.values()].filter(
    (subscription) =>
      subscription.stream !== 'chat' && subscription.countsForConnection,
  )
  const allCurrent = connectionSubscriptions.every(
    (subscription) => subscription.current,
  )
  if (allCurrent) {
    connectionState.value = 'current'
    reconnectAttempt = 0
    staleCycle.value = false
  } else if (
    connectionSubscriptions.some((subscription) => subscription.unavailable)
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
    if (subscription.stream === 'map') {
      mapFeedCurrent.value = {
        ...mapFeedCurrent.value,
        [subscription.id]: false,
      }
    }
  }
  for (const owner of commandFanOutOwners.values()) {
    if (owner.commandRotationTimer)
      window.clearTimeout(owner.commandRotationTimer)
    owner.commandRotationTimer = undefined
    owner.controlIndex = 0
    owner.commandIndex = 0
    owner.commandFreshChunks.clear()
  }
  for (const feed of Object.values(commandFanOutFeeds.value)) {
    feed.controls_current = false
    feed.commands_current = false
  }
  commandFanOutFeeds.value = { ...commandFanOutFeeds.value }
  staleCycle.value = true
  connectionState.value = hasSnapshot.value ? 'stale' : 'reconnecting'
}

function scheduleReconnect() {
  if (!import.meta.client || !liveDataStarted || reconnectTimer) return
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
    freshnessNow.value = Date.now()
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

function startLiveData() {
  if (!import.meta.client) return
  if (!liveDataStarted) {
    liveDataStarted = true
    ensureBaseSubscriptions()
  }
  ensureConnection()
}

function stopLiveData() {
  if (!import.meta.client || !liveDataStarted) return
  liveDataStarted = false
  clearTimeout(reconnectTimer)
  reconnectTimer = undefined
  stopWatchdog()
  const current = socket
  socket = null
  current?.close(1000, 'view closed')
  connectionState.value = 'idle'
  cachedDeathStates.value = {}
}

export function useLiveData() {
  return {
    agents: readonly(agents),
    characters: readonly(characters),
    fleetCharacters: readonly(fleetCharacters),
    groups: readonly(groups),
    characterDetail: readonly(characterDetail),
    characterResources: readonly(characterResources),
    commandHistory: readonly(commandHistory),
    characterControls: readonly(characterControls),
    eventFeeds: readonly(eventFeeds),
    chatFeeds: readonly(chatFeeds),
    chatFeedCurrent: readonly(chatFeedCurrent),
    mapFeeds: readonly(mapFeeds),
    mapFeedCurrent: readonly(mapFeedCurrent),
    commandFanOutFeeds: readonly(commandFanOutFeeds),
    connectionState: readonly(connectionState),
    freshnessNow: readonly(freshnessNow),
    liveStale,
    liveLoading,
    startLiveData,
    stopLiveData,
    setCharacterListFilter,
    clearCharacterListFilter,
    setCharacterDetail,
    setCharacterResources,
    clearCharacterResources,
    setCharacterCommands,
    setCharacterControls,
    setEventFeed,
    clearEventFeed,
    setMapFeed,
    clearMapFeed,
    setChatFeed,
    clearChatFeed,
    applyChatReadState,
    clearCharacterCommandSubscriptions,
    setCommandFanOutTargets,
    refreshCommandFanOutTargets,
    trackCommandFanOut,
    clearCommandFanOutOwner,
    refreshLiveData,
  }
}
