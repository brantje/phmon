import type {
  CharacterView,
  ControlsSnapshot,
  MapNpc,
} from '../../shared/types/live'
import { useCommandFanOut } from '~/composables/useCommandFanOut'
import {
  createMapTeleportIntent,
  mapTeleportCommand,
  mapTeleportGateTitle,
} from '~/utils/mapTeleportAction'
import {
  pickDefaultTeleportDestination,
  sortedTeleportRoutes,
} from '~/utils/mapTeleportRoutes'

export function useMapTeleportAction(options: {
  server(): string
  selectedTargetIDs(): string[]
  characters(): CharacterView[]
  mapFeedCurrent(): boolean
  reviewActions(): boolean
}) {
  const menuOpen = ref(false)
  const menuPresentation = ref<'dialog' | 'context'>('dialog')
  const menuAnchor = ref({ x: 0, y: 0 })
  const menuNpc = ref<MapNpc | null>(null)
  const destination = ref('')
  const menuElement = ref<HTMLElement | null>(null)
  const activeOperationID = ref('')
  const reviewingOperationID = ref('')
  const openSequence = ref(0)
  let returnFocusElement: HTMLElement | null = null

  const getCharacter = (characterID: string) =>
    options
      .characters()
      .find((character) => character.character_id === characterID)

  const intent = computed(() => {
    const npc = menuNpc.value
    if (!npc) return null
    return createMapTeleportIntent({
      server: options.server(),
      npc,
      destination: destination.value,
      targetIDs: options.selectedTargetIDs(),
    })
  })

  function definition() {
    return mapTeleportCommand({
      getIntent: () => intent.value,
      getCharacter,
      getControls(characterID) {
        const controls = fanout.feed.value?.targets[characterID]?.controls
        return controls ? (controls as ControlsSnapshot) : null
      },
      mapFeedCurrent: options.mapFeedCurrent,
    })
  }

  const fanout = useCommandFanOut({
    command: definition(),
    get scopeKey() {
      const npc = menuNpc.value
      return npc
        ? `teleport:${options.server().toLowerCase()}:${npc.servername || npc.id}`
        : 'unavailable'
    },
    scopeKeyForCharacter(character: CharacterView, operationScopeKey?: string) {
      if (character.server.toLowerCase() !== options.server().toLowerCase()) {
        return `server:${character.server.toLowerCase()}`
      }
      return operationScopeKey || `server:${character.server.toLowerCase()}`
    },
    currentScopeKey(characterID: string) {
      const character = getCharacter(characterID)
      if (!character) return 'unavailable'
      if (character.server.toLowerCase() !== options.server().toLowerCase()) {
        return `server:${character.server.toLowerCase()}`
      }
      const npc = menuNpc.value
      if (npc) {
        return `teleport:${options.server().toLowerCase()}:${npc.servername || npc.id}`
      }
      return `server:${character.server.toLowerCase()}`
    },
    currentCharacter: getCharacter,
  })

  const operations = computed(() => fanout.operations.value)
  const menuOperation = computed(() =>
    operations.value.find(
      (item) => item.operationID === activeOperationID.value,
    ),
  )
  const reviewOperation = computed(() =>
    operations.value.find(
      (item) => item.operationID === reviewingOperationID.value,
    ),
  )

  const gateLabel = computed(() =>
    menuNpc.value ? mapTeleportGateTitle(menuNpc.value) : 'Teleporter',
  )

  const discoveredRoutes = computed(() =>
    menuNpc.value ? sortedTeleportRoutes(menuNpc.value) : [],
  )

  const counts = computed(() =>
    menuOperation.value
      ? fanout.counts(menuOperation.value)
      : { selected: 0, eligible: 0, skipped: 0 },
  )

  const menuSummary = computed(() => {
    if (fanout.error.value) return fanout.error.value
    if (!destination.value.trim())
      return 'Enter a destination town name (for example Jangan).'
    if (!intent.value)
      return 'Destination is invalid or the gate is unavailable.'
    if (!options.selectedTargetIDs().length)
      return 'Tick characters in the panel to teleport them.'
    if (fanout.preparing.value) return ''
    if (!counts.value.eligible) {
      const children = menuOperation.value?.children || []
      const lines = children
        .filter((child) => child.skipReason?.message)
        .map(
          (child) =>
            `${child.characterName}: ${child.skipReason?.message || 'Unavailable.'}`,
        )
      if (lines.length === 1) return lines[0]!
      if (lines.length > 1) return lines.join(' ')
      return options.selectedTargetIDs().length === 1
        ? 'Character cannot teleport.'
        : 'Selected characters cannot use this teleporter.'
    }
    return counts.value.skipped
      ? `${counts.value.skipped} of ${counts.value.selected} characters unavailable.`
      : 'Uses get_teleport_data then one teleport script line per character.'
  })

  const targetLabel = computed(() => {
    const dest = destination.value.trim()
    const ids = options.selectedTargetIDs()
    if (!dest) return 'Teleport'
    if (ids.length === 1) {
      const name = getCharacter(ids[0]!)?.name
      return name ? `Teleport ${name} to ${dest}` : `Teleport to ${dest}`
    }
    if (ids.length > 1) return `Teleport ${ids.length} characters to ${dest}`
    return `Teleport to ${dest}`
  })

  async function prepareOperation() {
    const currentIntent = intent.value
    if (!currentIntent) return
    const sequence = openSequence.value
    const operation = await fanout.prepare(
      [...currentIntent.targetIDs],
      definition(),
      `teleport:${currentIntent.server.toLowerCase()}:${currentIntent.gateServername}`,
    )
    if (sequence !== openSequence.value || !menuOpen.value) {
      if (operation?.state === 'prepared') fanout.dismiss(operation)
      return
    }
    if (operation) activeOperationID.value = operation.operationID
  }

  async function openMenu(
    npc: MapNpc,
    anchor: { x: number; y: number },
    presentation: 'dialog' | 'context',
    focusTarget?: HTMLElement | null,
  ) {
    const previous = menuOperation.value
    if (previous?.state === 'prepared') fanout.dismiss(previous)
    returnFocusElement =
      focusTarget ?? document.querySelector<HTMLElement>('.map-canvas')
    menuPresentation.value = presentation
    menuNpc.value = npc
    destination.value = pickDefaultTeleportDestination(
      npc,
      destination.value.trim() || undefined,
    )
    menuAnchor.value = { x: anchor.x, y: anchor.y }
    activeOperationID.value = ''
    reviewingOperationID.value = ''
    menuOpen.value = true
    openSequence.value++
    await nextTick()
    await prepareOperation()
    await nextTick()
    if (presentation === 'dialog') {
      menuElement.value?.querySelector<HTMLInputElement>('input')?.focus()
    } else {
      menuElement.value?.focus()
    }
  }

  async function open(
    npc: MapNpc,
    anchor: { x: number; y: number },
    focusTarget?: HTMLElement | null,
  ) {
    await openMenu(npc, anchor, 'dialog', focusTarget)
  }

  async function openContext(
    npc: MapNpc,
    anchor: { x: number; y: number },
    focusTarget?: HTMLElement | null,
  ) {
    await openMenu(npc, anchor, 'context', focusTarget)
  }

  async function setDestination(next: string) {
    if (destination.value === next) return
    destination.value = next
    await onDestinationInput()
  }

  function teleportScopeKey() {
    const npc = menuNpc.value
    return npc
      ? `teleport:${options.server().toLowerCase()}:${npc.servername || npc.id}`
      : 'unavailable'
  }

  async function showDestinationDialog() {
    menuPresentation.value = 'dialog'
    await nextTick()
    menuElement.value?.querySelector<HTMLInputElement>('input')?.focus()
  }

  async function submitForCharacter(
    characterID: string,
    nextDestination?: string,
  ) {
    if (nextDestination) destination.value = nextDestination
    const currentIntent = intent.value
    if (!currentIntent) return
    const previous = menuOperation.value
    if (previous?.state === 'prepared') fanout.dismiss(previous)
    activeOperationID.value = ''
    const operation = await fanout.prepare(
      [characterID],
      definition(),
      teleportScopeKey(),
    )
    if (!menuOpen.value || !operation) return
    activeOperationID.value = operation.operationID
    const child = operation.children.find(
      (item) => item.characterID === characterID,
    )
    if (!child || child.submission !== 'ready') return
    if (options.reviewActions() && !reviewingOperationID.value) {
      reviewingOperationID.value = operation.operationID
      return
    }
    reviewingOperationID.value = ''
    await fanout.submit(operation)
    close(false)
  }

  function close(restoreFocus = true) {
    menuOpen.value = false
    menuPresentation.value = 'dialog'
    reviewingOperationID.value = ''
    const operation = menuOperation.value
    if (operation?.state === 'prepared') fanout.dismiss(operation)
    activeOperationID.value = ''
    menuNpc.value = null
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

  async function onDestinationInput() {
    const operation = menuOperation.value
    if (operation?.state === 'prepared') fanout.dismiss(operation)
    activeOperationID.value = ''
    await prepareOperation()
  }

  async function submit() {
    const operation = menuOperation.value
    if (!operation || !counts.value.eligible) return
    if (options.reviewActions() && !reviewingOperationID.value) {
      reviewingOperationID.value = operation.operationID
      return
    }
    reviewingOperationID.value = ''
    await fanout.submit(operation)
    close(false)
  }

  function cancelReview() {
    reviewingOperationID.value = ''
  }

  async function confirmReview() {
    const operation = reviewOperation.value
    if (!operation) return
    reviewingOperationID.value = ''
    await fanout.submit(operation)
    close(false)
  }

  watch(
    () => options.selectedTargetIDs().join('\u0000'),
    () => {
      if (menuOpen.value) void prepareOperation()
    },
  )

  function outsidePointer(event: PointerEvent) {
    const target = event.target
    if (!menuOpen.value || !(target instanceof Node)) return
    if (menuElement.value?.contains(target)) return
    if (
      target instanceof Element &&
      target.closest('.map-teleport-menu, .map-navigation-context')
    )
      return
    close(false)
  }

  function escape(event: KeyboardEvent) {
    if (event.key === 'Escape' && menuOpen.value) {
      event.preventDefault()
      close()
    }
  }

  onMounted(() => {
    window.addEventListener('pointerdown', outsidePointer)
    window.addEventListener('keydown', escape)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('pointerdown', outsidePointer)
    window.removeEventListener('keydown', escape)
    fanout.dispose()
  })

  return {
    menuOpen,
    menuPresentation,
    menuAnchor,
    menuElement,
    menuNpc,
    destination,
    gateLabel,
    discoveredRoutes,
    targetLabel,
    menuSummary,
    counts,
    preparing: fanout.preparing,
    submitting: fanout.submitting,
    operations,
    menuOperation,
    reviewOperation,
    reviewingOperationID,
    open,
    openContext,
    close,
    submit,
    setDestination,
    submitForCharacter,
    showDestinationDialog,
    onDestinationInput,
    cancelReview,
    confirmReview,
    fanout,
  }
}
