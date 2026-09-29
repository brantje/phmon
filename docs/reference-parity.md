# Reference parity ledger

This ledger records implementation evidence against the public phMonitor demo
baseline captured in docs/reference on 2026-09-26. Reference screenshots are
inspection evidence only and are never shipped as PhMon application assets.

## Cave-floor map reference inspection — 2026-09-28

In the operator-supplied phMonitor v0.5.0 map at `192.168.10.105`, Quick navigation
to Donwhang Stone Cave opened a cave image and a compact bar inside the bottom of
the map viewport. The bar contained Back to world map, the cave name, and 1F–4F;
1F was highlighted initially. Clicking 2F and 4F visibly replaced the map image
and moved the active highlight. The same structure appeared for Tomb of Qin-Shi
with B1–B6 and Job Temple with 1F, 2F and Annex 1–5. Back to world map removed
the floor bar and returned to outdoor imagery. The existing layer controls stayed
beside the map. This is visible interaction evidence, not access to the reference's
coordinate implementation or asset catalog.

The cave readout displayed X -24,288.0, Y -96.0 and 135% on entry to Donwhang
Stone Cave; selecting 2F and 3F retained that displayed X/Y and zoom while the
floor image changed. Tomb of Qin-Shi displayed X -23,232.0, Y 194.0 and 135%; Job
Temple displayed X -21,407.0, Y -1.0 and 135%. After Back to world map, the
readout displayed X 113.0, Y 54.0 and 125%. A scroll over the Donwhang cave map
changed the readout to X -24,286.4, Y -305.7, tile
`dh_a01_floor01_128x126.webp`, and 116%. These values show that the reference has
an active-view coordinate/tile/zoom readout and separate cave view presets. They do
not establish a conversion to phBot region/X/Y/Z, marker placement, or safe map
command targets. PhMon must validate those independently per floor before enabling
coordinate-based actions.

## Death status and occurrence history — death increment during Slice 4

The supplied `phmonitor_screenshots/02-stats-death.png` shows a death marker on the
Stats navigation item and a saved group, with separate Dead/Alive and Online badges
on the character cards. The implementation now derives badge state from fresh,
boolean live state and keeps presence separate. During page subscription sync, cards
reuse the app-wide last-known death sample, keyed by server, character and session.
Fleet, filtered-list, group and detail snapshots hydrate from that cache; a session
change invalidates the prior value, and the original sample timestamp controls
freshness. The same cache feeds Dashboard, Stats and group indicators. Missing,
expired and offline samples show Unknown. When the transport is stale but the
character sample remains within its freshness window, its cached state remains
visible alongside the stale connection indication. Grouped character links render
under Stats and show a death dot when a fresh, online group member is dead.

The supplied `phmonitor_screenshots/02-dashboard.png` shows online/offline/alive/dead
counts, Last Deaths, Recent Events, server information, rare drops, chat and offers.
Dashboard now fills the four presence/death counts and the two event panels from
PostgreSQL-backed live subscriptions; unrelated Slice 5/6/13 panels keep their
future-slice state. The supplied `phmonitor_screenshots/05-deaths.png` shows compact
event-type tabs, character/date filters, count badge, timestamp/character/reason/
location/map columns and pagination. Events → Deaths follows that hierarchy and
supports server scope, character search, inclusive date range and cursor pagination.
The source only supplies `EVENT_DIED` with empty callback data, so cause is unknown.
Coordinates are recorded when observed, but the map action remains unavailable
until a region transform is validated in Slice 7.

Death observations live in the nullable `characters.dead` field. Only phBot's
`get_character_data()['dead']` boolean updates current status; callback type 7 creates
an occurrence. HP and snapshots do not backfill historical deaths. Protocol v5 adds
the event frame/ACK and bounded local spool while accepting v2–v4 agents. The server
accepts replay only when the referenced durable session belongs to the authenticated
agent and stated character, then deduplicates by stable event UUID.

Visual comparison is structurally checked against the three supplied captures. This
increment still needs local captures at 1440×1000, 1280×800 and 390×844 and the
keyboard/focus/whole-page-width review before screenshot parity is accepted. No real
character was operated; actual phBot death callback validation remains open. This
was the original death-only implementation; the current canonical pipeline is
recorded in the Slice 5 entry below.

## Slice 5 — Canonical event pipeline (2026-09-28)

Live level-up correction (2026-09-29): the Greatest instance showed four
`Reached level 71` rows for nuker1–4, while the character details showed level 72.
The stored callback payloads were all `{"level":71}` and came from phBot 20.1.2
with plugin 1.5.0. For this phBot version, server ingestion now preserves
`callback_level:71` and stores the reached `level:72`; migration 000015 corrects
existing rows. The live deployment still needs the migration and updated server
before this UI evidence can be closed.

### Reference inspection

