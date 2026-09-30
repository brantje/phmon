import type { CharacterView, MapTrainingArea } from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import type {
  FanOutCommandDefinition,
  FanOutChild,
} from '~/utils/commandFanOut'
import {
  rasterPixelsToWorldRadius,
  rasterPositionToGame,
  type RasterPosition,
} from '~/utils/mapCoordinates'
import {
  clampTrainingRadius,
  reconcileTrainingDraft,
  roundTrainingCenter,
  runTrainingApplySteps,
  trainingApplySteps,
  trainingCenterDirty,
  trainingDraftDirty,
  trainingRadiusDirty,
  type TrainingApplyStep,
  type TrainingAreaDraft,
  type TrainingStepOutcome,
  type TrainingStepResult,
} from '~/utils/mapTrainingAreas'

export interface MapTrainingEditorScope {
  server: string
  areaID: string
  floorID: string
}

export interface UseMapTrainingEditorOptions {
  scope(): MapTrainingEditorScope | null
  profile(): MapProfile | null
  areas(): MapTrainingArea[]
  characters(): CharacterView[]
  reviewActions(): boolean
  resultTimeoutMs?: number
}

const TERMINAL_STATES = ['completed', 'failed', 'expired', 'unknown']

const stepLabel = (step: TrainingApplyStep) =>
  step.name === 'training.area.set'
    ? 'Move training center'
    : 'Set training radius'

function stepSummary(step: TrainingApplyStep) {
  if (step.name === 'training.radius.set') return `Radius ${step.args.radius}`
  const { x, y } = step.args as { x: number; y: number }
  return `Center ${x.toFixed(1)}, ${y.toFixed(1)}`
}

