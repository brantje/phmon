# PhMon phBot plugin

The current local plugin release is **1.9.32**, using agent protocol **18**. It
accepts a display-only route from another plugin in the same phBot process through
`submit_external_route` and publishes `navigation.observed`. PhMon does not start
or stop that route. Walks stay visible when a trade script also contains blank
lines, comments, shop commands, or a teleport name with punctuation; those lines
are omitted, and a route is kept when at least one walk remains. `lines` may be a
list or one script string. `character.navigate` accepts the same 4096-line and
512 KiB route budget. It looks up a thief's own position in `get_players()` when
`EVENT_THIEF_SPAWN` fires, and otherwise keeps the observer position. It also
adds the session-fenced, latest-value `character.position` transport used by the
live Map for controlled characters, while retaining navigation transport fixes, script
diagnostics, direct `move_to` support and scoped recall-point packet submission for
phBot 20.1.3 on Greatest. Position observations are memory-only: packet callbacks
never write WebSockets, the network worker keeps only the newest unsent coordinate,
and a failed socket write leaves that coordinate pending without consuming its
sequence. It also
correlates a documented player-attack callback with a death callback from the same
character within ten seconds. The death event records the recent player's name or
`Monster / environment` as an inferred reason. This does not identify the actual
killing blow. The profile-scoped **Potions** and **Pills** options default on and
suppress new item events of the selected types before they enter the event spool.
Item types resolve through phBot's `get_item(model)` data on the callback thread;
inventory snapshots continue updating normally. These classifications add no fields to the wire
protocol. The historical capability summary below starts at plugin 1.9.12 and
protocol 13.

At the 1.9.12 development baseline, plugin **1.9.12** (`vsro_1188_passive_r2`, API
evidence schema 2), used agent protocol v13 over the existing authenticated
connection. It retained canonical callbacks, inbound chat, alchemy attempts, reliable
membership/container deltas and bounded v6 event batches. It added current nearby
monster snapshots and profile-scoped durable observation samples. Protocol v8 adds
transient route reports for generated-script navigation. Protocol v9 adds ephemeral
`map.npcs` snapshots from `get_npcs()`: at most 128 rows, sampled about every two
seconds, immediately after a teleport or region change, and refreshed at least every
15 seconds while the normalized view is unchanged. `GATE_<name>` server names are
teleporters; other rows are NPCs. `unavailable` clears that character's markers.
There is no NPC history. Protocol v10 adds ephemeral `map.players` snapshots from
optional `get_players()` (operator-tuned **1 s** poll and **2 s** unchanged refresh;
NPC cadence remains 2 s / 15 s), 128-row and 64 KiB bounds, and
observer Z in the publish signature. Each player row may include `zone` from
`get_zone_name` for that player's region, or the observer region when the row has
no region. Equipment and player Z are not copied. The
backend keeps accepting protocol v2–v9 and older plugins continue
sending death events through their original frame. Rare and normal drops remain
separate and retain only the model ID documented by phBot. Chat keeps its raw server
type. Explicit channel names are normalized, along with operator-confirmed runtime
values 1 (General/All), 2 (Private), 4 (Party), 5 (Guild) and 6 (Global); other numeric
values remain `unknown`. The plugin optionally
imports `phBotChat`, reports callable General/Private/Party/Guild/Union/Global modes,
and accepts bounded `chat.send` commands through the existing callback-thread
dispatcher. A boolean API result records phBot acceptance only, not delivery. No real
chat callback or send was exercised in the simulator tests.

Version 1.9.12 keeps protocol 13 and records unique spawn/kill notices from
Joymax opcode `0x300C` (type byte 5 or 6, then the model ID). The documented
`EVENT_UNIQUE_SPAWN` callback is unchanged. The notice does not copy the
observer's position. Version 1.9.11 supports confirmed `character.reverse_return` (issue #34), retaining
protocol 13. A callable `reverse_return` enables last Return Scroll and death
locations. Party mode also requires `get_party` and rechecks the chosen observed
member immediately before using a scroll; self-targets are rejected. Named
mode calls `reverse_return(3, name)` without a party dependency. The backend offers
only localized names from the executing character's selected server profile;
the Greatest catalog resolves enabled `refoptionalteleport.txt` rows through
`textdata_object.txt`. Missing profile names remain unavailable. Boolean API
acceptance records scroll use, not
arrival. Unchanged party observations refresh only after a new callback collection.

