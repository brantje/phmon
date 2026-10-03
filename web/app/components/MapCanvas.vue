<script setup lang="ts">
import type {
  Circle as LeafletCircle,
  LayerGroup,
  Map as LeafletMap,
  LatLng,
  Marker as LeafletMarker,
} from 'leaflet'
import 'leaflet/dist/leaflet.css'
import type {
  ActivityEvent,
  MapMonster,
  MapNpc,
  MapOtherPlayer,
  MapPartyMember,
} from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import type {
  MonsterReferenceArea,
  MonsterReferenceBounds,
  MonsterReferenceGuideRow,
  MonsterReferencePoint,
} from '~~/shared/types/monsterReference'
import type { CharacterMarkerInput } from '~/utils/mapCharacterMarkers'
import type { MapHeatLayer } from '~/utils/mapHeatmap'
import type { MapRouteOverlay } from '~/utils/mapNavigationRoutes'
import {
  estimateTrainingLabelSize,
  placeTrainingAreaLabels,
  type TrainingAreaOverlay,
} from '~/utils/mapTrainingAreas'
import {
  NPC_MARKER_ICON,
  npcDisplayLabel,
  npcRoleLabel,
} from '~/utils/mapNpcMarkers'
import { PARTY_MEMBER_ICON } from '~/utils/mapPartyPresentation'
import {
  OTHER_PLAYER_ICON,
  playerAliveLabel,
} from '~/utils/mapPlayerPresentation'
import {
  localMapAsset,
  monsterDisplayName,
  monsterHPBarFraction,
  monsterHPFraction,
  monsterMapName,
  monsterTypePresentation,
} from '~/utils/mapMarkerPresentation'
import {
  leafletToRasterPosition,
  rasterTileCenterToLeaflet,
  type RasterPosition,
} from '~/utils/mapCoordinates'
import {
  INITIAL_MAP_ZOOM,
  MAP_ZOOM_OPTIONS,
  MAP_ZOOM_PERCENT_STEP,
  mapZoomLevelForPercent,
  mapZoomPercentForLevel,
  snapMapZoomPercent,
} from '~/utils/mapZoom'
import { mapCharacterClusters } from '~/utils/mapCharacterClusters'
import { interpolateMarkerPosition } from '~/utils/mapMarkerAnimation'
import { displayedGuideCells } from '~/utils/guideCells'
import { paddedMapPanBounds } from '~/utils/mapPanBounds'

interface MapCanvasMarker {
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
  character?: CharacterMarkerInput
  party?: MapPartyMember
  player?: MapOtherPlayer
  npc?: MapNpc
  monster?: MapMonster
  showLabel?: boolean
  observerName?: string
  zoneLabel?: string
  itemName?: string
  itemIconUrl?: string
  event?: ActivityEvent
}

const props = defineProps<{
  profile: MapProfile
  compact?: boolean
  externalControls?: boolean
  focusedCharacterID?: string
  initialPosition?: RasterPosition | null
  focusRequest?: number
  initialTile?: { x: number; y: number }
  markers?: MapCanvasMarker[]
  heatLayers?: MapHeatLayer[]
  referenceAreas?: MonsterReferenceArea[]
  referenceGuideRows?: MonsterReferenceGuideRow[]
  referencePoints?: MonsterReferencePoint[]
  referenceFocus?: MonsterReferenceBounds & { sequence: number }
  navigationRoutes?: MapRouteOverlay[]
  trainingAreas?: TrainingAreaOverlay[]
  trainingEditable?: boolean
  trainingAcceptDisabled?: boolean
  trainingDiscardDisabled?: boolean
  trainingAcceptTitle?: string
}>()
const emit = defineEmits<{
  viewchange: [
    view: {
      tileX: number
      tileY: number
      zoomPercent: number
      bounds: MonsterReferenceBounds
    },
  ]
  trainingselect: [characterID: string]
  trainingmove: [characterID: string, point: RasterPosition]
  trainingresize: [characterID: string, radiusPixels: number]
  trainingaccept: [characterID: string]
  trainingdiscard: [characterID: string]
  pointselect: [point: RasterPosition]
  contextaction: [
    action: { point: RasterPosition; anchor: { x: number; y: number } },
  ]
  navigateto: [point: RasterPosition, anchor: { x: number; y: number }]
  teleportto: [
    npc: NonNullable<MapCanvasMarker['npc']>,
    point: RasterPosition,
    anchor: { x: number; y: number },
  ]
  teleportercontext: [
    npc: NonNullable<MapCanvasMarker['npc']>,
    point: RasterPosition,
    anchor: { x: number; y: number },
  ]
  mapdrag: []
  opencharacter: [characterID: string]
  inspectcharacter: [characterID: string]
}>()
const element = ref<HTMLDivElement | null>(null)
let map: LeafletMap | undefined
let heatLayerGroup: LayerGroup | undefined
let referenceAreaLayer: LayerGroup | undefined
let referenceLabelLayer: LayerGroup | undefined
let referencePointLayer: LayerGroup | undefined
let markerLayer: LayerGroup | undefined
let navigationLayerGroup: LayerGroup | undefined
let trainingLayerGroup: LayerGroup | undefined
let leaflet: typeof import('leaflet') | undefined
interface RenderedTrainingArea {
  circle: LeafletCircle
  label: LeafletMarker
  angle: number
  centerHandle?: LeafletMarker
  edgeHandle?: LeafletMarker
}
const renderedTraining = new Map<string, RenderedTrainingArea>()
let visibleTrainingAreas: TrainingAreaOverlay[] = []
let trainingDragging = ''
let heatRenderer: L.Canvas | undefined
let makeHeatCircle: typeof import('leaflet').circleMarker | undefined
let makePolyline: typeof import('leaflet').polyline | undefined
let makeRouteCircle: typeof import('leaflet').circleMarker | undefined
let createLatLng: ((latitude: number, longitude: number) => LatLng) | undefined
let makeMarker:
  | ((
      marker: MapCanvasMarker,
      point: LatLng,
      existing?: LeafletMarker,
    ) => LeafletMarker)
  | undefined
const renderedMarkers = new Map<string, LeafletMarker>()
const markerIconSignatures = new Map<string, string>()
const markerAnimationFrames = new Map<string, number>()
const MARKER_ANIMATION_DURATION_MS = 120
/** On-map display size for 8×8 minimap sign PNGs (party / other player / NPC). */
const MINIMAP_SIGN_ICON_PX = 16

function styleMinimapSignImage(img: HTMLImageElement) {
  const size = `${MINIMAP_SIGN_ICON_PX}px`
  img.style.width = size
  img.style.height = size
  img.style.imageRendering = 'pixelated'
}
let canvasResizeObserver: ResizeObserver | undefined
const clusterChoices = ref<{ id: string; name: string }[]>([])
let stopped = false
let lastFocusedTile = ''
let lastFocusRequest = 0
let initialPositionApplied = false
let viewAdjusted = false

function indexAt(position: LatLng) {
  return leafletToRasterPosition(
    props.profile.tiles,
    position.lat,
    position.lng,
  )
}

function rasterContentBounds() {
  const rows = props.profile.tiles.max_y - props.profile.tiles.min_y + 1
  const columns = props.profile.tiles.max_x - props.profile.tiles.min_x + 1
  return { south: -rows * 256, west: 0, north: 0, east: columns * 256 }
}

function syncPanBounds() {
  if (!map || !leaflet) return
  const size = map.getSize()
  const padded = paddedMapPanBounds(
    rasterContentBounds(),
    { width: size.x, height: size.y },
    2 ** map.getZoom(),
  )
  map.setMaxBounds(
    leaflet.latLngBounds(
      leaflet.latLng(padded.south, padded.west),
      leaflet.latLng(padded.north, padded.east),
    ),
  )
}

function setInitialView() {
  if (!map || !createLatLng) return
  if (props.initialPosition) {
    initialPositionApplied = true
    lastFocusedTile = `${props.initialPosition.tileX}:${props.initialPosition.tileY}`
    lastFocusRequest = props.focusRequest || 0
    const column = props.initialPosition.tileX - props.profile.tiles.min_x
    const row = props.profile.tiles.max_y - props.initialPosition.tileY
    map.setView(
      createLatLng(
        -(row * 256 + props.initialPosition.pixelY),
        column * 256 + props.initialPosition.pixelX,
      ),
      INITIAL_MAP_ZOOM,
    )
    return
  }
  const initial = props.initialTile || { x: 168, y: 97 }
  lastFocusedTile = `${initial.x}:${initial.y}`
  lastFocusRequest = props.focusRequest || 0
  const center = rasterTileCenterToLeaflet(
    props.profile.tiles,
    initial.x,
    initial.y,
  )
  if (center) {
    map.setView(createLatLng(center.lat, center.lng), INITIAL_MAP_ZOOM)
    return
  }
  const columns = props.profile.tiles.max_x - props.profile.tiles.min_x + 1
  const rows = props.profile.tiles.max_y - props.profile.tiles.min_y + 1
  const column = Math.max(
    0,
    Math.min(columns - 1, initial.x - props.profile.tiles.min_x),
  )
  const row = Math.max(
    0,
    Math.min(rows - 1, props.profile.tiles.max_y - initial.y),
  )
  map.setView(
    createLatLng(-(row * 256 + 128), column * 256 + 128),
    INITIAL_MAP_ZOOM,
  )
}

function publishView() {
  if (!map) return
  const visible = map.getBounds()
  const northWest = indexAt(visible.getNorthWest())
  const southEast = indexAt(visible.getSouthEast())
  emit('viewchange', {
    ...indexAt(map.getCenter()),
    zoomPercent: mapZoomPercentForLevel(map.getZoom()),
    bounds: {
      min_x: Math.max(props.profile.tiles.min_x, northWest.tileX),
      max_x: Math.min(props.profile.tiles.max_x, southEast.tileX),
      min_y: Math.max(props.profile.tiles.min_y, southEast.tileY),
      max_y: Math.min(props.profile.tiles.max_y, northWest.tileY),
    },
  })
}

function snapZoomToPercentStep() {
  if (!map) return
  const snappedPercent = snapMapZoomPercent(
    mapZoomPercentForLevel(map.getZoom()),
  )
  const targetZoom = mapZoomLevelForPercent(snappedPercent)
  if (Math.abs(map.getZoom() - targetZoom) > 1e-9) map.setZoom(targetZoom)
}

