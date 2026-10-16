<script setup lang="ts">
import type { NotificationCatalogCategory } from '~/composables/useNotificationPreferences'

const route = useRoute()
const settingsSection = computed(() =>
  route.query.section === 'agents'
    ? 'agents'
    : route.query.section === 'command-review'
      ? 'command-review'
      : 'notifications',
)
const { preferences, ready, busy, error, load, save } =
  useNotificationPreferences()
const reviewActions = useReviewActionsPreference()
const catalog = ref<NotificationCatalogCategory[]>([])
const catalogError = ref('')
const permission = ref<'default' | 'granted' | 'denied' | 'unsupported'>(
  'default',
)
const permissionError = ref('')
const expanded = ref<Record<string, boolean>>({})

onMounted(async () => {
  refreshPermission()
  window.addEventListener('focus', refreshPermission)
  await Promise.all([load(), loadCatalog()])
})
onBeforeUnmount(() => window.removeEventListener('focus', refreshPermission))

function refreshPermission() {
  permission.value =
    'Notification' in window ? Notification.permission : 'unsupported'
}

async function loadCatalog() {
  try {
    const result = await $fetch<{ categories: NotificationCatalogCategory[] }>(
      '/api/notifications/catalog',
    )
    catalog.value = result.categories
    catalogError.value = ''
  } catch {
    catalogError.value = 'The event catalog is unavailable.'
  }
}

function selected(kind: string) {
  return preferences.value.popup_kinds.includes(kind)
}
function nativeSelected(kind: string) {
  return preferences.value.browser_kinds.includes(kind)
}
function channelSelected(channel: string) {
  return preferences.value.chat_channels.includes(channel)
}

function categoryState(category: NotificationCatalogCategory) {
  const selectedCount = category.kinds.filter((kind) =>
    selected(kind.id),
  ).length
  if (selectedCount === 0) return 'none'
  if (selectedCount === category.kinds.length) return 'all'
  return 'mixed'
}

function update(next: {
  popup_kinds: readonly string[]
  browser_kinds: readonly string[]
  chat_channels: readonly string[]
  chat_sound: boolean
}) {
  void save({
    popup_kinds: [...next.popup_kinds],
    browser_kinds: [...next.browser_kinds],
    chat_channels: [...next.chat_channels],
    chat_sound: next.chat_sound,
  })
}

function toggleCategory(category: NotificationCatalogCategory) {
  if (busy.value || !ready.value) return
  const ids = new Set(category.kinds.map((kind) => kind.id))
  const turnOn = categoryState(category) !== 'all'
  update({
    ...preferences.value,
    chat_channels:
      turnOn &&
      ids.has('chat.message_received') &&
      preferences.value.chat_channels.length === 0
        ? ['general', 'private', 'party', 'guild', 'union', 'global', 'unknown']
        : preferences.value.chat_channels,
    popup_kinds: turnOn
      ? [...new Set([...preferences.value.popup_kinds, ...ids])]
      : preferences.value.popup_kinds.filter((kind) => !ids.has(kind)),
    browser_kinds: turnOn
      ? [...preferences.value.browser_kinds]
      : preferences.value.browser_kinds.filter((kind) => !ids.has(kind)),
  })
}

function toggleKind(kind: string, enabled: boolean) {
  if (busy.value || !ready.value) return
  update({
    ...preferences.value,
    chat_channels:
      enabled &&
      kind === 'chat.message_received' &&
      preferences.value.chat_channels.length === 0
        ? ['general', 'private', 'party', 'guild', 'union', 'global', 'unknown']
        : preferences.value.chat_channels,
    popup_kinds: enabled
      ? [...new Set([...preferences.value.popup_kinds, kind])]
      : preferences.value.popup_kinds.filter((entry) => entry !== kind),
    browser_kinds: enabled
      ? [...preferences.value.browser_kinds]
      : preferences.value.browser_kinds.filter((entry) => entry !== kind),
  })
}