The public demo was opened at `https://phmonitor.com/demo` in the in-app browser. The
History page exposed All, Level Ups, Custom, Deaths, Rare Drops, Normal Drops and
Uniques tabs, a result count, empty-state text, page-size selector and previous/next
controls. The Rare Drops screen used Item, Time, Character, Location and Map columns.
The Alchemy screen exposed Live log and Sessions, an attempt summary and an empty
session table with Item, Highest Plus, Failures and Total attempts. The server status
was Disconnected and its recurring `remoteToken is required` overlay was hidden only
for the History inspection; it returned after navigation and was not investigated.
Easy/advanced mode was toggled once, but saving failed on that same demo connection.
The browser default viewport was not measured, so this inspection is structural
evidence only and is not a matching screenshot comparison.

Reference captures supplied with the repository remain the visual baseline:
`phmonitor_screenshots/05-history-01.png`, `05-rare-drops.png`,
`05-normal-drops.png`, `05-uniques.png` and `05-alchemy.png`. The public page used
“History”; the independent PhMon implementation uses Events navigation and History
headings inside its own shell.

### Implemented behavior and evidence

- Protocol v6 batches generalized the v5 death event frame. Migration
  `000007_event_pipeline.sql` adds nullable agent-level context, sequence, dedupe key
  and indexed item identity. The Go backend authenticates agent ownership, fences
  character/session scope, validates bounded kind-specific payloads, commits a batch
  transaction and returns per-event results after commit. v2–v5 compatibility remains.
- The profile-scoped spool upgrades pending death rows, retains event IDs through
  retry and reserves 512 / 8 MiB for critical events plus 2,048 / 16 MiB for ordinary
  events. Callback and worker queues stay bounded; overflow/disk failures set status
  and log. Process termination before worker spooling remains a documented loss window.
- All documented `handle_event` IDs 0–10 are normalized. Rare and normal drops stay
  separate and include only the documented model ID. `alchemy_update` emits attempts;
  `EVENT_ALCHEMY_FINISHED` emits completion. Chat preserves raw type and bounded text
  under `channel:"unknown"`. Official source and runtime limitations are in
  [`phbot-capabilities.md`](phbot-capabilities.md).
- Continuous identity-aware party, academy, pet and owned-container snapshots produce
  join/leave, summon/dismiss and quantity/transfer events. Initial state, container
  set changes and sample gaps reset baselines. Tests cover bag split/merge, positive
  bag and job-pouch deltas, opening storage, pet summon and pet-to-bag transfer.
  Causes remain unknown where source evidence does not prove them.
- `/api/events` and the live `events` stream share server, character, kind, category,
  item, date and cursor filters. The Events page has seven tabs, compact filters,
  counts, item and character context, cursor pagination and disabled map actions
  pending validated transforms. Dashboard recent events and rare drops use this
  stream. `/alchemy` presents attempts, known outcomes, highest observed plus,
  item/character/date filters and cursor paging. Academy membership transitions are
  available in this history. Slice 6 can consume `chat.message_received`.
- Dashboard Recent Events groups identical chat messages from the same sender, raw
  type and server when distinct characters observe them within two seconds. One row
  lists all observers; the canonical event history retains each observation.

Verification passed: `go test ./...`, `go vet ./...`, `go build ./...`,
`python -m py_compile plugin/PhMon.py`, `python -m unittest plugin.test_phmon`
(71 tests, including spool capacity and disk failure), `npm run test:unit` (8 tests,
including cross-character chat grouping),
`npm run typecheck`, `npm run lint`,
`npm run format:check` and `npm run build`. ESLint reports 22 non-fatal
`vue/html-self-closing` warnings. Database integration tests compile but skip because
this workspace has no `TEST_DATABASE_URL`, Docker CLI or disposable PostgreSQL;
migration execution, database batch atomicity/replay/filter queries and the complete
plugin→Go→PostgreSQL→UI simulator flow therefore remain unverified.

Local authenticated UI smoke checks used an empty, temporary loopback fixture backend;
they verify page structure and navigation only, not persistence or live data. The
Events and Alchemy captures are saved under `docs/reference/local/` at exact 1440×1000
and 1280×800 content viewports. The Firefox tooling clamps its minimum window width
to 500 CSS pixels, so the narrow captures are 500×844 and do not satisfy the required
390×844 comparison. At the rendered desktop sizes and the 500-pixel narrow size,
`document.documentElement.scrollWidth` matched the viewport width; the event table
scrolls within its own region on narrow screens. The local screenshots show fixture
empty states and are not presented as a populated backend flow. A narrow navigation
keyboard smoke check confirmed Tab reaches the visible menu button, Enter opens the
navigation and focuses its first link, and Escape closes it and restores focus. A full
app-wide focus review and exact 390-pixel comparison remain open.

