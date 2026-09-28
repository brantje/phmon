# Agent protocol versions 2–6

Slice 1 introduced authenticated agent connectivity (v1). Slice 2 evolves that
contract to v2 and adds character identity registration, snapshots, state updates and
leave messages. Slice 3 adds v3 command delivery. Slice 4 adds v4 resource snapshots
and deltas. Protocol v5 adds nullable live death state and the original death event
frame. Protocol v6 adds canonical event batches. The backend continues accepting
v2–v5 agents; v2–v4 cannot submit events, and v5 retains its death frame and
individual acknowledgement.
Sections below retain the v2 baseline contract; later sections define version-specific
extensions and limits.

## Transport and authentication

- Endpoint: GET /agent upgraded to WebSocket.
- Production deployments use wss:// with normal certificate validation. Plain ws://
  is for trusted local development only.
- Every upgrade request carries Authorization: Bearer <agent-token>.
- Tokens are created per logical agent through the dashboard or `phmonctl agent create`.
  Both use the same server-side generator/store path. PostgreSQL stores only a
  SHA-256 token hash. A token is permanently bound to one stable agent_id.
- Multiple concurrent phBot sockets/profiles may intentionally reuse the same
  agent ID/token when they belong to one logical agent. Use separate credentials
  when they should be separate logical agents; credentials are not per-socket.
- Tokens never belong in URLs, logs or QR codes. The credential-creation response is
  the sole browser-visible exception: it returns the newly generated plaintext token
  once with `Cache-Control: no-store`. Existing tokens are never retrievable.
- The server rejects a missing/invalid token before WebSocket upgrade.
- The first application message must be hello within 5 seconds. The agent_id in
  hello must match the identity authenticated by the bearer token.
- Application messages are JSON text frames. Protocol v2/v3 frames are limited to
  8 KiB; v4 resource snapshot/delta frames may be up to 256 KiB.

Protocol version: latest 5. Version 1 agents are rejected with an explicit
unsupported protocol close reason because the character identity/state contract is
required. Versions 2–4 remain accepted for rolling deployment compatibility.

## hello

Agent to server:

    {
      "type": "hello",
      "protocol_version": 2,
      "agent_id": "9d63c35b-1ff2-4d76-a17c-10514af2f664",
      "plugin_version": "1.1.0",
      "phbot_version": "21.1.9",
      "sent_at": "2026-09-26T14:00:00Z"
    }

plugin_version and phbot_version are bounded to 64 characters. sent_at is diagnostic
agent time; server-owned timestamps remain authoritative.

A valid hello authenticates the durable agent and updates first/last-seen and version
metadata. An agent may have multiple active WebSocket connections and observe
multiple characters at the same time. Each socket receives a distinct server-owned
connection generation; reconnecting or closing one socket must not cancel another
socket for the same agent. The agent is connected while any authenticated socket is
active.

Server to agent:

    {
      "type": "hello.ack",
      "protocol_version": 2,
      "heartbeat_interval_seconds": 10,
      "heartbeat_timeout_seconds": 30
    }

Unsupported protocol versions close with an explicit policy/unsupported-data reason.
Malformed or identity-mismatched hello messages are rejected.

## heartbeat

After hello acknowledgement, the plugin sends:

    {
      "type": "heartbeat",
      "protocol_version": 2,
      "sent_at": "2026-09-26T14:00:10Z"
    }

A valid heartbeat refreshes the server-owned last_seen_at value. If no valid
application message arrives for 30 seconds, the server closes that socket and ends
only character sessions owned by its connection generation. The agent is marked
disconnected only when its last authenticated socket closes. WebSocket ping/pong
control frames may exist at the transport layer but do not replace the application
heartbeat.

## Character identity and registration

Agent identity and character identity are separate UUIDs. Before the backend has
resolved a character, the plugin sends an authenticated registration request:

    {"type":"character.identify","protocol_version":2,"server":"Example Silkroad",
     "name":"CharacterName","guild":"Optional guild name","sent_at":"2026-09-26T14:00:11Z"}

The backend trims and case-folds server and name for a durable identity key, assuming
character names are unique within one game server. `guild` is optional: omitted means
guild was unavailable, while an explicit empty string means the plugin observed no
current guild and clears saved guild metadata. It does not use agent ID,
connection generation, guild, or undocumented player/account ID stability. This is
the only character-related request before the backend issues an ID and establishes a
live session claim for that character on this connection generation. A successful
identify makes presence online; until the following snapshot arrives, state fields
are unknown for this newly claimed observation. The backend responds:

    {"type":"character.registered","protocol_version":2,"character_id":"<uuid>"}

The character UUID is stable across agents, reconnects and backend restarts. Every
subsequent character-scoped message MUST include `character_id`; the server never
routes state from connection identity alone. An agent can register multiple game
characters over time or multiplex several by sending explicit IDs.

## Character snapshots, updates and presence

After identification on join or reconnect, the plugin sends the complete available
snapshot. Identification establishes character-level authority; a snapshot cannot
open a session or retake a character after another generation has taken authority.
An explicit later identify may transfer authority. Delayed writes from a generation
that lost ownership are rejected/fenced. This is per character, so one generation
may continue observing B while another owns A.

    {"type":"character.snapshot","protocol_version":2,"character_id":"<uuid>",
     "sent_at":"2026-09-26T14:00:12Z","state":{"level":110,"hp":32207,"mp":8744,
     "current_exp":34360711,"max_exp":4044607839,"sp":997208910,"gold":99920423940,
     "region":25000,"zone":"Jangan","x":6428.2,"y":1086.7,"z":-32.6,"botting":null}}

