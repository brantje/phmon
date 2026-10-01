<script setup lang="ts">
import type {
  ActivityEvent,
  CharacterView,
  MapMonster,
  MapNpc,
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
import { npcMapMarkers } from '~/utils/mapNpcMarkers'
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
import { useMapNavigationAction } from '~/composables/useMapNavigationAction'
import { useMapTrainingEditor } from '~/composables/useMapTrainingEditor'
import {
  TRAINING_RADIUS_MAX,
  TRAINING_RADIUS_MIN,
  trainingAreaOverlays,
} from '~/utils/mapTrainingAreas'

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
} = useLiveData()
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
const selectedNavigationRouteID = ref('')
const actionTargetScopeKey = computed(() =>
  mapActionTargetScopeKey({
    server: server.value,
    area: areaID.value,
    floor: floorID.value,
    region: regionID.value,
  }),
)
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
const selectedTile = ref<RasterPosition | null>(null)
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
function selectMapPoint(point: RasterPosition, trainingAreaID?: string | null) {
  if (trainingEditor.moveArmed.value) {
    trainingEditor.moveCenter(trainingEditor.selectedID.value, point)
    return
  }
  if (trainingAreaID) selectTrainingArea(trainingAreaID)
  else if (trainingAreaID === null && trainingEditor.selectedID.value)
    trainingEditor.select('')
  selectedTile.value = point
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
const currentCharacterPositionFresh = computed(() => {
  return positionIsFresh(currentCharacter.value)
})
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
const characterPlacement = computed(() =>
  exactCharacterRasterPosition.value
    ? 'exact'
    : characterRasterPosition.value
      ? 'region-tile'
      : null,
)
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
const selectedGamePosition = computed(() => {
  if (!mapProfile.value || !selectedTile.value) return null
  return rasterPositionToGame(
    mapProfile.value,
    areaID.value,
    floorID.value,
    currentRegion.value,
    selectedTile.value,
    currentCharacter.value?.z ?? 0,
  )
})
const jumpAvailable = computed(() =>
  Boolean(
    currentCharacter.value &&
    positionCanBeDisplayed(currentCharacter.value) &&
    mapProfile.value,
  ),
)
const selectedRegionAmbiguous = computed(() => {
  const floor = profileArea.value?.floors.find(
    (item) => item.id === floorID.value,
  )
  return Boolean(
    selectedTile.value &&
    floor?.region_ids &&
    floor.region_ids.length > 1 &&
    !floor.region_ids.includes(currentRegion.value ?? 0),
  )
})
function openSelectedNavigation(
  event: MouseEvent,
  focusAction: 'navigate' | 'training' = 'navigate',
) {
  if (!selectedTile.value) return
  const rect = (event.currentTarget as HTMLElement).getBoundingClientRect()
  void navigationAction.open(
    selectedTile.value,
    { x: rect.left, y: rect.bottom },
    event.currentTarget as HTMLElement,
    focusAction,
  )
}
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
  void navigationAction.open(
    action.point,
    action.anchor,
    document.querySelector<HTMLElement>('.map-canvas'),
  )
}
function openNpcNavigation(
  point: RasterPosition,
  anchor: { x: number; y: number },
) {
  void navigationAction.open(
    point,
    anchor,
    document.querySelector<HTMLElement>('.map-canvas'),
  )
}
const scopedCharacters = computed(() => {
  const items = mapSnapshotInFeedScope.value
    ? mapSnapshot.value?.characters || []
    : []
  if (regionID.value !== 0)
    return items.filter((character) => character.region === regionID.value)
  return items
})
const applicableActionTargetIDs = computed(
  () =>
    new Set(scopedCharacters.value.map((character) => character.character_id)),
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
  if (!layerMonsters.value) return []
  return dedupeCurrentMonsters(mapSnapshot.value?.monsters || [])
})
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
    kind: 'character' | 'party' | 'npc' | 'monster' | 'death' | 'drop' | 'event'
    position: RasterPosition
    placement?: 'exact' | 'region-tile'
    selected?: boolean
    party?: MapPartyMember
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
        { monster: entry, showLabel: showNearbyMonsterNames.value },
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
      { itemName: event.item_name, itemIconUrl: event.item_icon_url, event },
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

