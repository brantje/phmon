<script setup lang="ts">
import type {
  ActivityEvent,
  RecentTradesSnapshot,
  ThiefSighting,
  ThiefSightingPage,
  TradeReport,
} from '~~/shared/types/live'
import {
  formatSignedTradeGold,
  formatTradeDuration,
  tradeOutcomeLabel,
  tradeRouteLabel,
} from '~/utils/tradeReportSummary'
import {
  groupRecentEvents,
  mergeRecentEventFeeds,
  recentEventFeedsSettled,
} from '~/utils/groupRecentEvents'
import { notificationPresentation } from '~/utils/notificationPresentation'
import { deathCauseLabel } from '~/utils/deathCause'
import { formatCompactObservedGold } from '~/utils/fleetGold'
import { offerItemChips } from '~/utils/offerTaxonomy'
import { itemRecordFromActivityEvent } from '~/utils/itemDetailPopup'
import {
  DASHBOARD_NOISY_KINDS,
  uniqueEventDetails,
  uniqueEventHeadline,
} from '~/utils/uniqueEvent'

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
const {
  eventFeeds,
  eventFeedStatus,
  offerFeeds,
  offerFeedStatus,
  connectionState,
  liveStale,
  setEventFeed,
  clearEventFeed,
  setOfferFeed,
  clearOfferFeed,
  tradeFeeds,
  tradeFeedStatus,
  setTradeFeed,
  clearTradeFeed,
  thiefFeeds,
  thiefFeedStatus,
  setThiefFeed,
  clearThiefFeed,
  refreshLiveData,
} = useLiveData()
const { serverScope } = useServerScope()
const lastDeaths = computed(
  () => eventFeeds.value['dashboard-deaths']?.events || [],
)
// Recent Events scrolls inside its card. The server leaves out the noisy kinds,
// so every feed returns rows the card actually shows.
const RECENT_EVENT_ROWS = 25
const RECENT_EVENT_FEEDS = [
  'dashboard-uniques',
  'dashboard-hunters',
  'dashboard-events',
] as const
// Wait for every feed (an unavailable one counts as answered) so one arriving
// first does not briefly show only uniques or only hunters.
const recentEventsLoaded = computed(() =>
  recentEventFeedsSettled(
    RECENT_EVENT_FEEDS,
    eventFeeds.value,
    eventFeedStatus.value,
  ),
)
const recentEventsUnavailable = computed(() =>
  RECENT_EVENT_FEEDS.some((id) => eventFeedStatus.value[id] === 'unavailable'),
)
const recentEvents = computed(() =>
  !recentEventsLoaded.value
    ? []
    : groupRecentEvents(
        mergeRecentEventFeeds(
          RECENT_EVENT_FEEDS.map((id) => ({
            events: eventFeeds.value[id]?.events || [],
            limit: RECENT_EVENT_ROWS,
          })),
        ),
        RECENT_EVENT_ROWS,
      ),
)
const recentEventRows = computed(() =>
  recentEvents.value.map((group) => {
    const { event } = group
    const unique = uniqueEventDetails(event)
    const item = ITEM_EVENT_KIND.test(event.kind)
      ? itemRecordFromActivityEvent(event)
      : null
    return {
      id: event.event_id,
      event,
      art: unique?.imageUrl,
      item,
      title: item ? itemEventLabel(event) : eventHeadline(event),
      detail: recentEventDetail(group),
    }
  }),
)
const recentRareDrops = computed(
  () => eventFeeds.value['dashboard-rare-drops']?.events || [],
)
// Several characters observe the same General/Global line; show it once.
const recentChatMessages = computed(() =>
  groupRecentEvents(eventFeeds.value['dashboard-chat']?.events || [], 3),
)
const chatChannelLabels: Record<string, string> = {
  general: 'General chat',
  private: 'Private message',
  party: 'Party chat',
  guild: 'Guild chat',
  union: 'Union chat',
  global: 'Global chat',
}

function chatChannelLabel(event: ActivityEvent) {
  return chatChannelLabels[textField(event.payload.channel)] || 'Chat'
}

