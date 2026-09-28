<script setup lang="ts">
import type { ChatMessage, ChatSnapshot } from '~~/shared/types/live'
import { setActiveChatConversation } from '~/composables/useChatPreferences'

type ChatChannel =
  'general' | 'private' | 'party' | 'guild' | 'union' | 'global' | 'unknown'

const channels: { key: ChatChannel; label: string }[] = [
  { key: 'general', label: 'General' },
  { key: 'private', label: 'Private' },
  { key: 'party', label: 'Party' },
  { key: 'guild', label: 'Guild' },
  { key: 'union', label: 'Union' },
  { key: 'global', label: 'Global' },
]
const feedID = 'chat-page-main'
const {
  fleetCharacters,
  characterControls,
  commandHistory,
  chatFeeds,
  chatFeedCurrent,
  setCharacterControls,
  setCharacterCommands,
  clearCharacterCommandSubscriptions,
  setChatFeed,
  clearChatFeed,
  applyChatReadState,
  connectionState,
  liveStale,
  liveLoading,
} = useLiveData()
const { serverScope, matchesServer } = useServerScope()
const route = useRoute()
const mode = useCookie<'easy' | 'advanced'>('phmon-mode', {
  default: () => 'easy',
})
const advancedMode = computed(() => mode.value === 'advanced')
const visibleChannels = computed(() =>
  advancedMode.value
    ? [...channels, { key: 'unknown' as const, label: 'Unknown' }]
    : channels,
)
const selectedCharacterID = ref('')
const activeChannel = ref<ChatChannel>('general')
const privatePeer = ref('')
const recipientDraft = ref('')
const messageDraft = ref('')
const olderMessages = ref<ChatMessage[]>([])
const olderCursor = ref('')
const hasOlder = ref(false)
const loadingOlder = ref(false)
const sending = ref(false)
const sendError = ref('')
const sendState = ref('')
const showGlobalConfirmation = ref(false)
const showContactsMobile = ref(true)
const emojiOpen = ref(false)
const timeline = ref<HTMLElement | null>(null)
const atLatest = ref(true)
const lastReadMessageID = ref('')
const selectableCharacters = computed(() =>
  fleetCharacters.value.filter((item) => matchesServer(item.server)),
)
const selectedCharacter = computed(
  () =>
    selectableCharacters.value.find(
      (item) => item.character_id === selectedCharacterID.value,
    ) || null,
)
const snapshot = computed(() => chatFeeds.value[feedID])
const chatSnapshotCurrent = computed(
  () => chatFeedCurrent.value[feedID] === true,
)
const contacts = computed(() => snapshot.value?.contacts || [])
const unread = computed(() => snapshot.value?.unread_by_channel || {})
const page = computed(() => snapshot.value?.page)
const privatePeerKey = computed(() => privatePeer.value.trim().toLowerCase())
const messages = computed(() => {
  const combined = [...olderMessages.value, ...(page.value?.messages || [])]
  const seen = new Set<string>()
  return combined.filter(
    (item) => !seen.has(item.message_id) && seen.add(item.message_id),
  )
})
const chatCapability = computed(() => {
  const controls = characterControls.value
  if (
    !selectedCharacter.value ||
    controls?.character_id !== selectedCharacter.value.character_id ||
    controls.session_id !== selectedCharacter.value.session_id
  )
    return undefined
  return controls.capabilities['chat.send']
})
const canSend = computed(() => {
  const character = selectedCharacter.value
  if (
    !character?.online ||
    !character.session_id ||
    liveStale.value ||
    !chatCapability.value?.supported
  )
    return false
  if (
    !(chatCapability.value.modes || []).includes(activeChannel.value) ||
    activeChannel.value === 'unknown'
  )
    return false
  if (activeChannel.value === 'private' && !privatePeer.value.trim())
    return false
  return (
    !!messageDraft.value.trim() &&
    draftByteLength.value <= 2048 &&
    !sending.value
  )
})
const draftByteLength = computed(
  () => new TextEncoder().encode(messageDraft.value).byteLength,
)
const sendDisabledReason = computed(() => {
  if (!selectedCharacter.value) return 'Choose a character to send from.'
  if (!selectedCharacter.value.online || !selectedCharacter.value.session_id)
    return 'This character is offline.'
  if (liveStale.value) return 'Live session data is stale. Sending is paused.'
  if (!chatCapability.value)
    return 'Waiting for this session to report chat capabilities.'
  if (
    !chatCapability.value.supported ||
    !(chatCapability.value.modes || []).includes(activeChannel.value)
  )
    return (
      chatCapability.value.reason ||
      'This channel is not supported by this phBot session.'
    )
  if (activeChannel.value === 'private' && !privatePeer.value.trim())
    return 'Choose a contact or enter a recipient.'
  if (draftByteLength.value > 2048)
    return 'Messages are limited to 2,048 UTF-8 bytes.'
  return ''
})
const lastChatCommand = computed(() =>
  commandHistory.value.find((item) => item.name === 'chat.send'),
)
const visibleChannelCount = (channel: ChatChannel) => unread.value[channel] || 0
const isServerWideChannel = (channel: ChatChannel) =>
  channel === 'general' || channel === 'global'
