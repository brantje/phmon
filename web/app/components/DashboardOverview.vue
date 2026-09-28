<script setup lang="ts">
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
const recentEvents = computed(
  () => eventFeeds.value['dashboard-events']?.events || [],
)

function watchDashboardEvents(server: string) {
  const filter = { server: server === 'all' ? undefined : server, limit: 5 }
  setEventFeed('dashboard-deaths', { ...filter, kind: 'character.died' })
  setEventFeed('dashboard-events', filter)
}

watch(serverScope, watchDashboardEvents, { immediate: true })
onBeforeUnmount(() => {
  clearEventFeed('dashboard-deaths')
  clearEventFeed('dashboard-events')
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
          <UIcon name="i-lucide-skull" />
          <NuxtLink :to="`/characters/${item.character_id}`">
            <strong>{{ item.character }}</strong>
            <span>Cause unknown · {{ item.server }}</span>
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
        <li v-for="item in recentEvents" :key="item.event_id">
          <UIcon name="i-lucide-skull" />
          <NuxtLink :to="`/characters/${item.character_id}`">
            <strong>{{ item.character }} died</strong>
            <span>Cause unknown · {{ item.server }}</span>
          </NuxtLink>
          <time :datetime="item.occurred_at">{{
            formatTimestamp(item.occurred_at)
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
          <div><h2>Last Rare Drop</h2></div>
          <span class="later-badge">LATER</span>
        </div>
        <div class="later-content compact-later">
          <UIcon name="i-lucide-gem" /><strong>LATER</strong>
        </div>
      </article>
      <article class="panel dashboard-later">
        <div class="panel-header compact">
          <div><h2>Chat Messages</h2></div>
          <span class="later-badge">LATER</span>
        </div>
        <div class="later-content compact-later">
          <UIcon name="i-lucide-messages-square" /><strong>LATER</strong>
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
