import type { ControlsSnapshot } from '~~/shared/types/live'

export function traceActivitySummary(
  controls: ControlsSnapshot | null | undefined,
): string[] {
  const state = controls?.training
  if (!state) return []
  const lines: string[] = []
  const activity = state.activity_state
  if (activity === 'tracing') lines.push('Tracing')
  else if (activity === 'not_tracing') lines.push('Not tracing')
  else if (activity === 'unknown') lines.push('Trace unknown')
  const requested = state.trace_requested_name?.trim()
  if (requested) lines.push(`Requested: ${requested}`)
  return lines
}
