import { createIdempotencyKey } from '~/utils/createIdempotencyKey'
import type { NavigationRoute } from '~~/shared/types/live'

export const NAVIGATION_STOP_ELIGIBLE = new Set([
  'moving',
  'waiting_for_arrival',
  'transition_awaiting_evidence',
  'progress_uncertain',
])

export function navigationRouteCanStop(
  route: NavigationRoute | undefined,
  stopSupported: boolean,
): boolean {
  return Boolean(
    route &&
    stopSupported &&
    NAVIGATION_STOP_ELIGIBLE.has(route.status) &&
    route.command_id &&
    route.route_sequence > 0,
  )
}

export async function submitNavigationStop(input: {
  characterID: string
  sessionID: string
  commandID: string
  routeSequence: number
}) {
  return await $fetch<{ command_id: string; state: string }>('/api/commands', {
    method: 'POST',
    body: {
      character_id: input.characterID,
      expected_session_id: input.sessionID,
      name: 'character.navigate.stop',
      args: {
        command_id: input.commandID,
        route_sequence: input.routeSequence,
      },
      idempotency_key: createIdempotencyKey(),
    },
  })
}