function moveMarker(markerKey: string, marker: LeafletMarker, target: LatLng) {
  const previousFrame = markerAnimationFrames.get(markerKey)
  if (previousFrame != null) cancelAnimationFrame(previousFrame)

  const start = marker.getLatLng()
  const distance = Math.hypot(target.lat - start.lat, target.lng - start.lng)
  if (distance < 0.01) {
    markerAnimationFrames.delete(markerKey)
    marker.setLatLng(target)
    return
  }

  let startedAt: number | undefined
  const animate = (timestamp: number) => {
    if (stopped || !renderedMarkers.has(markerKey)) {
      markerAnimationFrames.delete(markerKey)
      return
    }
    startedAt ??= timestamp
    const progress = Math.min(
      1,
      (timestamp - startedAt) / MARKER_ANIMATION_DURATION_MS,
    )
    const position = interpolateMarkerPosition(start, target, progress)
    marker.setLatLng(createLatLng!(position.lat, position.lng))
    if (progress >= 1) {
      markerAnimationFrames.delete(markerKey)
      if (!markerAnimationFrames.size) layoutCharacterLabels()
      return
    }
    markerAnimationFrames.set(markerKey, requestAnimationFrame(animate))
  }
  markerAnimationFrames.set(markerKey, requestAnimationFrame(animate))
}

function syncMarkers() {
  if (!map || !markerLayer || !createLatLng) return
  if (!makeMarker) return
  const current = new Set<string>()
  for (const marker of (props.markers || []).slice(0, 2000)) {
    const { tileX, tileY, pixelX, pixelY } = marker.position
    if (
      tileX < props.profile.tiles.min_x ||
      tileX > props.profile.tiles.max_x ||
      tileY < props.profile.tiles.min_y ||
      tileY > props.profile.tiles.max_y ||
      !Number.isFinite(pixelX) ||
      !Number.isFinite(pixelY) ||
      pixelX < 0 ||
      pixelX >= 256 ||
      pixelY < 0 ||
      pixelY >= 256
    )
      continue
    const column = tileX - props.profile.tiles.min_x
    const row = props.profile.tiles.max_y - tileY
    const point = createLatLng(-(row * 256 + pixelY), column * 256 + pixelX)
    const key = `${marker.kind}:${marker.id}`
    current.add(key)
    renderedMarkers.set(
      key,
      makeMarker(marker, point, renderedMarkers.get(key)),
    )
  }
  for (const [key, rendered] of renderedMarkers) {
    if (current.has(key)) continue
    markerLayer.removeLayer(rendered)
    renderedMarkers.delete(key)
    markerIconSignatures.delete(key)
    const animationFrame = markerAnimationFrames.get(key)
    if (animationFrame != null) cancelAnimationFrame(animationFrame)
    markerAnimationFrames.delete(key)
  }
  layoutCharacterLabels()
}

function layoutCharacterLabels() {
  if (!map || props.compact) return
  const points = [...renderedMarkers.entries()]
    .filter(([key]) => key.startsWith('character:'))
    .map(([key, marker]) => {
      const point = map!.latLngToContainerPoint(marker.getLatLng())
      return { id: key.slice('character:'.length), x: point.x, y: point.y }
    })
  const clusters = mapCharacterClusters(points)
  for (const point of points) {
    const element = renderedMarkers.get(`character:${point.id}`)?.getElement()
    if (!element) continue
    const cluster = clusters.find((group) => group.includes(point.id))
    element
      .querySelector('.phmon-map-character-name')
      ?.classList.toggle(
        'phmon-map-name-collapsed',
        Boolean(cluster && props.focusedCharacterID !== point.id),
      )
    let button = element.querySelector<HTMLButtonElement>(
      '.phmon-map-character-cluster',
    )
    if (cluster?.[0] === point.id) {
      const membership = JSON.stringify(cluster)
      if (button?.dataset.characterIds === membership) {
        button.textContent = `${cluster.length} characters`
        continue
      }
      button?.remove()
      button = document.createElement('button')
      button.className = 'compact-button phmon-map-character-cluster'
      button.type = 'button'
      button.dataset.characterIds = membership
      button.textContent = `${cluster.length} characters`
      button.addEventListener('pointerdown', (event) => event.stopPropagation())
      button.addEventListener('click', (event) => {
        event.stopPropagation()
        clusterChoices.value = cluster.map((id) => ({
          id,
          name:
            props.markers?.find(
              (marker) => marker.kind === 'character' && marker.id === id,
            )?.character?.name || id,
        }))
      })
      element.append(button)
    } else button?.remove()
  }
}
function heatLayerColor(id: MapHeatLayer['id']) {
  switch (id) {
    case 'deaths':
      return '#e75b64'
    case 'drops':
      return '#f0c75e'
    case 'unique_sightings':
      return '#b88cff'
    case 'player_movement':
      return '#56bff2'
    case 'mob_types':
      return '#ee8a4c'
    case 'mob_observer_average':
      return '#75d783'
    default:
      return '#a9b4c2'
  }
}

function syncHeatLayers() {
  if (
    !map ||
    !heatLayerGroup ||
    !heatRenderer ||
    !makeHeatCircle ||
    !createLatLng
  )
    return
  heatLayerGroup.clearLayers()
  for (const layer of props.heatLayers || []) {
    for (const point of layer.points.slice(0, 2000)) {
      const { tileX, tileY, pixelX, pixelY } = point.position
      if (
        tileX < props.profile.tiles.min_x ||
        tileX > props.profile.tiles.max_x ||
        tileY < props.profile.tiles.min_y ||
        tileY > props.profile.tiles.max_y ||
        !Number.isFinite(pixelX) ||
        !Number.isFinite(pixelY)
      )
        continue
      const column = tileX - props.profile.tiles.min_x
      const row = props.profile.tiles.max_y - tileY
      const rendered = makeHeatCircle(
        createLatLng(-(row * 256 + pixelY), column * 256 + pixelX),
        {
          renderer: heatRenderer,
          radius: 8 + 22 * point.intensity,
          stroke: false,
          fill: true,
          fillColor: heatLayerColor(layer.id),
          fillOpacity: 0.12 + 0.5 * point.intensity,
          interactive: !props.compact,
          bubblingMouseEvents: false,
        },
      )
      if (!props.compact) {
        const value =
          point.denominator != null
            ? `${point.numerator ?? point.count} / ${point.denominator} · ${point.weight.toFixed(2)}`
            : `${point.count} · ${point.weight.toFixed(2)}`
        rendered.bindTooltip(`${layer.label}: ${value}`, {
          direction: 'top',
          opacity: 0.92,
        })
      }
      rendered.addTo(heatLayerGroup)
    }
  }
}

function syncNavigationRoutes() {
  if (
    !navigationLayerGroup ||
    !makePolyline ||
    !makeRouteCircle ||
    !createLatLng
  )
    return
  const toLatLng = createLatLng
  navigationLayerGroup.clearLayers()
  for (const route of props.navigationRoutes || []) {
    const opacity = route.selected ? (route.stale ? 0.48 : 0.92) : 0.2
    for (const [blockIndex, block] of route.blocks.entries()) {
      const points = [...block]
      if (blockIndex === 0 && route.currentAnchor)
        points.unshift(route.currentAnchor)
      const latLngs = points.map(({ tileX, tileY, pixelX, pixelY }) => {
        const column = tileX - props.profile.tiles.min_x
        const row = props.profile.tiles.max_y - tileY
        return toLatLng(-(row * 256 + pixelY), column * 256 + pixelX)
      })
      if (latLngs.length > 1) {
        const line = makePolyline(latLngs, {
          color: '#37d6d1',
          weight: 3,
          dashArray: '6 4',
          opacity,
          lineCap: 'round',
          lineJoin: 'round',
          interactive: true,
        })
        line.bindTooltip(`${route.characterName} · remaining route`, {
          sticky: true,
          opacity: 0.95,
        })
        line.addTo(navigationLayerGroup)
      }
      for (const point of block) {
        const column = point.tileX - props.profile.tiles.min_x
        const row = props.profile.tiles.max_y - point.tileY
        const dot = makeRouteCircle(
          toLatLng(-(row * 256 + point.pixelY), column * 256 + point.pixelX),
          {
            radius: 3.5,
            color: '#b8ffff',
            weight: 1.5,
            fillColor: '#23c9cc',
            fillOpacity: opacity,
            opacity,
            interactive: true,
          },
        )
        dot.bindTooltip(`${route.characterName} · route waypoint`, {
          direction: 'top',
          opacity: 0.95,
        })
        dot.addTo(navigationLayerGroup)
      }
    }
  }
}

function rasterToLatLng(position: RasterPosition) {
  const column = position.tileX - props.profile.tiles.min_x
  const row = props.profile.tiles.max_y - position.tileY
  return createLatLng!(
    -(row * 256 + position.pixelY),
    column * 256 + position.pixelX,
  )
}

/** `angle` is degrees clockwise from north on the circle's edge. */
function trainingLabelPoint(center: LatLng, radius: number, angle = 0) {
  const radians = (angle * Math.PI) / 180
  return createLatLng!(
    center.lat + radius * Math.cos(radians),
    center.lng + radius * Math.sin(radians),
  )
}

/** Pushes the chip outward from the circle so it never covers its own edge. */
function applyTrainingLabelAngle(rendered: RenderedTrainingArea) {
  const element = rendered.label.getElement()
  if (!element) return
  const radians = (rendered.angle * Math.PI) / 180
  const sin = Math.sin(radians)
  const cos = Math.cos(radians)
  element.style.translate = `calc(${-50 + 50 * sin}% + ${3 * sin}px) calc(${-50 - 50 * cos}% - ${3 * cos}px)`
}
const trainingEdgePoint = (center: LatLng, radius: number) =>
  createLatLng!(center.lat, center.lng + radius)

function trainingHandleIcon(kind: 'center' | 'edge') {
  return leaflet!.divIcon({
    className: `phmon-map-training-handle phmon-map-training-handle--${kind}`,
    iconSize: [14, 14],
    iconAnchor: [7, 7],
  })
}

function removeTrainingHandles(rendered: RenderedTrainingArea) {
  if (rendered.centerHandle)
    trainingLayerGroup?.removeLayer(rendered.centerHandle)
  if (rendered.edgeHandle) trainingLayerGroup?.removeLayer(rendered.edgeHandle)
  rendered.centerHandle = undefined
  rendered.edgeHandle = undefined
}

function finishTrainingDrag() {
  trainingDragging = ''
  // The parent may reject or clamp the drop; resync to its accepted draft.
  void nextTick(syncTrainingAreas)
}

