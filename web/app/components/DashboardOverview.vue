<script setup lang="ts">
import type { ActivityEvent } from '~~/shared/types/live'
import { groupRecentEvents } from '~/utils/groupRecentEvents'

const {
  onlineCharacterCount,
  offlineCharacterCount,
  aliveCharacterCount,
  deadCharacterCount,
  unknownDeathCharacterCount,
  displayCharacterCount,
  displayDeathCount,
  observedGold,
} = useFleetSummary()
const { eventFeeds, connectionState, liveStale, setEventFeed, clearEventFeed } =
  useLiveData()
const { serverScope } = useServerScope()
const lastDeaths = computed(
  () => eventFeeds.value['dashboard-deaths']?.events || [],
)
const recentEvents = computed(() =>
  groupRecentEvents(eventFeeds.value['dashboard-events']?.events || [], 5),
)
const recentRareDrops = computed(
  () => eventFeeds.value['dashboard-rare-drops']?.events || [],
)
const recentChatMessages = computed(
  () => eventFeeds.value['dashboard-chat']?.events || [],
)

function textField(value: unknown) {
  return typeof value === 'string' ? value : ''
}

function eventHeadline(event: ActivityEvent) {
  const payload = event.payload || {}
  switch (event.kind) {
    case 'character.died':
      return `${event.character || 'Character'} died`
    case 'character.level_up':
      return `${event.character || 'Character'} reached level ${String(payload.level ?? '?')}`
    case 'drop.rare':
    case 'drop.item': {
      const details = payload.item as Record<string, unknown> | undefined
      const name =
        textField(event.item_metadata?.name) ||
        textField(details?.name) ||
        textField(payload.item_name)
      const model = payload.model ?? event.item_model
      return `${event.kind === 'drop.rare' ? 'Rare drop' : 'Item drop'}${name ? ` · ${name}` : model != null ? ` · model ${String(model)}` : ''}`
    }
    case 'world.unique_spawned':
      return `${textField(payload.value) || 'Unique'} spawned`
    case 'chat.message_received':
      return `${textField(payload.sender) || event.character || 'Chat'}: ${textField(payload.message)}`
    default:
      return event.kind.replaceAll('.', ' ')
  }
}

function eventObservers(item: (typeof recentEvents.value)[number]) {
  if (item.observers.length > 1) {
    return `Observed by ${item.observers.map((observer) => observer.character).join(', ')} · ${item.event.server}`
  }
  if (item.event.character) {
    return item.event.server
      ? `${item.event.character} · ${item.event.server}`
      : item.event.character
  }
  return item.event.server || 'Agent event'
}

function rareItemFilter(item: (typeof recentRareDrops.value)[number]) {
  const snapshot = item.payload.item as Record<string, unknown> | undefined
  return (
    textField(item.item_code) ||
    textField(snapshot?.servername) ||
    String(item.item_model ?? item.payload.model ?? '')
  )
}

function watchDashboardEvents(server: string) {
  const scope = { server: server === 'all' ? undefined : server }
  setEventFeed('dashboard-deaths', {
    ...scope,
    limit: 5,
    kind: 'character.died',
  })
  setEventFeed('dashboard-events', { ...scope, limit: 50 })
  setEventFeed('dashboard-rare-drops', {
    ...scope,
    limit: 5,
    kind: 'drop.rare',
  })
  setEventFeed('dashboard-chat', {
    ...scope,
    limit: 3,
    kind: 'chat.message_received',
  })
}

watch(serverScope, watchDashboardEvents, { immediate: true })
onBeforeUnmount(() => {
  clearEventFeed('dashboard-deaths')
  clearEventFeed('dashboard-events')
  clearEventFeed('dashboard-rare-drops')
  clearEventFeed('dashboard-chat')
})
</script>

