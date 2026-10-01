import type { MapTrainingArea } from '../../shared/types/live.ts'
import type { MapProfile } from '../../shared/types/map.ts'
import {
  worldPositionToRaster,
  worldRadiusToRasterPixels,
  type GamePosition,
  type RasterPosition,
} from './mapCoordinates.ts'

export const TRAINING_RADIUS_MIN = 1
export const TRAINING_RADIUS_MAX = 10_000
// phBot may round training coordinates on readback; half a game unit is far
// below one rendered pixel at every supported zoom.
const READBACK_TOLERANCE = 0.5

export interface TrainingAreaDraft {
  readonly characterID: string
  readonly sessionID: string
  readonly center?: Readonly<GamePosition>
  readonly radius?: number
}

export interface TrainingAreaOverlay {
  id: string
  label: string
  center: RasterPosition
  radiusPixels: number
  selected: boolean
  draft: boolean
}

export interface TrainingApplyStep {
  name: 'training.area.set' | 'training.radius.set'
  args: Record<string, unknown>
}

export type TrainingStepOutcome =
  | 'completed'
  | 'failed'
  | 'expired'
  | 'unknown'
  | 'skipped'
  | 'rejected'
  | 'uncertain'
  | 'not_sent'

export interface TrainingStepResult {
  step: TrainingApplyStep
  outcome: TrainingStepOutcome
  message?: string
}

export function trainingRegionsMatch(left: number, right: number) {
  return (
    left === right ||
    (left === -32767 && right === 32767) ||
    (left === 32767 && right === -32767)
  )
}

export function clampTrainingRadius(value: number) {
  if (!Number.isFinite(value)) return TRAINING_RADIUS_MIN
  return Math.min(
    TRAINING_RADIUS_MAX,
    Math.max(TRAINING_RADIUS_MIN, Math.round(value)),
  )
}

export function roundTrainingCenter(position: GamePosition): GamePosition {
  const round = (value: number) => Math.round(value * 10) / 10
  return {
    region: position.region,
    x: round(position.x),
    y: round(position.y),
    z: round(position.z),
  }
}

function draftFor(area: MapTrainingArea, draft: TrainingAreaDraft | null) {
  return draft &&
    draft.characterID === area.character_id &&
    draft.sessionID === area.session_id
    ? draft
    : null
}

export function trainingCenterDirty(
  draft: TrainingAreaDraft | null,
  area: MapTrainingArea | undefined,
) {
  const center = area && draftFor(area, draft)?.center
  if (!area || !center) return false
  return (
    !trainingRegionsMatch(center.region, area.region) ||
    Math.abs(center.x - area.x) > READBACK_TOLERANCE ||
    Math.abs(center.y - area.y) > READBACK_TOLERANCE
  )
}

export function trainingRadiusDirty(
  draft: TrainingAreaDraft | null,
  area: MapTrainingArea | undefined,
) {
  const radius = area && draftFor(area, draft)?.radius
  if (!area || radius == null) return false
  return Math.abs(radius - area.radius) > READBACK_TOLERANCE
}

export function trainingDraftDirty(
  draft: TrainingAreaDraft | null,
  area: MapTrainingArea | undefined,
) {
  return trainingCenterDirty(draft, area) || trainingRadiusDirty(draft, area)
}

/**
 * Keeps only draft fields that still differ from the latest observation.
 * A missing area or a replaced session discards the draft entirely so it can
 * never be applied to another session.
 */
export function reconcileTrainingDraft(
  draft: TrainingAreaDraft | null,
  area: MapTrainingArea | undefined,
): TrainingAreaDraft | null {
  if (!draft) return null
  if (!area || !draftFor(area, draft)) return null
  const centerDirty = trainingCenterDirty(draft, area)
  const radiusDirty = trainingRadiusDirty(draft, area)
  if (
    centerDirty === Boolean(draft.center) &&
    radiusDirty === (draft.radius != null)
  )
    return draft
  return Object.freeze({
    characterID: draft.characterID,
    sessionID: draft.sessionID,
    ...(centerDirty ? { center: draft.center } : {}),
    ...(radiusDirty ? { radius: draft.radius } : {}),
  })
}

export function trainingApplySteps(
  draft: TrainingAreaDraft | null,
  area: MapTrainingArea | undefined,
): TrainingApplyStep[] {
  const steps: TrainingApplyStep[] = []
  if (!area || !draft) return steps
  if (trainingCenterDirty(draft, area) && draft.center)
    steps.push({
      name: 'training.area.set',
      args: { mode: 'position', ...draft.center },
    })
  if (trainingRadiusDirty(draft, area) && draft.radius != null)
    steps.push({
      name: 'training.radius.set',
      args: { radius: draft.radius },
    })
  return steps
}

