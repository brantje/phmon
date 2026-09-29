<script setup lang="ts">
import type {
  ActivityEvent,
  CharacterView,
  MapMonster,
  MapSnapshot,
} from '~~/shared/types/live'
import type { MapAreaProfile, MapProfile } from '~~/shared/types/map'
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
import {
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

const {
  mapFeeds,
  mapFeedCurrent,
  fleetCharacters,
  groups,
  connectionState,
  freshnessNow,
  setMapFeed,
  clearMapFeed,
  characterControls,
  commandHistory,
  liveStale,
  setCharacterControls,
  setCharacterCommands,
  clearCharacterCommandSubscriptions,
} = useLiveData()
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
const selectedDestinationID = ref('')
const dateRange = ref('24h')
const layerCharacters = ref(true)
const layerMonsters = ref(true)
const layerDeaths = ref(true)
const layerDrops = ref(true)
const mapProfile = ref<MapProfile | null>(null)
const profileLoading = ref(false)
const profileError = ref('')
const mapView = ref({ tileX: 168, tileY: 97, zoomPercent: 100 })
const selectedTile = ref<RasterPosition | null>(null)
const jumpSequence = ref(0)
const actionBusy = ref(false)
const actionMessage = ref('')
const acceptedCommandID = ref('')
const snapshot = computed(() => mapFeeds.value[subscriptionID])
const mapSnapshot = computed(() => snapshot.value as MapSnapshot | undefined)
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
const commandTargetReady = computed(() =>
  Boolean(
    currentCharacter.value?.online &&
    currentCharacter.value.session_id &&
    !liveStale.value &&
    streamCurrent.value &&
    characterControls.value?.character_id ===
      currentCharacter.value.character_id &&
    characterControls.value.session_id === currentCharacter.value.session_id,
  ),
)
const acceptedCommand = computed(() =>
  commandHistory.value.find(
    (command) => command.command_id === acceptedCommandID.value,
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
function mapActionReason(name: string) {
  if (!selectedTile.value) return 'Select a point on the map.'
  if (selectedRegionAmbiguous.value)
    return 'Choose a verified cave region or a character currently in that region.'
  if (!selectedGamePosition.value)
    return 'The selected point has no verified region and X/Y conversion.'
  if (!commandTargetReady.value)
    return 'Select an online character with a current session and capability report.'
  const capability = characterControls.value?.capabilities[name]
  return capability?.supported
    ? ''
    : capability?.reason || 'The phBot action is unavailable in this session.'
}
async function submitMapAction(
  name: 'training.area.set' | 'character.navigate',
) {
  if (
    mapActionReason(name) ||
    !selectedGamePosition.value ||
    !currentCharacter.value?.session_id
  )
    return
  const target = {
    id: currentCharacter.value.character_id,
    session: currentCharacter.value.session_id,
    name: currentCharacter.value.name,
  }
  const point = { ...selectedGamePosition.value }
  const label =
    name === 'character.navigate'
      ? 'start a generated path'
      : 'set the training area'
  if (
    !window.confirm(
      'Confirm ' +
        label +
        ' for ' +
        target.name +
        ' at ' +
        point.x.toFixed(1) +
        ', ' +
        point.y.toFixed(1) +
        ', Z ' +
        point.z.toFixed(1) +
        ' in ' +
        zoneNameForRegion(point.region) +
        '?',
    )
  )
    return
  if (currentCharacter.value?.session_id !== target.session) return
  actionBusy.value = true
  actionMessage.value = ''
  try {
    const accepted = await $fetch<{ command_id: string }>('/api/commands', {
      method: 'POST',
      body: {
        character_id: target.id,
        expected_session_id: target.session,
        name,
        args:
          name === 'training.area.set' ? { mode: 'position', ...point } : point,
        idempotency_key: createIdempotencyKey(),
        confirmation: true,
      },
    })
    acceptedCommandID.value = accepted.command_id
    actionMessage.value = `Command ${accepted.command_id} accepted; waiting for phBot's result.`
    setCharacterCommands(target.id)
  } catch (error) {
    const failure = error as { data?: { message?: string; error?: string } }
    actionMessage.value =
      failure.data?.message ||
      failure.data?.error ||
      'The command was rejected.'
  } finally {
    actionBusy.value = false
  }
}
const scopedCharacters = computed(() => {
  const items = mapSnapshot.value?.characters || []
  if (regionID.value !== 0)
    return items.filter((character) => character.region === regionID.value)
  return items
})
const currentMonsters = computed(() => {
  if (!layerMonsters.value) return []
  return dedupeCurrentMonsters(mapSnapshot.value?.monsters || [])
})
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
  const markers: Array<{
    id: string
    label: string
    kind: 'character' | 'monster' | 'death' | 'drop' | 'event'
    position: RasterPosition
    placement?: 'exact' | 'region-tile'
    monster?: MapMonster
    itemName?: string
    itemIconUrl?: string
    event?: ActivityEvent
  }> = layerCharacters.value
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
      )
    : []
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
        { monster: entry },
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
const placedCharacterCount = computed(
  () => mapMarkers.value.filter((marker) => marker.kind === 'character').length,
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

watch(server, (value) => {
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
  if (characterID) selectedDestinationID.value = ''
  acceptedCommandID.value = ''
  actionMessage.value = ''
  if (characterID) {
    setCharacterControls(characterID)
    setCharacterCommands(characterID)
  }
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
const eventWindowNow = ref(Date.now())
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
  clearCharacterCommandSubscriptions()
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
            {{ zoneNameForRegion(region) }}
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
              :key="`${server}:${areaID}:${floorID}:${selectedCharacterID}:${jumpSequence}`"
              :profile="canvasProfile"
              :initial-position="mapInitialPosition"
              :initial-tile="mapInitialTile"
              :markers="mapMarkers"
              @viewchange="mapView = $event"
              @pointselect="selectedTile = $event"
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
            >Pan and zoom the exported tile grid. Click, touch or right-click
            selects a raster tile.</span
          >
          <span
            >Z for action:
            {{ selectedGamePosition?.z.toFixed(1) ?? '—' }} (selected character
            or 0)</span
          >
        </div>
        <div class="map-point-actions" aria-label="Selected map point actions">
          <button
            class="compact-button"
            type="button"
            :disabled="
              actionBusy || Boolean(mapActionReason('character.navigate'))
            "
            :title="mapActionReason('character.navigate')"
            @click="submitMapAction('character.navigate')"
          >
            Generate path and navigate
          </button>
          <button
            class="compact-button"
            type="button"
            :disabled="
              actionBusy || Boolean(mapActionReason('training.area.set'))
            "
            :title="mapActionReason('training.area.set')"
            @click="submitMapAction('training.area.set')"
          >
            Set training area here
          </button>
          <span v-if="selectedRegionAmbiguous" role="status"
            >This floor has two possible region IDs. Choose a region above or
            select a character in that region.</span
          >
          <span v-if="acceptedCommand" role="status"
            >{{ acceptedCommand.name }}: {{ acceptedCommand.state
            }}{{ acceptedCommand.message ? ` · ${acceptedCommand.message}` : ''
            }}{{
              acceptedCommand.verification
                ? ` · ${acceptedCommand.verification}`
                : ''
            }}</span
          >
          <span v-else-if="actionMessage" role="status">{{
            actionMessage
          }}</span>
        </div>
        <div class="map-validation-note" role="status">
          {{
            mapProfile?.tiles.semantics ||
            'Tile and coordinate evidence is unavailable.'
          }}
          Outdoor region IDs locate their root tile directly. Reported X/Y
          positions locate markers within that tile; outlined dots indicate a
          region-only position. Cave imagery uses a 2D X/Y anchor and
          region/floor rules. Actions reuse the selected character's reported Z,
          or 0 when unavailable. Older observations remain visible as last
          observed positions.
        </div>
      </section>

      <aside class="map-side-panel panel">
        <section>
          <h2>Layers</h2>
          <label class="map-layer-toggle"
            ><input v-model="layerCharacters" type="checkbox" /> Characters
            <span>{{ placedCharacterCount }} shown</span></label
          >
          <label class="map-layer-toggle"
            ><input v-model="layerMonsters" type="checkbox" /> Current nearby
            monsters <span>{{ currentMonsters.length }}</span></label
          >
          <label
            class="map-layer-toggle disabled"
            title="Historical density overlays are part of Slice 9"
            ><input type="checkbox" disabled /> Mob density
            <span>Slice 9</span></label
          >
          <label
            class="map-layer-toggle disabled"
            title="Historical monster type overlays are part of Slice 9"
            ><input type="checkbox" disabled /> Mob types
            <span>Slice 9</span></label
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
        <section class="map-side-list">
          <div class="map-list-heading">
            <h2>Characters</h2>
            <span>{{ layerCharacters ? scopedCharacters.length : 0 }}</span>
          </div>
          <button
            v-for="character in layerCharacters ? scopedCharacters : []"
            :key="character.character_id"
            class="map-character-row"
            :class="{
              selected: selectedCharacterID === character.character_id,
            }"
            type="button"
            @click="selectedCharacterID = character.character_id"
          >
            <span
              ><strong>{{ character.name }}</strong
              ><small
                >{{ character.server }} ·
                {{ zoneNameText(character.zone) }}</small
              ></span
            >
            <small>{{
              character.online
                ? positionIsFresh(character)
                  ? 'Online'
                  : 'Online · last observed position'
                : 'Offline · last position'
            }}</small>
          </button>
          <p v-if="!scopedCharacters.length" class="map-empty-copy">
            No characters in this server and zone scope.
          </p>
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
                >Lv. {{ entry.level ?? 'unavailable' }} · Type:
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
  </section>
</template>