Version 1.7.1 reports botting state from a boolean `get_character_data()` field
when available, otherwise from narrowly recognized `get_status()` values. Unknown
statuses remain unknown; the meaning of `stopped` and `None` still needs runtime
verification.

Version 1.8.0 replaces the issue #36 manual player/equipment probes with production
`map.players` collection for the PhMon **Other players** map layer. Historical probe
findings remain in [runtime evidence](../docs/phbot-capabilities.md#issue-36--manual-players-api-runtime-probe-2026-10-01).
Install/reload **1.8.0**, connect to a protocol-10 backend, and open the Map with
**Other players** enabled. There is no equipment inspection, player history, or
packet fallback.

Version 1.9.4 adds audited `character.teleport` for issue #33: live `GATE_*` gate in
session `get_npcs()`, `get_teleport_data(source, destination)`, then one bounded
`teleport,source,destination` line via `start_script`. Map UI uses action-target
fan-out with an operator-entered destination (no menu enumeration). `start_script=True`
does not prove arrival.

The Map menu prepares a **Designate Recall Point** review for an observed
teleporter. On phBot 20.1.3 and Greatest, the plugin resolves that gate again
from the target character's `get_npcs()` snapshot and submits the captured
`0x7059` request using the gate's live runtime ID. Two operator-run manual
captures and an in-game success message established the request. PhMon records
its own request as **sent; outcome unverified** because the `0xB059` response
byte has not been interpreted for an automated command. Other builds and
servers remain disabled. The temporary read-only capture procedure and evidence
are in [`tools/recall-point-capture`](../tools/recall-point-capture/README.md)
and [the investigation](../docs/reference/recall-point-investigation.md).

Version 1.9.3 adds **Test Hotan→Jangan** (operator-only): requires the Hotan
`GATE_KT` gate in `get_npcs()`, resolves `get_teleport_data`, then runs one
`teleport,Hotan,Jangan` or `teleport,GATE_KT,GATE_CH` script line via
`start_script`. The read-only probe also includes those reference pairs (tagged
`hotan_to_jangan` / `gate_kt_to_jangan_gate`).

Version 1.9.2 adds a read-only **Probe teleporters** QtBind action for issue #32.
It discovers matching `phBot` symbol names and runs at most sixteen bounded
`get_teleport_data` pair checks from the current `GATE_*` snapshot. It never
injects packets or starts scripts. Findings belong in
[issue32-teleporter-investigation.md](../docs/reference/issue32-teleporter-investigation.md).

Version 1.9.0 adds `character.navigate.stop` through documented `stop_script()`,
trace `activity_state` on control state (from optional `get_status()`), session-scoped
`trace_requested_name`, and protocol v11 capability reporting. Connect to a
protocol-11 backend for navigation Stop and the trace activity fields.

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

Reverse return fixture smoke (disposable stack only): set `SMOKE_WEB_URL`,
`PHMON_AGENT_URL`, `OPERATOR_ACCESS_SECRET` and `NUXT_OPERATOR_COOKIE_NAME` for
your test stack, then run:

```sh
PHMON_SMOKE_COMMAND=character.reverse_return PHMON_SMOKE_SKIP_THIRD=true python3 scripts/command_smoke.py
```

It imports the production plugin worker, admits eligible commands concurrently,
and verifies one fake API success, one fake API failure, and one unsupported
skip with independent audited results. `remote-controls` fixtures also support
`PHMON_SIMULATOR_PARTY_NAMES` (comma-separated observed names),
`PHMON_SIMULATOR_FALSE_ACTION=reverse_return`, and omission through
`PHMON_SIMULATOR_UNSUPPORTED_PRIMITIVES=reverse_return` or `get_party`.
No simulator result validates Windows/phBot scroll behavior.

### Job information investigation (1.9.30 / protocol 18)

Live job detection is **not yet verified on Greatest/phBot 20.1.3**. The earlier
`decode_unverified_spawn_fixture` used an invented record format, not a Silkroad
spawn packet. It has been replaced by an independently authored, offline candidate
decoder for the classic layout, with explicit `vsro-1.188` variant selection.
This variant label is a hypothesis, not proof of Greatest compatibility.
Production `SPAWN_CHARACTER_DECODER_ENABLED` remains false. Candidate results are
marked `candidate_only` and rejected by the verified runtime cache.

The structural reference is
[xBot's EntitySpawn parser](https://github.com/JellyBitz/xBot-WinForms/blob/5fc222aceb6e2d2eb21b7790002b7b864273c312/xBot/Game/PacketParser.cs).
The candidate skips visible item structures without retaining equipment. It reads
the length-prefixed ASCII name and independent job bytes after movement/state
structures; normal appearance does not force job `none`. It rejects unknown item
shapes, masks, buffs, unsupported entities/versions, truncated or trailing bytes,
duplicate runtime IDs and mixed groups it cannot fully consume. It supports the
candidate movement, riding, stall and job-suit branches. None is runtime verified.
Raw packet coordinates remain distinct from world coordinates. Real group framing
has an action byte followed by a uint16 count; spawn and despawn groups differ.
Buffers, chunk counts and group duration are bounded. Completion of assembly
alone never proves the entity count or enables decoding. Cache attachment requires
the exact observed name as well as runtime ID; it never links normal/job names.

The temporary job evidence capture was removed in **1.9.32** at the operator's
request. Its GUI controls, raw packet buffer, API snapshots, export files and
callback hooks are no longer part of the plugin. Existing operator-provided
captures remain local historical evidence. Verified own-character job collection
continues through the normal character sampler and `map.players` transport.
Cleanup validation: all 247 remaining plugin/protocol tests pass, and
`git diff --check` passes. Reload the updated plugin to remove the controls
from an already-running phBot instance.

The two existing local captures inspected on 2026-10-10 contained no `0x3015`,
`0x3017`, `0x3019` or `0x3018`, so they cannot validate spawn/job decoding.
The [official Character API](https://plugins.phbot.org/phbot-api/character)
documents own-character job experience/name, but not job type or job level.
The [Players API](https://plugins.phbot.org/phbot-api/players) example does not
provide these fields either. Own-character `0x3013` has a different layout from
nearby-player spawns; it is not parsed as `0x3015`.
The managed-character state transport also has no job fields; this plugin-only
change does not invent new backend fields or claim that screen is implemented.

Next required gate: inspect fresh packet bytes and API values against nuker1's
reported Trader level 7 and a separately verified nearby player, identify the
actual source/opcode/version and every relevant branch, then enable only the
proven decoder through the existing authenticated transport. Do not enable the
candidate merely because synthetic tests pass.

Validation: 248 tests pass via `python -m unittest discover -s plugin -p 'test_*.py'`, covering both
the decoder foundations and the then-present capture behavior; fixtures are synthetic, not proof
of live job detection. The outbound protocol fingerprint is unchanged and the live
transport audit passes. Only `plugin/` is changed by this increment.

### Verified own-character job fields (1.9.31 / protocol 18)

Operator capture `12456dfc-80a2-4da8-bf21-b24a503e2950`, SHA-256
`6cd498bac559ee035f6d380624fc220cfea3c973579558747a24dc3f80c05c6a`, recorded
Greatest/phBot 20.1.3 on nuker1 starting 2026-10-09 23:49:26 UTC. All fourteen
own-character API samples independently contain `job_type: "trader"` and
`job_level: 7`, matching the operator's stated reference. `get_character_data()`
also supplies the model and character level. This supersedes the earlier lack of
runtime API evidence for **connected characters**; public documentation omitted
these fields, but the actual getter exposes them. The raw capture stays local
and is not an application asset or test dependency.

Version 1.9.31 uses that same existing character sample to add an own-character
observation to the existing authenticated `map.players` envelope, which already
accepts the optional `job`, `job_level` and `model_id` fields. This reaches the
existing player registry/level-snapshot ingestion without adding ignored fields
to managed-character state or changing the server, database, frontend, protocol
version, socket implementation or polling cadence. It is a data-source change,
not a new map control or layer. An own observation with unavailable nearby data
remains explicitly partial (`truncated`). Count/byte bounds still apply.

Only explicit canonical textual roles and integer job levels in range are used;
missing fields remain omitted. Job experience is not converted to a level.
An assigned `job_name` does not establish active job mode, so `is_jobbing` stays
unknown. No job alias is emitted or linked to the character. If a nearby snapshot
already uses the same runtime ID under a different name, that independent
observation is retained without attaching the own-character facts. Matching
normal-name rows are enriched without duplicating runtime IDs. Identity, region
and position checks prevent cross-character/world attribution.

The own-character capture contained no `0x3013` or spawn/group packets. It does
not verify **nearby-player** job fields, active-job classification or the offline
candidate decoder; production spawn parsing remains disabled. Nearby source
availability remains unresolved; the temporary capture controls were removed in
1.9.32. On 2026-10-10, the operator confirmed that nuker1's Trader / job level 7
reaches the player registry. This closes the live own-character job collection,
transport and registry-display check. Reconnect behavior and retention across
restarts remain separate acceptance checks.

Validation: 255 plugin/protocol tests pass, including the real transport envelope,
job-level changes, missing fields, alias conflicts, identity/region fencing and
count/byte limits. The runtime API evidence above verifies the input; these tests
verify the independently authored normalization and transport behavior.

### Nearby capture result and remaining gate

Operator capture `39df70b8-3c4f-439d-ab4f-10c951593632`, SHA-256
`8cd1a6b19d39c9590b4387d314bee131c74055e558d34be78c8536c59b0bd756`, recorded
Greatest/phBot 20.1.3 with plugin 1.9.30, observer nuker1 and target nuker2,
starting 2026-10-09 23:54:07 UTC. The complete 15-second window has no omitted
packets. The target was absent at arm time and first appears in the getter at
8,796 ms, runtime ID 21402605, character level 91. Its getter exposes only
`dead`, `grant`, `guild`, `level`, `name`, `region`, `x` and `y`; character level
is not job level. Thirteen own-character samples again report Trader job level 7.

The packet callback received seven packets totaling 80 bytes: four `0x38F5`, two
`0xB021` and one `0xAA76`. None contains the target name or the candidate
spawn/group opcodes. This establishes a getter transition, not delivery of a
decodable spawn. There is insufficient evidence to reinterpret `0x38F5` as job
data, infer a role/level, or enable the candidate decoder. The cause of the
missing spawn remains unverified; absent-from-getter is not proof that the
server had despawned the entity. Local installed packet handlers examined during
the investigation return True; no blocking handler was identified there.

The operator subsequently confirmed entry from far away with no job suit equipped.
The target's job type/level remains independently unconfirmed. This strengthens
the missing-spawn discrepancy; it does not verify a numeric job field. Official
[event documentation](https://plugins.phbot.org/phbot-api/events) describes
`handle_joymax` as receiving all server packets, but this observed window did not
deliver a spawn. The operator also confirmed that the observer was not clientless.
Clientless mode therefore does not explain this particular capture. No general
callback filtering behavior is established by the evidence.

The next gate is to explain that callback discrepancy. A confirmed fresh spawn
must reach the passive callback
or another documented, runtime-verified public API must expose the job facts
before nearby detection can be enabled. Keep the production decoder disabled and
unknown nearby job fields omitted. Only plugin files are changed; raw evidence
remains local and is not a test dependency.

### Feasibility research - 2026-10-10

**Conclusion:** connected-character job type/level is runtime verified above.
Nearby job type/level is plausible from Silkroad spawn data, including classic
normal-appearance records, but not verified through phBot 20.1.3 on Greatest.
The missing callback payload is an unresolved source-access problem. It does not
prove that Silkroad never sends the fields or that phBot deliberately blocks them.
Research does not enable the candidate decoder or change the plugin protocol.

Primary implementation evidence:

- [xBot parser, pinned commit](https://github.com/JellyBitz/xBot-WinForms/blob/5fc222aceb6e2d2eb21b7790002b7b864273c312/xBot/Game/PacketParser.cs#L736)
  reads name, job type and job level before its suit-dependent guild tail. Its
  [player definition](https://github.com/JellyBitz/xBot-WinForms/blob/5fc222aceb6e2d2eb21b7790002b7b864273c312/xBot/Game/Objects/Entity/SRPlayer.cs)
  distinguishes job role from equipped suit. This supports the offline classic
  candidate, not Greatest's layout or the meaning of its populated values.
- [RSBot player parser, pinned commit](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/Spawn/SpawnedPlayer.cs#L285)
  independently reads job type after the name. Older variants read job level
  independently of suit state; newer branches condition extra job bytes on job
  appearance. Its [client-type enum](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/GameClientType.cs)
  is needed to interpret those comparisons. A universal parser is unsafe.
- RSBot's [single-spawn handler](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntitySingleSpawnResponse.cs)
  and [group-begin handler](https://github.com/myildirimofficial/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Network/Handler/Agent/Entity/EntityGroupSpawnBeginResponse.cs)
  corroborate candidate opcodes/framing, not delivery on this installed runtime.

The [official phBot event API](https://plugins.phbot.org/phbot-api/events)
describes all incoming packets reaching handle_joymax, but our complete nearby
window contains no spawn despite confirmed entry from far away with the client
running. The [Players API](https://plugins.phbot.org/phbot-api/players) is labeled
disabled and provides an older example without job fields; the actual getter
works for identity/location but exposes neither target job field. Documentation
alone cannot settle installed-runtime availability. Hunter events include traders
and supply no level; thief events supply a name only. Those events cannot provide
exact role plus level for every nearby character.

[get_locale](https://plugins.phbot.org/phbot-api/locale) uses the same vSRO locale
for several versions. The [startup guide](https://guide.phbot.org/initial-startup)
requires the correct private-server variant because the wrong one causes parsing
errors. Locale or three-job gameplay alone cannot select a decoder. Even a valid
spawn must be compared with known normal and active-job references before its
values can be treated as assigned role/level. An unknown field stays unknown.

Historical rev6 evidence is weaker than the primary code evidence above. A
[contemporary revbot tutorial](https://www.elitepvpers.com/forum/sro-hacks-bots-cheats-exploits/106896-only-working-bots-today-2.html)
describes nearby observations uploaded to rev6's ladder database. This supports
observation aggregation as the historical design, but is not original rev6 source
and does not verify its exact job decoding. Original rev6/archive pages were not
retrievable during this research. Rev6 therefore provides historical precedent,
not proof that all normal players' exact job levels are accessible through modern
phBot. PhMon already has aggregation/persistence; another collector or external
rev6 service does not solve missing source data.

The next discriminating check is a minimal independent passive logger following
phBot's [official packet example](https://plugins.phbot.org/example-plugins), with
no injection or bot actions. Compare all incoming opcodes with PhMon's bounded
capture during a known entry; do not filter only candidate opcodes. If the
independent callback receives a spawn, investigate PhMon's capture/module
lifecycle and decode that actual payload. If both lack it, obtain phBot maintainer
clarification or a public API exposing parsed fields before claiming plugin-only
feasibility here. An external packet interceptor changes the collection mechanism
and is outside the current plan; none was installed. No third-party message was
sent. Same-name connected characters can already contribute their own verified
job fields through existing ingestion; that does not verify observer-side parsing
for unrelated players or link job aliases.

Only plugin documentation changed during this research. The subsequent 1.9.32
cleanup removed the temporary job capture probes at the operator's request.
Polling, transport and decoder-disabled state are unchanged. Remaining acceptance gates
are actual packet receipt, correct server variant, independently verified normal
and active job fields, full record boundaries and entity lifecycle fencing.

### Navigation diagnostics and direct movement (1.9.20)

The plugin logs each command ID when received, queued, started on the callback,
and completed or rejected. Generated navigation also logs generation, validation,
source sampling and script-start timings. Credentials and generated scripts are
never logged. The network worker drains up to 32 inbound frames per iteration so
acknowledgements cannot indefinitely bury commands.

The Map sidebar offers a default-off **Click to walk (uses move_to)** test toggle.
A map click immediately sends a `character.move_to` command to every selected
character, without a frontend controls preflight, path generation, confirmation
review or arrival tracking. The callback invokes `move_to(x,y,z)` once; its
documented `None` return establishes invocation only. Normal authenticated
admission, current-session fencing, bounded arguments and audit records remain.
Right-click continues to use generated-script navigation.

The **1.9.21** debug follow-up logs a command-correlated script summary before
invocation: walk/wait/teleport counts, UTF-8 byte length, a SHA-256 prefix, trailing
newline flag, source/destination region and Z. Optional `get_status()` is read on
the callback before and after `start_script`, with timings and bounded labels.
`None`, unavailable or unfamiliar status never implies a running/stopped script;
the probe never changes admission, retries, script text or bot state. No arbitrary
native exception text is printed. Kalypso's fresh 1.9.21 session passed after the
operator restart, and both repeated eight-character short/long routes had eight
observed arrivals. The earlier native False reason remains unknown; these logs
are available if it recurs. See the [live report](../docs/reference/navigation-2026-10-04.md).
