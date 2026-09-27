# phBot capability evidence

Slice 1 starts the real phBot integration. This document records only capabilities
verified from public phBot plugin documentation or an actual runtime. Simulator
coverage is tracked separately and is never treated as proof of a real phBot run.

## Sources checked on 2026-09-26

- Plugin introduction: https://plugins.phbot.org/
  - The documentation describes the embedded plugin environment as Python 3.4/3.8.
  - Plugins are loaded from the phBot Plugins folder.
- Example plugins: https://plugins.phbot.org/example-plugins
  - A documented socket example imports Python's socket module and performs TCP I/O.
  - The page states socket support was restored in plugin API v2.0.0.
- Events: https://plugins.phbot.org/phbot-api/events
  - finished() is called when Python unloads / phBot exits.
  - connected() and disconnected() describe the game-server connection, not the
    monitoring backend connection. joined_game() runs after the player selects a
    character.
  - event_loop() runs every 500 ms, but Slice 1 does not need to perform networking
    in that latency-sensitive callback.
- GUI API: https://plugins.phbot.org/gui-api
  - QtBind.init(**name**, ...) creates a plugin tab and must be called at module load.
  - createLineEdit/createButton provide operator-editable fields and actions.
  - text()/setText() read and update widget values.
- Config: https://plugins.phbot.org/phbot-api/config
  - get_config_dir() returns the phBot Config directory with a trailing slash.
  - get_config_path() returns the active player's JSON configuration path when in
    game; the docs warn direct changes to that JSON may later be overwritten.
  - PhMon uses get_config_path() as the player/account portion of its settings key
    and keeps its own per-profile settings file under Config/PhMon/.
- Misc: https://plugins.phbot.org/phbot-api/misc
  - get_version() returns the phBot version string.
- get_profile() returns the active profile name, an empty string for the default
  profile, or None while no player is logged in.

These per-profile settings files are a local configuration choice only. An agent
ID/token identifies one logical PhMon agent; operators may reuse it across multiple
concurrent phBot processes/profiles that should belong to the same logical agent, or
create separate credentials for separate logical agents.

## Login and connection status behavior

Saved credentials are loaded only after get_profile() reports a logged-in player;
None is not treated as the default profile. The connected() and joined_game()
callbacks both trigger profile loading, while event_loop() checks for a newly
available/changed profile every 500 ms. This covers login timing differences without
network work in phBot callbacks. The worker publishes Connecting, Connected, and
retrying states; event_loop() displays those states through QtBind on phBot's callback
thread.

## Slice 1 integration decisions

The official documentation verifies Python socket support but does not document a
bundled third-party WebSocket client. PhMon therefore does not depend on an
unverified websocket package inside phBot. The plugin implements the small RFC 6455
client subset it needs with Python standard-library modules.

Idle receive polling uses select before any bytes of the next WebSocket frame are
consumed. Once frame decoding starts, reads run under a bounded socket deadline; a
mid-frame stall fails the connection and lets the worker reconnect rather than
discarding partial frame bytes and continuing on a corrupted stream.

Configuration is operator-facing through phBot's native QtBind GUI rather than a
hand-edited PhMon JSON file. The GUI contains backend URL, agent ID and token fields
plus Save & Connect. Persistence is scoped with both get_config_path() and
get_profile(): the player configuration identifies the account/character and the
explicit profile name distinguishes named profiles for that player. PhMon stores its
own file under Config/PhMon/ and never writes to the JSON path returned by phBot.
The token field is cleared after load/save; the persisted local token is reused only
while URL and agent ID are unchanged.

The plugin keeps backend networking on a worker thread. phBot callbacks never wait
for backend network I/O. The plugin is authoritative only for its current process;
the Go backend owns durable identity, authentication and connection history.

## Runtime validation

Operator-provided evidence: **Real phBot → PhMon plugin/backend connectivity has
been manually verified.** This confirms basic connectivity only; it does not
validate APIs not listed in the runtime evidence below.

Live-runtime update (2026-09-26): phBot 20.1.1 agents running plugin 1.1.0
registered four online characters through protocol v2. The API showed three active
sockets for one agent identity, with three distinct character IDs/sessions under
that agent, plus another online character under a second agent. Server, character
name, zone, level, HP/MP, XP/SP, gold, region, position, and advancing state
timestamps were observed; guild was returned when available. This manually verifies
multiple concurrent character sessions behind one agent and separation across
agents. It does not verify multiple profiles within one phBot process, character
switching, or botting-state reporting.

