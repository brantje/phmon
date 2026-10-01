<script setup lang="ts">
import type { MapActionNotification } from '~/utils/mapActionNotifications'

defineProps<{ notification: MapActionNotification | null }>()
</script>

<template>
  <div class="map-action-toast-region" role="status" aria-atomic="true">
    <div
      v-if="notification"
      class="map-action-toast panel"
      :class="notification.tone"
    >
      <UIcon
        :name="
          notification.tone === 'success'
            ? 'i-lucide-circle-check'
            : 'i-lucide-triangle-alert'
        "
        aria-hidden="true"
      />
      <span>{{ notification.message }}</span>
    </div>
  </div>
</template>

<style scoped>
.map-action-toast-region {
  position: absolute;
  z-index: 1200;
  right: 12px;
  bottom: 12px;
  left: 12px;
  display: flex;
  justify-content: center;
  pointer-events: none;
}
.map-action-toast {
  display: flex;
  align-items: center;
  gap: 10px;
  max-width: 100%;
  padding: 10px 14px;
  border-radius: 7px;
  color: var(--ph-text);
  font-size: 13px;
  overflow-wrap: anywhere;
  box-shadow: 0 8px 24px #0005;
}
.map-action-toast .iconify {
  flex: 0 0 auto;
  color: #6f9bce;
}
.map-action-toast.warning .iconify {
  color: var(--ph-primary);
}
</style>
