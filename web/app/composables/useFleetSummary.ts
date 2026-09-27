import type { CharacterView as Character } from '~~/shared/types/live'

export function useFleetSummary() {
  const {
    agents: lastAgents,
    fleetCharacters: lastFleetCharacters,
    connectionState: liveConnectionState,
    liveStale,
  } = useLiveData()
  const onlineCharacterCount = computed(
    () =>
      lastFleetCharacters.value.filter((character) => character.online).length,
  )
  const offlineCharacterCount = computed(
    () =>
      lastFleetCharacters.value.filter((character) => !character.online).length,
  )
  const characterCountsUnknown = computed(
    () =>
      liveConnectionState.value !== 'current' &&
      lastFleetCharacters.value.length === 0,
  )
  function displayCharacterCount(value: number) {
    return characterCountsUnknown.value ? '—' : String(value)
  }
  const combinedVitals = computed(() => {
    const withHP = lastFleetCharacters.value.filter(
      (character) =>
        character.hp != null &&
        character.hp_max != null &&
        character.hp_max > 0,
    )
    const withMP = lastFleetCharacters.value.filter(
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
    const values = lastFleetCharacters.value
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
    displayCharacterCount,
    combinedVitals,
    observedGold,
    connectedAgents,
    fleetStatus,
  }
}
