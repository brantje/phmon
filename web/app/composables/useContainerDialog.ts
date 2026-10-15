export type ContainerDialogTarget =
  | {
      kind: 'character'
      server: string
      characterID: string
      characterName: string
      container: 'inventory' | 'storage'
    }
  | { kind: 'guild'; server: string; guild: string }

type GuildResource = {
  resource_key: string
  availability: string
  payload: Record<string, unknown>
  observed_at?: string
  checked_at?: string
  observer_character_id?: string
  observer_name?: string
}
type GuildStorageSnapshot = {
  server: string
  guild: string
  items: GuildResource[]
}
type ResourceView = {
  availability: string
  payload: Record<string, unknown>
  observed_at?: string
  checked_at?: string
  observer_name?: string
}

export type ContainerDialogStatus = 'loading' | 'current' | 'unavailable'

export function containerDialogTargetKey(target: ContainerDialogTarget) {
  return JSON.stringify(
    target.kind === 'guild'
      ? ['guild', target.server.toLocaleLowerCase(), target.guild]
      : ['character', target.characterID, target.container],
  )
}

// Reads one complete container for the dialog. Character containers use a
// dedicated live resources subscription; guild storage is fetched from the
// scoped API on each opening. Late responses for older targets are discarded.
export function useContainerDialog(target: Ref<ContainerDialogTarget | null>) {
  const {
    resourceFeeds,
    resourceFeedStatus,
    setResourceFeed,
    clearResourceFeed,
    refreshLiveData,
    liveStale,
    connectionState,
  } = useLiveData()
  const subscriptionID = `resource-dialog-${useId()
    .replace(/[^a-zA-Z0-9_-]/g, '')
    .slice(-24)}`
  const guildResource = shallowRef<GuildResource | null>(null)
  const guildStatus = ref<ContainerDialogStatus>('loading')
  let guildController: AbortController | undefined
  let guildGeneration = 0

  async function loadGuild(server: string, guild: string) {
    const current = ++guildGeneration
    guildController?.abort()
    guildController = new AbortController()
    guildStatus.value = 'loading'
    try {
      const result = await $fetch<GuildStorageSnapshot>('/api/guild-storage', {
        query: { server, guild },
        signal: guildController.signal,
        retry: 0,
        timeout: 10_000,
      })
      if (current !== guildGeneration) return
      guildResource.value =
        result.items.find((item) => item.resource_key === 'guild_storage') ||
        null
      guildStatus.value = 'current'
    } catch {
      if (current === guildGeneration) guildStatus.value = 'unavailable'
    }
  }

  function release() {
    guildGeneration++
    guildController?.abort()
    clearResourceFeed(subscriptionID)
  }

  watch(
    target,
    (next, previous) => {
      if (
        next &&
        previous &&
        containerDialogTargetKey(next) === containerDialogTargetKey(previous)
      )
        return
      release()
      guildResource.value = null
      if (!next) return
      if (next.kind === 'guild') void loadGuild(next.server, next.guild)
      else setResourceFeed(subscriptionID, next.characterID, [next.container])
    },
    { immediate: true },
  )
  onBeforeUnmount(release)

  const resource = computed<ResourceView | null>(() => {
    const current = target.value
    if (!current) return null
    if (current.kind === 'guild') return guildResource.value
    const feed = resourceFeeds.value[subscriptionID]
    if (!feed || feed.character_id !== current.characterID) return null
    return feed.resources[current.container] || null
  })
  const status = computed<ContainerDialogStatus>(() => {
    if (target.value?.kind === 'guild') return guildStatus.value
    return resourceFeedStatus.value[subscriptionID] || 'loading'
  })
  const payload = computed(() => resource.value?.payload || {})
  const observation = computed<
    Record<string, unknown> & { availability: string }
  >(() => ({
    ...payload.value,
    availability: resource.value?.availability || 'unavailable',
    observed_at: resource.value?.observed_at,
    checked_at: resource.value?.checked_at,
  }))
  const gold = computed(() => {
    const value = payload.value.gold
    return typeof value === 'number' && Number.isFinite(value) && value >= 0
      ? value
      : null
  })
  const occupied = computed(() =>
    Array.isArray(payload.value.slots)
      ? payload.value.slots.filter(Boolean).length
      : null,
  )
  const capacity = computed(() => {
    const value = payload.value.capacity
    return typeof value === 'number' && Number.isFinite(value) && value >= 0
      ? Math.floor(value)
      : null
  })
  const stale = computed(
    () =>
      target.value?.kind === 'character' &&
      (liveStale.value || connectionState.value === 'stale'),
  )

  function retry() {
    const current = target.value
    if (!current) return
    if (current.kind === 'guild') void loadGuild(current.server, current.guild)
    else refreshLiveData([subscriptionID])
  }

  return {
    resource,
    status,
    observation,
    gold,
    occupied,
    capacity,
    stale,
    retry,
  }
}
