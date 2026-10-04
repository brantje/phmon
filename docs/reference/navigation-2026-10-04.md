# Live navigation follow-up — 2026-10-04

The operator authorized real movement on
`http://192.168.10.25:3005/map?server=Greatest&area=world&floor=world`:
one character and all eight Greatest characters, each over a short and long path,
plus an experimental direct `move_to` sidebar option. Tests used the authenticated
browser and existing audited command API. No credential is retained in this report.

**Final result:** all four requested generated-navigation cases passed. Kalypso's
initial native script refusal ceased after its phBot restart; repeated short and
long group routes each had eight observed arrivals. Direct `move_to` also moved all
eight characters. The underlying cause of the earlier native False remains unknown.

## Changes and identified causes

- Map actions reuse the sidebar's continuously maintained controls snapshot.
  Opening navigation no longer starts two redundant controls subscriptions, and
  submitting no longer forces another subscription refresh. Eligibility, session,
  scope and command admission guards remain. Independent POSTs launch concurrently;
  the backend retains four delivery workers.
- Generated navigation freezes the chosen destination Z for the prepared command.
  A fresh terrain-height change no longer invalidates an unchanged X/Y destination
  or changes its exact retry body.
- The plugin transport previously skipped receiving after publishing a character
  sample or heartbeat, and sent several acknowledgement-producing frames while
  reading at most one per loop. It now flushes telemetry once per iteration and
  drains up to 32 incoming frames, with a 250 ms maximum initial receive wait.
  This removes a demonstrated acknowledgement-backlog starvation mechanism.
- Plugin 1.9.20 logs command receipt, queue TTL, callback start and terminal result
  by command ID, alongside existing native stage timings. Logs do not dump scripts
  or credentials. The final source additionally restricts logged reasons to bounded
  reason codes rather than arbitrary native exception strings.
- The default-off **Click to walk (uses move_to)** checkbox sits below the sidebar
  action controls. A left map click immediately posts one fixed `character.move_to`
  command per selected character, with no controls preflight, planner, review or
  result/arrival wait. Right-click still offers generated navigation. The plugin
  calls `move_to(x,y,z)` once and skips command-specific position/control readback.
  Authentication, capability admission, session fencing, expiry, deduplication and
  bounded arguments remain. A `None` result is unverified invocation, not arrival.
- A newer route previously made a completed child row revert to “Waiting for
  movement evidence.” The UI now retains at most 128 script-free observations,
  keyed by character/session/command. Observed arrivals remain finished after
  replacement; a proven higher route sequence in the same session marks an
  uncompleted older route superseded without claiming arrival. Dismissals retain
  the same route ID and historical rows cannot stop a newer script.

The live server also had an older migration-21 observation schema and an unsigned
activity-event region constraint. These caused repeated persistence failures and
spooled-event retries during the first test. Migration 23 provides the current
columns while preserving legacy level/source/dataset evidence, and accepts signed
cave event regions. The migration was applied in a concurrent maintenance change
in this shared checkout (`c5e436b`); this follow-up adds its isolated regression
fixture. Later server logs had no recurrence of those schema failures. The initial
timeout cannot be attributed solely to either schema errors or transport starvation.

## Real-runtime results

[Sanitized command and browser evidence](navigation-2026-10-04-evidence.json)
retains exact command IDs, requested/effective arguments, HTTP times, durable
delivery/acknowledgement/result timestamps and first observed route states.
Live agent reports identify **phBot 20.1.3**, protocol 14, and plugin **1.9.20**
for all eight profiles during the group/direct runs. Single-character runs preceded
the final all-profile reload. Times below are UTC; the supplied phBot log uses
UTC+2.