The current phBot process was not inspectable through the available UI surface, so no
callback was observed on the installed runtime. Slice 4 Party Setup, inventory-slot,
packet, item-family and visual gates also remain open; Slice 5 must not be marked
complete until those dependencies and its database, exact mobile viewport and runtime
acceptance checks are resolved.

Local evidence:

- Events, desktop: [1440×1000](reference/local/slice5-events-1440x1000.png),
  [1280×800](reference/local/slice5-events-1280x800.png)
- Events, narrow tool viewport (500×844):
  [capture](reference/local/slice5-events-browser-500x844.png)
- Alchemy, desktop: [1440×1000](reference/local/slice5-alchemy-1440x1000.png),
  [1280×800](reference/local/slice5-alchemy-1280x800.png)
- Alchemy, narrow tool viewport (500×844):
  [capture](reference/local/slice5-alchemy-browser-500x844.png)

## Slice 4 — Stats, containers, pets, party and academy

Status: Slice 4 has a live API-backed collection and presentation implementation.
The open gates are incomplete/unverified family and blue semantics, packet-based
movement retention, several UI acceptance comparisons and verified Party Setup
application. Current runtime evidence is recorded in the dated entries and
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
item preview. Personal storage remains on the character card. A typed-confirmation
purge now removes all saved guild-storage snapshots and normalized item rows for the
selected server/guild only; the operator must authenticate and pass the exact guild
name through the server API. It does not alter in-game storage. A later agent
observation can create new saved data. The operation is logged with scope and row
counts; it does not keep a durable audit-history record.

Local evidence to date: see the 2026-09-28 audit below. The authenticated deployed
PhMon tab was stale after backend restart; current equipment was unavailable. A
matching 1440 × 1000 capture could not be obtained from the local tab, and the Stats
group/data differed from the two-card reference. At 1440 × 1000, 1280 × 800 and
390 × 844, recheck the authenticated local app with fresh data for overview, bag,
equipment, personal/guild storage, pet inventory and both supplied tooltip families.
Reference backdrop and legally usable local item icons remain gaps.

### 2026-09-28 Slice 4 evidence and implementation audit

