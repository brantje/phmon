type OperatorSessionResponse = {
  authenticated: boolean
  actor?: string
  expires_at?: string
}

const operatorReady = ref(false)
const operatorAuthenticated = ref(false)
const operatorExpiresAt = ref<string | null>(null)
const operatorBusy = ref(false)
const operatorError = ref('')
let refreshPromise: Promise<void> | null = null

async function refreshOperatorSession() {
  if (refreshPromise) return refreshPromise
  refreshPromise = (async () => {
    try {
      const session = await $fetch<OperatorSessionResponse>('/api/auth/session')
      operatorAuthenticated.value = session.authenticated === true
      operatorExpiresAt.value = session.expires_at || null
      operatorError.value = ''
    } catch {
      operatorAuthenticated.value = false
      operatorExpiresAt.value = null
    } finally {
      operatorReady.value = true
      refreshPromise = null
    }
  })()
  return refreshPromise
}

async function loginOperator(secret: string) {
  operatorBusy.value = true
  operatorError.value = ''
  try {
    const session = await $fetch<OperatorSessionResponse>('/api/auth/login', {
      method: 'POST',
      body: { secret },
    })
    operatorAuthenticated.value = session.authenticated === true
    operatorExpiresAt.value = session.expires_at || null
    operatorReady.value = true
    return operatorAuthenticated.value
  } catch {
    operatorAuthenticated.value = false
    operatorExpiresAt.value = null
    operatorError.value = 'Sign-in failed. Check the operator access secret.'
    return false
  } finally {
    operatorBusy.value = false
  }
}

async function logoutOperator() {
  operatorBusy.value = true
  try {
    await $fetch('/api/auth/logout', { method: 'POST' })
    operatorAuthenticated.value = false
    operatorExpiresAt.value = null
    operatorError.value = ''
    return true
  } catch {
    operatorError.value = 'Sign-out failed. The session may still be active.'
    return false
  } finally {
    operatorBusy.value = false
  }
}

export function useOperatorSession() {
  onMounted(() => {
    void refreshOperatorSession()
  })
  return {
    ready: readonly(operatorReady),
    authenticated: readonly(operatorAuthenticated),
    expiresAt: readonly(operatorExpiresAt),
    busy: readonly(operatorBusy),
    error: readonly(operatorError),
    refreshOperatorSession,
    loginOperator,
    logoutOperator,
  }
}