const isServerWideReadChannel = (channel: ChatChannel) =>
  channel !== 'private' && channel !== 'guild' && channel !== 'union'
const chatServerFilter = (server: string) =>
  server === 'all' ? selectedCharacter.value?.server : server

watch(
  [selectableCharacters, () => route.query.character_id],
  ([items, requestedID]) => {
    const requested = typeof requestedID === 'string' ? requestedID : ''
    if (requested && items.some((item) => item.character_id === requested)) {
      selectedCharacterID.value = requested
      return
    }
    if (!items.some((item) => item.character_id === selectedCharacterID.value))
      selectedCharacterID.value = items[0]?.character_id || ''
  },
  { immediate: true },
)
watch(
  () => route.query.channel,
  (value) => {
    if (channels.some((channel) => channel.key === value))
      activeChannel.value = value as ChatChannel
    else if (value === 'unknown') activeChannel.value = 'unknown'
  },
  { immediate: true },
)
watch(selectedCharacterID, (id) => {
  clearCharacterCommandSubscriptions()
  if (!id) return
  setCharacterControls(id)
  setCharacterCommands(id, 'chat.send')
})
watch(
  [serverScope, selectedCharacterID, activeChannel, privatePeerKey],
  ([server, characterID, channel, peer]) => {
    olderMessages.value = []
    olderCursor.value = ''
    lastReadMessageID.value = ''
    if (!characterID) {
      clearChatFeed(feedID)
      return
    }
    setChatFeed(feedID, {
      server: chatServerFilter(server),
      // General/Global history stays server-wide; this ID scopes Private,
      // Guild, and Union unread state to the selected sender.
      character_id: characterID,
      channel,
      peer: channel === 'private' ? peer || undefined : undefined,
      limit: 50,
    })
  },
  { immediate: true },
)
watch(page, (next) => {
  hasOlder.value = !!next?.has_older
  olderCursor.value = next?.older_cursor || ''
  if (next?.messages?.length && atLatest.value) nextTick(scrollToLatest)
  void markCurrentRead()
})
watch([activeChannel, privatePeerKey, selectedCharacterID], () => {
  olderMessages.value = []
  atLatest.value = true
  showContactsMobile.value = activeChannel.value !== 'private'
})
watch(
  [selectedCharacter, activeChannel, privatePeerKey, atLatest],
  ([character, channel, peer, latest]) => {
    setActiveChatConversation(
      character && latest
        ? {
            server: character.server,
            characterID: character.character_id,
            channel,
            peer: channel === 'private' ? peer : '',
          }
        : null,
    )
  },
  { immediate: true },
)

function scrollToLatest() {
  if (!timeline.value) return
  timeline.value.scrollTop = timeline.value.scrollHeight
  atLatest.value = true
}

function updateScrollPosition() {
  const element = timeline.value
  if (!element) return
  atLatest.value =
    element.scrollHeight - element.scrollTop - element.clientHeight < 64
}

function chooseContact(contact: { peer_name: string }) {
  privatePeer.value = contact.peer_name
  recipientDraft.value = contact.peer_name
  showContactsMobile.value = false
}

