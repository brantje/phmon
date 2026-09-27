# Slice 3 — Remote commands: implementation plan for GPT-6 Luna

Prepared 2026-09-27; baseline repository check was code head `9ce0828`, including the
Nuxt pages/components refactor and supplied screenshots. Implementation is in
progress on `codex/slice-3-plan`; this plan remains the sequential P0–P7 contract.
No real-character actions have been performed. “Stage 3” means
**Slice 3 — Remote commands** in `AGENTS.md`.

## 1. Execution scope

Implement Slice 3 in small, sequential increments using this plan and the canonical
guide. Stop after Slice 3 and its validation; do not continue to Slice 4–15 under
this scoped handoff. Historical “do not start Slice 3” notes describe earlier
Slice 2/2.5 tasks. This planning request does not itself authorize an implementation
run; use the implementation prompt at the end when starting that work.

Deliver a real, authenticated, audited path:

```text
Operator selects a character and action
  -> same-origin HTTP command submission
  -> Go validates authorization, arguments, capability and current session
  -> PostgreSQL records intent
  -> exact owning agent socket receives command
  -> phBot callback dispatcher validates and invokes allowlisted API
  -> acknowledgement/result returns through that authenticated socket
  -> Go persists lifecycle/evidence
  -> existing /api/live WebSocket updates UI and command history
```

Required controls: Start/Stop Training, Set Training Area, Set Training Radius,
Walk, Return Scroll, Disconnect, and Go Clientless where safely supported. Also
include Start/Stop Trace: the supplied Actions screenshot explicitly shows these,
and the official API documents them. Keep Start Training/Start Bot as one canonical
command, likewise Stop Training/Stop Bot.

Party Setup belongs to Slice 4. Script discovery/management/execution and Quest
workflows remain assigned to later tool-parity work in the matrix/Slice 15. Retain
their navigation/slot placement with an honest later-slice explanation; do not add
raw script text, Python, shell commands, filesystem paths or packet injection to
this command API. A disabled **required Slice 3** control is an unresolved gate,
not completion.

## 2. Starting point and constraints verified in the repository

| Existing code | Reuse / required extension |
| --- | --- |
| `server/internal/characters/store.go` | Stable server/name identity; durable `character_sessions.session_id`; per-character advisory locks and agent/generation fencing. Reuse these fences. |
| `server/internal/agents/registry.go` | Multiple sockets per agent are intentional. Registry currently tracks presence, not outgoing command delivery. Never replace it with one socket per agent. |
| `server/internal/httpapi/agent.go` | Agent protocol v2; authenticated hello; character registration/state/leave; no commands. Add bounded outgoing dispatch and result handling. |
| `server/internal/database/migrations/000001`–`000003` | Existing identities, sessions and groups. Add the next migration; do not modify applied migrations. |
| `plugin/PhMon.py` | One networking worker, callback sampling, one-entry coalescing state queue. Commands/results need separate non-coalescing queues. |
| `AgentWorker._publish_sample()` | Currently performs a synchronous receive expecting `character.registered`. Refactor before commands can interleave with registration replies. |
| `server/internal/httpapi/live.go` | Shared bounded browser subscriptions, revisions, invalidation and recovery. Extend this; no new polling transport. |
| `web/app/composables/useLiveData.ts`, `web/shared/types/live.ts` | One browser socket, stale data and revision fencing. Add typed control/capability/history snapshots here. |
| `web/app/pages/stats.vue`, `pages/characters/[id].vue` | Existing file-based Stats/detail routes. Add focused action/history components here and to `components/CharacterPanel.vue`. |
| `web/app/app.vue`, `layouts/default.vue`, `components/AppSidebar.vue` | Provider/page outlet and persistent shell already extracted. Keep feature state out of the outlet/layout; add Client at `pages/phbot/client.vue`. |
| `web/server/api/live.ts` | Relay currently forwards no browser credentials. Authentication below requires an explicit, narrow amendment, not accidental forwarding of all cookies/headers. |
| `scripts/agent_simulator.py` | Imports the production Python worker. Extend with fake API adapters and a simulated callback loop; do not write a second command protocol. |
| `scripts/live_transport_audit.py` | Currently has Slice 2-specific route checks. Extend its mutation allowlist and command-read prohibitions without weakening existing coverage. |

Human/operator authentication does not exist yet. Trusted-network access and an
agent bearer token are not operator authorization. Implement minimum control-plane
authentication before exposing mutations; do not postpone it to Slice 14.

Slice 2 and 2.5 still have recorded visual/runtime/map gaps. Carry them forward
honestly. Numeric, current-region command controls do not require a finished map
renderer or resolved minimap transforms. Do not reopen the exporter or build a map
as a dependency of commands. Real phBot 20.1.1 connectivity is already evidenced;
that does not verify mutation APIs or embedded Python compatibility.

## 3. Layout evidence: use the supplied screenshots

These files were visually inspected while preparing this plan. They are layout
references, never application assets:

| Reference | What it establishes for this slice |
| --- | --- |
| [`02-stats-05.png`](../phmonitor_screenshots/02-stats-05.png) | Character card **Actions** tab; compact two-column button grid; order: Start/Stop Training, Start/Stop Trace, Training Area/Radius, Return Scroll/Walk, Disconnect/Execute Script. |
| [`02-stats-02.png`](../phmonitor_screenshots/02-stats-02.png) | Character name/server/status header and compact tab strip around the Actions surface. |
| [`03-phbot-tools.png`](../phmonitor_screenshots/03-phbot-tools.png) | phBot → Client navigation, `phBot | Client` title, wide introductory panel, narrow local section menu, main Go Clientless panel and inset result/status area. |
| [`02-stats-01.png`](../phmonitor_screenshots/02-stats-01.png) | Group context around character controls. Group selection must not silently become a bulk command target. |
| [`02-stats-03.png`](../phmonitor_screenshots/02-stats-03.png), [`02-stats-04.png`](../phmonitor_screenshots/02-stats-04.png) | Consistent card/tab sizing; equipment and analytics content belongs to later work. |

Original supplied captures are **2560 × 1315**. Existing `docs/reference/` desktop
captures are 1440 × 1000. Compare like-sized captures rather than measuring a
resized preview as CSS pixels. Keep the guide's 228 px navigation/34 px top strip,
compact density, cream headings and dark translucent panels.

Before implementing these surfaces, open the public demo in a real browser and
inspect available action dialogs, easy/advanced visibility and Client navigation.
Dismiss/hide only the recurring connection-error overlay. Record inaccessible
dialogs as inaccessible; the screenshots prove button placement, not hidden form
semantics. Do not inspect private protocols or bypass paid access.

The Client screenshot describes terminating all `sro_client.exe` programs. Adapt
that behavior to an explicit selected character and a verified per-instance phBot
primitive. **Never implement a machine-wide process kill, local broker, arbitrary
Windows command or backend access to the operator's machine.** Do not copy the
promotional column, Premium decorations, streaming controls or phMonitor identity.

Use one reusable action component in the Stats character surface and stable
`/characters/{character_id}` details. Preserve the observed order and two-column
density; do not turn actions into a large generic settings form. Add `/phbot/client`
through `web/app/pages/phbot/client.vue` with an explicit character selector. Preserve
later section labels without suggesting their workflows are implemented.

## 4. API evidence and command catalog

Official documentation was checked on 2026-09-27. Recheck it and the installed
runtime during implementation. URLs establish documented behavior, not availability
on every phBot build. Check optional functions independently; one missing import
must not disable the existing monitoring plugin.

| Canonical command | Typed arguments | Documented primitive / result interpretation |
| --- | --- | --- |
| `bot.start` | `{}` | `start_bot()` returns bool. Reference label: Start Training. |
| `bot.stop` | `{}` | `stop_bot()` returns bool. Reference label: Stop Training. |
| `trace.start` | `{name: string}` | `start_trace(name)` returns bool. Bound/trim player name; no name-to-agent routing. |
| `trace.stop` | `{}` | `stop_trace()` returns bool. |
| `training.area.set` | Discriminated modes below | `set_training_area(name)` selects a named area; `set_training_position(region,x,y,z)` changes the active area's location. These are distinct operations. |
| `training.radius.set` | `{radius: number}` | `set_training_radius(radius)` returns bool; verify readback with `get_training_area()`. |
| `character.walk` | `{region: integer,x: number,y: number,z: number}` destination | Require `generate_path(x,y)`, step at most 256 returned waypoints via `move_to_region` from callback ticks, and use `get_position()` for waypoint/destination readback. Same-region only; no teleport; reject invalid/cross-region paths. |
| `character.return` | `{}` | `use_return_scroll()` returns bool; consuming/starting the scroll does not prove teleport completion. |
| `character.disconnect` | `{}` | `disconnect()` returns no value and leaves relog settings unchanged. Do not change auto-relog configuration. |
| `client.clientless` | `{}` | No safe mutation was located in the checked public Client/Misc/API index or targeted documentation search. Investigate official versioned/runtime evidence; otherwise report unsupported with the precise blocker. |