function ensureTrainingHandles(id: string, rendered: RenderedTrainingArea) {
  const L = leaflet!
  const center = rendered.circle.getLatLng()
  const radius = rendered.circle.getRadius()
  if (!rendered.centerHandle) {
    const handle = L.marker(center, {
      icon: trainingHandleIcon('center'),
      draggable: true,
      keyboard: false,
      title: 'Drag to move the training center',
      zIndexOffset: 1400,
    })
    handle.on('dragstart', () => (trainingDragging = id))
    handle.on('drag', () => {
      const next = handle.getLatLng()
      const size = rendered.circle.getRadius()
      rendered.circle.setLatLng(next)
      rendered.label.setLatLng(trainingLabelPoint(next, size, rendered.angle))
      rendered.edgeHandle?.setLatLng(trainingEdgePoint(next, size))
    })
    handle.on('dragend', () => {
      emit('trainingmove', id, indexAt(handle.getLatLng()))
      finishTrainingDrag()
    })
    handle.addTo(trainingLayerGroup!)
    rendered.centerHandle = handle
  }
  if (!rendered.edgeHandle) {
    const handle = L.marker(trainingEdgePoint(center, radius), {
      icon: trainingHandleIcon('edge'),
      draggable: true,
      keyboard: false,
      title: 'Drag to resize the training radius',
      zIndexOffset: 1400,
    })
    handle.on('dragstart', () => (trainingDragging = id))
    handle.on('drag', () => {
      const origin = rendered.circle.getLatLng()
      const edge = handle.getLatLng()
      const size = Math.max(
        0.5,
        Math.hypot(edge.lat - origin.lat, edge.lng - origin.lng),
      )
      rendered.circle.setRadius(size)
      rendered.label.setLatLng(trainingLabelPoint(origin, size, rendered.angle))
    })
    handle.on('dragend', () => {
      emit('trainingresize', id, rendered.circle.getRadius())
      finishTrainingDrag()
    })
    handle.addTo(trainingLayerGroup!)
    rendered.edgeHandle = handle
  }
}

function stopTrainingActionEvent(event: Event) {
  event.stopPropagation()
}

function trainingActionIcon(kind: 'discard' | 'accept') {
  const svg = document.createElementNS('http://www.w3.org/2000/svg', 'svg')
  svg.setAttribute('viewBox', '0 0 12 12')
  svg.setAttribute('aria-hidden', 'true')
  const path = document.createElementNS('http://www.w3.org/2000/svg', 'path')
  path.setAttribute('fill', 'none')
  path.setAttribute('stroke', 'currentColor')
  path.setAttribute('stroke-width', '1.7')
  path.setAttribute('stroke-linecap', 'round')
  path.setAttribute('stroke-linejoin', 'round')
  path.setAttribute(
    'd',
    kind === 'discard'
      ? 'M2.2 2.2l7.6 7.6M9.8 2.2L2.2 9.8'
      : 'M2.1 6.3l2.7 2.7 5.1-5.6',
  )
  svg.append(path)
  return svg
}

function trainingActionButton(kind: 'discard' | 'accept', label: HTMLElement) {
  const button = document.createElement('button')
  button.type = 'button'
  button.className = `phmon-map-training-label-action phmon-map-training-label-action--${kind}`
  button.dataset.action = kind
  button.append(trainingActionIcon(kind))
  const emitAction = (event: Event) => {
    stopTrainingActionEvent(event)
    const characterID = label.dataset.areaId
    if (!characterID || button.disabled) return
    if (kind === 'accept') emit('trainingaccept', characterID)
    else emit('trainingdiscard', characterID)
  }
  for (const type of [
    'mousedown',
    'pointerdown',
    'touchstart',
    'dblclick',
    'contextmenu',
    'click',
    'keydown',
  ])
    button.addEventListener(type, stopTrainingActionEvent)
  button.addEventListener('click', emitAction)
  return button
}

function updateTrainingLabel(
  rendered: RenderedTrainingArea,
  area: TrainingAreaOverlay,
) {
  const element = rendered.label.getElement()
  const label = element?.querySelector<HTMLElement>('.phmon-map-training-label')
  if (!element || !label) return
  label.dataset.areaId = area.id
  label.classList.toggle('phmon-map-training-label--selected', area.selected)
  label.classList.toggle('phmon-map-training-label--draft', area.draft)
  let name = label.querySelector('.phmon-map-training-label-name')
  if (!name) {
    label.replaceChildren()
    name = document.createElement('span')
    name.className = 'phmon-map-training-label-name'
    label.append(name)
  }
  name.textContent = area.label
  const showActions = area.draft && !props.compact
  let actions = label.querySelector<HTMLElement>(
    '.phmon-map-training-label-actions',
  )
  let status = label.querySelector('.phmon-map-training-label-status')
  if (showActions) {
    if (!actions) {
      actions = document.createElement('span')
      actions.className = 'phmon-map-training-label-actions'
      actions.append(
        trainingActionButton('discard', label),
        trainingActionButton('accept', label),
      )
      leaflet?.DomEvent.disableClickPropagation(actions)
      name.after(actions)
    }
    if (!status) {
      status = document.createElement('span')
      status.className = 'phmon-map-training-label-status'
      status.textContent = '· unsaved'
      actions.after(status)
    }
    const discard = actions.querySelector<HTMLButtonElement>(
      '[data-action="discard"]',
    )
    const accept = actions.querySelector<HTMLButtonElement>(
      '[data-action="accept"]',
    )
    if (discard) {
      discard.disabled = Boolean(props.trainingDiscardDisabled)
      discard.setAttribute('aria-label', `Discard changes for ${area.label}`)
      discard.title = 'Discard changes'
    }
    if (accept) {
      accept.disabled = Boolean(props.trainingAcceptDisabled)
      accept.setAttribute('aria-label', `Accept changes for ${area.label}`)
      accept.title = props.trainingAcceptTitle || 'Accept changes'
    }
  } else {
    actions?.remove()
    status?.remove()
  }
  element.setAttribute(
    'aria-label',
    `Training area for ${area.label}${area.draft ? ', unsaved changes' : ''}${area.selected ? ', selected' : ''}`,
  )
  element.setAttribute('aria-pressed', String(area.selected))
}

function syncTrainingAreas() {
  if (!leaflet || !trainingLayerGroup || !createLatLng) return
  const L = leaflet
  const current = new Set<string>()
  visibleTrainingAreas = (props.trainingAreas || [])
    .slice(0, 256)
    .filter(
      (area) =>
        Number.isFinite(area.center.pixelX) &&
        Number.isFinite(area.center.pixelY) &&
        Number.isFinite(area.radiusPixels),
    )
  for (const area of visibleTrainingAreas) {
    current.add(area.id)
    const center = rasterToLatLng(area.center)
    let rendered = renderedTraining.get(area.id)
    if (!rendered) {
      const circle = L.circle(center, {
        radius: area.radiusPixels,
        color: '#4db9ff',
        fillColor: '#4db9ff',
        interactive: false,
      })
      const labelContent = document.createElement('span')
      labelContent.className = 'phmon-map-training-label'
      const label = L.marker(trainingLabelPoint(center, area.radiusPixels), {
        icon: L.divIcon({
          className: 'phmon-map-training-label-marker',
          html: labelContent,
          // Leaflet sizes the icon from CSS when iconSize is null; its typings omit null.
          iconSize: null as unknown as L.PointExpression,
        }),
        keyboard: !props.compact,
        interactive: !props.compact,
        zIndexOffset: 1300,
      })
      label.on('click', () => emit('trainingselect', area.id))
      circle.addTo(trainingLayerGroup)
      label.addTo(trainingLayerGroup)
      label.getElement()?.addEventListener('keydown', (event) => {
        if (event.key !== 'Enter' && event.key !== ' ') return
        if (
          event.target instanceof Element &&
          event.target.closest('.phmon-map-training-label-action')
        )
          return
        event.preventDefault()
        emit('trainingselect', area.id)
      })
      rendered = { circle, label, angle: 0 }
      renderedTraining.set(area.id, rendered)
    } else if (trainingDragging !== area.id) {
      rendered.circle.setLatLng(center)
      rendered.circle.setRadius(area.radiusPixels)
    }
    rendered.label.setZIndexOffset(area.selected ? 1350 : 1300)
    rendered.circle.setStyle({
      weight: area.selected ? 3 : 2,
      opacity: area.selected ? 0.95 : 0.7,
      fillOpacity: area.selected ? 0.14 : 0.06,
      dashArray: area.draft ? [6, 5] : [],
    })
    updateTrainingLabel(rendered, area)
    if (area.selected && props.trainingEditable && !props.compact) {
      ensureTrainingHandles(area.id, rendered)
      if (trainingDragging !== area.id) {
        rendered.centerHandle?.setLatLng(center)
        rendered.edgeHandle?.setLatLng(
          trainingEdgePoint(center, area.radiusPixels),
        )
      }
    } else removeTrainingHandles(rendered)
  }
  for (const [id, rendered] of renderedTraining) {
    if (current.has(id)) continue
    removeTrainingHandles(rendered)
    trainingLayerGroup.removeLayer(rendered.circle)
    trainingLayerGroup.removeLayer(rendered.label)
    renderedTraining.delete(id)
    if (trainingDragging === id) trainingDragging = ''
  }
  // Smaller circles paint over larger ones.
  for (const area of [...visibleTrainingAreas].sort(
    (left, right) => right.radiusPixels - left.radiusPixels,
  ))
    renderedTraining.get(area.id)?.circle.bringToFront()
  layoutTrainingLabels()
}

function layoutTrainingLabels() {
  if (!map) return
  const angles = placeTrainingAreaLabels(
    visibleTrainingAreas,
    2 ** map.getZoom(),
    (area) => {
      const chip = renderedTraining
        .get(area.id)
        ?.label.getElement()
        ?.querySelector<HTMLElement>('.phmon-map-training-label')
      return chip?.offsetWidth
        ? { width: chip.offsetWidth, height: chip.offsetHeight }
        : estimateTrainingLabelSize(area)
    },
  )
  for (const [id, rendered] of renderedTraining) {
    if (trainingDragging === id) continue
    rendered.angle = angles.get(id) ?? 0
    rendered.label.setLatLng(
      trainingLabelPoint(
        rendered.circle.getLatLng(),
        rendered.circle.getRadius(),
        rendered.angle,
      ),
    )
    applyTrainingLabelAngle(rendered)
  }
}

const integer = (value: number | undefined) =>
  value == null || !Number.isFinite(value)
    ? '—'
    : Math.round(value).toLocaleString('en-US')
const compactNumber = (value: number | undefined) =>
  value == null || !Number.isFinite(value)
    ? '—'
    : new Intl.NumberFormat('en-US', {
        notation: 'compact',
        maximumFractionDigits: 1,
      }).format(value)
