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
const operatorSecret = ref('')
const {
  ready: operatorReady,
  authenticated: operatorAuthenticated,
  busy: operatorBusy,
  error: operatorError,
  loginOperator,
} = useOperatorSession()
const { startLiveData, stopLiveData } = useLiveData()

watch(
  operatorAuthenticated,
  (authenticated) => {
    if (authenticated) startLiveData()
    else stopLiveData()
  },
  { immediate: true },
)

async function submitOperatorLogin() {
  const secret = operatorSecret.value
  if (!secret || operatorBusy.value) return
  if (await loginOperator(secret)) operatorSecret.value = ''
}

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
  <div v-if="!operatorReady" class="operator-gate">
    <div class="operator-card">
      <p class="eyebrow">PhMon control plane</p>
      <h1>Checking operator session…</h1>
    </div>
  </div>
  <div v-else-if="!operatorAuthenticated" class="operator-gate">
    <form class="operator-card" @submit.prevent="submitOperatorLogin">
      <p class="eyebrow">PhMon control plane</p>
      <h1>Operator sign in</h1>
      <p>
        Enter the access secret configured on this self-hosted instance. The
        secret is submitted once and is never stored in browser storage.
      </p>
      <label for="operator-secret">Access secret</label>
      <input
        id="operator-secret"
        v-model="operatorSecret"
        aria-label="Operator access secret"
        type="password"
        autocomplete="current-password"
        :disabled="operatorBusy"
      />
      <p v-if="operatorError" class="form-error" role="alert">
        {{ operatorError }}
      </p>
      <button type="submit" :disabled="operatorBusy || !operatorSecret">
        {{ operatorBusy ? 'Signing in…' : 'Sign in' }}
      </button>
    </form>
  </div>
  <div
    v-else
    class="phmon-shell"
    :class="{ 'is-collapsed': sidebarCollapsed }"
    @keydown.esc="mobileNavigationOpen = false"
  >
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