Changed observations use `character.state` with the same explicit ID and state
shape. Missing fields mean unavailable/not observed; null botting means the verified
phBot API has no read-only state getter. Values are validated before persistence.
Server-owned state/session/activity timestamps are authoritative; plugin `sent_at` is
diagnostic only.

`character.left` carries protocol version, explicit character ID and diagnostic
`sent_at`. The plugin sends it before identifying a switched character. A full
`character.snapshot` replaces all current state fields, clearing unavailable values
to unknown instead of carrying them from an earlier session. Plugin `character.state`
messages also carry the complete current observation: omitted/unavailable fields are
cleared, including position/zone when the current position read fails. The server
replaces current fields for both state and snapshot messages; neither message may
make unavailable values look freshly observed by retaining earlier values. Updates
are accepted only for that active agent/generation/session. Socket close and heartbeat
expiry close sessions owned by that generation; backend process startup closes all
persisted live sessions with a distinct end reason. After
reconnect a character starts offline and becomes online after explicit identification;
the full snapshot then restores authoritative current state. Backend restart retains durable records/state and never resurrects
live sessions.

The backend fences writes with the authenticated agent ID and socket's connection
generation, which is stored on character sessions. A socket cannot modify another
socket's session, and at most one live observer is represented per character; a new
authorized observer for the same character supersedes the old session. Different
characters observed over different sockets for one agent can remain online together.

If a valid snapshot, state update or leave targets a character session no longer owned
by that socket generation, the server sends a nonfatal character-scoped response and
keeps the socket and its other character sessions alive:

    {"type":"character.rejected","protocol_version":2,
     "character_id":"<uuid>","reason":"not_current_session"}

The plugin stops publishing updates for that rejected identity until a different
character is observed or the socket reconnects. Malformed protocol/state data remains
connection-fatal. PostgreSQL cleanup is attempted when a socket closes; while the Go
process is running it also periodically compares open durable sessions with the exact
live `(agent_id, connection_generation)` registry entries. After a database outage,
sessions for dead generations are ended as `agent_disconnected` without affecting
still-live sibling sockets. This recovery check runs every three seconds while the
database is healthy; outage/recovery lifecycle coverage is simulator/Compose evidence,
not an additional real-phBot claim.

## reconnect

The plugin reconnects automatically after transport/backend failure. Backoff begins
near 1 second, doubles to a 30-second cap, adds bounded jitter, and resets after a
successful hello/ack. On reconnect the plugin sends hello, resolves its observed
characters and publishes full snapshots. There is no durable local event store.

## Browser live-data protocol v1

All live monitoring data exposed to the browser uses a separate versioned WebSocket
protocol. The browser connects only to the same-origin Nuxt endpoint
`GET /api/live`; Nitro 2 relays that WebSocket to the private Go `/api/live`
endpoint. `NUXT_BACKEND_URL` stays server-only. The relay forwards only the named
operator-session cookie and browser `Origin` needed for authentication; it does not
forward arbitrary cookies, authorization headers or other credentials. Streamed
payloads contain monitoring presentation data only.

Protocol version: 1.

This is the only browser transport for current/live monitoring state. Initial
snapshots, filtered character lists, character details, agent status, groups,
dashboard-derived values, manual refresh, committed-state replacements and reconnect
synchronization all use this WebSocket. XHR, `fetch`, `useFetch`, HTTP polling,
long polling, SSE and server-side HTTP bootstrap are prohibited for live views.
Future map positions and live layers follow the same rule; static map tiles/icons may
still use HTTP.

### Subscriptions and revisions

A browser connection may keep up to 32 subscriptions. Subscription IDs are stable
browser-owned identifiers and revisions are monotonically increasing integers.
Changing a filter or detail identity sends a newer revision for the same subscription
ID. Revisions remain monotonic even when an ID is unsubscribed and later recreated on
the same socket. The server retains the highest accepted revision for that connection
and rejects an equal/older replacement as `obsolete_revision`; the browser also ignores
any response whose subscription ID, revision or stream no longer matches its active
subscription. This prevents an already-queued snapshot from an earlier detail identity
being accepted as the current detail after subscription reuse.

Examples:

    {"type":"subscribe","protocol_version":1,
     "subscription_id":"agents","revision":1,"stream":"agents"}

    {"type":"subscribe","protocol_version":1,
     "subscription_id":"character-list","revision":7,"stream":"characters",
     "filter":{"q":"Fixture","group_id":"<group-uuid>"}}

    {"type":"subscribe","protocol_version":1,
     "subscription_id":"character-detail","revision":3,"stream":"character",
     "filter":{"character_id":"<character-uuid>"}}

    {"type":"subscribe","protocol_version":1,
     "subscription_id":"groups","revision":1,"stream":"groups"}

Character-list search preserves the existing backend search semantics for
name/guild/server/zone and optional persisted `group_id`. Character detail uses the
stable server-scoped `character_id`. Identity and filtering remain server-owned; the
Nuxt relay does not reinterpret them.

Manual refresh is a WebSocket protocol action against the current revision:

    {"type":"refresh","protocol_version":1,
     "subscription_id":"agents","revision":1}

Unsubscribe uses the active subscription ID/revision:

    {"type":"unsubscribe","protocol_version":1,
     "subscription_id":"character-detail","revision":3}

The server responds with whole replacement snapshots rather than incremental patches.
That keeps one durable source of truth and makes reconnect synchronization explicit:

    {"type":"snapshot","protocol_version":1,
     "subscription_id":"agents","revision":1,"stream":"agents",
     "data":{"agents":[...]}}

    {"type":"snapshot","protocol_version":1,
     "subscription_id":"character-list","revision":7,"stream":"characters",
     "data":{"characters":[...]}}

    {"type":"snapshot","protocol_version":1,
     "subscription_id":"character-detail","revision":3,"stream":"character",
     "data":{"character":{...}}}

    {"type":"snapshot","protocol_version":1,
     "subscription_id":"groups","revision":1,"stream":"groups",
     "data":{"groups":[...]}}