Sources: [Botting](https://plugins.phbot.org/phbot-api/botting),
[Training Area](https://plugins.phbot.org/phbot-api/training-area),
[Movement](https://plugins.phbot.org/phbot-api/movement),
[Inventory](https://plugins.phbot.org/phbot-api/inventory),
[Misc](https://plugins.phbot.org/phbot-api/misc),
[Client](https://plugins.phbot.org/phbot-api/client),
[Events](https://plugins.phbot.org/phbot-api/events),
[API index](https://plugins.phbot.org/phbot-api).

Training argument design, to freeze after inspecting the reference dialog:

- `mode: "current_position"`: read fresh position on the phBot callback immediately
  before the call. The browser supplies no cached coordinates. Record the resolved
  region/x/y/z in the result.
- `mode: "position"`: explicit region/x/y/z; validate again against live runtime
  context. Keep this form separate from choosing an existing named training area.
- `mode: "named"`: exact bounded name passed to the documented name selector. Expose
  this when reference/runtime inspection establishes the intended workflow. Do not
  invent an API that lists names or treat a script filepath as an area name.
- Document which modes were implemented and why. Missing required dialog behavior
  remains a gap. Do not silently interpret “Set Training Area” as another operation.

Training position/radius require an active area. Read it before and after setting;
send only useful numeric settings and availability, not the local script `path`.
Official docs allow automatic region derivation with region zero only outdoors.
Use an explicit observed region; do not enable zero-region guessing for caves.

Reject unknown fields, wrong types (including Python bool as numeric), NaN/Infinity,
oversized names and missing coordinates. Define shared documented application safety
bounds in the command catalog, then intersect them with verified runtime limits.
The docs do not specify numeric radius limits: do not invent a “phBot maximum.”
Phase 0 must settle and record bounds before exposing those controls. Validate
boundary fixtures in both Go and Python. Initially restrict walking to the currently
observed region; cross-region requests must fail explicitly. Special-area coordinates
require verified runtime semantics, not the unresolved tile-to-world transform.

Separate **API outcome** from **observed state**. A bool success can complete the
documented API operation. Void-return calls can report “API call returned; arrival/
disconnect not yet verified,” not “reached destination.” Use result evidence such as
`api_return`, `effective_args`, `observed_after`, and `verification` (`api_confirmed`,
`observed`, `unverified`). Never set `botting=true/false` from the last command:
there is still no documented read-only botting getter in the checked sources.

## 5. Freeze these architecture decisions before coding transport

### 5.1 Minimal operator authentication

Use one self-hosted operator identity in this slice; no accounts/RBAC framework.

- Configure a high-entropy operator access secret in Go's private environment; do
  not use an agent token or a built-in/default production credential. Missing auth
  configuration must leave controls disabled/fail closed, never anonymous.
- Go owns `POST /api/auth/login`, `/logout`, and `GET /api/auth/session`. Successful
  login creates a cryptographically random opaque session, stored hashed in a
  bounded in-memory store with an absolute expiry (proposed: 8 hours). Restart
  requires login again. Compare access secrets safely and rate-limit login attempts.
- Use an HttpOnly, SameSite=Strict, path=/ cookie; Secure is mandatory under HTTPS.
  Reject plain HTTP origins by default. Permit them only with the explicit
  `OPERATOR_ALLOW_INSECURE_HTTP` opt-in and an exact configured origin; this can
  support trusted LAN deployments without TLS but sends session cookies unencrypted.
  Never put credentials in localStorage, URLs, live payloads or logs.
- Protect command submission/history, credential creation, existing mutations and
  browser monitoring API access consistently. Keep `/agent` on its separate
  agent-token authentication and health endpoints on their operational boundary.
- Nitro forwards **only the named PhMon session cookie** to private Go routes and
  the upstream WebSocket; no arbitrary browser authorization/cookies. If the
  installed Node WebSocket API cannot set upstream headers, use the existing
  compatible server-side WebSocket facility or a narrowly scoped pinned adapter;
  do not put the cookie into the URL or weaken backend authentication.
- Validate browser Origin against configured instance origin(s), including local
  dev ports, for cookie-authenticated mutations and WebSocket upgrades. Do not
  trust arbitrary forwarded-host headers. Explicitly document TLS/proxy behavior.
- Revoke active authenticated browser sockets on logout/expiry; stop reconnect loops
  on auth errors and present sign-in. Database state remains unchanged by logout.

This deliberately changes the old relay's “no credentials upstream” rule to a
documented named-cookie exception. Update `docs/protocol.md`, setup, Compose and
tests together. Live monitoring still never uses HTTP bootstrap/polling. Session
status/auth actions are control-plane HTTP, not live monitoring reads.

### 5.2 Targeting and capability ownership

Every command targets one stable `character_id` plus the expected durable
`session_id` shown in the live UI. Go resolves authenticated `agent_id` and exact
`connection_generation` itself. Never accept a client-supplied socket or trust an
agent ID as sufficient routing authority.

Expose the existing session UUID in the current-character control view and in v3
`character.registered`. Bind it to the plugin's fresh server/name identity and
local profile/connection epoch. A generation counter alone can repeat after Go
restart; a durable session UUID prevents that ABA mistake.

Capabilities belong to the exact runtime connection and current character session.
Different phBot processes using one token can have different versions/functions.
Publish a bounded allowlisted capability report with command/schema version,
supported flag, reason code and applicable bounds/modes. Agent views may show a
per-connection summary, but must not union sibling capabilities to authorize a
character. Missing/old/stale reports mean unsupported, not optimistic support.

Go independently knows its catalog. Intersect it with the owning runtime report
and current state; do not allow a plugin to advertise a new arbitrary command.
Plugin repeats capability, identity, profile, session and expiry checks immediately
before calling phBot. Clear pending execution on leave, switch, rejection, worker
replacement, socket loss or plugin unload.

Use the existing per-character lock order for command admission and session
replacement. Revalidate ownership immediately before dispatch. Deliver revocation
for a superseded session and discard queued plugin work when received. An already
invoked remote effect cannot be retracted: document this in-flight boundary and
never claim cross-machine atomicity or exactly-once execution across crashes.

### 5.3 Protocol evolution

Introduce agent protocol **v3** for commands/session capabilities. Continue accepting
v2 monitoring agents with controls explicitly disabled as `plugin_upgrade_required`.
Respond using the negotiated version; test v2 monitoring and v3 monitoring/control.
Do not send new command frame types to a v2 worker, which currently disconnects on
unexpected messages. Browser protocol v1 can gain optional fields/new stream types;
keep existing message/revision semantics unchanged.

Proposed v3 frames (final field names must be documented before implementation):

```json
{"type":"command.execute","protocol_version":3,"command_id":"cmd_<uuid>","character_id":"<uuid>","session_id":"<uuid>","name":"training.radius.set","args":{"radius":50},"expires_at":"<UTC RFC3339>","ttl_ms":10000}
{"type":"command.ack","protocol_version":3,"command_id":"cmd_<uuid>","character_id":"<uuid>","session_id":"<uuid>"}
{"type":"command.result","protocol_version":3,"command_id":"cmd_<uuid>","character_id":"<uuid>","session_id":"<uuid>","status":"completed","verification":"observed","effective_args":{"radius":50},"observed_after":{"radius":50}}
```

Also define bounded `agent.capabilities` and character-scoped control-state reports
for current training settings/availability. Capability reports may be agent-scoped;
every character control-state/ack/result/revocation includes explicit character and
session IDs. Keep frames within the existing 8 KiB limit.

Refactor Python to one application-frame receive/demultiplex path after hello.
Pending identification waits for its registered response while still processing
heartbeats, rejections and control frames. Bound registration wait time and associate
replies with the pending identity; never apply a late reply to a switched character.
All outgoing agent frames, including registration replies and command dispatch in
Go, use a single ordered writer per socket. Keep queues/deadlines bounded and never
hold the global registry lock during I/O.

### 5.4 Durable lifecycle and delivery policy

Required visible states: `queued`, `sent`, `acknowledged`, `completed`, `failed`,
`expired`. Add `dispatching` for the send/persist crash window and `unknown` for a
possibly executed action without a trustworthy final result. These extra states
avoid falsely reporting failure or success after transport loss.

| Transition / situation | Required behavior |
| --- | --- |
| Valid submission | Commit intent and initial audit event before making it eligible for dispatch. Return HTTP 202 and command ID; this is not execution success. |
| Dispatch begins | Atomically claim `queued -> dispatching` before writing. Only one dispatcher wins. |
| Socket write succeeds | Record `sent`/sent time. A socket write is not acknowledgement or completion. |
| Plugin accepts into bounded callback queue | Send `command.ack`; no phBot side effect yet. |
| Plugin completes the documented operation | Persist result/evidence, then invalidate browser subscriptions. |
| Known pre-execution rejection | `failed` with a bounded reason, such as target changed, unsupported, queue full or invalid arguments. |
| Definitely unexecuted command reaches deadline | `expired`; never dispatch/replay it. |
| Write/result delivery is ambiguous, backend restarts mid-dispatch, or execution deadline passes without evidence | `unknown`; show that execution may have occurred. Never auto-resend. |
| Backend restarts with queued work | Fail it as interrupted (or expire if overdue); do not carry manual actions into a new character session. |
| Duplicate ack/result | Idempotent; never repeat the side effect, regress state or append duplicate lifecycle events. |
| Result beats `sent` persistence | Preserve the more advanced state. Persist write metadata without overwriting a terminal result. |

Manual commands require a current online session; there is no offline backlog.
Use a short server-owned execution TTL, proposed 10 seconds (test with injected
clocks). Expiry is the latest allowed **start** of an operation, not proof an
already-started movement stopped. Define a separate bounded result wait, proposed
30 seconds. No automatic retry of the action, including “safe-looking” start/stop.

Send server time during v3 handshake and a remaining TTL per command. Convert to
a monotonic callback deadline conservatively; account for network/clock uncertainty
and never reset TTL on duplicates. Test skew, delayed frames and wall-clock jumps.
If freshness cannot be established, reject rather than extend the execution window.

Use a browser-generated idempotency key retained for retries of the same user
submission. Persist `(operator, key)` plus a canonical request hash: same key/same
request returns the existing ID; changed target/action/args with that key is 409.
An explicit new user action gets a new key. Bound keys and retain deduplication for
the documented history period. Never automatically repeat a POST with a new key
after a timeout.

Do not assume results always arrive while a character is online: `disconnect()` can
cause `character.left` before its result. Validate results against the **original
command's** agent/socket/session and dispatch attempt, even if that session ended;
that permits audit recording, not permission to run new commands. Wrong sockets or
new sessions cannot finish old commands. Late authoritative evidence may resolve
`unknown` with a recorded reconciliation event; never rewrite an observed result
silently or change an expired/unexecuted command into success from an unrelated frame.

### 5.5 Persistence and bounds

Add `000004_commands.sql` if still the next migration at implementation time:

- `commands`: `cmd_` UUID ID, character/session/agent references, generation,
  operator identity, idempotency key/hash, command name/schema version, validated
  args, state, creation/expiry/dispatch/sent/ack/finish times, bounded result code,
  message, verification and effective/observed arguments.
- `command_events`: command ID, monotonically ordered event ID, server timestamp,
  lifecycle kind and small bounded evidence. Current state and event insert commit
  together. This is a focused audit log, not a general event-sourcing system.
- Unique idempotency constraint; indexes for character/time pagination and pending
  deadlines. No cascading deletion of command audit just because a session ends.
- Store last observed training/control data in a small session-scoped record or
  explicitly typed columns; keep capabilities tied to the live connection. Clear
  availability on session loss; retained settings are marked last known.

Concrete starting resource policy: one in-flight action per character; 32 pending
frames per agent socket; 16 callback commands and 32 outbound results per worker;
256 duplicate IDs retained through their maximum retry/result horizon. Do not
evict a still-live deduplication entry to accept more work—reject with backpressure.
Allow at most one command invocation per callback tick. Reserve result capacity
before accepting work; command/result queues must never use state coalescing.

Use configurable, tested admission limits (starting policy: 60 submissions/minute
per operator and 10/minute per character, with bounded limiter storage). Enforce
limits on Go, not just UI disabling. Audit records default to 90-day retention,
pruned in bounded batches; do not delete pending records or idempotency entries
inside their promised retry window. Document settings and test the cutoff.

Keep one Go process owning sockets/dispatch as the supported deployment for this
slice. Do not imply multi-replica safety or add a message broker. Inject clocks and
small transport interfaces only where needed for deterministic lifecycle tests.

## 6. Sequential work packages for Luna

Execute in order. Each package ends with a working checkpoint and focused evidence.
Update the progress ledger after each; continue without asking the operator to
choose routine implementation details. Do not mark all packages complete from
simulator success alone.

### P0 — Freeze evidence, semantics and acceptance fixtures

1. Read `AGENTS.md`, this plan, protocol/capability/parity docs and Git status;
   preserve unrelated changes and verify filenames/head have not moved.
2. Open the screenshot references above and the accessible public demo. Record
   exact action order and observed form semantics in `docs/reference-parity.md`.
3. Verify functions, phBot/embedded Python versions and supported optional imports
   from official docs/read-only runtime evidence. No mutation probes on real bots.
4. Record numerical bounds, training modes, result semantics, clientless blocker,
   auth/protocol decisions and exact command catalog in the three contract docs.
5. Define fixtures: two agents; two sockets sharing a token with different
   capabilities; distinct characters; one socket capable of multiplexing IDs in a
   backend fixture; stale session; area unavailable; API true/false/void/exception.

**Exit:** implementable schemas and a capability matrix with verified/unverified/
blocked distinctions. If one primitive is blocked, continue all independent work.

### P1 — Authenticate the operator boundary

Files: new `server/internal/auth/`, HTTP auth middleware/handlers, config and main
wiring; Nitro auth proxy/relay, a small login surface; `.env.example`, Compose,
README and relevant test helpers.

Implement section 5.1. Keep the existing agent authentication separate. Update
smoke/browser scripts to log in using generated test credentials without logging
secrets. Preserve production and dev WebSocket paths.

**Exit checks:** anonymous/cross-origin controls denied; direct-Go bypass denied;
agent token cannot act as operator; login/logout/expiry/restart work; authenticated
monitoring reconnects; no secret in browser storage, URLs, console or snapshots.

### P2 — Command domain, schema and HTTP admission

Files: `server/internal/commands/{types,catalog,store,service}.go` (or comparably
small layout), next migration, `httpapi/command.go`, router/main wiring.

Implement typed catalog validation, state transitions, idempotency, durable audit,
rate/queue limits, current-session resolution and expiry/recovery rules. Add
`POST /api/commands` with `character_id`, `expected_session_id`, `name`, `args`,
`idempotency_key` and an explicit confirmation field for disruptive operations.
Server selects agent/socket. Do not rely on confirmation as authorization.

Use 400 invalid payload, 401 unauthenticated, 403 forbidden, 404 unknown target,
409 stale session/conflicting idempotency/in-flight action, 422 unsupported
capability, 429 admission limit and 503 unavailable database/dispatch service.
Use stable machine-readable error codes plus sanitized messages. Do not expose
partially committed commands or dispatch anything after a failed database commit.

**Exit checks:** PostgreSQL tests cover migration on fresh/existing DB, concurrent
idempotency, competing dispatch claims, lifecycle ordering, expiry/restart,
character/session fences and bounded history queries. No agent writer needed yet.

### P3 — v3 transport and safe callback dispatch

Files: `httpapi/agent.go`, agent registry/connection writer, character registration,
`plugin/PhMon.py`, `plugin/test_phmon.py`, protocol tests.

Implement v2 compatibility, session IDs, capability reporting, one ordered writer
and Python frame demultiplexing. Add an explicit Python dispatch table of trusted
adapters. Worker validates/enqueues; `event_loop()` validates current runtime context
again and calls at most one action; worker sends acknowledgement/results. No network
I/O, waiting for game travel or arbitrary callable lookup on the callback thread.

First vertical increment: `bot.stop` against a fake phBot adapter through production
worker → Go → PostgreSQL. Then `bot.start`, trace, return and disconnect; finally
training area/radius and walk. Add clientless only after safe primitive verification.
Do not make unavailable optional APIs fail the entire `phBot` import block.

Trigger fresh callback sampling after an operation; publish control readback and
normal character state through existing paths. Do not call phBot getters on the
network thread. Disconnect callbacks can repeat; result handling must remain
deterministic. Queued tasks from an old profile/worker must never execute in a new one.

**Exit checks:** all adapters validate input and preserve API return meanings; no
calls on worker thread; duplicate command invokes adapter once; stale queued work
is discarded; registration/command interleaving and bounded shutdown work.

### P4 — Real delivery lifecycle and recovery

Files: command dispatcher/reconciler, registry, agent handler, main startup,
session reconciler and database integration tests.

Connect admission to the exact owning socket. Persist-before-dispatch, serialize
competing admissions, recheck ownership and deadlines, and consume authenticated
ack/results. Never broadcast to all sockets sharing an agent token. Add startup
and periodic reconciliation for pending deadlines, DB outages and lost sockets.

Test crashes before write, after write and before result commit; synchronous
results racing writer bookkeeping; a leaving character producing a valid result;
old results after a different session starts. Results can be buffered only within
bounded same-connection recovery; if proof is lost, preserve `unknown` and do not
replay. Keep polling/reconciliation internal to Go, not browser HTTP polling.

**Exit:** simulator end-to-end `bot.stop` completes with one durable audit trail;
failure/expiry/unknown cases are distinguishable and survive backend restart.

### P5 — Live capability, control state and command history

Files: `httpapi/live.go`, shared live types/composable, command query store,
source transport audit and live smoke tests.

Add character-scoped `commands` and `controls` streams to the existing socket.
Use bounded replacement snapshots, subscription revisions and deterministic
pagination (proposed page size 25, maximum 100; cursor by timestamp + command ID).
History filters: explicit character/server, command, status and time range. Validate
every filter and include it in frontend filter equality/revision handling.

The controls snapshot includes current session, capabilities/reasons, training
state and freshness. Command history includes requested/effective args, actor,
lifecycle timestamps, result/evidence and clear unknown state. Include the latest
command even when it finishes rapidly; do not rely on receiving every transient
live state to reconstruct the audit. Read full transition details from stored data.

Invalidate only after commits, following current outage/recovery rules. HTTP POST
returns an accepted ID; authoritative command and character state changes arrive
over `/api/live`. No HTTP/SSE live reads, even after mutation, filter or refresh.

**Exit:** two browsers see the same command/result; reconnect and DB recovery
restore history; switching character/filter rejects old revisions; bounds and slow
consumers preserve agent ingestion capacity.

### P6 — Reference-shaped controls and history UI

Files: focused components under `web/app/components/`, optional action composable,
`pages/stats.vue`, `pages/characters/[id].vue`, new `pages/phbot/client.vue`,
`components/CharacterPanel.vue`, `components/AppSidebar.vue`, scoped/shared CSS,
and Nitro `api/commands.post.ts`. Keep `app.vue` as the provider/layout/page outlet.

1. Build the reusable compact Actions grid from `02-stats-05.png`. Keep a visible
   character/server heading and immutable target for each open form.
2. Add target-aware forms for trace name, training mode/position/radius and walk.
   Show supported bounds and current observed settings; never silently fill unknown
   fields with zero or stale backend values. Reject stale form session on submit.
3. Confirm Disconnect, Return Scroll and Clientless with named character, server,
   action and relevant effects. Require a new confirmation if target/session changes.
4. Show queued/sending/sent/acknowledged separately from API success/observed result,
   failure, expiry and unknown. A disabled control has a readable capability/stale/
   offline reason. A previous command result never masquerades as live bot status.
5. Add Client layout from `03-phbot-tools.png`, explicit target selector and inset
   latest-result panel. Unsupported clientless is a documented missing capability.
6. Add a compact command-history section/drawer with filters, pagination and audit
   details. Reuse the same state in card, detail and Client surfaces.
7. Easy mode may simplify advanced inputs but never changes API authorization.
   Preserve keyboard focus, dialog cancellation, labels and narrow-screen reflow.
8. Use `NuxtLink` and file-based pages. Dispose page-specific subscriptions,
   watchers and timers on navigation while keeping the shell's one shared live
   connection. Verify direct links, back/forward and fast detail-to-detail changes.

**Exit:** operator can submit each supported action, inspect durable results and
recover from failure without ambiguity or stale-target submission. No app-wide
restyling, fake functional tabs or later inventory/map implementation.

### P7 — Regression, browser, production and runtime gates

Add `scripts/command_smoke.py` and a focused command browser flow, extending
`scripts/agent_simulator.py` with deterministic fake API behavior. Wire them into
the existing `validate`/`stack` jobs and authentication-aware live network audit.
All command integration runs use fixtures/test instances, never real bot characters.

Run the focused tests after each package and the existing full checks once the
increment is integrated. Use `scripts/check.sh` in the supported Bash/CI environment;
on Windows run its equivalent commands with available tools and record any missing
environment gate precisely:

```text
cd server
go vet ./...
go test -race ./...
go build ./cmd/server
go build ./cmd/phmonctl
cd ..
python -m unittest discover -s plugin -p 'test_*.py'
python scripts/live_transport_audit.py
npm --prefix web run format:check
npm --prefix web run lint
npm --prefix web run typecheck
npm --prefix web run build
docker compose --env-file .env.example config -q
```

PostgreSQL tests require `TEST_DATABASE_URL` pointing to a disposable test database.
Do not count skipped database tests as passes. Retain existing agent/character/live,
outage/recovery and production/dev relay smoke scenarios when adapting CI for auth.
Run full container builds and the new command smoke/browser scripts against a local
test stack; report actual commands/results, not a copied historical CI success.

Capture Actions, Client, confirmation, pending, failed and unsupported states at
1440 × 1000, 1280 × 800 and 390 × 844. Also capture 2560 × 1315 for direct supplied
reference comparison. Check no page overflow, usable touch/focus, correct button
order, density, title/panel proportions and local-only assets. Save local evidence
under `docs/evidence/` and link comparisons in `docs/reference-parity.md`.

Real runtime validation is a separate gate. Record phBot/plugin/Python versions,
each supported adapter, effective settings and independently observed effects.
Prepare an explicit named test-character checklist for the operator; only operate
real characters when authorized. Basic connectivity evidence is already present;
do not describe connectivity as absent or simulator mutation tests as real runtime.

## 7. Mandatory behavior test matrix

| Boundary | Required scenarios |
| --- | --- |
| Authorization | Anonymous, expired login, cross-origin, direct Go access, forged actor, agent token used as operator, unauthorized credential minting, logout with open live socket. |
| Target routing | Agent A/B; sibling sockets sharing one token; different capabilities per sibling; multiple explicit IDs on a fixture socket; target transfers between enqueue and dispatch; local character/profile switch before callback; same generation number after backend restart. |
| Input | Every command schema; unknown command/field; missing ID/session; bool/string/null instead of number; NaN/Infinity; oversized name/body; wrong server/region; unavailable active training area. |
| Delivery | Success; API false/exception/void; duplicate submit/ack/result; lost HTTP response; result before sent bookkeeping; queue full; writer failure; no automatic action retry. |
| Time | Expired before send, expired in callback queue, transit delay/skew, duplicate TTL not refreshed, operation started before expiry but result later, uncertain result timeout. |
| Recovery | Backend restart at each lifecycle phase; DB outage before intent/after send/during result; plugin reload/socket loss; no stale action replay; old-session result accepted only for its original audit. |
| Truthfulness | Bool API success versus observed setting; walk reports observed arrival only from callback-time position readback after bounded same-region waypoints; disconnect does not promise relog disabled; return does not promise teleport finished; botting remains unknown without a getter. |
| Live/UI | POST 202 is not success; cross-client updates; stale controls disabled; old dialog/filter revisions ignored; refresh/reconnect/history through WS only; auth expiry shown; keyboard/mobile forms. |
| Resource limits | Bounded command/result queues and dedup cache; rate limits; history pagination/retention; oversized frames; slow browser and agent writers; shutdown leaves no leaked workers. |

## 8. Completion checklist and reporting

- [ ] Supported catalog actions work browser → Go → PostgreSQL → production plugin
  worker/callback adapter → Go → browser live stream.
- [ ] Operator auth, explicit session targets, capability checks and confirmations
  prevent anonymous, wrong-target and unsupported dispatch.
- [ ] IDs, idempotency, ack/result, expiry, audit and unknown-outcome semantics are
  implemented and tested; there is no automatic replay across reconnect/restart.
- [ ] Training settings round-trip actual readback; walk records effective arguments
  and honest API/observation semantics.
- [ ] Reference-shaped Actions and Client UI, inspectable durable history, all
  required responsive comparisons and no HTTP live-data regressions.
- [ ] Existing regression, PostgreSQL/race, Python, frontend, browser and container
  checks pass, with exact evidence recorded.
- [ ] Required capabilities without a source/primitive remain explicit blockers;
  no invented `go_clientless()` or machine-wide process killing.
- [ ] Real runtime results are recorded separately; absent mutation authorization/
  runtime evidence keeps that gate open. (Updated 2026-09-27: one authorized
  `training.radius.set = 20` round trip/readback completed on nuker1, phBot
  20.1.1/plugin 1.1.0; see `docs/phbot-capabilities.md`. Remaining command/runtime
  catalog coverage is still open.)
- [ ] Update `AGENTS.md`, `docs/protocol.md`, `docs/phbot-capabilities.md`,
  `docs/reference-parity.md`, `README.md`, `plugin/README.md` and configuration docs.

Do not report Slice 3 fully complete while required controls or real-runtime gates
remain unresolved. Report implementation/simulator completion separately from
blocked capability/runtime gates. At context boundaries write: active package,
completed change, affected files, checks actually run, unresolved blockers and the
exact next step. Preserve historical evidence and unrelated slice status.

### 2026-09-27 LAN test update

The operator-authorized same-value training-radius command on nuker1 reached durable
acceptance, completed as `observed`, and its following live training-area readback
remained radius 20. This confirms a narrow real-runtime command path and browser
`/api/live` delivery only. Plain HTTP LAN form submission now generates
cryptographically random idempotency keys with `crypto.getRandomValues()`; this fixes
the secure-context restriction on `crypto.randomUUID()` without changing the HTTP LAN
deployment settings.

P7 verification update: the LAN app was reviewed at 1440×1000, 1280×800,
390×844 and the 2560×1315 reference resolution. The Actions grid remained two
columns at mobile width, and the command history was reachable by vertical scrolling;
the Stats table kept horizontal overflow inside its own scroll region. CI `validate`
passed with PostgreSQL and `TEST_DATABASE_URL`, including `go test -race ./...`.
Local race testing remains unavailable because this Windows toolchain has
`CGO_ENABLED=0`. On the live nuker1 session, the browser received
`unsupported_runtime_primitive` for Clientless and kept it disabled; no command was
submitted. Execute Script remained disabled. The disposable production-worker
fake-adapter command smoke passed. A CI reconnect-smoke harness request lacked the
operator cookie; the workflow was corrected to log in before credential creation and
is pending rerun. The 390×844 Stats capture also showed a hidden character table:
mobile CSS hid every `.agent-table`, including the character table, but supplied no
replacement cards. The rule now targets only `.agent-panel .agent-table`, and the
production browser audit asserts that a live character row remains visible within the
table's bounded scroller.

CI follow-up on `fd30d8c`: PostgreSQL/race validation, production fake-adapter
command execution, the browser `/api/live` audit, authenticated agent reconnect and
responsive assertions passed. The database-outage probe found a PostgreSQL type
error in command reconciliation (`text * interval`); reconciliation now builds a
typed interval and its integration test checks both the expired state and durable
audit row. Local Go tests and vet pass; the host lacks Docker/PostgreSQL, so this
integration fix awaits the next GitHub CI run. Exact next action is to push and rerun
CI, then inspect review findings for that head.

Remaining gates: real Walk route traversal on plugin 1.1.2 (not installed and no
Walk action issued), safe per-session Clientless primitive, broad real-runtime API
coverage, and subsequent review of the corrected CI head. Do not claim Slice 3
complete or proceed to Slice 4.

## 9. Ready-to-use implementation prompt for GPT-6 Luna

```text
Implement Slice 3 — Remote commands in this repository, following AGENTS.md and
docs/slice-3-implementation-plan.md. This request is scoped to Slice 3 only: stop
after its implementation and acceptance checks; do not continue to later slices.

Read Git status and the plan's repository map first. Work through P0–P7 sequentially,
with small tested increments and concise progress updates. Use phmonitor_screenshots,
especially 02-stats-05.png and 03-phbot-tools.png, as layout references. Inspect the
public demo before UI implementation; inaccessible dialogs are not verified evidence.

Preserve multi-socket agent behavior and stable character/session fencing. Commands
must be authenticated, explicitly targeted, capability-aware, expiring, idempotent
and durably audited. Keep every live browser update on the existing /api/live socket.
Dispatch phBot calls only through a bounded allowlist on the callback thread.
Do not invent APIs, infer botting state from a command, execute arbitrary scripts or
kill client processes as a clientless workaround. Investigate unsupported required
capabilities and record precise blockers while finishing independent work.

Use the production plugin worker with fake adapters for command tests and simulator
flows. Do not operate real bot characters without explicit authorization. Keep
simulator and real-runtime evidence separate. Update the contract/evidence docs and
the AGENTS.md resume ledger after each package. Do not stop after a plan, scaffolding,
one action or a passing unit test. Complete all unblocked Slice 3 work, run the
acceptance matrix, and report any remaining gates honestly. Do not merge or publish.
```