async function toggleNative(kind: string, enabled: boolean) {
  if (busy.value || !ready.value || !selected(kind)) return
  permissionError.value = ''
  if (enabled) {
    if (!('Notification' in window)) {
      permission.value = 'unsupported'
      permissionError.value =
        'This browser does not support native notifications.'
      return
    }
    try {
      permission.value = await Notification.requestPermission()
    } catch {
      permissionError.value = 'Browser permission could not be requested.'
      return
    }
    if (permission.value !== 'granted') {
      permissionError.value =
        permission.value === 'denied'
          ? 'Native notifications are blocked in browser settings.'
          : 'Allow browser notifications to enable this checkbox.'
      return
    }
  }
  update({
    ...preferences.value,
    browser_kinds: enabled
      ? [...new Set([...preferences.value.browser_kinds, kind])]
      : preferences.value.browser_kinds.filter((entry) => entry !== kind),
  })
}

function toggleChannel(channel: string, enabled: boolean) {
  if (busy.value || !ready.value) return
  const channels = enabled
    ? [...new Set([...preferences.value.chat_channels, channel])]
    : preferences.value.chat_channels.filter((entry) => entry !== channel)
  const noChat = channels.length === 0
  update({
    ...preferences.value,
    chat_channels: channels,
    popup_kinds: noChat
      ? preferences.value.popup_kinds.filter(
          (kind) => kind !== 'chat.message_received',
        )
      : [...preferences.value.popup_kinds],
    browser_kinds: noChat
      ? preferences.value.browser_kinds.filter(
          (kind) => kind !== 'chat.message_received',
        )
      : [...preferences.value.browser_kinds],
  })
}

function toggleSound(enabled: boolean) {
  if (busy.value || !ready.value) return
  update({ ...preferences.value, chat_sound: enabled })
}
</script>

