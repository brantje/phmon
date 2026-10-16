<script setup lang="ts">
import {
  formatDate,
  toIsoAttribute,
  type DateFormatVariant,
} from '~/utils/dateFormat'

const props = withDefaults(
  defineProps<{
    value?: string | number | Date | null
    variant?: DateFormatVariant
    fallback?: string
  }>(),
  { value: null, variant: 'datetime', fallback: '—' },
)

// Locale/time zone are browser-only; render the stable ISO value until mounted
// so SSR and hydration agree.
const mounted = ref(false)
onMounted(() => {
  mounted.value = true
})

const iso = computed(() => toIsoAttribute(props.value))
const text = computed(() => {
  if (!mounted.value && iso.value)
    return iso.value.slice(0, 16).replace('T', ' ')
  return formatDate(props.value, props.variant, props.fallback)
})
</script>

<template>
  <time :datetime="iso" :title="iso">{{ text }}</time>
</template>