A missing character detail is represented by `{"character":null}`. A temporary
database/read failure does not cause an HTTP fallback. It produces:

    {"type":"subscription.unavailable","protocol_version":1,
     "subscription_id":"character-list","revision":7,"stream":"characters",
     "reason":"temporarily_unavailable"}

The browser retains its last received data, marks it stale, and waits for a fresh
WebSocket snapshot. PostgreSQL outage detection invalidates live subscriptions once so
connected browsers become stale; recovery invalidates them again so they synchronize
even when no new agent event occurs.

Relevant committed changes invalidate active subscriptions: agent connect/disconnect
and heartbeat metadata, character identify/snapshot/state/leave, group create/rename/
delete/member mutations and session reconciliation. Invalidations are coalesced for
500 ms per browser, which also enforces a minimum interval between rebuild passes. The
hub permits at most two browser snapshot rebuild passes to query live state at once,
leaving database-pool capacity available for agent ingestion. A notification that
arrives while a snapshot is being built remains queued, so another throttled snapshot
pass observes changes committed during the first pass.

### Heartbeats, reconnect and bounds

The Go live endpoint sends an application heartbeat approximately every 10 seconds:

    {"type":"heartbeat","protocol_version":1,"sent_at":"2026-09-27T00:00:00Z"}

The browser responds with:

    {"type":"heartbeat","protocol_version":1}

A browser connection that does not exchange application messages within the heartbeat
timeout is closed. Browser reconnect starts near one second, uses jittered exponential
backoff and is capped at 30 seconds. On a new socket every active subscription is
restored and must receive a fresh snapshot before the connection is marked current.
Last snapshots remain visible and stale while reconnecting. HTTP fallback is never
attempted.

Client messages are limited to 16 KiB, server messages to 512 KiB, and per-client
outgoing/pre-connect queues are bounded. Snapshot/database operations and writes have
deadlines. Slow consumers are disconnected (1013) rather than being allowed to block
agent ingestion; oversized messages are rejected (1009).

Browser upgrades at the Nuxt relay require a same-origin `Origin`. The private Go
endpoint independently rejects a cross-origin browser upgrade; server-to-server Nitro
connections carry only the named operator-session cookie and `Origin` needed for
authentication. Browser live subscriptions require a valid operator session, while
the trusted-network deployment boundary remains in effect.

### HTTP compatibility boundary

Existing read endpoints (`GET /api/agents`, `GET /api/characters`,
`GET /api/characters/{character_id}` and `GET /api/groups`) remain available for
diagnostics/backward compatibility only. The Nuxt live UI MUST NOT call them.

HTTP remains the action transport for credential creation and group mutations, as well
as static assets, uploads/downloads, health checks and future non-live historical
queries. A successful action response acknowledges that action only; browser live
state changes exclusively when the corresponding WebSocket replacement arrives.

## Slice 3 protocol v3: remote commands (2026-09-27)

Protocol v3 is negotiated per agent socket. Protocol v2 remains accepted for
monitoring; the backend does not send v3 command frames to v2 sockets and admission
reports `plugin_upgrade_required`. Multiple sockets under one logical agent remain
independent; command routing uses the exact current `(agent_id, connection_generation)`
and stable `character_id + session_id` recorded at admission.

After `hello.ack` (which includes a server-owned whole-second UTC RFC3339
`server_time` for compatibility with the embedded Python parser), a v3 worker sends
one bounded `agent.capabilities` frame. The server intersects its fixed catalog
with support reported by that exact socket. Capability modes are schema-bound; the
current training-area modes are reported as `current_position`, `position` and
`named` only when their documented primitives are importable. There is no sibling
socket capability union.

The server keeps both `server_time` and command `expires_at` at whole-second UTC
RFC3339 for compatibility with deployed embedded parsers that validate `Z` at
position 19. The durable PostgreSQL deadline and Go send deadline retain full
precision; the wire timestamp is truncated, which can only shorten effective
validity by less than one second. `ttl_ms` remains a second upper bound. Plugin
1.1.1 also accepts up to nine fractional digits for forward compatibility.

V3 `character.registered`, snapshots, state and leave frames carry `session_id`.
Character-targeted controls and results always carry both character and session IDs.
Training state is reported as `character.control_state`, scoped to the same session;
the local training-script path is omitted. New ownership sends a `command.revoke` to
the prior generation. A worker also clears queued commands on local character/profile
switch, callback leave or revoke.

Command dispatch is a Go-owned bounded queue and one ordered writer per agent socket.
The durable command intent and audit event commit before the `queued` row is eligible
for dispatch. Go claims `queued -> dispatching`, rechecks durable session ownership,
then writes only to the recorded generation. A successful WebSocket write records
`sent`; the plugin's `command.ack` means accepted to its bounded callback queue, not
invoked. `command.result` is accepted only from the command's original authenticated
agent generation and session. Duplicate acknowledgements/results do not duplicate
effects or audit transitions.

An execute frame carries `command_id`, `character_id`, `session_id`, fixed catalog
`name`, validated `args`, server-owned `expires_at` and remaining `ttl_ms`. The
callback rejects stale/expired frames, repeats all schema/runtime checks and invokes
at most one fixed adapter per phBot `event_loop()` callback. The networking worker
never calls phBot APIs. The plugin retains bounded command-ID deduplication but does
not replay pending work after socket reconnect. If delivery or final evidence is
ambiguous, the durable state is `unknown`; Go never re-dispatches it. A transport
fence cannot retract an effect after phBot invocation.

