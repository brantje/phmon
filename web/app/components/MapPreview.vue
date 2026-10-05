<script setup lang="ts">
import type { CharacterView } from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import { regionTileCenter, worldPositionToRaster } from '~/utils/mapCoordinates'
import { zoneNameText } from '~/utils/event-location'
import { characterMapMarkers } from '~/utils/mapCharacterMarkers'
import { characterPositionIsFresh } from '~/utils/characterPositionFreshness'
import { mapPreviewLocation } from '~/utils/mapNavigation'

const mapProfileCache = new Map<string, Promise<MapProfile>>()

const props = defineProps<{
  title: string
  server: string
  members: CharacterView[]
  characterCard?: boolean
  summaryItems?: Array<{ label: string; value: string }>
}>()

const profile = ref<MapProfile | null>(null)
const profileUnavailable = ref(false)
const positionFreshness = (character: CharacterView) =>
  characterPositionIsFresh(character)
const locatedMembers = computed(() =>
  props.members.filter(
    (character) =>
      character.server.toLowerCase() === props.server.toLowerCase() &&
      character.region != null &&
      character.x != null &&
      character.y != null,
  ),
)
const freshLocatedMembers = computed(() =>
  locatedMembers.value.filter(positionFreshness),
)
const centerCharacter = computed(
  () =>
    freshLocatedMembers.value
      .slice()
      .sort(
        (left, right) =>
          Date.parse(right.state_updated_at || '') -
          Date.parse(left.state_updated_at || ''),
      )[0],
)
const positionCharacter = computed(() => {
  if (centerCharacter.value) return centerCharacter.value
  return locatedMembers.value
    .slice()
    .sort(
      (left, right) =>
        Date.parse(right.state_updated_at || '') -
        Date.parse(left.state_updated_at || ''),
    )[0]
})
const centerExactPosition = computed(() => {
  const character = centerCharacter.value
  if (!profile.value || !character) return null
  return worldPositionToRaster(
    profile.value,
    'world',
    'world',
    character.region,
    character.x,
    character.y,
  )
})
const centerRasterPosition = computed(() => {
  if (centerExactPosition.value) return centerExactPosition.value
  if (!profile.value || !centerCharacter.value) return null
  return regionTileCenter(
    profile.value,
    'world',
    'world',
    centerCharacter.value.region,
  )
})
const sameRegionMembers = computed(() => {
  if (!centerCharacter.value) return []
  return freshLocatedMembers.value.filter(
    (character) => character.region === centerCharacter.value?.region,
  )
})
const previewMarkers = computed(() => {
  if (!profile.value) return []
  return characterMapMarkers(
    profile.value,
    'world',
    'world',
    sameRegionMembers.value,
  )
})
const initialTile = computed(() => {
  const preset = profile.value?.view_presets.find(
    (item) => item.area_id === 'world' && item.floor_id === 'world',
  )
  return preset ? { x: preset.tile_x, y: preset.tile_y } : { x: 168, y: 97 }
})
const sameRegionCount = computed(() => {
  return sameRegionMembers.value.length
})
const offMapCount = computed(() =>
  centerCharacter.value
    ? Math.max(0, props.members.length - sameRegionCount.value)
    : props.members.length,
)
const mapLink = computed(() =>
  mapPreviewLocation(props.server, centerCharacter.value),
)

async function loadMapProfile(server: string) {
  profile.value = null
  profileUnavailable.value = false
  if (!import.meta.client || !server) return
  const key = server.toLowerCase()
  let request = mapProfileCache.get(key)
  if (!request) {
    request = $fetch<MapProfile>(
      `/api/map/profile?server=${encodeURIComponent(server)}`,
    )
    mapProfileCache.set(key, request)
  }
  try {
    profile.value = await request
  } catch {
    if (mapProfileCache.get(key) === request) mapProfileCache.delete(key)
    profileUnavailable.value = true
  }
}