const positionText = (
  x: number | undefined,
  y: number | undefined,
  z: number | undefined,
) =>
  x == null || y == null
    ? 'Unavailable'
    : `${[x, y, z].map((value) => (value == null ? '—' : value.toLocaleString('en-US', { minimumFractionDigits: 1, maximumFractionDigits: 1 }))).join(', ')}`

function detailRow(label: string, value: string) {
  const row = document.createElement('div')
  row.className = 'phmon-map-detail-row'
  const heading = document.createElement('span')
  heading.textContent = label
  const content = document.createElement('span')
  content.textContent = value
  row.append(heading, content)
  return row
}

function markerPortrait(
  url: string | undefined,
  name: string,
  kind: 'portrait' | 'item',
) {
  const frame = document.createElement('span')
  frame.className = 'phmon-map-detail-portrait'
  frame.textContent = name.slice(0, 1).toUpperCase() || '?'
  const safeURL = localMapAsset(url, kind)
  if (safeURL) {
    const img = document.createElement('img')
    img.src = safeURL
    img.alt = ''
    img.onerror = () => img.remove()
    frame.append(img)
  }
  return frame
}

function partyIconElement(className: string) {
  const frame = document.createElement('span')
  frame.className = className
  const img = document.createElement('img')
  img.src = PARTY_MEMBER_ICON
  img.alt = ''
  frame.append(img)
  return frame
}

function markerPopup(marker: MapCanvasMarker) {
  const panel = document.createElement('section')
  panel.className = `phmon-map-detail phmon-map-detail--${marker.kind}`
  const header = document.createElement('div')
  header.className = 'phmon-map-detail-head'
  const copy = document.createElement('div')
  const title = document.createElement('strong')
  const subtitle = document.createElement('span')
  copy.append(title, subtitle)
  header.append(copy)
  const details = document.createElement('div')
  details.className = 'phmon-map-detail-grid'

  if (marker.kind === 'character' && marker.character) {
    const character = marker.character
    title.textContent = character.name
    subtitle.textContent = character.online
      ? character.position_stale
        ? `Level ${integer(character.level)} | Online · last observed position`
        : `Level ${integer(character.level)} | ${character.dead ? 'Dead' : 'In Field'}`
      : `Level ${integer(character.level)} | Offline · last known position`
    header.prepend(
      markerPortrait(character.portrait_url, character.name, 'portrait'),
    )
    details.append(
      detailRow('Group', character.group_name || '—'),
      detailRow('Zone', character.zone || '—'),
      detailRow(
        'Position',
        positionText(character.x, character.y, character.z),
      ),
      detailRow(
        'HP / MP',
        `${integer(character.hp)}/${integer(character.hp_max)} | ${integer(character.mp)}/${integer(character.mp_max)}`,
      ),
      detailRow(
        'Gold / SP',
        `${compactNumber(character.gold)} | ${compactNumber(character.sp)}`,
      ),
    )
    if (marker.placement === 'region-tile')
      details.append(detailRow('Map placement', 'Region tile only'))
    const actions = document.createElement('div')
    actions.className = 'phmon-map-detail-actions'
    const open = document.createElement('button')
    open.type = 'button'
    open.textContent = 'Open Stats'
    open.addEventListener('click', (event) => {
      event.stopPropagation()
      emit('opencharacter', character.character_id)
    })
    actions.append(open)
    panel.append(header, details, actions)
  } else if (marker.kind === 'party' && marker.party) {
    const party = marker.party
    title.textContent = party.name || 'Party member'
    subtitle.textContent = 'Party member'
    header.prepend(partyIconElement('phmon-map-detail-party-icon'))
    const rows: HTMLElement[] = []
    if (party.guild) rows.push(detailRow('Guild', party.guild))
    if (party.level != null) rows.push(detailRow('Level', String(party.level)))
    if (party.hp_percent != null)
      rows.push(detailRow('HP', `${party.hp_percent}%`))
    if (party.mp_percent != null)
      rows.push(detailRow('MP', `${party.mp_percent}%`))
    panel.append(header)
    if (rows.length) {
      details.append(...rows)
      panel.append(details)
    }
  } else if (marker.kind === 'player' && marker.player) {
    const other = marker.player
    title.textContent = other.name || `Player ${other.player_id}`
    subtitle.textContent = 'Other player'
    const icon = document.createElement('img')
    icon.className = 'phmon-map-detail-player-icon'
    icon.src = OTHER_PLAYER_ICON
    icon.alt = ''
    header.prepend(icon)
    const rows: HTMLElement[] = []
    if (other.guild) rows.push(detailRow('Guild', other.guild))
    if (other.grant) rows.push(detailRow('Grant', other.grant))
    if (other.level != null) rows.push(detailRow('Level', String(other.level)))
    rows.push(detailRow('State', playerAliveLabel(other.dead)))
    const observers = other.observers
      .map((observer) => observer.name)
      .filter(Boolean)
      .join(', ')
    details.append(
      detailRow('Zone', marker.zoneLabel || 'Unknown zone'),
      detailRow('Position', positionText(other.x, other.y, other.observer_z)),
      detailRow('Observed by', observers || '—'),
    )
    panel.append(header)
    if (rows.length) {
      details.append(...rows)
      panel.append(details)
    }
  } else if (marker.kind === 'npc' && marker.npc) {
    const npc = marker.npc
    title.textContent = npcDisplayLabel(npc)
    subtitle.textContent = npcRoleLabel(npc.role)
    const icon = document.createElement('img')
    icon.className = 'phmon-map-detail-npc-icon'
    icon.src = NPC_MARKER_ICON
    icon.alt = ''
    header.prepend(icon)
    const observers = npc.observers
      .map((observer) => observer.name)
      .filter(Boolean)
      .join(', ')
    details.append(
      detailRow('Server name', npc.servername || '—'),
      detailRow('Model', npc.model_id == null ? '—' : String(npc.model_id)),
      detailRow('Region', String(npc.region)),
      detailRow('Position', positionText(npc.x, npc.y, npc.observer_z)),
      detailRow('Observed by', observers || '—'),
    )
    const actions = document.createElement('div')
    actions.className = 'phmon-map-detail-actions'
    const navigate = document.createElement('button')
    navigate.type = 'button'
    navigate.textContent = 'Navigate here'
    navigate.addEventListener('click', (event) => {
      event.stopPropagation()
      const rect = navigate.getBoundingClientRect()
      emit('navigateto', marker.position, { x: rect.left, y: rect.bottom })
    })
    actions.append(navigate)
    if (npc.role === 'teleporter') {
      const teleport = document.createElement('button')
      teleport.type = 'button'
      teleport.textContent = 'Teleport to…'
      teleport.addEventListener('click', (event) => {
        event.stopPropagation()
        const rect = teleport.getBoundingClientRect()
        emit('teleportto', npc, marker.position, {
          x: rect.left,
          y: rect.bottom,
        })
      })
      actions.append(teleport)
    }
    panel.append(header, details, actions)
  } else if (marker.kind === 'monster' && marker.monster) {
    const monster = marker.monster
    title.textContent = monsterDisplayName(monster)
    subtitle.textContent = `${monsterTypePresentation(monster).label} · ${marker.zoneLabel || 'Unknown zone'}`
    const dot = document.createElement('span')
    dot.className = 'phmon-map-detail-monster-dot'
    header.prepend(dot)
    details.append(
      detailRow(
        'Level',
        monster.level == null ? 'Unavailable' : String(monster.level),
      ),
      detailRow('Position', positionText(monster.x, monster.y, monster.z)),
      detailRow('Seen by', marker.observerName || 'Unavailable'),
    )
    panel.append(header, details)
    const fraction = monsterHPBarFraction(monster)
    if (fraction != null) {
      const track = document.createElement('div')
      track.className = 'phmon-map-detail-hp-track'
      const fill = document.createElement('span')
      fill.style.width = `${(fraction * 100).toFixed(1)}%`
      const label = document.createElement('strong')
      label.className = 'phmon-map-detail-hp-label'
      label.textContent = `HP ${integer(monster.hp)} / ${integer(monster.max_hp)}`
      track.append(fill, label)
      panel.append(track)
    }
  } else {
    title.textContent = marker.itemName || marker.label
    subtitle.textContent =
      marker.kind === 'drop'
        ? 'World drop'
        : marker.kind === 'death'
          ? 'Death'
          : 'Event'
    const icon =
      marker.kind === 'drop'
        ? markerPortrait(marker.itemIconUrl, marker.itemName || 'Item', 'item')
        : markerPortrait(
            marker.event?.portrait_url,
            marker.event?.character || 'Event',
            'portrait',
          )
    if (marker.event) {
      const age = Math.max(
        0,
        Math.floor((Date.now() - Date.parse(marker.event.occurred_at)) / 60000),
      )
      subtitle.textContent += Number.isFinite(age) ? ` · ${age}m ago` : ''
    }
    header.prepend(icon)
    if (marker.event) {
      details.append(
        detailRow('Character', marker.event.character || '—'),
        detailRow('Zone', marker.zoneLabel || marker.event.zone || '—'),
        detailRow(
          'Position',
          positionText(marker.event.x, marker.event.y, marker.event.z),
        ),
      )
    }
    panel.append(header, details)
  }
  return panel
}