The wire `expires_at` uses whole-second UTC RFC3339. The durable server deadline is
not rounded; flooring the wire value can only make the plugin expire the command
earlier, and the minimum of that value and `ttl_ms` prevents extension. Plugin
1.1.1 accepts both whole-second and fractional timestamps without relying on
Python `datetime.fromisoformat`; older v3 plugins that support the rest of the
command contract can continue using the whole-second server timestamp.

Lifecycle states are `queued`, `dispatching`, `sent`, `acknowledged`, `completed`,
`failed`, `expired` and `unknown`. Queued expiry, dispatch-deadline expiry before
send, and known writer backpressure before enqueue are definitely unexecuted and
record as `failed` or `expired`; `unknown` is reserved for delivery or execution that
may have occurred without conclusive evidence. A 30-second result timeout after the
10-second execution window becomes `unknown` for ordinary
commands; `character.walk` has a six-minute result grace to cover its bounded
five-minute callback-driven route. API booleans
are stored as `api_return`; void calls complete only with `verification: unverified`
unless a fresh documented readback verifies the setting. Walk uses the documented
path finder and advances bounded same-region waypoints through callback-time movement
calls; it never uses generated teleport scripts. Completion is reported only after
live position readback reaches the last waypoint and destination within the documented
application tolerance. Target/region changes, missing readback, or the five-minute
route limit produce an honest unknown outcome. Return-scroll does not claim teleport
completion from its API call; disconnect does not change relog configuration; botting
remains unknown absent a documented getter.
Walk admission additionally requires the serving socket to advertise plugin version
1.1.2 or newer, so older v3 agents cannot receive the prior direct-movement behavior.

Operator authentication is separate from agent bearer authentication. Browser
control, monitoring, credential and group endpoints require the named HttpOnly
operator cookie. The Nitro `/api/live` relay forwards only that cookie and the
browser Origin to the private Go endpoint; it never forwards arbitrary browser
headers/cookies. Same-origin browser WebSocket upgrades and configured Go Origin
checks remain required. Logout/expiry closes associated live sockets. Live command
history and control/capability replacements use the existing browser protocol v1
`/api/live` subscriptions (`commands` and `controls`); no HTTP history/live polling
is added. The command POST returns only durable acceptance and a command ID.

The command catalog's application safety bounds are: JSON request body ≤16 KiB;
trace name 1–64 UTF-8 bytes; named training area 1–100 UTF-8 bytes; coordinates
finite and each absolute axis ≤10,000,000; positive explicit region; and radius from
1 through 10,000. These are PhMon safety limits, not phBot-published maximums.
Coordinate training-area mode must use the currently observed region; walking is
same-region only. Frames remain ≤8 KiB, one action is in flight per character,
callback queue capacity is 16, outbound result capacity is 32, and the plugin retains
up to 256 command IDs during its process lifetime.

## HTTP agent API

GET /api/agents returns safe presentation fields for agents that have connected at
least once: stable id, connected state, first/last seen, connect/disconnect times,
current connection start when active, protocol version, plugin version and phBot
version. Stored credential hashes/tokens are never returned.

POST /api/agents/credentials creates one new stable agent identity/token pair using
the same domain generator/store path as phmonctl. The plaintext token is returned in
that creation response only; PostgreSQL persists only its SHA-256 hash. Both endpoints
are no-store.

Nuxt retains same-origin diagnostic read equivalents and HTTP action routes. The live
browser UI does not call those diagnostic reads; it uses the browser live-data
WebSocket protocol above. Browser credential provisioning requires the operator
session described in the Slice 3 contract below.

Slice 2 retains diagnostic `GET /api/characters?q=&group_id=` for
name/guild/server/zone search, `GET /api/characters/{character_id}` for stable
details, and persisted groups under `/api/groups` with explicit
`/members/{character_id}` mutation operations. Responses never contain agent
credential material. Group edits and credential provisioning require the
authenticated operator session and a trusted deployment network.

## Foundation health contract

- GET /healthz: process liveness; HTTP 200 with status ok even if PostgreSQL becomes
  unavailable after startup.
- GET /readyz: fresh PostgreSQL ping with a two-second deadline.
- Nuxt GET /api/health proxies readiness server-side with a bounded timeout.
- Health errors are sanitized and no credentials are returned.

## Slice 3 command/auth contract (frozen 2026-09-27)

Slice 3 introduces agent protocol v3 for command delivery while preserving v2
monitoring. A v2 agent remains valid for monitoring and is always control-disabled
with reason `plugin_upgrade_required`; the server never sends v3 command frames to
a v2 connection.

Operator mutations use a separate cookie-authenticated control plane. The operator
secret is configured only in the Go server environment. Successful login creates a
cryptographically-random opaque session token; only its SHA-256 hash is retained
in bounded process memory. Sessions expire absolutely after eight hours and are
lost on backend restart. The cookie is HttpOnly, SameSite=Strict, Path=/, and Secure
for HTTPS origins. Plain HTTP origins are rejected unless
`OPERATOR_ALLOW_INSECURE_HTTP=true`; this is intended only for explicitly configured
trusted LAN deployments and sends the cookie without encryption. The agent bearer token is
never accepted as operator authentication.

Cookie-authenticated HTTP mutations and browser WebSocket upgrades validate Origin
against configured instance origins. The Nuxt server forwards only the named PhMon
operator-session cookie to private Go routes; it does not forward arbitrary browser
cookies or Authorization headers. Credentials are never placed in URLs, localStorage,
live payloads, or logs. The Go live handler skips the WebSocket library's default
Host-versus-Origin comparison only after this exact Origin allowlist and operator
session middleware succeeds. This supports the same-origin browser-to-Nuxt-to-Go
proxy where the backend Host differs from the browser Origin.

