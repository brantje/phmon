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
    monitoring backend connection.
  - event_loop() runs every 500 ms, but Slice 1 does not need to perform networking
    in that latency-sensitive callback.
- GUI API: https://plugins.phbot.org/gui-api
  - QtBind.init(__name__, ...) creates a plugin tab and must be called at module load.
  - createLineEdit/createButton provide operator-editable fields and actions.
  - text()/setText() read and update widget values.
- Config: https://plugins.phbot.org/phbot-api/config
  - get_config_dir() returns the phBot Config directory with a trailing slash.
  - get_config_path() returns the active player's JSON configuration path when in
    game; the docs warn direct changes to that JSON may later be overwritten.
  - PhMon therefore uses get_config_path() only as the active profile identity and
    keeps its own per-profile settings file under Config/PhMon/.
- Misc: https://plugins.phbot.org/phbot-api/misc
  - get_version() returns the phBot version string.
  - get_profile() returns the active profile name, an empty string for the default
    profile, or None while no player is logged in.

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

Status: BLOCKED — no compatible real Windows/phBot process is available in this
execution environment.

The deterministic simulator added with Slice 1 imports the production PhMon.py
transport and exercises the same protocol contract. Hosted CI has verified credential
creation, connect, backend restart, automatic reconnect and disconnect through that
transport, but simulator success must not be recorded as real phBot validation.

Public phBot documentation explicitly supports socket. The actual embedded runtime
still needs to prove that every imported module used by this path is available and
behaves as expected, especially:

- socket
- ssl
- select
- threading
- hashlib
- base64
- struct
- urllib.parse

The real-runtime gate requires installing PhMon.py in a supported phBot build,
configuring at least two distinct bot profiles through the PhMon QtBind tab, and
recording the observed phBot version and embedded Python version. Confirm each profile
loads its own URL/agent ID, keeps the saved token hidden in the GUI, connects as the
correct agent, and survives plugin reload/disconnect/restart with automatic reconnect.
Also exercise a profile switch to prove one profile cannot silently reuse another
profile's credentials. Record the observed module/import behavior and results here;
do not infer them from desktop CPython or the simulator.