/**
 * Runs each command only after the previous one reached a known outcome. The
 * backend admits one in-flight command per character, so an unknown submission
 * blocks the remaining steps instead of racing it.
 */
export async function runTrainingApplySteps(
  steps: TrainingApplyStep[],
  run: (
    step: TrainingApplyStep,
  ) => Promise<{ outcome: TrainingStepOutcome; message?: string }>,
): Promise<TrainingStepResult[]> {
  const results: TrainingStepResult[] = []
  let blocked = false
  for (const step of steps) {
    if (blocked) {
      results.push({
        step,
        outcome: 'not_sent',
        message:
          'Not sent because the previous command outcome is still unknown.',
      })
      continue
    }
    const result = await run(step)
    results.push({ step, ...result })
    if (result.outcome === 'uncertain') blocked = true
  }
  return results
}

const TILE_PIXELS = 256

/** Continuous raster pixels; Y grows downward like the rendered map. */
function planarPoint(position: RasterPosition) {
  return {
    x: position.tileX * TILE_PIXELS + position.pixelX,
    y: -position.tileY * TILE_PIXELS + position.pixelY,
  }
}

const stableOrder = (
  left: Pick<TrainingAreaOverlay, 'id' | 'label'>,
  right: Pick<TrainingAreaOverlay, 'id' | 'label'>,
) => left.label.localeCompare(right.label) || left.id.localeCompare(right.id)

/**
 * Picks the training area under a map point. Nested and partly overlapping
 * circles resolve to the smallest radius, then the closest center, so every
 * visible area keeps a clickable region.
 */
export function trainingAreaAtPoint(
  areas: readonly TrainingAreaOverlay[],
  point: RasterPosition,
): string | null {
  const target = planarPoint(point)
  let best: { area: TrainingAreaOverlay; distance: number } | null = null
  for (const area of areas) {
    if (!Number.isFinite(area.radiusPixels) || area.radiusPixels <= 0) continue
    const center = planarPoint(area.center)
    const distance = Math.hypot(target.x - center.x, target.y - center.y)
    if (!(distance <= area.radiusPixels)) continue
    if (
      !best ||
      area.radiusPixels < best.area.radiusPixels ||
      (area.radiusPixels === best.area.radiusPixels &&
        (distance < best.distance ||
          (distance === best.distance && stableOrder(area, best.area) < 0)))
    )
      best = { area, distance }
  }
  return best?.area.id ?? null
}

export interface TrainingLabelSize {
  width: number
  height: number
}

const LABEL_GAP = 3
const LABEL_SLOTS = 16
const DEFAULT_LABEL_HEIGHT = 18

export function estimateTrainingLabelSize(
  area: Pick<TrainingAreaOverlay, 'label' | 'draft'>,
): TrainingLabelSize {
  const text = area.label.length + (area.draft ? 10 : 0)
  // Discard and accept controls sit between the name and the unsaved suffix.
  const actions = area.draft ? 36 : 0
  return {
    width: Math.round(text * 6.6 + 14 + actions),
    height: DEFAULT_LABEL_HEIGHT,
  }
}

function clusterAngles(count: number) {
  if (count <= 1) return [0]
  if (count === 2) return [-30, 30]
  if (count === 3) return [0, 120, 240]
  return Array.from({ length: count }, (_, index) => (index * 360) / count)
}

/**
 * Screen-space box of a label whose anchor sits on the circle at `angle`
 * (degrees clockwise from north) and which extends outward from the circle.
 */
function labelBox(
  center: { x: number; y: number },
  radius: number,
  angle: number,
  size: TrainingLabelSize,
) {
  const radians = (angle * Math.PI) / 180
  const sin = Math.sin(radians)
  const cos = Math.cos(radians)
  const anchorX = center.x + radius * sin
  const anchorY = center.y - radius * cos
  const middleX = anchorX + (size.width / 2 + LABEL_GAP) * sin
  const middleY = anchorY - (size.height / 2 + LABEL_GAP) * cos
  return {
    left: middleX - size.width / 2,
    right: middleX + size.width / 2,
    top: middleY - size.height / 2,
    bottom: middleY + size.height / 2,
  }
}

