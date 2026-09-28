# Reference parity ledger

This ledger records implementation evidence against the public phMonitor demo
baseline captured in docs/reference on 2026-09-26. Reference screenshots are
inspection evidence only and are never shipped as PhMon application assets.

## Slice 4 — Stats, containers, pets, party and academy

Status: Slice 4 has a live API-backed collection and presentation implementation.
The open gates are full family/blue metadata coverage, packet-based movement retention,
several UI acceptance comparisons and verified Party Setup application. Current
runtime evidence is recorded in the dated entries and
[`item-instance-evidence.md`](item-instance-evidence.md).

The user's supplied Stats captures show two grouped character cards, compact
character identity/status, independent tabs, page controls above a slot-preserving
item grid, occupancy below it, gold footer, and in-game item previews for both a
stackable and an armor item. They are visual requirements, not source API or packet
fixtures. The operator-supplied LAN Stats page was also inspected on 2026-09-27. Its
card tabs are Overview, Info, Progress, Inventory, Storage, Pet and Actions. Info is
the character-set equipment arrangement around the avatar; Progress contains
historical rates and training behavior. The implementation now mirrors those tabs:
equipment is under Info, Progress shows current XP/SP/gold and labels historical
rates unavailable until Slice 12, and Inventory opens directly to the bag. It also
adds separate Party and Academy views for the verified Slice 4 observations. Storage
defaults to personal storage. `/characters/{id}` reuses the same card component,
while the existing Slice 3 Actions lifecycle remains the command source.

Inventory pages use four columns by eight rows and preserve empty positions. The
shared item preview opens on hover/focus without click-to-pin behavior and uses
bounded internal scrolling. Plugin API observations also retain typed whites, blues
and scalar fields; the backend presents verified available values after
dataset/model/code matching. Greatest has an explicit dataset mapping and validated
static catalog. Item packet layouts and unreported values remain unavailable;
first-13 equipment separation retains its explicit runtime-unverified status. See
the live verification ledger below.
Container freshness comes from committed observation timestamps. Storage and guild
storage preserve their last confirmed contents while marking current availability
not observed/unavailable; the UI labels these as last known. Pet type, provided pet
inventory, current party membership and current academy membership are API snapshot
views. Party Setup is visibly read-only until a write/reload/readback contract is
verified.

The plugin and Go backend support protocol-v4 full baselines and revision-checked
deltas, bounded multi-frame assembly, exact agent-generation/session fencing,
transactional resource persistence, idempotent content timestamps, guild/server
scope and filtered resource subscriptions on the existing `/api/live` connection.
Protocol v2/v3 agents remain accepted without Slice 4 resources. Collector fixtures
and Go unit tests cover shape validation, chunk assembly, revision gaps and fencing.

The shell's persisted server selector now scopes fleet summaries and the Stats
character cards. Guild Storage has a dedicated sidebar route and authenticated API
query keyed by explicit server and guild; it shows the newest confirmed observation,
observer identity, item freshness, and searchable source-slot results through the same
item preview. Personal storage remains on the character card. Explicit removal of
stored guild records is not implemented yet.

Local evidence to date: 39 plugin tests, Go tests/vet, Nuxt typecheck, lint, format
check and production build pass; lint retains 13 existing void-element style
warnings. The resource migration integration test compiles but skips without
`TEST_DATABASE_URL`; local PostgreSQL, Docker and race coverage are unavailable.
The supplied reference URL loads its live Stats view and the Info, Progress, Pet and
Storage tabs were inspected. The local app still reaches the operator sign-in gate,
so matching authenticated screenshots have not been reviewed. Do not treat this
reference inspection as proof of API semantics or runtime correctness. At 1440 ×
1000, 1280 × 800, 390 × 844 and the reference-native viewport, review overview, bag,
equipment, personal/guild storage, pet inventory and both supplied tooltip families
when a local authenticated browser/runtime fixture is available. Reference backdrop
and legally usable local item icons remain gaps.

## Slice 1 — shell and instance access

Status: Slice 1 shell implementation and automated CI validation complete; required
same-viewport browser comparison remains BLOCKED in the current execution environment.

### Implemented behavior

- Desktop shell uses the reference proportions: approximately 228 px navigation rail
  and 34 px top strip, with compact rows and controls rather than a generic SaaS
  landing page.
- Semantic colors start from primary #FEF6C3, panel #0D131D and text #EAF1FF, with
  near-black/navy surfaces, blue-gray borders, subdued secondary text and restrained
  status colors.