- Official [party API](https://plugins.phbot.org/phbot-api/party) documents only
  current membership through `get_party()`. Official
  [config API](https://plugins.phbot.org/phbot-api/config) warns that direct JSON edits may be
  overwritten; [misc API](https://plugins.phbot.org/phbot-api/misc) documents
  `set_profile()` as changing the active profile, not Party Setup. No supported
  Party Setup setter, reload contract or effective-state readback was found. The
  party card now separates membership from an unavailable, read-only Party Setup
  section and keeps its edit control disabled. Required evidence to enable edits:
  installed-version documentation/runtime proof for exact configuration fields,
  supported writer, reload operation, and post-reload readback, exercised through
  the authenticated command lifecycle.
- The flat inventory getter still does not document an equipment boundary. The
  first-13 mapping remains `adapter_lead_runtime_unverified`; the equipment view now
  explains that equipment labels and remaining bag capacity may be wrong. No
  independent runtime/API evidence establishes its indices.
- Plugin 1.2.6 live API captures support exact observed white fields and blues for
  the captured families; family mappings, scalar fields, unknown-blue passthrough
  and evidence limits are detailed in `item-instance-evidence.md`. The passive item
  parser still relies on corroborating layouts without a live Greatest packet
  fixture. Safe fail-closed invalidation is implemented and fixture-tested;
  movement/transfer retention is unsupported pending verified subtype layouts and
  identity semantics.
- Guild-record purge is scoped to saved PhMon observations and normalized slot rows
  for exactly one server/guild. Authorization is enforced on the Go DELETE route,
  the client requires the literal guild name, and the response reports deleted
  counts and future-observation behavior. It never issues phBot or in-game item
  operations.
- Visual review used the reference Stats view and deployed PhMon at the available
  browser sizes. PhMon was stale, showed four “Nukers” characters instead of the
  reference's two and had no equipment observation. The 1440×1000 viewport override
  was not applied to the deployed tab, so this is not a matching viewport capture.

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

## 2026-09-28 live death status and event deployment

The server and web implementation for protocol v5 death state and the `character.died`
event path is deployed at `10.25`. Both `/readyz` and `/api/health` returned
`status=ok,database=ok`, and PostgreSQL contains `activity_events` from migration
`000006`. Three connected agents report plugin `1.3.0` / protocol 5; five online
characters currently have fresh Alive state. The callback event table is empty because
no natural death callback occurred during verification. Death cause remains unknown,
and no character was operated to generate a test death.

This verifies deployed service health, migration and live plugin/state compatibility.
It does not close screenshot parity: no authenticated review browser was available for
same-size captures at 1440×1000, 1280×800 and 390×844. Compare
`phmonitor_screenshots/02-dashboard.png`, `02-stats-death.png` and `05-deaths.png`
against the deployed Dashboard, Stats/sidebar and Events → Deaths pages once a
signed-in browser is available. The Map action stays disabled without verified region
coordinate mapping. Other event kinds remain assigned to Slice 5.

### 2026-09-28 PR #11 review corrections

CodeRabbit review identified event durability and UI-boundary issues. The plugin now
preserves the last registered session across backend disconnect cleanup, defers
binding only when no character session is registered, fences callbacks observed
against a different registered character, and stores each profile's spool in a
profile-keyed file. Deferred binding is persisted locally before transmission and
accepted server-side only when no competing same-agent session covers the event
time. Invalid permanent event payloads now receive a terminal rejection ack.

Death date filters now send local-midnight RFC3339 bounds to HTTP and live event
queries, while the server preserves date-only UTC compatibility. The sidebar only
marks Deaths as the current page when Events is active. Focused plugin and Go tests
cover reconnect context, deferred binding, per-profile spool identity, timezone
bounds, error classification and session fencing. PR CI/CodeRabbit recheck remains
pending until these changes are pushed.

## 2026-09-28 selected-server API scope audit

The server selector persists in the browser. Audited the API and live streams that
carry server filters. Character lists (`GET /api/characters` and the `characters`
stream), saved groups (`GET /api/groups` and the `groups` stream), and character
detail (`GET /api/characters/{id}` and the `character` stream) now use an exact,
case-insensitive server match in PostgreSQL. Group membership snapshots contain
only characters from the requested server; groups with members only on other servers
are omitted. The group stream retains the current browser scope when reconnecting.
The character picker and command target picker apply the same scope, and character
detail returns not-found when its ID belongs to a different selected server.

The other server-filtered APIs were already scoped: `GET /api/events` plus the live
`events` stream filter event rows by server, while `GET` and `DELETE
/api/guild-storage` require the exact server and guild pair. The shared validator now
applies the same length/NUL checks to event, character, group and detail filters.
`/api/agents` remains global because one logical agent can monitor characters on
several game servers. Character resources and commands are addressed by globally
unique character ID and current session; the UI exposes those controls only from a
character in the selected scope. Group records remain global and may intentionally
include characters from more than one server; each scoped response returns only its
server's members.

Cross-server PostgreSQL integration coverage checks case-insensitive character
filtering, mixed-server group memberships, hidden foreign-only groups, and detail
rejection. Full Go tests passed against an isolated PostgreSQL 18.6 fixture; `go vet
./...` passed. Frontend unit tests, Nuxt typecheck, ESLint, Prettier and production
build passed. ESLint reports 17 non-fatal HTML void-element warnings. The production
rollout to `10.25` completed with server, web and PostgreSQL healthy and both backend
readiness endpoints returning `status=ok,database=ok`. The available browser opened
the deployed Events page at operator sign-in, so the live Servar-vs-Greatest UI
comparison remains open pending an authenticated browser session.

The character inventory uses the bundled local SRO item assets and static
definitions across servers. The backend indexes full item presentation by the
item's stable `servername` code, including rarity/seal, type/classification,
requirements and reference-stat ranges. Shared definitions label and interpret
observed item state without substituting another character's inventory values.
Conflicting fields across configured catalogs are omitted; numeric model IDs are
never used for cross-server guesses. All 14,227 catalog icon paths exist locally.
The code now applies shared definitions and reference stats to an unmapped server's
API item observations. The extension is deployed to `10.25`: `/readyz` on port 8081
and `/api/health` on port 3005 both report `status=ok,database=ok`, and a local
game icon returned HTTP 200. PostgreSQL stayed healthy and was not restarted. The
available browser is unauthenticated, so visual confirmation on a live character's
Servar inventory remains open.


## Slice 6 — Chat implementation evidence (2026-09-28)

- The user's open v0.5.0 screen was inspected read-only. It showed the sender selector,
  six channel tabs (General, Private, Party, Guild, Union and Global), an offline
  sender choice and an empty chronological General history. Earlier inspection showed
  separate General/Private views and Global history. No message was sent and private
  message text was not copied. The checked-in
  [desktop chat capture](reference/phmonitor-chat.png) and
  [mobile chat capture](reference/phmonitor-chat-mobile.png) remain the design
  baselines for the conversation pane, private contact column and composer.
- PhMon adds the six named channel tabs, per-character sender, private contact/new-chat
  flow, history paging, unread/read cursors, jump-to-latest, composer states, emoji
  insertion, global confirmation, settings-backed browser/sound notifications, and a
  narrow-screen contacts/conversation switch. Unsupported outbound channels are
  read-only when the selected session explicitly reports that a mode is unsupported.
  If the live capability snapshot is absent, the composer can still submit; the Go
  command service validates current-session and channel support before queueing. The
  operator reports phBot plugin 1.4.2 installed and chat sending verified. The web
  page no longer shows a capability-waiting message or blocks on a missing display
  snapshot.
- Dashboard now shows the three latest canonical `chat.message_received` events with
  sender, channel and message preview, linking each row to the corresponding chat
  context. Empty state remains in the same stacked recent-chat card.
- The operator confirmed numeric callback mappings from phBot 20.1.2: `1` is
  General/All, `2` is Private, `4` is Party, `5` is Guild and `6` is Global. The
  source event payloads and chat projection for types 1, 4, 5 and 6 were corrected
  across 144 records by migration 10. A deployed read-only check later found one
  unclassified message while a private conversation showed only the outgoing row;
  type 2 now maps
  to Private and migration 12 backfills the existing event and projection.
  Migration 11 linked two unique same-session echoes to their outgoing commands and
  fixed the outbound echo-link constraint, preventing duplicate rows in chat history.
  Unknown numeric values remain in the Advanced-mode Unknown lane.
- General and Global history spans all characters on the selected server. General,
  Party and Unknown read cursors and unread counts are server scoped; Private, Guild
  and Union remain scoped to the selected character (and Private to the peer) so a
  read in one character's guild/union does not clear another character's unread
  messages. General/Global copies observed by multiple characters are combined
  within the existing two-second cross-observer window; repeated messages from one
  character remain distinct. Server-wide unread counts use the same grouping, while
  Global is omitted from the unread counter and its badge is hidden.
- A successful mark-read response now includes fresh contact and channel unread state
  computed after the cursor is saved. The chat page applies those counts immediately,
  then the shared live snapshot reconciles them. Changing private contacts keeps the
  contact roster visible while only the conversation pane loads.
- At the operator's direction, every channel uses flat chronological log rows with
  sender/time labels. General and other channels do not use private-message bubble
  alignment. The local chat page passes Nuxt typecheck, unit tests, formatting and
  production build; authenticated browser comparison remains open.
- Browser comparison at 1440×1000, 1280×800 and 390×844 is still open. PostgreSQL
  migration/store integration and simulator end-to-end verification are also open
  because this shell has no `TEST_DATABASE_URL`. Plugin adapter unit tests use fake
  methods and do not establish real phBot integration.

## Character portraits — 2026-09-28

Character snapshots now optionally carry phBot's documented integer `model` from
`get_character_data()`. Migration `000008` persists it as nullable `model_id`;
claiming a new session clears the old value until a fresh state arrives, while
ending a session retains the last observation for an offline character. Character,
group, detail, live and death-event responses expose the model and a profile-scoped
local `portrait_url`. Existing plug-ins remain compatible when they omit `model`.

The mapping evidence and exact Chinese/European model ranges are recorded in
[`character-portrait-investigation.md`](reference/character-portrait-investigation.md).
The exporter joins entity model IDs to existing local character DDJ files only when
the race/gender code agrees with the verified phMonitor v0.5.0 mapping. Rebuilding
the active Greatest profile found 52 mapped models; `server/game-data` resolves all
52 to distinct local PNGs, with no missing asset paths. Other profiles and unknown
models retain the initials fallback. Pet-body, monster and other entity roles remain
unmapped.

The reusable portrait component is present in Stats cards and the character table,
character detail, Dashboard death/event cards and the death-event list. Unknown
models and absent or failed local images display initials. A temporary deterministic
protocol fixture supplied model `1907`, an unknown model and one death event for
visual review; fixture data was never written to the database or shipped. At
1440×1000, 1280×800 and 390×844, Stats, detail and Deaths rendered the local portrait
and initials fallback. The image loaded at 128×128, requests stayed on the local
origin, and each page had zero horizontal document overflow. The 390 px check used a
same-origin fixed-size iframe because the headless browser clamps its outer window
to 500 px. Ignored screenshots are in
`exports/portrait-fixture-screenshots/`; the 1440 px Stats capture was compared with
[`phmonitor-stats.png`](reference/phmonitor-stats.png), and the Deaths capture with
`phmonitor_screenshots/05-deaths.png`. The screenshots use clearly synthetic
fixture names and demonstrate image presentation, not real phBot data.

Plug-in, exporter and frontend tests passed; Go unit tests load the bundled profile
and verify its 52 mappings. The database integration test for persistence/session
fencing is present but was skipped because this worktree has no `TEST_DATABASE_URL`;
no local PostgreSQL or phBot runtime is available. Those runtime checks remain open.

### Live rollout — portraits, 2026-09-28

Rebuilt and deployed the server and web services at `192.168.10.25` from the current
Slice 5 implementation with portraits. The release snapshot is
`/var/www/phmon/.deploy-character-portraits-20260928`; the existing PostgreSQL
container was left running and retained the same container ID. Its migration ledger
now includes `000008_character_portraits.sql` after Slice 5's `000007_event_pipeline.sql`.
The web `/api/health` and Go `/readyz` endpoints both return HTTP 200 with
`status=ok,database=ok`. The deployed web origin serves all 52 profile-mapped local
portrait URLs as `image/png`; no external asset requests were introduced.

The operator will upload the updated plugin separately. Until it sends a fresh
character state with phBot's `model` field, character portraits use the initials
fallback. Next verification is a live agent observation of a mapped `model_id` and
the resulting portrait in Stats/detail/event views. PostgreSQL restart persistence
and real phBot callback/runtime validation remain open.

### Drop display correction — 2026-09-29

The Events Rare Drops and Normal Drops rows previously showed only a numeric
model when the callback contained no item snapshot. Event responses now resolve
static name, icon and reference ranges through the server's versioned game-data
profile. The model is resolved only within its mapped server profile; a stable
item code can supply shared presentation across servers. This also applies to
historical occurrences at query time. Rare-drop event labels and item names
are gold. The Dashboard rare-drop headline uses the same resolved name.

The item detail expander labels catalog ranges as reference stats. phBot's
documented drop callbacks supply a model ID only, so exact rolled stats, blue
options and plus values in the supplied tooltip image remain unavailable for
these occurrences. No individual roll is inferred from a catalog range. A
verified per-drop item observation or correlatable packet is required to close
that gap. The live `192.168.10.25` Events API returned 401 from this workspace,
so authenticated live data and screenshot comparison were not available here.

The operator-provided `192.168.10.105` reference was inspected read-only in a
browser on 2026-09-29. Normal Drops rows showed an item icon and name first,
then time, character and location; clicking the name opened a compact dark
tooltip with individual defense/reinforcement/durability values and percentages.
Rare Drops used gold item names and a similarly opened tooltip with seal and
stats. Some tooltip trailing values lacked labels, so they are not copied into
PhMon. This confirms the remaining rolled-stat visual/collection gap rather than
turning catalog ranges into apparent instance values.

Focused Go resource/API/event tests, Nuxt typecheck and 13 frontend unit tests
passed locally. Browser and deployed-server verification remain open.

## Slice 7–8 map and Stats placement — 2026-09-29

The operator corrected the reference target to
`http://192.168.10.105/?server=greatest&view=map&guild=ibot&x_from=2026-09-21&x_to=2026-09-28&e_sub=custom&an_from=2026-09-21&an_to=2026-09-28&u_page=8&c_tab=union&c_char=greatest%7Cgreatest%3Anuker1%3A1907`.
After the
reference connected, Map showed the large raster viewport, character selector and
Jump To Character button on the left, Quick navigation on the right, coordinate/tile/
zoom readouts above the viewport, and layer controls alongside the map for characters,
Academy members, deaths, drops, mob density and mob types. The app normalized away the
`server=greatest` query value and displayed All servers in its selector; the saved
baseline capture remains the Greatest-scoped reference. Existing screenshot:
[`07-map.png`](../phmonitor_screenshots/07-map.png).

Stats screenshots confirm two separate map placements. The group summary card puts a
compact map on its left beside group event counts, HP/MP bars, gold and online counts
([`02-stats-01.png`](../phmonitor_screenshots/02-stats-01.png)). Each character's
Overview card has its own smaller map on the right beside status, region and position
([`02-stats-02.png`](../phmonitor_screenshots/02-stats-02.png)). In the local build,
`CharacterPanel.vue` supplies the group-scope preview and `CharacterCard.vue` places a
separate preview inside each Overview card. Both links carry server, area/floor,
region and selected character into `/map`.

Current GreatestSRO tiles can be displayed as an edge-continuity-supported raster
grid, with increasing X rightward and increasing Y upward. That evidence does not
validate a live character's within-tile world coordinates. The shared adapter is
covered by synthetic round-trip/refusal tests and still returns no position for this
active profile, whose transform, region mapping and command-Z evidence are open.
Therefore the local previews currently show a labelled raster reference and position
readout without asserting that the character marker is in the right pixel. Cave-floor
imagery and navigation actions stay unavailable pending their separate gates.

The worktree's Nuxt production build and development server both start. The local
`/map` browser check reached the operator sign-in screen, so I could not inspect the
authenticated Map or Stats UI or capture local comparisons at 1440×1000, 1280×800 and
390×844. The PostgreSQL integration suite was also skipped because this environment
does not define `TEST_DATABASE_URL`. These remain verification gates; simulator and
unit results do not substitute for them.

### Test deployment and protocol E2E — 2026-09-29

The operator authorized a test deployment to `node@192.168.10.25:/var/www/phmon` and
provided an operator access secret. The value is stored in the remote `.env` only and
is intentionally omitted from this document. A private rollback snapshot of the
previous app and `.env` is at `/var/www/.deploy-slice-7-8-luna-20260929`. The existing
5,118 local map tiles were already present on the host; the count and representative
hashes matched, so no game assets were transferred.

Rebuilt server and web, applied migration 14, and verified `/readyz` and `/api/health`
healthy. Authenticated read-only HTTP checks through the deployed Nuxt proxies returned
the Greatest map profile (4 areas, 5,118 tiles), density readback (currently no real
observations), `/map` and `/stats` HTML, and a local PNG map tile. The remote stack
still reports the same PostgreSQL container ID; the agent API reports three records,
two connected, and the character API reports eight records.

The protocol v7 simulator then ran in a second Compose project bound only to
localhost, with a separate empty PostgreSQL volume. It passed movement, monster
appearance, empty/unavailable/truncated snapshots, death/drop events, reconnect and
idempotent sample replay. The separate density readback returned two observer cells,
numerator 1, denominator 2, and averages 1 and 0. The temporary test project, data
volume, environment file and copied harness were removed; the production database
received no synthetic agent, character, event or mob data.

The full Go suite was also run in a temporary Go container against a separate empty
PostgreSQL project; every package passed, including the mob integration test for two
observers, sample-ID replay and stationary sampling limits. That project's database
volume and environment files were removed after the run.

The in-app browser reaches the deployed sign-in screen, but I did not automate the
sign-in form. Therefore matched visual captures at 1440×1000, 1280×800 and 390×844
remain outstanding. Deployment and simulator evidence do not validate the live
coordinate transform, cave imagery, real phBot behavior or map navigation commands.

### Stats group map and Map navigation correction — 2026-09-29

The operator reported that `/stats?group_id=497f328c-bd2f-45b0-9b50-8f4431ce7945`
showed no map and that Map remained disabled in Tools. The Stats template hid every
group summary whenever a group was selected; it now keeps the selected group's map
preview visible. The advanced Tools navigation now links Map to `/map` while keeping
unimplemented tools disabled. Nuxt typecheck, all 18 frontend unit tests, lint (zero
errors; 29 existing void-element warnings) and the production build passed.

Deployed only the web changes, preserving the Go server, PostgreSQL container, `.env`
and data volume. The new web container is healthy; `/map` and the reported group Stats
route return successfully, `/readyz` reports the database healthy, and migration 14
remains applied. Two live agents continue to report plugin 1.5.0 / protocol 7. The
production database has 17 mob samples and 37 observations. Authenticated visual
browser inspection remains open because the UI is at its operator sign-in screen and
sign-in was not automated.

## Slice 7–8 review findings — 2026-09-29

Addressed the six scoped review findings in `codex/slice-7-8-luna`:

- The Leaflet preset-to-tile conversion now uses the same north-up `max_y - tile_y`
  transform as tile rendering; a round-trip regression confirms preset (168, 97)
  opens and reads back as tile (168, 97).
- Last hour/day/week event windows roll forward every 30 seconds so incoming deaths
  and drops enter the map snapshot without changing the range selection.
- Map activity queries select deaths and drops separately, require mapped-position
  fields, and apply server region constraints before their per-query limits. The
  merged map list is then capped at 100. Event-ID deep links perform a server-scoped
  exact lookup outside the rolling window and use the selected profile's region to
  resolve the correct area/floor. If location transforms or cave imagery are absent,
  the map reports why it cannot display the event.
- Map profile responses are ignored when their request sequence or server scope is
  stale.
- Observation readback is explicitly named `observer_local_average_count`; it reports
  observer-cell sample counts and observed monster rows, says coverage is unverified,
  and does not return a `density` field. A cross-cell PostgreSQL integration assertion
  ensures a monster coordinate alone does not imply sampled coverage in that cell.

Evidence: `go test ./...`, 23 Nuxt utility tests, Nuxt typecheck, ESLint (0 errors;
29 void-element warnings), Nuxt production build, and 83 plugin tests passed. The
event and mob PostgreSQL integration tests were skipped because this worktree has no
`TEST_DATABASE_URL`; Docker is unavailable, so the new cross-cell DB assertion remains
unexecuted here. No authenticated browser or real phBot run was made for these fixes.
The Slice 7–8 acceptance gates remain open for outdoor coordinate/reverse-transform
validation, cave images/transforms, visual checks at the required viewports, and real
runtime evidence.
Exact next action: rerun the event and cross-cell mob PostgreSQL integration tests
with a disposable `TEST_DATABASE_URL`, then complete authenticated map verification
at the required viewports when that environment is available.

### Review-fix deployment — 2026-09-29

Deployed the six review fixes to the operator-authorized test host
`node@192.168.10.25:/var/www/phmon`. The Go server and Nuxt web images both built;
only those two containers were restarted. PostgreSQL was not restarted, its container
ID remained `96e300a6b9864d6d426fa21dc1a92f150e41e882169be9b038f3601308e8e20d`, and
migration 14 remains current. The pre-update server/web source snapshot is at
`/var/www/.deploy-slice-7-8-review-fixes-20260929/source-before.tar.gz`.

Post-deploy checks returned HTTP 200 for `/map`, the reported `/stats` route,
`/api/health`, and a local map tile; `/readyz` reports the database healthy. The
deployed source hashes match the worktree for the map live handler, observation
readback, MapCanvas, map page, events page and event-link resolver. The 10,172
deployed game assets were preserved (representative tile SHA-256 unchanged). No
database fixture data or phBot commands were sent. PostgreSQL integration tests and
authenticated visual/browser checks remain open; keep Slices 7 and 8 in progress.
Exact next action remains to run the event and cross-cell mob PostgreSQL integration
tests against a disposable database, then perform authenticated map verification at
the required viewports.

### Live character marker correction — 2026-09-29

The deployed API has four fresh online Greatest characters in region 25735, but the
map had no character markers: the renderer rejected every marker while the active
profile's transform was marked unvalidated, and the default viewport opened at
tile (168,97) rather than their region tile (135,100). The prior ledger entry had
treated the exporter metadata as conclusive and missed the separately documented
outdoor transform. See `docs/minimap-verification.md` for the official phBot
position examples, exported tile joins, and synchronized reference marker evidence.

The Greatest profile now supplies forward 192-coordinate-unit transforms for four
verified outdoor regions, including 25735. `/map` and Stats previews render fresh
characters at their tile-local X/Y and center on a fresh character when no explicit
selection is present. A region with an exact tile join but no pixel transform may
show a labelled approximate tile marker; unsupported regions still show none. The
map's marker count reports placed characters, and the Jump control distinguishes
exact position from tile-only placement. Commands still require verified reverse
conversion and Z, so this display fix does not enable character operation.

### Map marker and popup comparison — 2026-09-29

The operator's three screenshot crops and authorized static inspection of
`%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe` establish 28 px circular character
portraits with a blue outline and small name badge, monster bubbles with a 12 px
normal/party-general base size, 1.2× champion and 1.5× giant scale, a thin HP ring,
and 280 px dark monster/character popups. The reference uses a 36 px item icon card
with a small drop badge, and a 34 px death icon with a character portrait badge.

PhMon now enriches map characters and events with model portraits, renders local
item icons from the selected server catalog, sizes monster bubbles by numeric type,
and uses the documented monster name, HP/max HP and attack state when received.
Clicking a marker opens its detail popup; the character action opens the detail page.
The map keeps one popup open across live refreshes by updating markers in place.
The deployed browser showed loaded portraits for four Greatest characters, one
stable popup after an 11.5-second live refresh, and successful Open Stats navigation.

The currently connected 1.5.0 agents do not transmit monster name or HP. The
deployed popup therefore labels a live mob by model and shows unavailable HP; the
1.5.1 plugin source in this worktree adds those optional fields without changing
protocol v7. The reference drop badge artwork is absent from the approved local
asset export, so the current locally drawn badge is a visible parity gap. The
required cave floors, command Z/reverse transform, and real 1.5.1 phBot verification
remain open; Slices 7–8 are in progress.

Map zoom now spans 50%–2000%, starts at 125%, and advances in 25% increments. The
50% tile layer combines four adjacent exported tiles into one raster tile so the
minimum zoom displays the local map artwork.

### Nearby monster labels and cross-character deduplication — 2026-09-29

The Nearby monsters list now prefers a monster's display name, falls back to a
humanized server name, and shows its level. Monster type stays in the marker popup.
The installed phBot docs for
`get_monsters()` do not promise a level property; plugin 1.5.2 forwards it only when
the runtime supplies a bounded integer. The UI clearly says “level unavailable” when
that evidence is absent. Nearby list and map marker labels contain only name and
level; numeric model/type codes remain available in the detailed popup for diagnosis
and are never presented as monster levels.

Current snapshots previously keyed rows by each process-local monster ID, which
could duplicate one mob when two characters assigned it different IDs and could
merge unrelated mobs when IDs collided. The UI now collapses only cross-session
Sightings with the same server, region, model/server identity, rank and position
within 8 world units, keeping the freshest row. Same-session rows remain distinct.
Each row names the character whose observation is freshest. Focused frontend,
plugin and Go regression cases cover these rules. Connected 1.5.0 agents still lack
the newer descriptive fields until replaced; real-runtime level availability and
Slice 7–8 gates remain unverified.

The change was deployed to the authorized test host after the server and Nuxt
production images built successfully. `/map`, `/api/health` and server `/readyz`
returned HTTP 200; PostgreSQL reports healthy, and the staged plugin source hash
matches 1.5.2. The authenticated map rendered seven current rows with readable names
such as Edimmu and Dimension pillar. Every row showed “Lv. unavailable”, confirming
the connected observations did not provide a usable level. No bot command was sent.
