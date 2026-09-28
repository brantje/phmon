import type { ChatMessage } from '~~/shared/types/live'
import { pruneSeenChatMessageIDs } from '~/utils/pruneSeenChatMessageIDs'

export interface ChatPreferences {
  browser_notifications: boolean
  message_sound: boolean
}

const preferences = ref<ChatPreferences>({
  browser_notifications: false,
  message_sound: false,
})
const ready = ref(false)
const busy = ref(false)
const error = ref('')
let loadPromise: Promise<void> | undefined

export function useChatPreferences() {
  async function load() {
    if (loadPromise) return loadPromise
    loadPromise = (async () => {
      try {
        preferences.value = await $fetch<ChatPreferences>(
          '/api/chat/preferences',
        )
        error.value = ''
      } catch {
        error.value = 'Chat preferences are temporarily unavailable.'
      } finally {
        ready.value = true
        loadPromise = undefined
      }
    })()
    return loadPromise
  }

  async function save(next: ChatPreferences) {
    busy.value = true
    try {
      preferences.value = await $fetch<ChatPreferences>(
        '/api/chat/preferences',
        { method: 'PUT', body: next },
      )
      error.value = ''
      return true
    } catch {
      error.value = 'Could not save chat preferences.'
      return false
    } finally {
      busy.value = false
    }
  }

  async function requestBrowserPermission() {
    if (!import.meta.client || !('Notification' in window)) {
      error.value = 'This browser does not support desktop notifications.'
      return false
    }
    let permission: NotificationPermission
    try {
      permission = await Notification.requestPermission()
    } catch {
      error.value = 'Browser notification permission could not be requested.'
      return false
    }
    const enabled = permission === 'granted'
    const saved = await save({
      ...preferences.value,
      browser_notifications: enabled,
    })
    if (!saved) return false
    if (!enabled)
      error.value = 'Browser notification permission was not granted.'
    return enabled
  }

  onMounted(() => void load())
  return {
    preferences: readonly(preferences),
    ready: readonly(ready),
    busy: readonly(busy),
    error: readonly(error),
    load,
    save,
    requestBrowserPermission,
  }
}

type ActiveChat = {
  server: string
  characterID: string
  channel: string
  peer: string
} | null

const activeChat = ref<ActiveChat>(null)
const channelNames = [
  'general',
  'private',
  'party',
  'guild',
  'union',
  'global',
  'unknown',
]
const seenIDs = new Set<string>()
const initializedFeeds = new Set<string>()

function remember(id: string) {
  if (!id || seenIDs.has(id)) return false
  seenIDs.add(id)
  return true
}

function activeContains(
  message: Pick<
    ChatMessage,
    'server' | 'character_id' | 'channel' | 'peer_key'
  >,
) {
  const current = activeChat.value
  if (!current) return false
  const sameServerAndChannel =
    current.server.toLowerCase() === message.server.toLowerCase() &&
    current.channel === message.channel
  if (!sameServerAndChannel) return false
  if (message.channel === 'general' || message.channel === 'global') return true
  return (
    current.characterID === message.character_id &&
    (message.channel !== 'private' ||
      current.peer.toLowerCase() === (message.peer_key || '').toLowerCase())
  )
}

function playMessageSound() {
  if (!import.meta.client) return
  const audio = new Audio('/sounds/chat-message.wav')
  audio.volume = 0.35
  void audio.play().catch(() => {})
}

function dispatchMessage(
  message: Pick<
    ChatMessage,
    'server' | 'character_id' | 'channel' | 'peer_key' | 'sender' | 'message'
  > & { id: string },
) {
  if (activeContains(message) && document.visibilityState === 'visible') return
  const prefs = preferences.value
  if (prefs.message_sound && document.visibilityState !== 'visible')
    playMessageSound()
  if (
    prefs.browser_notifications &&
    'Notification' in window &&
    Notification.permission === 'granted' &&
    document.visibilityState !== 'visible'
  ) {
    const title =
      message.channel === 'private'
        ? `${message.sender || message.peer_key || 'Private'} · ${message.server}`
        : `${message.channel[0]?.toUpperCase()}${message.channel.slice(1)} · ${message.server}`
    const notice = new Notification(title, {
      body: message.message.slice(0, 240),
      tag: `phmon-chat-${message.id}`,
    })
    notice.onclick = () => {
      window.focus()
      navigateTo('/chat')
      notice.close()
    }
  }
}

export function useChatNotifications() {
  const { chatFeeds, setChatFeed, clearChatFeed } = useLiveData()
  const { serverScope } = useServerScope()
  const prefs = useChatPreferences()
  let active = false

  function observe() {
    const currentSnapshotIDs = new Set<string>()
    for (const snapshot of Object.values(chatFeeds.value)) {
      for (const item of snapshot.page.messages)
        currentSnapshotIDs.add(item.event_id || item.message_id)
      for (const contact of snapshot.contacts)
        currentSnapshotIDs.add(contact.last_message_id)
    }

    for (const channel of channelNames) {
      const id = `chat-notify-${channel}`
      const snapshot = chatFeeds.value[id]
      if (!snapshot) continue
      if (!initializedFeeds.has(id)) {
        initializedFeeds.add(id)
        for (const item of snapshot.page.messages)
          remember(item.event_id || item.message_id)
        if (channel === 'private') {
          for (const contact of snapshot.contacts)
            remember(contact.last_message_id)
        }
        continue
      }
      for (const item of snapshot.page.messages) {
        const identity = item.event_id || item.message_id
        if (item.direction === 'inbound' && remember(identity)) {
          dispatchMessage({ ...item, id: identity })
        } else {
          remember(identity)
        }
      }
      if (channel === 'private') {
        for (const contact of snapshot.contacts) {
          if (
            contact.last_direction !== 'inbound' ||
            !remember(contact.last_message_id)
          )
            continue
          dispatchMessage({
            id: contact.last_message_id,
            server: contact.server,
            character_id: contact.character_id,
            channel: 'private',
            peer_key: contact.peer_key,
            sender: contact.peer_name,
            message: contact.last_message,
          })
        }
      }
    }
    pruneSeenChatMessageIDs(seenIDs, currentSnapshotIDs)
  }

  function start() {
    if (!import.meta.client) return
    active = true
    for (const channel of channelNames) {
      setChatFeed(`chat-notify-${channel}`, {
        server: serverScope.value === 'all' ? undefined : serverScope.value,
        channel,
        limit: 50,
      })
    }
    void prefs.load()
  }

  function stop() {
    if (!active) return
    active = false
    for (const channel of channelNames) clearChatFeed(`chat-notify-${channel}`)
    initializedFeeds.clear()
  }

  watch(chatFeeds, observe, { deep: true })
  watch(serverScope, () => {
    if (active) {
      for (const channel of channelNames)
        initializedFeeds.delete(`chat-notify-${channel}`)
      start()
    }
  })
  onBeforeUnmount(stop)

  return { start, stop, preferences: prefs.preferences }
}

export function setActiveChatConversation(value: ActiveChat) {
  activeChat.value = value
}
