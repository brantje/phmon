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

## Slice 4 resource/API evidence (2026-09-27)

The plugin's resource collector uses the documented getters below. Official API
examples provide deterministic shape fixtures; they are not captures from the current
phBot runtime. The v1.2.0 plugin has not yet been installed/validated on a live phBot
process. The previously verified phBot 20.1.1 / plugin 1.1.0 session proves the older
Slice 3 agent path only.

- [Inventory](https://plugins.phbot.org/phbot-api/inventory): `get_inventory()` is
  `None` or an object with `size`, `gold`, and `items`; empty slots are `None`. The
  documented item fields are `model`, `servername`, `name`, `quantity`, `plus`, and
  `durability`. `get_storage()`, `get_guild_storage()` and `get_job_pouch()` return
  `None` or an object with `size` and `items`; storage APIs report no items before the
  character enters the storage. The docs do not define whether inventory `size`
  includes equipped positions or establish the equipment-slot mapping. The supplied
  `plugin/phMonitorAdapter.py` independently suggests the first 13 entries are
  equipment, but this is a lead only; the collector labels that mapping
  `adapter_lead_runtime_unverified` and exposes the limitation in the UI.
- [Pets](https://plugins.phbot.org/phbot-api/pets): `get_pets()` is `None` or a
  dictionary keyed by pet ID; an empty dictionary is a valid no-summoned-pets state.
  The example pet has `name`, `servername`, `model`, `type`, `hp`, `mounted`, and
  `items`, with `None` empty item positions. Documented types are `none`, `fellow`,
  `horse`, `pick`, `transport`, and `wolf`. The collector preserves unknown/horse
  types and only renders an inventory when an item list is supplied. These integer
  dictionary IDs are only stable for the API observation; cross-session pet identity
  has not been established.
- [Party](https://plugins.phbot.org/phbot-api/party): `get_party()` is `None` or a
  dictionary keyed by party ID; empty is a valid empty party. Example member fields
  are `name`, `guild`, `player_id`, `level`, `x`, `y`, `hp_percent`, and `mp_percent`.
  `player_id` may be zero until a member spawns nearby. The source percentages are
  0–10; the collector multiplies them by ten for display.
- [Academy](https://plugins.phbot.org/phbot-api/academy): `get_academy()` is `None` or
  an object keyed by member ID with an `id` field and members containing `online`,
  `type`, `x`, `y`, `level`, and `name`. The slice stores the latest current
  observation only; event history and unread actions belong to Slice 5.
- [Configuration](https://plugins.phbot.org/phbot-api/config):
  `get_config_dir()` provides the Config directory, and `get_config_path()` is the
  active player's JSON path. Current character server identity selects the matching
  entry in the configured `vSRO.json`; the plugin reads only the version-variant
  flags. The locally observed numeric `version=296` is not protocol 1.188 and is
  ignored. Only a matching entry with known variant flags and no enabled 1.065,
  1.193 or 1.274 variant is treated as generic vSRO 1.188. Missing/ambiguous config
  disables enrichment without affecting API snapshots. No selector is exposed.

The existing adapter file is reference evidence only. Its network/streaming behavior,
guessed item properties, and configuration-success reporting are not copied. Resource
fixtures in `plugin/test_phmon.py` cover documented empty, unavailable and populated
shapes with credentials and unrelated traffic excluded. There are no captured live
API payloads or item packets yet.

`get_party()` and `get_pets()` establish current membership/state only; they do not
establish a Party Setup write contract. The adapter's JSON write plus delayed
`reload_profile()` is an unverified lead. Party Setup remains read-only until the
supported configuration fields, write API, reload behavior and effective-state
readback are confirmed on phBot. No Party Setup mutation is enabled.

Historical status at the initial Slice 4 collector implementation (superseded by
the 2026-09-27 and 2026-09-28 evidence below): generic passive item enrichment
targets vSRO 1.188 only. The archived
[SilkroadDoc packet index](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/Packets)
identifies packet families, and its repository states that its analysis targets vSRO
1.188. At that point, item layouts had not yet been implemented or corroborated.
Current parser and API-backed item presentation behavior, along with unresolved
runtime gates, is recorded in the dated evidence below and in
[`item-instance-evidence.md`](item-instance-evidence.md).

At the time of the initial Slice 4 collector, static item enrichment also remained
open. It was subsequently implemented through an explicit server-to-dataset mapping;
see the 2026-09-27/28 evidence below for current catalog coverage and limits.

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

### Death status and event source (verified 2026-09-28)

- [Character API — `get_character_data()`](https://plugins.phbot.org/phbot-api/character)
  includes a `dead: False` boolean in its documented example. PhMon samples this
  field only when its runtime value is a Python boolean. Missing or non-boolean data
  stays unknown; HP values do not imply alive/dead status.
- [Events API — `handle_event(t, data)`](https://plugins.phbot.org/phbot-api/events)
  documents `EVENT_DIED = 7` and specifies that its data is an empty string. The
  callback records the occurrence; it supplies no cause, so PhMon stores and shows
  `cause: unknown`. A dead-state snapshot alone never creates an event.
- [Character API — `get_position()`](https://plugins.phbot.org/phbot-api/character)
  documents optional region and x/y/z position. The event stores only numeric values
  observed at callback time. This does not validate an outdoor-to-map transform;
  the Deaths view keeps Map navigation disabled until Slice 7 validates a transform.
- Collection is copied into the worker queue from `handle_event`; filesystem and
  network operations stay out of phBot's callback. Repeat notifications are
  coalesced until a live boolean alive observation or a new game-character join.

This source mapping is implemented during Slice 4 as a bounded death increment of
the Slice 5 event pipeline. It does not complete Slice 5's other event kinds,
derived acquisition events, notification rules or general history screens. Actual
phBot death-callback validation remains open because no character was operated for
this increment.

## Slice 3 remote-command capability matrix (2026-09-27)

The official API was rechecked during implementation. These rows describe public
documented behavior and the PhMon adapter policy; they do not claim availability
on every installed phBot build.

| PhMon command                        | Public primitive                                                         | Result semantics                                                  | Slice 3 status                                                                                                                                                                                     |
| ------------------------------------ | ------------------------------------------------------------------------ | ----------------------------------------------------------------- | -------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- |
| `bot.start`                          | `start_bot()`                                                            | bool                                                              | required                                                                                                                                                                                           |
| `bot.stop`                           | `stop_bot()`                                                             | bool                                                              | required                                                                                                                                                                                           |
| `trace.start`                        | `start_trace(name)`                                                      | bool                                                              | required                                                                                                                                                                                           |
| `trace.stop`                         | `stop_trace()`                                                           | bool                                                              | required                                                                                                                                                                                           |
| `training.area.set` named            | `set_training_area(name)`                                                | bool                                                              | required when runtime symbol exists                                                                                                                                                                |
| `training.area.set` position/current | `set_training_position(region,x,y,z)`                                    | bool                                                              | required when an active area exists                                                                                                                                                                |
| `training.radius.set`                | `set_training_radius(radius)` + `get_training_area()`                    | bool plus readback                                                | required                                                                                                                                                                                           |
| `character.walk`                     | `generate_path(x,y)` + `move_to_region(region,x,y,z)` + `get_position()` | async waypoint route; completion waits for live position readback | required when all three symbols exist; single region, no teleport                                                                                                                                  |
| `character.return`                   | `use_return_scroll()`                                                    | bool                                                              | required                                                                                                                                                                                           |
| `character.disconnect`               | `disconnect()`                                                           | void; relog unchanged                                             | required                                                                                                                                                                                           |
| `client.clientless`                  | no safe public mutation found in Client/Misc/index                       | n/a                                                               | blocked: `unsupported_runtime_primitive`                                                                                                                                                           |
| Execute Script                       | `start_script(str)` / `stop_script()`                                    | starts/stops a script string                                      | not exposed in Slice 3: the public API supplies no trusted script catalog/listing contract; raw script bodies are an arbitrary game-action surface and are excluded by the Slice 3 safety boundary |

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
  teleport/wait strings returned by `generate_script(region,x,y,z)`. The latter
  returns `None`, `False`, or a list of script-command strings that may contain
  walking, waits and teleports; the Paths page limits these functions to one call
  per five seconds.
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
  background; its example passes newline-separated command text. Joining
  `generate_script`'s returned lines with newlines for `start_script` is a plausible
  integration inferred from the two pages, not an explicitly documented combined
  call or a verified result on the installed runtime. The pages do not document
  script ownership, concurrent-script behavior or an arrival/status getter. Slice 3
  does not accept raw script text, a filesystem path, Python or shell content as a
  remote command. The public API page documents no safe
  list/discovery/manifest operation that could bind a named catalog entry to
  reviewed game actions, so Execute Script remains unavailable pending that bounded
  contract. This is separate from supported typed commands such as `character.walk`.

**Map navigation follow-up (2026-09-28):** The documented functions support a
candidate generated-script navigation flow, but the deployed `character.walk` still
uses `generate_path` plus observed same-region waypoints. `generate_script` and
`start_script` are not yet probed or invoked by PhMon. Before replacing map-facing
navigation, verify both symbols on the supported phBot runtime, the returned command
grammar and limits, whether starting/stopping affects an existing script, and how
completion or cancellation can be observed. Keep the destination typed and
server/session/floor-scoped; never relay raw script text from the browser.

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

### CI and live capability status refresh (2026-09-27)

CI run `36330451436` validates command lifecycle, expiry/audit persistence and the
production plugin worker using fake adapters, with results delivered on `/api/live`.
It does not establish real API effects for every command. The live phBot 20.1.1 /
plugin 1.1.0 capability snapshot reports Clientless unsupported with
`unsupported_runtime_primitive`; the UI leaves that action disabled and no request
was sent. Walk traversal was not exercised per operator instruction; the installed
plugin remains below the required 1.1.2 pathfinding capability. Execute Script stays
outside Slice 3's bounded command catalog because no trusted script catalog contract
is available. Do not infer or implement a Clientless primitive without official API
and runtime evidence for a session-targeted safe action.


### 2026-09-27 item presentation correction (Slice 4 remains open)

- Inventory's documented fields are model/code/name/quantity/plus/durability:
  https://plugins.phbot.org/phbot-api/inventory . These do not establish rolls/blues.
- PhMon 1.2.1 shows its loaded version and transport version in QtBind. It retains
  a bounded allowlist of additional item API evidence, including decimal-string
  64-bit variance, under `api_fields`. Evidence is not interpreted by heuristics.
- Backend read-time metadata now uses explicit server mapping in
  `server/game-data/servers.json`, with `ITEM_METADATA_DIR` directory override.
  Matching model and nonconflicting code are required. Observed instance JSON
  remains unchanged in storage. Guild and pet slots use the same resolver.
- Greatest maps to operator dataset `gamedata-47c969ded0613d4c2a22`; 14,238 items,
  3,411 distinct local icon files, all present. Verified table-derived rarity,
  degree (legacy degrees 1–10), seal, equipment classification/position and
  requirements are separate from instance data. Unknown semantics stay omitted.
- Open: passive vSRO 1.188 item decoding and representative captured packet
  fixtures are not implemented/available. Therefore rolled defense/absorption,
  percentages, max durability and individual blues are NOT confirmed live.
  The new evidence must be inspected after the operator transfers 1.2.1; if the
  getter does not expose them, a validated passive decoder is still necessary.


### 2026-09-27 item-instance parser implementation (plugin 1.2.2)

Plugin 1.2.2 adds a bounded passive queue and parsers for the corroborated 0x3040
item-stat and 0x3052 durability updates. Any 0xB034 inventory operation invalidates
all cached instance details because operation subtypes are not yet decoded. Malformed
or unsupported packets, queue overflow, session/profile changes, and API model/plus/
empty-slot conflicts also invalidate. The plugin continues publishing API-backed
resources when protocol selection is unknown.

The vSRO 1.188 protocol flags are read from the current server entry in `vSRO.json`;
numeric `version` remains ignored. Malformed flags, conflicting selectors and
duplicate server matches now return `unknown`. Packet opcodes are listed in the
SilkroadDoc index, but the checked-out pages do not establish the complete field
layouts. RSBot is pinned as corroborating implementation evidence only. No live
Greatest packet fixture has been captured, so live decoding is not confirmed.

Evidence matrix: [item-instance-evidence.md](item-instance-evidence.md). Absolute
stat formulas, max durability, verified blue definitions/scales, full inventory
snapshots, storage/pet layouts and decoded moves remain open. No real character was
operated to generate traffic.

### 2026-09-27 active configuration and plugin 1.2.3 check

The current phBot 20.1.1 installation places `vSRO.json` beside the `Config`
directory and represents profiles as a root mapping (for example,
`GreatestSRO: {servers: [Greatest], ...}`). Numeric `version=296` is unrelated to
the game protocol. The previous plugin detector searched only inside `Config` and
expected a different JSON shape, so it reported `unknown` while all four agents
correctly sent plugin 1.2.3 / agent protocol v4 resource telemetry. The corrected
plugin 1.2.4 searches the adjacent file, matches the active server, validates the
explicit variant flags, and reports `protocol_reason` for diagnosis. A local
read-only check resolved the current config to vSRO 1.188. Live confirmation of the
corrected build and actual item packet decoding is pending operator transfer and
naturally arriving updates; see [item-instance-evidence.md](item-instance-evidence.md).

### 2026-09-27 API evidence correction (plugin 1.2.5)

Operator-authorized static reference inspection found that its inventory tooltip
path consumes phBot API attributes and blues. The official inventory example is
not evidence that richer fields are unavailable on this runtime. PhMon's sanitizer
discarded integer dictionary keys, making potentially populated option maps appear
empty. Plugin 1.2.5 preserves typed map entries, additional attribute aliases and
bounded source field types. No such fields are automatically trusted as display
values. Inspect the new API evidence before assuming every missing tooltip input
requires passive packets. See [the investigation](reference/item-tooltip-investigation.md)
for evidence, known formula discrepancies and the next live validation gate.


## 2026-09-27: plugin 1.2.5 API evidence confirmed

Read-only live verification at 21:44:58 UTC confirmed all four characters on
plugin 1.2.5, phBot 20.1.1 and vSRO 1.188. All 244 current items retained evidence
schema 2. Rich data is arriving: `whites` contains integer attribute IDs and integer
percentages, and `blues` contains integer option IDs and values. The prior loss of
integer dictionary keys was the primary blocker for these fields.

Sanitized actual observations are checked in as
`server/internal/resources/testdata/phbot-20.1.1-api-items.json` (14 items; no
credentials, character/session identifiers or unrelated traffic). Python Casque
reports defense rolls 12/19, parry 22, reinforcement 3/32, durability 9, Int 3 and
MP 5; Tiger Bone Coronet reports 61/32, 45, 0/9, 0, Steady 2 and Parry 5%.
Flame Platinum Necklace reports absorption rolls 6/12; Copper Ring reports 0/0.
These match the supplied screenshots. Source observations remain unchanged.

Backend presentation now recognizes these observed API maps only with evidence
schema/type/count validation and an exact dataset/model/code match. Verified white
mappings currently cover Chinese armor/protector and accessories. Four dataset
option codes have verified labels/scales: MATTR_INT, MATTR_MP, MATTR_SOLID and
MATTR_ER. Other families/options remain explicit gaps; unknown raw values are
preserved without invented presentation. Missing and confirmed-empty options remain
distinct. Presentation is recomputed per observation, never retained by slot.

The API field schema also exposes `phys_def`, `mag_def`, `parry`, `block`,
`critical`, `attack_rate`, `max_durability`, attack/reinforcement/absorption min/max
fields. Plugin 1.2.6 adds those exact field names to bounded raw evidence collection.
Their real values, scaling and enhancement/blue effects still require verification;
this release does not promote them to trusted absolute stats. The collector test
values are synthetic and are not runtime evidence. No game action was performed.

### 2026-09-28 plugin 1.2.6 runtime evidence

Connected phBot 20.1.1 agents report plugin 1.2.6. Their persisted API items
expose 21 field names, including typed physical/magical defense, attack,
reinforcement and absorption values, parry, block, attack rate, critical,
max durability, whites and blues. Live Python Casque and Phoenix Horn Spear
observations corroborate the values listed in
[item-instance-evidence.md](item-instance-evidence.md). The backend now presents
the exact typed scalars, family-matched white percentages and every observed blue
entry after dataset/model/code matching. Missing `whites` does not suppress a
separate scalar. Unfamiliar blue codes retain their literal code, option ID and
raw value; no unobserved blue roll-quality percentage is invented.

The live getters have not reported Advanced elixir eligibility or maximum number
of magic options. `MATTR_REPAIR`, when present, has a verified label, but it must
not be assumed on equipment without that option. The passive packet paths, full
family formula coverage and change/invalidation runtime gate remain open.

### 2026-09-28 Party Setup and inventory-slot contract re-check

Rechecked the official [Party API](https://plugins.phbot.org/phbot-api/party),
[Config API](https://plugins.phbot.org/phbot-api/config), and
[Misc API](https://plugins.phbot.org/phbot-api/misc). They document `get_party()`
for current membership; `get_config_path()` for the active player JSON, with an
explicit warning that direct changes may be overwritten; and `set_profile()` for
profile selection. They do not document Party Setup field names, a supported writer,
profile-reload behavior, or effective-state readback. The adapter's file write and
delayed `reload_profile()` remain an unverified lead. The UI exposes Party Setup as a
separate read-only section, with edits disabled; there is no `party.setup.apply`
command capability.

To enable writes, record evidence from the installed phBot version for: (1) exact
supported party invitation/acceptance, leader-list and party-type fields; (2) an
officially supported mutation path; (3) whether/how it reloads the active profile;
and (4) a readback that proves effective application after reload. The test must run
through the normal session-fenced authenticated command lifecycle and confirm
success only from the effective readback. Until then, keep mutations disabled.

The official [Inventory API](https://plugins.phbot.org/phbot-api/inventory) describes
`get_inventory()` as a flat item list with size, but does not define equipment slot
indices or whether capacity includes them. Current 0–12 separation stays marked
`adapter_lead_runtime_unverified`; `version=296` and the supplied adapter do not
prove it. A phBot-documented slot contract or same-session raw-slot-to-equipment
mapping from independent runtime evidence is required before changing that status.

Party Setup remains read-only/unverified. The first-13 equipment split remains
unverified. For item instance evidence and the Slice 4 visual audit, see
[`item-instance-evidence.md`](item-instance-evidence.md) and
[`reference-parity.md`](reference-parity.md). No character or game traffic was
generated for these checks.

## Slice 5 event-source evidence (2026-09-28)

The public [Events API](https://plugins.phbot.org/phbot-api/events) was rechecked on
2026-09-28. It says `handle_event(t, data)` receives a string `data`; the table below
records the documented values and the independent canonical mapping. Event IDs are
not inferred from third-party snippets.

| ID | Official data meaning | Canonical event | Stored source detail |
| ---: | --- | --- | --- |
| 0 `EVENT_UNIQUE_SPAWN` | Monster name | `world.unique_spawned` | Bounded `value` string |
| 1 `EVENT_HUNTER_SPAWN` | Player name, including traders | `job.hunter_trader_seen` | Bounded `value` string |
| 2 `EVENT_THIEF_SPAWN` | Player name | `job.thief_seen` | Bounded `value` string |
| 3 `EVENT_TRANSPORT_DIED` | Transport ID, including horses | `pet.transport_died` | Bounded `value` string |
| 4 `EVENT_PLAYER_ATTACKING` | Player name | `character.attacked` | Bounded `value` string |
| 5 `EVENT_RARE_DROP` | Equippable item model ID | `drop.rare` | Numeric model ID only |
| 6 `EVENT_ITEM_DROP` | Equippable item model ID | `drop.item` | Numeric model ID only |
| 7 `EVENT_DIED` | Empty string | `character.died` | Cause remains `unknown` |
| 8 `EVENT_ALCHEMY_FINISHED` | Empty string | `alchemy.finished` | Empty payload; no attempt is inferred |
| 9 `EVENT_GM_SPAWNED` | Player name | `world.gm_spawned` | Bounded `value` string |
| 10 `EVENT_LEVEL_UP` | New level | `character.level_up` | Validated integer, 1–255 |

### Level-up callback correction (2026-09-29)

The official [Events API](https://plugins.phbot.org/phbot-api/events) labels
`EVENT_LEVEL_UP` data as the new level. The live Greatest deployment contradicts
that description for phBot 20.1.2 with PhMon plugin 1.5.0 (protocol 7): four
independent callbacks on 2026-09-29 carried `71`, one for each of nuker1–4, while
all four character records and current character views report level 72. Each
character has exactly one level-up occurrence in the stored timeline. The deployed
plugin forwards the callback integer unchanged; the Events UI also displayed it
unchanged. This is evidence for the installed runtime, not a universal phBot API
guarantee.

For agents reporting phBot 20.1.2, the server now maps an unmarked
`phbot.callback` `EVENT_LEVEL_UP` payload to the reached level by adding one,
retaining the original integer as `callback_level`. Other phBot versions keep the
documented interpretation pending runtime evidence.
A payload containing both fields is accepted only when `level` is exactly
`callback_level + 1`, so a future plugin can send the normalized value explicitly.
Migration `000015_level_up_callback_correction.sql` applies the same correction to
prior unmarked occurrences from agents currently recorded as phBot 20.1.2.
Recheck this behavior against future phBot versions before changing the mapping.

The official [Alchemy API](https://plugins.phbot.org/phbot-api/alchemy) documents
`alchemy_update(slot, success, plus)` and says it runs after an elixir is used on an
item. PhMon records one `alchemy.attempt`; it preserves `success` only when Python
returns a boolean and `plus` only when it returns a bounded integer. The page does
not define further type semantics or probabilities, and no callback was naturally
observed during this implementation. Item details are attached only if the current
same-session inventory observation has that callback slot; otherwise the attempt
keeps its slot and callback values without an item identity.

The official [Chat API](https://plugins.phbot.org/chat-api) documents optional
`phBotChat` import and these call signatures: `All(text)`, `Party(text)`,
`Guild(text)`, `Union(text)`, `Stall(text)`, `Private(name, text)`, `Note(name, text)`
and `Global(text)`. The documentation says `True` means the API sent the message and
`False` means sending failed; it does not promise recipient delivery. PhMon imports
the module optionally and advertises only callable `All`, `Private`, `Party`, `Guild`,
`Union` and `Global` methods as per-session `chat.send` modes. Stall and Note do not
have matching Slice 6 conversations, and `Notice`/`ClientNotice` are GM functions.
Global commands require an explicit UI confirmation. The 2,048 UTF-8 byte application
bound is a transport safety limit; phBot's public page does not establish the game's
message or encoding limit.

The Events API documents `handle_chat(t, player, msg)`, identifies `t` as the type
sent by the server, and says `player` may be `None` for non-private messages. It does
not publish a type-to-channel table. During operator testing on phBot 20.1.2, inbound
records were observed on active agents and the operator confirmed numeric types 1
(General/All), 2 (Private), 4 (Party), 5 (Guild) and 6 (Global). PhMon preserves
bounded original text/raw type and available sender data, canonicalizes these
observed values and explicit channel names, and leaves other numbers unknown. This
runtime observation
does not verify outbound `phBotChat` methods, their supported channels or delivery;
those gates remain open.

Lifecycle source details from the same page: `connected()` fires when phBot connects
to the game server; `disconnected()` may fire several times; `joined_game()` runs on
character selection before character data loads; `teleported()` runs on teleport and
right after `joined_game()`; and `event_loop()` runs every 500 ms. PhMon suppresses
repeated connected/disconnected and joined-game state notifications while still
allowing later transitions. Event callbacks only enqueue bounded in-memory records;
the worker performs atomic spool writes and network sends. If no worker exists yet,
a bounded callback queue holds the event until `event_loop()` starts/observes the
worker. Process termination before worker spooling can lose those in-memory entries.

The official [Drops API](https://plugins.phbot.org/phbot-api/drops) documents
`get_drops()` as nearby pickable items keyed by pick ID, with observed name, server
item code, model, region, coordinates, pickability, blue flag and plus. `handle_event`
drop callbacks report only an equippable model ID; no stable pick-ID/time correlation
is documented. PhMon does not query `get_drops()` for these callbacks and does not
invent ground identity or item-instance fields. Ownership events are separately
derived from continuous bag, pet, job-pouch and observed storage quantity snapshots.

Runtime boundary for this implementation: a phBot process was present on the Windows
host, but its native window and plugin callback values were not accessible through the
available browser-only computer-control surface. This run therefore did not confirm
the current installed phBot or plugin version, observe any of IDs 0–10, receive chat
or alchemy callbacks, or validate lifecycle ordering against the running client. The
previous phBot 20.1.1 / plugin 1.1.0 check establishes only the older agent transport;
it is not Slice 5 callback evidence. Keep all listed callbacks and the real-runtime
gate open until naturally observed on a recorded supported runtime. No game action
was performed to generate an event.

No Slice 5 packet decoder is enabled. Existing Slice 4 item packet handling remains
separately constrained by its recorded opcode/profile fixtures and does not supply
event decoding evidence.

### Live named training-area selection check (2026-09-28)

The operator authorized one test on Greatest/nuker1. Its connected agent reported
phBot 20.1.2, PhMon plugin 1.4.2 and protocol 6 with the `training.area.set`
`named` mode supported. Before the command, the current-session control readback
reported region 25735, X 100, Y 1559, Z 0 and radius 20. One uniquely generated
`PhMonMissing_<UUID>` name was submitted through the authenticated, session-scoped
`training.area.set` command. The plugin called `set_training_area(name)` once; the
command finished `failed` with `api_return:false`, `result_code:api_return_false`
and `verification:api_confirmed`. Its immediate `get_training_area()` readback
reported the same region, coordinates and radius. No movement or other bot action
was requested. This is direct evidence that an unknown name did not create or
select a training area on this runtime; it is not a guarantee for every phBot
version. The official [Training Area API](https://plugins.phbot.org/phbot-api/training-area)
documents `set_training_area(name)` as changing the selected area and provides no
creation primitive. The official [training-area guide](https://guide.phbot.org/phbot/training-area)
documents creating a new area through the phBot UI's Add action.

## Slice 7–8 position and monster observation source (2026-09-29)

- The official [Monsters API](https://plugins.phbot.org/phbot-api/monsters)
  documents no-argument `get_monsters()`. It returns `None` or a dictionary that may
  be empty; dictionary keys are monster IDs. The example documents model, type,
  region and x/y along with name/server name and combat fields. PhMon keeps only the
  identifier, model, type, region and coordinates required for map observations.
- The official [Character API](https://plugins.phbot.org/phbot-api/character)
  documents no-argument `get_position()` returning current region and x/y/z, or
  `None`. This supplies observer positions and retains optional observed Z; it does
  not establish a map transform.
- Plugin v1.5.0 / agent protocol v7 polls the getter every ten seconds and limits each
  snapshot to 128 entries. `None`/missing/exception, observed empty and truncated are
  kept distinct. Only complete untruncated snapshots enter the local durable sample
  spool. Sample cadence is one minute per session/region/unmapped-floor/192-unit
  observer cell. The `unmapped` floor label avoids claiming outdoor or cave membership
  without verified region mappings; no real-runtime behavior is inferred from tests.
- Cave member region/floor is not documented by the Academy API. The map reports
  Academy member coordinates unavailable rather than projecting unscoped rows.
- No navigation command is enabled. The official [Paths API](https://plugins.phbot.org/phbot-api/paths)
  rate-limits path generation to one call per five seconds; `generate_script()` can
  produce bounded route text, but its effects and arrival confirmation have not
  been verified on the installed runtime. Coordinate conversion also requires
  a verified reverse transform and command-Z evidence. The active profile has only
  four forward outdoor marker transforms and no command-Z evidence.

Go unit checks cover protocol bounds, empty clearing and expiry; frontend unit checks
cover transform round trips. PostgreSQL integration tests for sample deduplication,
density math and multiple observers are present but were not run because this worktree
has no `TEST_DATABASE_URL`. The real phBot v1.5.0 integration gate remains open.

### Outdoor position display evidence — 2026-09-29

The documented [character getters](https://plugins.phbot.org/phbot-api/character)
return region and X/Y, including examples `(25000, 6428.2373, 1086.6726)` and
`(24744, 6435.8999, 828.8)`. Both agree with 192 displayed coordinate units per
outdoor region and the selected export's root tiles `(168,97)` and `(168,96)`.
Synchronized marker positions on the connected Greatest reference map confirm +X
right and +Y up. This supports forward marker placement for the four explicitly
joined outdoor regions in the current profile. It does not supply command Z,
dedicated cave transforms, or real phBot navigation evidence; the command gate
above remains closed.

### Map monster presentation fields — 2026-09-29

The documented `get_monsters()` response exposes each nearby monster's `name`,
`servername`, `model`, numeric `type`, `region`, X/Y, `hp`, `max_hp` and `attacking`.
Plugin 1.5.1 carries bounded optional name, numeric type, HP and attacking values
through the existing v7 `map.monsters` frame. Go validates them before replacing
the current snapshot. Older agents remain compatible, but their absent fields
cannot be reconstructed from a map screenshot or the stored observation rows.
The 1.5.1 frame and rendering path passed plugin, Go and Nuxt checks; the installed
real phBot runtime has not yet been verified with 1.5.1.
