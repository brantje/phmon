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
