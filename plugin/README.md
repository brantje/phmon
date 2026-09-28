# PhMon phBot plugin

The current Slice 4 development release is **1.2.6** (`vsro_1188_passive_r2`, API evidence schema 2). It adds passive resource
snapshots over agent protocol v4 while retaining backend compatibility with v2/v3
agents. The plugin collects documented inventory, storage, pets, party and academy
state on phBot callbacks and sends bounded snapshots/deltas from its worker. Protocol
v4 passively decodes only the bounded 0x3040 item-stat and 0x3052 durability updates
when the active server is unambiguously selected as vSRO 1.188. Their layouts still
need a naturally captured Greatest runtime fixture. Earlier v2/v3 plugins retain their existing monitoring/command
capabilities but do not provide Slice 4 resources.

PhMon.py is the phBot-side connector for the self-hosted PhMon backend. Each running
phBot instance owns one stable agent identity and makes its own outbound WebSocket
connection. The plugin reports connectivity facts; durable identity, authentication,
history and future command policy remain server-owned.

## Install

1. Provision a credential from the PhMon dashboard with **Create credential**, or
   use the CLI for a headless/operator flow:

       docker compose exec server phmonctl agent create

   Both paths generate the credential in the Go backend. The agent_id and agent_token
   are shown once while PostgreSQL stores only the token hash. Keep the plaintext
   token private.

2. Copy plugin/PhMon.py into the phBot Plugins directory and reload the plugin.
3. Join the game with the account/profile you want to configure.
4. Open **Plugins -> PhMon** and enter:
   - Backend WebSocket URL, for example `wss://phmon.example.internal/agent`
   - Agent ID from the dashboard or CLI provisioning result
   - Agent token from that same one-time result
5. Click **Save & Connect**. The agent should appear in PhMon after the authenticated
   hello handshake succeeds.

Configuration is profile-scoped. PhMon asks phBot for the active player's
`get_config_path()` and `get_profile()`, derives a matching filename, and stores
its own settings in:

    <phBot Config>/PhMon/<active-profile>.cfg

PhMon never modifies phBot's player JSON; the documented phBot API warns that direct
changes to that JSON may be overwritten. The player configuration separates
accounts/characters and the explicit profile name separates multiple named profiles
for the same character.

The saved file contains the bearer token because automatic reconnect requires it.
Keep the phBot Config directory private. QtBind documents a normal line edit rather
than a password widget, so the token is visible while being pasted; PhMon clears the
token field immediately after loading/saving and reuses the stored token only when
the displayed backend URL and agent ID still match.

The default Compose binding keeps the Go agent listener on loopback. Use ws:// only
when the plugin runs on the same host or on an explicitly trusted development network
with a deliberate SERVER_BIND_ADDR override. For remote agents, terminate TLS in
front of Go and configure wss://; the plugin uses normal system certificate
validation and does not provide an insecure TLS bypass.

Do not put the token in the backend URL. The plugin rejects URL credentials, query
parameters and fragments so secrets do not leak into logs, browser history or proxy
access logs.

## Runtime behavior

The plugin uses only Python standard-library networking. Public phBot documentation
verifies socket support but does not document a bundled third-party WebSocket client,
so no pip install inside phBot is required. A real phBot runtime still needs to verify
the embedded availability/behavior of ssl, select, threading, hashlib, base64, struct
and urllib.parse; simulator CPython is not evidence for that gate.

A dedicated worker thread owns WebSocket connect/read/write work. It polls readiness
before starting a frame, then completes the frame under a bounded socket deadline; a
mid-frame stall fails the connection rather than resuming from a partially consumed
stream. The worker negotiates agent protocol v4 while the backend continues to accept
v2/v3 agents. It sends independently probed capabilities and current character/session
state, then bounded resource baselines and revision-checked deltas. A separate bounded
command queue is never coalesced with state samples. Network callbacks only validate
and enqueue; `event_loop()` checks
the live target, expiry, input schema and optional API availability again, then calls
at most one fixed adapter. No phBot mutation runs on the network worker and no API
is selected through arbitrary callable names.

Supported documented adapters include bot/trace start-stop, training area/radius,
same-region walk, return scroll and disconnect. Walk requires `generate_path`,
`move_to_region` and `get_position`; the plugin requests phBot's waypoint route and
advances it callback-by-callback as live position reaches each node. Cross-region
routes and teleports are not supported, and generated scripts are never executed.
Training controls require their respective getters/setters; local training script
paths are not sent. The documented
Client API only inspects client state, so clientless stays unavailable. The plugin
does not kill processes, change relog settings, run arbitrary scripts, persist
commands locally or replay them after reconnect. Bool and void API returns remain
distinct; walk, return-scroll and disconnect effects are unverified until separately
observed. Botting status remains unknown.

The stop path closes the worker socket to unblock bounded reads without joining from
a phBot callback. Outbound results use their own bounded, non-coalescing queue and may
be lost on transport loss; the backend then retains `unknown` rather than replaying
the action.

## Simulator

For development without a Windows/phBot process:

    PHMON_AGENT_URL='ws://127.0.0.1:8081/agent' \
    PHMON_AGENT_ID='...' \
    PHMON_AGENT_TOKEN='...' \
    python3 scripts/agent_simulator.py

The simulator imports the exact same transport/worker and command dispatcher as the
plugin and reports simulator-fixture as its phBot version. The `commands` scenario
uses a fake `stop_bot` adapter and exercises callback dispatch without real phBot.
It is fixture coverage only and is never evidence that real phBot integration has
been validated.

Version 1.2.5 fixes API evidence collection: integer-keyed option/attribute maps are
preserved as ordered typed `mapping_entries`, additional attribute aliases are
retained, and `api_field_types` records bounded structural information without
unknown field values or credentials. `api_evidence_version=2` identifies this format
on items and in enrichment telemetry. The evidence remains uninterpreted until the
actual phBot field semantics are verified; this release does not yet enable new
tooltip calculations. See [the inspection findings](../docs/reference/item-tooltip-investigation.md).

The protocol detector from 1.2.4 reads the selected vSRO profile beside `Config`,
matches the current server, and checks variant flags while ignoring numeric
`version`. This detector is confirmed live on the current four characters. The
bounded passive item decoder remains available for naturally observed supported
updates. API-derived details should be used where verified; packet data is needed
only for missing inputs. Absolute formulas, max durability and blue presentation
still require validated definitions and matching live observations.
