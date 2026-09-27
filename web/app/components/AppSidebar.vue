<script setup lang="ts">
defineProps<{
  collapsed: boolean
  mobileOpen: boolean
  advancedMode: boolean
}>()
const route = useRoute()
const { connectedAgents, fleetStatus } = useFleetSummary()
const { liveStale, connectionState } = useLiveData()
const agentsUnavailable = computed(
  () => liveStale.value || connectionState.value === 'stale',
)
const primaryNavigation = [
  { label: 'Dashboard', icon: 'i-lucide-layout-dashboard', href: '/' },
  { label: 'Stats', icon: 'i-lucide-chart-no-axes-combined', href: '/stats' },
  { label: 'Events', icon: 'i-lucide-activity' },
  { label: 'Chat', icon: 'i-lucide-messages-square' },
  { label: 'Economy', icon: 'i-lucide-coins' },
  { label: 'Alchemy', icon: 'i-lucide-flask-conical' },
  { label: 'Academy', icon: 'i-lucide-graduation-cap' },
  { label: 'Guild Storage', icon: 'i-lucide-warehouse' },
  { label: 'phBot', icon: 'i-lucide-bot' },
]

const advancedNavigation = [
  { label: 'Analytics', icon: 'i-lucide-chart-no-axes-column-increasing' },
  { label: 'Map', icon: 'i-lucide-map' },
  { label: 'Item Search', icon: 'i-lucide-search' },
  { label: 'Skill Builder', icon: 'i-lucide-git-branch' },
  { label: 'Automations', icon: 'i-lucide-zap' },
]
</script>

<template>
  <aside class="app-sidebar" :class="{ 'is-mobile-open': mobileOpen }">
    <div class="brand-block">
      <div class="brand-mark" aria-hidden="true">P</div>
      <div class="brand-copy">
        <strong>PhMon</strong>
        <span>self-hosted · slice 2</span>
        <span
          class="brand-connect"
          :class="
            !agentsUnavailable && (connectedAgents ?? 0) > 0
              ? 'is-online'
              : 'is-offline'
          "
          >{{ fleetStatus }}</span
        >
      </div>
      <span
        class="connection-dot"
        :class="
          !agentsUnavailable && (connectedAgents ?? 0) > 0
            ? 'is-online'
            : 'is-offline'
        "
        :title="fleetStatus"
      />
    </div>

    <div class="scope-block">
      <span>Server scope · LATER</span>
      <button type="button" disabled>
        <UIcon name="i-lucide-layers-3" />
        <span>All</span>
        <span class="nav-soon">LATER</span>
        <UIcon name="i-lucide-chevron-down" />
      </button>
    </div>

    <nav class="navigation" aria-label="Primary navigation">
      <template v-for="item in primaryNavigation" :key="item.label">
        <NuxtLink
          v-if="item.href"
          :to="item.href"
          class="nav-item"
          :class="{ active: route.path === item.href }"
          :aria-current="route.path === item.href ? 'page' : undefined"
        >
          <UIcon :name="item.icon" />
          <span>{{ item.label }}</span>
        </NuxtLink>
        <button
          v-else
          type="button"
          class="nav-item"
          disabled
          :title="collapsed ? item.label : undefined"
        >
          <UIcon :name="item.icon" />
          <span>{{ item.label }}</span>
          <span class="nav-soon">later</span>
        </button>
      </template>

      <template v-if="advancedMode">
        <p class="nav-heading">Tools</p>
        <button
          v-for="item in advancedNavigation"
          :key="item.label"
          type="button"
          class="nav-item"
          disabled
          :title="collapsed ? item.label : undefined"
        >
          <UIcon :name="item.icon" />
          <span>{{ item.label }}</span>
          <span class="nav-soon">later</span>
        </button>
      </template>

      <p class="nav-heading">{{ advancedMode ? 'Misc' : 'System' }}</p>
      <button
        class="nav-item"
        type="button"
        disabled
        title="Settings arrive progressively"
      >
        <UIcon name="i-lucide-settings" />
        <span>Settings</span>
        <span class="nav-soon">later</span>
      </button>
      <button
        v-if="advancedMode"
        class="nav-item"
        type="button"
        disabled
        title="Server List arrives later"
      >
        <UIcon name="i-lucide-server" />
        <span>Server List</span>
        <span class="nav-soon">later</span>
      </button>
    </nav>

    <InstanceAccess :collapsed="collapsed" />
  </aside>
</template>
