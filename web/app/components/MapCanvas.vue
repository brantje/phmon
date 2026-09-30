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
  MapPartyMember,
} from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import type { CharacterMarkerInput } from '~/utils/mapCharacterMarkers'
import type { MapHeatLayer } from '~/utils/mapHeatmap'
import type { MapRouteOverlay } from '~/utils/mapNavigationRoutes'
import type { TrainingAreaOverlay } from '~/utils/mapTrainingAreas'
import { PARTY_MEMBER_ICON } from '~/utils/mapPartyPresentation'
import {
  localMapAsset,
  monsterDisplayName,
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
  mapZoomLevelForPercent,
  mapZoomPercentForLevel,
  snapMapZoomPercent,
} from '~/utils/mapZoom'
import { interpolateMarkerPosition } from '~/utils/mapMarkerAnimation'

interface MapCanvasMarker {
  id: string
  label: string
  kind: 'character' | 'party' | 'monster' | 'death' | 'drop' | 'event'
  position: RasterPosition
  placement?: 'exact' | 'region-tile'
  selected?: boolean
  character?: CharacterMarkerInput
  party?: MapPartyMember
  monster?: MapMonster
  showLabel?: boolean
  itemName?: string
  itemIconUrl?: string
  event?: ActivityEvent
}

const props = defineProps<{
  profile: MapProfile
  compact?: boolean
  initialPosition?: RasterPosition | null
  focusRequest?: number
  initialTile?: { x: number; y: number }
  markers?: MapCanvasMarker[]
  heatLayers?: MapHeatLayer[]
  navigationRoutes?: MapRouteOverlay[]
  trainingAreas?: TrainingAreaOverlay[]
  trainingEditable?: boolean
}>()
const emit = defineEmits<{
  viewchange: [view: { tileX: number; tileY: number; zoomPercent: number }]
  trainingselect: [characterID: string]
  trainingmove: [characterID: string, point: RasterPosition]
  trainingresize: [characterID: string, radiusPixels: number]
  pointselect: [point: RasterPosition]
  contextaction: [
    action: { point: RasterPosition; anchor: { x: number; y: number } },
  ]
  mapdrag: []
  opencharacter: [characterID: string]
}>()
const element = ref<HTMLDivElement | null>(null)
let map: LeafletMap | undefined
let heatLayerGroup: LayerGroup | undefined
let markerLayer: LayerGroup | undefined
let navigationLayerGroup: LayerGroup | undefined
let trainingLayerGroup: LayerGroup | undefined
let leaflet: typeof import('leaflet') | undefined
interface RenderedTrainingArea {
  circle: LeafletCircle
  label: LeafletMarker
  centerHandle?: LeafletMarker
  edgeHandle?: LeafletMarker
}
const renderedTraining = new Map<string, RenderedTrainingArea>()
let trainingDragging = ''
let heatRenderer: L.Canvas | undefined
let makeHeatCircle: typeof import('leaflet').circleMarker | undefined
let makePolyline: typeof import('leaflet').polyline | undefined
let makeRouteCircle: typeof import('leaflet').circleMarker | undefined
let makeDivIcon: typeof import('leaflet').divIcon | undefined
let makeRouteMarker: typeof import('leaflet').marker | undefined
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
let stopped = false
let lastFocusedTile = ''
let lastFocusRequest = 0

function indexAt(position: LatLng) {
  return leafletToRasterPosition(
    props.profile.tiles,
    position.lat,
    position.lng,
  )
}

