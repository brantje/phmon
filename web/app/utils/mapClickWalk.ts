import type { CharacterView } from '../../shared/types/live'
import type { MapProfile } from '../../shared/types/map'
import type { MapNavigationIntent } from './mapNavigationAction'
import { resolveMapPointForCharacter } from './mapNavigationAction.ts'

/** Experimental direct movement: launch every POST before awaiting any result. */
export function sendMapClickWalk(options: {
  intent: MapNavigationIntent
  profile: MapProfile
  characters: CharacterView[]
  key(): string
  post(request: Record<string, unknown>): Promise<unknown>
  failed(characterName: string, error: unknown): void
}) {
  for (const id of options.intent.targetIDs) {
    const character = options.characters.find((row) => row.character_id === id)
    if (!character) continue
    const { destination, reason } = resolveMapPointForCharacter(
      options.intent,
      options.profile,
      character,
    )
    if (!destination) {
      options.failed(
        character.name,
        new Error(reason?.message || 'Unknown coordinates'),
      )
      continue
    }
    const { x, y, z } = destination
    void options
      .post({
        character_id: id,
        expected_session_id: character.session_id || '',
        name: 'character.move_to',
        args: { x, y, z },
        confirmation: false,
        idempotency_key: options.key(),
      })
      .catch((error: unknown) => options.failed(character.name, error))
  }
}