- Segoe UI, Tahoma and Arial are the primary system font stack; the page title and
  dense navigation/table sizing follow the recorded baseline.
- Sidebar grouping exposes the known product areas without pretending later slices
  are implemented. Future screens are disabled and advanced-only tools appear only
  when advanced mode is enabled.
- Easy/advanced mode and desktop sidebar collapse persist as same-site preferences.
  They are presentation state only and grant no authorization.
- Mobile navigation becomes off-canvas; agent rows reflow into compact cards instead
  of forcing whole-page horizontal overflow.
- Instance access uses the current PhMon origin for copy/QR. The QR never contains an
  agent ID, token or other credential.
- Backend readiness remains a calm top-strip diagnostic. Agent API failures retain the
  last successful list and show stale/unavailable state instead of a recurring modal.
- The Slice 1 dashboard is backed by GET /api/agents and shows real connected state,
  stable agent ID, plugin/phBot/protocol versions, connection age and last-seen time.
  It can also create a new agent ID/token pair through a no-store same-origin POST;
  the plaintext token remains visible only in the current provisioning panel and is
  not recoverable later. No fabricated monitoring data is used.

### Evidence and deliberate deferrals

Backend evidence is the authenticated /agent protocol, PostgreSQL agent metadata and
generation-fenced active registry. Plugin evidence is PhMon.py plus the shared
simulator contract. UI evidence is the Nuxt shell and same-origin /api/agents route.

The neutral dark backdrop is deliberate Slice 1 development treatment; approved
game-world artwork is still required before final visual completion. Character/server
data, dashboard game panels and server scoping depend on later owning slices and are
not represented as working controls.

Hosted CI is green for the Slice 1 transport/test head, including the complete Docker
agent lifecycle and database outage/recovery path. That automated evidence does not
substitute for same-viewport visual comparison.

Required browser evidence remains open because this execution environment cannot run
the complete local Nuxt/Docker browser stack:

- 1440x1000: BLOCKED — no screenshot captured or side-by-side comparison observed.
- 1280x800: BLOCKED — no screenshot captured or side-by-side comparison observed.
- 390x844: BLOCKED — no screenshot captured or side-by-side comparison observed.

The sidebar contract calls for a PhMon version/build identifier. The current
"self-hosted · slice 1" text is a development/slice label, not an application version.
No canonical PhMon application version/build value currently exists in package
metadata or runtime configuration. A hardcoded invented version would be misleading,
so the version portion of the parity ledger remains explicitly open until canonical
release/build metadata exists.

Real Windows/phBot runtime validation is also still open. The simulator exercises the
same production wire contract but is not evidence that the embedded phBot runtime has
loaded and operated the plugin successfully.

## Slice 2 — character overview and detail

Status: Slice 2 UI implementation and PR #3 correctness follow-up are validated.
Visual parity and remaining real phBot lifecycle/data checks remain open.

### Reference evidence reviewed

- Reopened `docs/reference/phmonitor-dashboard.png` and
  `docs/reference/phmonitor-stats.png` (1440 × 1000). Dashboard uses an asymmetric
  panel layout with fleet character counters, last deaths, server information, a
  broad recent-events panel, rare-drop/chat stack and offers. Stats opens with a
  compact thematic header and a large grouped-character surface; its empty state
  offers a first-group prompt rather than implying characters have loaded.
- These saved captures are from the recorded 2026-09-26 public-demo inspection. The
  live demo and advanced character subtabs were not independently re-exercised in
  this slice; hidden details are not inferred from these screenshots.

### Implemented and evidenced behavior

- Go/PostgreSQL automatically resolves joined identities by normalized server plus
  character name, tracks generation-scoped live sessions/current stats and closes
  sessions on leave, replacement, disconnect and backend startup. An explicit identify
  claims per-character session authority; snapshots replace current fields and clear
  unavailable values rather than making old-session state appear fresh.
- Protocol v2 identifies a character before registration, then requires explicit
  `character_id` on each snapshot/state/left message. The plugin reports documented
  state fields and sends full snapshots after reconnect.
- Character overview sends debounced server/name/guild/zone search to the backend,
  keeps persisted group filtering and membership controls, and separates logical
  connected-agent counts from socket counts. Stable detail path is
  `/characters/{character_id}`. It shows known state and labels later inventory,
  pets, party, map and action areas as not yet implemented.
