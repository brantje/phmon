<script setup lang="ts">
import type { NotificationEvent } from '~~/shared/types/live'
import { createChatNotificationDeduper } from '~/utils/chatNotificationDedup'
import {
  notificationPresentation,
  type NotificationPresentation,
} from '~/utils/notificationPresentation'

type Card = {
  event: NotificationEvent
  presentation: NotificationPresentation
  expiresAt: number
}

const { preferences, ready, load } = useNotificationPreferences()
const { serverScope } = useServerScope()
const { setNotificationFeed, clearNotificationFeed, onNotification } =
  useLiveData()
const active = ref<Card[]>([])
const now = ref(Date.now())
const waiting: Card[] = []
const seen = new Set<string>()
const shouldShowChatNotification = createChatNotificationDeduper()
const displayMilliseconds = 8000
let removeListener: (() => void) | undefined
let progressTimer: ReturnType<typeof setInterval> | undefined
// The map page uses its own layout that wraps the default one, so a navigation
// between layouts mounts the new host before the old one unmounts. The shared
// notification subscription belongs to the last host standing.
const liveHosts = useState('notification-hosts', () => 0)
if (import.meta.client) liveHosts.value += 1

function dismiss(card: Card) {
  active.value = active.value.filter((item) => item !== card)
  pump()
}

function stopProgressTimer() {
  if (progressTimer) clearInterval(progressTimer)
  progressTimer = undefined
}

function tick() {
  now.value = Date.now()
  active.value = active.value.filter((card) => card.expiresAt > now.value)
  pump()
}

function pump() {
  if (document.visibilityState !== 'visible') return
  now.value = Date.now()
  while (active.value.length < 3 && waiting.length) {
    const card = waiting.shift()!
    card.expiresAt = now.value + displayMilliseconds
    // Newest first: the stack is anchored to the bottom, so a new card grows it
    // upward instead of pushing every visible card (and its countdown) up a slot.
    active.value = [card, ...active.value]
  }
  if (active.value.length && !progressTimer)
    progressTimer = setInterval(tick, 100)
  else if (!active.value.length) stopProgressTimer()
}

function onVisibilityChange() {
  if (document.visibilityState === 'visible') tick()
  else stopProgressTimer()
}

function playChatSound() {
  const audio = new Audio('/sounds/chat-message.wav')
  audio.volume = 0.35
  void audio.play().catch(() => {})
}

function showNative(card: Card) {
  if (
    document.visibilityState === 'visible' ||
    !preferences.value.browser_kinds.includes(card.event.kind) ||
    !('Notification' in window) ||
    Notification.permission !== 'granted'
  )
    return
  try {
    const notice = new Notification(card.presentation.title, {
      body: [card.presentation.body, card.event.character, card.event.server]
        .filter(Boolean)
        .join(' · ')
        .slice(0, 240),
      tag: `phmon-event-${card.event.event_id}`,
    })
    notice.onclick = () => {
      window.focus()
      void navigateTo(card.presentation.to)
      notice.close()
    }
    const closeTimer = setTimeout(() => notice.close(), displayMilliseconds)
    notice.onclose = () => clearTimeout(closeTimer)
  } catch {
    // Browser notification availability can change after permission was read.
  }
}

function receive(event: NotificationEvent) {
  const popupEnabled = preferences.value.popup_kinds.includes(event.kind)
  const browserEnabled = preferences.value.browser_kinds.includes(event.kind)
  const chatSoundEnabled =
    event.kind === 'chat.message_received' && preferences.value.chat_sound
  if (
    !ready.value ||
    !(popupEnabled || browserEnabled || chatSoundEnabled) ||
    (serverScope.value !== 'all' &&
      serverScope.value.toLowerCase() !== event.server.toLowerCase()) ||
    (event.kind === 'chat.message_received' &&
      !preferences.value.chat_channels.includes(
        String(event.payload.channel || 'unknown'),
      )) ||
    seen.has(event.event_id)
  )
    return
  seen.add(event.event_id)
  if (seen.size > 10000) seen.delete(seen.values().next().value!)
  if (!shouldShowChatNotification(event)) return
  const card: Card = {
    event,
    presentation: notificationPresentation(event),
    expiresAt: 0,
  }
  if (popupEnabled) waiting.push(card)
  showNative(card)
  if (chatSoundEnabled && document.visibilityState !== 'visible')
    playChatSound()
  if (popupEnabled) pump()
}

watch(
  [
    ready,
    () => preferences.value.popup_kinds.join('\u0000'),
    () => preferences.value.browser_kinds.join('\u0000'),
    () => preferences.value.chat_sound,
    serverScope,
  ],
  ([loaded, _popups, _browser, _sound, server]) => {
    const kinds = new Set([
      ...preferences.value.popup_kinds,
      ...preferences.value.browser_kinds,
    ])
    if (preferences.value.chat_sound) kinds.add('chat.message_received')
    if (loaded && kinds.size) {
      setNotificationFeed([...kinds], server === 'all' ? undefined : server)
    } else clearNotificationFeed()
  },
  { immediate: true },
)

