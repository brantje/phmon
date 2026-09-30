import type {
  CharacterView,
  ControlsSnapshot,
  MapSnapshot,
} from '~~/shared/types/live'
import type { MapProfile } from '~~/shared/types/map'
import type { RasterPosition } from '~/utils/mapCoordinates'
import { useCommandFanOut } from '~/composables/useCommandFanOut'
import {
  createMapNavigationIntent,
  mapNavigationCommand,
  mapTrainingPositionCommand,
} from '~/utils/mapNavigationAction'
import type {
  FanOutCommandDefinition,
  FanOutOperation,
} from '~/utils/commandFanOut'

export interface MapNavigationScope {
  server: string
  areaID: string
  floorID: string
  region: number
  datasetID: string
  datasetVersion: string
}

export function mapNavigationScopeKey(scope: MapNavigationScope) {
  return [
    scope.server.toLowerCase(),
    scope.areaID,
    scope.floorID,
    scope.region,
    scope.datasetID,
    scope.datasetVersion,
  ].join('\u0000')
}

export function useMapNavigationAction(options: {
  scope(): MapNavigationScope | null
  profile(): MapProfile | null
  selectedTargetIDs(): string[]
  characters(): CharacterView[]
  mapSnapshot(): MapSnapshot | undefined
  mapFeedCurrent(): boolean
  reviewActions(): boolean
  now(): number
}) {
  const menuOpen = ref(false)
  const menuAnchor = ref({ x: 0, y: 0 })
  const menuPoint = ref<RasterPosition | null>(null)
  const preparedTargetIDs = ref<readonly string[] | null>(null)
  const preparedScopeKey = ref('')
  const activeOperationID = ref('')
  const reviewingOperationID = ref('')
  const notice = ref('')
  const openSequence = ref(0)
  const menuElement = ref<HTMLElement | null>(null)
  const resultsElement = ref<HTMLElement | null>(null)
  let returnFocusElement: HTMLElement | null = null

  const getCharacter = (characterID: string) =>
    options
      .characters()
      .find((character) => character.character_id === characterID)
  const currentScopeKey = (characterID: string) => {
    const scope = options.scope()
    const character = getCharacter(characterID)
    if (!scope || !character) return 'unavailable'
    if (character.server.toLowerCase() !== scope.server.toLowerCase())
      return `server:${character.server.toLowerCase()}`
    return mapNavigationScopeKey(scope)
  }
  const command: FanOutCommandDefinition = {
    name: 'character.navigate',
    label: 'Navigate to map point',
    impact: 'movement' as const,
    buildArgs: () => ({}),
  }
  const fanOutOptions = (definition: FanOutCommandDefinition) => ({
    command: definition,
    get scopeKey() {
      const scope = options.scope()
      return scope ? mapNavigationScopeKey(scope) : 'unavailable'
    },
    scopeKeyForCharacter(character: CharacterView, scopeKey?: string) {
      const scope = options.scope()
      return scope &&
        character.server.toLowerCase() === scope.server.toLowerCase()
        ? scopeKey || mapNavigationScopeKey(scope)
        : `server:${character.server.toLowerCase()}`
    },
    currentScopeKey,
    currentCharacter: getCharacter,
  })
  const fanout = useCommandFanOut(fanOutOptions(command))
  const trainingFanout = useCommandFanOut(
    fanOutOptions({
      name: 'training.area.set',
      label: 'Set training position',
      impact: 'routine',
      buildArgs: () => ({}),
    }),
  )
  const activeTrainingOperationID = ref('')

  const operations = computed(() => fanout.operations.value)
  const trainingOperations = computed(() => trainingFanout.operations.value)
  const menuOperation = computed(() =>
    operations.value.find(
      (item) => item.operationID === activeOperationID.value,
    ),
  )
  const trainingMenuOperation = computed(() =>
    trainingOperations.value.find(
      (item) => item.operationID === activeTrainingOperationID.value,
    ),
  )
  const reviewOperation = computed(
    () =>
      operations.value.find(
        (item) => item.operationID === reviewingOperationID.value,
      ) ||
      trainingOperations.value.find(
        (item) => item.operationID === reviewingOperationID.value,
      ),
  )
  const fanoutFor = (operation: FanOutOperation) =>
    trainingOperations.value.includes(operation) ? trainingFanout : fanout
  const targetIDs = computed(
    () =>
      menuOperation.value?.children.map((child) => child.characterID) ||
      preparedTargetIDs.value ||
      options.selectedTargetIDs(),
  )
  const targetLabel = computed(() => {
    const ids = targetIDs.value
    if (ids.length === 1) {
      const name =
        menuOperation.value?.children[0]?.characterName ||
        getCharacter(ids[0]!)?.name
      return name ? `Navigate ${name} here` : 'Navigate here'
    }
    if (ids.length > 1) return `Navigate ${ids.length} characters here`
    return 'Navigate here'
  })
  const counts = computed(() =>
    menuOperation.value
      ? fanout.counts(menuOperation.value)
      : { selected: 0, eligible: 0, skipped: 0 },
  )
  const menuSummary = computed(() => {
    if (notice.value || fanout.error.value)
      return notice.value || fanout.error.value
    if (fanout.preparing.value) return ''
    if (!targetIDs.value.length) return 'Select characters first.'
    if (!counts.value.eligible)
      return targetIDs.value.length === 1
        ? menuOperation.value?.children[0]?.skipReason?.message ||
            'Character is unavailable.'
        : 'Selected characters are unavailable.'
    return counts.value.skipped
      ? `${counts.value.skipped} of ${counts.value.selected} characters unavailable.`
      : ''
  })

  const trainingLabel = computed(() => {
    const operation = trainingMenuOperation.value
    const ids =
      operation?.children.map((child) => child.characterID) ||
      preparedTargetIDs.value ||
      options.selectedTargetIDs()
    if (ids.length === 1) {
      const name =
        operation?.children[0]?.characterName || getCharacter(ids[0]!)?.name
      return name
        ? `Set training position for ${name}`
        : 'Set training position'
    }
    if (ids.length > 1)
      return `Set training position for ${ids.length} characters`
    return 'Set training position'
  })
  const trainingCounts = computed(() =>
    trainingMenuOperation.value
      ? trainingFanout.counts(trainingMenuOperation.value)
      : { selected: 0, eligible: 0, skipped: 0 },
  )
  const trainingSummary = computed(() => {
    if (trainingFanout.error.value) return trainingFanout.error.value
    if (trainingFanout.preparing.value || !targetIDs.value.length) return ''
    if (!trainingCounts.value.eligible)
      return targetIDs.value.length === 1
        ? trainingMenuOperation.value?.children[0]?.skipReason?.message ||
            'Character is unavailable.'
        : 'Selected characters cannot set a training position.'
    return trainingCounts.value.skipped
      ? `${trainingCounts.value.skipped} of ${trainingCounts.value.selected} characters cannot set a training position.`
      : ''
  })

  function trainingResultStatusNote(operation: FanOutOperation) {
    return operation.children.some(
      (child) => child.executionState === 'completed',
    )
      ? 'phBot accepted the new center; the training circle follows the next readback.'
      : undefined
  }

  function resultStatusNote(operation: FanOutOperation) {
    const completed = operation.children.filter(
      (child) =>
        child.executionState === 'completed' && child.apiReturn !== false,
    )
    if (!completed.length) return undefined
    const routes = options.mapSnapshot()?.navigation
    const missing = completed.filter(
      (child) => !routes?.some((route) => route.command_id === child.commandID),
    )
    if (
      !missing.length &&
      completed.every((child) =>
        routes?.some(
          (route) =>
            route.command_id === child.commandID && route.status === 'arrived',
        ),
      )
    )
      return 'Arrival observed from a fresh character position.'
    const legacy = missing.filter((child) => {
      const controls = fanout.feed.value?.targets[child.characterID]
        ?.controls as ControlsSnapshot | undefined
      return Boolean(
        controls?.agent_protocol_version && controls.agent_protocol_version < 8,
      )
    })
    if (legacy.length) {
      return 'This session uses a plugin older than 1.6.0 and cannot report route geometry. Script accepted; arrival has not been observed.'
    }
    if (missing.length && options.mapSnapshot()?.navigation === undefined) {
      return 'The live backend has no route-reporting field yet. Script accepted; arrival has not been observed.'
    }
    if (missing.length) {
      return 'Waiting for the plugin route report. Script accepted; arrival has not been observed.'
    }
    return 'Script accepted; arrival has not been observed.'
  }

  function definitionForIntent(
    intent: ReturnType<typeof createMapNavigationIntent>,
  ) {
    return mapNavigationCommand({
      getIntent: () => intent,
      getProfile: options.profile,
      getCharacter,
      getControls(characterID) {
        const controls = fanout.feed.value?.targets[characterID]?.controls
        return controls ? (controls as ControlsSnapshot) : null
      },
      mapFeedCurrent: options.mapFeedCurrent,
      now: options.now,
    })
  }

  function updateAnchor() {
    if (!menuElement.value || !menuOpen.value) return
    const rect = menuElement.value.getBoundingClientRect()
    const margin = 8
    menuAnchor.value = {
      x: Math.max(
        margin,
        Math.min(menuAnchor.value.x, window.innerWidth - rect.width - margin),
      ),
      y: Math.max(
        margin,
        Math.min(menuAnchor.value.y, window.innerHeight - rect.height - margin),
      ),
    }
  }

  function trainingDefinitionForIntent(
    intent: ReturnType<typeof createMapNavigationIntent>,
  ) {
    return mapTrainingPositionCommand({
      getIntent: () => intent,
      getProfile: options.profile,
      getCharacter,
      getControls(characterID) {
        const controls =
          trainingFanout.feed.value?.targets[characterID]?.controls
        return controls ? (controls as ControlsSnapshot) : null
      },
      mapFeedCurrent: options.mapFeedCurrent,
    })
  }

  async function open(
    point: RasterPosition,
    anchor: { x: number; y: number },
    focusTarget?: HTMLElement | null,
    focusAction: 'navigate' | 'training' = 'navigate',
  ) {
    const scope = options.scope()
    const profile = options.profile()
    if (!scope || !profile) return
    const previous = menuOperation.value
    if (previous && previous.state === 'prepared') fanout.dismiss(previous)
    const previousTraining = trainingMenuOperation.value
    if (previousTraining?.state === 'prepared')
      trainingFanout.dismiss(previousTraining)
    menuPoint.value = Object.freeze({ ...point })
    returnFocusElement =
      focusTarget ?? document.querySelector<HTMLElement>('.map-canvas')
    menuAnchor.value = { x: anchor.x, y: anchor.y }
    activeOperationID.value = ''
    activeTrainingOperationID.value = ''
    reviewingOperationID.value = ''
    notice.value = ''
    menuOpen.value = true
    const sequence = ++openSequence.value
    const intent = createMapNavigationIntent({
      point,
      server: scope.server,
      areaID: scope.areaID,
      floorID: scope.floorID,
      explicitRegion: scope.region,
      datasetID: scope.datasetID,
      datasetVersion: scope.datasetVersion,
      targetIDs: options.selectedTargetIDs(),
    })
    preparedTargetIDs.value = intent.targetIDs
    preparedScopeKey.value = mapNavigationScopeKey(scope)
    const [operation, trainingOperation] = await Promise.all([
      fanout.prepare(
        [...intent.targetIDs],
        definitionForIntent(intent),
        mapNavigationScopeKey(scope),
      ),
      trainingFanout.prepare(
        [...intent.targetIDs],
        trainingDefinitionForIntent(intent),
        mapNavigationScopeKey(scope),
      ),
    ])
    if (sequence !== openSequence.value || !menuOpen.value) {
      if (operation?.state === 'prepared') fanout.dismiss(operation)
      if (trainingOperation?.state === 'prepared')
        trainingFanout.dismiss(trainingOperation)
      return
    }
    if (operation) activeOperationID.value = operation.operationID
    if (trainingOperation)
      activeTrainingOperationID.value = trainingOperation.operationID
    await nextTick()
    updateAnchor()
    const preferred = menuElement.value?.querySelector<HTMLButtonElement>(
      `button[data-map-action="${focusAction}"]:not(:disabled)`,
    )
    const action =
      preferred ||
      menuElement.value?.querySelector<HTMLButtonElement>(
        'button:not(:disabled)',
      )
    ;(action || menuElement.value)?.focus()
  }

  function close(restoreFocus = true) {
    menuOpen.value = false
    reviewingOperationID.value = ''
    const operation = menuOperation.value
    if (operation?.state === 'prepared') fanout.dismiss(operation)
    const trainingOperation = trainingMenuOperation.value
    if (trainingOperation?.state === 'prepared')
      trainingFanout.dismiss(trainingOperation)
    activeOperationID.value = ''
    activeTrainingOperationID.value = ''
    preparedTargetIDs.value = null
    preparedScopeKey.value = ''
    notice.value = ''
    openSequence.value++
    if (restoreFocus) {
      const target = returnFocusElement
      requestAnimationFrame(() => {
        if (target?.isConnected) target.focus()
        else document.querySelector<HTMLElement>('.map-canvas')?.focus()
      })
    }
    returnFocusElement = null
  }

  async function submit() {
    await submitOperation(menuOperation.value)
  }

  async function submitTraining() {
    await submitOperation(trainingMenuOperation.value)
  }

  async function submitOperation(operation: FanOutOperation | undefined) {
    if (!operation) return
    const owner = fanoutFor(operation)
    if (
      owner.preparing.value ||
      owner.submitting.value ||
      !owner.counts(operation).eligible
    )
      return
    if (options.reviewActions()) {
      reviewingOperationID.value = operation.operationID
      notice.value = ''
      menuOpen.value = false
      openSequence.value++
      await nextTick()
      resultsElement.value?.scrollIntoView({ block: 'nearest' })
      resultsElement.value?.focus()
      return
    }
    const changed = await owner.refreshPreview(operation)
    if (changed) {
      notice.value =
        'Eligibility or destination changed. Close and reopen this menu to prepare a new action.'
      return
    }
    void owner.submit(operation)
    close(false)
    await nextTick()
    resultsElement.value?.focus({ preventScroll: true })
  }

  async function submitReviewed() {
    const operation = reviewOperation.value
    if (!operation) return
    const owner = fanoutFor(operation)
    if (owner.preparing.value || owner.submitting.value) return
    const changed = await owner.refreshPreview(operation)
    if (changed) {
      notice.value =
        'Eligibility or destination changed. Close and reopen this menu to prepare a new action.'
      return
    }
    reviewingOperationID.value = ''
    void owner.submit(operation)
    close(false)
    await nextTick()
    resultsElement.value?.focus({ preventScroll: true })
  }

  function cancelReview() {
    const operation = reviewOperation.value
    if (operation?.state === 'prepared') fanoutFor(operation).dismiss(operation)
    close()
  }

  function dismissResults(operation: FanOutOperation) {
    fanoutFor(operation).dismiss(operation)
  }

  function retry(operation: FanOutOperation, characterID: string) {
    return fanoutFor(operation).retrySubmission(operation, characterID)
  }

  function outsidePointer(event: PointerEvent) {
    const target = event.target as Node | null
    if (menuOpen.value && target && !menuElement.value?.contains(target))
      close(false)
  }
  function escape(event: KeyboardEvent) {
    if (event.key === 'Escape' && (menuOpen.value || reviewOperation.value)) {
      event.preventDefault()
      close()
    }
  }

  onMounted(() => {
    window.addEventListener('pointerdown', outsidePointer)
    window.addEventListener('keydown', escape)
    window.addEventListener('resize', updateAnchor)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('pointerdown', outsidePointer)
    window.removeEventListener('keydown', escape)
    window.removeEventListener('resize', updateAnchor)
    fanout.dispose()
    trainingFanout.dispose()
  })

  watch(
    () => options.scope(),
    (scope) => {
      if (
        (menuOpen.value || reviewOperation.value) &&
        (!scope || mapNavigationScopeKey(scope) !== preparedScopeKey.value)
      )
        close(false)
    },
    { deep: true },
  )

  return {
    menuOpen: readonly(menuOpen),
    menuAnchor: readonly(menuAnchor),
    menuPoint: readonly(menuPoint),
    menuElement,
    resultsElement,
    menuOperation,
    reviewOperation,
    operations,
    targetLabel,
    targetIDs,
    counts,
    menuSummary,
    resultStatusNote,
    trainingMenuOperation,
    trainingOperations,
    trainingLabel,
    trainingCounts,
    trainingSummary,
    trainingResultStatusNote,
    trainingPreparing: trainingFanout.preparing,
    trainingSubmitting: trainingFanout.submitting,
    trainingStale: trainingFanout.stale,
    notice: readonly(notice),
    preparing: fanout.preparing,
    submitting: fanout.submitting,
    error: fanout.error,
    stale: fanout.stale,
    open,
    close,
    submit,
    submitTraining,
    submitReviewed,
    cancelReview,
    dismissResults,
    retry,
    updateAnchor,
  }
}