- Empty character state explains automatic phBot discovery. Group operations are
  persisted but remain organizational metadata.
- No external phMonitor assets or service calls were introduced. The neutral
  development backdrop remains; approved local artwork and final visual treatment
  remain open.

### Dashboard/header visual follow-up (2026-09-26)

- Compared the local dashboard render against the tracked
  [Dashboard](reference/phmonitor-dashboard.png) and [Stats](reference/phmonitor-stats.png)
  baselines, alongside the user-supplied detailed captures. Added the
  reference-style three-part live summary in the 34 px top strip (online/offline
  characters, combined HP/MP percentages and compact total gold), moved the
  easy/advanced toggle to the right edge, added a distinct Dashboard header and
  asymmetric dashboard panels, and added the reference sidebar QR/copy-link utility.
- Dashboard counters, vitals and gold use observed character records. Alive/dead,
  deaths, server information, event timeline, rare drops, chat and global offers
  are explicitly labeled `LATER`; they are owned by later slices. Missing character
  data displays as unknown (`—`) instead of implying zero.
- Added a working Dashboard ↔ Stats navigation link. The Stats page keeps the existing
  searchable/group-manageable character surface; the user explicitly excluded its
  table and the phBot agents section from this visual comparison. Advanced navigation
  now follows the observed Tools/Misc order and marks unimplemented destinations
  `LATER`. Server scope remains visibly deferred.
- Browser comparison was made from the local dev build at 1264 × 710, with the
  backend unavailable in that isolated shell. Layout and unavailable states were
  reviewed, and Stats navigation was clicked and verified. This is not a populated
  data screenshot or the required 1440 × 1000 same-viewport evidence. The current
  `192.168.10.25:3005` tab still serves the prior build; Docker is unavailable in the
  shell, so that instance was not rebuilt. No phMonitor artwork or external assets
  were copied; the dark backdrop remains an explicit visual gap.
- Local checks after the follow-up: Nuxt typecheck and production build passed;
  ESLint passed with the three existing HTML-input self-closing warnings; Prettier
  passed for the changed files. Full `format:check` was also invoked and remains
  red because 19 unrelated existing files in `web/` are not formatted.

### Comparison and gaps

- A compact character table now appears above the existing agent/operations panels;
  the reference's deaths, server-information artwork, events, drops/chat and offers
  remain owned by later slices. This is the Slice 2 state surface, not complete
  dashboard parity.
- Local overview evidence: [1440 × 1000](evidence/slice2-overview-1440x1000.png),
  [1280 × 800](evidence/slice2-overview-1280x800.png) and
  [390 × 844](evidence/slice2-overview-390x844.png). The 1440 capture was reviewed
  against the saved dashboard/stats references. At 390 px, document/body scroll width
  equals the viewport width; the mobile navigation remains collapsed and content
  stacks without page-level horizontal overflow. [Character detail at 1440 × 1000](evidence/slice2-character-detail-1440x1000.png)
  has no direct reference capture; its hierarchy follows the shared shell while later
  panels remain explicit gaps.
- Botting/training remains unknown because official docs expose start/stop mutations
  but no read-only getter. Real phBot 20.1.1/plugin 1.1.0 evidence separately shows
  server, character name, zone, level, HP/MP, XP/SP, gold, region and position in live
  records; it does not validate every lifecycle edge or botting state. The PR review
  fixes were exercised through the deterministic simulator over the production
  transport, not manually revalidated on phBot.

The review follow-up also verifies stale character-scoped state/snapshot/leave
rejections over real WebSockets: the old socket remains usable for its unrelated
character while the new observer retains the taken-over character. Compose CI runs a
two-socket outage scenario: one same-agent character disconnects during a real
PostgreSQL stop, the second socket remains live, and recovery reconciliation closes
only the dead generation. Both are automated simulator/backend evidence, not new
real-runtime validation.

## Slice 3 — remote commands planning evidence

Status: Slice 3 Actions and Client surfaces are implemented on `codex/slice-3-plan`;
automated command smoke and a live LAN browser check pass. Responsive behavior was
visually reviewed at the reference and required responsive viewports below. Broad
real mutation coverage remains open, including Walk traversal.

