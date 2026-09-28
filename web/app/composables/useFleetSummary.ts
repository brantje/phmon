import type { CharacterView as Character } from '~~/shared/types/live'
import { characterDeathState } from '../utils/characterDeath'

export function useFleetSummary() {
  const {
    agents: lastAgents,
    fleetCharacters: lastFleetCharacters,
    connectionState: liveConnectionState,
    liveStale,
    freshnessNow,
  } = useLiveData()
  const { matchesServer } = useServerScope()
  const scopedCharacters = computed(() =>
    lastFleetCharacters.value.filter((character) =>
      matchesServer(character.server),
    ),
  )
  const onlineCharacterCount = computed(
    () => scopedCharacters.value.filter((character) => character.online).length,
  )
  const offlineCharacterCount = computed(
    () =>
      scopedCharacters.value.filter((character) => !character.online).length,
  )
  const deathCounts = computed(() => {
    const result = { alive: 0, dead: 0, unknown: 0 }
    for (const character of scopedCharacters.value) {
      if (!character.online) continue
      const state = characterDeathState(character, false, freshnessNow.value)
      if (state === 'alive') result.alive += 1
      else if (state === 'dead') result.dead += 1
      else result.unknown += 1
    }
    return result
  })
  const displayDeathCount = (value: number) =>
    liveConnectionState.value !== 'current' &&
    scopedCharacters.value.length === 0
      ? '—'
      : String(value)
  const characterCountsUnknown = computed(
    () =>
      liveConnectionState.value !== 'current' &&
      scopedCharacters.value.length === 0,
  )
  function displayCharacterCount(value: number) {
    return characterCountsUnknown.value ? '—' : String(value)
  }
  const combinedVitals = computed(() => {
    const withHP = scopedCharacters.value.filter(
      (character) =>
        character.hp != null &&
        character.hp_max != null &&
        character.hp_max > 0,
    )
    const withMP = scopedCharacters.value.filter(
      (character) =>
        character.mp != null &&
        character.mp_max != null &&
        character.mp_max > 0,
    )
    const ratio = (
      items: Character[],
      current: 'hp' | 'mp',
      max: 'hp_max' | 'mp_max',
    ) =>
      items.length
        ? `${(
            (items.reduce(
              (sum, character) => sum + (character[current] || 0),
              0,
            ) /
              items.reduce(
                (sum, character) => sum + (character[max] || 0),
                0,
              )) *
            100
          ).toFixed(1)}%`
        : '—'
    return {
      hp: ratio(withHP, 'hp', 'hp_max'),
      mp: ratio(withMP, 'mp', 'mp_max'),
    }
  })
  const observedGold = computed(() => {
    const values = scopedCharacters.value
      .map((character) => character.gold)
      .filter((value): value is number => value != null)
    return values.length
      ? new Intl.NumberFormat('en', {
          notation: 'compact',
          maximumFractionDigits: 1,
        }).format(values.reduce((sum, value) => sum + value, 0))
      : '—'
  })

  const connectedAgents = computed<number | null>(() =>
    liveStale.value || liveConnectionState.value === 'stale'
      ? null
      : lastAgents.value.filter((agent) => agent.connected).length,
  )
  const fleetStatus = computed(() => {
    if (liveStale.value) return 'Live data stale'
    if (liveConnectionState.value !== 'current') return 'Connecting live data'
    if (lastAgents.value.length === 0) return 'Waiting for agents'
    return (connectedAgents.value ?? 0) > 0
      ? 'Agents connected'
      : 'Fleet offline'
  })

  return {
    onlineCharacterCount,
    offlineCharacterCount,
    aliveCharacterCount: computed(() => deathCounts.value.alive),
    deadCharacterCount: computed(() => deathCounts.value.dead),
    unknownDeathCharacterCount: computed(() => deathCounts.value.unknown),
    displayDeathCount,
    displayCharacterCount,
    combinedVitals,
    observedGold,
    connectedAgents,
    fleetStatus,
  }
}