watch(
  () => props.server,
  (server) => void loadMapProfile(server),
)
onMounted(() => void loadMapProfile(props.server))
</script>

<template>
  <section
    class="map-preview-card"
    :class="{ 'map-preview-card--character': characterCard }"
    :aria-label="title"
  >
    <div v-if="!characterCard" class="map-preview-heading">
      <div>
        <strong>{{ title }}</strong>
        <span
          >{{ server }} · World ·
          {{
            centerCharacter
              ? sameRegionCount + ' in ' + zoneNameText(centerCharacter.zone)
              : 'No fresh positions'
          }}</span
        >
      </div>
      <NuxtLink class="compact-button" :to="mapLink">Open map</NuxtLink>
    </div>
    <NuxtLink
      v-if="characterCard"
      class="map-preview-open-corner"
      :to="mapLink"
      :aria-label="`Open ${title} in full map`"
      title="Open in map"
      ><UIcon name="i-lucide-arrow-up-right"
    /></NuxtLink>
    <div
      class="map-preview-content"
      :class="{ 'map-preview-content--group': summaryItems?.length }"
    >
      <ClientOnly>
        <MapCanvas
          v-if="profile?.tiles.status === 'available-for-inspection'"
          :profile="profile"
          compact
          :initial-position="centerRasterPosition"
          :initial-tile="initialTile"
          :markers="previewMarkers"
        />
        <template #fallback>
          <div class="map-preview-fallback" aria-hidden="true" />
        </template>
      </ClientOnly>
      <div v-if="!profile || profileUnavailable" class="map-preview-fallback">
        <UIcon name="i-lucide-map" />
        <span>{{
          profileUnavailable ? 'Map profile unavailable' : 'Loading map tiles'
        }}</span>
      </div>
      <div v-if="summaryItems?.length" class="map-preview-group-summary">
        <div class="map-preview-group-position">
          <strong v-if="centerCharacter">{{ centerCharacter.name }}</strong>
          <span v-if="centerCharacter"
            >{{ zoneNameText(centerCharacter.zone) }} ·
            {{ centerCharacter.x?.toFixed(1) }},
            {{ centerCharacter.y?.toFixed(1) }}</span
          >
          <span v-else>No fresh member position</span>
          <span v-if="offMapCount && centerCharacter"
            >{{ offMapCount }} outside this zone</span
          >
          <span v-else-if="offMapCount"
            >{{ offMapCount }} position{{
              offMapCount === 1 ? '' : 's'
            }}
            unavailable</span
          >
        </div>
        <div
          v-for="item in summaryItems"
          :key="item.label"
          class="map-preview-group-stat"
        >
          <span>{{ item.label }}</span
          ><strong>{{ item.value }}</strong>
        </div>
      </div>
      <div v-else class="map-preview-position">
        <span v-if="positionCharacter">
          World · {{ positionCharacter.name }} ·
          {{ zoneNameText(positionCharacter.zone) }} ·
          {{ positionCharacter.x?.toFixed(1) }},
          {{ positionCharacter.y?.toFixed(1) }}
        </span>
        <span v-else>Position not available</span>
        <span v-if="offMapCount && centerCharacter"
          >{{ offMapCount }} outside this zone</span
        >
        <span v-else-if="offMapCount"
          >{{ offMapCount }} position{{
            offMapCount === 1 ? '' : 's'
          }}
          unavailable for this scope</span
        >
        <span
          v-if="positionCharacter && !positionFreshness(positionCharacter)"
          class="map-preview-stale"
        >
          Position is stale
        </span>
      </div>
    </div>
    <p class="map-preview-note">
      {{
        characterCard
          ? centerExactPosition
            ? 'Centered from validated position data.'
            : centerRasterPosition
              ? 'Centered on the matched region tile; exact pixel unavailable.'
              : 'Position placement unavailable.'
          : previewMarkers.some((marker) => marker.placement === 'region-tile')
            ? 'Tile-only markers are spread for visibility; exact pixels are unavailable.'
            : previewMarkers.length
              ? 'Fresh members use the validated outdoor position transform.'
              : 'No fresh member position has a supported map transform.'
      }}
    </p>
  </section>
