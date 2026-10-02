<script setup lang="ts">
import type {
  ActivityEvent,
  CharacterView,
  ControlsSnapshot,
  MapMonster,
  MapNpc,
  MapOtherPlayer,
  MapPartyMember,
  MapSnapshot,
} from '~~/shared/types/live'
import type { MapAreaProfile, MapProfile } from '~~/shared/types/map'
import type { HeatmapLayerID } from '~~/shared/types/mapAnalytics'
import {
  caveFloorForPosition,
  tileCatalogForFloor,
  rasterPositionToGame,
  regionTileCenter,
  worldPositionToRaster,
  type RasterPosition,
} from '~/utils/mapCoordinates'
import {
  characterHasDisplayableMapPosition,
  characterMapMarkers,
  displayableMapCharacters,
} from '~/utils/mapCharacterMarkers'
import { npcDisplayLabel, npcMapMarkers } from '~/utils/mapNpcMarkers'
import { playerMapMarkers } from '~/utils/mapPlayerMarkers'
import { partyMapMarkers } from '~/utils/mapPartyMarkers'
import {
  DEFAULT_SHOW_NEARBY_MONSTER_NAMES,
  dedupeCurrentMonsters,
  monsterDisplayName,
  monsterTypePresentation,
} from '~/utils/mapMarkerPresentation'
import {
  mapFeedRegion,
  mapEventLocation,
  mapProfileRequestIsCurrent,
} from '~/utils/mapNavigation'
import { relativeMapEventWindow } from '~/utils/mapTimeRange'
import { zoneNameText } from '~/utils/event-location'
import { uniqueRegionOptionLabels } from '~/utils/mapRegionLabels'
import {
  heatmapLayerLabel,
  heatmapResultToLayer,
  historicalHeatmapWindow,
} from '~/utils/mapHeatmap'
import {
  applyMapActionTargetGroup,
  clearMapActionTargets,
  mapActionTargetGroupState,
  mapActionTargetScopeKey,
  reconcileMapActionTargets,
  selectAllMapActionTargets,
  toggleMapActionTarget,
} from '~/utils/mapActionTargets'
import {
  mapNavigationRouteOverlays,
  mapNavigationStatusLabel,
} from '~/utils/mapNavigationRoutes'
import {
  mapNavigationTrayRows,
  navigationTrayProgressSummary,
} from '~/utils/mapNavigationTray'
import { submitNavigationStop } from '~/utils/mapNavigationStop'
import { traceActivitySummary } from '~/utils/mapTraceActivity'
import { useMapNavigationAction } from '~/composables/useMapNavigationAction'
import { useMapTeleportAction } from '~/composables/useMapTeleportAction'
import { useMapTrainingEditor } from '~/composables/useMapTrainingEditor'
import type { MapActionNotification } from '~/utils/mapActionNotifications'
import {
  TRAINING_RADIUS_MAX,
  TRAINING_RADIUS_MIN,
  trainingAreaOverlays,
} from '~/utils/mapTrainingAreas'

definePageMeta({ layout: 'map' })

const reviewActions = useReviewActionsPreference()
const {
  mapFeeds,
  mapFeedCurrent,
  fleetCharacters,
  groups,
  connectionState,
  freshnessNow,
  setMapFeed,
  clearMapFeed,
  liveStale,
  setCommandFanOutTargets,
  clearCommandFanOutOwner,
  commandFanOutFeeds,
  refreshLiveData,
} = useLiveData()
const MAP_ACTIVITY_OWNER = 'map-character-activity'
const {
  enabled: historicalLayers,
  results: heatmapResults,
  loading: heatmapLoading,
  errors: heatmapErrors,
  facets: heatmapFacets,
  facetsLoading: heatmapFacetsLoading,
  facetsError: heatmapFacetsError,
  activeLayers: activeHistoricalLayers,
  refresh: refreshHistoricalHeatmaps,
  reset: resetHistoricalHeatmap,
  clear: clearHistoricalHeatmaps,
} = useMapHeatmaps()
const { serverScope } = useServerScope()
const route = useRoute()
const router = useRouter()
const subscriptionID = 'map-page'
const regionOptions = computed(() => {
  const set = new Set<number>()
  const floor = mapProfile.value?.areas
    .find((area) => area.id === areaID.value)
    ?.floors.find((item) => item.id === floorID.value)
  for (const region of floor?.region_ids || []) set.add(region)
  for (const character of mapSnapshot.value?.characters || []) {
    if (character.region != null) set.add(character.region)
  }
  for (const result of Object.values(heatmapResults)) {
    for (const point of result?.points || []) set.add(point.region)
  }
  return [...set].sort((left, right) => left - right)
})
const server = computed(() => {
  if (typeof route.query.server === 'string') return route.query.server
  if (serverScope.value !== 'all') return serverScope.value
  return fleetCharacters.value[0]?.server || 'greatest'
})
const serverOptions = computed(() =>
  [
    ...new Set([
      ...fleetCharacters.value.map((character) => character.server),
      server.value,
    ]),
  ]
    .filter(Boolean)
    .sort((left, right) => left.localeCompare(right)),
)
const areaID = ref(
  typeof route.query.area === 'string' ? route.query.area : 'world',
)
const floorID = ref(
  typeof route.query.floor === 'string' ? route.query.floor : 'world',
)
const linkedEventID = computed(() =>
  typeof route.query.event_id === 'string' ? route.query.event_id : '',
)
const regionID = ref(
  typeof route.query.region === 'string' ? Number(route.query.region) || 0 : 0,
)
const selectedCharacterID = ref(
  typeof route.query.character_id === 'string' ? route.query.character_id : '',
)
const actionTargetIDs = ref(new Set<string>())
const dismissedNavigationRows = ref(new Set<string>())
const navigationStopPending = ref(new Set<string>())
const navigationTrayOpen = ref(true)
const navigationTrayCompact = ref(false)
const mapWorkspaceElement = ref<HTMLElement | null>(null)
let workspaceObserver: ResizeObserver | undefined
// User-controlled disclosure: live action updates must not change this state.
const actionFeedbackOpen = ref(false)
const selectedNavigationRouteID = ref('')
const inspectorOpen = ref(true)
const legendOpen = ref(false)
const linkedNoticeDismissed = ref('')
const mapCanvas = ref<{
  zoomIn(): void
  zoomOut(): void
  focusAt(point: RasterPosition): void
} | null>(null)
const copyNotice = ref('')
const actionTargetScopeKey = computed(() =>
  mapActionTargetScopeKey({
    server: server.value,
    area: areaID.value,
    floor: floorID.value,
    region: regionID.value,
  }),
)
const remoteActionScopeKey = computed(() =>
  [
    actionTargetScopeKey.value,
    mapProfile.value?.dataset_id || 'no-dataset',
    mapProfile.value?.dataset_version || 'no-version',
  ].join('\u0000'),
)
function mapControlScopeKeyForCharacter(
  character: CharacterView,
  scopeKey: string,
) {
  return character.server.toLocaleLowerCase() ===
    server.value.toLocaleLowerCase()
    ? scopeKey
    : `server:${character.server.toLocaleLowerCase()}`
}
function mapControlCurrentScopeKey(characterID: string) {
  const character = fleetCharacters.value.find(
    (item) => item.character_id === characterID,
  )
  return character
    ? mapControlScopeKeyForCharacter(character, remoteActionScopeKey.value)
    : 'unavailable'
}
function mapControlCurrentCharacter(characterID: string) {
  return fleetCharacters.value.find(
    (character) => character.character_id === characterID,
  )
}
const expectedMapFeedScope = computed(() => ({
  server: server.value,
  area: areaID.value,
  floor: floorID.value,
  region:
    areaID.value === 'world'
      ? mapFeedRegion(regionID.value, selectedCharacterID.value) || 0
      : 0,
}))
const selectedDestinationID = ref('')
const dateRange = ref('24h')
const eventWindowNow = ref(Date.now())
const heatmapRange = ref('24h')
const heatmapCustomFrom = ref('')
const heatmapCustomTo = ref('')
const historicalHeatmapsOpen = ref(false)
const sideTab = ref<'characters' | 'activity' | 'layers'>('characters')
const sideTabs = ['characters', 'activity', 'layers'] as const
function moveSideTab(event: KeyboardEvent) {
  const index = sideTabs.indexOf(sideTab.value)
  const next =
    event.key === 'ArrowRight'
      ? (index + 1) % 3
      : event.key === 'ArrowLeft'
        ? (index + 2) % 3
        : event.key === 'Home'
          ? 0
          : event.key === 'End'
            ? 2
            : -1
  if (next < 0) return
  event.preventDefault()
  sideTab.value = sideTabs[next]!
  nextTick(() => document.getElementById(`map-tab-${sideTab.value}`)?.focus())
}
const analyticsCharacterID = ref('')
const analyticsMobType = ref('')
const resetLayer = ref<HeatmapLayerID>('deaths')
const resetOpen = ref(false)
const resetWindow = ref<{ from: string; to: string } | null>(null)
const resetBusy = ref(false)
const resetError = ref('')
const confirmBroadReset = ref(false)
const layerCharacters = ref(true)
const layerParty = ref(true)
const layerPlayers = ref(true)
const layerNPCs = ref(true)
const layerTraining = ref(true)
const layerMonsters = ref(true)
const showNearbyMonsterNames = ref(DEFAULT_SHOW_NEARBY_MONSTER_NAMES)
const layerDeaths = ref(false)
const layerDrops = ref(false)
const mapProfile = ref<MapProfile | null>(null)
const profileLoading = ref(false)
const profileError = ref('')
const mapView = ref({ tileX: 168, tileY: 97, zoomPercent: 100 })
const jumpSequence = ref(0)
const snapshot = computed(() => mapFeeds.value[subscriptionID])
const mapSnapshot = computed(() => snapshot.value as MapSnapshot | undefined)
const mapSnapshotInFeedScope = computed(
  () =>
    Boolean(mapSnapshot.value) &&
    mapSnapshotMatchesScope(mapSnapshot.value!, expectedMapFeedScope.value),
)
const heatmapWindow = computed(() =>
  historicalHeatmapWindow(
    heatmapRange.value,
    eventWindowNow.value,
    heatmapCustomFrom.value,
    heatmapCustomTo.value,
  ),
)
const navigationAction = useMapNavigationAction({
  scope: () =>
    mapProfile.value
      ? {
          server: server.value,
          areaID: areaID.value,
          floorID: floorID.value,
          region: regionID.value,
          datasetID: mapProfile.value.dataset_id,
          datasetVersion: mapProfile.value.dataset_version,
        }
      : null,
  profile: () => mapProfile.value,
  selectedTargetIDs: () => [...actionTargetIDs.value],
  characters: () => [...fleetCharacters.value],
  mapSnapshot: () => mapSnapshot.value,
  mapFeedCurrent: () =>
    streamCurrent.value && mapSnapshotInFeedScope.value && !liveStale.value,
  reviewActions: () => reviewActions.value,
  now: () => freshnessNow.value,
})
const teleportAction = useMapTeleportAction({
  server: () => server.value,
  selectedTargetIDs: () => [...actionTargetIDs.value],
  characters: () => [...fleetCharacters.value],
  mapFeedCurrent: () =>
    streamCurrent.value && mapSnapshotInFeedScope.value && !liveStale.value,
  reviewActions: () => reviewActions.value,
})
const actionNotification = ref<MapActionNotification | null>(null)
let actionNotificationTimer: ReturnType<typeof setTimeout> | undefined
function showMapActionNotification(notification: MapActionNotification) {
  if (actionNotificationTimer) clearTimeout(actionNotificationTimer)
  actionNotification.value = notification
  actionNotificationTimer = setTimeout(() => {
    actionNotification.value = null
    actionNotificationTimer = undefined
  }, 5_000)
}
useMapActionNotifications(
  () => [
    ...navigationAction.operations.value,
    ...navigationAction.trainingOperations.value,
    ...teleportAction.operations.value,
  ],
  showMapActionNotification,
)
const trainingAreas = computed(() =>
  mapSnapshotInFeedScope.value
    ? mapSnapshot.value?.training_areas?.areas || []
    : [],
)
const trainingEditor = useMapTrainingEditor({
  scope: () =>
    mapProfile.value
      ? { server: server.value, areaID: areaID.value, floorID: floorID.value }
      : null,
  profile: () => mapProfile.value,
  areas: () => trainingAreas.value,
  characters: () => [...fleetCharacters.value],
  reviewActions: () => reviewActions.value,
})
const renderedTrainingAreas = computed(() =>
  layerTraining.value && mapProfile.value
    ? trainingAreaOverlays({
        profile: mapProfile.value,
        areaID: areaID.value,
        floorID: floorID.value,
        areas: trainingAreas.value,
        regionFilter: regionID.value,
        selectedID: trainingEditor.selectedID.value,
        draft: trainingEditor.draft.value,
      })
    : [],
)
function selectTrainingArea(characterID: string) {
  inspectorOpen.value = true
  trainingEditor.select(
    trainingEditor.selectedID.value === characterID ? '' : characterID,
  )
}
function acceptTrainingDraft(characterID: string) {
  if (trainingEditor.selectedID.value !== characterID) return
  void trainingEditor.apply()
}
function discardTrainingDraft(characterID: string) {
  if (trainingEditor.selectedID.value !== characterID) return
  trainingEditor.reset()
}
function trainingAreaSummary(characterID: string) {
  const area = trainingAreas.value.find(
    (item) => item.character_id === characterID,
  )
  if (!area) return ''
  return `${area.zone || zoneNameForRegion(area.region)} · radius ${area.radius}`
}
const trainingEmptyCopy = computed(() => {
  if (!layerTraining.value) return 'The training area layer is hidden.'
  if (mapSnapshot.value && !mapSnapshot.value.training_areas)
    return 'This backend does not report training areas.'
  return 'No active training areas are reported for this floor.'
})
const trainingOutcomeLabel = (outcome: string) =>
  ({
    completed: 'applied',
    failed: 'failed',
    expired: 'expired',
    unknown: 'result unknown',
    skipped: 'skipped',
    rejected: 'rejected',
    uncertain: 'outcome unknown',
    not_sent: 'not sent',
  })[outcome] || outcome