function updateRouteQuery() {
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
  void router.replace({ path: '/map', query })
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

function jumpToCharacter() {
  const character = currentCharacter.value
  const profile = mapProfile.value
  if (!character || !profile) return
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
  nextTick(() => {
    jumpSequence.value++
  })
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
watch(selectedCharacterID, (characterID) => {
  jumpSequence.value++
  if (characterID) selectedDestinationID.value = ''
  const selected = mapSnapshot.value?.characters.find(
    (character) => character.character_id === characterID,
  )
  if (selected && regionID.value && regionID.value !== selected.region)
    regionID.value = 0
})
watch(currentCharacter, (character) => {
  if (character && regionID.value && regionID.value !== character.region)
    regionID.value = 0
})
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
watch([server, areaID, floorID, regionID, selectedCharacterID], () => {
  selectedTile.value = null
})
watch(mapProfile, (profile) => {
  if (!profile) return
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
  profileRequestID++
  clearMapFeed(subscriptionID)
})

useHead({ title: 'Map · PhMon' })
</script>

<template>
  <section class="map-page">
    <header class="page-header map-page-header">
      <div>
        <h1><UIcon name="i-lucide-map" /> Map</h1>
        <p>
          Live positions, nearby monster observations and recent event
          locations.
        </p>
      </div>
      <div class="map-page-status" :class="streamCurrent ? 'current' : 'stale'">
        <span />{{
          streamCurrent
            ? 'Live scope current'
            : connectionState === 'stale'
              ? 'Showing stale map data'
              : 'Syncing map data'
        }}
      </div>
    </header>

    <div class="map-toolbar panel">
      <label>
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
          <option v-for="option in serverOptions" :key="option" :value="option">
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
            v-for="area in mapProfile?.areas || []"
            :key="area.id"
            :value="area.id"
          >
            {{ area.label }}
          </option>
        </select>
      </label>
      <label v-if="profileArea?.kind === 'cave'">
        Floor
        <select v-model="floorID" aria-label="Cave map floor">
          <option
            v-for="floor in profileArea.floors"
            :key="floor.id"
            :value="floor.id"
          >
            {{ floor.label }}
          </option>
        </select>
      </label>
      <label>
        Zone
        <select v-model.number="regionID" aria-label="Filter map data by zone">
          <option :value="0">All zones</option>
          <option v-for="region in regionOptions" :key="region" :value="region">
            {{ zoneOptionLabels.get(region) }}
          </option>
        </select>
      </label>
      <label>
        Quick destination
        <select
          :value="selectedDestinationID"
          aria-label="Quick destination"
          :disabled="!mapProfile"
          @change="
            selectQuickDestination(($event.target as HTMLSelectElement).value)
          "
        >
          <option value="">
            {{
              mapProfile?.quick_destinations.length
                ? 'Select destination'
                : 'No verified destinations'
            }}
          </option>
          <option
            v-for="destination in mapProfile?.quick_destinations.filter(
              (item) => item.status === 'validated',
            ) || []"
            :key="destination.id"
            :value="destination.id"
          >
            {{ destination.label }}
          </option>
          <optgroup
            v-for="area in mapProfile?.areas.filter(
              (item) => item.kind === 'cave',
            ) || []"
            :key="area.id"
            :label="area.label"
          >
            <option
              v-for="floor in area.floors"
              :key="floor.id"
              :value="`floor:${area.id}:${floor.id}`"
            >
              {{ area.label }} · {{ floor.label }}
            </option>
          </optgroup>
        </select>
      </label>
      <label>
        Event range
        <select v-model="dateRange" aria-label="Recent event time range">
          <option value="1h">Last hour</option>
          <option value="24h">Last 24 hours</option>
          <option value="7d">Last 7 days</option>
        </select>
      </label>
      <label>
        Character
        <select v-model="selectedCharacterID" aria-label="Select character">
          <option value="">Choose character</option>
          <option
            v-for="character in fleetCharacters.filter(
              (item) => item.server.toLowerCase() === server.toLowerCase(),
            )"
            :key="character.character_id"
            :value="character.character_id"
          >
            {{ character.name }} · {{ character.server }}
          </option>
        </select>
      </label>
      <button
        class="compact-button"
        type="button"
        :disabled="!jumpAvailable"
        :title="
          characterPlacement === 'exact'
            ? 'Jump to the selected character position.'
            : characterPlacement === 'region-tile'
              ? 'Jump to the character’s encoded outdoor region tile.'
              : 'No outdoor tile is available for this character.'
        "
        @click="jumpToCharacter"
      >
        {{
          characterPlacement === 'region-tile'
            ? 'Jump to region tile'
            : 'Jump to character'
        }}
      </button>
    </div>

    <div class="map-workspace">
      <section
        class="map-viewport-panel panel"
        aria-label="Game world raster map"
      >
        <div class="map-viewport-header">
          <div>
            <strong>{{ profileArea?.label || 'World map' }}</strong>
            <span>{{
              mapProfile?.dataset_id ||
              (profileLoading ? 'Loading profile…' : 'Profile unavailable')
            }}</span>
          </div>
          <div class="map-readouts" aria-live="polite">
            <span>Tile {{ mapView.tileX }} × {{ mapView.tileY }}</span>
            <span>Zoom {{ mapView.zoomPercent }}%</span>
            <span>{{
              selectedCharacterID
                ? `${currentCharacterPositionFresh ? '' : 'Stale · '}${characterLocation(currentCharacter)}`
                : 'Select a character for coordinates'
            }}</span>
          </div>
        </div>
        <div v-if="linkedEventMessage" class="map-link-status" role="status">
          {{ linkedEventMessage }}
        </div>
        <div class="map-canvas-frame">
          <ClientOnly>
            <MapCanvas
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
              @viewchange="mapView = $event"
              @pointselect="selectMapPoint"
              @trainingselect="selectTrainingArea"
              @trainingmove="trainingEditor.moveCenter"
              @trainingresize="trainingEditor.resizeFromPixels"
              @trainingaccept="acceptTrainingDraft"
              @trainingdiscard="discardTrainingDraft"
              @contextaction="openContextNavigation"
              @navigateto="openNpcNavigation"
              @mapdrag="navigationAction.close(false)"
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
              {{ navigationAction.menuSummary.value }}
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
            <strong>{{ profileArea.label }}</strong>
            <div>
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
          </div>
        </div>
        <div class="map-viewport-footer">
          <span v-if="selectedTile && selectedGamePosition"
            >Selected {{ selectedGamePosition.x.toFixed(1) }},
            {{ selectedGamePosition.y.toFixed(1) }} in
            {{ zoneNameForRegion(selectedGamePosition.region) }}.</span
          >
          <span v-else-if="selectedTile"
            >Selected raster tile {{ selectedTile.tileX }} ×
            {{ selectedTile.tileY }}. Game coordinates are not inferred.</span
          >
          <span v-else
            >Pan and zoom the exported tile grid. Click or touch selects a
            point; right-click opens map point actions. Enter or Space selects
            the map center for keyboard users.</span
          >
          <span
            >Map point actions reuse each target's current Z, or 0 when it is
            unknown.</span
          >
        </div>
        <div class="map-point-actions" aria-label="Selected map point actions">
          <button
            class="compact-button"
            type="button"
            :disabled="!selectedTile"
            :title="selectedTile ? '' : 'Select a point on the map.'"
            @click="openSelectedNavigation"
          >
            {{ navigationAction.targetLabel.value }}
          </button>
          <button
            class="compact-button"
            type="button"
            :disabled="!selectedTile"
            :title="selectedTile ? '' : 'Select a point on the map.'"
            @click="openSelectedNavigation($event, 'training')"
          >
            {{ navigationAction.trainingLabel.value }}
          </button>
          <span class="map-action-target-note" role="note">
            Map point actions use the selected action targets.
          </span>
          <span v-if="selectedRegionAmbiguous" role="status"
            >This floor has two possible region IDs. Choose a region above or
            select a character in that region.</span
          >
          <div
            :ref="navigationAction.resultsElement"
            class="map-navigation-feedback"
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
        <div class="map-validation-note" role="status">
          {{
            mapProfile?.tiles.semantics ||
            'Tile and coordinate evidence is unavailable.'
          }}
          Outdoor region IDs locate their root tile directly. Reported X/Y
          positions locate markers within that tile; outlined dots indicate a
          region-only position. Cave imagery uses a 2D X/Y anchor and
          region/floor rules. Navigation and training position resolve
          coordinates and Z per target. Older observations remain visible as
          last observed positions.
        </div>
      </section>

      <aside class="map-side-panel panel">
        <!--
          Visual placeholders for later issues. These buttons and the Trace
          select have no click handlers, commands, or live data. Do not treat
          Start/Stop bot, Return scroll, Disconnect, or Trace as implemented.
        -->
        <section class="map-side-list map-character-actions">
          <div class="map-list-heading">
            <h2>Character actions</h2>
          </div>
          <div class="map-target-toolbar">
            <button class="compact-button" type="button">
              Start bot for {{ actionTargetIDs.size }} characters
            </button>
            <button class="compact-button" type="button">
              Stop bot for {{ actionTargetIDs.size }} characters
            </button>
            <button class="compact-button" type="button">
              Return scroll for {{ actionTargetIDs.size }} characters
            </button>
            <button class="compact-button" type="button">
              Disconnect {{ actionTargetIDs.size }} characters
            </button>
          </div>
          <div class="">
            <div>
              Trace:
              <select>
                <option value="char1">char1</option>
                <option value="char2">char2</option>
                <option value="char3">char3</option>
              </select>
              <button class="compact-button" type="button">
                Refresh player list
              </button>
            </div>
            <button class="compact-button" type="button">Start trace</button>
            <button class="compact-button" type="button">Stop trace</button>
          </div>
        </section>
        <section class="map-side-list map-character-list">
          <div class="map-list-heading">
            <h2>Characters</h2>
            <span>{{ scopedCharacters.length }}</span>
          </div>
          <div class="map-target-toolbar">
            <button
              class="compact-button"
              type="button"
              :disabled="!scopedCharacters.length"
              @click="selectAllActionTargets"
            >
              All
            </button>
            <button
              class="compact-button"
              type="button"
              :disabled="!actionTargetIDs.size"
              @click="clearActionTargets"
            >
              None
            </button>
            <span role="status"
              >Selected for actions: {{ actionTargetIDs.size }}</span
            >
          </div>
          <div v-if="mapTargetGroups.length" class="map-target-groups">
            <label
              v-for="group in mapTargetGroups"
              :key="group.group_id"
              class="map-target-group"
            >
              <input
                type="checkbox"
                :checked="group.state === 'checked'"
                :indeterminate="group.state === 'indeterminate'"
                :aria-checked="
                  group.state === 'indeterminate'
                    ? 'mixed'
                    : group.state === 'checked'
                      ? 'true'
                      : 'false'
                "
                :aria-label="`Target group ${group.name} for actions`"
                @change="toggleActionTargetGroup(group.memberIDs)"
              />
              <span>{{ group.name }}</span>
              <small>{{ group.memberIDs.length }}</small>
            </label>
          </div>
          <MapCharacterStatusRow
            v-for="character in scopedCharacters"
            :key="character.character_id"
            :character="character"
            :selected="selectedCharacterID === character.character_id"
            :targeted="actionTargetIDs.has(character.character_id)"
            :position-fresh="positionIsFresh(character)"
            @toggle-target="toggleActionTarget(character.character_id)"
            @focus="
              selectedCharacterID =
                selectedCharacterID === character.character_id
                  ? ''
                  : character.character_id
            "
          />
          <p v-if="!scopedCharacters.length" class="map-empty-copy">
            No characters in this server and zone scope.
          </p>
        </section>
        <section class="map-side-list map-navigation-route-list">
          <div class="map-list-heading">
            <h2>Navigation routes</h2>
            <span>{{ mapNavigationRoutes.length }}</span>
          </div>
          <button
            v-if="selectedNavigationRouteID"
            class="compact-button map-route-clear-selection"
            type="button"
            @click="selectedNavigationRouteID = ''"
          >
            Show all routes
          </button>
          <button
            v-for="routeOverlay in mapNavigationRoutes"
            :key="routeOverlay.id"
            class="map-navigation-route-row"
            type="button"
            :class="{
              selected: selectedNavigationRouteID === routeOverlay.characterID,
              dimmed:
                selectedNavigationRouteID &&
                selectedNavigationRouteID !== routeOverlay.characterID,
            }"
            :aria-pressed="
              selectedNavigationRouteID === routeOverlay.characterID
            "
            @click="
              selectedNavigationRouteID =
                selectedNavigationRouteID === routeOverlay.characterID
                  ? ''
                  : routeOverlay.characterID
            "
          >
            <strong>{{ routeOverlay.characterName }}</strong>
            <span>{{
              mapNavigationStatusLabel(
                routeOverlay.status,
                routeOverlay.blocks.length,
              )
            }}</span>
            <small v-if="routeOverlay.reason">{{ routeOverlay.reason }}</small>
          </button>
          <p v-if="!mapNavigationRoutes.length" class="map-empty-copy">
            No routes are reported for this server and active session.
          </p>
          <p
            v-if="mapSnapshot?.navigation_omitted_count"
            class="map-empty-copy"
            role="status"
          >
            {{ mapSnapshot.navigation_omitted_count }} route record(s) were
            omitted to keep the live map within its payload budget.
          </p>
        </section>
        <section class="map-training-list">
          <div class="map-list-heading">
            <h2>Training areas</h2>
            <span>{{ renderedTrainingAreas.length }}</span>
          </div>
          <div v-if="renderedTrainingAreas.length" class="map-training-rows">
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
            Only the first training areas are shown to keep the live map within
            its payload budget.
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
              {{ zoneNameForRegion(trainingEditor.draft.value.center.region) }}
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
            <div class="map-training-editor-actions">
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
                  result.step.name === 'training.area.set' ? 'Center' : 'Radius'
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
        </section>
        <section>
          <h2>Layers</h2>
          <label class="map-layer-toggle"
            ><input v-model="layerCharacters" type="checkbox" /> Characters
            <span>{{ placedCharacterCount }} shown</span></label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerParty" type="checkbox" /> Party members
            <span>{{ placedPartyCount }} shown</span></label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerNPCs" type="checkbox" /> NPCs
            <span>{{ placedNpcCount }} shown</span></label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerTraining" type="checkbox" /> Training areas
            <span>{{ renderedTrainingAreas.length }} shown</span></label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerMonsters" type="checkbox" /> Current nearby
            monsters <span>{{ currentMonsters.length }}</span></label
          >
          <label class="map-layer-toggle map-layer-toggle-subordinate"
            ><input v-model="showNearbyMonsterNames" type="checkbox" /> Show
            nearby monsters names</label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerDeaths" type="checkbox" /> Recent
            deaths</label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerDrops" type="checkbox" /> Recent drops</label
          >
          <label
            class="map-layer-toggle disabled"
            title="Academy member region and floor are unavailable from the documented API"
            ><input type="checkbox" disabled /> Academy members
            <span>Unavailable</span></label
          >
        </section>
        <section class="heatmap-controls">
          <button
            class="map-list-heading map-section-toggle"
            type="button"
            :aria-expanded="historicalHeatmapsOpen"
            aria-controls="historical-heatmap-controls"
            @click="historicalHeatmapsOpen = !historicalHeatmapsOpen"
          >
            <h2>Historical heatmaps</h2>
            <span>{{ renderedHeatLayers.length }} active</span>
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
            <div v-if="heatmapRange === 'custom'" class="heatmap-custom-range">
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
            <label>
              Historical character
              <select
                v-model="analyticsCharacterID"
                aria-label="Historical heatmap character"
              >
                <option value="">All characters</option>
                <option
                  v-for="character in historicalCharacters"
                  :key="character.character_id"
                  :value="character.character_id"
                >
                  {{ character.name }}
                </option>
              </select>
            </label>
            <label
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
                  v-for="monsterRankValue in historicalMobTypes"
                  :key="monsterRankValue"
                  :value="monsterRankValue"
                >
                  {{
                    monsterTypePresentation({ type: monsterRankValue }).label
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

            <label
              class="map-layer-toggle disabled"
              title="Observation coverage is not verified"
            >
              <input type="checkbox" disabled /> Mob density
              <span>Unavailable</span>
            </label>
            <p class="heatmap-warning">
              Spatial mob density is unavailable until observation coverage is
              verified.
            </p>
            <label class="map-layer-toggle">
              <input
                v-model="historicalLayers.mob_observer_average"
                type="checkbox"
              />
              Observer-local mob average
              <span>{{
                heatmapResults.mob_observer_average?.points.length || 0
              }}</span>
            </label>
            <label class="map-layer-toggle">
              <input v-model="historicalLayers.mob_types" type="checkbox" /> Mob
              ranks
              <span>{{ heatmapResults.mob_types?.points.length || 0 }}</span>
            </label>
            <label class="map-layer-toggle">
              <input v-model="historicalLayers.deaths" type="checkbox" /> Deaths
              <span>{{ heatmapResults.deaths?.points.length || 0 }}</span>
            </label>
            <label class="map-layer-toggle">
              <input v-model="historicalLayers.drops" type="checkbox" /> Drops
              <span>{{ heatmapResults.drops?.points.length || 0 }}</span>
            </label>
            <label class="map-layer-toggle">
              <input
                v-model="historicalLayers.unique_sightings"
                type="checkbox"
              />
              Unique sightings
              <span>{{
                heatmapResults.unique_sightings?.points.length || 0
              }}</span>
            </label>
            <label class="map-layer-toggle">
              <input
                v-model="historicalLayers.player_movement"
                type="checkbox"
              />
              Player movement
              <span>{{
                heatmapResults.player_movement?.points.length || 0
              }}</span>
            </label>

            <template
              v-for="layer in activeHistoricalLayers()"
              :key="`heat-status-${layer}`"
            >
              <p v-if="heatmapLoading[layer]" class="map-empty-copy">
                Refreshing {{ heatmapLayerLabel(layer) }}…
              </p>
              <p v-else-if="heatmapErrors[layer]" class="heatmap-warning">
                {{ heatmapLayerLabel(layer) }}: {{ heatmapErrors[layer] }}
              </p>
              <p
                v-else-if="heatmapResults[layer]?.status === 'unsupported'"
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
                No {{ heatmapLayerLabel(layer).toLowerCase() }} data in this
                scope.
              </p>
              <p
                v-if="heatmapResults[layer]?.truncated"
                class="heatmap-warning"
              >
                {{ heatmapLayerLabel(layer) }} reached the bounded result limit.
              </p>
            </template>

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
            <button
              class="compact-button"
              type="button"
              :disabled="!resettableLayers.length"
              @click="openHeatmapReset"
            >
              Reset selected heatmap…
            </button>
          </div>
        </section>
        <section class="map-side-list">
          <div class="map-list-heading">
            <h2>Nearby monsters</h2>
            <span>{{ currentMonsters.length }}</span>
          </div>
          <div
            v-for="entry in currentMonsters.slice(0, 30)"
            :key="`${entry.observer.session_id}:${entry.id}`"
            class="map-observation-row"
          >
            <span
              ><strong>{{ monsterDisplayName(entry) }}</strong
              ><small
                >Lv. {{ entry.level ?? 'unavailable' }} ·
                {{ monsterTypePresentation(entry).label }} ·
                {{ entry.observer.character }} ·
                {{ zoneNameForRegion(entry.region) }}</small
              ></span
            >
            <small>{{ entry.x.toFixed(0) }}, {{ entry.y.toFixed(0) }}</small>
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
            <h2>Recent deaths and drops</h2>
            <span>{{ visibleEvents.length }}</span>
          </div>
          <NuxtLink
            v-for="event in visibleEvents.slice(0, 12)"
            :key="event.event_id"
            class="map-event-row"
            :to="{ path: '/events', query: { kind: event.kind } }"
          >
            <span
              ><strong>{{
                event.kind === 'character.died'
                  ? 'Death'
                  : event.kind === 'drop.rare'
                    ? 'Rare drop'
                    : 'Drop'
              }}</strong
              ><small
                >{{ event.character }} · {{ zoneNameText(event.zone) }}</small
              ></span
            >
            <time :datetime="event.occurred_at">{{
              formatTimestamp(event.occurred_at)
            }}</time>
          </NuxtLink>
          <p v-if="!visibleEvents.length" class="map-empty-copy">
            No matching death or drop events in the selected time range.
          </p>
        </section>
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
.heatmap-custom-range label {
  display: grid;
  gap: 4px;
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
</style>