The supplied 2560 × 1315 Stats/Actions and phBot Client captures establish a compact
two-column Actions grid and a dedicated `phBot | Client` surface. Slice 3 preserves
the observed action ordering: Start/Stop Training, Start/Stop Trace, Training
Area/Radius, Return Scroll/Walk and Disconnect. The screenshot's Execute Script slot
remains a later-slice placeholder; arbitrary script execution is not part of the
remote-command API. Party Setup remains Slice 4.

The Client reference is adapted to an explicit selected character. PhMon will not
implement the reference text's machine-wide `sro_client.exe` termination behavior.
Clientless stays visibly unsupported until a safe per-instance phBot mutation is
verified. Reference captures are layout evidence only; hidden dialog semantics that
cannot be observed are not invented.

P0 application safety policy is now frozen in `docs/protocol.md`: exact
character/session targeting, v3 per-runtime capabilities, same-region walking,
finite numeric bounds, durable command lifecycle, honest bool/void verification,
operator authentication, and no automatic command replay. These constraints are
product safety boundaries rather than claims about undocumented phBot limits.

### Slice 3 Actions and Client comparison (2026-09-27)

Primary captures [`02-stats-05.png`](../phmonitor_screenshots/02-stats-05.png) and
[`03-phbot-tools.png`](../phmonitor_screenshots/03-phbot-tools.png) were opened at
their native 2560 × 1315 resolution. The Stats capture shows per-character compact
tabs with Actions selected and a two-column action grid in this order: Start
Training / Stop Training, Start Trace / Stop Trace, Set Training Area / Set Training
Radius, Return Scroll / Walk, Disconnect / Execute Script. The current local detail
surface now has a reusable compact two-column Actions grid, fixed target/server/
session label, capability reasons, dialogs for trace/area/radius/walk and live command
history. Group selection remains presentation only and never changes the target.
Walk treats entered coordinates as a destination and follows the route from phBot's
documented path finder, stepping same-region waypoints on callback ticks and checking
arrival against live position. Teleport paths are not used.

The supplied Client image shows a wide introductory panel, narrow local tool menu,
Go Clientless action and inset result panel. It describes killing every
`sro_client.exe`; the PhMon page instead selects one live character and disables the
button when the runtime does not report a safe per-instance API. Party, Scripts and
Quest are explicitly marked later work. No process kill, local broker or Windows
command is used.