watch(layerTraining, (visible) => {
  if (!visible) trainingEditor.clear()
})
function selectMapPoint(point: RasterPosition) {
  if (trainingEditor.moveArmed.value) {
    trainingEditor.moveCenter(trainingEditor.selectedID.value, point)
    return
  }
  inspectorOpen.value = false
  if (trainingEditor.selectedID.value) trainingEditor.select('')
}
const historicalCharacters = computed(() =>
  fleetCharacters.value
    .filter(
      (character) =>
        character.server.toLowerCase() === server.value.toLowerCase(),
    )
    .sort((left, right) => left.name.localeCompare(right.name)),
)
const historicalMobTypes = computed(() =>
  [
    ...new Set(
      heatmapFacets.value
        .map((facet) => facet.monster_type?.trim())
        .filter((value): value is string => Boolean(value)),
    ),
  ].sort((left, right) => left.localeCompare(right)),
)
const mapSelectFocused = ref(false)
const stableRegionOptions = ref(regionOptions.value)
const stableHistoricalCharacters = ref(historicalCharacters.value)
const stableHistoricalMobTypes = ref(historicalMobTypes.value)
const stableServerOptions = ref(serverOptions.value)
const stableMapAreas = ref(mapProfile.value?.areas ?? [])
const frozenTraceCandidates = ref(historicalCharacters.value)
const frozenMapPlayers = ref(mapSnapshot.value?.players)
function syncMapSelectOptionSnapshots() {
  stableRegionOptions.value = regionOptions.value
  stableHistoricalCharacters.value = historicalCharacters.value
  stableHistoricalMobTypes.value = historicalMobTypes.value
  stableServerOptions.value = serverOptions.value
  stableMapAreas.value = mapProfile.value?.areas ?? []
  frozenTraceCandidates.value = historicalCharacters.value
  frozenMapPlayers.value = mapSnapshot.value?.players
}
watch(regionOptions, (next) => {
  if (!mapSelectFocused.value) stableRegionOptions.value = next
})
watch(historicalCharacters, (next) => {
  if (!mapSelectFocused.value) {
    stableHistoricalCharacters.value = next
    frozenTraceCandidates.value = next
  }
})
watch(historicalMobTypes, (next) => {
  if (!mapSelectFocused.value) stableHistoricalMobTypes.value = next
})
watch(serverOptions, (next) => {
  if (!mapSelectFocused.value) stableServerOptions.value = next
})
watch(
  () => mapProfile.value?.areas,
  (next) => {
    if (!mapSelectFocused.value) stableMapAreas.value = next ?? []
  },
)
watch(
  () => mapSnapshot.value?.players,
  (next) => {
    if (!mapSelectFocused.value) frozenMapPlayers.value = next
  },
)
function armMapSelectFreeze(event: Event) {
  if (event.target instanceof HTMLSelectElement) mapSelectFocused.value = true
}
function onMapSelectFocusOut(event: FocusEvent) {
  if (!(event.target instanceof HTMLSelectElement)) return
  queueMicrotask(() => {
    const page = event.currentTarget as HTMLElement | null
    if (page?.querySelector('select:focus')) return
    mapSelectFocused.value = false
    syncMapSelectOptionSnapshots()
  })
}
function maybeClearRegionFilterForCharacterRegion(
  characterRegion: number | null | undefined,
) {
  if (mapSelectFocused.value) return
  if (!regionID.value || characterRegion == null) return
  if (regionID.value !== characterRegion) regionID.value = 0
}
const historicalQuery = computed(() => {
  const window = heatmapWindow.value
  if (!window) return null
  return {
    server: server.value,
    area: areaID.value,
    floor: floorID.value,
    region: regionID.value || undefined,
    character_id: analyticsCharacterID.value || undefined,
    monster_type: analyticsMobType.value || undefined,
    from: window.from,
    to: window.to,
  }
})
const renderedHeatLayers = computed(() => {
  if (!mapProfile.value) return []
  return activeHistoricalLayers()
    .map((layer) => heatmapResults[layer])
    .filter((result) => result && result.status !== 'unsupported')
    .map((result) => heatmapResultToLayer(result!, mapProfile.value!))
    .filter((layer) => layer != null)
})
const resettableLayers = computed(() =>
  activeHistoricalLayers().filter(
    (layer) => heatmapResults[layer]?.status !== 'unsupported',
  ),
)
const resetIsBroad = computed(
  () => regionID.value === 0 && analyticsCharacterID.value === '',
)
const appliedLinkedEventKey = ref('')
const linkedEvent = computed(() =>
  linkedEventID.value
    ? mapSnapshot.value?.events.find(
        (event) => event.event_id === linkedEventID.value,
      )
    : undefined,
)
const streamCurrent = computed(
  () => mapFeedCurrent.value[subscriptionID] === true,
)
const profileArea = computed<MapAreaProfile | undefined>(() =>
  mapProfile.value?.areas.find((area) => area.id === areaID.value),
)
const canvasProfile = computed(() => {
  const profile = mapProfile.value
  if (!profile) return null
  const tiles = tileCatalogForFloor(profile, areaID.value, floorID.value)
  return tiles ? { ...profile, tiles } : null
})
const linkedEventLocation = computed(() =>
  mapProfile.value && linkedEvent.value
    ? mapEventLocation(mapProfile.value, linkedEvent.value)
    : null,
)
const linkedEventMessage = computed(() => {
  if (!linkedEventID.value) return ''
  if (!mapSnapshot.value || !streamCurrent.value)
    return 'Looking up the linked event…'
  if (!linkedEvent.value)
    return 'The linked event is unavailable in the selected server scope.'
  if (!mapProfile.value)
    return 'The linked event was found, but its server map profile is unavailable.'
  if (linkedEventLocation.value?.status === 'region-unmapped')
    return `${zoneNameText(linkedEvent.value.zone)} has no area mapping in this server profile; the event cannot be placed.`
  if (linkedEventLocation.value?.status === 'coordinates-unmappable')
    return `The event is in ${linkedEventLocation.value.areaID} / ${linkedEventLocation.value.floorID}, but its coordinates cannot be mapped with the current profile.`
  if (linkedEventLocation.value?.areaID !== 'world') {
    const area = mapProfile.value.areas.find(
      (item) => item.id === linkedEventLocation.value?.areaID,
    )
    const floor = area?.floors.find(
      (item) => item.id === linkedEventLocation.value?.floorID,
    )
    return floor?.image_status === 'available'
      ? `Linked event location resolved on ${floor.label}.`
      : `The linked event coordinates resolve to ${linkedEventLocation.value?.areaID} / ${linkedEventLocation.value?.floorID}, but that floor image is unavailable, so the location cannot be displayed.`
  }
  return `Linked event location resolved in ${linkedEventLocation.value?.areaID} / ${linkedEventLocation.value?.floorID}.`
})
const currentCharacter = computed(() =>
  fleetCharacters.value.find(
    (character) =>
      character.character_id === selectedCharacterID.value &&
      character.server.toLowerCase() === server.value.toLowerCase(),
  ),
)
const currentRegion = computed(
  () => regionID.value || currentCharacter.value?.region,
)
function positionIsFresh(character?: CharacterView) {
  const updatedAt = character?.state_updated_at
    ? Date.parse(character.state_updated_at)
    : Number.NaN
  const age = freshnessNow.value - updatedAt
  return Boolean(
    character?.online &&
    Number.isFinite(updatedAt) &&
    age >= -5_000 &&
    age <= 35_000,
  )
}
function positionCanBeDisplayed(character?: CharacterView) {
  return Boolean(
    character &&
    characterHasDisplayableMapPosition(character, freshnessNow.value),
  )
}
const exactCharacterRasterPosition = computed(() => {
  const character = currentCharacter.value
  if (!mapProfile.value || !character || !positionCanBeDisplayed(character))
    return null
  return worldPositionToRaster(
    mapProfile.value,
    areaID.value,
    floorID.value,
    character.region,
    character.x,
    character.y,
    character.z,
  )
})
const characterRasterPosition = computed(() => {
  if (exactCharacterRasterPosition.value)
    return exactCharacterRasterPosition.value
  const character = currentCharacter.value
  if (!mapProfile.value || !character || !positionCanBeDisplayed(character))
    return null
  return regionTileCenter(
    mapProfile.value,
    areaID.value,
    floorID.value,
    character.region,
  )
})
const inspectorCharacter = computed(() => {
  if (!inspectorOpen.value) return undefined
  const trainingOwner = trainingEditor.selectedArea.value?.character_id
  if (trainingOwner)
    return fleetCharacters.value.find(
      (item) => item.character_id === trainingOwner,
    )
  return characterRasterPosition.value ? currentCharacter.value : undefined
})
const inspectorAge = computed(() => {
  const at = Date.parse(inspectorCharacter.value?.state_updated_at || '')
  return Number.isFinite(at)
    ? `${Math.max(0, Math.floor((freshnessNow.value - at) / 1000))}s ago`
    : 'unknown'
})
function inspectCharacter(id: string) {
  inspectorOpen.value = true
  selectedCharacterID.value = id
  trainingEditor.select(
    layerTraining.value &&
      trainingAreas.value.some((area) => area.character_id === id)
      ? id
      : '',
  )
}
watch(inspectorCharacter, (character) => {
  if (
    character &&
    !trainingEditor.selectedID.value &&
    layerTraining.value &&
    trainingAreas.value.some(
      (area) => area.character_id === character.character_id,
    )
  )
    trainingEditor.select(character.character_id)
})
const selectedDestination = computed(() =>
  mapProfile.value?.quick_destinations.find(
    (destination) => destination.id === selectedDestinationID.value,
  ),
)
const destinationRasterPosition = computed(() => {
  const destination = selectedDestination.value
  if (
    !mapProfile.value ||
    !destination ||
    destination.status !== 'validated' ||
    areaID.value !== destination.area_id ||
    floorID.value !== destination.floor_id
  )
    return null
  return worldPositionToRaster(
    mapProfile.value,
    destination.area_id,
    destination.floor_id,
    destination.region,
    destination.x,
    destination.y,
    destination.z,
  )
})
const mapInitialPosition = computed(() => {
  const linked = linkedEventLocation.value
  if (
    linked?.status === 'mapped' &&
    linked.areaID === areaID.value &&
    linked.floorID === floorID.value
  )
    return linked.position
  if (destinationRasterPosition.value) return destinationRasterPosition.value
  if (selectedCharacterID.value) return characterRasterPosition.value
  if (!mapProfile.value) return null
  const characters = [...(mapSnapshot.value?.characters || [])].sort(
    (left, right) => Number(right.online) - Number(left.online),
  )
  for (const character of characters) {
    if (
      !positionCanBeDisplayed(character) ||
      (regionID.value !== 0 && character.region !== regionID.value)
    )
      continue
    const position =
      worldPositionToRaster(
        mapProfile.value,
        areaID.value,
        floorID.value,
        character.region,
        character.x,
        character.y,
        character.z,
      ) ||
      regionTileCenter(
        mapProfile.value,
        areaID.value,
        floorID.value,
        character.region,
      )
    if (position) return position
  }
  return null
})
const mapInitialTile = computed(() => {
  const preset = mapProfile.value?.view_presets.find(
    (item) => item.area_id === areaID.value && item.floor_id === floorID.value,
  )
  return preset ? { x: preset.tile_x, y: preset.tile_y } : { x: 168, y: 97 }
})
const menuSkipNote = computed(() =>
  [
    ...new Set(
      [
        ...(navigationAction.menuOperation.value?.children || []),
        ...(navigationAction.trainingMenuOperation.value?.children || []),
      ]
        .filter((child) => child.skipReason)
        .map((child) => `${child.characterName}: ${child.skipReason!.message}`),
    ),
  ].join(' · '),
)
const contextGamePosition = computed(() =>
  mapProfile.value && navigationAction.menuPoint.value
    ? rasterPositionToGame(
        mapProfile.value,
        areaID.value,
        floorID.value,
        currentRegion.value,
        navigationAction.menuPoint.value,
        currentCharacter.value?.z ?? 0,
      )
    : null,
)
async function copyMapCoordinates() {
  const point = contextGamePosition.value
  if (!point) return
  try {
    await navigator.clipboard.writeText(
      `${point.x.toFixed(1)}, ${point.y.toFixed(1)}`,
    )
    copyNotice.value = 'Coordinates copied'
  } catch {
    copyNotice.value = 'Could not copy coordinates'
  }
}
watch(navigationAction.menuPoint, () => {
  copyNotice.value = ''
})
const contextRegionAmbiguous = computed(() => {
  const floor = profileArea.value?.floors.find(
    (item) => item.id === floorID.value,
  )
  return Boolean(
    navigationAction.menuPoint.value &&
    floor?.region_ids &&
    floor.region_ids.length > 1 &&
    !floor.region_ids.includes(currentRegion.value ?? 0),
  )
})
function moveMenuFocus(step: number) {
  const items = [
    ...(navigationAction.menuElement.value?.querySelectorAll<HTMLButtonElement>(
      'button[role="menuitem"]:not(:disabled)',
    ) || []),
  ]
  if (!items.length) return
  const index = items.indexOf(document.activeElement as HTMLButtonElement)
  const next =
    index < 0
      ? step > 0
        ? 0
        : items.length - 1
      : (index + step + items.length) % items.length
  items[next]?.focus()
}
function openContextNavigation(action: {
  point: RasterPosition
  anchor: { x: number; y: number }
}) {
  clearTeleportSubmenus()
  void navigationAction.open(
    action.point,
    action.anchor,
    document.querySelector<HTMLElement>('.map-canvas'),
  )
  syncTeleportGateForNavigation()
}
function openNpcNavigation(
  point: RasterPosition,
  anchor: { x: number; y: number },
) {
  clearTeleportSubmenus()
  void navigationAction.open(
    point,
    anchor,
    document.querySelector<HTMLElement>('.map-canvas'),
  )
  syncTeleportGateForNavigation()
}
function openCustomTeleportDestination() {
  void teleportAction.showDestinationDialog()
  navigationAction.close(false)
}
function openTeleporterContext(
  npc: MapNpc,
  point: RasterPosition,
  anchor: { x: number; y: number },
) {
  clearTeleportSubmenus()
  const focusTarget = document.querySelector<HTMLElement>('.map-canvas')
  void navigationAction.open(point, anchor, focusTarget)
  syncTeleportGateForNavigation(npc)
}
const rangedTeleporters = computed(() => {
  const npcs = (mapSnapshot.value?.npcs?.npcs || []).filter(
    (npc) => npc.role === 'teleporter',
  )
  const targets = actionTargetIDs.value
  if (!targets.size) return npcs
  return npcs.filter((npc) =>
    npc.observers?.some((observer) => targets.has(observer.character_id)),
  )
})
function syncTeleportGateForNavigation(preferred?: MapNpc) {
  const gates = rangedTeleporters.value
  const gate =
    preferred && gates.some((item) => item.id === preferred.id)
      ? preferred
      : gates.length === 1
        ? gates[0]
        : null
  if (!gate) {
    if (teleportAction.menuPresentation.value === 'context')
      teleportAction.close(false)
    return
  }
  if (
    teleportAction.menuOpen.value &&
    teleportAction.menuPresentation.value === 'context' &&
    teleportAction.menuNpc.value?.id === gate.id
  )
    return
  void teleportAction.openContext(
    gate,
    navigationAction.menuAnchor.value,
    document.querySelector<HTMLElement>('.map-canvas'),
  )
}
const teleportMenuTrigger = ref<HTMLButtonElement | null>(null)
const teleportGateMenu = ref<{ x: number; y: number } | null>(null)
const teleportDestinationMenu = ref<{ x: number; y: number } | null>(null)
const teleportCharacterMenu = ref<{
  destination: string
  x: number
  y: number
} | null>(null)
let teleportSubmenuTimer: ReturnType<typeof setTimeout> | undefined

function teleportFlyoutOrigin(rect: DOMRect, width: number) {
  const x =
    rect.right + 6 + width > window.innerWidth
      ? Math.max(8, rect.left - width - 6)
      : rect.right + 2
  const y = Math.max(8, Math.min(rect.top - 4, window.innerHeight - 48))
  return { x, y }
}

function keepTeleportSubmenus() {
  if (teleportSubmenuTimer) clearTimeout(teleportSubmenuTimer)
  teleportSubmenuTimer = undefined
}

function clearTeleportSubmenus() {
  keepTeleportSubmenus()
  teleportGateMenu.value = null
  teleportDestinationMenu.value = null
  teleportCharacterMenu.value = null
}

function hideTeleportSubmenusSoon() {
  if (teleportSubmenuTimer) clearTimeout(teleportSubmenuTimer)
  teleportSubmenuTimer = setTimeout(() => {
    teleportGateMenu.value = null
    teleportDestinationMenu.value = null
    teleportCharacterMenu.value = null
  }, 280)
}

function showTeleportMenu(element: HTMLElement) {
  keepTeleportSubmenus()
  const gates = rangedTeleporters.value
  if (gates.length > 1) {
    teleportGateMenu.value = teleportFlyoutOrigin(
      element.getBoundingClientRect(),
      220,
    )
    teleportDestinationMenu.value = null
    teleportCharacterMenu.value = null
    return
  }
  teleportGateMenu.value = null
  if (gates[0]) syncTeleportGateForNavigation(gates[0])
  showTeleportDestinations(element)
}

function showTeleportGateDestinations(npc: MapNpc, element: HTMLElement) {
  keepTeleportSubmenus()
  syncTeleportGateForNavigation(npc)
  showTeleportDestinations(element)
}

function showTeleportDestinations(element: HTMLElement) {
  keepTeleportSubmenus()
  teleportDestinationMenu.value = teleportFlyoutOrigin(
    element.getBoundingClientRect(),
    280,
  )
  teleportCharacterMenu.value = null
}

function showTeleportGroup(destination: string, element: HTMLElement) {
  keepTeleportSubmenus()
  if (!teleportDestinationMenu.value && teleportMenuTrigger.value)
    showTeleportDestinations(teleportMenuTrigger.value)
  teleportCharacterMenu.value = {
    destination,
    ...teleportFlyoutOrigin(element.getBoundingClientRect(), 240),
  }
  void teleportAction.setDestination(destination)
}

