<script setup lang="ts">
import type {
  ActivityEvent,
  RecentTradesSnapshot,
  TradeReport,
} from '~~/shared/types/live'
import {
  formatSignedTradeGold,
  formatTradeDuration,
  tradeOutcomeLabel,
  tradeRouteLabel,
} from '~/utils/tradeReportSummary'
import { groupRecentEvents } from '~/utils/groupRecentEvents'
import { deathCauseLabel } from '~/utils/deathCause'
import { offerItemChips } from '~/utils/offerTaxonomy'
import { itemRecordFromActivityEvent } from '~/utils/itemDetailPopup'
import {
  dashboardNoisyEvent,
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
  refreshLiveData,
} = useLiveData()
const { serverScope } = useServerScope()
const lastDeaths = computed(
  () => eventFeeds.value['dashboard-deaths']?.events || [],
)
const recentEvents = computed(() => {
  const general = (eventFeeds.value['dashboard-events']?.events || []).filter(
    (event) => !dashboardNoisyEvent(event.kind),
  )
  const uniques = eventFeeds.value['dashboard-uniques']?.events || []
  const byID = new Map<string, ActivityEvent>()
  for (const event of [...uniques, ...general]) byID.set(event.event_id, event)
  const merged = [...byID.values()].sort(
    (left, right) =>
      Date.parse(right.occurred_at) - Date.parse(left.occurred_at),
  )
  return groupRecentEvents(merged, 5)
})
const recentRareDrops = computed(
  () => eventFeeds.value['dashboard-rare-drops']?.events || [],
)
const recentChatMessages = computed(
  () => eventFeeds.value['dashboard-chat']?.events || [],
)
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
const tradeProfitLabel = computed(() => {
  const snapshot = tradeSnapshot.value
  if (!snapshot || snapshot.profit_samples_24h === 0) return '—'
  return formatSignedTradeGold(snapshot.profit_24h)
})

function tradeRowMeta(trade: TradeReport) {
  const parts = [trade.reporter.name]
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
  setEventFeed('dashboard-deaths', {
    ...scope,
    limit: 5,
    kind: 'character.died',
  })
  setEventFeed('dashboard-events', { ...scope, limit: 50 })
  setEventFeed('dashboard-uniques', {
    ...scope,
    limit: 5,
    kind: 'world.unique_spawned',
  })
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
  clearEventFeed('dashboard-uniques')
  clearEventFeed('dashboard-rare-drops')
  clearEventFeed('dashboard-chat')
  clearOfferFeed('dashboard-offers')
  clearTradeFeed('dashboard-trades')
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

    <article
      class="panel recent-trades"
      aria-label="Recent trades"
      :aria-busy="tradesStatus === 'loading'"
    >
      <div class="panel-header compact">
        <div>
          <h2>Recent Trades</h2>
          <p>
            Last 24h profit:
            <strong
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
      <ul v-if="recentTrades.length" class="recent-trades-list">
        <li v-for="trade in recentTrades" :key="trade.trade_id">
          <NuxtLink to="/events?view=trades">
            <span class="recent-trades-main">
              <strong>{{ tradeRouteLabel(trade) }}</strong>
              <span>{{ tradeOutcomeLabel(trade) }}</span>
              <small>{{ tradeRowMeta(trade) }}</small>
            </span>
            <span class="recent-trades-side">
              <strong
                v-if="typeof trade.gold === 'number'"
                :class="{
                  'trade-profit-positive': trade.gold > 0,
                  'trade-profit-negative': trade.gold < 0,
                }"
                >{{ formatSignedTradeGold(trade.gold) }}</strong
              >
              <time :datetime="trade.finished_at">{{
                formatTimestamp(trade.finished_at)
              }}</time>
            </span>
          </NuxtLink>
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
          <img
            v-if="uniqueEventDetails(item.event)?.imageUrl"
            class="unique-event-art"
            :src="uniqueEventDetails(item.event)?.imageUrl"
            alt=""
          />
          <UIcon v-else name="i-lucide-activity" />
          <div
            v-if="itemRecordFromActivityEvent(item.event)"
            class="dashboard-event-row"
          >
            <NuxtLink
              v-if="item.event.character_id"
              :to="`/characters/${item.event.character_id}`"
            >
              <CharacterPortrait
                :name="item.event.character"
                :portrait-url="item.event.portrait_url"
                size="small"
              />
            </NuxtLink>
            <span class="dashboard-event-identity">
              <span class="dashboard-event-item">
                <strong>{{ itemEventLabel(item.event) }}</strong>
                <ItemDetailPopup
                  :item="itemRecordFromActivityEvent(item.event)!"
                  :to="dashboardItemTarget(item.event)"
                />
              </span>
              <span>{{ eventObservers(item) }}</span>
            </span>
          </div>
          <NuxtLink
            v-else-if="item.event.character_id"
            :to="`/characters/${item.event.character_id}`"
          >
            <CharacterPortrait
              v-if="!uniqueEventDetails(item.event)?.imageUrl"
              :name="item.event.character"
              :portrait-url="item.event.portrait_url"
              size="small"
            />
            <span class="dashboard-event-identity">
              <strong>{{ eventHeadline(item.event) }}</strong>
              <span>{{
                uniqueEventDetails(item.event)
                  ? `Seen by ${item.event.character || 'unknown'}`
                  : eventObservers(item)
              }}</span>
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
        <div
          v-if="recentRareDrops[0]"
          class="later-content compact-later dashboard-rare-link"
        >
          <ItemDetailPopup
            v-if="itemRecordFromActivityEvent(recentRareDrops[0])"
            :item="itemRecordFromActivityEvent(recentRareDrops[0])!"
            :to="{
              path: '/events',
              query: {
                kind: 'drop.rare',
                item: rareItemFilter(recentRareDrops[0]) || undefined,
              },
            }"
          />
          <strong v-else class="rare-drop-item">{{
            eventHeadline(recentRareDrops[0])
          }}</strong>
          <span>{{
            recentRareDrops[0].character || recentRareDrops[0].server
          }}</span>
        </div>
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
            ><time :datetime="offer.occurred_at">{{
              formatTimestamp(offer.occurred_at)
            }}</time>
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
