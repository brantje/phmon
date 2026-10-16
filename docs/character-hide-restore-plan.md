# Character visibility and automatic return

Prepared 2026-09-28 on the isolated `codex/character-hide-restore` worktree.
This is a plan only; no character data, code, or deployment has been changed for
this feature. Implement against the then-current `main` while preserving the
work in the original checkout. The current `main` has agent protocol v6 and
migrations through `000008_character_portraits.sql`; Slice 6's separate branch
already proposes migration 000009. Use `000010_character_visibility.sql` for this
feature and reconcile any intervening migrations before implementation.

## Operator contract

- Put **Delete character** in `/characters/{id}` → Actions. This removes the
  character from PhMon's visible monitoring, not from the game or database.
  Allow the action while the character is online or offline. Require the
  authenticated operator to type the exact displayed character name. The
  confirmation identifies the selected server and explains that monitoring and
  already submitted commands may continue while the character is hidden.
- A hidden character is absent from Dashboard counters/cards, Stats and group
  members, search/selection lists, history, character detail and direct
  character-scoped APIs. If the last visible character on a server disappears,
  the server selector may fall back to All servers. Global records without a
  character association remain visible. Group records themselves remain saved.
- Retain the same character UUID, server/name identity, group memberships,
  sessions, commands, events, resource observations and items. Keep accepting
  authenticated observations while hidden; do not erase or rewrite history.
  Reject new operator commands for the hidden character. Existing submitted
  commands follow their current lifecycle.
- Restore visibility only when phBot reports a **new game join** for that exact
  server/name identity. Backend reconnect, browser reconnect, backend restart,
  plugin reload into an already-joined game, and repeated identification of the
  same game session must leave it hidden. After restoration, show the same UUID
  and retained history; a fresh character snapshot supplies current live state.

## Implementation path

1. **Durable state and concurrency.** Add `characters.hidden_at` and
   `characters.last_game_join_id` as nullable fields in migration 000010. Existing
   rows default visible. Extend character resolution to accept a verified
   game-join UUID from the plugin. Serialize hide and resolve on the existing
   server/name advisory lock. On identify, update the last join ID and clear
   `hidden_at` only when a nonempty join ID came from a newly observed
   `joined_game()` callback and differs from the last recorded ID. A null or
   repeated join ID never restores visibility. Keep session claiming and state
   ingestion independent of visibility.
2. **Plugin and agent protocol.** Advance the agent protocol from v6 to v7 and
   add an optional `game_join_id` to `character.identify`. Generate a UUID when
   `joined_game()` fires, carry that UUID through samples and socket reconnects,
   and replace it only on a later callback. If the plugin starts while a game
   character is already joined and missed that callback, identify without a
   join ID. Do not treat the existing agent-scoped `session.joined_game` event
   or a transport reconnect as sufficient proof of a character's new login.
   Accept older protocol versions for existing monitoring, but they cannot
   restore hidden characters; show this compatibility requirement in the
   confirmation and installation guidance.
3. **Operator API.** Add authenticated, origin-checked
   `DELETE /api/characters/{id}` with a bounded JSON body
   `{ "server": "<selected server>", "confirmation": "<exact character name>" }`.
   Require exact name confirmation and a case-insensitive exact server match.
   Return 204 on success or an identical repeated request, 400 for invalid
   input/confirmation, 404 for unknown ID or wrong server, and 503 when storage
   is unavailable. Add the same-origin Nuxt proxy. Publish a live-data
   invalidation after the transaction commits and log the operator action
   without credentials.
4. **Visibility at read and command boundaries.** Apply `hidden_at IS NULL` in
   character list/detail/group queries and character-backed event/history
   queries, including totals and pagination. Gate direct character resources,
   command history/control subscriptions and command admission on visibility;
   the latter must recheck inside its transaction to cover a hide/submit race.
   Hide guild-storage observations attributed only to a hidden character.
   Keep the same rule for chat, item search, analytics, map, automations and
   other character-backed views as those areas land or when rebasing onto a
   branch that already contains them. Do not filter agent-level health or
   unrelated global events.
5. **UI and documentation.** Place a compact danger action below the existing
   remote commands in the Actions tab. Use an accessible typed-name dialog
   showing server, retention and online behavior; disable submit until the
   name matches. On success, navigate to the overview after the live snapshot
   removes the character. Handle stale connection, server mismatch and API
   errors without pretending the action succeeded. Update the protocol guide,
   plugin guidance, record-management ledger and parity evidence.

## Acceptance checks

- PostgreSQL integration tests prove hide/restore preserves UUID, all dependent
  row counts and group memberships; other servers and same-named characters
  remain unaffected. Test online hide, offline hide, repeated DELETE, bad
  confirmation, authorization, scope and hide/identify/command races.
- Plugin unit and deterministic protocol simulator tests distinguish a real
  `joined_game()` callback from backend reconnect and plugin reload. Replay
  identification, snapshots and events while hidden, then verify one new join
  restores exactly once without duplicate historical rows. Record the actual
  Windows/phBot runtime check as a separate gate; simulator evidence does not
  close it.
- HTTP and live-stream tests verify lists, detail, group members, events,
  counts, resources, guild storage, controls and commands are hidden consistently
  and return after the new join. Check current and legacy plugin versions.
- Browser-check Actions confirmation, direct URL behavior and live removal/
  restoration at 1440×1000, 1280×800 and 390×844. Run focused Go/Python/Node
  checks and production builds. Do not operate a real character merely to test
  this feature.

## Assumptions and limits

The operator chose **hidden everywhere**, **restore on game login**, and **allow
deletion while online**. "Delete" is therefore a reversible visibility change;
there is no manual restore screen or permanent purge in this increment. A plugin
loaded after `joined_game()` already fired cannot prove a new login and leaves a
hidden character hidden until the next observed callback. A protocol-v7 plugin is
required for automatic restoration; older agents continue monitoring visible
characters and may keep writing hidden-character data.