function markerIconContent(
  marker: MapCanvasMarker,
  openMonsterPopup?: () => void,
) {
  const content = document.createElement('span')
  if (marker.kind === 'character') {
    content.className = 'phmon-map-character-pin'
    if (marker.selected)
      content.classList.add('phmon-map-character-pin--selected')
    if (marker.character?.online === false)
      content.classList.add('phmon-map-character-pin--offline')
    if (marker.character?.position_stale)
      content.classList.add('phmon-map-character-pin--stale')
    const fallback = document.createElement('span')
    fallback.textContent =
      marker.character?.name.slice(0, 1).toUpperCase() || 'C'
    content.append(fallback)
    const portrait = localMapAsset(marker.character?.portrait_url, 'portrait')
    if (portrait) {
      const img = document.createElement('img')
      img.src = portrait
      img.alt = ''
      img.onerror = () => img.remove()
      content.append(img)
    }
    const name = document.createElement('span')
    name.className = 'phmon-map-character-name'
    name.textContent = marker.character?.name || marker.label
    content.append(name)
  } else if (marker.kind === 'party') {
    content.className = 'phmon-map-party-icon'
    const img = document.createElement('img')
    img.src = PARTY_MEMBER_ICON
    img.alt = ''
    styleMinimapSignImage(img)
    content.append(img)
    const name = document.createElement('span')
    name.className = 'phmon-map-party-name'
    name.textContent = marker.party?.name?.trim() || marker.label
    content.append(name)
  } else if (marker.kind === 'player') {
    content.className = 'phmon-map-player-icon'
    const img = document.createElement('img')
    img.src = OTHER_PLAYER_ICON
    img.alt = ''
    styleMinimapSignImage(img)
    content.append(img)
    const name = document.createElement('span')
    name.className = 'phmon-map-player-name'
    name.textContent = marker.player?.name?.trim() || marker.label
    content.append(name)
  } else if (marker.kind === 'npc') {
    content.className = 'phmon-map-npc-icon'
    const img = document.createElement('img')
    img.src = NPC_MARKER_ICON
    img.alt = ''
    styleMinimapSignImage(img)
    content.append(img)
    const name = document.createElement('span')
    name.className = 'phmon-map-npc-name'
    name.textContent = marker.npc ? npcDisplayLabel(marker.npc) : marker.label
    content.append(name)
  } else if (marker.kind === 'monster') {
    content.className = 'phmon-map-monster-bubble'
    if (openMonsterPopup) {
      content.addEventListener('click', (event) => {
        event.stopPropagation()
        openMonsterPopup()
      })
    }
    const fraction = marker.monster && monsterHPFraction(marker.monster)
    content.style.setProperty('--phmon-monster-hp', String(fraction ?? 0))
    if (marker.monster) {
      const presentation = monsterTypePresentation(marker.monster)
      const mapName = monsterMapName(marker.monster, Boolean(marker.showLabel))
      if (mapName) {
        const name = document.createElement('span')
        name.className = 'phmon-map-monster-name'
        const appendLabelIcon = (src: string, className: string) => {
          if (!src) return
          const icon = document.createElement('img')
          icon.className = className
          icon.src = src
          icon.alt = ''
          icon.onerror = () => icon.remove()
          name.append(icon)
        }
        appendLabelIcon(presentation.iconUrl, 'phmon-map-monster-rank-icon')
        appendLabelIcon(
          presentation.partyBadgeUrl,
          'phmon-map-monster-party-badge',
        )
        const text = document.createElement('span')
        text.className = 'phmon-map-monster-name-text'
        text.textContent = mapName
        name.append(text)
        content.append(name)
      }
    }
  } else if (marker.kind === 'drop') {
    content.className = 'phmon-map-drop-icon'
    const icon = localMapAsset(marker.itemIconUrl, 'item')
    if (icon) {
      const img = document.createElement('img')
      img.src = icon
      img.alt = ''
      img.onerror = () => img.remove()
      content.append(img)
    } else {
      content.textContent = '✦'
    }
    const badge = document.createElement('span')
    badge.className = 'phmon-map-drop-badge'
    badge.textContent = '✦'
    content.append(badge)
  } else if (marker.kind === 'death') {
    content.className = 'phmon-map-death-icon'
    const img = document.createElement('img')
    img.src = '/game-assets/icon/item/etc/etc_helmet_stone.png'
    img.alt = ''
    img.onerror = () => img.remove()
    content.append(img)
    const portrait = localMapAsset(marker.event?.portrait_url, 'portrait')
    const badge = document.createElement('span')
    badge.className = 'phmon-map-death-badge'
    if (portrait) {
      const badgeImage = document.createElement('img')
      badgeImage.src = portrait
      badgeImage.alt = ''
      badgeImage.onerror = () => badgeImage.remove()
      badge.append(badgeImage)
    }
    content.append(badge)
  }
  return content
}

function referenceLatLng(position: {
  tile_x: number
  tile_y: number
  pixel_x: number
  pixel_y: number
}) {
  return leaflet!.latLng(
    -((props.profile.tiles.max_y - position.tile_y) * 256 + position.pixel_y),
    (position.tile_x - props.profile.tiles.min_x) * 256 + position.pixel_x,
  )
}

function tileRectangle(cell: {
  x: number
  y: number
  width: number
  height: number
}) {
  const west = (cell.x - props.profile.tiles.min_x) * 256
  const east = west + cell.width * 256
  const north = -(props.profile.tiles.max_y - (cell.y + cell.height - 1)) * 256
  const south = -(props.profile.tiles.max_y - cell.y + 1) * 256
  return leaflet!.latLngBounds([south, west], [north, east])
}

function referencePopup(
  rows: Array<{ name: string; code: string; level?: number }>,
  kind: 'area' | 'point',
) {
  const panel = document.createElement('section')
  panel.className = 'phmon-monster-reference-popup'
  const heading = document.createElement('strong')
  heading.textContent =
    kind === 'area'
      ? 'Mob area · client guide'
      : 'Exact spawn · client reference point'
  panel.append(heading)
  for (const row of rows) {
    const line = document.createElement('div')
    line.textContent = `${row.name || row.code} · ${row.level == null ? 'Level unavailable' : `Lv ${row.level}`}`
    const code = document.createElement('small')
    code.textContent = row.code
    line.append(code)
    panel.append(line)
  }
  const source = document.createElement('p')
  source.textContent = `Dataset ${props.profile.dataset_id}. Reference coverage; current presence is unverified.`
  panel.append(source)
  return panel
}

function syncMonsterReferences() {
  if (
    !map ||
    !leaflet ||
    !referenceAreaLayer ||
    !referenceLabelLayer ||
    !referencePointLayer
  )
    return
  const L = leaflet
  referenceAreaLayer.clearLayers()
  referenceLabelLayer.clearLayers()
  referencePointLayer.clearLayers()
  const areas = props.referenceAreas || []
  const indexed = areas.flatMap((area) =>
    area.cells.map((cell) => ({ area, cell })),
  )
  const displayed = displayedGuideCells(
    indexed.map((item) => item.cell),
    {
      minX: props.profile.tiles.min_x,
      maxX: props.profile.tiles.max_x,
      minY: props.profile.tiles.min_y,
      maxY: props.profile.tiles.max_y,
      guideOriginY: props.profile.guide_origin_y,
    },
    props.referenceGuideRows?.map((row) => ({
      y: row.y,
      minX: row.min_x,
      maxX: row.max_x,
    })),
  )
  const drawn = indexed.map((item, index) => ({
    area: item.area,
    cell: displayed[index]!,
  }))
  for (const item of drawn) {
    L.rectangle(tileRectangle(item.cell), {
      color: '#e4dba8',
      weight: 1,
      opacity: 0.42,
      fillColor: '#fef6c3',
      fillOpacity: 0.18,
      interactive: true,
    })
      .bindPopup(referencePopup([item.area], 'area'), {
        className: 'phmon-map-popup phmon-monster-reference-frame',
        maxWidth: 280,
      })
      .addTo(referenceAreaLayer)
  }
  const labelCandidates = drawn
    .map((item) => {
      const placed = item.cell
      const center = L.latLng(
        -(props.profile.tiles.max_y - placed.y - placed.height / 2 + 1) * 256,
        (placed.x - props.profile.tiles.min_x + placed.width / 2) * 256,
      )
      return {
        area: item.area,
        center,
        pixel: map!.latLngToContainerPoint(center),
      }
    })
    .filter(
      ({ pixel }) =>
        pixel.x >= -130 &&
        pixel.y >= -58 &&
        pixel.x <= map!.getSize().x + 130 &&
        pixel.y <= map!.getSize().y + 58,
    )
  const groups = new Map<string, typeof labelCandidates>()
  const compact = mapZoomPercentForLevel(map.getZoom()) < 100
  for (const candidate of labelCandidates) {
    const x = Math.floor(candidate.pixel.x / (compact ? 90 : 130))
    const y = Math.floor(candidate.pixel.y / (compact ? 45 : 58))
    const key = `${x}:${y}`
    const group = groups.get(key) || []
    if (!group.some((entry) => entry.area.model_id === candidate.area.model_id))
      group.push(candidate)
    groups.set(key, group)
  }
  for (const group of groups.values()) {
    const label = document.createElement('span')
    label.className = 'phmon-monster-area-label'
    if (compact) {
      label.textContent = `${group.length} mob${group.length === 1 ? '' : 's'}`
    } else {
      for (const entry of group.slice(0, 3)) {
        const line = document.createElement('span')
        line.textContent = `${entry.area.name || entry.area.code} · ${entry.area.level == null ? 'Lv —' : `Lv ${entry.area.level}`}`
        label.append(line)
      }
      if (group.length > 3) {
        const more = document.createElement('span')
        more.textContent = `+${group.length - 3} more`
        label.append(more)
      }
    }
    const icon = L.divIcon({
      html: label,
      className: 'phmon-monster-area-label-icon',
      iconSize: [130, 36],
      iconAnchor: [65, 18],
    })
    L.marker(group[0]!.center, { icon, keyboard: true, zIndexOffset: -300 })
      .bindPopup(
        referencePopup(
          group.map((entry) => entry.area),
          'area',
        ),
        {
          className: 'phmon-map-popup phmon-monster-reference-frame',
          maxWidth: 280,
        },
      )
      .addTo(referenceLabelLayer)
  }
  const pointGroups = new Map<string, MonsterReferencePoint[]>()
  for (const point of props.referencePoints || []) {
    const position = referenceLatLng(point.position)
    const pixel = map.latLngToContainerPoint(position)
    const key = `${Math.floor(pixel.x / 22)}:${Math.floor(pixel.y / 22)}`
    const group = pointGroups.get(key) || []
    group.push(point)
    pointGroups.set(key, group)
  }
  for (const group of pointGroups.values()) {
    const first = group[0]!
    const marker = L.circleMarker(referenceLatLng(first.position), {
      renderer: heatRenderer,
      radius: group.length > 1 ? 7 : 4,
      color: '#fef6c3',
      weight: 1.5,
      fillColor: '#597b9c',
      fillOpacity: 0.92,
    })
    marker
      .bindPopup(referencePopup(group, 'point'), {
        className: 'phmon-map-popup phmon-monster-reference-frame',
        maxWidth: 280,
      })
      .addTo(referencePointLayer)
    if (group.length > 1)
      marker.bindTooltip(String(group.length), {
        permanent: true,
        direction: 'center',
        className: 'phmon-monster-point-count',
      })
  }
}

function focusReferenceBounds() {
  if (!map || !leaflet || !props.referenceFocus) return
  const bounds = props.referenceFocus
  const southWest = tileRectangle({
    x: bounds.min_x,
    y: bounds.min_y,
    width: 1,
    height: 1,
  }).getSouthWest()
  const northEast = tileRectangle({
    x: bounds.max_x,
    y: bounds.max_y,
    width: 1,
    height: 1,
  }).getNorthEast()
  map.fitBounds(leaflet.latLngBounds(southWest, northEast), {
    padding: [32, 32],
    maxZoom: mapZoomLevelForPercent(225),
  })
}

