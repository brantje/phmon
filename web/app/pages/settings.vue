<script setup lang="ts">
const { preferences, ready, busy, error, save, requestBrowserPermission } =
  useChatPreferences()
const permission = ref<'default' | 'granted' | 'denied' | 'unsupported'>(
  'default',
)
const localSound = ref(false)
const savingNotice = ref(false)

watch(
  preferences,
  (value) => {
    localSound.value = value.message_sound
  },
  { immediate: true },
)
const notificationStatus = computed(() => {
  if (!import.meta.client || !('Notification' in window)) return 'unsupported'
  return Notification.permission
})
onMounted(() => {
  permission.value = notificationStatus.value
})

async function toggleSound() {
  await save({ ...preferences.value, message_sound: localSound.value })
}

async function enableNotifications() {
  savingNotice.value = true
  if (preferences.value.browser_notifications) {
    await save({ ...preferences.value, browser_notifications: false })
  } else {
    await requestBrowserPermission()
  }
  savingNotice.value = false
  permission.value = notificationStatus.value
}
</script>

<template>
  <div class="settings-page">
    <!-- eslint-disable vue/html-self-closing -->
    <PageHeader
      title="Settings"
      icon="i-lucide-settings"
      description="Operator preferences for chat messages and browser alerts."
    />
    <section class="panel preferences-panel">
      <div class="panel-heading">
        <div>
          <h2>Chat notifications</h2>
          <p>These preferences are stored for this PhMon operator account.</p>
        </div>
        <span v-if="ready" class="preference-state">{{
          busy ? 'Saving…' : 'Saved on this instance'
        }}</span>
      </div>
      <div class="preference-row">
        <div>
          <strong>Message sound</strong>
          <p>
            Play a short local sound for new messages while this browser tab is
            in the background.
          </p>
        </div>
        <label class="switch-label">
          <input
            v-model="localSound"
            type="checkbox"
            :disabled="busy || !ready"
            @change="toggleSound"
          /><span>Enabled</span>
        </label>
      </div>
      <div class="preference-row">
        <div>
          <strong>Browser notifications</strong>
          <p>
            Show message previews while PhMon is in the background. The browser
            asks for permission when you press the button.
          </p>
        </div>
        <div class="notification-actions">
          <span class="permission-state" :class="`permission-${permission}`">{{
            permission === 'granted'
              ? 'Allowed'
              : permission === 'denied'
                ? 'Blocked in browser settings'
                : permission === 'unsupported'
                  ? 'Not supported'
                  : 'Permission not requested'
          }}</span>
          <button
            class="compact-button"
            type="button"
            :disabled="
              busy ||
              savingNotice ||
              !ready ||
              (!preferences.browser_notifications &&
                (permission === 'unsupported' || permission === 'denied'))
            "
            @click="enableNotifications"
          >
            {{
              savingNotice
                ? 'Saving…'
                : preferences.browser_notifications
                  ? 'Disable alerts'
                  : permission === 'granted'
                    ? 'Enable alerts'
                    : 'Allow notifications'
            }}
          </button>
        </div>
      </div>
      <p v-if="error" class="form-error" role="alert">{{ error }}</p>
      <p class="settings-note">
        Desktop alerts use chat event IDs for duplicate suppression. Message
        text is shown only after browser permission is granted.
      </p>
    </section>
  </div>
</template>

<style scoped>
.settings-page {
  display: grid;
  gap: 14px;
}
.preferences-panel {
  padding: 16px;
}
.panel-heading {
  display: flex;
  align-items: start;
  justify-content: space-between;
  gap: 12px;
  border-bottom: 1px solid var(--ph-border-soft);
  padding-bottom: 12px;
}
.panel-heading h2 {
  margin: 0;
  color: var(--ph-primary);
  font-size: 17px;
}
.panel-heading p,
.preference-row p {
  margin: 4px 0 0;
  color: var(--ph-muted);
  font-size: 12px;
  line-height: 1.45;
}
.preference-state {
  color: var(--ph-green);
  font-size: 11px;
}
.preference-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  padding: 14px 0;
  border-bottom: 1px solid var(--ph-border-soft);
}
.preference-row strong {
  color: var(--ph-text);
}
.switch-label,
.notification-actions {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ph-muted);
  font-size: 12px;
  white-space: nowrap;
}
.switch-label input {
  accent-color: #5882ac;
  width: 16px;
  height: 16px;
}
.compact-button {
  border: 1px solid var(--ph-border);
  border-radius: 4px;
  background: #151e2a;
  color: var(--ph-text);
  padding: 7px 10px;
  cursor: pointer;
}
.compact-button:disabled {
  opacity: 0.5;
  cursor: not-allowed;
}
.permission-state {
  color: var(--ph-muted);
  font-size: 11px;
}
.permission-granted {
  color: var(--ph-green);
}
.permission-denied {
  color: var(--ph-amber);
}
.form-error {
  color: var(--ph-red);
  font-size: 12px;
}
.settings-note {
  margin: 12px 0 0;
  color: var(--ph-muted);
  font-size: 11px;
}
@media (max-width: 640px) {
  .preference-row,
  .panel-heading {
    align-items: flex-start;
    flex-direction: column;
  }
  .notification-actions {
    align-items: flex-start;
    flex-direction: column;
    white-space: normal;
  }
}
</style>