Protocol v3 keeps the v2 hello/heartbeat/character message semantics and adds:

    {"type":"character.registered","protocol_version":3,
     "character_id":"<uuid>","session_id":"<uuid>"}

    {"type":"agent.capabilities","protocol_version":3,
     "schema_version":1,"commands":[
       {"name":"bot.stop","supported":true},
       {"name":"client.clientless","supported":false,
        "reason":"unsupported_runtime_primitive"}
     ]}

    {"type":"command.execute","protocol_version":3,
     "command_id":"cmd_<uuid>","character_id":"<uuid>",
     "session_id":"<uuid>","name":"bot.stop","args":{},
     "expires_at":"<UTC RFC3339>","ttl_ms":10000}

    {"type":"command.ack","protocol_version":3,
     "command_id":"cmd_<uuid>","character_id":"<uuid>",
     "session_id":"<uuid>"}

    {"type":"command.result","protocol_version":3,
     "command_id":"cmd_<uuid>","character_id":"<uuid>",
     "session_id":"<uuid>","status":"completed",
     "verification":"api_confirmed","api_return":true,
     "effective_args":{},"observed_after":{}}

Every character-scoped command frame, acknowledgement, result, control-state report
and revocation carries both explicit `character_id` and durable `session_id`.
The server resolves the authenticated agent and exact connection generation from
the current durable character session; neither is accepted from the browser.

Canonical Slice 3 commands and application bounds:

| name                   | arguments                                                | application policy                                                                                                       |
| ---------------------- | -------------------------------------------------------- | ------------------------------------------------------------------------------------------------------------------------ |
| `bot.start`            | `{}`                                                     | documented bool API result                                                                                               |
| `bot.stop`             | `{}`                                                     | documented bool API result                                                                                               |
| `trace.start`          | `{name:string}`                                          | trimmed 1..64 chars                                                                                                      |
| `trace.stop`           | `{}`                                                     | documented bool API result                                                                                               |
| `training.area.set`    | discriminated `current_position`, `position`, or `named` | region must be explicit/observed and positive; coordinates finite and abs <= 10,000,000; named area trimmed 1..100 chars |
| `training.radius.set`  | `{radius:number}`                                        | finite 1..10,000; this is a PhMon safety bound, not a claimed phBot maximum                                              |
| `character.walk`       | `{region:int,x:number,y:number,z:number}`                | same observed region only; finite coordinates abs <= 10,000,000                                                          |
| `character.return`     | `{}`                                                     | bool means scroll invocation accepted, not teleport completion                                                           |
| `character.disconnect` | `{}`                                                     | void return; does not alter relog settings                                                                               |
| `client.clientless`    | `{}`                                                     | unsupported until a safe documented/versioned per-instance primitive is verified                                         |

The documented phBot `start_script(str)` accepts script text, while the public API
does not provide a trusted script catalog/list operation. Slice 3 intentionally does
not accept raw script text or filesystem paths as a command; a future script surface
needs a bounded reviewed catalog before it can be exposed.

Unknown fields, wrong JSON types, booleans supplied as numbers, NaN/Infinity,
oversized strings, stale session IDs, unavailable training areas and cross-region
walks are rejected before dispatch. Python repeats equivalent validation against
fresh callback-thread runtime context immediately before invocation.

The command lifecycle is durable: `queued -> dispatching -> sent -> acknowledged ->
completed|failed`, with `expired` for work that never becomes valid to invoke and
`unknown` when execution may have occurred but a trustworthy final result was
lost. Intent/audit is committed before dispatch. A socket write is not execution
success. No remote action is automatically replayed after reconnect or backend
restart. One in-flight action is admitted per character.

Browser live protocol v1 remains one shared WebSocket and gains optional
`controls` and `commands` streams. HTTP POST returns only acceptance/command ID;
authoritative lifecycle/history/control-state replacement snapshots arrive over
`/api/live`.

LAN deployment note (2026-09-27): browser `crypto.randomUUID()` is restricted to
secure contexts and therefore unavailable on the explicitly supported plain HTTP
local-LAN setup. The command UI uses 16 bytes from `crypto.getRandomValues()` to
produce the idempotency key instead; this remains cryptographically random in that
context. A named operator-authorized `training.radius.set` same-value check completed
on the live phBot 20.1.1/plugin 1.1.0 runtime and its durable `observed` result was
received over `/api/live`. This evidence covers only that specific API/readback path.

Verification update (2026-09-27): CI's PostgreSQL-backed validation job passed with
`TEST_DATABASE_URL` set and ran `go test -race ./...`. The disposable stack command
smoke used the production plugin worker with fake adapters and received the result on
`/api/live`. A read-only LAN browser check of the selected nuker1 session received
`client.clientless.supported=false` / `unsupported_runtime_primitive`; no action was
submitted. Live plugin 1.1.0 does not satisfy Walk's 1.1.2 pathfinding requirement.
These checks do not verify real Walk traversal, Clientless, or the remaining command
mutations.

The `fd30d8c` PostgreSQL/race job passed, but its database-outage smoke exposed a
reconciliation query type error (`text * interval`). The query now constructs the
timeout interval with a typed `make_interval` argument; the integration test asserts
expired state and its audit event. CI must rerun this fix before the outage/recovery
path can be reported as passing. CI then showed the outage/recovery smoke fixtures
were not carrying an operator session cookie through `/api/live` or protected API
calls; they now use the shared login helper and await another run.
The first integration assertion setup also violated `expires_at > created_at`; its
timestamps are now explicit typed arguments. CI also required the credential POST
in the outage fixture to carry its operator cookie; that probe now sends it.
The subsequent stack run passed agent/live smoke but exceeded a 15-second Chrome
DevTools startup deadline. The browser audit now allows 30 seconds and includes
Chrome stderr if startup still fails.

