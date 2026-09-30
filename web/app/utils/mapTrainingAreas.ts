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
