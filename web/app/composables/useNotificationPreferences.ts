export interface NotificationPreferences {
  popup_kinds: string[]
  browser_kinds: string[]
  chat_channels: string[]
  chat_sound: boolean
}

export interface NotificationCatalogCategory {
  id: string
  label: string
  kinds: { id: string; label: string }[]
}

const preferences = ref<NotificationPreferences>({
  popup_kinds: [],
  browser_kinds: [],
  chat_channels: [
    'general',
    'private',
    'party',
    'guild',
    'union',
    'global',
    'unknown',
  ],
  chat_sound: false,
})
const ready = ref(false)
const busy = ref(false)
const error = ref('')
let loading: Promise<void> | undefined

export function useNotificationPreferences() {
  async function load() {
    if (loading) return loading
    loading = (async () => {
      try {
        preferences.value = await $fetch<NotificationPreferences>(
          '/api/notifications/preferences',
        )
        ready.value = true
        error.value = ''
      } catch {
        error.value = 'Notification preferences are unavailable.'
      } finally {
        loading = undefined
      }
    })()
    return loading
  }

  async function save(next: NotificationPreferences) {
    busy.value = true
    try {
      preferences.value = await $fetch<NotificationPreferences>(
        '/api/notifications/preferences',
        { method: 'PUT', body: next },
      )
      ready.value = true
      error.value = ''
      return true
    } catch {
      error.value = 'Could not save notification preferences.'
      return false
    } finally {
      busy.value = false
    }
  }

  return {
    preferences: readonly(preferences),
    ready: readonly(ready),
    busy: readonly(busy),
    error: readonly(error),
    load,
    save,
  }
}