### Latest Slice 3 acceptance verification (2026-09-27)

CI run `36330451436` passed the PostgreSQL integration and Go race suites, including
command expiry/audit persistence; production-worker command dispatch with fake
adapters; authenticated agent reconnect and database-outage recovery; and browser
checks through the existing `/api/live` path at desktop and mobile widths. This is
simulator/CI evidence, not proof of broad real phBot command effects. The read-only
nuker1 capability report still rejects Clientless with
`unsupported_runtime_primitive`; no command was submitted. Walk path traversal was
excluded from this test pass by operator instruction. Keep real Walk, safe per-session
Clientless and broad real-runtime command validation open.

## Slice 4 protocol v4: resource observations (2026-09-27)

Protocol v4 retains the v3 hello, character/session fencing, command lifecycle and
heartbeat behavior. The backend continues to accept v2 and v3. Those older agents
retain their prior behavior and do not send Slice 4 resources.

After character registration, the plugin sends `resource.snapshot` with `full: true`,
`revision`, `base_revision: 0`, `character_id`, `session_id`, `sent_at` and a
`resources` object. Later `resource.delta` frames carry the next revision and the
previous revision as `base_revision`, with only changed resource keys. A baseline or
delta may be split across indexed `chunk_index` / `chunk_count` frames; the receiver
assembles and validates every chunk before applying a complete revision in one
PostgreSQL transaction. Each frame is capped at 256 KiB, a complete observation at
2 MiB, at most 12 resource keys/chunks are accepted, and no more than four incomplete
assemblies are retained per agent connection.

The backend verifies the exact authenticated agent generation and active character
session. A first observation or a new session requires a full baseline. A delta must
advance by exactly one from the stored base revision; gaps and stale sessions receive
`resource.resync` and require a new full snapshot. The backend replies `resource.ack`
only after durable commit. Acknowledgement is not a claim that optional packet
enrichment or unsupported source APIs supplied fields.

Each resource value has `availability` (`observed`, `not_observed` or `unavailable`)
and a typed source payload. Observed empty lists remain distinct from missing APIs or
containers that have not been opened. Repeated identical content advances the checked
revision/time without advancing `observed_at`; when a getter stops observing a
container, the last confirmed payload and its observation time remain available with
the current availability marked stale/unavailable.

The authenticated browser `/api/live` connection adds a `resources` subscription
stream. Its filter requires a character ID and may specify up to the supported
resource keys, allowing visible character cards and their active tabs to subscribe
independently. Server authorization and current server scope still apply. Resource
baselines never emit acquisition events; before/after reasoning belongs to later
backend slices.

### Slice 4 item-instance details (2026-09-27)

Protocol v4 resource item objects may now carry the optional `instance` object. Its
source is `vsro_1188_packet`; values are observed packet fields, not interpreted
`api_fields`. The object is tied to the active session, tracker epoch, source slot,
RefObjID and ordered packet sequence. The 64-bit variance and packet option ID/value
pairs are decimal strings so browsers cannot round them. Missing option data remains
`not_observed`; a packet that explicitly carries zero options is an observed empty
list.

The plugin passively queues only 0x3040, 0x3052 and 0xB034. Decoding runs in the
network worker, not `handle_joymax`. It bounds the queue to 128 packets / 2 MiB and
each decoded packet to 256 KiB. 0xB034 operations are not subtype-decoded; every
operation invalidates cached enrichment. Queue loss, unknown flags, malformed data,
session/profile changes and mismatched API item state invalidate as well. The current
decoder follows a pinned vSRO 1.188 packet index plus a pinned RSBot implementation
for field details, but exact Greatest packet layouts and packet-based item enrichment
still require captured runtime fixtures. Independently, phBot 20.1.1 API observations
from plugin 1.2.5+ provide typed white/blues, and 1.2.6+ collects additional typed
scalar fields; the backend presents validated fields after exact dataset/model/code
matching. Greatest's explicit mapping and catalog are validated. Unsupported item
families, unverified blue labels/scales, maximum durability and packet movement
retention remain unavailable. Storage snapshot and movement decoders remain
unavailable.

On read, the backend attaches optional `instance_details` only after the server's
explicit dataset/model/code match. It emits family-specific 5-bit roll quality using
`floor(roll × 100 / 31)`, plus ordered blues only for validated dataset definitions.
API-backed typed scalar fields are presented when observed and validated; unsupported
absolute stats, maximum durability and blue labels/scales without source evidence
remain unavailable. Static metadata and the original source observation stay separate.
Evidence and the real-runtime gate are tracked in
[item-instance-evidence.md](item-instance-evidence.md).

Plugin 1.2.5 adds optional `api_evidence_version: 2` and `api_field_types` to
resource items. `api_fields` remains bounded raw evidence, not trusted presentation.
Integer-keyed dictionaries use `{"mapping_entries":[{"key_type":"integer",
"key":"9","value":3}]}` to preserve order, zero values, integer keys and
collisions with textual keys. String-only dictionaries retain their prior format;
large integers remain decimal strings. `api_field_types` contains only field names,
types, collection sizes and sampled key types, not unknown field values. Existing
v4 JSON persistence accepts these additive fields without a service change. Typed
API-backed `instance` conversion is still gated on actual runtime field semantics.


### API-backed item presentation (phBot 20.1.1)