type LabelBox = ReturnType<typeof labelBox>
const boxesOverlap = (left: LabelBox, right: LabelBox) =>
  left.left < right.right &&
  right.left < left.right &&
  left.top < right.bottom &&
  right.top < left.bottom

const normalizeAngle = (angle: number) => ((angle % 360) + 360) % 360

/**
 * Assigns each training label an angle on its own circle. Areas sharing
 * roughly the same center spread around that circumference; a greedy pass
 * then rotates any label still colliding with an earlier one.
 * `scale` converts raster pixels to screen pixels at the current zoom.
 */
export function placeTrainingAreaLabels(
  areas: readonly TrainingAreaOverlay[],
  scale: number,
  sizeOf: (
    area: TrainingAreaOverlay,
  ) => TrainingLabelSize = estimateTrainingLabelSize,
): Map<string, number> {
  const ordered = [...areas]
    .filter(
      (area) => Number.isFinite(area.radiusPixels) && area.radiusPixels > 0,
    )
    .sort(stableOrder)
  const safeScale = Number.isFinite(scale) && scale > 0 ? scale : 1
  const geometry = ordered.map((area) => {
    const center = planarPoint(area.center)
    return {
      area,
      center: { x: center.x * safeScale, y: center.y * safeScale },
      radius: area.radiusPixels * safeScale,
      size: sizeOf(area),
    }
  })

  const parent = geometry.map((_, index) => index)
  const root = (index: number): number => {
    while (parent[index] !== index) {
      parent[index] = parent[parent[index]!]!
      index = parent[index]!
    }
    return index
  }
  for (let left = 0; left < geometry.length; left++) {
    for (let right = left + 1; right < geometry.length; right++) {
      const a = geometry[left]!
      const b = geometry[right]!
      const threshold = Math.max(24, 0.35 * Math.min(a.radius, b.radius))
      if (
        Math.hypot(a.center.x - b.center.x, a.center.y - b.center.y) <=
        threshold
      )
        parent[root(right)] = root(left)
    }
  }
  const clusters = new Map<number, number[]>()
  geometry.forEach((_, index) => {
    const key = root(index)
    clusters.set(key, [...(clusters.get(key) || []), index])
  })
  const preferred = new Array<number>(geometry.length).fill(0)
  for (const members of clusters.values()) {
    const angles = clusterAngles(members.length)
    members.forEach((index, position) => {
      preferred[index] = angles[position]!
    })
  }

  const placed: LabelBox[] = []
  const result = new Map<string, number>()
  geometry.forEach((item, index) => {
    const start = preferred[index]!
    let chosen = start
    let box = labelBox(item.center, item.radius, start, item.size)
    if (placed.some((other) => boxesOverlap(box, other))) {
      for (let step = 1; step < LABEL_SLOTS; step++) {
        const offset = Math.ceil(step / 2) * (360 / LABEL_SLOTS)
        const angle = start + (step % 2 ? offset : -offset)
        const candidate = labelBox(item.center, item.radius, angle, item.size)
        if (!placed.some((other) => boxesOverlap(candidate, other))) {
          chosen = angle
          box = candidate
          break
        }
      }
    }
    placed.push(box)
    result.set(item.area.id, normalizeAngle(chosen))
  })
  return result
}

export function trainingAreaOverlays(input: {
  profile: MapProfile
  areaID: string
  floorID: string
  areas: MapTrainingArea[]
  regionFilter: number
  selectedID: string
  draft: TrainingAreaDraft | null
}): TrainingAreaOverlay[] {
  const overlays: TrainingAreaOverlay[] = []
  for (const area of input.areas) {
    if (
      input.regionFilter !== 0 &&
      !trainingRegionsMatch(input.regionFilter, area.region)
    )
      continue
    const draft = draftFor(area, input.draft)
    const center = draft?.center ?? area
    const radius = draft?.radius ?? area.radius
    const position = worldPositionToRaster(
      input.profile,
      input.areaID,
      input.floorID,
      center.region,
      center.x,
      center.y,
      center.z ?? area.z,
    )
    const radiusPixels = worldRadiusToRasterPixels(
      input.profile,
      input.areaID,
      input.floorID,
      center.region,
      radius,
    )
    if (!position || radiusPixels == null) continue
    overlays.push({
      id: area.character_id,
      label: area.name,
      center: position,
      radiusPixels,
      selected: area.character_id === input.selectedID,
      draft: trainingDraftDirty(input.draft, area),
    })
  }
  return overlays
}