watch(
  () => navigationAction.menuOpen.value,
  (open) => {
    if (!open && teleportAction.menuPresentation.value === 'context') {
      clearTeleportSubmenus()
      teleportAction.close(false)
    }
  },
)
watch(
  () => teleportAction.menuOpen.value,
  (open) => {
    if (!open) clearTeleportSubmenus()
  },
)
const scopedCharacters = computed(() => {
  const items = mapSnapshotInFeedScope.value
    ? mapSnapshot.value?.characters || []
    : []
  if (regionID.value !== 0)
    return items.filter((character) => character.region === regionID.value)
  return items
})
const mapActivityFeed = computed(
  () => commandFanOutFeeds.value[MAP_ACTIVITY_OWNER],
)
function mapCharacterControls(characterID: string) {
  return mapActivityFeed.value?.targets[characterID]?.controls ?? null
}
const navigationStopSupport = computed(() => {
  const result: Record<string, boolean> = {}
  for (const [characterID, target] of Object.entries(
    mapActivityFeed.value?.targets || {},
  )) {
    result[characterID] = Boolean(
      target.controls?.capabilities?.['character.navigate.stop']?.supported,
    )
  }
  return result
})
const mapActivityTargetKey = computed(() =>
  scopedCharacters.value
    .filter((character) => character.online && character.session_id)
    .map((character) => character.character_id)
    .sort()
    .join('\u0000'),
)
watch(
  mapActivityTargetKey,
  (key) => {
    setCommandFanOutTargets(MAP_ACTIVITY_OWNER, key ? key.split('\u0000') : [])
  },
  { immediate: true },
)
function characterTraceLines(character: CharacterView) {
  const lines = traceActivitySummary(
    mapCharacterControls(character.character_id) as ControlsSnapshot | null,
  )
  if (
    mapNavigationRoutes.value.some(
      (item) =>
        item.characterID === character.character_id &&
        !item.stale &&
        [
          'moving',
          'waiting_for_movement',
          'waiting_for_arrival',
          'transition_awaiting_evidence',
        ].includes(item.status),
    )
  )
    lines.unshift('Navigating')
  return lines
}
function refreshMapPlayers() {
  refreshLiveData([subscriptionID])
}
function characterGoToMeta(character: CharacterView) {
  const profile = mapProfile.value
  if (!profile) return zoneNameText(character.zone)
  const cave = caveFloorForPosition(profile, character.region, character.z)
  const area = cave?.areaID || 'world'
  const floor = cave?.floorID || 'world'
  const exact = worldPositionToRaster(
    profile,
    area,
    floor,
    character.region,
    character.x,
    character.y,
    character.z,
  )
  const tileOnly =
    !exact && regionTileCenter(profile, area, floor, character.region)
  return `${zoneNameText(character.zone)}${tileOnly ? ' · region tile' : ''}`
}
const goToOptions = computed(() => [
  ...fleetCharacters.value
    .filter((item) => item.server.toLowerCase() === server.value.toLowerCase())
    .map((character) => ({
      id: `character:${character.character_id}`,
      label: character.name,
      meta: characterGoToMeta(character),
      group: 'Characters' as const,
    }))
    .sort((a, b) => a.label.localeCompare(b.label)),
  ...(mapProfile.value?.quick_destinations || [])
    .filter((item) => item.status === 'validated')
    .map((item) => ({
      id: item.id,
      label: item.label,
      meta: 'Verified destination',
      group: 'Places' as const,
    })),
  ...(mapProfile.value?.areas || [])
    .filter((item) => item.kind === 'cave')
    .flatMap((area) =>
      area.floors.map((floor) => ({
        id: `floor:${area.id}:${floor.id}`,
        label: `${area.label} · ${floor.label}`,
        meta: 'Cave floor',
        group: 'Places' as const,
      })),
    ),
])
function selectMapCharacter(id: string) {
  inspectorOpen.value = true
  trainingEditor.select('')
  selectedCharacterID.value = selectedCharacterID.value === id ? '' : id
}
function canFocusCharacter(character: CharacterView) {
  const profile = mapProfile.value
  if (!profile || !positionCanBeDisplayed(character)) return false
  const cave = caveFloorForPosition(profile, character.region, character.z)
  const area = cave?.areaID || (character.region! > 0 ? 'world' : areaID.value)
  const floor = cave?.floorID || (area === 'world' ? 'world' : floorID.value)
  return Boolean(
    worldPositionToRaster(
      profile,
      area,
      floor,
      character.region,
      character.x,
      character.y,
      character.z,
    ) || regionTileCenter(profile, area, floor, character.region),
  )
}
async function focusMapCharacter(id: string) {
  const character = fleetCharacters.value.find(
    (item) => item.character_id === id,
  )
  if (!character || !canFocusCharacter(character)) return
  inspectorOpen.value = true
  trainingEditor.select('')
  selectedCharacterID.value = id
  await nextTick()
  await jumpToCharacter()
}
const outsideZoneCount = computed(() =>
  regionID.value
    ? (mapSnapshot.value?.characters || []).filter(
        (item) => item.online && item.region !== regionID.value,
      ).length
    : 0,
)
function chooseGoTo(id: string) {
  if (id.startsWith('character:')) {
    void focusMapCharacter(id.slice('character:'.length))
  } else selectQuickDestination(id)
}
const applicableActionTargetIDs = computed(
  () =>
    new Set(
      scopedCharacters.value
        .filter((character) => character.online)
        .map((character) => character.character_id),
    ),
)
const mapTargetGroups = computed(() =>
  groups.value
    .filter((group) =>
      group.members.some(
        (member) =>
          member.server.toLocaleLowerCase() ===
            server.value.toLocaleLowerCase() &&
          applicableActionTargetIDs.value.has(member.character_id),
      ),
    )
    .map((group) => ({
      group_id: group.group_id,
      name: group.name,
      memberIDs: [
        ...new Set(
          group.members
            .filter(
              (member) =>
                member.server.toLocaleLowerCase() ===
                  server.value.toLocaleLowerCase() &&
                applicableActionTargetIDs.value.has(member.character_id),
            )
            .map((member) => member.character_id),
        ),
      ],
    }))
    .map((group) => ({
      ...group,
      state: mapActionTargetGroupState(actionTargetIDs.value, group.memberIDs),
    })),
)
function toggleActionTarget(characterID: string) {
  if (!applicableActionTargetIDs.value.has(characterID)) return
  actionTargetIDs.value = toggleMapActionTarget(
    actionTargetIDs.value,
    characterID,
  )
}
function selectAllActionTargets() {
  actionTargetIDs.value = selectAllMapActionTargets(
    applicableActionTargetIDs.value,
  )
}
function clearActionTargets() {
  actionTargetIDs.value = clearMapActionTargets()
}
function toggleActionTargetGroup(memberIDs: string[]) {
  actionTargetIDs.value = applyMapActionTargetGroup(
    actionTargetIDs.value,
    memberIDs,
  )
}
const currentMonsters = computed(() => {
  return dedupeCurrentMonsters(
    mapSnapshotInFeedScope.value ? mapSnapshot.value?.monsters || [] : [],
  )
})
const groupedMonsters = computed(() => {
  const groups = new Map<
    string,
    { name: string; type: string; level: number | undefined; count: number }
  >()
  for (const monster of currentMonsters.value) {
    const name = monsterDisplayName(monster),
      type = monsterTypePresentation(monster).label
    const key = `${name}\0${type}`
    const group = groups.get(key) || {
      name,
      type,
      level: monster.level,
      count: 0,
    }
    group.count++
    groups.set(key, group)
  }
  return [...groups.values()].sort(
    (a, b) =>
      Number(b.type.toLowerCase().includes('unique')) -
        Number(a.type.toLowerCase().includes('unique')) ||
      a.name.localeCompare(b.name),
  )
})
const activityEvents = computed(() =>
  (mapSnapshotInFeedScope.value ? mapSnapshot.value?.events || [] : []).filter(
    (event) => event.kind === 'character.died' || event.category === 'drop',
  ),
)
const historicalLayerIDs: HeatmapLayerID[] = [
  'mob_observer_average',
  'mob_types',
  'deaths',
  'drops',
  'unique_sightings',
  'player_movement',
]
function eventAge(at: string) {
  const seconds = Math.max(
    0,
    Math.floor((freshnessNow.value - Date.parse(at)) / 1000),
  )
  return Number.isFinite(seconds)
    ? seconds < 60
      ? `${seconds}s ago`
      : seconds < 3600
        ? `${Math.floor(seconds / 60)}m ago`
        : `${Math.floor(seconds / 3600)}h ago`
    : 'Unknown time'
}
function focusMapEvent(event: ActivityEvent) {
  if (!mapProfile.value) return
  const location = mapEventLocation(mapProfile.value, event)
  areaID.value = location.areaID
  floorID.value = location.floorID
  regionID.value = location.areaID === 'world' ? event.region || 0 : 0
  selectedCharacterID.value = event.character_id
  inspectorOpen.value = false
  void router.replace({
    path: '/map',
    query: {
      ...route.query,
      event_id: event.event_id,
      server: server.value,
      area: areaID.value,
      floor: floorID.value,
      region: regionID.value ? String(regionID.value) : undefined,
      character_id: event.character_id,
    },
  })
  jumpSequence.value++
}
const currentPartyMembers = computed(
  () => mapSnapshot.value?.party.members || [],
)
const groupByCharacter = computed(() => {
  const names = new Map<string, string>()
  for (const group of groups.value) {
    for (const member of group.members) {
      if (member.server.toLowerCase() === server.value.toLowerCase())
        names.set(member.character_id, group.name)
    }
  }
  return names
})
const monsterSnapshots = computed(() => mapSnapshot.value?.monsters || [])
const monsterSnapshotUnavailable = computed(() =>
  monsterSnapshots.value.some((item) => item.status === 'unavailable'),
)
const monsterSnapshotTruncated = computed(() =>
  monsterSnapshots.value.some((item) => item.status === 'truncated'),
)
const visibleEvents = computed(() => {
  const events = mapSnapshot.value?.events || []
  return events.filter(
    (event) =>
      event.event_id === linkedEventID.value ||
      (event.kind === 'character.died'
        ? layerDeaths.value
        : event.category === 'drop'
          ? layerDrops.value
          : false),
  )
})
const mapMarkers = computed(() => {
  const profile = mapProfile.value
  if (!profile) return []
  const characterMarkers = layerCharacters.value
    ? characterMapMarkers(
        profile,
        areaID.value,
        floorID.value,
        displayableMapCharacters(
          scopedCharacters.value,
          freshnessNow.value,
        ).map((character) => ({
          ...character,
          group_name: groupByCharacter.value.get(character.character_id),
          position_stale: character.online && !positionIsFresh(character),
        })),
      ).map((marker) => ({
        ...marker,
        selected:
          marker.character.character_id === selectedCharacterID.value ||
          actionTargetIDs.value.has(marker.character.character_id),
      }))
    : []
  const markers: Array<{
    id: string
    label: string
    kind:
      | 'character'
      | 'party'
      | 'player'
      | 'npc'
      | 'monster'
      | 'death'
      | 'drop'
      | 'event'
    position: RasterPosition
    placement?: 'exact' | 'region-tile'
    selected?: boolean
    party?: MapPartyMember
    player?: MapOtherPlayer
    npc?: MapNpc
    monster?: MapMonster
    showLabel?: boolean
    itemName?: string
    itemIconUrl?: string
    event?: ActivityEvent
  }> = [...characterMarkers]
  const addMarker = (
    id: string,
    label: string,
    kind: 'character' | 'monster' | 'death' | 'drop' | 'event',
    region: number | undefined,
    x: number | undefined,
    y: number | undefined,
    z: number | undefined,
    details?: {
      monster?: MapMonster
      showLabel?: boolean
      observerName?: string
      zoneLabel?: string
      itemName?: string
      itemIconUrl?: string
      event?: ActivityEvent
    },
  ) => {
    if (regionID.value !== 0 && region !== regionID.value) return
    const position = worldPositionToRaster(
      profile,
      areaID.value,
      floorID.value,
      region,
      x,
      y,
      z,
    )
    if (position) markers.push({ id, label, kind, position, ...details })
  }
  if (layerParty.value) {
    markers.push(
      ...partyMapMarkers(
        profile,
        areaID.value,
        floorID.value,
        currentPartyMembers.value,
        layerCharacters.value
          ? characterMarkers.map((marker) => marker.character.name)
          : [],
      ),
    )
  }
  const playersSnapshot = mapSnapshot.value?.players
  if (
    layerPlayers.value &&
    playersSnapshot &&
    playersSnapshot.status !== 'unavailable'
  ) {
    markers.push(
      ...playerMapMarkers(
        profile,
        areaID.value,
        floorID.value,
        playersSnapshot.players,
        layerCharacters.value
          ? characterMarkers.map((marker) => marker.character.name)
          : [],
        layerParty.value ? currentPartyMembers.value : [],
      ),
    )
  }
  markers.push(
    ...npcMapMarkers(
      profile,
      areaID.value,
      floorID.value,
      mapSnapshot.value?.npcs?.npcs || [],
      layerNPCs.value,
    ),
  )
  if (layerMonsters.value) {
    for (const entry of currentMonsters.value) {
      addMarker(
        `${entry.observer.session_id}:${entry.id}`,
        `${monsterDisplayName(entry)} · Lv. ${entry.level ?? 'unavailable'}`,
        'monster',
        entry.region,
        entry.x,
        entry.y,
        entry.z ?? entry.observer.observer_z,
        {
          monster: entry,
          showLabel: showNearbyMonsterNames.value,
          observerName: entry.observer.character,
          zoneLabel: zoneNameForRegion(entry.region),
        },
      )
    }
  }
  for (const event of visibleEvents.value) {
    const kind =
      event.kind === 'character.died'
        ? 'death'
        : event.category === 'drop'
          ? 'drop'
          : 'event'
    addMarker(
      event.event_id,
      event.kind === 'character.died'
        ? `${event.character} died`
        : event.category === 'drop'
          ? `${event.character} · drop`
          : `Event · ${event.kind}`,
      kind,
      event.region,
      event.x,
      event.y,
      event.z,
      {
        itemName: event.item_name,
        itemIconUrl: event.item_icon_url,
        event,
        zoneLabel: zoneNameText(event.zone),
      },
    )
  }
  return markers.slice(0, 2000)
})
const mapNavigationRoutes = computed(() => {
  const profile = mapProfile.value
  if (!profile) return []
  return mapNavigationRouteOverlays({
    routes: mapSnapshot.value?.navigation,
    characters: [...fleetCharacters.value],
    profile,
    server: server.value,
    areaID: areaID.value,
    floorID: floorID.value,
    region: regionID.value,
    streamCurrent: streamCurrent.value && mapSnapshotInFeedScope.value,
    liveStale: liveStale.value,
    freshnessNow: freshnessNow.value,
    selectedRouteID: selectedNavigationRouteID.value,
  })
})
const placedCharacterCount = computed(
  () => mapMarkers.value.filter((marker) => marker.kind === 'character').length,
)
const placedPartyCount = computed(
  () => mapMarkers.value.filter((marker) => marker.kind === 'party').length,
)
const placedNpcCount = computed(
  () => mapMarkers.value.filter((marker) => marker.kind === 'npc').length,
)
const placedPlayerCount = computed(
  () => mapMarkers.value.filter((marker) => marker.kind === 'player').length,
)
const otherPlayersStatus = computed(
  () => mapSnapshot.value?.players?.status || 'unavailable',
)
const otherPlayersLayerDisabled = computed(
  () => otherPlayersStatus.value === 'unavailable',
)
const zoneNameForRegion = (region?: number | null) => {
  if (region == null) return 'Unknown zone'
  const character = mapSnapshot.value?.characters.find(
    (item) => item.region === region && item.zone,
  )
  if (character?.zone) return zoneNameText(character.zone)
  const event = mapSnapshot.value?.events.find(
    (item) => item.region === region && item.zone,
  )
  return zoneNameText(event?.zone)
}
const zoneOptionLabels = computed(() =>
  uniqueRegionOptionLabels(regionOptions.value, zoneNameForRegion),
)
const navigationTrayRows = computed(() =>
  mapNavigationTrayRows(
    mapNavigationRoutes.value,
    navigationAction.operations.value,
    server.value,
    dismissedNavigationRows.value,
    mapSnapshot.value?.navigation,
    navigationStopSupport.value,
  ).map((row) => {
    const route = mapSnapshot.value?.navigation?.find(
      (item) =>
        `${item.character_id}:${item.session_id}:${item.route_sequence}` ===
        row.id,
    )
    const progress =
      typeof route?.progress === 'number'
        ? ` · ~${Math.round(route.progress * 100)}% steps`
        : ''
    const eta =
      typeof route?.eta_seconds === 'number' && route.eta_seconds > 0
        ? ` · ~${route.eta_seconds}s ETA (approx.)`
        : ''
    return {
      ...row,
      detail:
        (row.detail ||
          (route
            ? `${zoneNameForRegion(route.destination.region)} · ${route.destination.x.toFixed(1)}, ${route.destination.y.toFixed(1)}`
            : '')) +
        progress +
        eta,
      status: navigationStopPending.value.has(row.id)
        ? 'stop_requested'
        : row.status,
    }
  }),
)
const navigationTrayProgress = computed(() =>
  navigationTrayProgressSummary(navigationTrayRows.value),
)
async function stopNavigationRow(row: {
  id: string
  characterID: string
  sessionID?: string
  commandID?: string
  routeSequence?: number
  canStop?: boolean
}) {
  if (
    !row.canStop ||
    !row.sessionID ||
    !row.commandID ||
    !row.routeSequence ||
    navigationStopPending.value.has(row.id)
  )
    return
  navigationStopPending.value = new Set([
    ...navigationStopPending.value,
    row.id,
  ])
  try {
    await submitNavigationStop({
      characterID: row.characterID,
      sessionID: row.sessionID,
      commandID: row.commandID,
      routeSequence: row.routeSequence,
    })
  } catch {
    navigationStopPending.value = new Set(
      [...navigationStopPending.value].filter((id) => id !== row.id),
    )
  }
}
watch(
  () => mapSnapshot.value?.navigation,
  (routes) => {
    if (!routes?.length || !navigationStopPending.value.size) return
    const terminal = new Set(
      routes
        .filter(
          (route) =>
            route.status === 'stopped' || route.status === 'stop_failed',
        )
        .map(
          (route) =>
            `${route.character_id}:${route.session_id}:${route.route_sequence}`,
        ),
    )
    if (!terminal.size) return
    const next = [...navigationStopPending.value].filter(
      (id) => !terminal.has(id),
    )
    if (next.length !== navigationStopPending.value.size) {
      navigationStopPending.value = new Set(next)
    }
  },
  { deep: true },
)
function dismissNavigationRow(id: string) {
  dismissedNavigationRows.value = new Set([
    ...dismissedNavigationRows.value,
    id,
  ])
}
function clearFinishedNavigation() {
  dismissedNavigationRows.value = new Set([
    ...dismissedNavigationRows.value,
    ...navigationTrayRows.value
      .filter((row) => row.group === 'done' || row.status === 'skipped')
      .map((row) => row.id),
  ])
}
function showNavigationOnMap(id: string) {
  selectedNavigationRouteID.value = id
  const overlay = mapNavigationRoutes.value.find(
    (item) => item.characterID === id,
  )
  const point = overlay?.currentAnchor || overlay?.blocks[0]?.[0]
  if (point) mapCanvas.value?.focusAt(point)
}
onMounted(() => {
  if (!mapWorkspaceElement.value) return
  workspaceObserver = new ResizeObserver(([entry]) => {
    navigationTrayCompact.value = Boolean(
      entry && entry.contentRect.height < 470,
    )
  })
  workspaceObserver.observe(mapWorkspaceElement.value)
})
onBeforeUnmount(() => workspaceObserver?.disconnect())
const characterLocation = (character?: CharacterView) => {
  if (
    !character ||
    character.region == null ||
    character.x == null ||
    character.y == null
  )
    return 'Position unavailable'
  return (
    zoneNameText(character.zone) +
    ' · ' +
    character.x.toFixed(1) +
    ', ' +
    character.y.toFixed(1) +
    (character.z == null ? '' : ', ' + character.z.toFixed(1))
  )
}
let profileRequestID = 0
async function loadProfile(selectedServer: string) {
  const requestID = ++profileRequestID
  mapProfile.value = null
  profileError.value = ''
  if (!selectedServer) return
  profileLoading.value = true
  try {
    const profile = await $fetch<MapProfile>(
      `/api/map/profile?server=${encodeURIComponent(selectedServer)}`,
    )
    if (
      !mapProfileRequestIsCurrent(
        requestID,
        profileRequestID,
        selectedServer,
        server.value,
      )
    )
      return
    mapProfile.value = profile
  } catch {
    if (
      mapProfileRequestIsCurrent(
        requestID,
        profileRequestID,
        selectedServer,
        server.value,
      )
    )
      profileError.value = 'The selected server has no available map profile.'
  } finally {
    if (
      mapProfileRequestIsCurrent(
        requestID,
        profileRequestID,
        selectedServer,
        server.value,
      )
    )
      profileLoading.value = false
  }
}