The public [demo](https://phmonitor.com/demo) was opened on 2026-09-27. Its accessible
page markup exposed the Client section and labels for Client/Party/Scripts/Quest but
did not expose an interactive action dialog or trustworthy Set Training Area form
semantics. No hidden paid content or private protocol was inspected. The local area
form offers callback-time current position, explicit coordinates in the current
region, and a manually entered named area only when the installed plugin reports the
officially documented `set_training_area(name)` primitive. The name is not inferred
from unavailable game data; the UI asks the operator for an exact phBot area name.

The disposable Compose command smoke verifies accepted and completed states through
the production worker with a fake adapter. Separately, the operator authorized one
real `nuker1` check on phBot 20.1.1/plugin 1.1.0: setting its already-observed
training radius to 20 completed as `observed`, and live `get_training_area()` readback
remained 20 on the same session. This is narrow runtime evidence, not simulator
evidence or validation of the other actions. Local screenshot comparison at the required
1440 × 1000, 1280 × 800, 390 × 844 and reference-native 2560 × 1315 was reviewed in
the authenticated LAN browser on 2026-09-27. At 1440 × 1000 the Stats table contained
its horizontal scroll and the Client screen retained its local-menu/result hierarchy.
At 1280 × 800 the Client panels remained visible without page-width overflow. At
390 × 844 the Client form fit the viewport and the character Actions grid remained
two columns; command history was available by vertical scrolling. At 2560 × 1315,
the Actions grid ordering and Client surface hierarchy remained consistent with the
reference captures. Walk was disabled because the live agents report plugin 1.1.0,
below the pathfinding capability version; Execute Script remained disabled. The
reference artwork backdrop is still absent from the local UI. Screenshots were
inspected in-session; they were not saved as repository artifacts. The saved evidence
from Slice 2 is not presented as evidence for these new screens.

The 390 × 844 Stats screenshot also exposed that the shared `.agent-table` mobile
hide rule concealed the character table, with no mobile character-card replacement.
The CSS rule is now scoped to the Agent panel, leaving the character table visible
inside its bounded horizontal scroll container. The browser smoke now asserts that
the mobile character table has a visible row and remains inside that container. A
CI run 36330451436 passed the mobile browser assertion for visible character rows inside the bounded table scroller. The in-session LAN review covered responsive sizes, but no post-fix screenshot artifact was saved.

On the selected nuker1 session, `/api/live` delivered capability reason
`unsupported_runtime_primitive` for Clientless; the Clientless button stayed disabled
and no command was submitted. This is real-runtime capability reporting evidence,
not a Clientless action test.


### Evidence — 2026-09-27 Slice 4 item correction

Scope remains Slice 4 only; Dashboard table preserved. Added explicit Greatest
item dataset mapping, read-time backend resolver, independent exporter static
metadata and compact-catalog builder. ItemSlot renders local images and white
normal/gold rare borders/titles, ordered static fields and deliberate numbers.
14,238 model definitions reference 3,411 verified local PNGs. Plugin 1.2.1 shows
loaded version in QtBind and retains bounded lossless API evidence in api_fields.
User transfers the plugin into phBot; no real-character commands were issued.

Checks: plugin 40 tests; exporter 26 tests with PYTHONPATH=exporter/src; Go all
package tests/vet; Nuxt typecheck/build passed, lint warnings only. Server/web
initial update deployed preserving PostgreSQL. Browser confirmed 93 images,
zero broken, white normal/gold rare borders, Casque and necklace static fields.
Follow-up web build hit ENOSPC; inspect and remove only unused build cache before
retrying. Operator signed in again after backend restart; keep backend running.

Open: live rolled stats, percentages, max durability and blues still unverified;
passive packet decoder not implemented. Slice 4 is NOT complete. Next after user
transfers 1.2.1: inspect actual api_fields; if getter lacks instance data, finish
generic vSRO 1.188 decoder against captured item-only fixtures. No guessed stats.

Follow-up deployment recovered after pruning 3.528 GB of unused Docker build cache. Final web image built and restarted; backend and PostgreSQL remained running. Browser checked 1440x1000, 1280x800 and 390x844; mobile pinned tooltip fits, Escape dismisses, no page-wide horizontal overflow at 1280.


### Evidence — 2026-09-27 rolled item detail implementation

Plugin release is now 1.2.2 and shows its loaded version in phBot. It adds bounded,
off-callback decoding for the corroborated 0x3040/0x3052 item updates, with session,
slot and model reconciliation. Any unclassified 0xB034 operation or ambiguous state
invalidates the cache. No inventory action or packet injection is used.

Backend read-time metadata now computes family-specific roll-quality percentages from
the 5-bit variance fields, preserving 64-bit variance exactly. The shared tooltip
shows those percentages as quality and distinguishes unobserved instance data. Blue
values remain raw in storage and are displayed only when a validated dataset supplies
their label/unit/scale definition. The rebuilt Greatest dataset contains 615 raw
magic-option records (298 with source ranges), but no exact localization labels or
verified scales/units; named blues and absolute rolled stats remain unavailable.
Maximum durability and inventory-move retention are also open. See
[`item-instance-evidence.md`](item-instance-evidence.md) for source-by-source support.

Validation: 48 plugin tests, Go package tests, Nuxt typecheck and production build
pass; lint has the existing self-closing HTML warnings. Captured packet layouts,
formula vectors and real browser tooltip values remain runtime acceptance gates after
the user transfers plugin 1.2.2. No synthetic fixture is presented as live evidence.

Runtime follow-up after operator transfer: the signed-in Stats page recovered and
showed four live characters with current inventory quantities. The backend confirmed
two agents reporting plugin 1.2.2, phBot 20.1.1 and agent protocol v4. At 20:49 UTC,
239 item rows were persisted, but none had an `instance` object; all four latest
`item_enrichment` observations reported `not_observed`, `protocol=unknown`, and
`reason=passive_item_packet_decoder_not_enabled`. This does not match the parser-enabled
workspace payload shape, so plugin version reporting alone cannot establish that the
correct file contents are loaded. The runtime validates ordinary inventory ingestion,
not item packet parsing or rolled tooltip details. See
[`item-instance-evidence.md`](item-instance-evidence.md) for counts and the next gate.

### Evidence — 2026-09-27 persistent live WebSocket

Dashboard/Stats navigation previously tied the shared live socket lifetime to mounted
page consumers. Moved start/stop ownership to the authenticated default layout, which
persists across those routes. Route components continue to add and remove their own
subscriptions while sharing the one app-level socket; reconnect now waits until a
closing socket has completed its close event. After deploying only the web service,
the signed-in browser navigated Stats → Dashboard → Stats. Playwright observed no new
WebSocket event on either transition, and Dashboard fleet data plus Stats characters
and inventory remained visible. Go server and PostgreSQL stayed healthy and were not
restarted.

### Evidence — 2026-09-27 item parser reupload follow-up

At 21:03 UTC, two live agents reported plugin 1.2.2 / phBot 20.1.1 / agent protocol
v4. Four characters and 242 API-backed item rows remained current, with zero instance
objects. The old `passive_item_packet_decoder_not_enabled` payload was stale: resource
persistence intentionally retained prior payload JSON whenever availability was
`not_observed`, so it could not identify the loaded parser build. The workspace parser
file had `PassiveItemTracker`, while the plugin file in the deployed checkout was the
pre-parser copy. Added a unique parser build marker to plugin 1.2.3 and made the
backend refresh only `item_enrichment` diagnostic payloads on `not_observed`; item
contents and observed timestamps retain their prior rules. The exact phBot file/runtime
still needs confirmation. Item details remain unavailable until matching item packets
arrive naturally.

At 21:24 UTC, all four active characters reported plugin 1.2.3 / phBot 20.1.1 / agent
protocol v4, with 251 persisted API-backed item rows but no instance rows. The latest
diagnostics now proved the parser build was loaded, and isolated protocol selection as
the failure: every character reported `protocol=unknown`. Inspection of the active
phBot installation showed `vSRO.json` beside (not inside) `Config`, with the current
server nested in a root profile map. The detector now handles that official runtime
configuration shape, ignores `version=296`, and recognizes the selected 1.188 variant
flags. Plugin 1.2.4 emits a unique `r2` marker and protocol reason; transfer and live
confirmation remain pending. No phMonitor executable or private protocol was
inspected.

### Evidence — 2026-09-27 authorized local item-tooltip inspection

The operator subsequently authorized inspecting the local phMonitor executable for
item tooltip semantics. Its embedded inventory path reads API item attributes and
blues and combines them with static reference data. PhMon's collector was dropping
integer dictionary keys and omitted some attribute aliases. Version 1.2.5 preserves
those values as uninterpreted evidence and exposes bounded field-type diagnostics;
52 plugin tests pass. The release file is staged locally and at `/var/www/phmon/plugin/PhMon.py`
with matching SHA-256. The live plugin still requires operator transfer. No UI or
backend calculator changed in this increment. Reference formula discrepancies and
the precise next gate are documented in
[item-tooltip-investigation.md](reference/item-tooltip-investigation.md).


## 2026-09-27 live API tooltip verification

Deployed API-backed percentages/blues verified in the logged-in Stats browser:
Python Casque (12/22/19/3/32/9, Int3/MP5) and Tiger Bone Coronet
(61/45/32/0/9/0, Steady2/Parry5%) display together in separately pinned tooltips.
Absolute values and maximum durability explicitly remain unavailable. Other item
families/blue definitions still need verification. This was live data, not simulator
content. Full viewport comparison and rolled-stat parity are not complete.

## 2026-09-28 plugin 1.2.6 item value comparison

The logged-in deployed Stats page now shows Python Casque physical/magical
defense 54.8 (+3%) / 73.3 (+32%), durability 77/77 (+9%), parry 23 (+22%),
reinforcement 13.9% (+12%) / 18.2% (+19%), and Int3/MP5 blues. Phoenix Horn
Spear shows observed attack ranges 375–435 (+16%) / 640–757 (+0%), durability
64/64 (+19%), attack rate 124 (+12%), critical 4 (+3%), and reinforcement
88.5%–105.4% (+6%) / 152.7%–186.7% (+22%). These values were checked against
the supplied in-game screenshots, allowing for the plugin's measured raw value
and one-decimal display precision. The tooltip remains independently pinnable
per card, with no maximum height or internal scrollbar. Advanced elixir and magic
option-capacity lines remain absent because the live API does not provide them;
blue roll-quality percentages and full viewport comparison remain open.

## 2026-09-28 item-slot interaction correction

Current item previews open on hover or keyboard focus only. Clicking an item slot no
longer pins or toggles the tooltip. Enhancement overlays are white for normal items
and gold only for metadata-confirmed rare items; the rare item border and title remain
gold. This corrects the live Stats page comments. The web-only rebuild/restart is
healthy and the deployed Stats route loads four live characters and their
inventories; a manual pointer-hover comparison remains to be recorded.