| Controlled case                    |        Menu ready | POST launch spread | ACK after admission | Movement evidence                                                             |
| ---------------------------------- | ----------------: | -----------------: | ------------------: | ----------------------------------------------------------------------------- |
| nuker4 short, about 21 units       |           39.1 ms |        one request |             20.6 ms | Arrived in the live tray; fresh position 6451,1143                            |
| nuker4 long, about 183 units       |           28.2 ms |        one request |              8.6 ms | Arrived, first recorded by 09:04:15.511Z; fresh position 6431,950             |
| all eight short, about 14–30 units |           28.1 ms |             2.9 ms |       97.1–189.9 ms | Seven observed arrivals; Kalypso script start rejected                        |
| all eight long, about 165 units    |           71.8 ms |             3.3 ms |        20.0–29.6 ms | Seven observed arrivals, last by 09:12:38.105Z; Kalypso script start rejected |
| direct move_to, all eight          | no menu/preflight |             2.8 ms |        21.4–38.4 ms | All eight fresh positions matched 6439.8,1115.4                               |

The direct batch's first POST started 0.9 ms after the click; all eight HTTP
responses were 202 in 44–46 ms. Invocation results took 40–1092 ms, including
callback scheduling. Generated script-start results took 1.1–2.3 seconds on the
long batch and 1.3–7.0 seconds on the short batch. Path generation and phBot callback
scheduling still contribute latency; removing frontend checks does not make those
native operations instantaneous.

The initial single-character long route was superseded by unrelated operator
commands, so it was excluded and repeated after the operator agreed to leave
characters idle. A temporary seven-target preview during plugin reload was
cancelled without posting. An old-plugin direct command was rejected for missing
capability, and one checkbox automation attempt never enabled the option and
posted nothing. These attempts are not counted as successful movement tests.
Subsequent operator movements after the controlled runs do not change their
recorded arrival evidence.

## Kalypso's native refusal and restart retest

Both group commands reached `generate_script`, passed the local parser and reached
`start_script`. The operator's log reports:

- Short command `cmd_d629d318-293d-4b13-bc88-edc9fe3a9fc2`: generation 1323 ms,
  validation/source readback 0 ms, script start 1 ms, `api_return_false`.
- Long command `cmd_cdcf2d57-e451-4660-978d-675a581b80cb`: generation 580 ms,
  validation/source readback 0 ms, script start 1 ms, `api_return_false`.
- Direct command `cmd_65a1e954-d180-4827-8ae4-402963838dac`: completed/unverified,
  and the subsequent fresh position matched the clicked destination.

The operator observed no separate script error or running-script message. An
additional authenticated Kalypso navigation probe with destination **Z=0**, command
`cmd_07601964-8d3e-43fd-ac9a-1ec33e4f8018`, also returned `api_return_false`
after producing a two-step path. The failure therefore persists when the terrain
height difference is removed. This is not proof of its underlying native cause.