function chatSender(event: ActivityEvent) {
  return textField(event.payload.sender) || event.character || 'Unknown'
}

function chatObservers(item: (typeof recentChatMessages.value)[number]) {
  const names = item.observers.map((observer) => observer.character)
  const observed = names.length
    ? names.join(', ')
    : item.event.character || 'Agent'
  return item.event.server
    ? `${item.event.server} · Observed by ${observed}`
    : `Observed by ${observed}`
}
const recentOffers = computed(
  () => offerFeeds.value['dashboard-offers']?.offers || [],
)
const tradeSnapshot = computed(
  () =>
    (tradeFeeds.value['dashboard-trades'] as
      RecentTradesSnapshot | undefined) ?? null,
)
const recentTrades = computed(() => tradeSnapshot.value?.reports || [])
const tradesStatus = computed(
  () => tradeFeedStatus.value['dashboard-trades'] || 'loading',
)
const thiefPage = computed(
  () =>
    (thiefFeeds.value['dashboard-thieves'] as ThiefSightingPage | undefined) ??
    null,
)
const recentThieves = computed(() => thiefPage.value?.sightings || [])
const thievesStatus = computed(
  () => thiefFeedStatus.value['dashboard-thieves'] || 'loading',
)

function thiefRowMeta(row: ThiefSighting) {
  const source =
    row.origin === 'phmon'
      ? 'PhMon'
      : row.reporter.app?.trim() || 'AdvancedAutoTrade'
  const location =
    row.zone?.trim() ||
    (row.position ? `Region ${row.position.region}` : 'Location unknown')
  return [location, row.reporter.name || 'Unknown reporter', source].join(' · ')
}

const tradeProfitLabel = computed(() => {
  const snapshot = tradeSnapshot.value
  if (!snapshot || snapshot.profit_samples_24h === 0) return '—'
  const sign = snapshot.profit_24h > 0 ? '+' : ''
  return `${sign}${formatCompactObservedGold(snapshot.profit_24h)} gold`
})
const tradeProfitTitle = computed(() => {
  const snapshot = tradeSnapshot.value
  if (!snapshot || snapshot.profit_samples_24h === 0) return undefined
  return formatSignedTradeGold(snapshot.profit_24h)
})

function tradeOutcomeIcon(trade: TradeReport) {
  if (trade.outcome === 'success') return 'i-lucide-circle-check'
  if (trade.outcome === 'failed') return 'i-lucide-circle-x'
  return 'i-lucide-circle-slash'
}

function tradeRowMeta(trade: TradeReport) {
  const parts = [tradeOutcomeLabel(trade), trade.reporter.name]
  if (typeof trade.duration_s === 'number') {
    parts.push(formatTradeDuration(trade.duration_s))
  }
  if (trade.stars) {
    parts.push(trade.stars === 'Max' ? 'Max stars' : `${trade.stars} stars`)
  }
  return parts.join(' · ')
}

const offersStatus = computed(
  () => offerFeedStatus.value['dashboard-offers'] || 'loading',
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
      return uniqueEventHeadline(event)
    case 'chat.message_received':
      return `${textField(payload.sender) || event.character || 'Chat'}: ${textField(payload.message)}`
    case 'job.hunter_trader_seen':
      return `Trader / Hunter seen · ${textField(payload.value).trim() || 'Name unavailable'}`
    default: {
      const { title, body } = notificationPresentation(event)
      return body ? `${title} · ${body}` : title
    }
  }
}

// Unique notices carry the monster model in payload.model, so only these kinds
// may be read as item events.
const ITEM_EVENT_KIND = /^(drop|item|alchemy)\./

