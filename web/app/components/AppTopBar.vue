<script setup lang="ts">
const collapsed = defineModel<boolean>('collapsed', { required: true })
const mobileOpen = defineModel<boolean>('mobileOpen', { required: true })
const advancedMode = defineModel<boolean>('advancedMode', { required: true })
const backendReady = useBackendHealth()
const {
  onlineCharacterCount,
  offlineCharacterCount,
  displayCharacterCount,
  combinedVitals,
  observedGold,
} = useFleetSummary()
</script>

<template>
  <div class="top-strip">
    <div class="top-left">
      <button
        class="icon-button mobile-menu"
        type="button"
        aria-label="Open navigation"
        @click="mobileOpen = true"
      >
        <UIcon name="i-lucide-menu" />
      </button>
      <button
        class="icon-button collapse-button"
        type="button"
        :aria-label="collapsed ? 'Expand sidebar' : 'Collapse sidebar'"
        @click="collapsed = !collapsed"
      >
        <UIcon
          :name="
            collapsed ? 'i-lucide-panel-left-open' : 'i-lucide-panel-left-close'
          "
        />
      </button>
      <div class="top-summary" aria-label="Live character summary">
        <span class="top-summary-item"
          ><UIcon name="i-lucide-users-round" /> Chars:
          <strong
            >{{ displayCharacterCount(onlineCharacterCount) }} online</strong
          ><i>|</i
          ><span
            >{{ displayCharacterCount(offlineCharacterCount) }} offline</span
          ></span
        >
        <span class="top-summary-item"
          ><UIcon name="i-lucide-heart-pulse" /> Combined stats:
          <strong>HP {{ combinedVitals.hp }}</strong
          ><i>|</i><strong>MP {{ combinedVitals.mp }}</strong></span
        >
        <span class="top-summary-item"
          ><UIcon name="i-lucide-coins" /> Total Gold:
          <strong>{{ observedGold }}</strong></span
        >
      </div>
      <span class="sr-only" role="status">{{
        backendReady ? 'Backend ready' : 'Backend unavailable'
      }}</span>
    </div>

    <label class="mode-toggle">
      <span>Easy</span>
      <input
        v-model="advancedMode"
        type="checkbox"
        aria-label="Toggle advanced mode"
      />
      <span>Advanced</span>
    </label>
  </div>
</template>
