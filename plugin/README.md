# PhMon phBot plugin

The current plugin development release is **1.7.2** (`vsro_1188_passive_r2`, API
evidence schema 2), using agent protocol v9 over the existing authenticated
connection. It retains canonical callbacks, inbound chat, alchemy attempts, reliable
membership/container deltas and bounded v6 event batches. It adds current nearby
monster snapshots and profile-scoped durable observation samples. Protocol v8 adds
transient route reports for generated-script navigation. Protocol v9 adds ephemeral
`map.npcs` snapshots from `get_npcs()`: at most 128 rows, sampled about every two
seconds, immediately after a teleport or region change, and refreshed at least every
15 seconds while the normalized view is unchanged. `GATE_<name>` server names are
teleporters; other rows are NPCs. `unavailable` clears that character's markers.
There is no NPC history. The backend keeps accepting
protocol v2–v8 and older plugins continue
sending death events through their original frame. Rare and normal drops remain
separate and retain only the model ID documented by phBot. Chat keeps its raw server
type. Explicit channel names are normalized, along with operator-confirmed runtime
values 1 (General/All), 2 (Private), 4 (Party), 5 (Guild) and 6 (Global); other numeric
values remain `unknown`. The plugin optionally
imports `phBotChat`, reports callable General/Private/Party/Guild/Union/Global modes,
and accepts bounded `chat.send` commands through the existing callback-thread
dispatcher. A boolean API result records phBot acceptance only, not delivery. No real
chat callback or send was exercised in the simulator tests.

Version 1.7.1 reports botting state from a boolean `get_character_data()` field
when available, otherwise from narrowly recognized `get_status()` values. Unknown
statuses remain unknown; the meaning of `stopped` and `None` still needs runtime
verification.

