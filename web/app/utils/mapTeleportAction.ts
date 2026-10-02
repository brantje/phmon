import type {
  CharacterView,
  ControlsSnapshot,
  MapNpc,
} from '../../shared/types/live'
import type { FanOutCommandDefinition } from './commandFanOut.ts'
import { npcDisplayLabel } from './mapNpcMarkers.ts'

export interface MapTeleportIntent {
  server: string
  gateServername: string
  source: string
  destination: string
  targetIDs: readonly string[]
  npc: Pick<MapNpc, 'name' | 'servername' | 'observers'>
}

export function gateSourceLabel(
  npc: Pick<MapNpc, 'name' | 'servername'>,
): string {
  const name = npc.name?.trim()
  if (name) return name
  const servername = npc.servername?.trim()
  if (servername) return servername
  return ''
}

export function characterObservesGate(
  characterID: string,
  npc: Pick<MapNpc, 'observers'>,
): boolean {
  return Boolean(
    npc.observers?.some((observer) => observer.character_id === characterID),
  )
}

export function observerVerifiedDestination(
  character: Pick<CharacterView, 'character_id' | 'session_id'>,
  npc: Pick<MapNpc, 'observers'>,
  destination: string,
): boolean {
  const wanted = destination.trim().toLowerCase()
  if (!wanted || !character.session_id) return false
  const observer = npc.observers?.find(
    (item) =>
      item.character_id === character.character_id &&
      item.session_id === character.session_id,
  )
  return Boolean(
    observer?.teleport_routes?.some(
      (route) => route.destination.trim().toLowerCase() === wanted,
    ),
  )
}

export function teleportEligibleActionLabel(input: {
  destination: string
  eligible: number
  eligibleName?: string
}): string {
  const destination = input.destination.trim()
  if (!destination) return 'Teleport'
  if (input.eligible === 1) {
    return input.eligibleName
      ? `Teleport ${input.eligibleName} to ${destination}`
      : `Teleport 1 character to ${destination}`
  }
  if (input.eligible > 1)
    return `Teleport ${input.eligible} characters to ${destination}`
  return `Teleport to ${destination}`
}

export function createMapTeleportIntent(input: {
  server: string
  npc: MapNpc
  destination: string
  targetIDs: readonly string[]
}): MapTeleportIntent | null {
  const gateServername = input.npc.servername?.trim() || ''
  if (!gateServername.startsWith('GATE_')) return null
  const source = gateSourceLabel(input.npc)
  const destination = input.destination.trim()
  if (!source || !destination || destination.includes(',')) return null
  if (destination.length > 64 || source.length > 64) return null
  return {
    server: input.server,
    gateServername,
    source,
    destination,
    targetIDs: input.targetIDs,
    npc: {
      name: input.npc.name,
      servername: input.npc.servername,
      observers: input.npc.observers,
    },
  }
}

export function mapTeleportCommand(state: {
  getIntent(): MapTeleportIntent | null
  getCharacter(characterID: string): CharacterView | undefined
  getControls(characterID: string): ControlsSnapshot | null
  mapFeedCurrent(): boolean
}): FanOutCommandDefinition {
  const resolve = () => state.getIntent()
  return {
    name: 'character.teleport',
    label: 'Teleport',
    impact: 'movement',
    preEligibility(character) {
      if (!state.mapFeedCurrent())
        return {
          code: 'stale_map_scope',
          message: 'The current map snapshot is stale.',
        }
      const intent = resolve()
      if (!intent)
        return {
          code: 'teleport_unavailable',
          message: 'Teleport destination or gate is unavailable.',
        }
      if (character.server.toLowerCase() !== intent.server.toLowerCase())
        return {
          code: 'server_mismatch',
          message: 'Character is on a different server scope.',
        }
      if (!characterObservesGate(character.character_id, intent.npc))
        return {
          code: 'gate_not_observed',
          message:
            'This character does not currently observe the selected teleporter gate.',
        }
      if (
        !observerVerifiedDestination(character, intent.npc, intent.destination)
      )
        return {
          code: 'destination_unverified',
          message:
            'This character has not verified the selected teleport destination.',
        }
      return null
    },
    buildArgs() {
      const intent = resolve()
      if (!intent) throw new Error('Teleport intent is unavailable.')
      return {
        source: intent.source,
        destination: intent.destination,
        gate_servername: intent.gateServername,
      }
    },
    summarizeArgs(args) {
      return `${args.source} → ${args.destination} (${args.gate_servername})`
    },
    eligibility(character, _args, controls) {
      if (!controls.capabilities['character.teleport']?.supported)
        return {
          code: 'unsupported',
          message:
            'This session does not report teleporter script support (get_npcs, get_teleport_data, start_script).',
        }
      if (!character.online || !character.session_id)
        return {
          code: 'offline',
          message: 'Character is offline.',
        }
      return null
    },
    admissionGuard(child, request, context) {
      if (!state.mapFeedCurrent()) {
        return {
          code: 'stale_map_scope',
          message: 'The current map snapshot is stale.',
        }
      }
      const intent = resolve()
      if (!intent) {
        return {
          code: 'teleport_unavailable',
          message: 'Teleport intent expired. Reopen the teleporter action.',
        }
      }
      if (
        intent.targetIDs.indexOf(child.characterID) < 0 ||
        request.character_id !== child.characterID ||
        request.expected_session_id !== child.sessionID
      ) {
        return {
          code: 'session_changed',
          message: 'Character session or target selection changed.',
        }
      }
      const current = state.getCharacter(child.characterID)
      if (!current || current.session_id !== child.sessionID) {
        return {
          code: 'session_changed',
          message: 'Character session changed.',
        }
      }
      const controls = state.getControls(child.characterID)
      if (
        !controls ||
        controls.character_id !== child.characterID ||
        controls.session_id !== child.sessionID ||
        !controls.capabilities['character.teleport']?.supported
      ) {
        return {
          code: 'unsupported',
          message:
            'Teleporter commands are no longer supported for this session.',
        }
      }
      if (!characterObservesGate(child.characterID, intent.npc)) {
        return {
          code: 'gate_not_observed',
          message:
            'The teleporter gate is no longer observed for this character.',
        }
      }
      if (
        !current.session_id ||
        !observerVerifiedDestination(current, intent.npc, intent.destination)
      ) {
        return {
          code: 'destination_unverified',
          message:
            'This character has not verified the selected teleport destination.',
        }
      }
      if (
        !context?.exactRetry &&
        (request.args.source !== intent.source ||
          request.args.destination !== intent.destination ||
          request.args.gate_servername !== intent.gateServername)
      ) {
        return {
          code: 'arguments_changed',
          message:
            'Destination or gate changed after preparation. Reopen the teleporter action.',
        }
      }
      return null
    },
  }
}

export function mapTeleportGateTitle(npc: MapNpc): string {
  return npcDisplayLabel(npc)
}
