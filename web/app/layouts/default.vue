<script setup lang="ts">
const mode = useCookie<'easy' | 'advanced'>('phmon-mode', {
  default: () => 'easy',
  sameSite: 'lax',
})
const advancedMode = computed({
  get: () => mode.value === 'advanced',
  set: (value: boolean) => {
    mode.value = value ? 'advanced' : 'easy'
  },
})

const sidebarCollapsed = useCookie<boolean>('phmon-sidebar-collapsed', {
  default: () => false,
  sameSite: 'lax',
})
const mobileNavigationOpen = ref(false)

const route = useRoute()
watch(
  () => route.path,
  () => {
    mobileNavigationOpen.value = false
  },
)
await useBackendHealthMonitor()
</script>

<template>
  <div class="phmon-shell" :class="{ 'is-collapsed': sidebarCollapsed }">
    <button
      v-if="mobileNavigationOpen"
      class="mobile-backdrop"
      type="button"
      aria-label="Close navigation"
      @click="mobileNavigationOpen = false"
    />
    <AppSidebar
      :collapsed="sidebarCollapsed"
      :mobile-open="mobileNavigationOpen"
      :advanced-mode="advancedMode"
    />
    <div class="workspace">
      <AppTopBar
        v-model:collapsed="sidebarCollapsed"
        v-model:mobile-open="mobileNavigationOpen"
        v-model:advanced-mode="advancedMode"
      />
      <main class="workspace-content"><slot /></main>
    </div>
  </div>
</template>
