import type {
  CharacterView,
  ControlsSnapshot,
} from '../../shared/types/live.ts'
import type { FanOutSkipReason } from './commandFanOut.ts'

export function hasReverseReturnControlCharacters(value: string) {
  return [...value].some((char) => {
    const code = char.codePointAt(0)!
    return code < 32 || (code >= 127 && code <= 159)
  })
}

export const reverseReturnModes = [
  { type: 0, label: 'Last Return Scroll location' },
  { type: 1, label: 'Last death location' },
  { type: 2, label: 'Party member…' },
  { type: 3, label: 'Named location…' },
] as const
export const namedLocationReason =
  'No named locations are available for the selected server profiles.'

export function reverseReturnNamedLocations(
  controls: (
    | Pick<ControlsSnapshot, 'session_id' | 'reverse_return_named_locations'>
    | null
    | undefined
  )[],
) {
  const names = new Set<string>()
  for (const control of controls) {
    if (!control?.session_id) continue
    for (const name of control.reverse_return_named_locations ?? []) {
      if (
        name &&
        name === name.trim() &&
        new TextEncoder().encode(name).length <= 100 &&
        !hasReverseReturnControlCharacters(name)
      )
        names.add(name)
    }
  }
  return [...names].sort((a, b) => a.localeCompare(b))
}

export function freshReverseReturnParty(
  controls:
    Pick<ControlsSnapshot, 'session_id' | 'reverse_return'> | null | undefined,
  now = Date.now(),
) {
  const context = controls?.reverse_return
  const checked = Date.parse(context?.party_checked_at ?? '')
  return (
    !!context &&
    context.session_id === controls?.session_id &&
    context.party_status === 'observed' &&
    Number.isFinite(checked) &&
    now - checked >= -5000 &&
    now - checked <= 35000
  )
}

export function reverseReturnPartyNames(
  controls: (
    Pick<ControlsSnapshot, 'session_id' | 'reverse_return'> | null | undefined
  )[],
  now = Date.now(),
): string[] {
  const names = new Map<string, string>()
  for (const control of controls) {
    if (!freshReverseReturnParty(control, now)) continue
    for (const name of control!.reverse_return!.party_names.slice(0, 32)) {
      const trimmed = name.trim()
      if (
        trimmed &&
        new TextEncoder().encode(trimmed).length <= 100 &&
        !hasReverseReturnControlCharacters(name)
      )
        names.set(trimmed.toLowerCase(), trimmed)
    }
  }
  return [...names.values()].sort((a, b) => a.localeCompare(b))
}

export function reverseReturnEligibility(
  character: CharacterView,
  controls: ControlsSnapshot,
  args: Record<string, unknown>,
  now = Date.now(),
): FanOutSkipReason | null {
  const type = args.type
  const mode =
    type === 0
      ? 'last_return'
      : type === 1
        ? 'last_death'
        : type === 2
          ? 'party_member'
          : type === 3
            ? 'named_location'
            : ''
  if (
    !mode ||
    !controls.capabilities['character.reverse_return']?.modes?.includes(mode)
  )
    return {
      code: 'unsupported_argument_mode',
      message:
        'This runtime does not support the selected Reverse return mode.',
    }
  if (type === 3) {
    const names = controls.reverse_return_named_locations ?? []
    if (!names.length)
      return {
        code: 'named_location_names_unavailable',
        message: namedLocationReason,
      }
    if (!names.includes(String(args.name ?? '')))
      return {
        code: 'named_location_not_found',
        message:
          'The selected location is unavailable in this character’s server profile.',
      }
    return null
  }
  if (type !== 2) return null
  const name = String(args.name ?? '')
    .trim()
    .toLowerCase()
  if (name === character.name.toLowerCase())
    return {
      code: 'party_self_target',
      message: 'A character cannot Reverse return to itself.',
    }
  if (!freshReverseReturnParty(controls, now))
    return {
      code: 'party_unavailable',
      message: 'Fresh party membership is unavailable for this session.',
    }
  if (
    !controls.reverse_return!.party_names.some(
      (member) => member.toLowerCase() === name,
    )
  )
    return {
      code: 'party_member_not_found',
      message: 'The selected member is not in this character’s party.',
    }
  return null
}

export function reverseReturnSummary(
  args: Record<string, unknown>,
  controls: ControlsSnapshot,
) {
  const context = controls.reverse_return
  const checked = Date.parse(context?.inventory_checked_at ?? '')
  const age = Date.now() - checked
  const freshInventory =
    context?.session_id === controls.session_id &&
    Number.isFinite(checked) &&
    age >= -5000 &&
    age <= 35000
  const destination =
    args.type === 0
      ? 'last Return Scroll location'
      : args.type === 1
        ? 'last death location'
        : args.type === 2
          ? `party member ${args.name}`
          : `named location ${args.name}`
  const inventory =
    freshInventory && context?.scroll_observed === true
      ? 'Scroll observed in inventory'
      : 'Usable scroll availability will be checked by phBot'
  return `Use a Reverse Return Scroll to ${destination}. ${inventory}. Scroll acceptance does not prove arrival.`
}
