<script setup lang="ts">
const {
  onlineCharacterCount,
  offlineCharacterCount,
  displayCharacterCount,
  observedGold,
} = useFleetSummary()
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
        <div class="dashboard-count later-count">
          <span>Alive</span><strong>LATER</strong>
        </div>
        <div class="dashboard-count later-count">
          <span>Dead</span><strong>LATER</strong>
        </div>
      </div>
      <div class="dashboard-gold">
        <UIcon name="i-lucide-coins" /><span>Total Gold</span
        ><strong>{{ observedGold }}</strong>
      </div>
    </article>

    <article class="panel dashboard-later dashboard-deaths">
      <div class="panel-header compact">
        <div>
          <h2>Last Deaths</h2>
          <p>Recent character deaths</p>
        </div>
        <span class="later-badge">LATER</span>
      </div>
      <div class="later-content">
        <UIcon name="i-lucide-skull" /><strong>LATER</strong
        ><span>Event history arrives in a later slice.</span>
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

    <article class="panel dashboard-later dashboard-events">
      <div class="panel-header compact">
        <div>
          <h2>Recent Events</h2>
          <p>Latest activity across your characters</p>
        </div>
        <span class="later-badge">LATER</span>
      </div>
      <div class="later-content">
        <UIcon name="i-lucide-clock-3" /><strong>LATER</strong
        ><span>Timeline data arrives in a later slice.</span>
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