function setInitialView() {
  if (!map || !createLatLng) return
  if (props.initialPosition) {
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
  emit('viewchange', {
    ...indexAt(map.getCenter()),
    zoomPercent: mapZoomPercentForLevel(map.getZoom()),
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
    if (route.destination) {
      const point = route.destination
      const column = point.tileX - props.profile.tiles.min_x
      const row = props.profile.tiles.max_y - point.tileY
      const label = document.createElement('span')
      label.className = 'phmon-map-route-destination'
      label.textContent = `Destination · ${route.characterName}`
      if (makeDivIcon && makeRouteMarker) {
        const icon = makeDivIcon({
          className: 'phmon-map-route-destination-marker',
          html: label,
          iconSize: [0, 0],
          iconAnchor: [0, 12],
        })
        const marker = makeRouteMarker(
          toLatLng(-(row * 256 + point.pixelY), column * 256 + point.pixelX),
          {
            icon,
            interactive: true,
            keyboard: false,
          },
        )
        marker.setOpacity(opacity)
        marker
          .bindTooltip(`Destination · ${route.characterName}`)
          .addTo(navigationLayerGroup)
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

const trainingLabelPoint = (center: LatLng, radius: number) =>
  createLatLng!(center.lat + radius, center.lng)
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
      rendered.label.setLatLng(trainingLabelPoint(next, size))
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
      rendered.label.setLatLng(trainingLabelPoint(origin, size))
    })
    handle.on('dragend', () => {
      emit('trainingresize', id, rendered.circle.getRadius())
      finishTrainingDrag()
    })
    handle.addTo(trainingLayerGroup!)
    rendered.edgeHandle = handle
  }
}

function updateTrainingLabel(
  rendered: RenderedTrainingArea,
  area: TrainingAreaOverlay,
) {
  const element = rendered.label.getElement()
  const label = element?.querySelector('.phmon-map-training-label')
  if (!element || !label) return
  label.textContent = area.label
  label.classList.toggle('phmon-map-training-label--selected', area.selected)
  label.classList.toggle('phmon-map-training-label--draft', area.draft)
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
  for (const area of (props.trainingAreas || []).slice(0, 256)) {
    if (
      !Number.isFinite(area.center.pixelX) ||
      !Number.isFinite(area.center.pixelY) ||
      !Number.isFinite(area.radiusPixels)
    )
      continue
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
        event.preventDefault()
        emit('trainingselect', area.id)
      })
      rendered = { circle, label }
      renderedTraining.set(area.id, rendered)
    } else if (trainingDragging !== area.id) {
      rendered.circle.setLatLng(center)
      rendered.circle.setRadius(area.radiusPixels)
      rendered.label.setLatLng(trainingLabelPoint(center, area.radiusPixels))
    }
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
  } else if (marker.kind === 'monster' && marker.monster) {
    const monster = marker.monster
    title.textContent = monsterDisplayName(monster)
    subtitle.textContent = `${monsterTypePresentation(monster).label} · Position | ${positionText(monster.x, monster.y, monster.z)}`
    const dot = document.createElement('span')
    dot.className = 'phmon-map-detail-monster-dot'
    header.prepend(dot)
    details.append(
      detailRow(
        'Level',
        monster.level == null ? 'Unavailable' : String(monster.level),
      ),
      detailRow('HP', `${integer(monster.hp)} / ${integer(monster.max_hp)}`),
    )
    panel.append(header, details)
    const fraction = monsterHPFraction(monster)
    if (fraction != null) {
      const track = document.createElement('div')
      track.className = 'phmon-map-detail-hp-track'
      const fill = document.createElement('span')
      fill.style.width = `${(fraction * 100).toFixed(1)}%`
      track.append(fill)
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
    header.prepend(icon)
    if (marker.event) {
      details.append(
        detailRow('Character', marker.event.character || '—'),
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
    content.append(img)
    const name = document.createElement('span')
    name.className = 'phmon-map-party-name'
    name.textContent = marker.party?.name?.trim() || marker.label
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
      if (presentation.iconUrl) {
        const rankIcon = document.createElement('img')
        rankIcon.className = 'phmon-map-monster-rank-icon'
        rankIcon.src = presentation.iconUrl
        rankIcon.alt = ''
        rankIcon.onerror = () => rankIcon.remove()
        content.append(rankIcon)
      }
      if (presentation.partyBadgeUrl) {
        const partyBadge = document.createElement('img')
        partyBadge.className = 'phmon-map-monster-party-badge'
        partyBadge.src = presentation.partyBadgeUrl
        partyBadge.alt = ''
        partyBadge.onerror = () => partyBadge.remove()
        content.append(partyBadge)
      }
      const mapName = monsterMapName(marker.monster, Boolean(marker.showLabel))
      if (mapName) {
        const name = document.createElement('span')
        name.className = 'phmon-map-monster-name'
        name.textContent = mapName
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
  makeDivIcon = L.divIcon
  makeRouteMarker = L.marker
  heatRenderer = L.canvas({ padding: 0.5 })
  const rows = props.profile.tiles.max_y - props.profile.tiles.min_y + 1
  const columns = props.profile.tiles.max_x - props.profile.tiles.min_x + 1
  const bounds = L.latLngBounds(
    L.latLng(-rows * 256, 0),
    L.latLng(0, columns * 256),
  )
  map = L.map(element.value, {
    crs: L.CRS.Simple,
    ...MAP_ZOOM_OPTIONS,
    zoomSnap: 0,
    zoomDelta: 0.25,
    maxBounds: bounds,
    maxBoundsViscosity: 0.8,
    keyboard: true,
    zoomControl: !props.compact,
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
  heatLayerGroup = L.layerGroup().addTo(map)
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
        : marker.kind === 'party'
          ? 24
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
      type?.code,
      type?.scale,
      type?.party,
      type?.iconUrl,
      type?.partyBadgeUrl,
      type?.unknown,
      marker.monster
        ? monsterMapName(marker.monster, Boolean(marker.showLabel))
        : '',
      hp,
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
      if (!props.compact) existing.setPopupContent(markerPopup(marker))
      return existing
    }
    const rendered = L.marker(point, {
      icon: icon!,
      title: marker.label,
      keyboard: true,
      riseOnHover: true,
      zIndexOffset:
        marker.kind === 'character'
          ? 1000
          : marker.kind === 'party'
            ? 700
            : marker.kind === 'drop' || marker.kind === 'death'
              ? 400
              : 0,
    })
    if (!props.compact)
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
  const selectPoint = (position: LatLng) =>
    emit('pointselect', indexAt(position))
  const onKeydown = (event: KeyboardEvent) => {
    if (event.key !== 'Enter' && event.key !== ' ') return
    if (event.target !== element.value) return
    event.preventDefault()
    if (map) selectPoint(map.getCenter())
  }
  element.value.addEventListener('keydown', onKeydown)
  map.once('unload', () =>
    element.value?.removeEventListener('keydown', onKeydown),
  )
  map.on('click', (event: L.LeafletMouseEvent) => selectPoint(event.latlng))
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
  map.on('zoomend', snapZoomToPercentStep)
  map.on('moveend zoomend', publishView)
  publishView()
  syncHeatLayers()
  syncTrainingAreas()
  syncNavigationRoutes()
  syncMarkers()
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
      (position && `${position.tileX}:${position.tileY}` !== lastFocusedTile)
    )
      setInitialView()
  },
  { deep: true },
)
watch(() => props.markers, syncMarkers, { deep: true })
watch(() => props.heatLayers, syncHeatLayers, { deep: true })
watch(() => props.navigationRoutes, syncNavigationRoutes, { deep: true })
watch(
  [() => props.trainingAreas, () => props.trainingEditable],
  syncTrainingAreas,
  { deep: true },
)

function focusCanvas() {
  element.value?.focus()
}

defineExpose({ focus: focusCanvas })

onBeforeUnmount(() => {
  stopped = true
  for (const frame of markerAnimationFrames.values())
    cancelAnimationFrame(frame)
  markerAnimationFrames.clear()
  map?.remove()
  map = undefined
  heatLayerGroup = undefined
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
  makeDivIcon = undefined
  makeRouteMarker = undefined
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
        : 'Interactive raster map. Use arrow keys to pan, Enter or Space to select the center tile, or right-click to open map actions.'
    "
    role="application"
    tabindex="0"
  />
</template>

<style scoped>
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

:global(.phmon-map-route-destination-marker) {
  border: 0;
  background: transparent;
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
  display: block;
  padding: 1px 6px;
  border: 1px solid #4db9ffb3;
  border-radius: 3px;
  background: #0b1a29e6;
  color: #d8eeff;
  font-size: 11px;
  font-weight: 600;
  white-space: nowrap;
  cursor: pointer;
  box-shadow: 0 1px 4px #0009;
}

:global(.phmon-map-training-label--selected) {
  border-color: #9fdcff;
  background: #123a5ce6;
  color: #ffffff;
}

:global(.phmon-map-training-label--draft::after) {
  content: ' · unsaved';
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

:global(.phmon-map-route-destination) {
  display: inline-flex;
  align-items: center;
  gap: 5px;
  min-height: 22px;
  padding: 2px 7px;
  border: 1px solid #b8ffff;
  border-radius: 3px;
  background: #09232bf2;
  color: #eaffff;
  font-size: 11px;
  font-weight: 700;
  white-space: nowrap;
  box-shadow: 0 1px 5px #000a;
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
  width: 100%;
  height: 100%;
  filter: drop-shadow(0 1px 3px #000c);
}

:global(.phmon-map-party-name) {
  position: absolute;
  top: calc(100% - 2px);
  left: 50%;
  transform: translateX(-50%);
  max-width: 112px;
  overflow: hidden;
  padding: 2px 5px;
  border-radius: 7px;
  background: #0d1119ec;
  color: #eaf1ff;
  font-size: 10px;
  font-weight: 500;
  line-height: 1.1;
  text-overflow: ellipsis;
  white-space: nowrap;
  box-shadow: 0 1px 4px #000b;
  pointer-events: none;
}

:global(.phmon-map-party-icon img),
:global(.phmon-map-detail-party-icon img) {
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

:global(.phmon-map-monster-rank-icon) {
  position: absolute;
  inset: 1px;
  z-index: 1;
  width: calc(100% - 2px);
  height: calc(100% - 2px);
  object-fit: contain;
  pointer-events: none;
  filter: drop-shadow(0 1px 1px #000c);
}

:global(.phmon-map-monster-party-badge) {
  position: absolute;
  right: -4px;
  bottom: -4px;
  z-index: 2;
  width: 8px;
  height: 8px;
  object-fit: contain;
  pointer-events: none;
  filter: drop-shadow(0 1px 2px #000c);
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
  position: absolute;
  top: calc(100% + 5px);
  left: 50%;
  z-index: 3;
  transform: translateX(-50%);
  max-width: 120px;
  overflow: hidden;
  padding: 2px 5px;
  border-radius: 7px;
  background: #0d1119ec;
  color: #fff;
  font-size: 10px;
  font-weight: 500;
  line-height: 1.1;
  text-overflow: ellipsis;
  white-space: nowrap;
  box-shadow: 0 1px 4px #000b;
  pointer-events: none;
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
  height: 8px;
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
</style>
