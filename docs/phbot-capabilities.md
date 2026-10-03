# phBot capability evidence

Slice 1 starts the real phBot integration. This document records only capabilities
verified from public phBot plugin documentation or an actual runtime. Simulator
coverage is tracked separately and is never treated as proof of a real phBot run.

## Issue #36 — manual Players API runtime probe (2026-10-01)

The official [Players API](https://plugins.phbot.org/phbot-api/players), rechecked
on 2026-10-01, explicitly says `get_players()` is disabled. Its documented return
is `None` or a possibly empty dictionary keyed by player ID, with name, guild,
grant, items, X/Y and dead fields. Region and Z are not documented for players.
The official [Client API](https://plugins.phbot.org/phbot-api/client) documents
`get_client()` returning `None` or a dictionary with a boolean `running` field;
this is process state, not proof of a logged-in clientless session. The official
[Misc API](https://plugins.phbot.org/phbot-api/misc) documents `get_version()` as
the bot version string. The [GUI API](https://plugins.phbot.org/gui-api) verifies
`createButton` with a named callback and `setText` for the local result label.

Plugin **1.7.2**, agent protocol **9**, adds **Test get_players** to its existing
QtBind tab. One operator click imports/probes the actual `phBot` module and calls
`get_players()` at most once; clicks are throttled to two seconds. The bounded
local JSON log records module/symbol/callable availability, result classification,
native call duration, selected session/client/observer context and at most three
sanitized samples. It distinguishes `None` from `{}`, handles exceptions without
logging their messages, inspects at most 128 entries and labels overflow explicitly.
Only selected first-entry field types are included for structural evidence;
equipment and unrelated field values are excluded. No native API call is moved
to an unverified background thread, and no network/disk I/O is added to the probe.

Validation: **131 Python plugin tests** pass, including nine new diagnostic tests
for absent/missing/noncallable/disabled/empty/populated/unexpected API behavior,
malformed coordinates/entries, overflow and log bounds, import probing, unknown
client state and the backend-independent throttled button. These are CPython
fixtures, not installed phBot observations. Subsequent operator runtime evidence
and the corrected diagnostic are recorded below.
See [operator instructions](../plugin/README.md#test-the-players-api-issue-36).

### Operator runtime result and ID correction (2026-10-01)

Operator-provided plugin 1.7.2 log at **21:15:21 UTC / 23:15:21 Amsterdam**:

| Evidence | Observed result |
| --- | --- |
| phBot | **20.1.2**, `phbot_importable: true` |
| Players symbol | Present and callable |
| Native call | Returned in **19 ms** |
| Return | `dict`, **10 entries**, no truncation |
| Client/session | `client_running: true`, character data available, joined-game true |
| First entry | String ID key; dict with string name/guild/grant, boolean dead, integer region, float X/Y |
| Samples | None: ten entries rejected; the observed string-key shape fails 1.7.2's integer-only validator |

This establishes that the API returns populated data on this observed runtime with
the client running despite the documentation's disabled notice. It does not verify
clientless behavior, player coordinate values, cross-observer ID identity or cave
placement. `game_connected_callback: null` is unknown callback state after plugin
reload, not evidence of a disconnected game. The first entry exposes a `region`
field absent from the official example; its value and semantics remain unverified.
No player Z field appeared in the first-entry type evidence.

Plugin **1.7.3** fixes sampling by accepting nonblank string IDs up to 64 characters
and preserving them exactly, alongside the existing bounded integer IDs. No decimal
format is assumed before seeing actual values. Samples now retain valid per-player
region and optional finite bounded Z only when present on that entry; the observer's
region/Z are never substituted. All **133 plugin tests** pass, including two new
tests for the observed string-key shape and identifier bounds. Test positions/IDs
are synthetic; real player sightings are not persisted as test fixtures.

### Client-closed runtime result and equipment follow-up (2026-10-01)

The operator supplied a second log at **21:20:27 UTC / 23:20:27 Amsterdam**, from
phBot **20.1.2**, plugin **1.7.3**, on server **Greatest**. The API returned a
dictionary of **nine entries in 16 ms**, all nine valid, no truncation.
`client_running: false`, `character_data_available: true` and
`joined_game_callback: true` establish an observed joined session with no running
client. This is runtime evidence that the Players source is usable in that
clientless state, not merely a fixture or an inference from client process state
alone. The connection callback remained unknown after reload.

All three sanitized samples have decimal-string IDs, string name/guild/grant,
boolean dead, integer region **26244** and finite float X/Y. That region matches
the current observer in this observation; no player Z is supplied. This does not
establish cross-observer ID equivalence, cross-region behavior or cave-floor safety.
No `items` field appeared in the first entry's selected field-type evidence.
Other nearby players' equipment availability is therefore still unverified.

The operator then explicitly requested armor/weapon inspection. The official
[Players API](https://plugins.phbot.org/phbot-api/players), rechecked on 2026-10-01,
shows an `items` list whose examples include armor/weapons, with name, degree,
model, servername, level and plus. This example is a documented lead; it does not
prove the current runtime populates equipment. No equipment getter or request is
invented, and no list-index-to-equipment-slot mapping is assumed.

Plugin **1.7.4**, protocol **9**, adds a separate **Inspect player equipment**
button with an optional exact ID or case-insensitive name target. It reuses the
same bounded, manual Players getter. The general probe continues to omit items.
Equipment samples contain at most three players, selected field names/types
through the existing 2 KiB structural evidence helper, and at most 32 item
entries / 8 KiB equipment evidence per player. Only documented item fields and
source list index are copied. Missing/None/empty/unexpected/malformed/partial/
truncated states remain distinct; equipment is never transported or persisted.
One summary and one local log line per player keep results easy to copy.

Validation: **138 plugin tests** pass, including five new equipment tests for
availability distinctions, field preservation, target selection, unknown-value
exclusion, item/byte bounds and the backend-independent Qt callback. Subsequent
native results are recorded below. Map placement and identity rules remain
separate gates before issue #36 transport/UI implementation.

### Equipment runtime result: field absent (2026-10-01)

Operator-provided phBot **20.1.2** / plugin **1.7.4** logs establish two distinct
results with the client closed and the character joined:

- **21:29:23 UTC / 23:29:23 Amsterdam**, observer region 25733: `get_players()`
  returned `{}` in 0 ms, with zero entries/samples. Target matching occurs after
  the native call, so the requested target did not cause the empty dictionary.
  This response tests observed-empty player discovery, not equipment availability.
- **21:31:16 UTC / 23:31:16 Amsterdam**, observer region 26244: the getter returned
  ten valid entries in 18 ms. Name targeting matched one player and produced one
  sample, with no truncation. The sample's complete field evidence has exactly
  eight fields: `name`, `guild`, `grant`, `level`, `dead`, `region`, `x`, `y`.
  `field_types_truncated: false` establishes that the absence of equipment fields
  is not a structural logging limit. `items` is absent, and equipment reports
  `availability: unavailable`, `reason: items_missing`.

The sampled runtime record cannot supply that player's armor or weapon. There is
no alternate equipment field in this complete eight-field shape. This is a verified
source limitation for the observed player/clientless session, not proof about every
player, game mode or other phBot version. Integer `level` is now observed as a field
type; its value was not copied by this diagnostic. No item model/code/plus can be
derived from the supplied player record. Later XP/SP and gold log messages are
unrelated to this capability result.

The requested investigation is complete for this response. Equipment inspection
remains blocked on a documented/observed source exposing item data. The existing
probe can compare additional players/client-running state without a plugin update.
No guessed API or packet fallback is added. This document retains capability
metadata only, not player identities, equipment records or sighting history.

### Production map.players transport (plugin 1.8.0, protocol 10)

Plugin **1.8.0** removes the manual probe buttons and publishes bounded
`map.players` snapshots on the existing worker path (operator-tuned **1 s** poll,
**2 s** unchanged refresh; signature includes observer Z). Rows copy canonical decimal-string IDs,
name, guild, grant, dead, level, region, X/Y, and optional `zone` from
`get_zone_name` for that player's region (observer region when the row has none).
Equipment and player Z are omitted. The Go backend keeps ephemeral per-session snapshots with 35 s TTL,
generation-scoped disconnect cleanup, and map projection with party-style dedup.
The PhMon Map **Other players** layer is fixture-tested separately from installed
phBot validation; cross-observer identity and cave placement semantics remain
open gates until recorded on a live runtime after upgrade.

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

## Issue #23 live party map evidence (2026-09-30)

The official [Party API](https://plugins.phbot.org/phbot-api/party) documents
`get_party()` as `None` or an object keyed by party ID; an empty object is a
valid observed-empty party. `player_id` remains zero until that member spawns
near the observer, and Joymax HP/MP percentages are 0–10. PhMon's existing
resource collector remains the only party observation source. It keeps party
membership in the resource snapshot/delta path, normalizes HP/MP to 0–100, and
retains only finite bounded X/Y coordinates.

Issue #23 projects those canonical current resource observations into the
existing live map. The server fences each resource row against both
`character_resource_state.session_id` and the still-open
`character_sessions.session_id`; unavailable rows never reuse their retained
last-known payload for live party markers. Unchanged party data is not expired
only because its resource timestamp is old, since resource deltas intentionally
omit unchanged payloads. Party member region and Z remain unavailable from the
API. The observer's region/Z are carried only as live observation scope and must
come from fresh character state (the map's existing 35-second freshness window,
with its 5-second future-skew allowance). Cave floors fail closed when that scope
cannot be proven, and the frontend still uses the existing world-to-raster
transform for final placement.

No party-position history, packet fallback, second poller, party control action,
new WebSocket, or new persistence table is introduced. Automated plugin,
PostgreSQL session-fencing, server projection and frontend transform tests are
part of the implementation. Actual phBot runtime/browser validation of the new
party layer remains a separate gate until performed on an authorized runtime.

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
- Guild-storage gold is read from `get_guild_storage()` when the loaded phBot
  runtime returns a valid non-negative integer `gold` field. The public storage API
  documentation lists only `size` and `items`, but the operator-confirmed runtime
  exposes `gold` on this getter; PhMon preserves that observed API value instead of
  discarding it. Boolean, negative, non-integer, or missing values remain omitted.
  Guild-storage gold does not use a Joymax packet fallback; the passive packet decoder
  remains limited to item-instance evidence that is unavailable from verified API
  fields.
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

PhMon uses this function on the phBot callback thread to attach an optional zone
name to observed event positions and training-area readback. A missing result or
lookup exception leaves the name absent while preserving the numeric region and
coordinates; historical event names are not inferred. The API documentation does
not specify behavior for unsupported/custom region IDs, so those remain unnamed.
- [Botting](https://plugins.phbot.org/phbot-api/botting) documents `start_bot()` and
  `stop_bot()` mutations but no read-only botting/training-state getter. Slice 2
  initially reported this field as unknown rather than inferring state from
  commands or UI. Issue #35 later added guarded readback; see the current status
  update below.

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
and unit-tested, but not isolated as a manual runtime scenario. Issue #35 later
added guarded botting-state readback from a boolean character-data field or a
narrowly recognized optional status value. `stopped` and `None` remain unknown
until their meaning is verified on a supported phBot runtime.

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

Training readback exposes typed region/x/y/z/radius values and an optional
`training_zone` derived with phBot's documented `get_zone_name(region)` function.
The training-area name is separate from the character's current zone because a
configured area can be elsewhere.
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
commands. Return-scroll never claims teleport completion and disconnect never
claims relog was disabled. The public Botting API still documents no state getter;
Issue #35 uses the character-data boolean and a narrow optional status fallback,
with unknown values remaining unknown (see the current status update below).

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

The installed Greatest agents produced GM, hunter and thief callbacks, and zero
real `EVENT_UNIQUE_SPAWN` rows. phMonitor v0.5.0 records the same occurrences
from Joymax opcode `0x300C`: byte 0 is 5 for a spawn or 6 for a kill, and bytes
2–5 are the little-endian model ID. A kill may then carry a little-endian name
length and killer text. Plugin 1.9.12 queues that notice without the observer's
coordinates. The server resolves the model through `server/game-data/unique-monsters.json`
and the Events and Dashboard views use the matching file under
`/game-assets/monsters/`. Catalog HP is not in the exported monster reference,
so the card omits HP instead of inventing it. Notice chat remains a separate
phMonitor source and is not used here: stored type-7 messages on this server
are ordinary notices, not unique spawn lines.
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
- Plugin v1.5.4 / agent protocol v7 polls the getter every 0.1 seconds and limits each
  snapshot to 128 entries. `None`/missing/exception, observed empty and truncated are
  kept distinct. Only complete untruncated snapshots enter the local durable sample
  spool. Sample cadence is one minute per session/region/unmapped-floor/192-unit
  observer cell. The `unmapped` floor label avoids claiming outdoor or cave membership
  without verified region mappings; no real-runtime behavior is inferred from tests.
- Cave monster reports use the observing character's signed region and current Z
  when a monster omits Z. Plugin 1.5.4 preserves nonzero region IDs in
  `-32768..65535`; live Donwhang map evidence showed the `-32767` observer region
  and current monster coordinates. The server classifies the observation's floor
  from the observer's Z, then uses that Z only as the cave marker's floor context.
- Cave member region/floor is not documented by the Academy API. The map reports
  Academy member coordinates unavailable rather than projecting unscoped rows.
- Earlier outdoor-only verification had no navigation action because it lacked
  cave transforms and command-Z evidence. The later cave implementation below
  adds selected-point navigation using the executable's observed current-Z/zero
  rule. The official [Paths API](https://plugins.phbot.org/phbot-api/paths)
  rate-limits path generation to one call per five seconds; `generate_script()`
  produces bounded route text, but real-runtime route execution and arrival remain
  unverified.

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
joined outdoor regions in the profile at that time. This outdoor observation did
not supply command Z or real phBot navigation evidence; cave transforms are
documented separately below.

### Cave maps and reference-style point actions (2026-09-29)

Static inspection of the operator-authorized `phMonitor-v0.5.0.exe` found all
three cave families and the reference's 2D tile anchors. Its tile size is 192
world units. The cave tile coordinate maps X/Y only; there is no terrain-height
lookup. phMonitor's map action reuses the selected character's current Z when
converting a clicked destination, including a different floor, and initializes
Z to zero when unavailable. PhMon follows this 2D behavior and never calibrates
per-pixel height.

- GreatestSRO `Media.pk2` contains cave DDJ tiles for all 17 floors (6 Tomb, 4
  Donwhang Stone Cave, 7 Job Temple). The exporter now publishes only converted
  PNGs under local `game-assets/minimap_d/` paths and a separate `caveMaps`
  catalog; archives and exporter audit remain outside the app assets.
- Tomb floors have distinct observed regions -32761 through -32766. Donwhang
  uses observed region IDs -32767 and 32767 with Z bands -50..70, 71..210,
  211..350 and 351..490. The one observed Job Temple region (-32752) is shared;
  region alone classifies 1F only. Higher floors require explicit map selection.
- The official [Paths API](https://plugins.phbot.org/phbot-api/paths) documents
  `generate_script(region,x,y,z)` as returning route strings, including waits and
  teleports; it can return `None` for no route or `False` when rate-limited/not
  in game, with a five-second generation limit. The official [Script API](https://plugins.phbot.org/phbot-api/script) documents `start_script(str)` as
  background execution. PhMon's typed navigation command accepts only bounded
  generated `walk`, `wait` and `teleport` lines, reports the phBot return, and is
  enabled only when both runtime functions are reported. A completed command
  means phBot accepted the script; it is not an arrival claim.
- Point navigation and training-area positioning use the authenticated command
  lifecycle's selected character/session. Cave region ambiguity disables the
  action. Explicit signed cave regions are preserved, including cross-floor
  selection. Session freshness remains mandatory.

Observed live reference sample, kept separate from the stale PhMon observation:
the operator teleported nuker1 to Donwhang Stone Cave. phBot v20.1.2 showed
X=-24272.5, Y=-93.5; phMonitor v0.5.0 placed the marker on its 1F image at
approximately (-24273.0,-93.5), Z=0.0. Switching to its 2F image changed the
floor raster and hid the 1F marker. PhMon still showed nuker1 at a prior Hotan
position, observed at 2026-09-29 15:45:25Z, so that PhMon map observation was
stale and is not used as Donwhang validation. No command was sent to the live
character. The deterministic adapter test covers signed region navigation and
phBot rejection/acceptance; the installed real runtime has not run this command.

### Map monster presentation fields — 2026-09-29

The documented `get_monsters()` response exposes each nearby monster's `name`,
`servername`, `model`, numeric `type`, `region`, X/Y, `hp`, `max_hp` and `attacking`;
it does not document a level field. Plugin 1.5.1 carries bounded optional name,
numeric type, HP and attacking values, and 1.5.2 additionally forwards a level only
if supplied by the runtime. Go validates these fields in the existing v7
`map.monsters` frame. The map never treats a numeric type/model code as a level and
shows level unavailable when absent. Older agents remain compatible, but their
absent fields cannot be reconstructed from a map screenshot or stored rows. Current
map sightings are deduplicated by normalized server and monster ID. The operator's
2026-10-02 executable investigation verified that phMonitor uses this identity;
it supersedes the earlier model/name and 8-unit position matching rule. The
documented dictionary key is the monster's ID; real cross-observer and instanced
area behavior remains a runtime validation gate. HP/max HP remain raw fields
from one chosen snapshot, with no estimated or minimum-held health. See the
[HP investigation](reference/monster-hp-investigation.md). The installed
real phBot runtime has not yet been verified with 1.5.2 or a level field.

### Live NPC and teleporter snapshots — 2026-10-01

The official [NPC API](https://plugins.phbot.org/phbot-api/npc) documents
`get_npcs()` as `None` or a dictionary keyed by a runtime NPC id. Each value has
`name`, `servername`, `model`, `region`, `x`, and `y`. It does not document a type
or Z. The published example uses `GATE_CH` for the Jangan teleporter and `NPC_*`
server names for shops. Plugin 1.7.0 / protocol v9 samples this API about every two
seconds, immediately after `teleported()` or a region change, and at least every 15
seconds while the normalized snapshot is unchanged. A snapshot holds at most 128
rows. A missing API, `None`, a non-dictionary, or an exception is `unavailable` and
clears that character's markers. An empty dictionary is `observed` and clears them.
`GATE_<name>` is presented as a teleporter; every other row is an NPC. The map
stores these snapshots only in memory. Shop goods and teleport execution stay out
of this frame. The installed phBot runtime has not yet been observed with 1.7.0.

### Current map marker polling trial — 2026-09-29

The operator requested faster current monster snapshots because the previous
ten-second delay made combat markers too slow. `MOB_POLL_INTERVAL_SECONDS` is now
0.1; the durable observation spool still samples at most once per minute per
observer cell. The focused plugin suite passes (86 tests), including a regression
that verifies polling is skipped at 0.099 seconds and resumes at 0.1 seconds. This is
source/test evidence only: the updated plugin must be loaded by the connected phBot
runtime before actual CPU cost and map freshness can be measured.

### Teleport sampling and outdoor region placement — 2026-09-29

During a Hotan teleport, the agent heartbeat continued while character state stopped
advancing. A repeated plugin `connected()` callback cleared `_character_joined` even
for an existing connection, preventing position sampling until `joined_game()` or
a plugin reload. The callback now clears that flag only for a new connection; a
focused test covers the repeated-callback case. This is a plausible cause, not yet
confirmed on the installed phBot copy. After service recovery, a fresh Greatest
observation in region 26520 had X/Y near `(3423.1,2115.2)`, consistent with
outdoor tile `(152,103)`. The map uses `region = tileY*256 + tileX` as its primary
outdoor placement rule with 192 coordinate units per tile. Cave handling is separate.

### Slice 9 historical heatmap capability boundary — 2026-09-29

Slice 9 does not add a phBot protocol or plugin requirement for player movement.
The backend already receives authenticated, session-fenced `character.snapshot` /
`character.state` frames containing the documented region/X/Y position. After the
canonical character update succeeds, the server records a separate bounded movement
sample at most once every two seconds and only after a region transition or at least
four horizontal game units of movement. Analytics persistence is secondary to the
canonical character state and cannot make a successful character update fail.

No verified phBot API/runtime source establishes the spatial footprint covered by a
`get_monsters()` snapshot. Monster coordinates prove only that those monsters were
returned; they do not prove which surrounding cells were observable. Therefore the
new historical API keeps true `mob_density` explicitly unsupported with reason
`observation_coverage_unverified`. The existing observer-cell calculation is exposed
only as `mob_observer_average`: returned monster rows divided by eligible complete
samples whose observer stood in that cell. `mob_types` is a separate historical
sighting-count layer located at the monsters' reported coordinates and is likewise
not described as density.

The active map profile still lacks validated cave-floor imagery/transforms for Tomb
of Qin-Shi, Donwhang Stone Cave and Job Temple. Historical heatmap queries for an
area/floor without a usable transform return an unsupported state rather than
projecting the coordinates through the outdoor transform. This preserves the same
fail-closed coordinate boundary used by Slice 7.

### Protocol v8 generated-script route reporting — 2026-09-30

The official [Paths API](https://plugins.phbot.org/phbot-api/paths) documents
`generate_script(region, x, y, z)` as generating route command strings and imposing
a five-second rate limit. Its generated paths can include `walk`, `wait`, and
teleport instructions. The official [Script API](https://plugins.phbot.org/phbot-api/script)
documents `start_script(str)` as background execution and `stop_script()` as a
stop operation; neither document exposes a script-progress getter or proves that
the character arrived at the requested point.

Plugin 1.6.0 parses the generated result once, bounds and normalizes the supported
instruction grammar, and passes the exact validated text to `start_script`. An
explicit `False` return means failed start; other existing return classifications
remain unchanged. The plugin publishes bounded normalized route evidence only after
the call does not explicitly fail. It does not include script source or teleporter
identifiers. The Go server owns route admission against the completed durable
`character.navigate` command and keeps progress transiently, separate from position
history. Arrival is inferred only from a later accepted live position observation
within 12 game units and compatible region/floor scope; that radius is a PhMon map
presentation policy, not a phBot guarantee.

This contract has deterministic fake-adapter and protocol tests, including exact
script execution, invalid-route rejection, background acceptance versus observed
arrival, and wait/teleport barriers. There is no supported Windows/phBot process in
this environment, so plugin v1.6.0 has not been exercised against a real character;
the simulator is fixture evidence only. Protocol v8 is additive and the backend
continues accepting v2–v7 during rollout. Legacy agents cannot report route geometry.


### Issue #27 live follow-up — 2026-09-30

The operator-authorized Hotan checks used fresh PhMon 1.6.0/protocol 8 sessions
reported on phBot 20.1.2. Generated-script invocation returned true for a 12-step
nuker4 path across outdoor regions 23687/23686. Remaining geometry advanced in
one block, and a fresh compatible position established arrival after durable
command completion. Group checks showed simultaneous advancing routes with
independent completion/arrival and `path_not_found` results. The operator also
reported a ten-second `event_loop` warning. Those API failures/delays have no
isolated cause established here; early tests overlapped operator teleports and
preceded deployment confirmation. Preserve them as runtime limitations, not
proof of a particular defect.

The official [Paths API](https://plugins.phbot.org/phbot-api/paths) documents a
five-second generation limit and None/False failure meanings. The official
[Script API](https://plugins.phbot.org/phbot-api/script) documents background
execution but provides no arrival or progress getter. This verification adds
observational evidence and does not invent another execution API. Details and
sanitized timings: [runtime ledger](reference/issue27-navigation-runtime.md).

## Callback watchdog investigation — 2026-09-30

The operator explicitly requested investigation of the ten-second `event_loop`
warning. Navigation currently invokes both path generation and script start on
the callback thread; accepted-command timings do not separate those APIs from
sampling or queue delay. Plugin 1.6.1 logs each navigation stage before/after its
call and reports the total/four slowest callback stages whenever a callback takes
at least 500 ms. Tests cover success, False/None returns, exceptions, redaction,
fast-callback silence and timing-report execution after an exception. Logs contain
no script text, API arguments or exception details.

The official [Events API](https://plugins.phbot.org/phbot-api/events) says the
callback runs every 500 ms. The [script command documentation](https://plugins.phbot.org/handling-script-commands)
explains interpreter locking and why sleeping inside callbacks blocks other
callbacks. The Paths/Script contracts do not document thread safety for
`generate_script`/`start_script`. PhMon therefore retains callback invocation while
collecting runtime evidence; moving those calls to a thread would require further
verification. The warning is not resolved merely by these diagnostics. A fresh
operator-installed 1.6.1 log is required to isolate the stage before a corrective
change can be verified.

### Confirmed callback stall and bounded generation — 2026-09-30

Operator-supplied 1.6.1 timing logs isolate `generate_script` on nuker1/nuker2
at 8077/8124 ms. Validation and source readback took 0 ms; `start_script` took
3/2 ms. Total callback times were 8086/8131 ms. nuker4 generation took 577 ms,
script start 4 ms and callback total 589 ms. Thus synchronous path generation in
our callback dispatch is the confirmed blocking stage. The generation latency
itself remains native API behavior, not a diagnosed remote-service failure.

Plugin 1.6.2 makes a narrow exception to the older callback-only API plan: only
`generate_script` runs on a dedicated bounded daemon thread. One generation slot
is shared across profile workers in a plugin instance. Transport stays separate,
and all validation, position reads and script mutations remain callback-owned.
Expiry, current identity/profile, session and generation epoch are checked again
before invocation. Teleport, disconnect, revocation and stop discard late results;
no callback joins or waits on a generator. Tests exercise a deliberately blocked
generator, continued sampling/result flushes, API thread identity, duplicates,
invalid results, lifecycle rejection and slot bounds across worker replacement.

Official docs do not promise native generation thread safety or GIL behavior.
The installed-phBot gate is therefore explicit: load 1.6.2, verify generation
completes while fresh position sampling continues, verify script start/arrival,
and check that no ten-second callback warning returns. Do not claim this runtime
gate passed based only on Python fixture threads. Exact next action: push 1.6.2
for operator installation and inspect its callback/position evidence, then finish
final-head CI and CodeRabbit without merging PR #50.

## Issue #35 multi-character control mapping — 2026-10-01

The Issue #35 browser panel reuses the existing audited `POST /api/commands`
catalog and each current session's reported capability frame. It adds no plugin
primitive, wire command, protocol version or backend batch operation. `bot.start`,
`bot.stop`, `trace.start`, `trace.stop`, `character.return`, and
`character.disconnect` keep the signatures and result semantics in the Slice 3
matrix above. `character.disconnect` returns `None`; PhMon reports invocation as
unverified and does not infer that the character went offline. Return, Disconnect
and Clientless requests retain `confirmation: true` even when the browser-local
optional review preference is off.

`training.area.set` exposes only `current_position` and `named` in this panel.
`current_position` requires the exact reported mode and sends only that mode; the
production worker reads `get_position()` when it executes. `named` calls the
documented `set_training_area(name)` and remains independent of current active-area
readback; the operator supplies a profile-local name. Both current-position and
radius eligibility use only a matching-session `get_training_area()` readback to
explain a reported unavailable area. A missing readback is not treated as proof of
an absent area or its coordinates. The separate radius operation requires the
documented getter and setter and retains its post-call readback.

The documented Client API has `get_client()` but no safe per-session Clientless
mutation. The runtime continues to report `client.clientless` as
`unsupported_runtime_primitive`; the panel displays that reason for every target
and sends no mutation. It does not inspect or terminate client processes. Official
source pages: [Botting](https://plugins.phbot.org/phbot-api/botting), [Training
Area](https://plugins.phbot.org/phbot-api/training-area), [Misc](https://plugins.phbot.org/phbot-api/misc), and [Client](https://plugins.phbot.org/phbot-api/client).

The Issue #35 deterministic `remote-controls` fixture uses the production
`PhMon.py` worker with local fake adapters for bot/trace, return, disconnect and
training operations. It can omit primitives, report no active area, vary each
worker's position, return false independently, omit configured training modes,
and replace a character session on a chosen adapter-call count. Fake Disconnect
records a local call and returns `None`; fixture Clientless remains unsupported.
The browser eligibility preview labels a missing or mismatched training readback
as unconfirmed and blocks current-position/radius only when a current readback
reports no active area. Fixture outcomes verify transport and result handling
only, not Windows/phBot API effects.

Plugin 1.7.1 publishes botting state in the existing `CharacterView.botting`
field without changing the generated monitor output contract or agent protocol.
It prefers `get_character_data()['botting']` when it is a boolean.
Otherwise, the optional and undocumented `get_status()` fallback accepts only
`botting`/`training` as true and `tracing` as false. Unknown values, errors,
unavailable status, `stopped` and `None` remain unknown because the available
phBot 20.1.1 runtime evidence did not verify their semantics. Start skips a target
only when its latest observed state is true; Stop skips only when it is false.
Unknown state remains eligible under normal session/capability rules, and the UI
waits for the next observed state instead of updating optimistically. Verify
`stopped` and `None` against a supported phBot runtime before mapping either to
false.

An operator-authorized live check on 2026-10-01 observed four online Greatest
sessions with matching control/readback session IDs, `botting: true`, an available
active training area and radius 34. Two Zerkroad character records were offline.
The Client All selection previewed Start as 0 eligible/6 skipped and Stop as
4 eligible/2 skipped. A reviewed `training.radius.set` request using each target's
already-observed value 34 completed on all four online sessions with `observed`
verification; subsequent readback remained 34. Return Scroll and Disconnect
confirmation previews each showed four eligible targets and two offline skips and
were cancelled without submission. Clientless remained capability-blocked. The
view does not expose plugin version, and no Return Scroll, Disconnect, trace or
`training.area.set` mutation was submitted during this check.

The live numeric-radius form exposed a frontend-only type issue: Vue provided the
number input model as a number while validation assumed a string. Validation now
normalizes either representation before trimming/parsing; the added regression
test covers numeric input. This does not change the protocol or plugin contract.

### Issue #57 navigation stop and trace activity — plugin 1.9.0 / protocol 11

`character.navigate.stop` uses documented `stop_script()` with no script id or
return contract beyond boolean/unknown. PhMon tracks the last started navigation
`(command_id, route_sequence)` token and refuses the call when it no longer
matches. If the operator starts a different script in phBot after ours,
`stop_script()` can only stop whatever is current; PhMon still refuses when its
token does not match.

Optional `get_status()` text maps narrowly to `activity_state`: `tracing`,
`botting`/`training` → `not_tracing`, everything else → `unknown`. It does not
return a trace target name. `trace_requested_name` comes only from the admitted
`trace.start` argument until session replacement or completed `trace.stop`.

There is no documented navigation progress or ETA API; PhMon derives progress from
its observation cursor and approximate ETA from recent accepted movement on the
active route.

### Issue #32 teleporter investigation — plugin 1.9.2 probe — 2026-10-02

**Status:** [#32](https://github.com/brantje/phmon/issues/32) investigation complete; [#33](https://github.com/brantje/phmon/issues/33) **character.teleport** fan-out and map **Teleport to…** UI implemented in plugin **1.9.4** (no packet injection, no destination menu enumeration).

**Live gate identity:** Documented `get_npcs()` per session; `GATE_*` → teleporter; runtime id is the API dictionary key. Already shipped as protocol v9 `map.npcs` (issue #24).

**Destination enumeration:** **Unsupported.** Public phBot docs expose only `get_teleport_data(source, destination)` for a **known** pair. Inspected community plugins ([xControl](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xControl.py), [EnterVicious](https://github.com/Bunker141/Phbot-Plugins/blob/master/EnterVicious.py), [xNPC](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xNPC.py)) do not list a gate’s menu. phBot’s [map guide](https://guide.phbot.org/phbot/map) shows grouped destinations in the client UI without a matching plugin API. Exporter `teleportdata` / `teleportlink` and phBot locale SQLite `teleport` tables are static client data and are **rejected** as PhMon menus (issue #33 also forbids a persistent catalog).

**Pair resolution:** `get_teleport_data` with source from the character’s own gate `name` or `servername` and an explicit destination label. `None` → no route (including custom servers per [forum evidence](https://forum.projecthax.com/t/get-teleport-data-does-not-return-anything/7854)). Tuple element `1` is the reference teleport id used by community `0x705A` type-2 packets ([SilkroadDoc](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/AGENT_TELEPORT_USE)).

**Execution plan for #33:** After live-gate and pair checks, one plugin-built script line `teleport,{source},{destination}` passed to `start_script` (documented [script command](https://guide.phbot.org/phbot/script-commands)). Community flow uses `0x7045` select then `0x705A`; PhMon does **not** adopt that injection path. **Live evidence (2026-10-02, plugin 1.9.3):** operator **Test Hotan→Jangan** at `GATE_KT` — `get_teleport_data` code `1`, `start_script=True`, phBot log `Script: Teleporting` for `teleport,Hotan,Jangan`. Other pairs/servers and PhMon remote commands remain unimplemented.

**Designate Recall Point:** **Unsupported.** Community candidate: `inject_joymax(0x7059, struct.pack('I', npc_uid))` after name match in `get_npcs()` ([xControl](https://github.com/JellyBitz/phBot-xPlugins/blob/master/xControl.py)). Script command `recall` is pick-pet only. No authorized PhMon packet capture in this spike.

**Operator probe:** Plugin **1.9.2** QtBind **Probe teleporters** calls `probe_teleporter_capabilities()` — symbol name discovery, at most 16 read-only `get_teleport_data` tests, no `inject_joymax` / `start_script`. Full report: [issue32-teleporter-investigation.md](reference/issue32-teleporter-investigation.md).

**Live probe (2026-10-02):** Operator ran **Probe teleporters** on plugin 1.9.2 at the Hotan gate (`GATE_KT`, runtime npc id `4`). Symbol scan found no destination-menu or recall API. Early probe JSON lacked tagged `Hotan`→`Jangan` pairs (added in 1.9.3). See [issue32-teleporter-investigation.md](reference/issue32-teleporter-investigation.md) § Operator live probe.

**Live script test (2026-10-02, plugin 1.9.3):** **Test Hotan→Jangan** — `teleport,Hotan,Jangan`, `get_teleport_data` code `1`, `start_script=True`, phBot `Script: Teleporting`. Documented in investigation doc § Operator script test.

**Simulator/runtime:** Plugin unit tests cover the probe in CI; Hotan→Jangan script path verified on operator phBot for one pair; #33 remote/fan-out not built.

### Issue #65 monster levels and client reference data — 2026-10-03

The official phBot [`get_monsters()` response](https://plugins.phbot.org/phbot-api/monsters)
documents nearby monster ID, model/type/name/position and combat state, but does
not promise a level field. Existing protocol 13 therefore needs no new plugin
request or version bump. Go accepts a valid runtime `level` if one is actually
reported; otherwise it resolves by model and matching code within the observation
sample's dataset. A conflicting model/code pair stays unknown. Runtime validation
on a real phBot process remains outstanding; fixture/API tests do not establish
that this phBot version reports levels.

`npcpos.txt` and `worldmapguidedata*.txt` are static client export inputs, not
phBot observations. Loading their catalog creates no observation row and does not
assert current spawn presence. The installed GreatestSRO export
`gamedata-e184cceb0b359ca140c1` has 6,756 joined points; 6,451 pass the
supported map transforms. The 305 excluded points include unsupported interior
regions and 46 Donwhang points that fail bounds/floor placement. Job Temple's
shared-region floor ambiguity remains a placement blocker. No plugin or live bot
was operated for this issue.

## Issue #34 — Reverse return (2026-10-02)

Source: official [Inventory API](https://plugins.phbot.org/phbot-api/inventory),
rechecked for this implementation; `reverse_return(type, name)` documents mode 0
(last Return Scroll location), 1 (last death), 2 (party player name) and 3
(location name). True means a scroll was used; false means no usable scroll.
The official [Pets API](https://plugins.phbot.org/phbot-api/pets) example
identifies model 3795 as `ITEM_MALL_REVERSE_RETURN_SCROLL`. The example is
item-code evidence; pet contents are not evidence of usable character inventory.
The official [Party API](https://plugins.phbot.org/phbot-api/party) documents
`get_party()` returning a dictionary of party members including their names.
The operator supplied `web/public/game-assets/textdata/refoptionalteleport.txt`
as the type-3 destination source. Its 19-column layout is cross-checked against
[RSBot's RefOptionalTeleport definition](https://github.com/myildirimofficial/RSBot/blob/master/Library/RSBot.Core/Client/ReferenceObjects/RefOptionalTeleport.cs).
Enabled rows (service 1) use zero-based column 3 as the exact localization key in
`textdata_object.txt`, English column 8 (also zero-based). The Greatest archive has 42 rows: four
disabled rows and two unresolved `xxx` keys are excluded, leaving 36 unique names.
The `ObjName128` column contains replacement question marks and is not an API name.
Ambiguous localization, duplicate display names, control characters and names over
100 UTF-8 bytes are excluded. Coordinates/level restrictions are left to phBot.
Using the English localized label for the documented `name` argument is the
implementation's source-based interpretation; native name resolution still needs
operator-authorized Windows/phBot validation.

Plugin **1.9.11**, protocol **13**, imports these optional APIs through the normal
allowlisted adapter, advertises supported modes, and invokes `reverse_return` on
the existing controlled command callback. Party mode rechecks `get_party()` at
execution, uses the observed member's exact name and refuses the executing
character itself. All names are trimmed, at most 100 UTF-8 bytes, with no control
characters; malformed/unknown arguments never invoke the API. Boolean true/false,
exceptions and non-boolean results have distinct command outcomes and retain normal
expiry, duplicate suppression and session/generation fences.

Go projects optional Reverse return context through the existing single/batched
controls reads, joining active sessions and resource generations. Party names use
resource check time (35 seconds, five-second future tolerance), never content-change
time or coordinate-filtered markers. Unchanged party checks are republished after
callback collection; cached resource resends do not renew freshness. Fresh character
inventory can identify the documented scroll as advisory evidence; absent/stale/
unrecognized inventory and pet/storage items cannot establish authoritative usability.
Only phBot's API decides whether an attempt uses a scroll.

Validation: Python adapter/callback tests, Go validation/admission/resource tests,
frontend eligibility/concurrent admission tests and disposable simulator/browser
flows. The fixture worker executes production plugin validation and reports separate
true/false audited results plus an unsupported skipped target. These tests do not
validate native Windows/phBot behavior. Runtime gate remains: load 1.9.11 against
a protocol-13 backend, verify available primitive/modes and current party readback,
then perform an operator-authorized scroll attempt and observe its result/position.

## Pet inventory details and pickup source — 2026-10-03

The official [Pets API](https://plugins.phbot.org/phbot-api/pets) documents
`get_pets()` as the authority for current pet IDs, type, basic state and supplied
item lists. It does not document richer per-instance fields or a packet layout.
The official [`get_item(id)` API](https://plugins.phbot.org/phbot-api/game-data)
provides static item definitions, not current plus, durability, whites, blues or
rolled combat values. The official [event API](https://plugins.phbot.org/phbot-api/events)
documents drop callbacks with a model ID only; those callbacks cannot be correlated
to a pet pickup.

The operator-provided current runtime report is phBot **20.1.2**, plugin **1.9.12**,
agent protocol **13**. Its Pick pet reported 56 slots and five items, including two
necklaces; those API rows did not expose plus, whites or blues. This confirms a real
API-backed inventory and a missing-detail need, but no sanitized raw API payload or
matching pet packet bytes were available in this task. The same API contract applies
to Pick, Transport, Fellow and other types only where that runtime supplies `items`;
no family-specific packet layout is inferred from the shared field name.

Pinned RSBot handlers support investigating Joymax `0x30C8` as a pet data response
and `0xB034` as item operations, but are corroboration only, not evidence for the
installed phBot protocol. Plugin **1.9.14** / protocol **14** adds a presence probe
for both opcodes. Its resource diagnostic reports per-session counts and packet
length ranges; it retains no payload bytes and decodes no pet details or operation
semantics. `0xB034` still invalidates character item enrichment. The Pet tab shows
these probe counts so natural runtime traffic can establish which packet families
arrive. API slots remain persisted separately as basic facts.

This upload is an evidence-gathering build, not an enabled candidate decoder. The
runtime protocol, pet-family branches, item rental/binding variants, and packet
ordering still need to be established from operator runtime evidence before any
packet-derived fields or pickup events are produced.

The protocol-14 event validator accepts `joymax.pet_inventory` only for bounded,
positive `item.acquired` pet receipts carrying pet/slot destination and packet
observation identity. A validated worker helper freezes the item snapshot, hashes a
stable dedupe identity and enters the existing durable spool. No production parser
emits such receipts until a matching runtime fixture verifies packet branch, item
identity and delta semantics. Snapshot differences remain unknown-source item-gain
records and are never presented as verified pet pickups. Normal/Rare feeds can
separately include classified recipient-owned inventory/pet gains with a clear
source label.

Backend migration `000022` stores optional profile-derived normal/rare classification
and version. REST and live feed inclusion is opt-in and valid only for Normal or Rare
Drops with no category filter. It defaults off for existing clients. PhMon's two
drop tabs explicitly opt in, including legacy side-navigation URLs without the query
flag. Unknown classification remains in All. Callback drop classification retains
its existing observed callback kind.

Source references: [pinned RSBot pet response](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Cos/CosDataResponse.cs)
and [pinned inventory operations](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Inventory/InventoryOperationResponse.cs).
Required next evidence: sanitized `get_pets()` objects with missing/zero/empty
distinctions and captured naturally arriving `0x30C8`/`0xB034` bytes from each
supported pet family and runtime protocol, followed by natural, exactly observed
pet-to-bag comparisons. Until then, detailed packet stats and live pickup receipts
are explicit runtime blockers.

## Drop tooltip observation link — 2026-10-03

The official [Events API](https://plugins.phbot.org/phbot-api/events) still gives
only an equippable model ID for drop callbacks. The official
[Drops API](https://plugins.phbot.org/phbot-api/drops) exposes nearby pickable
items keyed by pick ID, with name/code/model/location, blue flag and plus in its
example; it does not document actual white rolls or option values. Static
inspection of the authorized local phMonitor v0.5.0 executable shows that it
checks later inventory snapshots for a changed item of the callback model and
emits a linked enrichment; see
[the investigation](reference/item-tooltip-investigation.md).

Workspace plugin **1.9.15** / protocol **14** retains the callback occurrence,
reads a fresh inventory model count, and accelerates resource sampling to one
second for up to 30 seconds while a
drop match is pending. It links only a unique callback model to one newly owned
item in one inventory slot with a count above that callback baseline, within
the same character session. The acquisition
event keeps `acquisition_method: unknown`; the link means a nearby-in-time
inventory gain, not proven ground pickup. The server joins the event snapshots
only when agent, session, character, model and event ID agree. The original
acquisition payload contains observed API/packet item details and the tooltip
uses its existing typed resolver. Multiple same-model drops or gains stay
unlinked. No live Windows/phBot callback-to-inventory capture or PostgreSQL
integration result for this increment is available from this workspace; those
are the next runtime gates. Ground-only drops may still lack exact stats when
they never enter an observed inventory.

### Live necklace drop correlation — 2026-10-03

The active nuker4 agent reports plugin 1.9.15. An authenticated read-only Events
API check found a model-1895 `EVENT_ITEM_DROP` at 18:44:10Z and one
`item.acquired` at 18:44:11Z for the same agent, character, session, region and
coordinates. The acquisition held API white percentages 87/32 and resolved
physical/magical absorption 23.4/23.1; no blue options were observed. The
acquisition lacked `drop_event_id`. The exact plugin gate that rejected or missed
the link cannot be recovered from the persisted events.

Plugin 1.9.16 fixes a separate verified code gap for an already owned same-model
item: a uniquely new inventory slot may now link an `item.quantity_increased`
event with that slot's own item evidence. The server also provides a conservative
read-time temporal match for a sole unlinked one-item acquisition within three
seconds and 24 XY / 32 Z units, with no competing drop of that model. This uses
saved observations, leaves cause unknown, and can recover the reported row after
deployment. It does not create stats for unpicked ground drops, establish actual
blue options from `get_drops()`, or explain why the 1.9.15 explicit gate missed
this callback. PostgreSQL and deployed UI validation remain open.

### Party distribution and recipient-owned gains — 2026-10-03

The official [Events API](https://plugins.phbot.org/phbot-api/events) provides an
equippable model for `EVENT_ITEM_DROP`/`EVENT_RARE_DROP`, not a loot recipient or
rolled instance. The [Inventory API](https://plugins.phbot.org/phbot-api/inventory)
returns the current character's items; the [Pets API](https://plugins.phbot.org/phbot-api/pets)
returns summoned pet items when available. The [Party API](https://plugins.phbot.org/phbot-api/party)
lists party members but does not document item allocation. A party distribution
line is therefore a hint to inspect the named character, not proof that a drop
callback belongs to that character or that an item gain was persisted.

Static inspection of the authorized `%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe`
found that its drop enrichment compares the callback observer's before/after
inventory and emits `drop_item_enriched` for a match; it does not parse the party
line as a recipient. Its normal diff loop defaults to 2 seconds and inventory
snapshot sync to 15 seconds. Copying the observer-only match would preserve this
party-sharing failure. PhMon instead compares each connected character's own
inventory/pet snapshots at a 2-second interval, independently of callback or
chat delivery. The gained instance is selected from the changed slot when unique,
including when an older copy of the same model already exists. Optional pet or
storage availability no longer suppresses an unrelated inventory gain; models
present in an appearing/disappearing container are conservatively skipped for
that comparison to avoid inventing an acquisition from a transfer. When several
same-model copies change and the gained instance is ambiguous, the event keeps
only the item identity and marks its instance unobserved; it never borrows
another copy's rolls.

An authenticated read-only check near the reported 21:17:39 local party line
found no Hydra Gauntlet drop or gain event. A later nuker2 inventory snapshot
contained male model 11840 in slot 29 with observed white rolls and no observed
blues. That current snapshot does not establish when the item arrived; the
historical event cannot be backfilled as an acquisition without an earlier
comparable owner snapshot. A recipient's future `item.acquired` or
`item.quantity_increased` event carries its own observed item evidence, unknown
acquisition cause and exact destination. The Normal/Rare feeds opt in to those
classified gains and label them **Owned item gain**, distinct from **Drop observed**
and verified **Pet pickup** rows. Brief items can still enter and leave between
polls; a runtime packet/change callback with verified semantics would be needed
to eliminate that sampling limit. CI validation with disposable PostgreSQL and
the stack smoke job passed for commit `501fe922`. Live phBot validation after
installing the new plugin remains open.
