import {
  hasReverseReturnControlCharacters,
  reverseReturnEligibility,
  reverseReturnSummary,
} from './reverseReturn.ts'
import type { FanOutCommandDefinition } from '~/utils/commandFanOut'

export type RemoteControlActionName =
  | 'bot.start'
  | 'bot.stop'
  | 'trace.start'
  | 'trace.stop'
  | 'character.return'
  | 'character.reverse_return'
  | 'character.disconnect'
  | 'client.clientless'
  | 'training.area.set'
  | 'training.radius.set'

export interface RemoteControlArgs {
  reverseReturnType?: number
  reverseReturnName?: string
  traceName?: string
  trainingAreaMode?: 'current_position' | 'named'
  trainingAreaName?: string
  trainingRadius?: string | number
}

function noActiveTrainingArea() {
  return {
    code: 'no_training_area',
    message:
      'This session reports no active training area. Create one in phBot first.',
  }
}

function trainingAreaReadbackSummary(
  controls: Parameters<
    NonNullable<FanOutCommandDefinition['summarizeArgs']>
  >[1],
) {
  const training = controls.training
  if (
    !training ||
    training.session_id !== controls.session_id ||
    !training.observed_at
  )
    return 'Training-area readback unavailable; active area is unconfirmed'

  const detail = [
    training.training_zone,
    training.training_radius == null
      ? undefined
      : `reported radius ${training.training_radius}`,
  ]
    .filter(Boolean)
    .join(' · ')
  return detail
    ? `Active training area reported · ${detail}`
    : 'Active training area reported'
}

export function remoteControlDefinition(
  name: RemoteControlActionName,
  args: RemoteControlArgs = {},
  scopeSnapshotCurrent = true,
): FanOutCommandDefinition {
  const inputArgs = Object.freeze({ ...args })
  const definitions: Record<
    RemoteControlActionName,
    Omit<FanOutCommandDefinition, 'name'>
  > = {
    'bot.start': {
      label: 'Start Training',
      impact: 'routine',
      buildArgs: () => ({}),
      eligibility: (character) =>
        character.botting === true
          ? {
              code: 'already_botting',
              message:
                'The latest observed state says this character is already botting.',
            }
          : null,
    },
    'bot.stop': {
      label: 'Stop Training',
      impact: 'routine',
      buildArgs: () => ({}),
      eligibility: (character) =>
        character.botting === false
          ? {
              code: 'not_botting',
              message:
                'The latest observed state says this character is not botting.',
            }
          : null,
    },
    'trace.start': {
      label: 'Start Trace',
      impact: 'routine',
      buildArgs: () => ({ name: inputArgs.traceName }),
    },
    'trace.stop': {
      label: 'Stop Trace',
      impact: 'routine',
      buildArgs: () => ({}),
    },
    'character.reverse_return': {
      label: 'Reverse return',
      impact: 'movement',
      buildArgs: () => ({
        type: inputArgs.reverseReturnType ?? 0,
        name: inputArgs.reverseReturnName ?? '',
      }),
      eligibility: (character, commandArgs, controls) =>
        reverseReturnEligibility(character, controls, commandArgs),
      summarizeArgs: (commandArgs, controls) =>
        reverseReturnSummary(commandArgs, controls),
    },
    'character.return': {
      label: 'Return Scroll',
      impact: 'movement',
      buildArgs: () => ({}),
    },
    'character.disconnect': {
      label: 'Disconnect',
      impact: 'disruptive',
      buildArgs: () => ({}),
    },
    'client.clientless': {
      label: 'Go Clientless',
      impact: 'disruptive',
      buildArgs: () => ({}),
    },
    'training.area.set': {
      label: 'Set Training Area',
      impact: 'routine',
      buildArgs: () =>
        inputArgs.trainingAreaMode === 'named'
          ? { mode: 'named', name: inputArgs.trainingAreaName }
          : { mode: 'current_position' },
      eligibility: (_character, commandArgs, controls) => {
        if (
          commandArgs.mode !== 'current_position' ||
          controls.training?.session_id !== controls.session_id ||
          !controls.training.observed_at ||
          controls.training.training_available
        )
          return null
        return noActiveTrainingArea()
      },
      summarizeArgs: (commandArgs, controls) => {
        if (commandArgs.mode === 'named')
          return `Select named training area “${String(commandArgs.name)}” in this character’s phBot profile`
        return [
          'Move the active training-area center to each character’s current position when phBot executes the command',
          trainingAreaReadbackSummary(controls),
        ].join(' · ')
      },
    },
    'training.radius.set': {
      label: 'Set Training Radius',
      impact: 'routine',
      buildArgs: () => ({ radius: Number(inputArgs.trainingRadius) }),
      eligibility: (_character, _commandArgs, controls) => {
        if (
          controls.training?.session_id !== controls.session_id ||
          !controls.training.observed_at ||
          controls.training.training_available
        )
          return null
        return noActiveTrainingArea()
      },
      summarizeArgs: (commandArgs, controls) =>
        `Set this character’s active training-area radius to ${commandArgs.radius} · ${trainingAreaReadbackSummary(controls)}`,
    },
  }
  const definition = definitions[name]
  return {
    name,
    ...definition,
    preEligibility: () =>
      scopeSnapshotCurrent
        ? null
        : {
            code: 'stale_map_scope',
            message:
              'The current map scope is stale. Refresh it before acting.',
          },
  }
}

export function requiresRemoteControlConfirmation(
  name: RemoteControlActionName,
  optionalReview: boolean,
) {
  return optionalReview || remoteControlDefinition(name).impact !== 'routine'
}

export function validateRemoteControlArgs(
  name: RemoteControlActionName,
  input: RemoteControlArgs,
): RemoteControlArgs | null {
  if (name === 'character.reverse_return') {
    const type = input.reverseReturnType ?? 0
    const name = input.reverseReturnName ?? ''
    if (
      !Number.isInteger(type) ||
      type < 0 ||
      type > 2 ||
      (type < 2 && name !== '') ||
      hasReverseReturnControlCharacters(name)
    )
      return null
    const trimmed = name.trim()
    if (
      new TextEncoder().encode(trimmed).length > 100 ||
      (type < 2 && trimmed) ||
      (type === 2 && !trimmed)
    )
      return null
    return { reverseReturnType: type, reverseReturnName: trimmed }
  }
  if (name === 'trace.start') {
    const value = input.traceName?.trim() || ''
    const bytes = new TextEncoder().encode(value)
    if (!value || bytes.length > 64 || value.includes('\0')) return null
    return { traceName: value }
  }
  if (name === 'training.area.set' && input.trainingAreaMode === 'named') {
    const value = input.trainingAreaName?.trim() || ''
    const bytes = new TextEncoder().encode(value)
    if (!value || bytes.length > 100 || value.includes('\0')) return null
    return { trainingAreaMode: 'named', trainingAreaName: value }
  }
  if (name === 'training.area.set')
    return { trainingAreaMode: 'current_position' }
  if (name === 'training.radius.set') {
    const rawValue = String(input.trainingRadius ?? '').trim()
    if (!rawValue) return null
    const value = Number(rawValue)
    return Number.isFinite(value) && value >= 1 && value <= 10_000
      ? { trainingRadius: String(value) }
      : null
  }
  return {}
}

export function remoteControlSignature(
  name: RemoteControlActionName,
  input: RemoteControlArgs,
  targetIDs: string[],
  scopeKey: string,
) {
  return JSON.stringify([name, input, [...new Set(targetIDs)], scopeKey])
}