function currentMapRouteQuery() {
  const query = {
    ...route.query,
    server: server.value,
    area: areaID.value,
    floor: floorID.value,
  } as Record<string, string | undefined>
  if (regionID.value) query.region = String(regionID.value)
  else delete query.region
  if (selectedCharacterID.value) query.character_id = selectedCharacterID.value
  else delete query.character_id
  return query
}

function updateRouteQuery() {
  void router.replace({ path: '/map', query: currentMapRouteQuery() })
}

function selectArea(area: MapAreaProfile) {
  areaID.value = area.id
  floorID.value = area.floors[0]?.id || 'world'
  regionID.value = 0
  selectedDestinationID.value = ''
  updateRouteQuery()
}

function returnToWorld() {
  const world = mapProfile.value?.areas.find((area) => area.id === 'world')
  if (world) selectArea(world)
}

async function jumpToCharacter() {
  const character = currentCharacter.value
  const profile = mapProfile.value
  if (!character || !profile) return
  if (linkedEventID.value) {
    const query = currentMapRouteQuery()
    delete query.event_id
    await router.replace({ path: '/map', query })
  }
  const cave = caveFloorForPosition(profile, character.region, character.z)
  if (
    cave &&
    (areaID.value !== cave.areaID || floorID.value !== cave.floorID)
  ) {
    areaID.value = cave.areaID
    floorID.value = cave.floorID
    regionID.value = 0
    updateRouteQuery()
  } else if (
    character.region != null &&
    character.region > 0 &&
    areaID.value !== 'world'
  ) {
    areaID.value = 'world'
    floorID.value = 'world'
    regionID.value = 0
    updateRouteQuery()
  }
  await nextTick()
  jumpSequence.value++
}

function selectQuickDestination(destinationID: string) {
  if (destinationID.startsWith('floor:')) {
    const [, area, floor] = destinationID.split(':')
    if (
      !mapProfile.value?.areas
        .find((item) => item.id === area)
        ?.floors.some((item) => item.id === floor)
    )
      return
    areaID.value = area || 'world'
    floorID.value = floor || 'world'
    regionID.value = 0
    selectedDestinationID.value = destinationID
    updateRouteQuery()
    jumpSequence.value++
    return
  }
  const destination = mapProfile.value?.quick_destinations.find(
    (item) => item.id === destinationID && item.status === 'validated',
  )
  if (!destination) return
  areaID.value = destination.area_id
  floorID.value = destination.floor_id
  regionID.value = destination.region
  selectedCharacterID.value = ''
  selectedDestinationID.value = destination.id
  updateRouteQuery()
  jumpSequence.value++
}

async function refreshHeatmaps() {
  const query = historicalQuery.value
  if (!query) return
  await refreshHistoricalHeatmaps(query)
}

function openHeatmapReset() {
  const available = resettableLayers.value
  const window = heatmapWindow.value
  if (!available.length || !window) return
  if (!available.includes(resetLayer.value)) resetLayer.value = available[0]!
  resetWindow.value = { ...window }
  resetError.value = ''
  confirmBroadReset.value = false
  resetOpen.value = true
}

async function performHeatmapReset() {
  const window = resetWindow.value
  if (!window || resetBusy.value) return
  resetBusy.value = true
  resetError.value = ''
  try {
    await resetHistoricalHeatmap({
      layer: resetLayer.value,
      server: server.value,
      area_id: areaID.value,
      floor_id: floorID.value,
      region: regionID.value || undefined,
      character_id: analyticsCharacterID.value || undefined,
      monster_type:
        resetLayer.value === 'mob_types' ||
        resetLayer.value === 'mob_observer_average'
          ? analyticsMobType.value || undefined
          : undefined,
      from: window.from,
      to: window.to,
      confirm_broad: resetIsBroad.value ? confirmBroadReset.value : false,
    })
    resetOpen.value = false
    resetWindow.value = null
    await refreshHeatmaps()
  } catch (error) {
    resetError.value =
      error instanceof Error ? error.message : 'Heatmap reset failed'
  } finally {
    resetBusy.value = false
  }
}

watch(server, (value) => {
  analyticsCharacterID.value = ''
  analyticsMobType.value = ''
  void loadProfile(value)
})
watch([mapProfile, linkedEvent], ([profile, event]) => {
  if (!profile || !event) return
  const eventKey = `${server.value.toLowerCase()}\u0000${event.event_id}`
  if (appliedLinkedEventKey.value === eventKey) return
  appliedLinkedEventKey.value = eventKey
  const location = mapEventLocation(profile, event)
  areaID.value = location.areaID
  floorID.value = location.floorID
  if (event.region != null && location.areaID === 'world')
    regionID.value = event.region
  else regionID.value = 0
  selectedCharacterID.value = event.character_id
})
watch(selectedCharacterID, (characterID, previousID) => {
  if (characterID) selectedDestinationID.value = ''
  if (characterID === previousID) return
  const selected = mapSnapshot.value?.characters.find(
    (character) => character.character_id === characterID,
  )
  maybeClearRegionFilterForCharacterRegion(selected?.region)
})
watch(
  () => currentCharacter.value?.region,
  (region) => {
    maybeClearRegionFilterForCharacterRegion(region)
  },
)
watch(
  actionTargetScopeKey,
  (nextScope, previousScope) => {
    if (previousScope && nextScope !== previousScope)
      actionTargetIDs.value = clearMapActionTargets()
  },
  { flush: 'sync' },
)
watch(
  [mapSnapshot, streamCurrent, expectedMapFeedScope],
  ([currentSnapshot, isCurrent, feedScope]) => {
    const reconciled = reconcileMapActionTargets(
      actionTargetIDs.value,
      applicableActionTargetIDs.value,
      Boolean(
        isCurrent &&
        currentSnapshot &&
        mapSnapshotMatchesScope(currentSnapshot, feedScope),
      ),
    )
    if (reconciled !== actionTargetIDs.value) actionTargetIDs.value = reconciled
  },
)
watch([server, areaID, floorID, regionID, selectedCharacterID], () => {})
watch(mapProfile, (profile) => {
  if (!profile || mapSelectFocused.value) return
  const selectedArea = profile.areas.find((area) => area.id === areaID.value)
  if (!selectedArea) {
    areaID.value = 'world'
    floorID.value = 'world'
    regionID.value = 0
    updateRouteQuery()
    return
  }
  if (!selectedArea.floors.some((floor) => floor.id === floorID.value)) {
    floorID.value = selectedArea.floors[0]?.id || 'world'
    updateRouteQuery()
  }
})
watch(
  [
    server,
    areaID,
    floorID,
    regionID,
    selectedCharacterID,
    dateRange,
    eventWindowNow,
    linkedEventID,
  ],
  ([selectedServer, area, floor, region, characterID, range, now, eventID]) => {
    if (!selectedServer) return
    const { from, to } = relativeMapEventWindow(range, now)
    setMapFeed(subscriptionID, {
      server: selectedServer,
      area,
      floor,
      region: area === 'world' ? mapFeedRegion(region, characterID) : undefined,
      from,
      to,
      event_id: eventID || undefined,
    })
  },
  { immediate: true },
)
watch(
  () => route.query,
  (query) => {
    if (typeof query.area === 'string') areaID.value = query.area
    if (typeof query.floor === 'string') floorID.value = query.floor
    regionID.value =
      typeof query.region === 'string' ? Number(query.region) || 0 : 0
    selectedCharacterID.value =
      typeof query.character_id === 'string' ? query.character_id : ''
  },
)
watch([areaID, floorID, regionID, selectedCharacterID], updateRouteQuery)
watch(
  [
    server,
    areaID,
    floorID,
    regionID,
    analyticsCharacterID,
    analyticsMobType,
    heatmapRange,
    heatmapCustomFrom,
    heatmapCustomTo,
  ],
  () => {
    clearHistoricalHeatmaps()
  },
)
watch(
  [
    server,
    areaID,
    floorID,
    regionID,
    analyticsCharacterID,
    analyticsMobType,
    heatmapRange,
    heatmapCustomFrom,
    heatmapCustomTo,
    eventWindowNow,
    () => historicalLayers.mob_observer_average,
    () => historicalLayers.mob_types,
    () => historicalLayers.deaths,
    () => historicalLayers.drops,
    () => historicalLayers.unique_sightings,
    () => historicalLayers.player_movement,
  ],
  () => {
    void refreshHeatmaps()
  },
  { immediate: true },
)
let eventWindowTimer: ReturnType<typeof setInterval> | undefined
onMounted(() => {
  void loadProfile(server.value)
  eventWindowTimer = setInterval(() => {
    eventWindowNow.value = Date.now()
  }, 30_000)
})
onBeforeUnmount(() => {
  if (eventWindowTimer) clearInterval(eventWindowTimer)
  if (actionNotificationTimer) clearTimeout(actionNotificationTimer)
  profileRequestID++
  clearCommandFanOutOwner(MAP_ACTIVITY_OWNER)
  clearMapFeed(subscriptionID)
})

useHead({ title: 'Map · PhMon' })
</script>