function recentEventDetail(item: (typeof recentEvents.value)[number]) {
  const { event } = item
  if (event.kind === 'character.died') {
    return [deathCauseLabel(event.payload), event.server]
      .filter(Boolean)
      .join(' · ')
  }
  if (uniqueEventDetails(event)) {
    const names = item.observers.length
      ? item.observers.map((observer) => observer.character).join(', ')
      : event.character || 'unknown'
    return [`Seen by ${names}`, event.server].filter(Boolean).join(' · ')
  }
  return eventObservers(item)
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

function rareItemFilter(item: ActivityEvent) {
  const snapshot = item.payload.item as Record<string, unknown> | undefined
  return (
    textField(item.item_code) ||
    textField(snapshot?.servername) ||
    String(item.item_model ?? item.payload.model ?? '')
  )
}

function itemEventLabel(event: ActivityEvent) {
  switch (event.kind) {
    case 'drop.rare':
      return 'Rare drop'
    case 'drop.item':
      return 'Item drop'
    case 'item.acquired':
      return 'Item acquired'
    case 'item.transferred':
      return 'Item transferred'
    case 'item.quantity_increased':
      return 'Item quantity increased'
    case 'item.quantity_decreased':
      return 'Item quantity decreased'
    case 'alchemy.attempt':
      return 'Alchemy attempt'
    case 'alchemy.finished':
      return 'Alchemy finished'
    default:
      return 'Item'
  }
}

function dashboardItemTarget(event: ActivityEvent) {
  return {
    path: '/events',
    query: {
      kind: event.kind,
      item: rareItemFilter(event) || undefined,
    },
  }
}

function watchDashboardEvents(server: string) {
  const scope = { server: server === 'all' ? undefined : server }
  setOfferFeed('dashboard-offers', { ...scope, limit: 5 })
  setTradeFeed('dashboard-trades', { ...scope, limit: 5 })
  setThiefFeed('dashboard-thieves', { ...scope, limit: 5 })
  setEventFeed('dashboard-deaths', {
    ...scope,
    limit: 5,
    kind: 'character.died',
  })
  setEventFeed('dashboard-events', {
    ...scope,
    limit: RECENT_EVENT_ROWS,
    exclude_kinds: [...DASHBOARD_NOISY_KINDS],
  })
  setEventFeed('dashboard-uniques', {
    ...scope,
    limit: RECENT_EVENT_ROWS,
    kind: 'world.unique_spawned',
  })
  setEventFeed('dashboard-hunters', {
    ...scope,
    limit: RECENT_EVENT_ROWS,
    kind: 'job.hunter_trader_seen',
    hide_own: true,
  })
  setEventFeed('dashboard-rare-drops', {
    ...scope,
    limit: 5,
    kind: 'drop.rare',
  })
  setEventFeed('dashboard-chat', {
    ...scope,
    // Fetch extra rows so observer duplicates still leave three messages.
    limit: 12,
    kind: 'chat.message_received',
  })
}

watch(serverScope, watchDashboardEvents, { immediate: true })
onBeforeUnmount(() => {
  clearEventFeed('dashboard-deaths')
  clearEventFeed('dashboard-events')
  clearEventFeed('dashboard-uniques')
  clearEventFeed('dashboard-hunters')
  clearEventFeed('dashboard-rare-drops')
  clearEventFeed('dashboard-chat')
  clearOfferFeed('dashboard-offers')
  clearTradeFeed('dashboard-trades')
  clearThiefFeed('dashboard-thieves')
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
              <span
                >{{ deathCauseLabel(item.payload) }} · {{ item.server }}</span
              >
            </span>
          </NuxtLink>
          <FormattedDate :value="item.occurred_at" />
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

    <article
      class="panel recent-trades"
      aria-label="Recent trades"
      :aria-busy="tradesStatus === 'loading'"
    >
      <div class="panel-header compact">
        <div>
          <h2>Recent Trades</h2>
          <p>
            Last 24h:
            <strong
              :title="tradeProfitTitle"
              :class="{
                'trade-profit-positive': (tradeSnapshot?.profit_24h ?? 0) > 0,
                'trade-profit-negative': (tradeSnapshot?.profit_24h ?? 0) < 0,
              }"
              >{{ tradeProfitLabel }}</strong
            >
            <template v-if="tradeSnapshot">
              · {{ tradeSnapshot.trades_24h }}
              {{ tradeSnapshot.trades_24h === 1 ? 'trade' : 'trades' }}
            </template>
          </p>
        </div>
        <NuxtLink class="panel-link" to="/events?view=trades">
          Show all trades <UIcon name="i-lucide-arrow-up-right" />
        </NuxtLink>
      </div>
      <div
        v-if="liveStale && recentTrades.length"
        class="event-stale-note"
        role="status"
      >
        Showing the last received trades as stale.
      </div>
      <div
        v-if="tradesStatus === 'unavailable'"
        class="dashboard-offer-status"
        role="status"
      >
        Trades are temporarily unavailable.
        <button
          class="compact-button"
          type="button"
          @click="refreshLiveData(['dashboard-trades'])"
        >
          Retry
        </button>
      </div>
      <ul v-if="recentTrades.length" class="dashboard-event-list">
        <li v-for="trade in recentTrades" :key="trade.trade_id">
          <NuxtLink to="/events?view=trades">
            <UIcon
              class="dashboard-trade-icon"
              :class="`is-${trade.outcome}`"
              :name="tradeOutcomeIcon(trade)"
            />
            <span class="dashboard-event-identity">
              <strong>{{ tradeRouteLabel(trade) }}</strong>
              <span>{{ tradeRowMeta(trade) }}</span>
            </span>
          </NuxtLink>
          <span class="dashboard-trade-side">
            <strong
              v-if="typeof trade.gold === 'number'"
              :class="{
                'trade-profit-positive': trade.gold > 0,
                'trade-profit-negative': trade.gold < 0,
              }"
              >{{ formatSignedTradeGold(trade.gold) }}</strong
            >
            <FormattedDate :value="trade.finished_at" />
          </span>
        </li>
      </ul>
      <div
        v-else-if="tradesStatus !== 'unavailable'"
        class="dashboard-event-empty"
      >
        <UIcon name="i-lucide-route" /><span>{{
          tradesStatus === 'loading'
            ? 'Loading trades…'
            : 'No trade reports recorded in this server scope.'
        }}</span>
      </div>
    </article>

    <article
      class="panel recent-thieves"
      aria-label="Recent thieves"
      :aria-busy="thievesStatus === 'loading'"
    >
      <div class="panel-header compact">
        <div>
          <h2>Recent Thieves</h2>
          <p>
            Last 5 sightings<template v-if="thiefPage">
              · {{ thiefPage.total }} recorded</template
            >
          </p>
        </div>
        <NuxtLink class="panel-link" to="/events?view=thieves">
          Show all thieves <UIcon name="i-lucide-arrow-up-right" />
        </NuxtLink>
      </div>
      <div
        v-if="liveStale && recentThieves.length"
        class="event-stale-note"
        role="status"
      >
        Showing the last received sightings as stale.
      </div>
      <div
        v-if="thievesStatus === 'unavailable'"
        class="dashboard-offer-status"
        role="status"
      >
        Thief sightings are temporarily unavailable.
        <button
          class="compact-button"
          type="button"
          @click="refreshLiveData(['dashboard-thieves'])"
        >
          Retry
        </button>
      </div>
      <ul v-if="recentThieves.length" class="dashboard-event-list">
        <li v-for="row in recentThieves" :key="row.sighting_id">
          <NuxtLink to="/events?view=thieves">
            <UIcon class="dashboard-trade-icon" name="i-lucide-user-x" />
            <span class="dashboard-event-identity">
              <strong>{{ row.thief_name }}</strong>
              <span>{{ thiefRowMeta(row) }}</span>
            </span>
          </NuxtLink>
          <span class="dashboard-trade-side">
            <FormattedDate :value="row.received_at" />
          </span>
        </li>
      </ul>
      <div
        v-else-if="thievesStatus !== 'unavailable'"
        class="dashboard-event-empty"
      >
        <UIcon name="i-lucide-user-x" /><span>{{
          thievesStatus === 'loading'
            ? 'Loading sightings…'
            : 'No thief sightings recorded in this server scope.'
        }}</span>
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
      <div
        v-if="liveStale && recentEventRows.length"
        class="event-stale-note"
        role="status"
      >
        Showing the last received events as stale.
      </div>
      <div v-if="recentEventRows.length" class="dashboard-recent-scroll">
        <ul class="dashboard-recent-list">
          <li v-for="row in recentEventRows" :key="row.id">
            <div v-if="row.item" class="dashboard-event-row">
              <CharacterPortrait
                v-if="row.event.character_id"
                :name="row.event.character"
                :portrait-url="row.event.portrait_url"
                :to="`/characters/${row.event.character_id}`"
                size="small"
              />
              <span v-else class="dashboard-event-lead">
                <UIcon name="i-lucide-gem" />
              </span>
              <span class="dashboard-event-identity">
                <span class="dashboard-event-item">
                  <strong>{{ row.title }}</strong>
                  <ItemDetailPopup
                    :item="row.item"
                    :to="dashboardItemTarget(row.event)"
                  />
                </span>
                <span>{{ row.detail }}</span>
              </span>
            </div>
            <NuxtLink
              v-else-if="row.event.character_id"
              :to="`/characters/${row.event.character_id}`"
            >
              <img
                v-if="row.art"
                class="unique-event-art"
                :src="row.art"
                alt=""
              />
              <CharacterPortrait
                v-else
                :name="row.event.character"
                :portrait-url="row.event.portrait_url"
                size="small"
              />
              <span class="dashboard-event-identity">
                <strong>{{ row.title }}</strong>
                <span>{{ row.detail }}</span>
              </span>
            </NuxtLink>
            <div v-else class="dashboard-event-row">
              <img
                v-if="row.art"
                class="unique-event-art"
                :src="row.art"
                alt=""
              />
              <span v-else class="dashboard-event-lead">
                <UIcon name="i-lucide-activity" />
              </span>
              <span class="dashboard-event-identity">
                <strong>{{ row.title }}</strong>
                <span>{{ row.detail }}</span>
              </span>
            </div>
            <FormattedDate :value="row.event.occurred_at" />
          </li>
        </ul>
      </div>
      <div v-else class="dashboard-event-empty">
        <UIcon name="i-lucide-clock-3" />
        <span>{{
          recentEventsLoaded && connectionState === 'current'
            ? 'No recent events.'
            : recentEventsLoaded && recentEventsUnavailable
              ? 'Event history is unavailable.'
              : 'Waiting for event history…'
        }}</span>
      </div>
    </article>

    <div class="dashboard-stack">
      <article class="panel dashboard-rare-drops">
        <div class="panel-header compact">
          <div><h2>Rare Drops</h2></div>
          <NuxtLink class="panel-link" to="/events?kind=drop.rare"
            >View all</NuxtLink
          >
        </div>
        <ul
          v-if="recentRareDrops.length"
          class="dashboard-event-list dashboard-rare-list"
        >
          <li
            v-for="event in recentRareDrops.slice(0, 5)"
            :key="event.event_id"
          >
            <span class="dashboard-event-item">
              <ItemDetailPopup
                v-if="itemRecordFromActivityEvent(event)"
                :item="itemRecordFromActivityEvent(event)!"
                :to="{
                  path: '/events',
                  query: {
                    kind: 'drop.rare',
                    item: rareItemFilter(event) || undefined,
                  },
                }"
              />
              <strong v-else class="rare-drop-item">{{
                eventHeadline(event)
              }}</strong>
            </span>
            <span class="dashboard-trade-side">
              <NuxtLink
                v-if="event.character_id"
                class="dashboard-rare-character"
                :to="`/characters/${event.character_id}`"
                >{{ event.character }}</NuxtLink
              >
              <span v-else class="dashboard-rare-character">{{
                event.character || event.server
              }}</span>
              <FormattedDate :value="event.occurred_at" />
            </span>
          </li>
        </ul>
        <div v-else class="dashboard-event-empty">
          <UIcon name="i-lucide-gem" />
          <span>No rare drops recorded</span>
        </div>
      </article>
      <article class="panel dashboard-chat">
        <div class="panel-header compact">
          <div><h2>Chat Messages</h2></div>
          <NuxtLink class="panel-link" to="/chat">Open chat</NuxtLink>
        </div>
        <ul v-if="recentChatMessages.length" class="dashboard-chat-list">
          <li
            v-for="{ event, observers } in recentChatMessages"
            :key="event.event_id"
          >
            <NuxtLink
              :class="[
                'dashboard-chat-message',
                `channel-${textField(event.payload.channel) || 'unknown'}`,
              ]"
              :to="{
                path: '/chat',
                query: {
                  character_id: event.character_id,
                  channel: textField(event.payload.channel) || 'unknown',
                  peer:
                    textField(event.payload.channel) === 'private'
                      ? chatSender(event)
                      : undefined,
                },
              }"
            >
              <span class="dashboard-chat-avatar" aria-hidden="true">{{
                chatSender(event).charAt(0).toUpperCase()
              }}</span>
              <span class="dashboard-chat-body">
                <span class="dashboard-chat-head">
                  <strong>{{ chatSender(event) }}</strong>
                  <FormattedDate :value="event.occurred_at" />
                </span>
                <span class="dashboard-chat-channel">{{
                  chatChannelLabel(event)
                }}</span>
                <span class="dashboard-chat-text">{{
                  textField(event.payload.message) || 'Message text unavailable'
                }}</span>
                <small class="dashboard-chat-source">{{
                  chatObservers({ event, observers })
                }}</small>
              </span>
            </NuxtLink>
          </li>
        </ul>
        <div v-else class="later-content compact-later dashboard-chat-empty">
          <UIcon name="i-lucide-messages-square" /><strong
            >No recent messages</strong
          >
        </div>
      </article>
    </div>

    <article
      class="panel dashboard-offers"
      aria-label="Recent Global offers"
      :aria-busy="offersStatus === 'loading'"
    >
      <div class="panel-header compact">
        <div>
          <h2>Global Offers</h2>
          <p>Recent buy, sell and trade offers</p>
        </div>
        <NuxtLink class="panel-link" to="/economy"
          >Show all offers <UIcon name="i-lucide-arrow-up-right"
        /></NuxtLink>
      </div>
      <div
        v-if="liveStale && recentOffers.length"
        class="event-stale-note"
        role="status"
      >
        Showing the last received offers as stale.
      </div>
      <div
        v-if="offersStatus === 'unavailable'"
        class="dashboard-offer-status"
        role="status"
      >
        Offers are temporarily unavailable.
        <button
          class="compact-button"
          type="button"
          @click="refreshLiveData(['dashboard-offers'])"
        >
          Retry
        </button>
      </div>
      <ul v-if="recentOffers.length" class="dashboard-offer-list">
        <li v-for="offer in recentOffers" :key="offer.message_id">
          <div class="dashboard-offer-meta">
            <strong>{{ offer.offer_type }}</strong
            ><span>{{ offer.sender }}</span
            ><FormattedDate :value="offer.occurred_at" />
          </div>
          <NuxtLink
            :to="{
              path: '/events',
              query: { event_id: offer.source_event_id },
            }"
            :title="offer.message"
            >{{ offer.message }}</NuxtLink
          >
          <div v-if="offer.items?.length" class="economy-chips">
            <span
              v-for="chip in offerItemChips(offer.items[0]!).slice(0, 5)"
              :key="chip"
              class="economy-chip"
              >{{ chip }}</span
            >
          </div>
          <div class="dashboard-offer-foot">
            <small>{{ offer.server }} · Observed by {{ offer.observer }}</small>
            <NuxtLink
              class="compact-button"
              :to="{
                path: '/chat',
                query: {
                  channel: 'private',
                  character_id: offer.observer_character_id,
                  peer: offer.sender,
                },
              }"
              >Write PM</NuxtLink
            >
          </div>
        </li>
      </ul>
      <div
        v-else-if="offersStatus !== 'unavailable'"
        class="dashboard-event-empty"
      >
        <UIcon name="i-lucide-store" /><span>{{
          offersStatus === 'loading'
            ? 'Loading Global offers…'
            : 'No buy, sell or trade offers recorded in this server scope.'
        }}</span>
      </div>
    </article>
  </section>
</template>