function startPrivateConversation() {
  const next = recipientDraft.value.trim()
  if (!next) return
  privatePeer.value = next
  showContactsMobile.value = false
}

function insertEmoji(value: string) {
  messageDraft.value += value
  emojiOpen.value = false
}

function openUnknownLane() {
  mode.value = 'advanced'
  activeChannel.value = 'unknown'
}

async function loadOlder() {
  if (
    !hasOlder.value ||
    !olderCursor.value ||
    loadingOlder.value ||
    !selectedCharacter.value
  )
    return
  loadingOlder.value = true
  sendError.value = ''
  try {
    const result = await $fetch<ChatSnapshot['page']>('/api/chat/messages', {
      query: {
        server: chatServerFilter(serverScope.value),
        character_id: selectedCharacter.value.character_id,
        channel: activeChannel.value,
        peer: activeChannel.value === 'private' ? privatePeer.value : undefined,
        before: olderCursor.value,
        limit: 50,
      },
    })
    olderMessages.value = [...result.messages, ...olderMessages.value]
    hasOlder.value = result.has_older
    olderCursor.value = result.older_cursor || ''
  } catch {
    sendError.value = 'Older messages could not be loaded.'
  } finally {
    loadingOlder.value = false
  }
}

async function markCurrentRead() {
  const character = selectedCharacter.value
  if (
    !import.meta.client ||
    !character ||
    document.visibilityState !== 'visible' ||
    !atLatest.value ||
    (activeChannel.value === 'private' && !privatePeer.value.trim())
  )
    return
  const latestInbound = [...messages.value]
    .reverse()
    .find((item) => item.direction === 'inbound')
  const messageID = latestInbound?.message_id || ''
  const canClearServerWideUnread =
    isServerWideReadChannel(activeChannel.value) &&
    visibleChannelCount(activeChannel.value) > 0
  if (!messageID && !canClearServerWideUnread) return
  const requestKey =
    messageID ||
    `server:${character.server}:${activeChannel.value}:${visibleChannelCount(activeChannel.value)}`
  if (requestKey === lastReadMessageID.value) return
  lastReadMessageID.value = requestKey
  try {
    const readState = await $fetch<
      Pick<ChatSnapshot, 'contacts' | 'unread_by_channel'> & { saved: boolean }
    >('/api/chat/read', {
      method: 'POST',
      body: {
        server: character.server,
        // The store shares read cursors for server channels and keeps
        // Private/Guild/Union cursors scoped to this character.
        character_id: character.character_id,
        channel: activeChannel.value,
        peer: activeChannel.value === 'private' ? privatePeer.value : '',
        message_id: messageID,
      },
    })
    if (readState.saved) applyChatReadState(feedID, readState)
  } catch {
    lastReadMessageID.value = ''
  }
}

async function submitMessage(globalConfirmed = false) {
  const character = selectedCharacter.value
  if (!character?.session_id || !canSend.value) return
  if (activeChannel.value === 'global' && !globalConfirmed) {
    showGlobalConfirmation.value = true
    return
  }
  sending.value = true
  sendError.value = ''
  sendState.value = 'Submitting to the selected phBot session…'
  try {
    const args: Record<string, string> = {
      channel: activeChannel.value,
      text: messageDraft.value,
    }
    if (activeChannel.value === 'private')
      args.recipient = privatePeer.value.trim()
    const result = await $fetch<{ command_id: string; state: string }>(
      '/api/commands',
      {
        method: 'POST',
        body: {
          character_id: character.character_id,
          expected_session_id: character.session_id,
          name: 'chat.send',
          args,
          confirmation: activeChannel.value === 'global',
          idempotency_key: createIdempotencyKey(),
        },
      },
    )
    sendState.value = `Queued ${result.command_id}. Waiting for phBot API result.`
    messageDraft.value = ''
    showGlobalConfirmation.value = false
  } catch (error) {
    const failure = error as { data?: { message?: string; error?: string } }
    sendError.value =
      failure.data?.message ||
      failure.data?.error ||
      'Message could not be queued.'
    sendState.value = ''
  } finally {
    sending.value = false
  }
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') void markCurrentRead()
}