<template>
  <section
    class="map-page"
    @pointerdown="armMapSelectFreeze"
    @focusin="armMapSelectFreeze"
    @focusout="onMapSelectFocusOut"
  >
    <header
      class="page-header map-page-header map-header-row character-filters"
    >
      <h1><UIcon name="i-lucide-map" /> Map</h1>

      <span v-if="serverScope !== 'all'">{{ server }}</span>
      <label v-if="serverScope === 'all'">
        Server
        <select
          :value="server"
          aria-label="Map server"
          @change="
            (event) =>
              router.replace({
                path: '/map',
                query: {
                  ...route.query,
                  server: (event.target as HTMLSelectElement).value,
                },
              })
          "
        >
          <option
            v-for="option in stableServerOptions"
            :key="option"
            :value="option"
          >
            {{ option }}
          </option>
        </select>
      </label>
      <label>
        Area
        <select
          :value="areaID"
          aria-label="Map area"
          @change="
            (event) => {
              const area = mapProfile?.areas.find(
                (item) => item.id === (event.target as HTMLSelectElement).value,
              )
              if (area) selectArea(area)
            }
          "
        >
          <option
            v-for="area in stableMapAreas"
            :key="area.id"
            :value="area.id"
          >
            {{ area.label }}
          </option>
        </select>
      </label>
      <div
        v-if="profileArea?.kind === 'cave'"
        class="map-header-floors"
        role="group"
        aria-label="Cave map floor"
      >
        <button
          v-for="floor in profileArea.floors"
          :key="floor.id"
          class="compact-button"
          :class="{ selected: floorID === floor.id }"
          type="button"
          :aria-pressed="floorID === floor.id"
          @click="floorID = floor.id"
        >
          {{ floor.label }}
        </button>
      </div>

      <label v-if="areaID === 'world'">
        Zone
        <select v-model.number="regionID" aria-label="Filter map data by zone">
          <option :value="0">All zones</option>
          <option
            v-for="region in stableRegionOptions"
            :key="region"
            :value="region"
          >
            {{ zoneOptionLabels.get(region) }}
          </option>
        </select>
      </label>
      <MapGoToSearch :options="goToOptions" @choose="chooseGoTo" />

      <div
        class="map-page-status"
        :class="streamCurrent ? 'current' : 'stale'"
        :title="
          streamCurrent
            ? 'Live scope current'
            : connectionState === 'stale'
              ? 'Showing stale map data'
              : 'Syncing map data'
        "
        role="status"
      >
        <span />{{
          streamCurrent
            ? 'Live'
            : connectionState === 'stale'
              ? 'Stale'
              : 'Syncing'
        }}
      </div>
    </header>

    <div ref="mapWorkspaceElement" class="map-workspace">
      <section
        class="map-viewport-panel panel"
        aria-label="Game world raster map"
      >
        <div class="map-canvas-frame">
          <MapActionToast :notification="actionNotification" />
          <ClientOnly>
            <MapCanvas
              ref="mapCanvas"
              :external-controls="true"
              :focusedCharacterID="selectedCharacterID"
              v-if="canvasProfile?.tiles.status === 'available-for-inspection'"
              :key="`${server}:${areaID}:${floorID}:${mapProfile?.dataset_id}:${mapProfile?.dataset_version}`"
              :profile="canvasProfile"
              :initial-position="mapInitialPosition"
              :focus-request="jumpSequence"
              :initial-tile="mapInitialTile"
              :markers="mapMarkers"
              :heat-layers="renderedHeatLayers"
              :navigation-routes="mapNavigationRoutes"
              :training-areas="renderedTrainingAreas"
              :training-editable="trainingEditor.editable.value"
              :training-accept-disabled="
                Boolean(trainingEditor.applyReason.value)
              "
              :training-discard-disabled="trainingEditor.applying.value"
              :training-accept-title="trainingEditor.applyReason.value"
              @inspectcharacter="inspectCharacter"
              @viewchange="mapView = $event"
              @pointselect="selectMapPoint"
              @trainingselect="selectTrainingArea"
              @trainingmove="trainingEditor.moveCenter"
              @trainingresize="trainingEditor.resizeFromPixels"
              @trainingaccept="acceptTrainingDraft"
              @trainingdiscard="discardTrainingDraft"
              @contextaction="openContextNavigation"
              @navigateto="openNpcNavigation"
              @teleportto="openTeleporterContext"
              @teleportercontext="openTeleporterContext"
              @mapdrag="
                () => {
                  navigationAction.close(false)
                  teleportAction.close(false)
                }
              "
              @opencharacter="
                (characterID) =>
                  router.push(`/characters/${encodeURIComponent(characterID)}`)
              "
            />
            <template #fallback
              ><div class="map-empty-view">
                Loading the local raster map…
              </div></template
            >
          </ClientOnly>
          <div
            v-if="actionTargetIDs.size"
            class="panel map-target-bar"
            @pointerdown.stop
            @click.stop
          >
            <strong>{{ actionTargetIDs.size }} targeted</strong
            ><span>Right-click the map to send them</span
            ><button
              class="map-text-action"
              type="button"
              @click="clearActionTargets"
            >
              Clear
            </button>
          </div>
          <div class="map-floating-controls" @pointerdown.stop @click.stop>
            <button
              class="compact-button"
              type="button"
              aria-label="Zoom in"
              @click="mapCanvas?.zoomIn()"
            >
              <UIcon name="i-lucide-plus" /></button
            ><button
              class="compact-button"
              type="button"
              aria-label="Zoom out"
              @click="mapCanvas?.zoomOut()"
            >
              <UIcon name="i-lucide-minus" /></button
            ><button
              class="compact-button"
              type="button"
              aria-label="Center on selected character"
              :disabled="
                !currentCharacter || !positionCanBeDisplayed(currentCharacter)
              "
              @click="jumpToCharacter"
            >
              <UIcon name="i-lucide-crosshair" /></button
            ><button
              class="compact-button"
              type="button"
              :aria-expanded="legendOpen"
              aria-controls="map-legend"
              aria-label="Map legend"
              @click="legendOpen = !legendOpen"
            >
              <UIcon name="i-lucide-info" />
            </button>
          </div>
          <section
            v-if="legendOpen"
            id="map-legend"
            class="panel map-floating-legend"
            aria-label="Map legend"
            @pointerdown.stop
            @click.stop
          >
            <div class="map-list-heading">
              <strong>Legend</strong
              ><button
                class="compact-button"
                type="button"
                aria-label="Close map legend"
                @click="legendOpen = false"
              >
                ×
              </button>
            </div>
            <p>
              Characters · Party members · NPCs · Nearby monsters · Deaths ·
              Drops · Training areas · Navigation routes
            </p>
            <details>
              <summary>How positions are placed</summary>
              <p>
                {{
                  mapProfile?.tiles.semantics ||
                  'Tile and coordinate evidence is unavailable.'
                }}
                Outdoor region IDs locate their root tile directly. Reported X/Y
                positions locate markers within that tile; outlined dots
                indicate a region-only position. Cave imagery uses a 2D X/Y
                anchor and region/floor rules. Navigation and training position
                resolve coordinates and Z per target. Older observations remain
                visible as last observed positions.
              </p>
              <p>
                Dataset {{ mapProfile?.dataset_id || 'unavailable' }} ·
                {{ mapProfile?.dataset_version }}
              </p>
            </details>
          </section>
          <div class="map-corner-readouts map-empty-copy" @pointerdown.stop>
            Tile {{ mapView.tileX }} × {{ mapView.tileY }} ·
            {{ mapView.zoomPercent }}%
          </div>
          <div
            v-if="linkedEventMessage && linkedNoticeDismissed !== linkedEventID"
            class="panel map-floating-notice"
            role="status"
            @pointerdown.stop
            @click.stop
          >
            {{ linkedEventMessage
            }}<button
              class="compact-button"
              type="button"
              aria-label="Dismiss linked event notice"
              @click="linkedNoticeDismissed = linkedEventID"
            >
              ×
            </button>
          </div>
          <section
            v-if="inspectorCharacter"
            class="panel map-inspector"
            aria-label="Character inspector"
            @pointerdown.stop
            @click.stop
          >
            <div class="map-list-heading">
              <small
                >Lv {{ inspectorCharacter.level ?? '—'
                }}{{
                  groupByCharacter.get(inspectorCharacter.character_id)
                    ? ` · ${groupByCharacter.get(inspectorCharacter.character_id)}`
                    : ''
                }}</small
              ><button
                class="compact-button"
                type="button"
                aria-label="Close character inspector"
                @click="inspectorOpen = false"
              >
                <UIcon name="i-lucide-x" />
              </button>
            </div>
            <h2>{{ inspectorCharacter.name }}</h2>
            <p class="map-empty-copy">
              {{ characterLocation(inspectorCharacter) }} · updated
              {{ inspectorAge }}
            </p>
            <MapCharacterResources :character="inspectorCharacter" />
            <p v-if="inspectorCharacter.dead === true" class="map-empty-copy">
              Character is dead.
            </p>
            <p
              v-if="!positionIsFresh(inspectorCharacter)"
              class="map-empty-copy"
            >
              Position is stale; last report {{ inspectorAge }}.
            </p>
            <div
              v-if="trainingEditor.selectedArea.value"
              class="map-training-editor"
              role="group"
              :aria-label="`Edit training area for ${trainingEditor.selectedArea.value.name}`"
            >
              <p class="map-training-readback">
                Observed center
                {{ trainingEditor.selectedArea.value.x.toFixed(1) }},
                {{ trainingEditor.selectedArea.value.y.toFixed(1) }} · radius
                {{ trainingEditor.selectedArea.value.radius }}
              </p>
              <p
                v-if="trainingEditor.draft.value?.center"
                class="map-training-draft"
              >
                New center
                {{ trainingEditor.draft.value.center.x.toFixed(1) }},
                {{ trainingEditor.draft.value.center.y.toFixed(1) }} in
                {{
                  zoneNameForRegion(trainingEditor.draft.value.center.region)
                }}
              </p>
              <p
                v-if="trainingEditor.editReason.value"
                class="map-empty-copy"
                role="note"
              >
                {{ trainingEditor.editReason.value }}
              </p>
              <div class="map-training-editor-controls">
                <button
                  class="compact-button"
                  :class="{ selected: trainingEditor.moveArmed.value }"
                  type="button"
                  :aria-pressed="trainingEditor.moveArmed.value"
                  :disabled="!trainingEditor.editable.value"
                  @click="
                    trainingEditor.moveArmed.value =
                      !trainingEditor.moveArmed.value
                  "
                >
                  {{
                    trainingEditor.moveArmed.value
                      ? 'Cancel moving'
                      : 'Move center'
                  }}
                </button>
                <label class="map-training-radius">
                  Radius
                  <input
                    type="number"
                    inputmode="numeric"
                    :min="TRAINING_RADIUS_MIN"
                    :max="TRAINING_RADIUS_MAX"
                    step="1"
                    :value="trainingEditor.displayedRadius.value"
                    :disabled="!trainingEditor.editable.value"
                    @change="
                      trainingEditor.setRadius(
                        Number(($event.target as HTMLInputElement).value),
                      )
                    "
                  />
                </label>
              </div>
              <p
                v-if="trainingEditor.moveArmed.value"
                class="map-empty-copy"
                role="status"
              >
                Click the map, or press Enter with the map focused, to place the
                new center. Dragging the center handle also works.
              </p>
              <div
                v-if="trainingEditor.dirty.value"
                class="map-training-editor-actions"
              >
                <button
                  class="compact-button primary"
                  type="button"
                  :disabled="Boolean(trainingEditor.applyReason.value)"
                  :title="trainingEditor.applyReason.value"
                  @click="trainingEditor.apply"
                >
                  {{ trainingEditor.applying.value ? 'Applying…' : 'Apply' }}
                </button>
                <button
                  class="compact-button"
                  type="button"
                  :disabled="
                    !trainingEditor.dirty.value || trainingEditor.applying.value
                  "
                  @click="trainingEditor.reset"
                >
                  Reset
                </button>
              </div>
              <ul
                v-if="trainingEditor.results.value.length"
                class="map-training-results"
                role="status"
              >
                <li
                  v-for="(result, index) in trainingEditor.results.value"
                  :key="index"
                  :class="`outcome-${result.outcome}`"
                >
                  <strong>{{
                    result.step.name === 'training.area.set'
                      ? 'Center'
                      : 'Radius'
                  }}</strong>
                  {{ trainingOutcomeLabel(result.outcome)
                  }}{{ result.message ? ` · ${result.message}` : '' }}
                </li>
              </ul>
              <p
                v-if="trainingEditor.message.value"
                class="map-empty-copy"
                role="status"
              >
                {{ trainingEditor.message.value }}
              </p>
            </div>

            <div class="map-training-editor-actions">
              <button
                class="compact-button"
                type="button"
                :disabled="
                  !applicableActionTargetIDs.has(
                    inspectorCharacter.character_id,
                  )
                "
                :aria-pressed="
                  actionTargetIDs.has(inspectorCharacter.character_id)
                "
                @click="toggleActionTarget(inspectorCharacter.character_id)"
              >
                {{
                  actionTargetIDs.has(inspectorCharacter.character_id)
                    ? 'Targeted'
                    : 'Target'
                }}
              </button>
              <NuxtLink
                class="compact-button"
                :to="`/characters/${encodeURIComponent(inspectorCharacter.character_id)}`"
                >Open Stats</NuxtLink
              >
            </div>
          </section>
          <div
            v-if="navigationAction.menuOpen.value"
            :ref="navigationAction.menuElement"
            class="map-navigation-context"
            role="menu"
            tabindex="-1"
            aria-label="Map point actions for selected characters"
            :style="{
              left: `${navigationAction.menuAnchor.value.x}px`,
              top: `${navigationAction.menuAnchor.value.y}px`,
            }"
            @pointerdown.stop
            @keydown.down.prevent="moveMenuFocus(1)"
            @keydown.up.prevent="moveMenuFocus(-1)"
          >
            <div class="map-context-heading">
              <strong>{{
                contextGamePosition
                  ? zoneNameForRegion(contextGamePosition.region)
                  : profileArea?.label
              }}</strong
              ><small v-if="contextGamePosition"
                >{{ contextGamePosition.x.toFixed(1) }},
                {{ contextGamePosition.y.toFixed(1) }}</small
              >
            </div>
            <p
              v-if="contextRegionAmbiguous"
              class="map-navigation-menu-summary"
            >
              This floor has two possible region IDs. Choose a region or a
              character in that region.
            </p>
            <p v-if="!actionTargetIDs.size" class="map-navigation-menu-summary">
              Tick characters in the panel to act on them.
            </p>
            <button
              class="map-navigation-menu-action"
              role="menuitem"
              data-map-action="navigate"
              type="button"
              :disabled="
                navigationAction.preparing.value ||
                navigationAction.submitting.value ||
                navigationAction.counts.value.eligible === 0
              "
              :aria-describedby="
                navigationAction.menuSummary.value
                  ? 'map-navigation-menu-summary'
                  : undefined
              "
              @click="navigationAction.submit"
            >
              <UIcon name="i-lucide-navigation" />
              {{
                navigationAction.preparing.value
                  ? 'Checking targets…'
                  : navigationAction.submitting.value
                    ? 'Submitting…'
                    : navigationAction.targetLabel.value
              }}
            </button>
            <p
              v-if="navigationAction.menuSummary.value"
              id="map-navigation-menu-summary"
              class="map-navigation-menu-summary"
              role="status"
            >
              {{ menuSkipNote || navigationAction.menuSummary.value }}
            </p>
            <button
              class="map-navigation-menu-action"
              role="menuitem"
              data-map-action="training"
              type="button"
              :disabled="
                navigationAction.trainingPreparing.value ||
                navigationAction.trainingSubmitting.value ||
                navigationAction.trainingCounts.value.eligible === 0
              "
              :aria-describedby="
                navigationAction.trainingSummary.value
                  ? 'map-training-menu-summary'
                  : undefined
              "
              @click="navigationAction.submitTraining"
            >
              <UIcon name="i-lucide-crosshair" />
              {{
                navigationAction.trainingPreparing.value
                  ? 'Checking targets…'
                  : navigationAction.trainingSubmitting.value
                    ? 'Submitting…'
                    : navigationAction.trainingLabel.value
              }}
            </button>
            <p
              v-if="navigationAction.trainingSummary.value"
              id="map-training-menu-summary"
              class="map-navigation-menu-summary"
              role="status"
            >
              {{ navigationAction.trainingSummary.value }}
            </p>
            <button
              v-if="rangedTeleporters.length"
              ref="teleportMenuTrigger"
              class="map-navigation-menu-action"
              role="menuitem"
              type="button"
              aria-haspopup="true"
              :aria-expanded="
                Boolean(teleportDestinationMenu || teleportGateMenu)
              "
              @mouseenter="
                showTeleportMenu($event.currentTarget as HTMLElement)
              "
              @mouseleave="hideTeleportSubmenusSoon"
              @focus="showTeleportMenu($event.currentTarget as HTMLElement)"
            >
              <UIcon name="i-lucide-signpost" />
              Teleport
              <UIcon
                name="i-lucide-chevron-right"
                class="map-context-submenu-chevron"
              />
            </button>
            <button
              class="map-navigation-menu-action"
              role="menuitem"
              type="button"
              :disabled="!contextGamePosition"
              @click="copyMapCoordinates"
            >
              <UIcon name="i-lucide-copy" />Copy coordinates
            </button>
            <p
              v-if="copyNotice"
              class="map-navigation-menu-summary"
              role="status"
            >
              {{ copyNotice }}
            </p>
          </div>
          <Teleport to="body">
            <div
              v-if="teleportGateMenu"
              class="map-teleport-flyout-panel map-teleport-menu"
              role="menu"
              aria-label="Teleporters in range"
              :style="{
                left: `${teleportGateMenu.x}px`,
                top: `${teleportGateMenu.y}px`,
              }"
              @pointerdown.stop
              @mouseenter="keepTeleportSubmenus"
              @mouseleave="hideTeleportSubmenusSoon"
            >
              <button
                v-for="npc in rangedTeleporters"
                :key="npc.id"
                class="map-navigation-menu-action"
                type="button"
                role="menuitem"
                aria-haspopup="true"
                @mouseenter="
                  showTeleportGateDestinations(
                    npc,
                    $event.currentTarget as HTMLElement,
                  )
                "
                @focus="
                  showTeleportGateDestinations(
                    npc,
                    $event.currentTarget as HTMLElement,
                  )
                "
              >
                {{ npcDisplayLabel(npc) }}
                <UIcon
                  name="i-lucide-chevron-right"
                  class="map-context-submenu-chevron"
                />
              </button>
            </div>
            <div
              v-if="teleportDestinationMenu"
              class="map-teleport-flyout-panel map-teleport-menu"
              role="menu"
              aria-label="Teleport destinations"
              :style="{
                left: `${teleportDestinationMenu.x}px`,
                top: `${teleportDestinationMenu.y}px`,
              }"
              @pointerdown.stop
              @mouseenter="keepTeleportSubmenus"
              @mouseleave="hideTeleportSubmenusSoon"
            >
              <template v-if="teleportAction.discoveredRoutes.value.length">
                <button
                  v-for="route in teleportAction.discoveredRoutes.value"
                  :key="route.destination"
                  class="map-navigation-menu-action"
                  type="button"
                  role="menuitem"
                  aria-haspopup="true"
                  :aria-expanded="
                    teleportCharacterMenu?.destination === route.destination
                  "
                  @mouseenter="
                    showTeleportGroup(
                      route.destination,
                      $event.currentTarget as HTMLElement,
                    )
                  "
                  @focus="
                    showTeleportGroup(
                      route.destination,
                      $event.currentTarget as HTMLElement,
                    )
                  "
                >
                  {{ route.destination }}
                  <UIcon
                    name="i-lucide-chevron-right"
                    class="map-context-submenu-chevron"
                  />
                </button>
              </template>
              <p v-else class="map-navigation-menu-summary">
                No verified destinations from this gate yet.
              </p>
              <button
                class="map-navigation-menu-action"
                type="button"
                role="menuitem"
                @click="openCustomTeleportDestination"
              >
                <UIcon name="i-lucide-pencil" />
                Other destination…
              </button>
            </div>
            <div
              v-if="teleportCharacterMenu"
              class="map-teleport-flyout-panel map-teleport-menu"
              role="menu"
              :aria-label="`Teleport to ${teleportCharacterMenu.destination}`"
              :style="{
                left: `${teleportCharacterMenu.x}px`,
                top: `${teleportCharacterMenu.y}px`,
              }"
              @pointerdown.stop
              @mouseenter="keepTeleportSubmenus"
              @mouseleave="hideTeleportSubmenusSoon"
            >
              <button
                class="map-navigation-menu-action"
                type="button"
                role="menuitem"
                :disabled="
                  teleportAction.preparing.value ||
                  teleportAction.submitting.value ||
                  teleportAction.counts.value.eligible === 0
                "
                @click="teleportAction.submit()"
              >
                {{
                  teleportAction.preparing.value
                    ? 'Checking targets…'
                    : teleportAction.targetLabel.value
                }}
              </button>
              <p
                v-if="teleportAction.menuSummary.value"
                class="map-navigation-menu-summary"
                role="status"
              >
                {{ teleportAction.menuSummary.value }}
              </p>
            </div>
          </Teleport>
          <div
            v-if="
              teleportAction.menuOpen.value &&
              teleportAction.menuPresentation.value === 'dialog'
            "
            :ref="teleportAction.menuElement"
            class="map-navigation-context map-teleport-context map-teleport-menu"
            role="dialog"
            aria-label="Teleporter destination"
            :style="{
              left: `${teleportAction.menuAnchor.value.x}px`,
              top: `${teleportAction.menuAnchor.value.y}px`,
            }"
            @pointerdown.stop
          >
            <div class="map-context-heading">
              <strong>{{ teleportAction.gateLabel.value }}</strong
              ><small>Teleporter</small>
            </div>
            <div class="map-teleport-destination">
              <span class="map-teleport-destination-label">Destination</span>
              <ul
                v-if="teleportAction.discoveredRoutes.value.length"
                class="map-teleport-route-list"
                role="listbox"
                aria-label="Discovered teleport destinations"
              >
                <li
                  v-for="route in teleportAction.discoveredRoutes.value"
                  :key="route.destination"
                  role="option"
                  :aria-selected="
                    teleportAction.destination.value === route.destination
                  "
                >
                  <button
                    type="button"
                    class="map-teleport-route-button"
                    :class="{
                      'is-selected':
                        teleportAction.destination.value === route.destination,
                    }"
                    @click="
                      teleportAction.destination.value = route.destination
                      teleportAction.onDestinationInput()
                    "
                  >
                    {{ route.destination }}
                  </button>
                </li>
              </ul>
              <label v-else class="map-teleport-destination-field">
                <span class="sr-only">Destination name</span>
                <input
                  type="text"
                  name="teleport-destination"
                  autocomplete="off"
                  spellcheck="false"
                  maxlength="64"
                  placeholder="Town name (for example Jangan)"
                  :value="teleportAction.destination.value"
                  @input="
                    teleportAction.destination.value = (
                      $event.target as HTMLInputElement
                    ).value
                    teleportAction.onDestinationInput()
                  "
                />
              </label>
              <p
                v-if="teleportAction.discoveredRoutes.value.length"
                class="map-teleport-route-note"
              >
                Routes verified with get_teleport_data at this gate. Type a
                different name below if needed.
              </p>
              <label
                v-if="teleportAction.discoveredRoutes.value.length"
                class="map-teleport-destination-field map-teleport-destination-other"
              >
                <span>Other destination</span>
                <input
                  type="text"
                  name="teleport-destination-custom"
                  autocomplete="off"
                  spellcheck="false"
                  maxlength="64"
                  :value="teleportAction.destination.value"
                  @input="
                    teleportAction.destination.value = (
                      $event.target as HTMLInputElement
                    ).value
                    teleportAction.onDestinationInput()
                  "
                />
              </label>
            </div>
            <p v-if="!actionTargetIDs.size" class="map-navigation-menu-summary">
              Tick characters in the panel to teleport them.
            </p>
            <CommandFanOutPreview
              v-if="teleportAction.reviewOperation.value"
              :operation="teleportAction.reviewOperation.value"
              :busy="
                teleportAction.preparing.value ||
                teleportAction.submitting.value
              "
              @submit="teleportAction.confirmReview"
              @cancel="teleportAction.cancelReview"
            />
            <template v-else>
              <button
                class="map-navigation-menu-action"
                type="button"
                :disabled="
                  teleportAction.preparing.value ||
                  teleportAction.submitting.value ||
                  teleportAction.counts.value.eligible === 0
                "
                @click="teleportAction.submit"
              >
                <UIcon name="i-lucide-signpost" />
                {{
                  teleportAction.preparing.value
                    ? 'Checking targets…'
                    : teleportAction.submitting.value
                      ? 'Submitting…'
                      : teleportAction.targetLabel.value
                }}
              </button>
              <p
                v-if="teleportAction.menuSummary.value"
                class="map-navigation-menu-summary"
                role="status"
              >
                {{ teleportAction.menuSummary.value }}
              </p>
            </template>
            <button
              class="map-text-action"
              type="button"
              @click="teleportAction.close()"
            >
              Cancel
            </button>
          </div>
          <div
            v-if="areaID !== 'world' && !canvasProfile"
            class="map-empty-view"
          >
            <UIcon name="i-lucide-map-pinned" />
            <strong>{{ profileArea?.label }} · {{ floorID }}</strong>
            <span>Floor imagery is unavailable for this dataset.</span>
          </div>
          <div
            v-else-if="mapProfile?.tiles.status !== 'available-for-inspection'"
            class="map-empty-view"
          >
            <UIcon name="i-lucide-map-off" />
            <strong>{{
              profileError || 'Map tiles are unavailable for this dataset.'
            }}</strong>
          </div>
          <div v-if="profileArea?.kind === 'cave'" class="map-floor-bar">
            <button class="compact-button" type="button" @click="returnToWorld">
              Back to world map
            </button>
          </div>
        </div>
        <section
          v-if="navigationTrayRows.length"
          class="panel map-navigation-tray"
          aria-label="Navigation tray"
        >
          <div class="map-list-heading map-tray-heading">
            <button
              class="compact-button"
              type="button"
              :aria-expanded="navigationTrayOpen && !navigationTrayCompact"
              aria-controls="map-tray-rows"
              @click="navigationTrayOpen = !navigationTrayOpen"
            >
              <UIcon name="i-lucide-chevron-down" />Navigation
              {{ navigationTrayRows.length }}
            </button>
            <span
              v-for="group in ['active', 'done', 'attention'] as const"
              v-show="navigationTrayRows.some((row) => row.group === group)"
              :key="group"
              >{{
                navigationTrayRows.filter((row) => row.group === group).length
              }}
              {{
                group === 'active'
                  ? 'en route'
                  : group === 'done'
                    ? 'arrived'
                    : 'need attention'
              }}</span
            >
            <button
              class="compact-button"
              type="button"
              @click="clearFinishedNavigation"
            >
              Clear finished
            </button>
            <small v-if="navigationTrayProgress" class="map-tray-progress">
              {{ navigationTrayProgress }}
            </small>
          </div>
          <div
            id="map-tray-rows"
            v-show="navigationTrayOpen && !navigationTrayCompact"
            class="map-tray-rows"
          >
            <div
              v-for="row in navigationTrayRows"
              :key="row.id"
              class="map-tray-row"
              :class="{
                selected: selectedNavigationRouteID === row.characterID,
              }"
            >
              <button
                class="compact-button map-tray-select"
                type="button"
                :aria-pressed="selectedNavigationRouteID === row.characterID"
                @click="
                  selectedNavigationRouteID =
                    selectedNavigationRouteID === row.characterID
                      ? ''
                      : row.characterID
                "
              >
                <strong>{{ row.name }}</strong
                ><span
                  :title="
                    mapNavigationStatusLabel(row.status, row.geometryCount)
                  "
                  >{{
                    row.status === 'submitting'
                      ? 'Submitting'
                      : [
                            'rejected',
                            'skipped',
                            'failed',
                            'expired',
                            'uncertain',
                            'unknown',
                          ].includes(row.status)
                        ? row.status
                        : mapNavigationStatusLabel(
                            row.status,
                            row.geometryCount,
                          ).split(' · ')[0]
                  }}</span
                ><small>{{ row.detail }}</small>
              </button>
              <button
                class="compact-button"
                type="button"
                :disabled="
                  !mapNavigationRoutes.some(
                    (route) =>
                      route.characterID === row.characterID &&
                      route.blocks.length,
                  )
                "
                @click="showNavigationOnMap(row.characterID)"
              >
                Show on map
              </button>
              <button
                v-if="row.group === 'active'"
                class="compact-button"
                type="button"
                :disabled="!row.canStop || row.status === 'stop_requested'"
                :title="
                  row.status === 'stop_requested'
                    ? 'Stop requested; waiting for the plugin result.'
                    : row.canStop
                      ? 'Stop the active navigation script for this character.'
                      : 'Stop is available once movement has started on a live route.'
                "
                @click="stopNavigationRow(row)"
              >
                {{ row.status === 'stop_requested' ? 'Stopping…' : 'Stop' }}
              </button>
              <button
                v-else
                class="compact-button"
                type="button"
                @click="dismissNavigationRow(row.id)"
              >
                Dismiss
              </button>
            </div>
            <p
              v-if="mapSnapshot?.navigation_omitted_count"
              class="map-empty-copy"
            >
              {{ mapSnapshot.navigation_omitted_count }} route record(s) omitted
              to keep the live map within its payload budget.
            </p>
          </div>
        </section>
      </section>

      <aside class="map-side-panel panel">
        <div
          class="map-panel-tabs"
          role="tablist"
          aria-label="Map panels"
          @keydown="moveSideTab"
        >
          <button
            v-for="tab in sideTabs"
            :id="`map-tab-${tab}`"
            :key="tab"
            class="map-panel-tab"
            :class="{ selected: sideTab === tab }"
            type="button"
            role="tab"
            :aria-selected="sideTab === tab"
            :aria-controls="`map-panel-${tab}`"
            :tabindex="sideTab === tab ? 0 : -1"
            @click="sideTab = tab"
          >
            {{
              tab === 'characters'
                ? 'Characters'
                : tab === 'activity'
                  ? 'Activity'
                  : 'Layers'
            }}
            <small>{{
              tab === 'characters'
                ? scopedCharacters.length
                : tab === 'activity'
                  ? activityEvents.length
                  : [
                      layerCharacters,
                      layerParty,
                      layerNPCs,
                      layerTraining,
                      layerMonsters,
                      layerDeaths,
                      layerDrops,
                    ].filter(Boolean).length + activeHistoricalLayers().length
            }}</small>
          </button>
        </div>
        <div class="map-panel-scroll">
          <div
            id="map-panel-characters"
            v-show="sideTab === 'characters'"
            class="map-tab-content"
            role="tabpanel"
            aria-labelledby="map-tab-characters"
          >
            <RemoteControlPanel
              variant="map"
              :trace-candidates="frozenTraceCandidates"
              :map-players="frozenMapPlayers"
              :selected-ids="[...actionTargetIDs]"
              :scope-key="remoteActionScopeKey"
              :scope-key-for-character="mapControlScopeKeyForCharacter"
              :current-scope-key="mapControlCurrentScopeKey"
              :current-character="mapControlCurrentCharacter"
              :map-snapshot-current="
                streamCurrent && mapSnapshotInFeedScope && !liveStale
              "
              @action-notification="showMapActionNotification"
              @refresh-nearby-players="refreshMapPlayers"
            />
            <section class="map-side-list map-character-list">
              <div class="map-target-toolbar">
                <button
                  v-for="group in mapTargetGroups"
                  :key="group.group_id"
                  class="compact-button map-group-chip"
                  :class="{ selected: group.state !== 'unchecked' }"
                  type="button"
                  role="checkbox"
                  :aria-checked="
                    group.state === 'indeterminate'
                      ? 'mixed'
                      : group.state === 'checked'
                  "
                  :aria-label="`Target group ${group.name} for actions`"
                  @click="toggleActionTargetGroup(group.memberIDs)"
                >
                  <UIcon
                    v-if="group.state !== 'unchecked'"
                    :name="
                      group.state === 'indeterminate'
                        ? 'i-lucide-minus'
                        : 'i-lucide-check'
                    "
                  />{{ group.name }} <small>{{ group.memberIDs.length }}</small>
                </button>
                <span class="map-target-quick"
                  ><button
                    type="button"
                    class="map-text-action"
                    :disabled="!applicableActionTargetIDs.size"
                    @click="selectAllActionTargets"
                  >
                    All</button
                  ><button
                    type="button"
                    class="map-text-action"
                    :disabled="!actionTargetIDs.size"
                    @click="clearActionTargets"
                  >
                    None
                  </button></span
                >
              </div>
              <MapCharacterStatusRow
                v-for="character in scopedCharacters"
                :key="character.character_id"
                :character="character"
                :selected="selectedCharacterID === character.character_id"
                :targeted="actionTargetIDs.has(character.character_id)"
                :position-fresh="positionIsFresh(character)"
                :focus-disabled="!canFocusCharacter(character)"
                :now="freshnessNow"
                :trace-lines="characterTraceLines(character)"
                @toggle-target="toggleActionTarget(character.character_id)"
                @select="selectMapCharacter(character.character_id)"
                @focus="focusMapCharacter(character.character_id)"
              />
              <p v-if="!scopedCharacters.length" class="map-empty-copy">
                No characters in this server and zone scope.
              </p>
            </section>
            <p v-if="outsideZoneCount" class="map-empty-copy">
              {{ outsideZoneCount }} online characters are outside
              {{ zoneNameForRegion(regionID) }}.
            </p>
            <details class="map-training-list">
              <summary>Training areas</summary>
              <div class="map-list-heading">
                <h2>Training areas</h2>
                <span>{{ renderedTrainingAreas.length }}</span>
              </div>
              <div
                v-if="renderedTrainingAreas.length"
                class="map-training-rows"
              >
                <button
                  v-for="overlay in renderedTrainingAreas"
                  :key="overlay.id"
                  class="map-navigation-route-row"
                  type="button"
                  :class="{ selected: overlay.selected }"
                  :aria-pressed="overlay.selected"
                  @click="selectTrainingArea(overlay.id)"
                >
                  <strong>{{ overlay.label }}</strong>
                  <span>{{ trainingAreaSummary(overlay.id) }}</span>
                  <small v-if="overlay.draft">Unsaved changes</small>
                </button>
              </div>
              <p v-if="!renderedTrainingAreas.length" class="map-empty-copy">
                {{ trainingEmptyCopy }}
              </p>
              <p
                v-if="mapSnapshot?.training_areas?.truncated"
                class="map-empty-copy"
                role="status"
              >
                Only the first training areas are shown to keep the live map
                within its payload budget.
              </p>
            </details>
            <button
              v-if="
                navigationAction.reviewOperation.value ||
                teleportAction.reviewOperation.value ||
                navigationAction.operations.value.length ||
                navigationAction.trainingOperations.value.length ||
                teleportAction.operations.value.length
              "
              class="compact-button"
              type="button"
              :aria-expanded="actionFeedbackOpen"
              @click="actionFeedbackOpen = !actionFeedbackOpen"
            >
              Action feedback
            </button>
            <div
              :ref="navigationAction.resultsElement"
              class="map-navigation-feedback"
              v-show="actionFeedbackOpen"
              tabindex="-1"
            >
              <CommandFanOutPreview
                v-if="navigationAction.reviewOperation.value"
                :operation="navigationAction.reviewOperation.value"
                :busy="
                  navigationAction.preparing.value ||
                  navigationAction.submitting.value ||
                  navigationAction.trainingPreparing.value ||
                  navigationAction.trainingSubmitting.value
                "
                :notice="navigationAction.notice.value"
                @submit="navigationAction.submitReviewed"
                @cancel="navigationAction.cancelReview"
              />
              <CommandFanOutPreview
                v-else-if="teleportAction.reviewOperation.value"
                :operation="teleportAction.reviewOperation.value"
                :busy="
                  teleportAction.preparing.value ||
                  teleportAction.submitting.value
                "
                @submit="teleportAction.confirmReview"
                @cancel="teleportAction.cancelReview"
              />
              <CommandFanOutResults
                v-for="operation in navigationAction.operations.value.filter(
                  (item) =>
                    item.state !== 'prepared' && item.state !== 'cancelled',
                )"
                :key="operation.operationID"
                :operation="operation"
                :status-note="navigationAction.resultStatusNote(operation)"
                :stale="
                  navigationAction.stale.value &&
                  operation.children.some((child) =>
                    ['accepted', 'uncertain'].includes(child.submission),
                  )
                "
                :on-retry="
                  (characterID: string) =>
                    navigationAction.retry(operation, characterID)
                "
                :on-dismiss="() => navigationAction.dismissResults(operation)"
              />
              <CommandFanOutResults
                v-for="operation in teleportAction.operations.value.filter(
                  (item) =>
                    item.state !== 'prepared' && item.state !== 'cancelled',
                )"
                :key="'teleport-' + operation.operationID"
                :operation="operation"
                status-note="Script acceptance does not prove arrival; check zone or position separately."
                :stale="false"
                :on-retry="
                  (characterID: string) =>
                    teleportAction.fanout.retrySubmission(
                      operation,
                      characterID,
                    )
                "
                :on-dismiss="() => teleportAction.fanout.dismiss(operation)"
              />
              <CommandFanOutResults
                v-for="operation in navigationAction.trainingOperations.value.filter(
                  (item) =>
                    item.state !== 'prepared' && item.state !== 'cancelled',
                )"
                :key="operation.operationID"
                :operation="operation"
                :status-note="
                  navigationAction.trainingResultStatusNote(operation)
                "
                :stale="
                  navigationAction.trainingStale.value &&
                  operation.children.some((child) =>
                    ['accepted', 'uncertain'].includes(child.submission),
                  )
                "
                :on-retry="
                  (characterID: string) =>
                    navigationAction.retry(operation, characterID)
                "
                :on-dismiss="() => navigationAction.dismissResults(operation)"
              />
            </div>
          </div>
          <div
            id="map-panel-activity"
            v-show="sideTab === 'activity'"
            class="map-tab-content"
            role="tabpanel"
            aria-labelledby="map-tab-activity"
          >
            <section class="map-side-list">
              <div class="map-list-heading">
                <h2>Nearby monsters</h2>
                <span>{{ currentMonsters.length }} seen now</span>
              </div>
              <div
                v-for="entry in groupedMonsters.slice(0, 30)"
                :key="`${entry.name}:${entry.type}`"
                class="map-observation-row"
              >
                <UIcon
                  :name="
                    entry.type.toLowerCase().includes('unique')
                      ? 'i-lucide-diamond'
                      : 'i-lucide-circle-small'
                  "
                /><span class="map-monster-name"
                  >{{ entry.name }} <small>×{{ entry.count }}</small></span
                ><small>Lv {{ entry.level ?? '—' }} · {{ entry.type }}</small>
              </div>
              <p v-if="!currentMonsters.length" class="map-empty-copy">
                {{
                  monsterSnapshots.length === 0
                    ? 'No current monster snapshot has arrived.'
                    : monsterSnapshotUnavailable
                      ? 'phBot monster data is unavailable; current monster markers are cleared.'
                      : monsterSnapshotTruncated
                        ? 'The current monster snapshot was truncated; its bounded observations are incomplete.'
                        : 'The current snapshot is observed empty. Historical density is a separate layer.'
                }}
              </p>
            </section>
            <section class="map-side-list">
              <div class="map-list-heading">
                <h2>Deaths &amp; drops</h2>
                <div
                  class="map-event-range"
                  role="group"
                  aria-label="Recent event time range"
                >
                  <button
                    v-for="range in ['1h', '24h', '7d']"
                    :key="range"
                    class="map-text-action"
                    :class="{ selected: dateRange === range }"
                    :aria-pressed="dateRange === range"
                    type="button"
                    @click="dateRange = range"
                  >
                    {{ range }}
                  </button>
                </div>
              </div>
              <div
                v-for="event in activityEvents.slice(0, 12)"
                :key="event.event_id"
                class="map-event-row"
              >
                <button
                  class="map-event-focus"
                  type="button"
                  @click="focusMapEvent(event)"
                >
                  <UIcon
                    :name="
                      event.kind === 'character.died'
                        ? 'i-lucide-skull'
                        : 'i-lucide-package'
                    "
                  /><span
                    ><strong>{{
                      event.kind === 'character.died'
                        ? `${event.character} died`
                        : `${event.kind === 'drop.rare' ? 'Rare drop' : 'Drop'}${event.item_name ? ` · ${event.item_name}` : ''}`
                    }}</strong
                    ><small
                      >{{ event.character }} ·
                      {{ zoneNameText(event.zone) }}</small
                    ></span
                  ><time
                    :datetime="event.occurred_at"
                    :title="formatTimestamp(event.occurred_at)"
                    >{{ eventAge(event.occurred_at) }}</time
                  >
                </button>
                <NuxtLink
                  :to="{ path: '/events', query: { kind: event.kind } }"
                  :aria-label="`Open ${event.kind} events`"
                  title="Open event history"
                  ><UIcon name="i-lucide-external-link"
                /></NuxtLink>
              </div>
              <p v-if="!activityEvents.length" class="map-empty-copy">
                No matching death or drop events in the selected time range.
              </p>
            </section>
          </div>
          <div
            id="map-panel-layers"
            v-show="sideTab === 'layers'"
            class="map-tab-content"
            role="tabpanel"
            aria-labelledby="map-tab-layers"
          >
            <section>
              <h2>On the map now</h2>
              <label class="map-layer-toggle"
                ><input
                  v-model="layerCharacters"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerCharacters"
                />
                Characters <span>{{ placedCharacterCount }}</span></label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerParty"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerParty"
                />
                Party members <span>{{ placedPartyCount }}</span></label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerPlayers"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerPlayers"
                  :disabled="otherPlayersLayerDisabled"
                />
                Other players <span>{{ placedPlayerCount }}</span>
                <span
                  v-if="otherPlayersStatus === 'unavailable'"
                  class="map-layer-status"
                  >unavailable</span
                >
                <span
                  v-else-if="otherPlayersStatus === 'truncated'"
                  class="map-layer-status"
                  >truncated</span
                ></label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerNPCs"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerNPCs"
                />
                NPCs <span>{{ placedNpcCount }}</span></label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerTraining"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerTraining"
                />
                Training areas
                <span>{{ renderedTrainingAreas.length }}</span></label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerMonsters"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerMonsters"
                />
                Nearby monsters
                <span>{{ currentMonsters.length }}</span></label
              >
              <label class="map-layer-toggle map-layer-toggle-subordinate"
                ><input
                  v-model="showNearbyMonsterNames"
                  type="checkbox"
                  role="switch"
                  :aria-checked="showNearbyMonsterNames"
                  :disabled="!layerMonsters"
                />
                Monster names</label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerDeaths"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerDeaths"
                />
                Recent deaths
                <span>{{
                  activityEvents.filter(
                    (event) => event.kind === 'character.died',
                  ).length
                }}</span></label
              >
              <label class="map-layer-toggle"
                ><input
                  v-model="layerDrops"
                  type="checkbox"
                  role="switch"
                  :aria-checked="layerDrops"
                />
                Recent drops
                <span>{{
                  activityEvents.filter((event) => event.category === 'drop')
                    .length
                }}</span></label
              >
            </section>
            <section class="heatmap-controls">
              <button
                class="map-list-heading map-section-toggle map-heat-heading"
                type="button"
                :aria-expanded="historicalHeatmapsOpen"
                aria-controls="historical-heatmap-controls"
                @click="historicalHeatmapsOpen = !historicalHeatmapsOpen"
              >
                <h2>Historical heatmaps</h2>
                <span class="map-heat-state">{{
                  activeHistoricalLayers().length
                    ? `${activeHistoricalLayers().length} on`
                    : 'Off'
                }}</span>
                <UIcon
                  name="i-lucide-chevron-down"
                  :class="{ expanded: historicalHeatmapsOpen }"
                  aria-hidden="true"
                />
              </button>
              <div
                id="historical-heatmap-controls"
                v-show="historicalHeatmapsOpen"
                class="heatmap-control-content"
              >
                <div class="heatmap-filter-row character-filters">
                  <label>
                    Range
                    <select
                      v-model="heatmapRange"
                      aria-label="Historical heatmap time range"
                    >
                      <option value="1h">Last hour</option>
                      <option value="24h">Last 24 hours</option>
                      <option value="7d">Last 7 days</option>
                      <option value="30d">Last 30 days</option>
                      <option value="custom">Custom</option>
                    </select>
                  </label>
                  <label>
                    Character
                    <select
                      v-model="analyticsCharacterID"
                      aria-label="Historical heatmap character"
                    >
                      <option value="">All characters</option>
                      <option
                        v-for="character in stableHistoricalCharacters"
                        :key="character.character_id"
                        :value="character.character_id"
                      >
                        {{ character.name }}
                      </option>
                    </select>
                  </label>
                </div>
                <div
                  v-if="heatmapRange === 'custom'"
                  class="heatmap-custom-range character-filters"
                >
                  <label
                    >From<input
                      v-model="heatmapCustomFrom"
                      type="datetime-local"
                      aria-label="Historical heatmap start"
                  /></label>
                  <label
                    >To<input
                      v-model="heatmapCustomTo"
                      type="datetime-local"
                      aria-label="Historical heatmap end"
                  /></label>
                </div>
                <label
                  class="character-filters"
                  v-if="
                    historicalLayers.mob_types ||
                    historicalLayers.mob_observer_average
                  "
                >
                  Monster rank
                  <select
                    v-model="analyticsMobType"
                    aria-label="Historical monster rank"
                  >
                    <option value="">All observed monster ranks</option>
                    <option
                      v-for="monsterRankValue in stableHistoricalMobTypes"
                      :key="monsterRankValue"
                      :value="monsterRankValue"
                    >
                      {{
                        monsterTypePresentation({ type: monsterRankValue })
                          .label
                      }}
                    </option>
                  </select>
                </label>
                <p v-if="heatmapFacetsLoading" class="map-empty-copy">
                  Loading observed monster ranks…
                </p>
                <p v-else-if="heatmapFacetsError" class="map-empty-copy">
                  {{ heatmapFacetsError }}
                </p>

                <div class="heatmap-layer-list">
                  <div v-for="layer in historicalLayerIDs" :key="layer">
                    <label class="map-layer-toggle"
                      ><input
                        v-model="historicalLayers[layer]"
                        type="checkbox"
                        role="switch"
                        :aria-checked="historicalLayers[layer]"
                      />{{
                        layer === 'mob_types'
                          ? 'Mob ranks'
                          : heatmapLayerLabel(layer)
                      }}<span>{{
                        heatmapResults[layer]?.points.length || 0
                      }}</span></label
                    >
                    <div v-if="historicalLayers[layer]">
                      <p v-if="heatmapLoading[layer]" class="map-empty-copy">
                        Refreshing {{ heatmapLayerLabel(layer) }}…
                      </p>
                      <p
                        v-else-if="heatmapErrors[layer]"
                        class="heatmap-warning"
                      >
                        {{ heatmapLayerLabel(layer) }}:
                        {{ heatmapErrors[layer] }}
                      </p>
                      <p
                        v-else-if="
                          heatmapResults[layer]?.status === 'unsupported'
                        "
                        class="heatmap-warning"
                      >
                        {{ heatmapLayerLabel(layer) }}:
                        {{ heatmapResults[layer]?.interpretation }}
                      </p>
                      <p
                        v-else-if="heatmapResults[layer]?.status === 'limited'"
                        class="heatmap-warning"
                      >
                        {{ heatmapResults[layer]?.interpretation }}
                      </p>
                      <p
                        v-else-if="
                          heatmapResults[layer] &&
                          heatmapResults[layer]?.points.length === 0
                        "
                        class="map-empty-copy"
                      >
                        No {{ heatmapLayerLabel(layer).toLowerCase() }} data in
                        this scope.
                      </p>
                      <p
                        v-if="heatmapResults[layer]?.truncated"
                        class="heatmap-warning"
                      >
                        {{ heatmapLayerLabel(layer) }} reached the bounded
                        result limit.
                      </p>
                    </div>
                  </div>
                </div>
                <div
                  v-if="renderedHeatLayers.length"
                  class="heatmap-legend"
                  aria-label="Heatmap legend"
                >
                  <div
                    v-for="layer in renderedHeatLayers"
                    :key="`legend-${layer.id}`"
                  >
                    <strong>{{ layer.label }}</strong>
                    <span>{{ layer.metric }} · low → high</span>
                  </div>
                </div>
              </div>
              <div class="heatmap-footer">
                <p
                  class="map-empty-copy map-unavailable-layers"
                  title="Mob density: observation coverage is not verified. Academy members: region and floor are unavailable from the documented API."
                >
                  <UIcon name="i-lucide-info" />2 layers unavailable
                </p>
                <button
                  class="compact-button map-heatmap-reset"
                  type="button"
                  aria-label="Reset selected historical heatmap"
                  title="Reset selected historical heatmap"
                  :disabled="!resettableLayers.length"
                  @click="openHeatmapReset"
                >
                  Reset…
                </button>
              </div>
            </section>
          </div>
        </div>
      </aside>
    </div>

    <div
      v-if="resetOpen"
      class="heatmap-reset-backdrop"
      @click.self="resetOpen = false"
    >
      <section
        class="panel heatmap-reset-dialog"
        role="dialog"
        aria-modal="true"
        aria-labelledby="heatmap-reset-title"
      >
        <h2 id="heatmap-reset-title">Reset historical heatmap</h2>
        <label>
          Layer
          <select v-model="resetLayer">
            <option
              v-for="layer in resettableLayers"
              :key="layer"
              :value="layer"
            >
              {{ heatmapLayerLabel(layer) }}
            </option>
          </select>
        </label>
        <dl>
          <div>
            <dt>Server</dt>
            <dd>{{ server }}</dd>
          </div>
          <div>
            <dt>Area / floor</dt>
            <dd>{{ areaID }} / {{ floorID }}</dd>
          </div>
          <div>
            <dt>Zone</dt>
            <dd>
              {{
                regionID
                  ? (zoneOptionLabels.get(regionID) ??
                    zoneNameForRegion(regionID))
                  : 'All zones'
              }}
            </dd>
          </div>
          <div>
            <dt>Character</dt>
            <dd>{{ analyticsCharacterID || 'All characters' }}</dd>
          </div>
          <div
            v-if="
              resetLayer === 'mob_types' ||
              resetLayer === 'mob_observer_average'
            "
          >
            <dt>Monster rank</dt>
            <dd>
              {{
                analyticsMobType
                  ? monsterTypePresentation({ type: analyticsMobType }).label
                  : 'All observed monster ranks'
              }}
            </dd>
          </div>
          <div>
            <dt>Time</dt>
            <dd>{{ resetWindow?.from }} → {{ resetWindow?.to }}</dd>
          </div>
        </dl>
        <p>
          This clears this historical heatmap projection only. Canonical event,
          movement and mob-observation history is preserved.
        </p>
        <label v-if="resetIsBroad" class="heatmap-broad-confirm">
          <input v-model="confirmBroadReset" type="checkbox" />
          Confirm this broader all-region/all-character reset scope.
        </label>
        <p v-if="resetError" class="heatmap-warning">{{ resetError }}</p>
        <div class="heatmap-reset-actions">
          <button
            class="compact-button"
            type="button"
            @click="resetOpen = false"
          >
            Cancel
          </button>
          <button
            class="compact-button"
            type="button"
            :disabled="resetBusy || (resetIsBroad && !confirmBroadReset)"
            @click="performHeatmapReset"
          >
            {{ resetBusy ? 'Resetting…' : 'Confirm reset' }}
          </button>
        </div>
      </section>
    </div>
  </section>