Schema-2 `api_fields.whites` and `api_fields.blues` may produce read-time
`instance_details` with source `phbot_api`, definition version
`phbot-20.1.1-api-v1`, partial status, ordered percentage/blue entries and explicit
availability. This does not manufacture a packet `instance`. Exact dataset/code
matching and source dict count/type validation are required. Raw persisted evidence
is immutable. Unknown families/definitions are omitted with diagnostic status/counts.
Plugin 1.2.6 adds observed named scalar fields as evidence only; absolute stats remain
unavailable until their real values and semantics are checked.

Plugin 1.2.6 was checked against live phBot 20.1.1 resource rows on 2026-09-28.
After exact dataset/model/code matching and typed-field validation, read-time
`instance_details.stats` now carries ordered absolute API values and any matching
observed white percentage. Current/max durability uses the reported current value
and typed `max_durability`; attack, reinforcement and absorption ranges use named
API scalars. Armor decimal defense uses the matching dataset reference range plus
observed white roll, while the API integer defense remains the floor check.
Every observed blue ID/raw value is retained in order; verified dataset codes
receive in-game labels and unfamiliar codes remain literal. A missing white map
does not hide an independently observed scalar or imply a zero roll. Blue quality
percentages, magic-option capacity and Advanced elixir eligibility are not present
in current API evidence and remain unavailable.

## Protocol v5: live death state and durable death events (2026-09-28)

The Go agent endpoint accepts protocol versions 2–5. Deploy the v5-compatible server
first, then the v5 plugin; older agents continue their existing registration,
snapshot, state, command and resource behavior. They do not submit death events.

### Live state

`character.snapshot` and `character.state` may carry `state.dead` as a JSON boolean.
Missing/non-boolean source data is sent as null/omitted and persists as SQL `NULL`.
Claiming a new character session clears prior live fields, including death state.
Online/offline derives from the current session independently. Browser Alive/Dead
counts include only online characters whose boolean state sample is at most 30 seconds
old; stale, offline, missing and invalid timestamps display as unknown.

### `character.died` and acknowledgement

Protocol v5 death occurrence frame, from an authenticated agent:

    {
      "type": "character.died",
      "protocol_version": 5,
      "character_id": "...",
      "session_id": "...",
      "sent_at": "2026-09-28T10:00:00Z",
      "event": {
        "event_id": "...",
        "occurred_at": "2026-09-28T10:00:00Z",
        "source": "phbot.callback",
        "source_ref": "EVENT_DIED",
        "region": 25000,
        "x": 10.0,
        "y": 20.0,
        "z": 30.0,
        "payload": {"cause": "unknown"}
      }
    }

Location fields and payload are optional; the callback's documented data is the
empty string and supplies no cause. The plugin coalesces duplicate callbacks until
it observes `dead: false` or `joined_game()`. It queues the occurrence away from the
callback thread and writes it to an atomic JSON spool before transmission. The spool
is capped at 512 occurrences / 2 MiB; queue overflow or disk failure is surfaced in
plugin status and logs.

Each profile's spool filename is derived from the agent ID and normalized profile
settings path, so two phBot profiles using the same agent credential cannot overwrite
each other's pending occurrences. If the callback arrives after a backend socket
ends, the plugin uses the last registered identity/session for that profile. If no
session is registered yet, it spools the event with the observed server and character
name and waits for that same identity to register before sending; it stores the bound
character/session IDs in the spool before transmission. Local binding fields are not
sent in the wire event. Such an event carries `payload.session_binding = "deferred"`
so the server can apply the explicit pre-session rule. A callback observed while a
different character is registered is discarded as stale rather than rebound to it.

The server replies `event.ack` with the event ID and one of `persisted`, `rejected` or
`retry`. It sends `persisted` only after PostgreSQL commit. The plugin removes a
`persisted` or terminally `rejected` event from its spool and retries a temporary
failure after a delay. Reconnect or plugin restart replays the same occurrence ID;
`activity_events.event_id` is the unique idempotency key.

Ingestion requires protocol v5, authenticated agent ownership, and a durable session
row tied to both the stated agent and character. A known historical session is
accepted for replay within its server-owned start/end interval with five minutes of
clock-skew allowance. For a callback observed after that session's end, the server
also permits a later occurrence while no newer session for the same agent and
character has started by that occurrence time. A deferred event bound after
registration may predate the new session by more than five minutes only when no
other session for that agent and character covers the occurrence time with skew.
These rules let socket-disconnect callbacks and callbacks queued before registration
retain their observed time while rejecting a prior session that reports a death
after a newer session has taken ownership. Unknown sessions, sessions belonging to
a different agent/character, and events outside those rules are rejected. The plugin
aligns occurrence time to the server clock offset received during hello. Timestamps
are also bounded to the last 365 days and five minutes into the future. Malformed or
permanently invalid events receive `status: "rejected"`; temporary storage failures
receive `status: "retry"`.
Cause and location are never inferred. No death is backfilled from HP or snapshots.

### Storage and reads

Migration `000006_deaths.sql` adds nullable `characters.dead` and `activity_events`.
Each event stores schema version, canonical kind/category, explicit agent/character/
session IDs, server, occurrence/receive times, source/reference, optional observed
region/coordinates and bounded JSON payload. Indexes support server, character and
kind time scans. The persisted kind is `character.died`.

Authenticated `GET /api/events` supports server, character ID/name substring, kind,
inclusive `from`/`to` dates, limit and opaque cursor. The authenticated `/api/live`
connection adds an `events` snapshot stream using the same filters. Pages are ordered
by `occurred_at DESC,event_id DESC`; the cursor continues from that pair and includes
the total matching count. The Events → Deaths screen uses this stream. Map links stay
disabled until Slice 7 verifies region transforms for the observed coordinates.