onMounted(() => {
  document.addEventListener('visibilitychange', onVisibilityChange)
  nextTick(scrollToLatest)
})
onBeforeUnmount(() => {
  document.removeEventListener('visibilitychange', onVisibilityChange)
  clearChatFeed(feedID)
  clearCharacterCommandSubscriptions()
  setActiveChatConversation(null)
})
</script>

<template>
  <div class="chat-page">
    <!-- eslint-disable vue/html-self-closing -->
    <PageHeader
      title="Chat"
      icon="i-lucide-messages-square"
      description="Read and send messages through the selected live phBot session."
    />

    <div
      v-if="liveStale || connectionState === 'stale'"
      class="status-banner warning"
      role="status"
    >
      Live connection is stale. Showing the last known chat history; sending is
      paused.
    </div>
    <div v-else-if="liveLoading" class="status-banner" role="status">
      Loading chat history…
    </div>

    <section class="panel chat-workspace" aria-label="Chat conversations">
      <div class="chat-toolbar">
        <label class="sender-select">
          <span>Send as</span>
          <select v-model="selectedCharacterID" aria-label="Select chat sender">
            <option value="">Select a character</option>
            <option
              v-for="character in selectableCharacters"
              :key="character.character_id"
              :value="character.character_id"
            >
              {{ character.name }} · {{ character.server
              }}{{ character.online ? '' : ' · offline' }}
            </option>
          </select>
        </label>
        <span
          v-if="selectedCharacter"
          class="sender-state"
          :class="selectedCharacter.online ? 'online' : 'offline'"
        >
          {{ selectedCharacter.online ? 'Online' : 'Offline' }} ·
          {{ selectedCharacter.server }}
        </span>
      </div>

      <nav class="chat-tabs" aria-label="Chat channels">
        <button
          v-for="channel in visibleChannels"
          :key="channel.key"
          type="button"
          class="chat-tab"
          :class="[
            `channel-${channel.key}`,
            { active: activeChannel === channel.key },
          ]"
          :aria-pressed="activeChannel === channel.key"
          @click="activeChannel = channel.key"
        >
          {{ channel.label }}
          <span
            v-if="channel.key !== 'global' && visibleChannelCount(channel.key)"
            class="unread-badge"
            >{{ visibleChannelCount(channel.key) }}</span
          >
        </button>
      </nav>
      <button
        v-if="!advancedMode && unread.unknown"
        class="unknown-shortcut"
        type="button"
        @click="openUnknownLane"
      >
        {{ unread.unknown }} unclassified messages · inspect in Advanced mode
      </button>

      <div
        class="chat-body"
        :class="{ 'private-view': activeChannel === 'private' }"
      >
        <aside
          v-if="activeChannel === 'private'"
          class="contact-panel"
          :class="{ 'mobile-hidden': !showContactsMobile }"
          aria-label="Private contacts"
        >
          <div class="contact-heading">
            <strong>Contacts</strong><span>{{ contacts.length }}</span>
          </div>
          <div class="new-chat-box">
            <label for="new-chat-recipient">New chat</label>
            <div class="new-chat-row">
              <input
                id="new-chat-recipient"
                v-model="recipientDraft"
                maxlength="64"
                placeholder="Character name"
                @keydown.enter.prevent="startPrivateConversation"
              />
              <button
                type="button"
                class="compact-button"
                :disabled="!recipientDraft.trim()"
                @click="startPrivateConversation"
              >
                Open
              </button>
            </div>
          </div>
          <p v-if="!contacts.length" class="contact-empty">
            No private contacts recorded for this sender.
          </p>
          <button
            v-for="contact in contacts"
            :key="`${contact.server}:${contact.peer_key}`"
            type="button"
            class="contact-row"
            :class="{ selected: privatePeerKey === contact.peer_key }"
            @click="chooseContact(contact)"
          >
            <span class="contact-avatar">{{
              contact.peer_name.slice(0, 1).toUpperCase()
            }}</span>
            <span class="contact-copy"
              ><strong>{{ contact.peer_name }}</strong
              ><small>{{ contact.last_message }}</small></span
            >
            <span v-if="contact.unread" class="unread-badge">{{
              contact.unread
            }}</span>
          </button>
        </aside>

        <section class="conversation" aria-label="Conversation">
          <header class="conversation-header">
            <button
              v-if="activeChannel === 'private'"
              class="back-to-contacts"
              type="button"
              @click="showContactsMobile = true"
            >
              ‹ Contacts
            </button>
            <div>
              <strong>{{
                activeChannel === 'private'
                  ? privatePeer || 'Choose a contact'
                  : `${activeChannel[0]?.toUpperCase()}${activeChannel.slice(1)} chat`
              }}</strong>
              <small v-if="selectedCharacter"
                >{{ selectedCharacter.name }} ·
                {{ selectedCharacter.server }}</small
              >
            </div>
            <button
              v-if="!atLatest"
              class="compact-button jump-latest"
              type="button"
              @click="scrollToLatest"
            >
              Jump to latest ↓
            </button>
          </header>

          <div
            ref="timeline"
            class="message-timeline"
            @scroll="updateScrollPosition"
          >
            <button
              v-if="hasOlder"
              class="load-older"
              type="button"
              :disabled="loadingOlder"
              @click="loadOlder"
            >
              {{ loadingOlder ? 'Loading…' : 'Load older messages' }}
            </button>
            <div v-if="activeChannel === 'unknown'" class="unknown-lane-note">
              Numeric phBot chat types are retained here until their meaning is
              verified on the installed runtime.
            </div>
            <div
              v-if="activeChannel === 'private' && !privatePeer"
              class="conversation-empty"
            >
              Choose a contact or enter a character name to open a private
              conversation.
            </div>
            <div v-else-if="!selectedCharacterID" class="conversation-empty">
              Select a sender to view that character’s chat history.
            </div>
            <div
              v-else-if="!chatSnapshotCurrent"
              class="conversation-empty"
              role="status"
            >
              Loading conversation…
            </div>
            <div
              v-else-if="!messages.length && snapshot"
              class="conversation-empty"
            >
              No {{ activeChannel }} messages recorded
              {{
                isServerWideChannel(activeChannel)
                  ? 'on this server'
                  : 'for this sender'
              }}
              yet.
            </div>
            <article
              v-for="item in messages"
              :key="item.message_id"
              class="message-row"
            >
              <div class="message-content">
                <div class="message-meta">
                  <strong>{{
                    item.direction === 'outbound'
                      ? 'You'
                      : item.sender || 'Unknown'
                  }}</strong
                  ><span
                    v-if="advancedMode && item.raw_type"
                    class="message-raw-type"
                    >type {{ item.raw_type }}</span
                  ><time :datetime="item.occurred_at">{{
                    new Date(item.occurred_at).toLocaleString()
                  }}</time
                  ><span
                    v-if="item.direction === 'outbound'"
                    class="message-state"
                    >{{ item.state }}</span
                  >
                </div>
                <p>{{ item.message }}</p>
              </div>
            </article>
          </div>

          <div v-if="activeChannel === 'unknown'" class="read-only-note">
            This lane is read only. The raw chat type is preserved in history.
          </div>
          <form v-else class="composer" @submit.prevent="submitMessage()">
            <div class="composer-tools">
              <div class="emoji-control">
                <button
                  type="button"
                  class="tool-button"
                  aria-label="Insert emoji"
                  :aria-expanded="emojiOpen"
                  @click="emojiOpen = !emojiOpen"
                >
                  ☺
                </button>
                <div
                  v-if="emojiOpen"
                  class="emoji-menu"
                  role="group"
                  aria-label="Emoji"
                >
                  <button
                    v-for="emoji in ['🙂', '😂', '❤️', '👍', '🎉']"
                    :key="emoji"
                    type="button"
                    @click="insertEmoji(emoji)"
                  >
                    {{ emoji }}
                  </button>
                </div>
              </div>
              <span v-if="activeChannel === 'global'" class="global-hint"
                >Global sends need confirmation.</span
              >
              <span class="byte-count">{{ draftByteLength }} / 2048 bytes</span>
            </div>
            <div class="composer-entry">
              <textarea
                v-model="messageDraft"
                aria-label="Chat message"
                rows="2"
                maxlength="2048"
                placeholder="Write a message…"
                :disabled="activeChannel === 'private' && !privatePeer"
                @keydown.ctrl.enter.prevent="submitMessage()"
                @keydown.meta.enter.prevent="submitMessage()"
              />
              <button
                class="send-button"
                type="submit"
                :disabled="!canSend"
                :title="sendDisabledReason"
              >
                {{ sending ? 'Sending…' : 'Send' }}
              </button>
            </div>
            <p v-if="sendDisabledReason && !sending" class="composer-hint">
              {{ sendDisabledReason }}
            </p>
            <p v-if="sendState" class="composer-result" role="status">
              {{ sendState
              }}<template v-if="lastChatCommand">
                · Latest result: {{ lastChatCommand.state
                }}{{
                  lastChatCommand.message ? ` · ${lastChatCommand.message}` : ''
                }}</template
              >
            </p>
            <p v-if="sendError" class="form-error" role="alert">
              {{ sendError }}
            </p>
          </form>
        </section>
      </div>
    </section>

    <div
      v-if="showGlobalConfirmation"
      class="confirmation-backdrop"
      @click.self="showGlobalConfirmation = false"
    >
      <section
        class="confirmation-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="global-confirm-title"
      >
        <h2 id="global-confirm-title">Confirm global message</h2>
        <p>
          Global chat may consume an in-game item or another resource. phBot can
          confirm whether it accepted this message; that does not guarantee
          delivery.
        </p>
        <blockquote>{{ messageDraft }}</blockquote>
        <div class="dialog-actions">
          <button
            class="compact-button"
            type="button"
            @click="showGlobalConfirmation = false"
          >
            Cancel</button
          ><button
            class="send-button"
            type="button"
            :disabled="sending"
            @click="submitMessage(true)"
          >
            Confirm and send
          </button>
        </div>
      </section>
    </div>
  </div>