Version 1.7.2 adds a manual, read-only **Test get_players** button for
[issue #36](https://github.com/brantje/phmon/issues/36). Its results stay in the
local phBot log; agent protocol remains v9.

## Test the Players API (issue #36)

The [official Players API](https://plugins.phbot.org/phbot-api/players) explicitly
marks `get_players()` disabled. This diagnostic checks the installed runtime before
any nearby-player map feature is implemented. It needs no backend configuration.

1. Replace `PhMon.py` in the phBot **Plugins** directory with this branch's file
   and reload it. Confirm the tab shows **PhMon v1.7.2**. Existing connection
   settings continue to work.
2. Open **Plugins -> PhMon** and click **Test get_players** while joined with the
   game client running, preferably where other players are visibly nearby. Repeat
   a few times, at least two seconds apart.
3. Repeat in a known clientless session if available. Note the actual mode with
   the results; the probe never changes it. Optionally test when already logged
   out/disconnected to compare availability.
4. Copy the `Plugin: PhMon get_players probe:` JSON lines from the phBot log,
   together with which mode you tested and whether other players were known to be
   nearby. A preceding `probe started` line without a result identifies a native
   call that has not returned.

The log distinguishes `import_failed`, `missing`, `not_callable`, `exception`,
`none`, `empty_dict`, `populated_dict` and `unexpected_type`. **`none` means
unavailable, not zero nearby players.** An `empty_dict` records what was returned;
it alone does not prove nearby-player discovery works. A populated dictionary
records total, inspected, valid and invalid entry counts, with `truncated: true`
when more than 128 entries were returned. The first entry's selected field types
help diagnose unexpected runtime shapes. At most three valid samples contain ID,
name/guild/grant, finite X/Y and an actual boolean dead flag when supplied. Text
fields are capped at 64 characters, and equipment/unknown field values are excluded.

Context includes phBot/plugin version, UTC timestamp, connection/join callbacks,
available observer identity/region/X/Y/Z, and the documented `get_client()` boolean
`running` value. A non-running client alone does not establish a logged-in
clientless session; missing context remains unknown. No client path, process ID,
token or backend URL is logged. This is a manual local diagnostic: no player
poller, player transport, map layer, player history or packet fallback is added.
Automated fixture tests cover classification and bounds; actual runtime evidence
is still pending the operator's logs.

The event spool upgrades profile-scoped death rows in place. Its bounded reserve is
512 important occurrences / 8 MiB plus 2,048 ordinary occurrences / 16 MiB. Callback
queues do not write files or use the network; the worker atomically spools before
sending. The short process-crash window between callback queueing and durable worker
spooling remains. Queue overflow and spool failures appear in plugin status and logs.

The plugin continues to collect documented inventory, storage, pets, party and
academy state and send bounded snapshots/deltas from its worker. Protocol v4 passively
decodes only the bounded 0x3040 item-stat and 0x3052 durability updates when the active
server is unambiguously selected as vSRO 1.188. Those layouts still need a naturally
captured Greatest runtime fixture. Older protocol-v2/v3 plugins retain their existing
monitoring/command capabilities but do not provide Slice 4 resources or Slice 5 event
families.

Protocol v7 polls documented `get_monsters()` every 0.1 seconds. Current snapshots
are capped at 128 entries; unavailable (`None`/missing/exception), observed empty
(`{}`), and truncated results stay distinct. Complete observations are locally
spooled at most once per minute per session, region, world floor and 192-unit
observer cell. Empty observations are durable zero samples. Unavailable and truncated
results never enter historical storage. The spool is bounded to 2,048 samples / 8 MiB
and rows leave it only after the Go service reports persistence or a terminal rejection.
Version 1.5.1 added bounded monster name, numeric type, HP/max HP and attack state;
1.5.2 also passes an optional bounded level when the runtime includes it. The
documented `get_monsters()` response does not promise a level field, so the map labels
the level unavailable when the active runtime omits it. Existing 1.5.0 agents still
connect, but their map popups cannot show fields they did not send.
This cadence, spool and transport have simulator/unit-test coverage; installed phBot
behavior and the PostgreSQL-backed reconnect/replay gate remain open.

Plugin 1.5.4 preserves signed cave region IDs, including Donwhang's observed
`get_position()` `-32767` / `get_monsters()` `32767` pairing, and includes the
observer's current Z in live monster snapshots so the server can scope sightings
to the observed cave floor when individual monster Z is unavailable.

Plugin 1.5.5 hardens the existing documented party resource collection for the live
map layer: party X/Y values are emitted only when finite and within the same bounded
coordinate envelope used by the backend projection. It does not add a second party
poller, packet fallback, or protocol-version change.

Plugin 1.5.6 fixes guild-storage gold collection by preserving the valid non-negative
integer `gold` value returned by `get_guild_storage()`. It removes the unnecessary
0x3253 packet fallback; guild-storage gold now follows the same canonical phBot API
resource path as the rest of guild storage. Agent protocol remains v7 because the
existing resource payload already supported an optional guild-storage `gold` field.

For `character.navigate`, the worker validates generated walk/wait/teleport script
lines once, executes that exact validated script, and publishes only normalized
route instructions after `start_script` does not explicitly fail. Script text and
teleporter identifiers stay inside the plugin. Route snapshots are memory-only,
repeated for recovery at five-second intervals, and cleared on session/profile
replacement. Plugin acceptance means phBot accepted the background script; arrival
is reported only after a later fresh position observation. Protocol v2–v7 agents
still support their existing commands but cannot report remaining route geometry.

PhMon.py is the phBot-side connector for the self-hosted PhMon backend. Each running
phBot instance owns one stable agent identity and makes its own outbound WebSocket
connection. The plugin reports connectivity facts; durable identity, authentication,
history and future command policy remain server-owned.

Version 1.6.1 adds local callback timing diagnostics. Navigation logs the start
and elapsed duration of generation, validation, source readback and script start,
including failed calls. An `event_loop` taking at least 500 ms logs its total and
four slowest stages, including resource collection and command invocation. These
logs contain stage names and durations only, never script text or API arguments.
If phBot reports `event_loop has been running for 10 seconds`, retain the preceding
`navigation ... started` line and subsequent duration/slow-callback lines. They
identify the blocked stage; the watchdog warning alone does not. Operator timing logs isolated `generate_script` as an 8-second callback stall.
Version 1.6.2 dispatches only generation on one bounded daemon worker; validation,
source readback and `start_script` remain on callbacks. Callback polling never
joins or waits for that worker. Late results cannot start scripts after expiry,
profile/session change, teleport, disconnect, revocation or plugin stop. An
uninterruptible generator retains its slot until it returns, including across
profile worker replacement. This threading change needs installed phBot runtime
verification; fixture tests cannot establish native API thread behavior.

## Install

1. Provision a credential under **Settings -> Agents** with **Create credential**, or
   use the CLI for a headless/operator flow:

       docker compose exec server phmonctl agent create

   Both paths generate the credential in the Go backend. The agent_id and agent_token
   are shown once while PostgreSQL stores only the token hash. Keep the plaintext
   token private.

2. Copy plugin/PhMon.py into the phBot Plugins directory and reload the plugin.
3. Join the game with the account/profile you want to configure.
4. Open **Plugins -> PhMon** and enter:
   - Backend WebSocket URL, for example `wss://phmon.example.internal/agent`
   - Agent ID from Settings -> Agents or the CLI provisioning result
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
stream. The worker negotiates agent protocol v5 while the backend continues to accept
v2/v3/v4 agents. It sends independently probed capabilities and current character/session
state, then bounded resource baselines and revision-checked deltas. A separate bounded
command queue is never coalesced with state samples. Network callbacks only validate
and enqueue; `event_loop()` checks
the live target, expiry, input schema and optional API availability again, then calls
at most one fixed adapter. No phBot mutation runs on the network worker and no API
is selected through arbitrary callable names.

The documented Character API's boolean `dead` field is sampled only when it is a
real boolean; missing or invalid values remain unknown. `EVENT_DIED` (7) is queued
as an occurrence with a stable ID and explicit character/session identity, then
removed from the bounded disk spool only after PostgreSQL acknowledges persistence
or terminal rejection. Reconnect and process restart replay the same ID. A death
snapshot alone never creates history, and the callback's empty data string does not
establish a cause.

The same documented `get_character_data()` response supplies the optional integer
`model` used for local character portraits. Values outside the positive unsigned
32-bit range, booleans and non-integers are omitted. Existing agents can continue
without `model`; the server stores a portrait only after observing it in the active
character session.

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
observed. Botting status is reported through the existing `CharacterView.botting`
field when `get_character_data()['botting']` is an actual boolean. The optional
`get_status()` fallback recognizes only `botting`/`training` as true and `tracing`
as false; unknown values, errors and unavailable status remain unknown. Commands
never optimistically set this state.

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