This section records the original v5 death transport. The canonical v6 event contract
below supersedes its storage and query descriptions while preserving v5 compatibility.

## Slice 5 protocol v6: canonical event batches (2026-09-28)

The v6 plugin sends canonical events over the existing authenticated agent WebSocket.
No second event connection or chat transport is introduced. A v5 plugin continues to
send `character.died` and receives `event.ack`; only a v6 hello may send `event.batch`.

    {"type":"event.batch","protocol_version":6,"sent_at":"<UTC RFC3339>",
     "events":[
       {"event_id":"<uuid>","schema_version":1,"kind":"drop.rare",
        "category":"drop","character_id":"<uuid>","session_id":"<uuid>",
        "server":"Silkroad","character":"Alpha","occurred_at":"<UTC RFC3339>",
        "sequence":12,"source":"phbot.callback","source_ref":"EVENT_RARE_DROP",
        "item_model":1234,"payload":{"model":1234}}
     ]}

Each batch has 1–16 events and stays within the WebSocket 256 KiB frame limit. Event
payloads are JSON objects no larger than 32 KiB, at most eight levels deep and 512
nodes, with 8 KiB maximum per string. Common event identity, kind/category, source,
timestamp, sequence, location and item identity remain columns. Agent ID is taken from
the authenticated connection, never from the event body.

Character-scoped events carry registered character and session IDs, matching
server/character identity when supplied, and a positive per-session sequence assigned
before the plugin's worker queue. The server checks that the authenticated agent owns
that exact character session and applies the bounded replay window described above.
Agent-level lifecycle events may omit character/session IDs and sequence. A sequence
collision with different content is a terminal rejection. Event UUID retries retain
the original envelope; identical retries are acknowledged as already persisted.

The backend stores a batch in one PostgreSQL transaction. It returns one result for
each event in `event.batch.ack` only after transaction commit. Result status is
`persisted`, `rejected` or `retry`; temporary database/commit failures retry every
otherwise-valid event in the rolled-back batch. Independently invalid events receive a
terminal rejection while valid siblings can commit in the same transaction. Stable
event ID, optional source-scoped dedupe key and per-session sequence enforce
idempotence.

Migration `000007_event_pipeline.sql` generalizes `activity_events` for nullable
agent-level context and adds sequence, dedupe key and indexed item model/code. Event
queries retain deterministic `(occurred_at DESC,event_id DESC)` order and an opaque
cursor. `GET /api/events` and the `/api/live` `events` stream support server,
character ID/name, kind, category, item, date, cursor and page-size filters. Item
search checks canonical item code/model and observed item name/code in payloads. The
alchemy-attempt result also includes attempts, recorded success/failure outcomes, and
highest observed plus; it makes no probability estimate.

The plugin upgrades the profile-scoped death spool in place and keeps stable UUIDs
while retrying. The durable spool reserves 512 critical events / 8 MiB and allows
2,048 ordinary events / 16 MiB, for 2,560 entries / 24 MiB total. Deaths, rare drops
and alchemy callbacks use the critical reserve. Spool writes use a temporary file,
flush and atomic replace on the network worker. Callback queues remain in memory and
bounded; overflow and disk failures set plugin status and write a concise log entry.
There is a short process-crash window after callback queueing but before the worker
commits the event to disk.

Normal drops (`drop.item`) and rare drops (`drop.rare`) remain distinct. The published
callbacks supply an equippable item model ID only; the pipeline does not turn that into
an item-instance snapshot. Inbound chat preserves bounded message text and raw chat
type. Explicit, named channel strings are normalized literally (`all`/`general`,
`private`, `party`, `guild`, `union`, `global`); numeric and unrecognized types remain
`unknown` until the type mapping is confirmed on a supported runtime. `alchemy_update`
creates one attempt event and `EVENT_ALCHEMY_FINISHED` one
completion event. Reliable party, academy, pet and owned-container transitions come
from identity-aware snapshots; startup, reconnect, missing containers and sampling
gaps reset their baselines. Unknown owned-item acquisition causes remain unknown.

No Slice 5 packet decoder was activated. Any Joymax event decoder still requires a
documented opcode/version allowlist and a captured fixture before activation.

## Slice 6: chat history and commands (2026-09-28)

Migration `000009_chat.sql` builds `chat_messages` as a rebuildable projection of
canonical inbound `activity_events` and audited outbound `commands`. The inbound event
remains authoritative; `event_id` is the projection identity, replay is idempotent,
and legacy chat events are backfilled as `unknown` unless they already carry a
canonical supported channel. Private conversations use a lowercase peer key while
retaining the original peer name, server and character scope. A unique echo is linked
only for the same character session, channel, exact text, private peer where
available, and a ten-second window; ambiguous matches remain separate.

Authenticated `GET /api/chat/contacts`, `GET /api/chat/messages`, and
`POST /api/chat/read` provide bounded conversation history, cursor-based older pages,
contacts, unread counts and durable per-operator read cursors. The existing `/api/live`
WebSocket has a revision-fenced `chat` stream. `GET`/`PUT /api/chat/preferences`
persist browser-notification and local-sound choices for the configured operator.
Migration and store integration tests are gated on `TEST_DATABASE_URL`.

Outbound `chat.send` takes exactly `{channel,text,recipient?}` and uses the existing
authenticated, idempotent, session-fenced, audited command lifecycle. The plugin calls
only the matching documented `phBotChat` method from `event_loop()`, reports callable
modes, and preserves the API boolean as acceptance/failure evidence. `True` does not
mean a remote recipient received the message. Global sends require explicit
confirmation. Numeric inbound channel mappings and phBot's actual accepted text limit
remain runtime gates; the app currently caps one message at 2,048 UTF-8 bytes and does
not split messages.