onMounted(() => {
  removeListener = onNotification(receive)
  document.addEventListener('visibilitychange', onVisibilityChange)
  void load()
})
onBeforeUnmount(() => {
  removeListener?.()
  liveHosts.value -= 1
  if (liveHosts.value <= 0) clearNotificationFeed()
  document.removeEventListener('visibilitychange', onVisibilityChange)
  stopProgressTimer()
  active.value = []
  waiting.length = 0
})
</script>

<template>
  <TransitionGroup
    tag="div"
    name="event-popup"
    class="event-popup-stack"
    aria-live="polite"
    aria-label="Event notifications"
  >
    <article
      v-for="card in active"
      :key="card.event.event_id"
      class="event-popup panel"
      :class="`tone-${card.presentation.tone}`"
    >
      <NuxtLink class="event-popup-link" :to="card.presentation.to">
        <span class="event-popup-heading">
          <strong>{{ card.presentation.title }}</strong>
          <span class="event-popup-time">
            <FormattedDate :value="card.event.occurred_at" variant="time" />
            <span aria-hidden="true">·</span>
            <span aria-hidden="true"
              >{{
                Math.max(0, Math.ceil((card.expiresAt - now) / 1000))
              }}s</span
            >
          </span>
        </span>
        <span v-if="card.presentation.body" class="event-popup-body">{{
          card.presentation.body
        }}</span>
        <small
          >{{
            card.event.character ||
            (card.event.kind.startsWith('session.')
              ? 'Character not selected'
              : 'Observer unavailable')
          }}<template v-if="card.event.server">
            · {{ card.event.server }}</template
          ></small
        >
      </NuxtLink>
      <button
        type="button"
        class="event-popup-dismiss"
        aria-label="Dismiss event notification"
        @click="dismiss(card)"
      >
        <UIcon name="i-lucide-x" aria-hidden="true" />
      </button>
      <span
        class="event-popup-progress"
        :style="{
          transform: `scaleX(${Math.max(0, (card.expiresAt - now) / displayMilliseconds)})`,
        }"
        aria-hidden="true"
      />
    </article>
  </TransitionGroup>
</template>

<style scoped>
.event-popup-stack {
  position: fixed;
  z-index: 4000;
  right: max(16px, env(safe-area-inset-right));
  bottom: max(16px, env(safe-area-inset-bottom));
  width: min(370px, calc(100vw - 32px));
  display: grid;
  gap: 8px;
  pointer-events: none;
}
.event-popup {
  --progress-color: #718ba9;
  position: relative;
  display: flex;
  border-left: 3px solid #718ba9;
  border-radius: 5px;
  background: #111b29f2;
  box-shadow: 0 8px 28px #0009;
  pointer-events: auto;
}
.tone-urgent {
  border-left-color: #d97775;
  --progress-color: #d97775;
}
.tone-highlight {
  border-left-color: #d9bd67;
  --progress-color: #d9bd67;
}
.tone-activity {
  border-left-color: #76a5c5;
  --progress-color: #76a5c5;
}
.event-popup-link {
  display: grid;
  gap: 4px;
  width: 100%;
  padding: 11px 38px 11px 13px;
  color: var(--ph-text);
  text-decoration: none;
  overflow-wrap: anywhere;
}
.event-popup-link:hover strong {
  text-decoration: underline;
}
.event-popup-heading {
  display: flex;
  align-items: baseline;
  justify-content: space-between;
  gap: 8px;
}
.event-popup-time {
  display: inline-flex;
  gap: 4px;
  flex: none;
  color: var(--ph-muted);
  font-size: 11px;
  font-variant-numeric: tabular-nums;
  white-space: nowrap;
}
.event-popup-link:focus-visible,
.event-popup-dismiss:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}
.event-popup-link strong {
  color: var(--ph-primary);
  font-size: 14px;
}
.event-popup-body {
  font-size: 13px;
  line-height: 1.35;
}
.event-popup-link small {
  color: var(--ph-muted);
  font-size: 11px;
}
.event-popup-dismiss {
  position: absolute;
  top: 7px;
  right: 7px;
  display: grid;
  place-items: center;
  width: 28px;
  height: 28px;
  border: 0;
  border-radius: 4px;
  color: var(--ph-muted);
  background: transparent;
  cursor: pointer;
}
.event-popup-dismiss:hover {
  color: var(--ph-text);
  background: #ffffff18;
}
.event-popup-progress {
  position: absolute;
  right: 0;
  bottom: 0;
  left: 0;
  height: 3px;
  border-radius: 0 0 4px 0;
  background: var(--progress-color);
  transform-origin: left;
  transition: transform 100ms linear;
  pointer-events: none;
}
/* Cards leave instantly; the ones left behind slide down into the gap. */
.event-popup-leave-active {
  display: none;
}
.event-popup-move,
.event-popup-enter-active {
  transition:
    transform 160ms ease,
    opacity 160ms ease;
}
.event-popup-enter-from {
  opacity: 0;
  transform: translateY(-8px);
}
@media (prefers-reduced-motion: reduce) {
  .event-popup-move,
  .event-popup-enter-active {
    transition: none;
  }
}
@media (max-width: 640px) {
  .event-popup-stack {
    right: 12px;
    bottom: max(12px, env(safe-area-inset-bottom));
  }
}
</style>