</template>

<style scoped>
.map-event-range {
  display: flex;
  gap: 4px;
}
.map-event-focus {
  display: flex;
  align-items: center;
  gap: 6px;
  flex: 1;
  min-width: 0;
  text-align: left;
}
.map-event-focus > span {
  display: flex;
  flex-direction: column;
  min-width: 0;
}
.map-event-focus time {
  margin-left: auto;
}

.map-navigation-tray {
  flex: none;
  padding: 8px;
}
.map-tray-heading,
.map-tray-row {
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 6px;
}
.map-tray-progress {
  flex-basis: 100%;
  color: #9eb4d0;
}
.map-tray-rows {
  max-height: 120px;
  overflow: auto;
}
.map-tray-select {
  display: flex;
  flex: 1;
  min-width: 160px;
  flex-wrap: wrap;
  gap: 6px;
  text-align: left;
}
.map-tray-select small {
  flex-basis: 100%;
}

.map-inspector {
  position: absolute;
  bottom: 12px;
  left: 12px;
  z-index: 900;
  width: min(320px, calc(100% - 24px));
  max-height: calc(100% - 24px);
  overflow: auto;
  padding: 12px;
  display: grid;
  gap: 8px;
}
.map-inspector h2,
.map-inspector p {
  margin: 0;
}

