import type {
  CharacterView,
  ControlsSnapshot,
  MapNpc,
} from '../../shared/types/live'
import type { FanOutCommandDefinition } from './commandFanOut.ts'

export interface MapRecallPointIntent {
  server: string
  gate: MapNpc
  targetIDs: readonly string[]
}

export function createMapRecallPointIntent(input: {
  server: string
  gate: MapNpc
  targetIDs: readonly string[]
}): MapRecallPointIntent | null {
  const gate = input.gate
  if (
    gate.role !== 'teleporter' ||
    !/^GATE_[A-Za-z0-9_]+$/.test(gate.servername || '') ||
    !Number.isInteger(gate.region) ||
    gate.region === 0 ||
    gate.region < -32768 ||
    gate.region > 65535 ||
    !Number.isFinite(gate.x) ||
    !Number.isFinite(gate.y) ||
    Math.abs(gate.x) > 10_000_000 ||
    Math.abs(gate.y) > 10_000_000 ||
    (gate.model_id !== undefined &&
      (!Number.isInteger(gate.model_id) ||
        gate.model_id < 1 ||
        gate.model_id > 0xffffffff))
  )
    return null
  return {
    server: input.server,
    gate,
    targetIDs: [...new Set(input.targetIDs)],
  }
}

function observesGate(
  character: Pick<CharacterView, 'character_id' | 'session_id'>,
  gate: MapNpc,
) {
  return Boolean(
    character.session_id &&
    gate.observers.some(
      (observer) =>
        observer.character_id === character.character_id &&
        observer.session_id === character.session_id,
    ),
  )
}

export function mapRecallPointCommand(state: {
  getIntent(): MapRecallPointIntent | null
  getCurrentGate(): MapNpc | null
  getCharacter(characterID: string): CharacterView | undefined
  getControls(characterID: string): ControlsSnapshot | null
  mapFeedCurrent(): boolean
}): FanOutCommandDefinition {
  const unavailable = (code: string, message: string) => ({ code, message })
  const sameGate = (expected: MapNpc, current: MapNpc | null) =>
    Boolean(
      current &&
      current.id === expected.id &&
      current.servername === expected.servername &&
      current.region === expected.region &&
      current.x === expected.x &&
      current.y === expected.y &&
      current.model_id === expected.model_id,
    )
  const argsFor = (gate: MapNpc) => ({
    gate_servername: gate.servername!,
    region: gate.region,
    x: gate.x,
    y: gate.y,
    ...(gate.model_id === undefined ? {} : { model_id: gate.model_id }),
  })
  return {
    name: 'character.recall_point.designate',
    label: 'Designate Recall Point',
    impact: 'disruptive',
    preEligibility(character) {
      const intent = state.getIntent()
      const currentGate = state.getCurrentGate()
      if (!state.mapFeedCurrent())
        return unavailable(
          'stale_map_scope',
          'The current map snapshot is stale.',
        )
      if (!intent || !sameGate(intent.gate, currentGate))
        return unavailable('gate_changed', 'The selected teleporter changed.')
      if (character.server.toLowerCase() !== intent.server.toLowerCase())
        return unavailable(
          'server_mismatch',
          'Character is on a different server.',
        )
      if (!observesGate(character, currentGate!))
        return unavailable(
          'gate_not_observed',
          'This character does not currently observe the selected teleporter.',
        )
      return null
    },
    buildArgs() {
      const intent = state.getIntent()
      if (!intent) throw new Error('Recall point intent is unavailable.')
      return argsFor(intent.gate)
    },
    summarizeArgs(args) {
      return `Set recall point at ${String(args.gate_servername)} (${String(args.region)}; ${String(args.x)}, ${String(args.y)})`
    },
    eligibility(character, _args, controls) {
      if (!character.online || !character.session_id)
        return unavailable('offline', 'Character is offline.')
      if (!controls.capabilities['character.recall_point.designate']?.supported)
        return unavailable(
          'unsupported',
          'This phBot session has not verified recall-point designation.',
        )
      return null
    },
    admissionGuard(child, request, context) {
      const intent = state.getIntent()
      const current = state.getCharacter(child.characterID)
      const gate = state.getCurrentGate()
      if (!state.mapFeedCurrent())
        return unavailable(
          'stale_map_scope',
          'The current map snapshot is stale.',
        )
      if (!intent || !sameGate(intent.gate, gate))
        return unavailable('gate_changed', 'The selected teleporter changed.')
      if (!current || current.session_id !== child.sessionID)
        return unavailable('session_changed', 'Character session changed.')
      if (!observesGate(current, gate!))
        return unavailable(
          'gate_not_observed',
          'This character no longer observes the selected teleporter.',
        )
      const controls = state.getControls(child.characterID)
      if (
        !controls ||
        controls.session_id !== child.sessionID ||
        !controls.capabilities['character.recall_point.designate']?.supported
      )
        return unavailable(
          'unsupported',
          'Recall-point capability is unavailable.',
        )
      if (
        !context?.exactRetry &&
        JSON.stringify(request.args) !== JSON.stringify(argsFor(gate!))
      )
        return unavailable(
          'arguments_changed',
          'The selected teleporter changed.',
        )
      return null
    },
  }
}
