export function deathCauseLabel(payload: Record<string, unknown>) {
  const reasonType = payload.reason_type
  const reasonValue = payload.reason_value
  if (
    reasonType === 'attacker' &&
    typeof reasonValue === 'string' &&
    reasonValue.trim()
  )
    return `Recent attacker: ${reasonValue.trim()}`
  if (reasonType === 'monster_or_environment') return 'Monster / environment'

  const cause = payload.cause
  if (typeof cause !== 'string' || !cause.trim() || cause === 'unknown')
    return 'Unknown cause'
  if (cause === 'monster_environment') return 'Monster / environment'
  return cause.replaceAll('_', ' ')
}
