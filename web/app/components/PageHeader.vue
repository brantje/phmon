<script setup lang="ts">
defineProps<{ title: string; icon: string; description: string }>()
</script>

<template>
  <header class="page-header">
    <div class="page-icon"><UIcon :name="icon" /></div>
    <div>
      <h1>{{ title }}</h1>
      <p>{{ description }}</p>
    </div>
    <slot />
  </header>
</template>