onMounted(async () => {
  if (
    !element.value ||
    props.profile.tiles.status !== 'available-for-inspection'
  )
    return
  const L = (await import('leaflet')).default
  if (stopped || !element.value) return
  leaflet = L
  createLatLng = L.latLng
  makeHeatCircle = L.circleMarker
  makePolyline = L.polyline
  makeRouteCircle = L.circleMarker
  heatRenderer = L.canvas({ padding: 0.5 })
  const content = rasterContentBounds()
  const bounds = L.latLngBounds(
    L.latLng(content.south, content.west),
    L.latLng(content.north, content.east),
  )
  map = L.map(element.value, {
    crs: L.CRS.Simple,
    ...MAP_ZOOM_OPTIONS,
    zoomSnap: 0,
    zoomDelta: 0.25,
    maxBounds: bounds,
    maxBoundsViscosity: 0.8,
    keyboard: true,
    zoomControl: !props.compact && !props.externalControls,
    attributionControl: false,
    preferCanvas: true,
  })
  const tiles = L.GridLayer.extend({
    options: {
      tileSize: 256,
      ...MAP_ZOOM_OPTIONS,
      noWrap: true,
      keepBuffer: 2,
    },
    createTile(
      coords: { x: number; y: number; z: number },
      done: (error: Error | null, tile: HTMLElement) => void,
    ) {
      const canvas = document.createElement('canvas')
      canvas.width = 256
      canvas.height = 256
      const ctx = canvas.getContext('2d')
      if (!ctx) {
        queueMicrotask(() => done(null, canvas))
        return canvas
      }
      const factor = coords.z < 0 ? 2 ** -coords.z : 1
      const pieces = factor * factor
      let remaining = pieces
      const complete = () => {
        remaining--
        if (remaining === 0) done(null, canvas)
      }
      const drawSourceTile = (
        tileX: number,
        tileY: number,
        sourceX: number,
        sourceY: number,
        sourceSize: number,
        destinationX: number,
        destinationY: number,
        destinationSize: number,
      ) => {
        if (
          tileX < props.profile.tiles.min_x ||
          tileX > props.profile.tiles.max_x ||
          tileY < props.profile.tiles.min_y ||
          tileY > props.profile.tiles.max_y
        ) {
          complete()
          return
        }
        const image = new Image()
        image.onload = () => {
          ctx.drawImage(
            image,
            sourceX,
            sourceY,
            sourceSize,
            sourceSize,
            destinationX,
            destinationY,
            destinationSize,
            destinationSize,
          )
          complete()
        }
        image.onerror = complete
        const url = props.profile.tiles.tile_url_format
          .replace('{x}', String(tileX))
          .replace('{y}', String(tileY))
        image.src = /^\/game-assets\/minimap(?:_d)?\/[a-z0-9_/-]+\.png$/.test(
          url,
        )
          ? url
          : ''
      }
      if (coords.z < 0) {
        const pieceSize = 256 / factor
        for (let row = 0; row < factor; row++) {
          for (let column = 0; column < factor; column++) {
            drawSourceTile(
              props.profile.tiles.min_x + coords.x * factor + column,
              props.profile.tiles.max_y - (coords.y * factor + row),
              0,
              0,
              256,
              column * pieceSize,
              row * pieceSize,
              pieceSize,
            )
          }
        }
      } else {
        const scale = 2 ** coords.z
        const baseColumn = Math.floor(coords.x / scale)
        const baseRow = Math.floor(coords.y / scale)
        const tileX = props.profile.tiles.min_x + baseColumn
        const tileY = props.profile.tiles.max_y - baseRow
        const offsetX = ((coords.x % scale) + scale) % scale
        const offsetY = ((coords.y % scale) + scale) % scale
        const sourceSize = 256 / scale
        drawSourceTile(
          tileX,
          tileY,
          offsetX * sourceSize,
          offsetY * sourceSize,
          sourceSize,
          0,
          0,
          256,
        )
      }
      return canvas
    },
  })
  new tiles().addTo(map)
  referenceAreaLayer = L.layerGroup().addTo(map)
  heatLayerGroup = L.layerGroup().addTo(map)
  referencePointLayer = L.layerGroup().addTo(map)
  referenceLabelLayer = L.layerGroup().addTo(map)
  trainingLayerGroup = L.layerGroup().addTo(map)
  navigationLayerGroup = L.layerGroup().addTo(map)
  markerLayer = L.layerGroup().addTo(map)
  makeMarker = (marker, point, existing) => {
    const markerKey = `${marker.kind}:${marker.id}`
    const type = marker.monster ? monsterTypePresentation(marker.monster) : null
    const hp = marker.monster ? monsterHPFraction(marker.monster) : null
    const size =
      marker.kind === 'character'
        ? 28
        : marker.kind === 'party' ||
            marker.kind === 'player' ||
            marker.kind === 'npc'
          ? MINIMAP_SIGN_ICON_PX
          : marker.kind === 'monster'
            ? Math.round(12 * (type?.scale || 1))
            : marker.kind === 'drop'
              ? 36
              : marker.kind === 'death'
                ? 34
                : 16
    const signature = JSON.stringify([
      marker.kind,
      marker.placement,
      marker.character?.name,
      marker.character?.portrait_url,
      marker.character?.online,
      marker.character?.position_stale,
      marker.selected,
      marker.npc ? npcDisplayLabel(marker.npc) : '',
      marker.npc?.role,
      marker.npc?.servername,
      marker.npc?.teleport_routes?.map((route) => route.destination),
      type?.code,
      type?.scale,
      type?.party,
      type?.iconUrl,
      type?.partyBadgeUrl,
      type?.unknown,
      marker.monster
        ? monsterMapName(marker.monster, Boolean(marker.showLabel))
        : '',
      hp == null,
      marker.monster?.max_hp,
      marker.monster?.attacking,
      marker.itemIconUrl,
    ])
    let popupMarker = existing
    const icon =
      !existing || markerIconSignatures.get(markerKey) !== signature
        ? L.divIcon({
            className: [
              'phmon-map-marker',
              `phmon-map-marker--${marker.kind}`,
              marker.placement === 'region-tile'
                ? 'phmon-map-marker--region-tile'
                : '',
              type?.party ? 'phmon-map-marker--party' : '',
              type?.unknown ? 'phmon-map-marker--unknown' : '',
              hp == null ? 'phmon-map-marker--hp-unavailable' : '',
              marker.monster?.attacking ? 'phmon-map-marker--attacking' : '',
            ]
              .filter(Boolean)
              .join(' '),
            iconSize: [size, size],
            iconAnchor: [size / 2, size / 2],
            html: markerIconContent(
              marker,
              !props.compact && marker.kind === 'monster'
                ? () => popupMarker?.openPopup()
                : undefined,
            ),
          })
        : undefined
    markerIconSignatures.set(markerKey, signature)
    if (existing) {
      moveMarker(markerKey, existing, point)
      if (icon) existing.setIcon(icon)
      if (marker.monster) {
        const bubble = existing
          .getElement()
          ?.querySelector<HTMLElement>('.phmon-map-monster-bubble')
        bubble?.style.setProperty('--phmon-monster-hp', (hp ?? 0).toFixed(4))
      }
      if (!props.compact && marker.kind !== 'character')
        existing.setPopupContent(markerPopup(marker))
      return existing
    }
    const rendered = L.marker(point, {
      icon: icon!,
      title: marker.label,
      keyboard: true,
      riseOnHover: true,
      bubblingMouseEvents: false,
      zIndexOffset:
        marker.kind === 'character'
          ? 1000
          : marker.kind === 'party'
            ? 700
            : marker.kind === 'player'
              ? 680
              : marker.kind === 'npc'
                ? 650
                : marker.kind === 'drop' || marker.kind === 'death'
                  ? 400
                  : 0,
    })
    if (!props.compact && marker.kind === 'character')
      rendered.on('click', () => emit('inspectcharacter', marker.id))
    if (
      !props.compact &&
      marker.kind === 'npc' &&
      marker.npc?.role === 'teleporter'
    ) {
      rendered.on('contextmenu', (event: L.LeafletMouseEvent) => {
        L.DomEvent.preventDefault(event.originalEvent)
        L.DomEvent.stopPropagation(event.originalEvent)
        emit('teleportercontext', marker.npc!, marker.position, {
          x: event.originalEvent.clientX,
          y: event.originalEvent.clientY,
        })
      })
    }
    if (!props.compact && marker.kind !== 'character')
      rendered.bindPopup(markerPopup(marker), {
        className: 'phmon-map-popup',
        closeButton: false,
        maxWidth: 280,
        offset: [0, -10],
        autoPan: false,
      })
    rendered.addTo(markerLayer!)
    popupMarker = rendered
    return rendered
  }

  setInitialView()
  focusReferenceBounds()
  const selectPoint = (position: LatLng) =>
    emit('pointselect', indexAt(position))
  const onKeydown = (event: KeyboardEvent) => {
    if (event.target !== element.value) return
    if (
      event.key === 'ContextMenu' ||
      (event.shiftKey && event.key === 'F10')
    ) {
      event.preventDefault()
      if (!map || props.compact) return
      const rect = element.value!.getBoundingClientRect()
      const point = indexAt(map.getCenter())
      emit('pointselect', point)
      emit('contextaction', {
        point,
        anchor: {
          x: rect.left + rect.width / 2,
          y: rect.top + rect.height / 2,
        },
      })
      return
    }
    if (event.key !== 'Enter' && event.key !== ' ') return
    event.preventDefault()
    if (map) selectPoint(map.getCenter())
  }
  const onMiddleButton = (event: MouseEvent) => {
    if (event.button !== 1) return
    event.preventDefault()
  }
  element.value.addEventListener('keydown', onKeydown)
  element.value.addEventListener('mousedown', onMiddleButton, true)
  element.value.addEventListener('auxclick', onMiddleButton, true)
  map.once('unload', () => {
    element.value?.removeEventListener('keydown', onKeydown)
    element.value?.removeEventListener('mousedown', onMiddleButton, true)
    element.value?.removeEventListener('auxclick', onMiddleButton, true)
  })
  map.on('click', (event: L.LeafletMouseEvent) => {
    selectPoint(event.latlng)
  })
  map.on('contextmenu', (event: L.LeafletMouseEvent) => {
    L.DomEvent.preventDefault(event.originalEvent)
    selectPoint(event.latlng)
    emit('contextaction', {
      point: indexAt(event.latlng),
      anchor: {
        x: event.originalEvent.clientX,
        y: event.originalEvent.clientY,
      },
    })
  })
  map.on('dragstart', () => emit('mapdrag'))
  map.on('movestart zoomstart', () => {
    viewAdjusted = true
  })
  map.on('zoomend', syncPanBounds)
  map.on('zoomend', snapZoomToPercentStep)
  map.on('zoomend', layoutTrainingLabels)
  map.on('moveend zoomend', publishView)
  map.on('moveend zoomend', layoutCharacterLabels)
  map.on('zoomend', syncMonsterReferences)
  publishView()
  syncHeatLayers()
  syncMonsterReferences()
  syncTrainingAreas()
  syncNavigationRoutes()
  syncMarkers()
  canvasResizeObserver = new ResizeObserver(() => {
    map?.invalidateSize({ pan: false })
    syncPanBounds()
    layoutCharacterLabels()
  })
  canvasResizeObserver.observe(element.value)
  syncPanBounds()
  if (props.compact) {
    map.dragging.disable()
    map.scrollWheelZoom.disable()
    map.doubleClickZoom.disable()
    map.touchZoom.disable()
    map.boxZoom.disable()
    map.keyboard.disable()
  }
})