.map-page {
  display: flex;
  min-height: 0;
  height: 100%;
  flex-direction: column;
}
.map-header-row {
  flex: none;
  flex-wrap: wrap;
  margin-bottom: 8px;
  gap: 8px;
}
.map-header-row h1 {
  margin: 0;
}
.map-header-row label {
  display: flex;
  align-items: center;
  gap: 4px;
  min-width: 0;
}
.map-header-row select {
  max-width: 170px;
  min-width: 0;
}
.map-header-floors,
.map-panel-tabs {
  display: flex;
  gap: 4px;
}
.map-workspace {
  flex: 1;
  min-height: 0;
  grid-template-columns: minmax(0, 1fr) 300px;
  align-items: stretch;
}
.map-viewport-panel {
  display: flex;
  flex-direction: column;
  min-height: 0;
}
.map-canvas-frame {
  flex: 1;
  height: auto;
  min-height: 300px;
}
.map-side-panel {
  display: flex;
  flex-direction: column;
  align-items: stretch;
  min-height: 0;
  padding: 8px;
}
.map-panel-tabs {
  flex: none;
}
.map-panel-tabs button {
  flex: 1;
  min-width: 0;
}
.map-panel-scroll {
  flex: 1;
  min-height: 0;
  overflow: auto;
}
.map-tab-content {
  display: grid;
  gap: 12px;
  padding-top: 8px;
}
.map-header-row .map-go-to {
  margin-left: auto;
}
.map-header-row .map-page-status {
  margin-left: 0;
}
@media (max-width: 899px) {
  .map-page {
    height: auto;
  }
  .map-workspace {
    grid-template-columns: minmax(0, 1fr);
  }
  .map-canvas-frame {
    height: 55dvh;
    min-height: 300px;
    flex: none;
  }
  .map-panel-scroll {
    max-height: 60dvh;
  }
  .map-header-row .map-go-to {
    margin-left: 0;
  }
}

