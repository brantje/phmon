# Slice 6 — Chat implementation plan

Prepared 2026-09-28 on `codex/slice-6-chat-plan` from `main` at `dd9219f`.
This is a planning deliverable. No Slice 6 feature code or real-character chat send
is authorized by this planning request. Implement Slice 6 in the order below when
an implementation run is requested; keep the canonical requirements in `AGENTS.md`.

## Starting point and dependency gates

| Existing contract | Slice 6 use or required change |
| --- | --- |
| `plugin/PhMon.py` `handle_chat` | Protocol v6 already spools `chat.message_received` through the Slice 5 event batch. It records bounded text/raw type but deliberately sets `channel: "unknown"`. Preserve that single inbound transport. |
| `activity_events`, `server/internal/events` | Durable, idempotent event IDs, session sequence, scope and ordered query exist. Extend chat validation only after verifying channel meanings. Chat history must be derived from these rows. |
| `server/internal/commands` and `/api/commands` | Authenticated, idempotent, session-fenced, audited command lifecycle exists. Add a typed chat command and per-channel capability modes here. |
| `web/app/composables/useLiveData.ts` and Go `LiveHub` | One browser WebSocket already manages subscriptions, revisions and stale state. Add chat invalidation/snapshots to it. |
| `web/app/components/AppSidebar.vue` | Chat is currently a label without a route. Add `/chat` and keep the existing shell. |
| Settings/notifications | No working Settings page or general notification dispatcher exists yet. Add the Slice 6 message preferences and browser/sound delivery foundation, with later event types, WAV management and Discord delivery remaining in their assigned slices. |

Slice 5 is **in progress**. Its PostgreSQL migration/integration and full protocol
simulator checks skipped locally for lack of a disposable database; the 390×844
comparison and actual phBot callback validation are also open. Before declaring
Slice 6 complete, run the Slice 5 migration and replay tests on PostgreSQL and
exercise an inbound chat event through the same plugin→Go→PostgreSQL path. Record
the real phBot runtime gate separately from simulator evidence. Slice 4 has its own
open Party Setup/item gates; do not mislabel them as Slice 6 completion.

## 0. Freeze source and reference evidence

1. Reopen the public demo in a real browser. Inspect all six tabs, sender selector,
   private contact/new-chat flow, recipient, composer, item/emoji buttons,
   easy/advanced mode and narrow layout. Ignore only the recurring connection
   error overlay. Capture reference/local comparisons at 1440×1000, 1280×800 and
   390×844; record inaccessible behavior rather than inferring it from an empty demo.
   The checked-in `docs/reference/phmonitor-chat.png` and
   `docs/reference/phmonitor-chat-mobile.png` already establish the tab strip,
   sender row, private contact column, conversation pane and bottom composer.
