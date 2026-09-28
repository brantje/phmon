<script setup lang="ts">
definePageMeta({
  validate: (route) =>
    typeof route.params.id === 'string' &&
    /^[0-9a-f-]{36}$/i.test(route.params.id),
})
const route = useRoute()
const { serverScope } = useServerScope()
const {
  characterDetail: detailCharacter,
  connectionState: liveConnectionState,
  liveStale,
  setCharacterDetail,
} = useLiveData()
let stopDetailWatch: (() => void) | undefined
onMounted(() => {
  stopDetailWatch = watch(
    [() => String(route.params.id), serverScope],
    ([id, server]) =>
      setCharacterDetail(String(id), server === 'all' ? undefined : server),
    { immediate: true },
  )
})
onBeforeUnmount(() => {
  stopDetailWatch?.()
  setCharacterDetail('')
})
const detailLoading = computed(
  () => liveConnectionState.value !== 'current' && !liveStale.value,
)
</script>

<template>
  <section class="character-detail-view">
    <PageHeader
      :title="detailCharacter?.name || 'Character detail'"
      icon="i-lucide-user-round"
      :description="
        (detailCharacter?.server || 'Loading identity') +
        ' · stable character record'
      "
    >
      <NuxtLink class="compact-button" to="/">Back to overview</NuxtLink>
    </PageHeader>
    <div v-if="liveStale" class="status-banner warning" role="status">
      <UIcon name="i-lucide-triangle-alert" />
      Live character data is stale. PhMon is retrying the WebSocket connection;
      HTTP fallback is disabled.
    </div>
    <CharacterCard
      v-if="detailCharacter"
      :character="detailCharacter"
      :stale="liveStale"
    />
    <div v-else class="panel empty-state">
      <strong>{{
        detailLoading
          ? 'Loading live character'
          : liveStale
            ? 'Last character snapshot unavailable'
            : 'Character unavailable'
      }}</strong>
      <p>
        {{
          detailLoading
            ? 'Waiting for the initial WebSocket detail snapshot.'
            : liveStale
              ? 'The WebSocket will retry and resynchronize without an HTTP fallback.'
              : 'The character ID is not present in the selected server scope.'
        }}
      </p>
    </div>
  </section>
</template>