<template>
  <div class="settings-page">
    <PageHeader
      title="Settings"
      icon="i-lucide-settings"
      description="Operator preferences and agent access management."
    />

    <nav class="settings-tabs" aria-label="Settings sections">
      <NuxtLink
        :to="{ path: '/settings' }"
        class="settings-tab"
        :class="{ active: settingsSection === 'notifications' }"
        :aria-current="settingsSection === 'notifications' ? 'page' : undefined"
      >
        <UIcon name="i-lucide-bell" /> Notifications
      </NuxtLink>
      <NuxtLink
        :to="{ path: '/settings', query: { section: 'agents' } }"
        class="settings-tab"
        :class="{ active: settingsSection === 'agents' }"
        :aria-current="settingsSection === 'agents' ? 'page' : undefined"
      >
        <UIcon name="i-lucide-bot" /> Agents
      </NuxtLink>
      <NuxtLink
        :to="{ path: '/settings', query: { section: 'command-review' } }"
        class="settings-tab"
        :class="{ active: settingsSection === 'command-review' }"
        :aria-current="
          settingsSection === 'command-review' ? 'page' : undefined
        "
      >
        <UIcon name="i-lucide-list-checks" /> Command Review
      </NuxtLink>
    </nav>

    <section
      v-if="settingsSection === 'notifications'"
      class="panel preferences-panel"
    >
      <div class="panel-heading">
        <div>
          <h2>Event notifications</h2>
          <p>
            Choose the events that appear at the bottom right while PhMon is
            open. Native alerts are optional for each kind when this tab is
            hidden.
          </p>
        </div>
        <span v-if="ready" class="preference-state">{{
          busy ? 'Saving…' : 'Saved on this instance'
        }}</span>
      </div>
      <div class="notification-options">
        <span
          >Browser permission:
          <strong :class="`permission-${permission}`">{{
            permission === 'granted'
              ? 'Allowed'
              : permission === 'denied'
                ? 'Blocked in browser settings'
                : permission === 'unsupported'
                  ? 'Not supported'
                  : 'Not requested'
          }}</strong></span
        >
        <label class="sound-option">
          <input
            type="checkbox"
            :checked="preferences.chat_sound"
            :disabled="!ready || busy"
            @change="toggleSound(($event.target as HTMLInputElement).checked)"
          />
          Play chat sound while the tab is hidden
        </label>
      </div>
      <p
        v-if="error || catalogError || permissionError"
        class="form-error"
        role="alert"
      >
        {{ error || catalogError || permissionError }}
      </p>
      <p v-if="!ready && !error" class="settings-note">
        Loading notification choices…
      </p>
      <div v-if="ready && catalog.length" class="notification-categories">
        <div
          v-for="category in catalog"
          :key="category.id"
          class="notification-category"
        >
          <div class="category-heading">
            <button
              type="button"
              class="category-expand"
              :aria-expanded="!!expanded[category.id]"
              :aria-controls="`kinds-${category.id}`"
              @click="expanded[category.id] = !expanded[category.id]"
            >
              <UIcon
                :name="
                  expanded[category.id]
                    ? 'i-lucide-chevron-down'
                    : 'i-lucide-chevron-right'
                "
                aria-hidden="true"
              />
              <strong>{{ category.label }}</strong>
              <span
                >{{
                  category.kinds.filter((kind) => selected(kind.id)).length
                }}/{{ category.kinds.length }}</span
              >
            </button>
            <label class="category-switch">
              <input
                type="checkbox"
                :checked="categoryState(category) === 'all'"
                :indeterminate.prop="categoryState(category) === 'mixed'"
                :aria-label="`Enable all ${category.label} notifications`"
                :disabled="busy"
                :data-mixed="categoryState(category) === 'mixed'"
                @change="toggleCategory(category)"
              />
              <span>{{
                categoryState(category) === 'all'
                  ? 'All on'
                  : categoryState(category) === 'mixed'
                    ? 'Some on'
                    : 'All off'
              }}</span>
            </label>
          </div>
          <div
            v-if="expanded[category.id]"
            :id="`kinds-${category.id}`"
            class="kind-list"
          >
            <div v-for="kind in category.kinds" :key="kind.id" class="kind-row">
              <div class="kind-name">
                <strong>{{ kind.label }}</strong
                ><small>{{ kind.id }}</small>
              </div>
              <label class="kind-check"
                ><input
                  type="checkbox"
                  :checked="selected(kind.id)"
                  :disabled="busy"
                  :aria-label="`Show ${kind.label} popups`"
                  @change="
                    toggleKind(
                      kind.id,
                      ($event.target as HTMLInputElement).checked,
                    )
                  "
                />
                Popup</label
              >
              <label class="kind-check"
                ><input
                  type="checkbox"
                  :checked="nativeSelected(kind.id)"
                  :disabled="
                    busy ||
                    !selected(kind.id) ||
                    (!nativeSelected(kind.id) &&
                      (permission === 'denied' || permission === 'unsupported'))
                  "
                  :aria-label="`Show native browser alerts for ${kind.label}`"
                  @change="
                    toggleNative(
                      kind.id,
                      ($event.target as HTMLInputElement).checked,
                    )
                  "
                />
                Browser</label
              >
            </div>
            <div v-if="category.id === 'chat'" class="channel-list">
              <p>Chat channels</p>
              <label
                v-for="channel in [
                  'general',
                  'private',
                  'party',
                  'guild',
                  'union',
                  'global',
                  'unknown',
                ]"
                :key="channel"
              >
                <input
                  type="checkbox"
                  :checked="channelSelected(channel)"
                  :disabled="busy"
                  @change="
                    toggleChannel(
                      channel,
                      ($event.target as HTMLInputElement).checked,
                    )
                  "
                />
                {{ channel }}
              </label>
            </div>
          </div>
        </div>
      </div>
      <p class="settings-note">
        New occurrences only. Existing history and events missed during a
        disconnect do not create popups. Trader / Hunter notifications hide your
        own characters.
      </p>
    </section>

    <AgentPanel v-else-if="settingsSection === 'agents'" />

    <section v-else class="panel preferences-panel review-actions-preference">
      <div class="panel-heading">
        <div>
          <h2>Command review</h2>
          <p>
            This preference is stored in a browser-local cookie on this device.
          </p>
        </div>
      </div>
      <div class="preference-row">
        <div>
          <strong id="review-actions-label"
            >Review actions before submitting</strong
          >
          <p>
            Show the selected characters, eligibility and planned commands
            before multi-character or disruptive single-character actions.
          </p>
        </div>
        <label class="switch-label"
          ><input
            v-model="reviewActions"
            type="checkbox"
            aria-labelledby="review-actions-label"
          /><span>{{ reviewActions ? 'Enabled' : 'Disabled' }}</span></label
        >
      </div>
    </section>
  </div>
</template>