.map-teleport-destination {
  display: grid;
  gap: 4px;
  padding: 6px 9px;
  font-size: 12px;
  color: var(--ph-muted);
}

.map-teleport-destination-label {
  font-size: 12px;
  color: var(--ph-muted);
}

.map-teleport-destination-field {
  display: grid;
  gap: 4px;
}

.map-teleport-route-list {
  display: grid;
  gap: 2px;
  margin: 0;
  padding: 0;
  list-style: none;
  max-height: 180px;
  overflow: auto;
}

.map-teleport-route-button {
  width: 100%;
  min-height: 30px;
  padding: 4px 8px;
  border: 1px solid #3a4d63;
  border-radius: 3px;
  background: #121a26;
  color: var(--ph-text);
  font-size: 13px;
  text-align: left;
  cursor: pointer;
}

.map-teleport-route-button.is-selected,
.map-teleport-route-button:hover {
  border-color: #6f8eb8;
  background: #1a2838;
}

.map-teleport-route-note {
  margin: 0;
  font-size: 11px;
  line-height: 1.35;
  color: #8fa3bc;
}

.map-teleport-destination-other span {
  font-size: 11px;
}

.map-teleport-destination input {
  width: 100%;
  min-height: 32px;
  padding: 4px 8px;
  border: 1px solid #495b6f;
  border-radius: 3px;
  background: #0a1018;
  color: var(--ph-text);
  font-size: 13px;
}

.map-context-submenu-chevron {
  width: 14px;
  height: 14px;
  margin-left: auto;
  flex-shrink: 0;
  opacity: 0.75;
}

.map-teleport-flyout-panel .map-navigation-menu-action {
  width: max-content;
  min-width: 100%;
  white-space: nowrap;
}

.map-teleport-flyout-panel {
  position: fixed;
  z-index: 1500;
  width: max-content;
  min-width: 180px;
  max-width: min(420px, calc(100vw - 16px));
  max-height: min(70vh, 420px);
  overflow: auto;
  padding: 5px;
  border: 1px solid #495b6f;
  border-radius: 5px;
  background: #0d131df5;
  color: #eaf1ff;
  box-shadow: 0 8px 28px #000b;
}

.map-navigation-context.map-teleport-flyout {
  z-index: 1500;
  overflow: visible;
  max-height: none;
}

.map-navigation-context {
  position: fixed;
  z-index: 1400;
  width: max-content;
  min-width: 200px;
  max-width: min(280px, calc(100vw - 16px));
  max-height: calc(100vh - 16px);
  overflow: auto;
  padding: 5px;
  border: 1px solid #495b6f;
  border-radius: 5px;
  background: #0d131df5;
  color: #eaf1ff;
  box-shadow: 0 8px 28px #000b;
}

.map-navigation-menu-action {
  display: flex;
  align-items: center;
  gap: 8px;
  width: 100%;
  min-height: 36px;
  padding: 6px 9px;
  border: 0;
  border-radius: 3px;
  background: transparent;
  color: var(--ph-text);
  text-align: left;
  font-size: 13px;
  cursor: pointer;
}

.map-navigation-menu-action:hover:not(:disabled) {
  background: #28394d;
}

.map-navigation-menu-action:disabled {
  color: var(--ph-muted);
  cursor: default;
}

.map-navigation-menu-summary {
  margin: 0;
  padding: 3px 9px 6px;
  color: var(--ph-muted);
  font-size: 12px;
  overflow-wrap: anywhere;
}

.map-navigation-feedback {
  display: grid;
  gap: 8px;
  width: 100%;
  min-width: 0;
}

.map-navigation-route-list {
  display: grid;
  gap: 6px;
}

.map-navigation-route-row {
  display: grid;
  gap: 3px;
  width: 100%;
  padding: 7px;
  border: 1px solid #334356;
  border-radius: 4px;
  background: #111923;
  color: #eaf1ff;
  text-align: left;
  cursor: pointer;
}

.map-navigation-route-row.selected {
  border-color: #42cdd0;
  background: #143039;
}

.map-navigation-route-row.dimmed {
  opacity: 0.45;
}

.map-navigation-route-row strong {
  font-size: 12px;
}

.map-navigation-route-row span,
.map-navigation-route-row small {
  color: #abb9c8;
  font-size: 11px;
  overflow-wrap: anywhere;
}

.map-navigation-route-row:focus-visible,
.map-route-clear-selection:focus-visible,
.map-navigation-context button:focus-visible {
  outline: 2px solid var(--ph-primary);
  outline-offset: 2px;
}

.map-route-clear-selection {
  justify-self: start;
}

.map-training-list,
.map-training-rows {
  display: grid;
  gap: 6px;
}

.map-training-rows {
  max-height: 220px;
  overflow: auto;
}

.map-training-list .map-navigation-route-row.selected {
  border-color: #4db9ff;
  background: #122a40;
}

.map-training-list .map-navigation-route-row small {
  color: var(--ph-primary);
}

.map-training-editor {
  display: grid;
  gap: 7px;
  padding: 8px;
  border: 1px solid #2c4a66;
  border-radius: 4px;
  background: #0d1a28;
}

.map-training-editor p {
  margin: 0;
  font-size: 12px;
}

.map-training-readback {
  color: #abb9c8;
}

.map-training-draft {
  color: var(--ph-primary);
}

.map-training-editor-controls,
.map-training-editor-actions {
  display: flex;
  flex-wrap: wrap;
  align-items: center;
  gap: 8px;
}

.map-training-radius {
  display: inline-flex;
  align-items: center;
  gap: 6px;
  font-size: 12px;
}

.map-training-radius input {
  width: 84px;
  min-height: 28px;
  padding: 2px 6px;
  border: 1px solid #334356;
  border-radius: 4px;
  background: #0b121b;
  color: #eaf1ff;
}

.map-training-results {
  display: grid;
  gap: 3px;
  margin: 0;
  padding: 0;
  list-style: none;
  font-size: 11px;
  color: #abb9c8;
}

.map-training-results .outcome-completed strong {
  color: #58bd8a;
}

.map-training-results li:not(.outcome-completed) strong {
  color: #e7a15b;
}

.map-navigation-feedback > :deep(.fanout-results) {
  grid-column: 1 / -1;
}

.heatmap-controls {
  display: grid;
  gap: 10px;
}

.heatmap-control-content {
  display: grid;
  gap: 10px;
}

.map-section-toggle {
  display: flex;
  width: 100%;
  align-items: center;
  gap: 0.4rem;
  padding: 0.4rem 0.25rem;
  border: 0;
  background: #111923;
  color: #eaf1ff;
  text-align: left;
  cursor: pointer;
}

.map-section-toggle h2 {
  margin: 0;
}

.map-section-toggle > span {
  margin-left: auto;
  color: #8796aa;
  font-size: 0.68rem;
}

.map-section-toggle :deep(.i-lucide-chevron-down) {
  transition: transform 140ms ease;
}

.map-section-toggle :deep(.expanded) {
  transform: rotate(180deg);
}

.map-section-toggle:focus-visible {
  outline: 2px solid #8ba8d0;
  outline-offset: 2px;
}

.heatmap-control-content > label:not(.map-layer-toggle),
.heatmap-filter-row label,
.heatmap-custom-range label {
  display: grid;
  min-width: 0;
  gap: 4px;
  color: var(--ph-muted);
  font-size: 12px;
}

.heatmap-filter-row {
  display: grid;
  grid-template-columns: repeat(2, minmax(0, 1fr));
  gap: 8px;
}

.heatmap-control-content select,
.heatmap-control-content input:not([role='switch']) {
  width: 100%;
  min-width: 0;
  max-width: none;
  font-size: 12px;
}

.heatmap-layer-list {
  display: grid;
}

.heatmap-layer-list .map-layer-toggle {
  min-height: 36px;
}

.heatmap-footer {
  display: flex;
  align-items: center;
  justify-content: space-between;
  gap: 8px;
}

.heatmap-footer .map-unavailable-layers {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  margin: 0;
}

.map-heatmap-reset {
  flex: none;
  min-height: 24px;
  padding: 0 8px;
}

.heatmap-custom-range {
  display: grid;
  gap: 8px;
  grid-template-columns: 1fr 1fr;
}

.heatmap-warning {
  margin: 0;
  color: #d6b979;
  font-size: 12px;
  line-height: 1.4;
}

.heatmap-legend {
  display: grid;
  gap: 6px;
  padding: 8px;
  border: 1px solid #394553;
  border-radius: 6px;
}

.heatmap-legend div {
  display: flex;
  justify-content: space-between;
  gap: 8px;
}

.heatmap-legend span {
  color: #aeb9c5;
  font-size: 11px;
}

.heatmap-reset-backdrop {
  position: fixed;
  inset: 0;
  z-index: 3000;
  display: grid;
  place-items: center;
  padding: 16px;
  background: #05080dbf;
}

.heatmap-reset-dialog {
  width: min(560px, 100%);
  display: grid;
  gap: 12px;
}

.heatmap-reset-dialog dl {
  display: grid;
  gap: 6px;
  margin: 0;
}

.heatmap-reset-dialog dl div {
  display: grid;
  grid-template-columns: 120px 1fr;
  gap: 8px;
}

.heatmap-reset-dialog dt {
  color: #aeb9c5;
}

.heatmap-reset-dialog dd {
  margin: 0;
  overflow-wrap: anywhere;
}

.heatmap-broad-confirm {
  display: flex;
  gap: 8px;
  align-items: flex-start;
}

.heatmap-reset-actions {
  display: flex;
  justify-content: flex-end;
  gap: 8px;
}

@media (max-width: 640px) {
  .heatmap-custom-range {
    grid-template-columns: 1fr;
  }

  .heatmap-reset-dialog dl div {
    grid-template-columns: 1fr;
    gap: 2px;
  }
}
.map-layer-toggle-subordinate {
  padding-left: 1.15rem;
}

.map-page {
  gap: 0;
}
.map-side-panel {
  max-height: none;
  overflow: hidden;
  padding: 0 12px 12px;
  gap: 0;
}
.map-panel-tabs {
  gap: 0;
}
.map-panel-tab {
  flex: 1;
  min-width: 0;
  display: flex;
  justify-content: center;
  align-items: center;
  gap: 5px;
  height: 42px;
  padding: 0 4px;
  border: 0;
  border-bottom: 2px solid transparent;
  background: transparent;
  color: var(--ph-muted);
  font-size: 12px;
  cursor: pointer;
}
.map-panel-tab.selected {
  border-bottom-color: var(--ph-blue);
  color: var(--ph-text);
}
.map-panel-tab small {
  font-size: 10px;
  color: var(--ph-muted);
}
.map-tab-content {
  gap: 24px;
  padding-top: 14px;
}
.map-side-panel .map-side-list {
  max-height: none;
  min-height: 0;
  overflow: visible;
}
.map-side-panel section {
  border: 0;
  padding-right: 0;
  padding-bottom: 0;
}
.map-side-panel h2 {
  margin: 0;
  color: var(--ph-muted);
  font-size: 12px;
  font-weight: 600;
  text-transform: uppercase;
  letter-spacing: 0.06em;
}
.map-side-panel .map-list-heading {
  position: static;
  margin-bottom: 10px;
  background: transparent;
}
.map-list-heading span {
  font-size: 11px;
}
.map-layer-toggle {
  gap: 9px;
  min-height: 32px;
  font-size: 13px;
}
.map-layer-toggle > span {
  order: 2;
  font-size: 11px;
}
.map-layer-toggle > input[role='switch'] {
  order: 3;
  appearance: none;
  position: relative;
  width: 26px;
  height: 14px;
  flex: none;
  margin: 0;
  border: 1px solid var(--ph-border);
  border-radius: 8px;
  background: var(--ph-panel);
  cursor: pointer;
}
.map-layer-toggle > input[role='switch']::after {
  content: '';
  position: absolute;
  top: 2px;
  left: 2px;
  width: 8px;
  height: 8px;
  border-radius: 50%;
  background: var(--ph-muted);
  transition: transform 120ms ease;
}
.map-layer-toggle > input[role='switch']:checked {
  background: var(--ph-active);
  border-color: var(--ph-blue);
}
.map-layer-toggle > input[role='switch']:checked::after {
  transform: translateX(12px);
  background: var(--ph-blue);
}
.map-layer-toggle > input[role='switch']:disabled {
  cursor: default;
  opacity: 0.5;
}
.map-layer-toggle-subordinate {
  padding-left: 16px;
}
.map-layer-toggle-subordinate input {
  margin-left: auto !important;
}
.map-heat-heading {
  padding: 0;
  border: 0;
  background: transparent;
}
.heatmap-controls .map-heat-heading {
  margin-bottom: 0;
}
.map-heat-heading > :last-child {
  flex: none;
  margin-left: 0;
}
.map-unavailable-layers {
  margin-top: 16px;
  font-size: 11px;
}
.map-target-toolbar {
  gap: 6px;
  padding: 0 0 10px;
  border-bottom: 1px solid var(--ph-border);
}
.map-target-quick {
  margin-left: auto;
  display: flex;
  gap: 12px;
}
.map-group-chip {
  font-size: 12px;
}
.map-group-chip small {
  color: var(--ph-muted);
  font-size: 10px;
}
.map-text-action {
  border: 0;
  padding: 3px 5px;
  background: transparent;
  color: var(--ph-blue);
  font-size: 12px;
  cursor: pointer;
}
.map-text-action:disabled {
  color: var(--ph-muted);
  cursor: default;
}
.map-event-range {
  margin-left: auto;
  gap: 2px;
}
.map-event-range .map-text-action {
  font-size: 11px;
  color: var(--ph-muted);
}
.map-event-range .selected {
  background: var(--ph-active);
  color: var(--ph-text);
}
.map-event-row,
.map-observation-row {
  border: 0;
  padding: 8px 2px;
  font-size: 13px;
}
.map-observation-row {
  gap: 8px;
}
.map-observation-row .map-monster-name {
  display: block;
  flex: 1;
}
.map-observation-row small {
  font-size: 11px;
}
.map-event-focus {
  width: 100%;
  padding: 0;
  border: 0;
  background: transparent;
  color: var(--ph-text);
  font-size: 13px;
  cursor: pointer;
}
.map-event-focus strong {
  font-weight: 400;
}
.map-event-focus > .iconify {
  flex: none;
}
.map-event-focus small,
.map-event-focus time {
  font-size: 11px;
}
.map-event-focus time {
  flex: none;
}
.map-event-row > a {
  flex: none;
  color: var(--ph-muted);
}
.map-training-list {
  font-size: 12px;
  color: var(--ph-muted);
}

.map-target-bar {
  position: absolute;
  top: 10px;
  left: 50%;
  transform: translateX(-50%);
  z-index: 850;
  display: flex;
  align-items: center;
  flex-wrap: wrap;
  gap: 8px;
  padding: 5px 10px;
  max-width: calc(100% - 100px);
  font-size: 12px;
}
.map-floating-controls {
  position: absolute;
  top: 10px;
  right: 10px;
  z-index: 900;
  display: grid;
  gap: 4px;
}
.map-floating-controls .compact-button {
  justify-content: center;
  width: 30px;
  height: 30px;
  padding: 0;
}
.map-floating-legend {
  position: absolute;
  top: 150px;
  right: 10px;
  z-index: 1000;
  width: min(300px, calc(100% - 24px));
  padding: 12px;
  font-size: 12px;
  max-height: calc(100% - 170px);
  overflow: auto;
}
.map-corner-readouts {
  position: absolute;
  bottom: 8px;
  right: 10px;
  z-index: 800;
}
.map-floating-notice {
  position: absolute;
  top: 10px;
  left: 10px;
  z-index: 850;
  max-width: calc(100% - 70px);
  padding: 8px;
  font-size: 12px;
}
.map-context-heading {
  display: grid;
  gap: 4px;
  padding: 8px 10px;
}
.map-context-heading small {
  color: var(--ph-muted);
}
.map-floor-bar {
  position: absolute;
  top: 10px;
  left: 10px;
  z-index: 850;
  padding: 0;
  background: transparent;
  border: 0;
}
.map-canvas-frame {
  min-height: 300px;
}
.map-panel-scroll {
  overflow-x: hidden;
}
@media (max-width: 899px) {
  .map-side-panel {
    min-height: 360px;
    max-height: none;
  }
  .map-target-bar {
    left: 10px;
    transform: none;
  }
}

.map-corner-readouts {
  padding: 2px 5px;
  background: var(--ph-panel-soft);
  font-size: 11px;
}
</style>
