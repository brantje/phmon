import type {
  CharacterView,
  MapOtherPlayersSnapshot,
} from '~~/shared/types/live'

export type NearbyPlayersStatus = 'available' | 'empty' | 'unavailable'

export interface TracePickerOption {
  value: string
  label: string
  group: 'managed' | 'nearby'
}

export function nearbyPlayersStatus(
  snapshot: MapOtherPlayersSnapshot | undefined,
  protocolVersion?: number,
): NearbyPlayersStatus {
  if (!snapshot || snapshot.status === 'unavailable') return 'unavailable'
  if ((protocolVersion ?? 0) < 10) return 'unavailable'
  if (!snapshot.players.length) return 'empty'
  return 'available'
}

export function tracePickerOptions(input: {
  managed: CharacterView[]
  players?: MapOtherPlayersSnapshot
  protocolVersion?: number
}): { options: TracePickerOption[]; nearbyStatus: NearbyPlayersStatus } {
  const managedNames = new Set(
    input.managed.map((character) => character.name.toLowerCase()),
  )
  const options: TracePickerOption[] = input.managed.map((character) => ({
    value: character.name,
    label: character.name,
    group: 'managed',
  }))
  const nearbyStatus = nearbyPlayersStatus(input.players, input.protocolVersion)
  if (nearbyStatus === 'available') {
    const seen = new Set<string>()
    for (const player of input.players?.players || []) {
      const name = player.name.trim()
      if (!name) continue
      const key = name.toLowerCase()
      if (managedNames.has(key) || seen.has(key)) continue
      seen.add(key)
      options.push({ value: name, label: name, group: 'nearby' })
    }
    options.sort((left, right) => {
      if (left.group !== right.group)
        return left.group === 'managed' ? -1 : 1
      return left.label.localeCompare(right.label)
    })
  }
  return { options, nearbyStatus }
}