export function useMapTrainingEditor(options: UseMapTrainingEditorOptions) {
  const live = useLiveData()
  const selectedID = ref('')
  const draft = ref<TrainingAreaDraft | null>(null)
  const moveArmed = ref(false)
  const applying = ref(false)
  const results = ref<TrainingStepResult[]>([])
  const message = ref('')

  const scopeKey = () => {
    const scope = options.scope()
    return scope
      ? `training:${scope.server.toLocaleLowerCase()}:${scope.areaID}:${scope.floorID}`
      : 'training:none'
  }
  const currentCharacter = (characterID: string) =>
    options
      .characters()
      .find((character) => character.character_id === characterID)
  const scopeForCharacter = (character: CharacterView | undefined) => {
    const scope = options.scope()
    return character &&
      scope &&
      character.server.toLocaleLowerCase() === scope.server.toLocaleLowerCase()
      ? scopeKey()
      : 'training:other'
  }

  const fanOut = useCommandFanOut({
    command: {
      name: 'training.radius.set',
      label: 'Set training radius',
      impact: 'routine',
      buildArgs: () => ({}),
    },
    scopeKey: scopeKey(),
    scopeKeyForCharacter: (character) => scopeForCharacter(character),
    currentScopeKey: (characterID) =>
      scopeForCharacter(currentCharacter(characterID)),
    currentCharacter,
  })

  const selectedArea = computed(() =>
    options.areas().find((area) => area.character_id === selectedID.value),
  )
  const dirty = computed(() =>
    trainingDraftDirty(draft.value, selectedArea.value),
  )
  const centerDirty = computed(() =>
    trainingCenterDirty(draft.value, selectedArea.value),
  )
  const radiusDirty = computed(() =>
    trainingRadiusDirty(draft.value, selectedArea.value),
  )
  const displayedRadius = computed(
    () => draft.value?.radius ?? selectedArea.value?.radius ?? 0,
  )
  const selectedCapabilities = computed(() => {
    const target = fanOut.feed.value?.targets[selectedID.value]
    const controls = target?.controls
    const area = selectedArea.value
    if (!controls || !area || controls.session_id !== area.session_id)
      return null
    return controls.capabilities
  })
  const editReason = computed(() => {
    const area = selectedArea.value
    if (!area) return 'Select a training area on the map.'
    const character = currentCharacter(area.character_id)
    if (!character?.online || character.session_id !== area.session_id)
      return 'This training area belongs to an offline or replaced session.'
    const capabilities = selectedCapabilities.value
    if (!capabilities) return 'Waiting for current command capabilities.'
    const position = capabilities['training.area.set']
    const radius = capabilities['training.radius.set']
    if (
      !position?.supported ||
      !position.modes?.includes('position') ||
      !radius?.supported
    )
      return 'This session does not report support for editing the training area.'
    return ''
  })
  const applyReason = computed(() => {
    if (applying.value) return 'Applying training area changes…'
    if (editReason.value) return editReason.value
    if (!dirty.value) return 'No unsaved training area changes.'
    return ''
  })

  function select(characterID: string) {
    if (applying.value) return
    if (characterID === selectedID.value) return
    if (dirty.value) draft.value = null
    selectedID.value = characterID
    moveArmed.value = false
    results.value = []
    message.value = ''
    live.setCommandFanOutTargets(
      fanOut.ownerID,
      characterID ? [characterID] : [],
    )
  }

  function clear() {
    selectedID.value = ''
    draft.value = null
    moveArmed.value = false
    message.value = ''
    results.value = []
    live.setCommandFanOutTargets(fanOut.ownerID, [])
  }

  function updateDraft(
    next: Partial<Pick<TrainingAreaDraft, 'center' | 'radius'>>,
  ) {
    const area = selectedArea.value
    if (!area || applying.value) return
    const current =
      draft.value?.characterID === area.character_id &&
      draft.value.sessionID === area.session_id
        ? draft.value
        : null
    draft.value = reconcileTrainingDraft(
      Object.freeze({
        characterID: area.character_id,
        sessionID: area.session_id,
        center: next.center ?? current?.center,
        radius: next.radius ?? current?.radius,
      }),
      area,
    )
    results.value = []
    message.value = ''
  }

  function moveCenter(characterID: string, point: RasterPosition) {
    const area = selectedArea.value
    const scope = options.scope()
    const profile = options.profile()
    if (!area || area.character_id !== characterID || !scope || !profile) return
    const position = rasterPositionToGame(
      profile,
      scope.areaID,
      scope.floorID,
      area.region,
      point,
      area.z ?? 0,
    )
    moveArmed.value = false
    if (!position) {
      message.value =
        'That point has no verified region and X/Y conversion; the center was not moved.'
      return
    }
    updateDraft({ center: roundTrainingCenter(position) })
  }

  function resizeFromPixels(characterID: string, pixels: number) {
    const area = selectedArea.value
    const scope = options.scope()
    const profile = options.profile()
    if (!area || area.character_id !== characterID || !scope || !profile) return
    const region = draft.value?.center?.region ?? area.region
    const radius = rasterPixelsToWorldRadius(
      profile,
      scope.areaID,
      scope.floorID,
      region,
      pixels,
    )
    if (radius == null) return
    updateDraft({ radius: clampTrainingRadius(radius) })
  }

  function setRadius(value: number) {
    if (!Number.isFinite(value)) return
    updateDraft({ radius: clampTrainingRadius(value) })
  }

  function reset() {
    if (applying.value) return
    draft.value = null
    moveArmed.value = false
    results.value = []
    message.value = ''
  }

  function waitForTerminal(child: FanOutChild) {
    if (TERMINAL_STATES.includes(child.executionState || ''))
      return Promise.resolve(child.executionState)
    return new Promise<string | undefined>((resolve) => {
      const timer = setTimeout(() => {
        stop()
        resolve(undefined)
      }, options.resultTimeoutMs ?? 90_000)
      const stop = watch(
        () => fanOut.operations.value,
        () => {
          if (!TERMINAL_STATES.includes(child.executionState || '')) return
          clearTimeout(timer)
          stop()
          resolve(child.executionState)
        },
      )
    })
  }

  async function runStep(
    area: MapTrainingArea,
    step: TrainingApplyStep,
  ): Promise<{ outcome: TrainingStepOutcome; message?: string }> {
    const command: FanOutCommandDefinition = {
      name: step.name,
      label: stepLabel(step),
      impact: 'routine',
      buildArgs: () => ({ ...step.args }),
      summarizeArgs: () => stepSummary(step),
      preEligibility: (character) =>
        character.session_id === area.session_id
          ? null
          : {
              code: 'session_changed',
              message: 'Character session changed after editing.',
            },
    }
    const operation = await fanOut.prepare(
      [area.character_id],
      command,
      scopeKey(),
    )
    const child = operation?.children[0]
    if (!operation || !child)
      return {
        outcome: 'not_sent',
        message: fanOut.error.value || 'Could not prepare the command.',
      }
    if (child.submission === 'skipped')
      return { outcome: 'skipped', message: child.skipReason?.message }
    await fanOut.submit(operation)
    // submit() mutates the child in place.
    const submission = child.submission as FanOutChild['submission']
    if (submission === 'skipped')
      return { outcome: 'skipped', message: child.skipReason?.message }
    if (submission === 'rejected')
      return { outcome: 'rejected', message: child.message }
    if (submission !== 'accepted')
      return { outcome: 'uncertain', message: child.message }
    const state = await waitForTerminal(child)
    if (!state)
      return {
        outcome: 'uncertain',
        message: 'No phBot result arrived in time; the outcome is unknown.',
      }
    return {
      outcome: state as TrainingStepOutcome,
      message: child.message,
    }
  }

  async function apply() {
    const area = selectedArea.value
    if (!area || applyReason.value) return
    const steps = trainingApplySteps(draft.value, area)
    if (!steps.length) return
    if (options.reviewActions()) {
      const summary = steps.map(stepSummary).join(' and ')
      if (
        !window.confirm(
          `Review training area changes for ${area.name}: ${summary}?`,
        )
      )
        return
    }
    applying.value = true
    moveArmed.value = false
    message.value = ''
    results.value = []
    try {
      results.value = await runTrainingApplySteps(steps, async (step) => {
        const result = await runStep(area, step)
        results.value = [...results.value, { step, ...result }]
        return result
      })
      const completed = results.value.every(
        (result) => result.outcome === 'completed',
      )
      message.value = completed
        ? 'phBot accepted the changes; the map follows the next readback.'
        : 'Some changes were not applied. Unsaved values stay in the draft.'
    } finally {
      applying.value = false
      draft.value = reconcileTrainingDraft(draft.value, selectedArea.value)
    }
  }

  watch(
    () => options.areas(),
    () => {
      if (applying.value) return
      if (selectedID.value && !selectedArea.value) {
        clear()
        return
      }
      draft.value = reconcileTrainingDraft(draft.value, selectedArea.value)
    },
    { deep: true },
  )

  watch(scopeKey, () => {
    if (!applying.value) clear()
  })

  onBeforeUnmount(() => fanOut.dispose())

  return {
    selectedID: readonly(selectedID),
    selectedArea,
    draft: readonly(draft),
    moveArmed,
    applying: readonly(applying),
    results: readonly(results),
    message: readonly(message),
    dirty,
    centerDirty,
    radiusDirty,
    displayedRadius,
    editable: computed(() => !editReason.value && !applying.value),
    editReason,
    applyReason,
    select,
    clear,
    moveCenter,
    resizeFromPixels,
    setRadius,
    reset,
    apply,
  }
}