</template>

<style scoped>
.map-preview-card {
  position: relative;
  min-width: 0;
  padding: 10px;
  border: 1px solid var(--panel-border, #273449);
  border-radius: 5px;
  background: rgba(8, 13, 21, 0.72);
}

.map-preview-heading,
.map-preview-heading > div {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.map-preview-heading strong {
  color: var(--text-primary, #eaf1ff);
  font-size: 13px;
}

.map-preview-heading span,
.map-preview-position,
.map-preview-note {
  color: var(--text-secondary, #98a7bd);
  font-size: 11px;
}

.map-preview-open-corner {
  position: absolute;
  z-index: 500;
  top: 7px;
  right: 7px;
  display: grid;
  width: 24px;
  height: 24px;
  place-items: center;
  border: 1px solid rgba(214, 229, 255, 0.5);
  border-radius: 4px;
  background: rgba(7, 12, 20, 0.84);
  color: #eaf1ff;
}

.map-preview-card--character {
  padding: 4px;
  border-color: #263346;
}

.map-preview-card--character .map-preview-content {
  grid-template-columns: minmax(0, 1fr);
  gap: 0;
  margin-top: 0;
}

.map-preview-card--character .map-preview-position {
  flex-direction: row;
  flex-wrap: wrap;
  justify-content: flex-start;
  gap: 2px 8px;
  padding: 4px 2px 0;
  font-size: 10px;
}

.map-preview-card--character .map-preview-note {
  margin-top: 3px;
  font-size: 9px;
}

.map-preview-content {
  position: relative;
  display: grid;
  grid-template-columns: minmax(120px, 1fr) minmax(140px, 0.8fr);
  gap: 8px;
  margin-top: 8px;
  overflow: hidden;
  border-radius: 4px;
  background: #101923;
}

.map-preview-content--group {
  grid-template-columns: minmax(180px, 1.05fr) minmax(180px, 0.95fr);
  align-items: stretch;
}

.map-preview-content--group :deep(.map-canvas) {
  min-height: 150px;
}

.map-preview-group-summary {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 4px;
  padding: 5px;
}

.map-preview-group-position,
.map-preview-group-stat {
  display: flex;
  min-width: 0;
  flex-direction: column;
  justify-content: center;
  gap: 3px;
  padding: 5px;
  border: 1px solid rgba(80, 99, 124, 0.45);
  border-radius: 4px;
  color: #aab8ca;
  font-size: 10px;
}

.map-preview-group-position strong,
.map-preview-group-stat strong {
  color: #eaf1ff;
  font-size: 12px;
}

.map-preview-group-position span,
.map-preview-group-stat span {
  overflow-wrap: anywhere;
}

.map-preview-content :deep(.map-canvas) {
  min-height: 112px;
}

.map-preview-fallback {
  display: flex;
  min-height: 112px;
  align-items: center;
  justify-content: center;
  gap: 6px;
  background-color: #101923;
  background-image:
    linear-gradient(rgba(130, 157, 187, 0.11) 1px, transparent 1px),
    linear-gradient(90deg, rgba(130, 157, 187, 0.11) 1px, transparent 1px);
  background-size: 24px 24px;
  color: #98a7bd;
  font-size: 11px;
}

.map-preview-position {
  display: flex;
  flex-direction: column;
  justify-content: center;
  gap: 6px;
  padding: 7px;
  line-height: 1.4;
}

.map-preview-stale {
  color: #e0bd73;
}

.map-preview-note {
  margin: 7px 0 0;
  line-height: 1.35;
}

@media (max-width: 640px) {
  .map-preview-content--group {
    grid-template-columns: minmax(0, 1fr);
  }
  .map-preview-content {
    grid-template-columns: minmax(100px, 0.8fr) minmax(120px, 1fr);
  }
}
</style>