watch(
  [() => props.initialPosition, () => props.focusRequest],
  ([position, focusRequest]) => {
    if (
      (focusRequest || 0) !== lastFocusRequest ||
      (position &&
        ((props.compact &&
          `${position.tileX}:${position.tileY}` !== lastFocusedTile) ||
          (!initialPositionApplied && !viewAdjusted)))
    )
      setInitialView()
  },
  { deep: true },
)
watch(() => props.markers, syncMarkers, { deep: true })
watch(() => props.heatLayers, syncHeatLayers, { deep: true })
watch(
  [
    () => props.referenceAreas,
    () => props.referenceGuideRows,
    () => props.referencePoints,
  ],
  syncMonsterReferences,
  { deep: true },
)
watch(() => props.referenceFocus, focusReferenceBounds, { deep: true })
watch(() => props.navigationRoutes, syncNavigationRoutes, { deep: true })
watch(
  [
    () => props.trainingAreas,
    () => props.trainingEditable,
    () => props.trainingAcceptDisabled,
    () => props.trainingDiscardDisabled,
    () => props.trainingAcceptTitle,
  ],
  syncTrainingAreas,
  { deep: true },
)

function chooseClusterCharacter(id: string) {
  emit('inspectcharacter', id)
  clusterChoices.value = []
}
function focusCanvas() {
  element.value?.focus()
}

function zoomBy(step: number) {
  if (map)
    map.setZoom(
      mapZoomLevelForPercent(
        snapMapZoomPercent(
          mapZoomPercentForLevel(map.getZoom()) + step * MAP_ZOOM_PERCENT_STEP,
        ),
      ),
    )
}
function focusAt(point: RasterPosition) {
  if (map && createLatLng)
    map.panTo(
      createLatLng(
        -((props.profile.tiles.max_y - point.tileY) * 256 + point.pixelY),
        (point.tileX - props.profile.tiles.min_x) * 256 + point.pixelX,
      ),
    )
}
watch(() => props.focusedCharacterID, layoutCharacterLabels)
defineExpose({
  focus: focusCanvas,
  zoomIn: () => zoomBy(1),
  zoomOut: () => zoomBy(-1),
  focusAt,
})

onBeforeUnmount(() => {
  stopped = true
  canvasResizeObserver?.disconnect()
  for (const frame of markerAnimationFrames.values())
    cancelAnimationFrame(frame)
  markerAnimationFrames.clear()
  map?.remove()
  map = undefined
  heatLayerGroup = undefined
  referenceAreaLayer = undefined
  referenceLabelLayer = undefined
  referencePointLayer = undefined
  markerLayer = undefined
  navigationLayerGroup = undefined
  trainingLayerGroup = undefined
  leaflet = undefined
  renderedTraining.clear()
  trainingDragging = ''
  heatRenderer = undefined
  makeHeatCircle = undefined
  makePolyline = undefined
  makeRouteCircle = undefined
  makeMarker = undefined
  renderedMarkers.clear()
  markerIconSignatures.clear()
})
</script>

<template>
  <div
    ref="element"
    class="map-canvas"
    :class="{ 'map-canvas-compact': compact }"
    :aria-label="
      compact
        ? 'Map tile preview'
        : 'Interactive raster map. Use arrow keys to pan and Shift+F10 or right-click to open map actions.'
    "
    role="application"
    tabindex="0"
  >
    <section
      v-if="clusterChoices.length"
      class="panel map-cluster-choices"
      aria-label="Overlapping characters"
      @pointerdown.stop
      @click.stop
      @keydown.esc.stop="clusterChoices = []"
    >
      <div>
        Choose character
        <button
          class="compact-button"
          type="button"
          aria-label="Close overlapping characters"
          @click="clusterChoices = []"
        >
          ×
        </button>
      </div>
      <button
        v-for="choice in clusterChoices"
        :key="choice.id"
        class="compact-button"
        type="button"
        @click="chooseClusterCharacter(choice.id)"
      >
        {{ choice.name }}
      </button>
    </section>
  </div>
</template>

<style scoped>
:global(.phmon-monster-area-label-icon.leaflet-div-icon) {
  border: 0;
  background: transparent;
}
:global(.phmon-monster-area-label) {
  display: grid;
  gap: 1px;
  min-width: 92px;
  max-width: 180px;
  padding: 3px 6px;
  color: #fef6c3;
  background: rgb(13 19 29 / 84%);
  border: 1px solid rgb(254 246 195 / 58%);
  border-radius: 4px;
  font:
    600 11px/1.2 'Segoe UI',
    Tahoma,
    Arial,
    sans-serif;
  text-align: center;
  white-space: nowrap;
  box-shadow: 0 2px 5px rgb(0 0 0 / 35%);
}
:global(.phmon-monster-area-label > span) {
  overflow: hidden;
  text-overflow: ellipsis;
}
:global(.phmon-monster-reference-popup) {
  display: grid;
  gap: 4px;
  min-width: 190px;
  color: #eaf1ff;
}
:global(.phmon-monster-reference-frame .leaflet-popup-content) {
  box-sizing: border-box;
  padding: 12px;
}
:global(.phmon-monster-reference-popup strong) {
  color: #fef6c3;
}
:global(.phmon-monster-reference-popup small) {
  display: block;
  opacity: 0.7;
}
:global(.phmon-monster-reference-popup p) {
  margin: 4px 0 0;
  font-size: 11px;
  opacity: 0.8;
}
:global(.phmon-monster-point-count) {
  border: 0;
  background: transparent;
  color: #fff;
  box-shadow: none;
  font-weight: 700;
}
.map-canvas {
  width: 100%;
  height: 100%;
  min-height: 340px;
  background: #101923;
  outline: none;
}

.map-canvas:focus-visible {
  box-shadow: inset 0 0 0 2px #9bc8ff;
}

.map-canvas-compact {
  min-height: 112px;
  height: 112px;
  pointer-events: none;
}

:global(.phmon-map-marker) {
  border: 0;
  background: transparent;
}

:global(.phmon-map-marker--party.leaflet-div-icon),
:global(.phmon-map-marker--player.leaflet-div-icon),
:global(.phmon-map-marker--npc.leaflet-div-icon),
:global(.phmon-map-marker--monster.leaflet-div-icon) {
  overflow: visible;
}

/* Leaflet positions the marker with `transform`; `translate` composes with it. */
:global(.phmon-map-training-label-marker) {
  width: max-content;
  border: 0;
  background: transparent;
  outline: none;
  translate: -50% calc(-100% - 3px);
}

:global(.phmon-map-training-label) {
  display: inline-flex;
  align-items: center;
  gap: 4px;
  padding: 1px 6px;
  border: 1px solid #4db9ffb3;
  border-radius: 3px;
  background: #0b1a29e6;
  color: #d8eeff;
  font-size: 11px;
  font-weight: 600;
  line-height: 16px;
  white-space: nowrap;
  cursor: pointer;
  box-shadow: 0 1px 4px #0009;
}

:global(.phmon-map-training-label--selected) {
  border-color: #9fdcff;
  background: #123a5ce6;
  color: #ffffff;
}

:global(.phmon-map-training-label-actions) {
  display: inline-flex;
  align-items: center;
  gap: 1px;
}

:global(.phmon-map-training-label-action) {
  display: inline-grid;
  place-items: center;
  width: 16px;
  height: 16px;
  margin: 0;
  padding: 0;
  border: 0;
  border-radius: 2px;
  background: transparent;
  color: #d8eeff;
  cursor: pointer;
}

:global(.phmon-map-training-label-action svg) {
  display: block;
  width: 12px;
  height: 12px;
}

:global(.phmon-map-training-label-action--discard) {
  color: #ffc9c9;
}

:global(.phmon-map-training-label-action--accept) {
  color: #9eecc4;
}

:global(.phmon-map-training-label-action:hover),
:global(.phmon-map-training-label-action:focus-visible) {
  background: #ffffff22;
  outline: none;
  box-shadow: 0 0 0 1px #9bc8ff;
}

:global(.phmon-map-training-label-action:disabled) {
  opacity: 0.4;
  cursor: not-allowed;
}

:global(.phmon-map-training-label-status) {
  color: #fef6c3;
  font-weight: 500;
}

:global(
  .phmon-map-training-label-marker:focus-visible .phmon-map-training-label
) {
  box-shadow: 0 0 0 2px #9bc8ff;
}

:global(.phmon-map-training-handle) {
  box-sizing: border-box;
  border: 2px solid #ffffff;
  border-radius: 50%;
  background: #4db9ff;
  box-shadow:
    0 0 0 1px #0b1a29,
    0 1px 4px #000a;
  cursor: grab;
}

:global(.phmon-map-training-handle--edge) {
  border-radius: 2px;
  cursor: ew-resize;
}

:global(.phmon-map-character-pin) {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  border: 2px solid #58bd8a;
  border-radius: 50%;
  background: #111923;
  color: #fff;
  font-size: 12px;
  font-weight: 700;
  box-shadow:
    0 0 0 2px #07111bcc,
    0 2px 7px #000b;
  position: relative;
}

:global(.phmon-map-character-pin--selected) {
  border-color: #4db9ff;
}

:global(.phmon-map-character-pin--offline) {
  background: #343a37;
  color: #d6dad7;
  box-shadow:
    0 0 0 2px #07111bcc,
    0 2px 7px #000b;
}

:global(.phmon-map-character-pin--stale) {
  background: #34332e;
  color: #e4d9bd;
}

:global(.phmon-map-character-pin--stale img) {
  filter: saturate(0.5);
  opacity: 0.8;
}

:global(.phmon-map-character-pin img) {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  border-radius: 50%;
  object-fit: cover;
}

:global(.phmon-map-character-pin--offline img) {
  filter: grayscale(1);
  opacity: 0.68;
}