</template>

<style scoped>
.chat-page {
  min-width: 0;
  display: grid;
  gap: 14px;
}
.chat-workspace {
  min-width: 0;
  padding: 12px;
}
.chat-toolbar {
  display: flex;
  align-items: end;
  gap: 12px;
  min-height: 52px;
}
.sender-select {
  display: grid;
  width: min(340px, 100%);
  gap: 4px;
  color: var(--ph-muted);
  font-size: 12px;
}
.sender-select select,
.new-chat-row input {
  min-width: 0;
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: #0a111b;
  color: var(--ph-text);
  padding: 7px 9px;
}
.sender-state {
  font-size: 12px;
  padding-bottom: 8px;
}
.sender-state.online {
  color: var(--ph-green);
}
.sender-state.offline {
  color: var(--ph-amber);
}
.chat-tabs {
  display: flex;
  gap: 5px;
  overflow-x: auto;
  border-bottom: 1px solid var(--ph-border);
  padding: 9px 0;
}
.chat-tab {
  min-height: 32px;
  display: inline-flex;
  align-items: center;
  gap: 7px;
  flex: 0 0 auto;
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  background: #0a111a;
  color: #aebbd0;
  padding: 5px 11px;
  cursor: pointer;
}
.chat-tab.active {
  color: var(--ph-primary);
  border-color: #64708a;
  background: rgba(52, 103, 163, 0.22);
}
.channel-general {
  border-bottom-color: #8caaa2;
}
.channel-private {
  border-bottom-color: #a58ab9;
}
.channel-party {
  border-bottom-color: #69a1c7;
}
.channel-guild {
  border-bottom-color: #87ad67;
}
.channel-union {
  border-bottom-color: #b28b56;
}
.channel-global {
  border-bottom-color: #b96565;
}
.unread-badge {
  display: inline-grid;
  min-width: 18px;
  height: 18px;
  place-items: center;
  border-radius: 9px;
  background: #375b80;
  color: #fff;
  font-size: 10px;
  padding: 0 5px;
}
.unknown-shortcut {
  justify-self: start;
  border: 0;
  background: transparent;
  color: #e5bf7f;
  padding: 5px 2px;
  font-size: 11px;
  cursor: pointer;
}
.chat-body {
  display: grid;
  min-width: 0;
  min-height: min(680px, calc(100vh - 280px));
  grid-template-columns: minmax(0, 1fr);
}
.chat-body.private-view {
  grid-template-columns: 248px minmax(0, 1fr);
}
.contact-panel {
  min-width: 0;
  border-right: 1px solid var(--ph-border);
  padding: 10px 9px 10px 0;
  overflow-y: auto;
}
.contact-heading {
  display: flex;
  justify-content: space-between;
  color: var(--ph-primary);
  padding: 0 7px 9px;
}
.contact-heading span,
.contact-copy small {
  color: var(--ph-muted);
  font-size: 11px;
}
.new-chat-box {
  border: 1px solid var(--ph-border-soft);
  border-radius: 4px;
  background: rgba(7, 12, 19, 0.55);
  padding: 9px;
  margin-bottom: 9px;
}
.new-chat-box label {
  display: block;
  font-size: 11px;
  color: var(--ph-muted);
  margin-bottom: 6px;
}
.new-chat-row {
  display: flex;
  gap: 5px;
}
.new-chat-row input {
  width: 100%;
}
.compact-button {
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: #151e2a;
  color: var(--ph-text);
  padding: 6px 9px;
  cursor: pointer;
}
.compact-button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.contact-row {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 8px;
  text-align: left;
  border: 1px solid transparent;
  border-radius: 4px;
  background: transparent;
  color: var(--ph-text);
  padding: 8px 6px;
  cursor: pointer;
}
.contact-row:hover,
.contact-row.selected {
  background: var(--ph-active);
  border-color: var(--ph-border);
}
.contact-avatar {
  display: grid;
  flex: 0 0 30px;
  width: 30px;
  height: 30px;
  place-items: center;
  border: 1px solid #3c526e;
  border-radius: 50%;
  background: #152234;
  color: #bdd8f5;
  font-weight: 600;
}
.contact-copy {
  min-width: 0;
  flex: 1;
  display: grid;
  gap: 3px;
}
.contact-copy strong,
.contact-copy small {
  overflow: hidden;
  white-space: nowrap;
  text-overflow: ellipsis;
}
.contact-empty {
  padding: 5px 7px;
  color: var(--ph-muted);
  font-size: 12px;
}
.conversation {
  min-width: 0;
  min-height: 0;
  display: flex;
  flex-direction: column;
  padding-left: 12px;
}
.conversation-header {
  display: flex;
  min-height: 54px;
  align-items: center;
  gap: 10px;
  border-bottom: 1px solid var(--ph-border-soft);
}
.conversation-header > div {
  display: grid;
  gap: 2px;
}
.conversation-header strong {
  color: var(--ph-primary);
}
.conversation-header small {
  color: var(--ph-muted);
  font-size: 11px;
}
.jump-latest {
  margin-left: auto;
}
.back-to-contacts {
  display: none;
  border: 0;
  background: none;
  color: var(--ph-blue);
}
.message-timeline {
  min-height: 200px;
  flex: 1;
  overflow: auto;
  padding: 12px 5px;
  scroll-behavior: smooth;
}
.message-row {
  display: block;
  border-bottom: 1px solid rgba(55, 70, 90, 0.28);
  padding: 8px 4px;
}
.message-content {
  min-width: 0;
  max-width: 100%;
}
.message-meta {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ph-muted);
  font-size: 11px;
}
.message-meta strong {
  color: #d7e2f4;
}
.message-meta time {
  font-variant-numeric: tabular-nums;
}
.message-raw-type {
  color: #d9b978;
}
.message-content p {
  margin: 4px 0 0;
  white-space: pre-wrap;
  overflow-wrap: anywhere;
  line-height: 1.45;
}
.message-state {
  color: var(--ph-amber);
}
.conversation-empty,
.unknown-lane-note,
.read-only-note {
  margin: 12px 4px;
  border: 1px dashed #34445a;
  border-radius: 4px;
  padding: 13px;
  color: var(--ph-muted);
  font-size: 12px;
}
.unknown-lane-note {
  color: #f0cc8c;
  border-color: #67583b;
}
.load-older {
  display: block;
  margin: 0 auto 10px;
  border: 0;
  background: transparent;
  color: var(--ph-blue);
  cursor: pointer;
}
.composer {
  border-top: 1px solid var(--ph-border);
  padding-top: 8px;
}
.composer-tools {
  position: relative;
  display: flex;
  align-items: center;
  gap: 8px;
  padding-bottom: 7px;
}
.tool-button {
  width: 28px;
  height: 26px;
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: #111a27;
  color: var(--ph-primary);
  cursor: pointer;
}
.emoji-menu {
  position: absolute;
  z-index: 4;
  bottom: 34px;
  left: 0;
  display: flex;
  gap: 3px;
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: #101925;
  padding: 5px;
  box-shadow: 0 8px 24px #0008;
}
.emoji-menu button {
  border: 0;
  background: transparent;
  font-size: 18px;
  cursor: pointer;
}
.byte-count,
.global-hint {
  color: var(--ph-muted);
  font-size: 10px;
}
.byte-count {
  margin-left: auto;
}
.global-hint {
  color: var(--ph-amber);
}
.composer-entry {
  display: flex;
  align-items: stretch;
  gap: 8px;
}
.composer-entry textarea {
  min-width: 0;
  flex: 1;
  resize: vertical;
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: #080f18;
  color: var(--ph-text);
  padding: 9px;
  line-height: 1.4;
}
.send-button {
  align-self: stretch;
  border: 1px solid #52749b;
  border-radius: 4px;
  background: #1b3653;
  color: #e8f3ff;
  padding: 0 17px;
  cursor: pointer;
}
.send-button:disabled {
  opacity: 0.45;
  cursor: not-allowed;
}
.composer-hint,
.composer-result {
  margin: 6px 0 0;
  color: var(--ph-muted);
  font-size: 11px;
}
.form-error {
  margin: 6px 0 0;
  color: var(--ph-red);
  font-size: 12px;
}
.confirmation-backdrop {
  position: fixed;
  z-index: 80;
  inset: 0;
  display: grid;
  place-items: center;
  padding: 16px;
  background: #02060bc9;
}
.confirmation-dialog {
  width: min(460px, 100%);
  border: 1px solid var(--ph-border);
  border-radius: 6px;
  background: #101824;
  padding: 18px;
  box-shadow: 0 20px 70px #000b;
}
.confirmation-dialog h2 {
  margin: 0 0 9px;
  color: var(--ph-primary);
  font-size: 18px;
}
.confirmation-dialog p {
  color: var(--ph-muted);
  line-height: 1.45;
}
.confirmation-dialog blockquote {
  margin: 12px 0;
  border-left: 2px solid var(--ph-amber);
  background: #0a111a;
  padding: 10px;
  overflow-wrap: anywhere;
}
.dialog-actions {
  display: flex;
  justify-content: end;
  gap: 8px;
}
.dialog-actions .send-button {
  min-height: 34px;
}
@media (max-width: 700px) {
  .chat-workspace {
    padding: 8px;
  }
  .chat-toolbar {
    align-items: stretch;
    flex-direction: column;
    gap: 5px;
  }
  .sender-select {
    width: 100%;
  }
  .sender-state {
    padding-bottom: 2px;
  }
  .chat-tabs {
    margin: 0 -2px;
  }
  .chat-body,
  .chat-body.private-view {
    min-height: calc(100dvh - 300px);
    grid-template-columns: minmax(0, 1fr);
  }
  .contact-panel {
    border-right: 0;
    padding: 8px 0;
  }
  .contact-panel.mobile-hidden {
    display: none;
  }
  .conversation {
    padding-left: 0;
  }
  .private-view .conversation {
    min-height: calc(100dvh - 300px);
  }
  .private-view .back-to-contacts {
    display: block;
  }
  .message-meta {
    flex-wrap: wrap;
    gap: 4px 7px;
  }
  .message-meta time {
    font-size: 10px;
  }
  .composer-entry textarea {
    min-height: 48px;
  }
  .send-button {
    padding: 0 12px;
  }
}
</style>