The deterministic simulator added with Slice 1 imports the production PhMon.py
transport and exercises the same protocol contract. Hosted CI has verified credential
creation, connect, backend restart, automatic reconnect and disconnect through that
transport, but simulator success must not be recorded as real phBot validation.
The outage/recovery scenario also uses two simulator workers with one token, closes
one while PostgreSQL is stopped, then confirms the other socket and its character
remain online after database recovery. This is automated backend/protocol evidence,
not manual phBot runtime validation.

Public phBot documentation explicitly supports socket. The actual embedded runtime
still needs to record which imports/data APIs behave as expected, especially:

- socket
- ssl
- select
- threading
- hashlib
- base64
- struct
- urllib.parse

## Slice 2 character and state APIs

Sources checked 2026-09-26:

- [Events](https://plugins.phbot.org/phbot-api/events): `joined_game()` is called
  after character selection, but the docs explicitly say character data is not
  loaded yet. `disconnected()` describes game-server disconnection and may be called
  repeatedly. `event_loop()` runs every 500 ms. The plugin therefore waits for a
  populated identity during `event_loop()` and never performs network I/O there.
  Plugin v1.1.0 also handles the case where the plugin is loaded after
  `joined_game()` has already fired: it treats a complete documented
  `get_character_data()` server/name identity as proof the data load finished.
- [Character](https://plugins.phbot.org/phbot-api/character): documented no-argument
  `get_character_data()` returns `None` or an object including server, name, guild,
  region, coordinates, HP/MP, level, gold, current/max EXP and SP. Its example also
  includes `player_id` and `account_id`, but the page does not specify their
  stability or uniqueness scope. PhMon does not use those undocumented semantics as
  a durable key.
- The same Character page documents no-argument `get_position()`, returning x/y/z
  and region, or `None`.
- [Game Data](https://plugins.phbot.org/phbot-api/game-data) documents
  `get_zone_name(region)` for deriving a display zone from the region code.
- [Botting](https://plugins.phbot.org/phbot-api/botting) documents `start_bot()` and
  `stop_bot()` mutations but no read-only botting/training-state getter. Slice 2
  reports this field as unknown rather than inferring state from commands or UI.

Implementation imports only these documented APIs. It copies primitive values on
the callback thread, change-detects at a one-second minimum and refreshes at five
seconds while unchanged. A bounded one-entry worker queue coalesces intermediate
samples. After reconnect the worker resolves the identity again and sends a full
snapshot. The backend timestamp is authoritative; `sent_at` is diagnostic only.
Identity is lowercased/trimmed character name scoped by lowercased/trimmed server
name. This assumes game character names are unique within a Silkroad server; PhMon
does not claim a globally unique game ID. Guild is mutable metadata, not identity.
The collector preserves a documented string value, including `""` for an observed
no-guild value; an absent or non-string guild value is sent as unavailable and does
not clear stored metadata.

Remaining Slice 2 runtime checks: record embedded Python version; verify repeated
disconnect callbacks, character switch, teleport/region change, and reconnect
snapshot behavior. The collector recovery after loading post-join is implemented
and unit-tested, but not isolated as a manual runtime scenario. Botting state remains
unavailable until an authoritative documented/read-only API is verified.

The real-runtime gate requires installing PhMon.py in a supported phBot build,
configuring at least two distinct bot profiles through the PhMon QtBind tab, and
recording the observed phBot version and embedded Python version. Confirm each profile
loads its own URL/agent ID, keeps the saved token hidden in the GUI, connects as the
correct agent, and survives plugin reload/disconnect/restart with automatic reconnect.
Also exercise a profile switch to prove one profile cannot silently reuse another
profile's credentials. Record the observed module/import behavior and results here;
do not infer them from desktop CPython or the simulator.


## Slice 3 remote-command capability matrix (2026-09-27)

The official API was rechecked during implementation. These rows describe public
documented behavior and the PhMon adapter policy; they do not claim availability
on every installed phBot build.

| PhMon command | Public primitive | Result semantics | Slice 3 status |
| --- | --- | --- | --- |
| `bot.start` | `start_bot()` | bool | required |
| `bot.stop` | `stop_bot()` | bool | required |
| `trace.start` | `start_trace(name)` | bool | required |
| `trace.stop` | `stop_trace()` | bool | required |
| `training.area.set` named | `set_training_area(name)` | bool | required when runtime symbol exists |
| `training.area.set` position/current | `set_training_position(region,x,y,z)` | bool | required when an active area exists |
| `training.radius.set` | `set_training_radius(radius)` + `get_training_area()` | bool plus readback | required |
| `character.walk` | `generate_path(x,y)` + `move_to_region(region,x,y,z)` + `get_position()` | async waypoint route; completion waits for live position readback | required when all three symbols exist; single region, no teleport |
| `character.return` | `use_return_scroll()` | bool | required |
| `character.disconnect` | `disconnect()` | void; relog unchanged | required |
| `client.clientless` | no safe public mutation found in Client/Misc/index | n/a | blocked: `unsupported_runtime_primitive` |
| Execute Script | `start_script(str)` / `stop_script()` | starts/stops a script string | not exposed in Slice 3: the public API supplies no trusted script catalog/listing contract; raw script bodies are an arbitrary game-action surface and are excluded by the Slice 3 safety boundary |

Optional imports are probed independently. One missing mutation symbol cannot disable
monitoring or unrelated controls. Capability reports are attached to the exact v3
socket/runtime and intersected with the server-owned catalog; capabilities from
sibling sockets sharing one agent token are never unioned for authorization.
The server also requires the advertised plugin version to be at least 1.1.2 for
`character.walk`; older v3 plugins expose only direct movement and are rejected for
this command even if they report `move_to_region` support.

Training readback exposes only typed region/x/y/z/radius availability and values.
The documented local script `path` is deliberately not sent to the backend. A
`current_position` training-area operation resolves position on the callback thread
immediately before invocation. Region zero auto-derivation is not used. Named-area
selection is separate from coordinate changes.

After character registration the phBot callback queues an initial session-scoped
training-area readback, refreshes it at most every 30 seconds, and queues another
readback after a control mutation. The network worker publishes these queued reports;
it never calls phBot APIs. A session change forces a fresh initial report.

Bool API success is recorded as `api_confirmed`. Void-return operations are
`unverified` unless a fresh documented observation establishes the effect. Walk
uses `generate_path` to produce a bounded list of at most 256 same-region waypoints,
then advances one `move_to_region` target from the phBot callback after `get_position`
observes the current waypoint within the application-defined 12-unit horizontal
tolerance. It reports completed/observed only when the final destination is also
within that tolerance; the route becomes `unknown` on target/region change, timeout
(five minutes), backend-session loss, or missing position evidence. This is a callback-driven route, not a
teleport and not execution of `generate_script` output. The public API rate-limits
path generation to once per five seconds and the plugin surfaces its documented
`False` (rate-limited or not in game) and `None` (no path) outcomes as failed
commands. Return-scroll never claims teleport completion, disconnect never claims
relog was disabled, and botting state remains unknown because the checked public
Botting API still exposes no authoritative read-only getter.

### Slice 3 implementation evidence refresh (2026-09-27)

Official docs rechecked on 2026-09-27:

- [Botting](https://plugins.phbot.org/phbot-api/botting) documents `start_bot()` /
  `stop_bot()` and `start_trace(name)` / `stop_trace()`, all with boolean results.
- [Training Area](https://plugins.phbot.org/phbot-api/training-area) documents
  `set_training_position(region,x,y,z)` and `set_training_radius(radius)` as
  booleans, `get_training_area()` as `None` or a dictionary, and
  `set_training_area(name)` as a distinct boolean name-selection operation. Region
  zero auto-derivation is explicitly limited to non-cave areas.
- [Movement](https://plugins.phbot.org/phbot-api/movement) documents
  `move_to_region(region,x,y,z)` as returning `None`; the API does not wait for
  arrival. PhMon uses only explicit positive same-region destinations.
- [Paths](https://plugins.phbot.org/phbot-api/paths) documents
  `generate_path(x,y)` as returning `None`, `False`, or a waypoint list; it is limited
  to one call per five seconds and does not support teleporting. Cave waypoint tuples
  include a region at index 0. PhMon consumes only same-region waypoints from
  `generate_path`, rejects a route that changes region, and does not execute the
  teleport/wait strings returned by `generate_script`.
- [Inventory](https://plugins.phbot.org/phbot-api/inventory) documents
  `use_return_scroll()` as boolean. [Misc](https://plugins.phbot.org/phbot-api/misc)
  documents `disconnect()` as void and explicitly says it does not change relog
  settings.
- [Client](https://plugins.phbot.org/phbot-api/client) documents only `get_client()`
  inspection (`window`, `pid`, `path`, `running`). No safe clientless mutation was
  located in the official Client/Misc/API index; no process-kill substitute is
  permitted.
- [Script](https://plugins.phbot.org/phbot-api/script) documents
  `start_script(str)` and `stop_script()`. The former runs script text in the
  background. Slice 3 does not accept raw script text, a filesystem path, Python or
  shell content as a remote command. The public API page documents no safe
  list/discovery/manifest operation that could bind a named catalog entry to
  reviewed game actions, so Execute Script remains unavailable pending that bounded
  contract. This is separate from supported typed commands such as `character.walk`.

The implementation probes these optional symbols independently at plugin startup
and reports capabilities for its v3 socket. The callback adapter uses only the
documented functions above. Training radius is exposed only when both setter and
readback getter exist. `current_position` requires `get_position`; explicit
coordinates require `set_training_position`; named selection requires
`set_training_area`. Settings readback allowlists numeric region/x/y/z/radius and
never sends the local `path` field.

Runtime evidence boundary: the live operator LAN runtime is phBot 20.1.1/plugin
1.1.0. Simulator adapter coverage is recorded separately and is not real-runtime
evidence. One narrowly scoped real action is now validated below; remaining mutation
APIs, action effects and clientless must not be inferred from it.

### Live LAN Slice 3 evidence (2026-09-27)

After the operator installed plugin 1.1.0, the deployed LAN stack observed two
protocol-v3 agent identities and four simultaneous character sessions on phBot
20.1.1. The browser received fresh `/api/live` agent/character/control snapshots;
one character's documented training-area readback reported region 25735, coordinates
100/1559/0 and radius 20. This confirms v3 handshake, multi-socket/session reporting,
capability delivery and `get_training_area()` readback on that runtime. It does not
confirm a mutation API or command side effect.

The LAN incident exposed two timestamp compatibility defects. The server initially
formatted `hello.ack.server_time` with fractional seconds although the embedded
parser requires the whole-second `...Z` form; this made authenticated sockets close
before capabilities/character registration. The server now sends whole-second UTC
RFC3339 for both handshake time and wire command expiry. The durable Go/PostgreSQL
command deadline keeps full precision; flooring only shortens wire validity and
`ttl_ms` remains an upper bound. Plugin 1.1.1 parses fractional timestamps too,
without using Python 3.7-only `datetime.fromisoformat`, for forward compatibility.
The operator's live phBot profiles still report plugin 1.1.0; their v3 timestamp
parser is compatible with the server's whole-second wire form.

### Operator-approved radius command validation (2026-09-27)

After explicit authorization to test with `nuker1`, one command set its already
observed training radius to the same value, 20. It targeted the current session
`84bb2a1f-36bf-4993-8b5a-7b0f60e83750`, completed with verification `observed`, and
the browser received the durable command result through `/api/live`; the next live
training-area readback remained region 25735, position 100/1559/0, radius 20. This is
one runtime round trip/readback, not broad validation of every API or a claim about
botting state. No other real command was issued. Remaining runtime checks require
specific authorized actions/observations; Clientless still has no verified safe
per-session primitive.

### Read-only Clientless gate check (2026-09-27)

On the live phBot 20.1.1 / PhMon plugin 1.1.0 deployment, the operator-selected
nuker1 session reported `client.clientless.supported=false` with reason
`unsupported_runtime_primitive` through the v3 capability stream. The authenticated
browser received that result through `/api/live`; the Go Clientless action stayed
disabled for that session. No command was submitted. This confirms fail-closed
capability handling for that runtime, not a Clientless API or action. The current
plugin version also predates the 1.1.2 Pathfinding Walk requirement; Walk remained
disabled and no movement was issued.