:global(.phmon-map-character-name) {
  position: absolute;
  top: calc(100% + 3px);
  left: 50%;
  transform: translateX(-50%);
  max-width: 105px;
  padding: 2px 5px;
  border-radius: 7px;
  background: #0d1119ec;
  color: #fff;
  font-size: 10px;
  font-weight: 500;
  line-height: 1.1;
  white-space: nowrap;
  overflow: hidden;
  text-overflow: ellipsis;
  box-shadow: 0 1px 4px #000b;
}

:global(.phmon-map-character-pin--offline .phmon-map-character-name) {
  color: #d4d8d5;
  background: #252b29ed;
}

:global(.phmon-map-marker--region-tile .phmon-map-character-pin) {
  border-style: dashed;
}

:global(.phmon-map-party-icon),
:global(.phmon-map-detail-party-icon) {
  display: grid;
  place-items: center;
}

:global(.phmon-map-party-icon) {
  position: relative;
  filter: drop-shadow(0 1px 3px #000c);
}

:global(.phmon-map-player-icon),
:global(.phmon-map-npc-icon) {
  display: flex;
  flex-direction: column;
  align-items: center;
  position: relative;
  filter: drop-shadow(0 1px 3px #000c);
}

:global(.phmon-map-detail-npc-icon) {
  width: 32px;
  height: 32px;
  object-fit: contain;
}

:global(.phmon-map-npc-name),
:global(.phmon-map-party-name),
:global(.phmon-map-player-name) {
  position: absolute;
  top: calc(100% + 2px);
  left: 50%;
  transform: translateX(-50%);
  width: max-content;
  max-width: 112px;
  overflow: hidden;
  padding: 2px 5px;
  border-radius: 7px;
  background: #0d1119ec;
  color: #eaf1ff;
  font-size: 10px;
  font-weight: 500;
  line-height: 1.1;
  text-align: center;
  text-overflow: ellipsis;
  white-space: nowrap;
  box-shadow: 0 1px 4px #000b;
  pointer-events: auto;
  cursor: pointer;
}

:global(.phmon-map-party-icon img),
:global(.phmon-map-player-icon img),
:global(.phmon-map-npc-icon img) {
  display: block;
  flex: none;
  image-rendering: pixelated;
  image-rendering: crisp-edges;
}

:global(.phmon-map-detail-party-icon img) {
  display: block;
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

:global(.phmon-map-detail-party-icon) {
  width: 32px;
  height: 32px;
}

:global(.phmon-map-monster-bubble) {
  display: block;
  position: relative;
  pointer-events: auto;
  cursor: pointer;
  width: 100%;
  height: 100%;
  box-sizing: border-box;
  border: 2px solid transparent;
  border-radius: 50%;
  background:
    radial-gradient(
        circle at 37% 32%,
        #ffafb0 0,
        #fa383c 25%,
        #95151a 76%,
        #350b12 100%
      )
      padding-box,
    conic-gradient(
        from -90deg,
        #72f077 0turn,
        #72f077 calc(var(--phmon-monster-hp) * 1turn),
        #324e39 calc(var(--phmon-monster-hp) * 1turn),
        #324e39 1turn
      )
      border-box;
  box-shadow:
    0 0 0 1px #101723ad,
    0 1px 4px #000a;
}

:global(.phmon-map-monster-rank-icon),
:global(.phmon-map-monster-party-badge) {
  flex: none;
  width: 12px;
  height: 12px;
  object-fit: contain;
  pointer-events: none;
}

:global(.phmon-map-marker--party .phmon-map-monster-bubble) {
  background:
    radial-gradient(
        circle at 37% 32%,
        #ffafb0 0,
        #fa383c 25%,
        #95151a 76%,
        #350b12 100%
      )
      padding-box,
    conic-gradient(
        from -90deg,
        #6baaff 0turn,
        #6baaff calc(var(--phmon-monster-hp) * 1turn),
        #304661 calc(var(--phmon-monster-hp) * 1turn),
        #304661 1turn
      )
      border-box;
}

:global(.phmon-map-monster-name) {
  display: inline-flex;
  align-items: center;
  gap: 3px;
  position: absolute;
  top: calc(100% + 5px);
  left: 50%;
  z-index: 3;
  transform: translateX(-50%);
  max-width: 148px;
  overflow: hidden;
  padding: 2px 4px 2px 3px;
  border-radius: 7px;
  background: #0d1119ec;
  color: #fff;
  font-size: 10px;
  font-weight: 500;
  line-height: 1.1;
  white-space: nowrap;
  box-shadow: 0 1px 4px #000b;
  pointer-events: auto;
  cursor: pointer;
}

:global(.phmon-map-monster-name-text) {
  min-width: 0;
  overflow: hidden;
  text-overflow: ellipsis;
}

:global(.phmon-map-marker--hp-unavailable .phmon-map-monster-bubble) {
  border-color: #9380ae;
}

:global(.phmon-map-marker--attacking .phmon-map-monster-bubble) {
  box-shadow:
    0 0 0 1px #f39040,
    0 0 8px #f04c3d;
}

:global(.phmon-map-drop-icon),
:global(.phmon-map-death-icon) {
  display: grid;
  place-items: center;
  width: 100%;
  height: 100%;
  position: relative;
  color: #f6d780;
  font-size: 20px;
}

:global(.phmon-map-drop-icon) {
  border: 1px solid #fff0c48c;
  border-radius: 6px;
  background: #090909eb;
  box-shadow: 0 2px 6px #0008;
}

:global(.phmon-map-drop-icon img),
:global(.phmon-map-death-icon > img) {
  max-width: 100%;
  max-height: 100%;
  object-fit: contain;
}

:global(.phmon-map-drop-badge) {
  position: absolute;
  right: -2px;
  bottom: -3px;
  display: grid;
  place-items: center;
  width: 15px;
  height: 15px;
  border-radius: 3px;
  background: #a87919;
  color: #fff7ce;
  font-size: 12px;
  line-height: 1;
  box-shadow: 0 1px 3px #000b;
}

:global(.phmon-map-death-badge) {
  position: absolute;
  right: -3px;
  bottom: -4px;
  display: grid;
  place-items: center;
  width: 18px;
  height: 18px;
  border: 1px solid #ffecc48c;
  border-radius: 50%;
  background: #090909eb;
  overflow: hidden;
  box-shadow: 0 2px 6px #0008;
}

:global(.phmon-map-death-badge img) {
  width: 16px;
  height: 16px;
  max-width: 16px;
  max-height: 16px;
  object-fit: cover;
}

:global(.phmon-map-marker--event) {
  border-radius: 50%;
  background: #c69bff;
}

:global(.phmon-map-popup .leaflet-popup-content-wrapper) {
  width: 280px;
  padding: 0;
  border: 1px solid #384551;
  border-top: 2px solid #4db9ff;
  border-radius: 9px;
  background: #1b2028f5;
  color: #eaf1ff;
  box-shadow: 0 8px 24px #000a;
}

:global(.phmon-map-popup .leaflet-popup-content) {
  width: 280px !important;
  margin: 0;
}

:global(.phmon-map-popup .leaflet-popup-tip) {
  background: #1b2028f5;
}

:global(.phmon-map-detail) {
  padding: 10px 12px;
  font:
    12px Segoe UI,
    Tahoma,
    Arial,
    sans-serif;
}

:global(.phmon-map-detail--monster) {
  border-top: 1px solid #ff555c;
}

:global(.phmon-map-detail-head) {
  display: flex;
  align-items: center;
  gap: 10px;
  min-height: 46px;
  padding-bottom: 8px;
}

:global(.phmon-map-detail-head > div) {
  min-width: 0;
  display: grid;
  gap: 2px;
}

:global(.phmon-map-detail-head strong) {
  color: #fff;
  font-size: 13px;
  font-weight: 600;
}

:global(.phmon-map-detail-head span) {
  color: #d7dce3;
}

:global(.phmon-map-detail-portrait),
:global(.phmon-map-detail-monster-dot) {
  flex: none;
  width: 42px;
  height: 42px;
  box-sizing: border-box;
  border: 2px solid #42b6ff;
  border-radius: 50%;
  display: grid;
  place-items: center;
  position: relative;
  background: #080c11;
  color: #fff;
  font-size: 17px;
}

:global(.phmon-map-detail-portrait img) {
  position: absolute;
  inset: 0;
  width: 100%;
  height: 100%;
  object-fit: cover;
  border-radius: 50%;
}

:global(.phmon-map-detail-monster-dot) {
  border-color: #ff1f31;
}

:global(.phmon-map-detail-grid) {
  border-top: 1px solid #3b414a75;
}

:global(.phmon-map-detail-row) {
  display: flex;
  justify-content: space-between;
  gap: 10px;
  padding: 7px 1px;
  border-bottom: 1px solid #3b414a75;
}

:global(.phmon-map-detail-row span:first-child) {
  color: #c8cfd8;
}

:global(.phmon-map-detail-row span:last-child) {
  color: #eaf1ff;
  text-align: right;
}

:global(.phmon-map-detail-hp-track) {
  position: relative;
  height: 16px;
  margin: 9px 1px 1px;
  border-radius: 5px;
  background: #532c32;
  overflow: hidden;
}

:global(.phmon-map-detail-hp-track span) {
  display: block;
  height: 100%;
  border-radius: inherit;
  background: #ff8584;
}

:global(.phmon-map-detail-actions) {
  display: flex;
  justify-content: flex-end;
  padding-top: 9px;
}

:global(.phmon-map-detail-actions button) {
  padding: 5px 8px;
  border: 1px solid #a9a9a2;
  border-radius: 4px;
  background: #9d9e9380;
  color: #fff;
  font: inherit;
  font-weight: 600;
  cursor: pointer;
}

:global(.phmon-map-name-collapsed) {
  display: none;
}
:global(.phmon-map-character-pin:hover .phmon-map-name-collapsed),
:global(.phmon-map-marker:focus .phmon-map-name-collapsed) {
  display: block;
}
:global(.phmon-map-character-cluster) {
  position: absolute;
  bottom: calc(100% + 5px);
  left: 50%;
  transform: translateX(-50%);
  white-space: nowrap;
}
.map-cluster-choices {
  position: absolute;
  top: 60px;
  left: 12px;
  z-index: 1100;
  display: grid;
  max-width: calc(100% - 24px);
  gap: 6px;
  padding: 10px;
}
:global(.phmon-map-detail-hp-label) {
  position: absolute;
  inset: 0;
  display: flex;
  align-items: center;
  justify-content: center;
  font-size: 11px;
  color: var(--ph-text);
}
</style>