<style scoped>
.settings-page {
  display: grid;
  gap: 14px;
}
.settings-tabs {
  display: flex;
  flex-wrap: wrap;
  gap: 6px;
  border-bottom: 1px solid var(--ph-border-soft);
  padding-bottom: 8px;
}
.settings-tab {
  display: inline-flex;
  min-height: 32px;
  align-items: center;
  gap: 6px;
  border: 1px solid transparent;
  border-radius: 4px;
  padding: 0 10px;
  color: var(--ph-muted);
  font-size: 12px;
  text-decoration: none;
}
.settings-tab:hover {
  border-color: var(--ph-border);
  color: var(--ph-text);
}
.settings-tab.active {
  border-color: #3d5270;
  background: #121d2b;
  color: var(--ph-primary);
}
.settings-tab :deep(svg) {
  width: 14px;
  height: 14px;
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
  white-space: nowrap;
}
.notification-options {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 12px 28px;
  padding: 13px 0;
  color: var(--ph-muted);
  font-size: 12px;
}
.notification-options strong {
  color: var(--ph-text);
}
.notification-options .permission-granted {
  color: var(--ph-green);
}
.notification-options .permission-denied {
  color: var(--ph-amber);
}
.sound-option,
.kind-check,
.category-switch,
.channel-list label {
  display: inline-flex;
  align-items: center;
  gap: 7px;
  cursor: pointer;
}
input[type='checkbox'] {
  width: 16px;
  height: 16px;
  accent-color: #7198c3;
  cursor: pointer;
}
input:disabled {
  opacity: 0.5;
  cursor: not-allowed;
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
.notification-categories {
  display: grid;
  gap: 7px;
}
.notification-category {
  border: 1px solid var(--ph-border-soft);
  border-radius: 5px;
  background: #101924a6;
}
.category-heading {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 12px;
  padding: 3px 12px 3px 4px;
}
.category-expand {
  display: inline-flex;
  align-items: center;
  gap: 8px;
  min-height: 37px;
  min-width: 0;
  flex: 1;
  border: 0;
  background: transparent;
  color: var(--ph-text);
  text-align: left;
  cursor: pointer;
}
.category-expand:hover {
  color: var(--ph-primary);
}
.category-expand span {
  color: var(--ph-muted);
  font-size: 11px;
}
.category-expand :deep(svg) {
  width: 14px;
  height: 14px;
}
.category-switch {
  color: var(--ph-muted);
  font-size: 11px;
  white-space: nowrap;
}
.category-switch input[data-mixed='true'] {
  outline: 1px solid var(--ph-primary);
  outline-offset: 2px;
}
.kind-list {
  border-top: 1px solid var(--ph-border-soft);
  padding: 2px 12px 9px 34px;
}
.kind-row {
  display: grid;
  grid-template-columns: minmax(0, 1fr) 84px 90px;
  align-items: center;
  gap: 10px;
  min-height: 43px;
  border-bottom: 1px solid var(--ph-border-soft);
}
.kind-name {
  display: grid;
  gap: 2px;
  min-width: 0;
}
.kind-name strong {
  color: var(--ph-text);
  font-size: 12px;
}
.kind-name small {
  color: var(--ph-muted);
  font-size: 10px;
  overflow-wrap: anywhere;
}
.kind-check {
  color: var(--ph-muted);
  font-size: 11px;
}
.channel-list {
  display: flex;
  flex-wrap: wrap;
  gap: 8px 16px;
  padding: 12px 0 3px;
  color: var(--ph-muted);
  font-size: 11px;
}
.channel-list p {
  flex-basis: 100%;
  margin: 0;
  color: var(--ph-text);
}
.preference-row {
  display: flex;
  justify-content: space-between;
  align-items: center;
  gap: 18px;
  padding: 14px 0;
}
.preference-row strong {
  color: var(--ph-text);
}
.switch-label {
  display: flex;
  align-items: center;
  gap: 8px;
  color: var(--ph-muted);
  font-size: 12px;
  white-space: nowrap;
}
button:focus-visible,
a:focus-visible,
input:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}
@media (max-width: 640px) {
  .panel-heading,
  .preference-row {
    align-items: flex-start;
    flex-direction: column;
  }
  .kind-list {
    padding-left: 12px;
  }
  .kind-row {
    grid-template-columns: minmax(0, 1fr) 67px 72px;
    gap: 5px;
  }
  .kind-check {
    font-size: 10px;
    gap: 3px;
  }
}
</style>
