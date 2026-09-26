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
- Config: https://plugins.phbot.org/phbot-api/config
  - get_config_dir() returns the phBot Config directory with a trailing slash.
  - get_config_path() points at phBot's own player JSON and the docs warn changes
    to it can be overwritten. PhMon therefore owns a separate PhMon.json file.
- Misc: https://plugins.phbot.org/phbot-api/misc
  - get_version() returns the phBot version string.

## Slice 1 integration decisions

The official documentation verifies Python socket support but does not document a
bundled third-party WebSocket client. PhMon therefore does not depend on an
unverified websocket package inside phBot. The plugin implements the small RFC 6455
client subset it needs using Python standard-library socket, ssl, hashlib, base64,
struct and select modules.

The plugin keeps backend networking on a worker thread. phBot callbacks never wait
for backend network I/O. The plugin is authoritative only for its current process;
the Go backend owns durable identity, authentication and connection history.

## Runtime validation

Status: not yet validated against a real Windows/phBot process in this repository
environment.

The deterministic simulator added with Slice 1 exercises the same protocol contract,
but simulator success must not be recorded as real phBot validation. The real-runtime
gate requires installing the plugin in a supported phBot build and recording the
observed phBot version and embedded Python behavior here.
