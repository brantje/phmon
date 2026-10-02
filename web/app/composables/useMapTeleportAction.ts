import type { CharacterView, ControlsSnapshot, MapNpc } from '../../shared/types/live'
import { useCommandFanOut } from '~/composables/useCommandFanOut'
import {
  createMapTeleportIntent,
  mapTeleportCommand,
  mapTeleportGateTitle,
} from '~/utils/mapTeleportAction'

export function useMapTeleportAction(options: {
  server(): string
  selectedTargetIDs(): string[]
  characters(): CharacterView[]
  mapFeedCurrent(): boolean
  reviewActions(): boolean
}) {
  const menuOpen = ref(false)
  const menuAnchor = ref({ x: 0, y: 0 })
  const menuNpc = ref<MapNpc | null>(null)
  const destination = ref('')
  const menuElement = ref<HTMLElement | null>(null)
  const activeOperationID = ref('')
  const reviewingOperationID = ref('')
  const openSequence = ref(0)
  let returnFocusElement: HTMLElement | null = null

  const getCharacter = (characterID: string) =>
    options.characters().find((character) => character.character_id === characterID)

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
      return character
        ? `server:${character.server.toLowerCase()}`
        : 'unavailable'
    },
    currentCharacter: getCharacter,
  })

  const operations = computed(() => fanout.operations.value)
  const menuOperation = computed(() =>
    operations.value.find((item) => item.operationID === activeOperationID.value),
  )
  const reviewOperation = computed(() =>
    operations.value.find(
      (item) => item.operationID === reviewingOperationID.value,
    ),
  )

  const gateLabel = computed(() =>
    menuNpc.value ? mapTeleportGateTitle(menuNpc.value) : 'Teleporter',
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

  async function open(
    npc: MapNpc,
    anchor: { x: number; y: number },
    focusTarget?: HTMLElement | null,
  ) {
    const previous = menuOperation.value
    if (previous?.state === 'prepared') fanout.dismiss(previous)
    returnFocusElement =
      focusTarget ?? document.querySelector<HTMLElement>('.map-canvas')
    menuNpc.value = npc
    if (!destination.value.trim() && npc.name?.toLowerCase() === 'hotan') {
      destination.value = 'Jangan'
    }
    menuAnchor.value = { x: anchor.x, y: anchor.y }
    activeOperationID.value = ''
    reviewingOperationID.value = ''
    menuOpen.value = true
    openSequence.value++
    await nextTick()
    await prepareOperation()
    await nextTick()
    menuElement.value?.querySelector<HTMLInputElement>('input')?.focus()
  }

  function close(restoreFocus = true) {
    menuOpen.value = false
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

  function escape(event: KeyboardEvent) {
    if (event.key === 'Escape' && menuOpen.value) {
      event.preventDefault()
      close()
    }
  }

  onMounted(() => {
    window.addEventListener('keydown', escape)
  })
  onBeforeUnmount(() => {
    window.removeEventListener('keydown', escape)
    fanout.dispose()
  })

  return {
    menuOpen,
    menuAnchor,
    menuElement,
    menuNpc,
    destination,
    gateLabel,
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
    close,
    submit,
    onDestinationInput,
    cancelReview,
    confirmReview,
    fanout,
  }
}
