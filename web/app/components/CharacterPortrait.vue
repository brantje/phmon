<script setup lang="ts">
const props = withDefaults(
  defineProps<{
    name: string
    portraitUrl?: string
    size?: 'small' | 'card' | 'large'
    to?: string
  }>(),
  { portraitUrl: '', size: 'small', to: undefined },
)
const failed = ref(false)
const source = computed(() => {
  const path = props.portraitUrl
  return path.startsWith('/game-assets/interface/character/') &&
    !path.includes('..') &&
    !path.includes('\\') &&
    !path.includes('?') &&
    !path.includes('#')
    ? path
    : ''
})
watch(source, () => {
  failed.value = false
})
</script>

<template>
  <NuxtLink
    v-if="to"
    :to="to"
    class="character-portrait"
    :class="`portrait-${size}`"
    :aria-label="`Open ${name} details`"
    aria-hidden="false"
  >
    <!-- prettier-ignore -->
    <img v-if="source && !failed" :src="source" alt="" @error="failed = true">
    <span v-else aria-hidden="true">{{
      name.slice(0, 1).toUpperCase() || '?'
    }}</span>
  </NuxtLink>
  <span
    v-else
    class="character-portrait"
    :class="`portrait-${size}`"
    aria-hidden="true"
  >
    <!-- prettier-ignore -->
    <img v-if="source && !failed" :src="source" alt="" @error="failed = true">
    <span v-else>{{ name.slice(0, 1).toUpperCase() || '?' }}</span>
  </span>
</template>
