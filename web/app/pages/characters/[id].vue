<script setup lang="ts">
definePageMeta({
  validate: (route) =>
    typeof route.params.id === 'string' &&
    /^[0-9a-f-]{36}$/i.test(route.params.id),
})
const route = useRoute()
const {
  characterDetail: detailCharacter,
  connectionState: liveConnectionState,
  liveStale,
  setCharacterDetail,
} = useLiveData()
let stopDetailWatch: (() => void) | undefined
onMounted(() => {
  stopDetailWatch = watch(
    () => String(route.params.id),
    (id) => setCharacterDetail(id),
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
    <div v-if="detailCharacter" class="detail-grid">
      <article class="panel detail-identity">
        <div class="panel-header compact">
          <div>
            <h2>Current status</h2>
            <p>{{ detailCharacter.guild || 'Guild unknown' }}</p>
          </div>
          <span
            class="status-chip"
            :class="detailCharacter.online ? 'online' : 'offline'"
            ><span />{{ detailCharacter.online ? 'Online' : 'Offline' }}</span
          >
        </div>
        <dl class="operation-list">
          <div>
            <dt>Serving agent</dt>
            <dd>{{ detailCharacter.agent_id || 'None' }}</dd>
          </div>
          <div>
            <dt>Level</dt>
            <dd>{{ detailCharacter.level ?? '—' }}</dd>
          </div>
          <div>
            <dt>HP / MP</dt>
            <dd>
              {{ formatHealthMana(detailCharacter) }}
            </dd>
          </div>
          <div>
            <dt>XP / SP</dt>
            <dd>
              {{ formatProgress(detailCharacter) }}
            </dd>
          </div>
          <div>
            <dt>Gold</dt>
            <dd>{{ detailCharacter.gold?.toLocaleString() ?? '—' }}</dd>
          </div>
          <div>
            <dt>Location</dt>
            <dd>
              {{ detailCharacter.zone || 'Unknown zone' }} ·
              {{ detailCharacter.x ?? '—' }}, {{ detailCharacter.y ?? '—' }},
              {{ detailCharacter.z ?? '—' }} (region
              {{ detailCharacter.region ?? '—' }})
            </dd>
          </div>
          <div>
            <dt>Training state</dt>
            <dd>
              {{
                detailCharacter.botting == null
                  ? 'Not reported by documented phBot API'
                  : detailCharacter.botting
                    ? 'Training'
                    : 'Idle'
              }}
            </dd>
          </div>
          <div>
            <dt>Session started</dt>
            <dd>
              {{ formatTimestamp(detailCharacter.session_started_at) }}
            </dd>
          </div>
          <div>
            <dt>Last activity</dt>
            <dd>
              {{ formatTimestamp(detailCharacter.last_activity_at) }}
            </dd>
          </div>
        </dl>
      </article>
      <article class="panel detail-future">
        <div class="panel-header compact">
          <div>
            <h2>Character tools</h2>
            <p>This panel is scoped to the current live session.</p>
          </div>
        </div>
        <div class="detail-links">
          <span>Inventory / equipment · Slice 4</span
          ><span>Pets and party · Slice 4</span
          ><span>Map position · Slice 7</span
          ><span>Actions / command history · below</span>
        </div>
      </article>
    </div>
    <RemoteCommandActions v-if="detailCharacter" :character="detailCharacter" />
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
              : 'The character ID is not present in the current live snapshot.'
        }}
      </p>
    </div>
  </section>
</template>
