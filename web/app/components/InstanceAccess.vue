<script setup lang="ts">
import QrcodeVue from 'qrcode.vue'
defineProps<{ collapsed: boolean }>()
const mobileAccessOpen = ref(false)
const copied = ref(false)
const copyFallbackNeeded = ref(false)
const mobileAccessTrigger = ref<HTMLButtonElement | null>(null)
const accessDialog = ref<HTMLElement | null>(null)
const accessCloseButton = ref<HTMLButtonElement | null>(null)
const runtimeConfig = useRuntimeConfig()
const requestURL = useRequestURL()
const configuredInstanceUrl = normalizeInstanceUrl(
  String(runtimeConfig.public.instanceUrl || ''),
)
const instanceUrl = ref(
  configuredInstanceUrl ||
    normalizeInstanceUrl(requestURL.origin) ||
    requestURL.origin,
)
const instanceUrlIsLoopback = computed(() => isLoopbackUrl(instanceUrl.value))

onMounted(() => {
  if (!configuredInstanceUrl) instanceUrl.value = window.location.origin
})
function normalizeInstanceUrl(value: string) {
  const candidate = value.trim()
  if (!candidate) return ''
  try {
    const url = new URL(candidate)
    if (url.username || url.password) return ''
    if (url.protocol !== 'http:' && url.protocol !== 'https:') return ''
    return url.origin
  } catch {
    return ''
  }
}

function isLoopbackUrl(value: string) {
  try {
    const hostname = new URL(value).hostname.toLowerCase()
    return (
      hostname === 'localhost' ||
      hostname.endsWith('.localhost') ||
      hostname === '127.0.0.1' ||
      hostname === '[::1]'
    )
  } catch {
    return true
  }
}

async function copyInstanceUrl() {
  if (!import.meta.client) return
  copyFallbackNeeded.value = false
  try {
    await navigator.clipboard.writeText(instanceUrl.value)
    copied.value = true
    window.setTimeout(() => {
      copied.value = false
    }, 1800)
  } catch {
    copied.value = false
    copyFallbackNeeded.value = true
  }
}

async function openMobileAccess() {
  copyFallbackNeeded.value = false
  mobileAccessOpen.value = true
  await nextTick()
  accessCloseButton.value?.focus()
}

async function closeMobileAccess() {
  mobileAccessOpen.value = false
  await nextTick()
  mobileAccessTrigger.value?.focus()
}

function handleAccessDialogKeydown(event: KeyboardEvent) {
  if (event.key === 'Escape') {
    event.preventDefault()
    void closeMobileAccess()
    return
  }
  if (event.key !== 'Tab' || !accessDialog.value) return

  const focusable = Array.from(
    accessDialog.value.querySelectorAll<HTMLElement>(
      'button:not([disabled]), [href], input:not([disabled]), select:not([disabled]), textarea:not([disabled]), [tabindex]:not([tabindex="-1"])',
    ),
  )
  if (focusable.length === 0) {
    event.preventDefault()
    return
  }
  const first = focusable[0]
  const last = focusable[focusable.length - 1]
  if (event.shiftKey && document.activeElement === first) {
    event.preventDefault()
    last?.focus()
  } else if (!event.shiftKey && document.activeElement === last) {
    event.preventDefault()
    first?.focus()
  }
}
</script>

<template>
  <div class="sidebar-footer">
    <button
      ref="mobileAccessTrigger"
      class="instance-button"
      type="button"
      :title="collapsed ? 'Mobile access' : undefined"
      @click="openMobileAccess"
    >
      <UIcon name="i-lucide-qr-code" />
      <span>Mobile access</span>
    </button>
    <div class="sidebar-qr" aria-label="PhMon instance QR code">
      <QrcodeVue :value="instanceUrl" :size="78" level="M" render-as="svg" />
    </div>
    <div class="instance-url" :title="instanceUrl">
      {{ instanceUrl }}
    </div>
    <button class="sidebar-copy" type="button" @click="copyInstanceUrl">
      {{ copied ? 'Copied' : 'Copy link' }}
    </button>
  </div>

  <Teleport to="#teleports">
    <div
      v-if="mobileAccessOpen"
      class="dialog-layer"
      role="presentation"
      @click.self="closeMobileAccess"
    >
      <section
        ref="accessDialog"
        class="access-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="mobile-access-title"
        @keydown="handleAccessDialogKeydown"
      >
        <div class="dialog-header">
          <div>
            <h2 id="mobile-access-title">Open PhMon on another device</h2>
            <p>
              The QR code contains only this instance URL. Agent credentials are
              never included.
            </p>
          </div>
          <button
            ref="accessCloseButton"
            class="icon-button"
            type="button"
            aria-label="Close"
            @click="closeMobileAccess"
          >
            <UIcon name="i-lucide-x" />
          </button>
        </div>
        <div v-if="instanceUrlIsLoopback" class="access-warning" role="status">
          This URL points back to the device opening PhMon and cannot be used
          from another device. Set NUXT_PUBLIC_INSTANCE_URL to a reachable HTTPS
          or LAN URL.
        </div>
        <div v-else class="qr-frame">
          <QrcodeVue
            id="phmon-instance-qr"
            :value="instanceUrl"
            :size="184"
            level="M"
            render-as="svg"
          />
        </div>
        <code class="dialog-url" tabindex="0">{{ instanceUrl }}</code>
        <p v-if="copyFallbackNeeded" class="copy-fallback" role="status">
          Clipboard access is unavailable here. Select the URL above and copy it
          manually.
        </p>
        <button
          class="compact-button dialog-copy"
          type="button"
          @click="copyInstanceUrl"
        >
          <UIcon :name="copied ? 'i-lucide-check' : 'i-lucide-copy'" />
          {{ copied ? 'Copied' : 'Copy instance URL' }}
        </button>
      </section>
    </div>
  </Teleport>
</template>
