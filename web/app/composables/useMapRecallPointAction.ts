import type {
  CharacterView,
  ControlsSnapshot,
  MapNpc,
} from '~~/shared/types/live'
import { useCommandFanOut } from '~/composables/useCommandFanOut'
import {
  createMapRecallPointIntent,
  mapRecallPointCommand,
} from '~/utils/mapRecallPointAction'

export function useMapRecallPointAction(options: {
  server(): string
  selectedTargetIDs(): string[]
  characters(): CharacterView[]
  gates(): MapNpc[]
  mapFeedCurrent(): boolean
}) {
  const menuOpen = ref(false)
  const menuAnchor = ref({ x: 0, y: 0 })
  const menuElement = ref<HTMLElement | null>(null)
  const selectedGate = ref<MapNpc | null>(null)
  const activeOperationID = ref('')
  let openSequence = 0
  let returnFocusElement: HTMLElement | null = null

  const getCharacter = (id: string) =>
    options.characters().find((character) => character.character_id === id)
  const currentGate = () =>
    options.gates().find((gate) => gate.id === selectedGate.value?.id) || null
  const intent = computed(() =>
    selectedGate.value
      ? createMapRecallPointIntent({
          server: options.server(),
          gate: selectedGate.value,
          targetIDs: options.selectedTargetIDs(),
        })
      : null,
  )
  const definition = () =>
    mapRecallPointCommand({
      getIntent: () => intent.value,
      getCurrentGate: currentGate,
      getCharacter,
      getControls(id) {
        const controls = fanout.feed.value?.targets[id]?.controls
        return controls ? (controls as ControlsSnapshot) : null
      },
      mapFeedCurrent: options.mapFeedCurrent,
    })
  const scopeKey = () => {
    const gate = selectedGate.value
    return gate
      ? `recall:${options.server().toLowerCase()}:${gate.id}`
      : 'unavailable'
  }
  const fanout = useCommandFanOut({
    command: definition(),
    get scopeKey() {
      return scopeKey()
    },
    scopeKeyForCharacter(character, operationScopeKey) {
      if (character.server.toLowerCase() !== options.server().toLowerCase())
        return `server:${character.server.toLowerCase()}`
      return operationScopeKey || scopeKey()
    },
    currentScopeKey(id) {
      const character = getCharacter(id)
      if (!character) return 'unavailable'
      if (character.server.toLowerCase() !== options.server().toLowerCase())
        return `server:${character.server.toLowerCase()}`
      return scopeKey()
    },
    currentCharacter: getCharacter,
  })
  const operations = computed(() => fanout.operations.value)
  const reviewOperation = computed(() =>
    operations.value.find(
      (item) => item.operationID === activeOperationID.value,
    ),
  )
  const counts = computed(() =>
    reviewOperation.value
      ? fanout.counts(reviewOperation.value)
      : { selected: 0, eligible: 0, skipped: 0 },
  )

  async function prepare() {
    const current = intent.value
    if (!current || !menuOpen.value) return
    const previous = reviewOperation.value
    if (previous?.state === 'prepared') fanout.dismiss(previous)
    activeOperationID.value = ''
    const sequence = ++openSequence
    const operation = await fanout.prepare(
      [...current.targetIDs],
      definition(),
      scopeKey(),
    )
    if (sequence !== openSequence || !menuOpen.value) {
      if (operation?.state === 'prepared') fanout.dismiss(operation)
      return
    }
    if (operation) activeOperationID.value = operation.operationID
    await nextTick()
    const rect = menuElement.value?.getBoundingClientRect()
    if (rect) {
      const margin = 8
      menuAnchor.value = {
        x: Math.max(
          margin,
          Math.min(menuAnchor.value.x, window.innerWidth - rect.width - margin),
        ),
        y: Math.max(
          margin,
          Math.min(
            menuAnchor.value.y,
            window.innerHeight - rect.height - margin,
          ),
        ),
      }
    }
    menuElement.value?.focus()
  }

  async function open(
    gate: MapNpc,
    anchor: { x: number; y: number },
    focusTarget?: HTMLElement | null,
  ) {
    close(false)
    selectedGate.value = gate
    menuAnchor.value = { ...anchor }
    menuOpen.value = true
    returnFocusElement = focusTarget || null
    await prepare()
  }

  function close(restoreFocus = true) {
    openSequence++
    const operation = reviewOperation.value
    if (operation?.state === 'prepared') fanout.dismiss(operation)
    activeOperationID.value = ''
    selectedGate.value = null
    menuOpen.value = false
    if (restoreFocus && returnFocusElement?.isConnected)
      requestAnimationFrame(() => returnFocusElement?.focus())
    returnFocusElement = null
  }

  async function confirm() {
    const operation = reviewOperation.value
    if (!operation || operation.state !== 'prepared' || !counts.value.eligible)
      return
    await fanout.submit(operation)
    close(false)
  }

  watch(
    () => options.mapFeedCurrent(),
    (current) => {
      if (!current && menuOpen.value) close(false)
    },
  )
  watch(
    () => options.server(),
    () => {
      if (menuOpen.value) close(false)
    },
  )
  watch(
    () => currentGate(),
    (gate) => {
      const selected = selectedGate.value
      if (
        menuOpen.value &&
        (!selected ||
          !gate ||
          gate.servername !== selected.servername ||
          gate.region !== selected.region ||
          gate.x !== selected.x ||
          gate.y !== selected.y ||
          gate.model_id !== selected.model_id)
      )
        close(false)
    },
  )
  watch(
    () =>
      options
        .selectedTargetIDs()
        .map((id) => `${id}:${getCharacter(id)?.session_id || ''}`)
        .join('\u0000'),
    () => {
      if (menuOpen.value) close(false)
    },
  )

  function outsidePointer(event: PointerEvent) {
    if (!menuOpen.value || !(event.target instanceof Node)) return
    if (menuElement.value?.contains(event.target)) return
    close(false)
  }

  function escape(event: KeyboardEvent) {
    if (event.key !== 'Escape' || !menuOpen.value) return
    event.preventDefault()
    close()
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
    menuAnchor,
    menuElement,
    selectedGate,
    operations,
    reviewOperation,
    counts,
    preparing: fanout.preparing,
    submitting: fanout.submitting,
    error: fanout.error,
    open,
    close,
    confirm,
    fanout,
  }
}