The official [Script API](https://plugins.phbot.org/phbot-api/script) documents
`start_script(str)` but does not explain a False result or provide an active-script
query. PhMon does not stop unrelated scripts, start the bot, retry automatically
or silently replace generated navigation with direct movement to conceal that
refusal. The initial eight-character generated-navigation gate failed and was
repeated after the operator restart as described below.

At the operator's request before restarting Kalypso, **plugin 1.9.21** adds bounded
diagnostics: command-correlated instruction counts, UTF-8 length, SHA-256 prefix,
trailing-newline flag, source/target region and Z, native return type/value class,
and optional `get_status()` labels before/after invocation with timing. `get_status`
is the existing optional installed-runtime compatibility symbol, not a newly
documented active-script API. None/unavailable/unfamiliar text remains unknown;
the diagnostic does not change script contents, admission or execution decisions.
The operator restarted Kalypso and it registered a fresh **1.9.21** session at
**09:31:18.910222Z**. The other seven profiles retained 1.9.20; all report phBot
20.1.3. Subsequent generated script starts returned **True / api_confirmed**:

| Retest                             | Evidence                                                               |          ACK | Script-start result |
| ---------------------------------- | ---------------------------------------------------------------------- | -----------: | ------------------: |
| Kalypso short                      | `cmd_e7685d73-a363-4309-9cb7-e9a14fec447c`, observed arrival           |      27.4 ms |            856.5 ms |
| Kalypso staging, about 98 units    | `cmd_ab711221-0989-4e19-a1a1-33531aed8483`, observed arrival           |      17.0 ms |            814.6 ms |
| All eight short, about 20–30 units | Eight exact command IDs observed arrived; HTTP 202 in 49–50 ms         | 14.7–59.5 ms |           1.1–2.5 s |
| All eight long, about 183 units    | Eight exact command IDs observed arrived; all fresh positions 6431,830 | 14.0–56.1 ms |           1.1–2.0 s |

The final short menu prepared in 67.6 ms. Server admissions spanned 16.2 ms for
short and 22.0 ms for long; these are database admission spans, distinct from the
earlier browser launch measurements. The browser automation session restarted while
waiting for the final long batch, so its exact first-arrival times and frontend
launch spread were not retained. A new authenticated snapshot at
**09:41:26.584Z** verified all eight exact long-command IDs as arrived. Command
completion alone was not used as arrival evidence.

Repeated Kalypso routes also verified the sidebar correction in the deployed
browser: its previous arrival stayed **Arrived / finished** after the newer route
replaced it, with “Arrival observed from a fresh position” detail. After the short
group test, the tray showed ten finished and no active rows. The final reloaded
tray shows all eight long routes arrived.

The restart retest closes the requested real movement acceptance gate. It does not
establish why phBot previously refused the script; no unsupported diagnosis or
automatic reset is added. The additional 1.9.21 diagnostics are available if the
refusal recurs. The operator's earlier failure logs and durable True results are
retained; the new local diagnostic text itself has not been copied from Windows.

Sources checked on 2026-10-04: [Movement](https://plugins.phbot.org/phbot-api/movement)
documents nonblocking `move_to(x,y,z)` returning None; [Paths](https://plugins.phbot.org/phbot-api/paths)
documents generated script lines and its five-second generation limit; the Script
API establishes script-string execution. Existing native signatures are unchanged.

## Validation and visual evidence

- Go vet, race tests and builds passed; PostgreSQL integration tests used the
  disposable `phmon_nav_checks` database, not production event tables.
- Final plugin discovery: **202 tests passed**, including receive-backlog/burst
  bounds, direct invocation without planner/readback, deduplication, argument
  rejection, safe log reasons and diagnostic failure noninterference.
- Final frontend: **187 tests passed**, typecheck and formatting passed; lint has
  zero errors and 69 existing/style warnings. Tests cover eight concurrent direct
  admissions, frozen Z/exact retry and bounded arrival/replacement/session evidence.
- Live transport audit passed. Production server/web builds passed and were
  restarted locally; the final UI build includes retained route observations.
- `scripts/check.sh` passed its Go/Python/frontend/build stages. Its final example
  Compose validation initially lacked the intentionally empty example operator
  secret; the same config check passed with a disposable placeholder secret and
  loopback allowed origin. No credential was committed.
- Deployed browser checks at 1440×1000, 1280×800 and 390×844 found no document
  horizontal overflow or page errors. The checkbox is below the action buttons,
  defaults off and was turned off again after direct testing. Screenshots retain
  live data outside the repository: `/tmp/phmon-nav-final-desktop.png`,
  `/tmp/phmon-nav-final-1280.png`, `/tmp/phmon-nav-final-mobile.png`.
  Final all-eight arrival screenshot: `/tmp/phmon-nav-final-all-arrived.png`.
- The public demo was opened in a real browser, with its recurring connection
  overlay dismissed for inspection. Map navigation remained on Dashboard, so this
  follow-up does not claim a fresh reference Map comparison. Existing repository
  reference captures and the current operator-approved Map layout remain baseline.

The targeted navigation follow-up is complete. If a native False recurs, capture
the command-correlated 1.9.21 diagnostic lines and compare state/script metadata
before changing execution behavior. Remaining roadmap acceptance gates are tracked
separately in AGENTS.md.