<template>
  <section class="dashboard-grid" aria-label="Dashboard overview">
    <article class="panel dashboard-characters">
      <div class="panel-header compact">
        <div>
          <h2>Characters</h2>
          <p>Current fleet presence and observed gold</p>
        </div>
        <NuxtLink class="panel-link" to="/stats#characters"
          >Show character stats <UIcon name="i-lucide-arrow-up-right"
        /></NuxtLink>
      </div>
      <div class="dashboard-counts">
        <div class="dashboard-count online-count">
          <span>Online</span
          ><strong>{{ displayCharacterCount(onlineCharacterCount) }}</strong>
        </div>
        <div class="dashboard-count">
          <span>Offline</span
          ><strong>{{ displayCharacterCount(offlineCharacterCount) }}</strong>
        </div>
        <div class="dashboard-count alive-count">
          <span>Alive</span
          ><strong>{{ displayDeathCount(aliveCharacterCount) }}</strong>
        </div>
        <div class="dashboard-count dead-count">
          <span>Dead</span
          ><strong>{{ displayDeathCount(deadCharacterCount) }}</strong>
        </div>
      </div>
      <p v-if="unknownDeathCharacterCount" class="dashboard-unknown-count">
        {{ unknownDeathCharacterCount }} online character status{{
          unknownDeathCharacterCount === 1 ? '' : 'es'
        }}
        unknown
      </p>
      <div class="dashboard-gold">
        <UIcon name="i-lucide-coins" /><span>Total Gold</span
        ><strong>{{ observedGold }}</strong>
      </div>
    </article>

    <article class="panel dashboard-deaths">
      <div class="panel-header compact">
        <div>
          <h2>Last Deaths</h2>
          <p>Recent character deaths</p>
        </div>
        <NuxtLink class="panel-link" to="/events?kind=character.died">
          Show all deaths <UIcon name="i-lucide-arrow-up-right" />
        </NuxtLink>
      </div>
      <div v-if="liveStale" class="event-stale-note" role="status">
        Showing the last received events as stale.
      </div>
      <ul v-if="lastDeaths.length" class="dashboard-event-list">
        <li v-for="item in lastDeaths" :key="item.event_id">
          <NuxtLink :to="`/characters/${item.character_id}`">
            <CharacterPortrait
              :name="item.character"
              :portrait-url="item.portrait_url"
              size="small"
            />
            <span class="dashboard-event-identity">
              <strong>{{ item.character }}</strong>
              <span>Cause unknown · {{ item.server }}</span>
            </span>
          </NuxtLink>
          <time :datetime="item.occurred_at">{{
            formatTimestamp(item.occurred_at)
          }}</time>
        </li>
      </ul>
      <div v-else class="dashboard-event-empty">
        <UIcon name="i-lucide-skull" />
        <span>{{
          connectionState === 'current'
            ? 'No deaths recorded in this server scope.'
            : 'Waiting for event history…'
        }}</span>
      </div>
    </article>

    <article class="panel dashboard-later dashboard-server">
      <div class="panel-header compact">
        <div>
          <h2>Server Information</h2>
          <p>Operator managed</p>
        </div>
        <span class="later-badge">LATER</span>
      </div>
      <div class="later-content">
        <UIcon name="i-lucide-server" /><strong>LATER</strong
        ><span>Server artwork and metadata arrive in a later slice.</span>
      </div>
    </article>

    <article class="panel dashboard-events">
      <div class="panel-header compact">
        <div>
          <h2>Recent Events</h2>
          <p>Latest recorded activity across your characters</p>
        </div>
        <NuxtLink class="panel-link" to="/events">
          Show recent events <UIcon name="i-lucide-arrow-up-right" />
        </NuxtLink>
      </div>
      <ul v-if="recentEvents.length" class="dashboard-recent-list">
        <li v-for="item in recentEvents" :key="item.event.event_id">
          <UIcon name="i-lucide-activity" />
          <NuxtLink
            v-if="item.event.character_id"
            :to="`/characters/${item.event.character_id}`"
          >
            <CharacterPortrait
              :name="item.event.character"
              :portrait-url="item.event.portrait_url"
              size="small"
            />
            <span class="dashboard-event-identity">
              <strong>{{ eventHeadline(item.event) }}</strong>
              <span>{{ eventObservers(item) }}</span>
            </span>
          </NuxtLink>
          <div v-else>
            <strong>{{ eventHeadline(item.event) }}</strong>
            <span>{{ eventObservers(item) }}</span>
          </div>
          <time :datetime="item.event.occurred_at">{{
            formatTimestamp(item.event.occurred_at)
          }}</time>
        </li>
      </ul>
      <div v-else class="dashboard-event-empty">
        <UIcon name="i-lucide-clock-3" />
        <span>{{
          connectionState === 'current'
            ? 'No recent events.'
            : 'Waiting for event history…'
        }}</span>
      </div>
    </article>

    <div class="dashboard-stack">
      <article class="panel dashboard-later">
        <div class="panel-header compact">
          <div><h2>Rare Drops</h2></div>
          <NuxtLink class="panel-link" to="/events?kind=drop.rare"
            >View all</NuxtLink
          >
        </div>
        <NuxtLink
          v-if="recentRareDrops[0]"
          class="later-content compact-later dashboard-rare-link"
          :to="{
            path: '/events',
            query: {
              kind: 'drop.rare',
              item: rareItemFilter(recentRareDrops[0]) || undefined,
            },
          }"
        >
          <UIcon name="i-lucide-gem" /><strong class="rare-drop-item">{{
            eventHeadline(recentRareDrops[0])
          }}</strong>
          <span>{{
            recentRareDrops[0].character || recentRareDrops[0].server
          }}</span>
        </NuxtLink>
        <div v-else class="later-content compact-later">
          <UIcon name="i-lucide-gem" /><strong>No rare drops recorded</strong>
        </div>
      </article>
      <article class="panel dashboard-chat">
        <div class="panel-header compact">
          <div><h2>Chat Messages</h2></div>
          <NuxtLink class="panel-link" to="/chat">Open chat</NuxtLink>
        </div>
        <ul
          v-if="recentChatMessages.length"
          class="dashboard-event-list dashboard-chat-list"
        >
          <li v-for="item in recentChatMessages" :key="item.event_id">
            <UIcon name="i-lucide-messages-square" />
            <NuxtLink
              :to="{
                path: '/chat',
                query: {
                  character_id: item.character_id,
                  channel: textField(item.payload.channel) || 'unknown',
                },
              }"
            >
              <strong>{{
                textField(item.payload.sender) || item.character
              }}</strong>
              <span class="dashboard-chat-preview"
                >{{
                  textField(item.payload.message) || 'Message text unavailable'
                }}
                · {{ textField(item.payload.channel) || 'Unknown' }} ·
                {{ item.server }}</span
              >
            </NuxtLink>
            <time :datetime="item.occurred_at">{{
              formatTimestamp(item.occurred_at)
            }}</time>
          </li>
        </ul>
        <div v-else class="later-content compact-later dashboard-chat-empty">
          <UIcon name="i-lucide-messages-square" /><strong
            >No recent messages</strong
          >
        </div>
      </article>
    </div>

    <article class="panel dashboard-later dashboard-offers">
      <div class="panel-header compact">
        <div>
          <h2>Global Offers</h2>
          <p>Recent buy, sell and trade offers</p>
        </div>
        <span class="later-badge">LATER</span>
      </div>
      <div class="later-content">
        <UIcon name="i-lucide-store" /><strong>LATER</strong
        ><span>Economy data arrives in a later slice.</span>
      </div>
    </article>
  </section>
</template>