2. Verify the installed phBot version, optional `phBotChat` module and each callable
   on the supported runtime. The official [Chat API](https://plugins.phbot.org/chat-api)
   documents `All`, `Party`, `Guild`, `Union`, `Stall`, `Private`, `Note` and `Global`,
   but does not state message length, encoding limits or per-channel availability.
   `Notice`/`ClientNotice` are GM functions and are outside ordinary operator chat.
   The [Events API](https://plugins.phbot.org/phbot-api/events) documents
   `handle_chat(t, player, msg)` but no type-to-channel mapping; `player` may be
   `None` outside private messages. Record observed raw types, locale, direction
   and API outcomes in `docs/phbot-capabilities.md` before enabling a mapping.
   Static inspection of the authorized local phMonitor executable may inform the
   investigation, but runtime/source evidence must substantiate PhMon's mapping.
   The requested `plugin/phManager.py` is absent from this checkout and was not
   found in the operator's Documents/Downloads folders. The existing untracked
   `plugin/phMonitorAdapter.py` was read as a local example, without modifying or
   versioning it. It suggests numeric types `1/2/4/5/6/10/11` for
   all/private/party/guild/global/global/union and uses text-prefix heuristics,
   an 89-character split, recent-send suppression and a `0xB025` send ACK for
   private/party. These are investigation leads, not verified PhMon constants or
   packet semantics; test them on the recorded runtime before adopting any part.
3. Freeze a versioned raw-type→channel mapping only for verified game/phBot profiles.
   Preserve unknown raw types and original text. If the runtime cannot be observed,
   ship an explicitly labelled Unknown history lane and keep the six named tabs
   read-only/empty for unverified inbound types; do not claim channel parity.
   Determine the outbound text and recipient bounds per verified encoding/profile;
   set a conservative application cap and validate it in Go and Python.

## 1. Inbound normalization and durable read model

1. Extend the Slice 5 chat payload with verified canonical channel, sender/recipient
   semantics, direction and optional resolved item reference. Keep raw type and
   original message unchanged. Version the payload/mapping so older `unknown`
   events remain interpretable; backfill only types whose meaning is proven for
   their recorded profile. Never parse arbitrary chat text into an item or offer.
2. Add migration `000008_chat.sql` for a projection keyed by canonical `event_id`
   for inbound rows and `command_id` for outbound rows, with exactly one origin per
   row. Keep first-class server, character, channel, normalized private peer,
   occurred time and stable sort ID, plus appropriate indexes. The authoritative
   inbound record remains `activity_events`; outbound command/result records remain
   authoritative for send outcome. Build the projection transactionally from event
   ingestion/command transitions and provide an idempotent backfill/rebuild task.
   Do not let projection retention silently diverge from its source records.
3. Define private conversation identity as server + canonical peer name, with
   character scope on each message. Preserve display casing; normalize comparison
   consistently without merging contacts across servers. The selected character
   filters history, while an explicit all-character view may show the same peer
   across characters as the reference describes. No contact is inferred from an
   unclassified message.
4. Add bounded `/api/chat/contacts` and `/api/chat/messages` reads with stable
   bidirectional cursors on `(occurred_at, message_id)`, plus a durable per-operator
   read cursor for each scoped conversation/channel. New late/replayed messages
   sort by occurrence and ID; unread counts reflect the read cursor and survive
   reload. Index/count queries must remain bounded. Add a `chat` live subscription
   to the existing hub for current contact/message refresh, with revision fencing
   and stale/recovered states. Scope every query and invalidation by server and
   character so changing the sidebar scope cannot leak another conversation.

## 2. Outbound command and echo lifecycle

1. Add one `chat.send` command with exact arguments `{channel, text, recipient?}`.
   Allow `general`, `private`, `party`, `guild`, `union` and `global` only when the
   installed runtime reports that specific mode supported. Require a recipient
   only for private, and reject unknown fields, invalid Unicode/encoding, empty or
   over-limit text and stale/wrong character sessions. Require explicit confirmation
   for every global send because it may consume an in-game item or other resource;
   display the consequence without inventing a price. Keep Stall/Note as additional
   verified modes only if their UI semantics are established.
2. Import `phBotChat` optionally, advertise per-mode capability, and dispatch its
   documented method on the phBot callback thread through the existing command
   executor. A `True` return means the phBot API accepted the send; it does not
   prove remote delivery or receipt. Surface queued, sent, rejected, failed,
   expired and unknown outcomes from the audited command record. Never expose
   arbitrary Python, shell or packet injection.
3. Project each outbound command into history with its command ID and state.
   Correlate a later callback echo only when server, character/session, channel,
   recipient, exact text and a bounded time window identify one unmatched send.
   Link the event ID to that command and render one message. Keep both source rows
   intact. If multiple sends could match, leave the echo uncorrelated rather than
   assert a false identity. Test quick repeated identical sends and reconnect.
   The local example's automatic multipart send and text-based duplicate
   suppression can hide real messages or leave a partial send. Start with one
   verified-size message per command; add chunking or packet-ACK correlation only
   after captured versioned fixtures establish their behavior and an explicit
   partial-send result model exists.

## 3. Chat UI and preferences

1. Build `web/app/pages/chat.vue` inside the existing shell: six compact channel
   tabs, count/unread badges, sender selector, two-column private contact/history
   layout, New chat and recipient entry, history paging, jump to latest, and a
   bottom composer with capability and send-state feedback. Match the reference's
   dark panels, compact spacing and colored tabs. On mobile, navigate between
   contact list and conversation without page-level horizontal overflow; keep the
   composer reachable with the keyboard and touch.
2. Preserve visible messages while the connection is stale and mark them as last
   known. Distinguish loading, no selected sender, no contacts, no messages,
   unsupported channel, send error and recovered states. Read-only inbound
   channels must not show an enabled composer. A sender choice never silently
   changes server scope or command target.
3. Store chat sound/browser notification preferences under the operator identity
   and expose them in Settings. Ask browser permission only from a user action.
   Subscribe to canonical chat events through a shared notification dispatcher,
   dedupe by event ID and respect active conversation/read state. Use a bundled
   local sound initially; WAV upload/assignment and Discord delivery can extend
   this dispatcher in Slices 10/15. Unicode emoji entry is allowed only within
   verified encoding/length constraints. Render canonical item references only
   when the source resolves them reliably and reuse the shared item detail UI.

## 4. Verification and acceptance

- Focused Python tests: callback payload mapping/unknown fallback, bounds,
  optional import, channel method dispatch, callback-thread execution, stale
  session rejection and global confirmation.
- PostgreSQL/Go tests: migration/backfill/rebuild, event replay idempotency,
  contact scope, private identity, both cursor directions, late ordering,
  unread persistence, command admission/capability/confirmation and echo
  correlation. Run them with `TEST_DATABASE_URL`; a skip is an open gate.
- Deterministic simulator flow: emit each verified channel through production
  protocol v6, replay a batch after disconnect, query after backend restart,
  send a fake-adapter outbound command through the production command path and
  return success/failure/echo variants. Label all messages fixture data.
- Browser flows: channel switching, sender/server scope, new private contact,
  history paging, unread/read, jump to latest, disabled read-only compose,
  send states, global confirmation, reconnect and keyboard navigation. Capture
  side-by-side evidence at all three required viewports and check whole-page width.
- Regression/build gates: `go test ./...`, `go vet ./...`, `go build ./...`,
  `python -m unittest plugin.test_phmon`, Python compile, `npm run test:unit`,
  `npm run typecheck`, `npm run lint`, `npm run format:check`, `npm run build`
  and container builds. Update `docs/protocol.md`, `docs/phbot-capabilities.md`,
  `docs/reference-parity.md`, `plugin/README.md` and the `AGENTS.md` ledger.

Slice 6 is complete only when verified inbound channels and supported sends work
end to end, each message has correct server/character/channel/peer identity,
reconnects do not duplicate it, history/read state survive restart, echo handling
is deterministic, unsupported capabilities are honestly read-only, and the chat
layout passes desktop/mobile comparison. A simulator pass does not close the
separate real phBot integration gate.
