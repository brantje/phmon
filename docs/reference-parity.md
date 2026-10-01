# Reference parity ledger

This ledger records implementation evidence against the public phMonitor demo
baseline captured in docs/reference on 2026-09-26. Reference screenshots are
inspection evidence only and are never shipped as PhMon application assets.

## Multi-character remote controls — 2026-10-01

Before the Issue #35 screen changes, the operator supplied a reachable LAN demo
at `http://192.168.10.105/` with Map selected. A real-browser inspection at the
desktop reference viewport showed the Map canvas, Select character / Jump To
Character selectors, quick destination selection, and right-side Characters,
Academy, deaths, drops, mob-density and mob-type layers. The phBot → Client view
kept the tool navigation and showed one Go Clientless panel. The panel text
described terminating every `sro_client.exe`; that machine-wide behavior is not
copied. The public `phmonitor.com/demo` host was inaccessible from this workspace,
so hidden dialogs and any behavior beyond the reachable LAN demo were not inferred.

Issue #35 adds a shared remote-control panel to Map and phBot → Client while
leaving character-card actions single-character. Map continues to use its current
deduplicated action targets and spatial/dataset scope; its right panel has its own
scroll region so the character list, layers, and map remain reachable. Client has
local fleet/group target selection, offline rows with eligibility reasons, an
independent Inspect action, and server labels in All servers scope. The panel
exposes bot start/stop, trace start/stop, current-position or named training area,
radius, Return Scroll, Disconnect and a capability-blocked Clientless action.
Controls show read-only eligible/skipped counts even when the operator's optional
review preference is off. Return Scroll and disruptive controls always require an
explicit target-count confirmation; the preference controls review for routine
actions. Per-character durable results reuse the existing command history lifecycle.

Training behavior is described from phBot readback rather than inferred from the
demo: current-position arguments contain no copied coordinates, named area names
are profile-specific, and radius remains a separate command. The action catalog
does not add nearby-player discovery, coordinates, point navigation, party setup,
scripts, or other tool workflows to these screens. The browser-local review
preference is default-off for routine actions. Return Scroll, Disconnect and
Clientless retain explicit confirmation regardless of that preference, with the
eligible target count shown before submission. Their requests still carry
`confirmation: true` for the existing server intent contract.

Start Training skips targets whose latest observed `botting` state is true; Stop
Training skips targets whose state is false. Unknown values remain eligible under
normal capability/session checks, and the browser does not optimistically change
state while a command runs. The collector prefers a boolean character-data field
and narrowly maps optional `get_status()` values. Since the recorded live runtime
predates that fallback, `stopped` and `None` remain unknown until their meaning is
verified on a supported runtime.

Local implementation screenshot evidence and fixture-only browser results are
recorded in `/tmp/phmon-issue35-evidence/`. Easy and advanced modes were captured
for Map and Client at 1440×1000, 1280×800 and 390×844. The largest
observed document widths were 1425, 1265 and 375 pixels respectively, within
their viewport widths; the map's narrow-screen control panel remained reachable
by scrolling below the canvas.

The isolated browser fixture selected a saved two-character Client group,
inspected a third character without changing action targets, and used All to
select all 18 displayed rows, including 15 offline records. Eligibility marked
the three current sessions eligible and the offline records skipped. With
review off, Start Training produced three independently completed results. With
review on, the two-character Trace preview waited for explicit submission;
Cancel sent zero `/api/commands` POSTs. On Map, All selected three current
fixture characters under the registered `greatest` dataset and Stop Training
produced three independently completed results. These end-to-end commands used
only the production worker against local fake APIs. The expanded
`remote-controls` smoke also confirmed exact training-mode projection, distinct
execution-time positions, readback, intent flags, void Disconnect semantics,
unsupported Clientless, and rejection of a stale session after controlled
replacement.

Screenshot files: `client-easy-1440x1000.png`,
`client-advanced-1440x1000.png`, `client-easy-1280x800.png`,
`client-advanced-1280x800.png`, `client-easy-390x844.png`,
`client-advanced-390x844.png`, and the corresponding `map-easy-*` and
`map-advanced-*` files. `map-remote-controls-mobile.png` records the scrolled
mobile control panel. The fixture screenshots remain outside the repository and
predate the final mandatory-confirmation correction. Operator-authorized
live-browser verification later confirmed both Return Scroll and Disconnect count
prompts with review off; both were cancelled without submission. The live radius
result capture is `/tmp/phmon-issue35-live-radius-results.png`.

Final verification passed `bash scripts/check.sh` on Node 24.20.0 against a fresh
isolated disposable PostgreSQL test database: Go race tests, 118 plugin tests,
126 frontend unit tests, live transport audit, Prettier, typecheck, lint,
production build and Compose configuration. ESLint reported 0 errors and 51 style
warnings. `scripts/command_smoke.py` and `scripts/remote_controls_smoke.py` both
passed on the isolated fixture stack. This validates the production worker against
local fake APIs; Windows/phBot runtime behavior remains unverified. Following the
focus-return accessibility adjustment for review cancellation, the frontend unit
suite, formatting, lint, typecheck and production build passed again.

## Map training areas — 2026-09-30

[Issue #25](https://github.com/brantje/phmon/issues/25) adds a **Training areas**
layer to the Map screen. Each current session's observed training readback draws a
scale-correct circle (192 world units per 256-pixel outdoor tile) with a name label
at its top edge. Selecting a label or its side-list row opens a compact
editor with the observed center/radius, Move center, a Radius input, Apply and
Reset. The center and edge handles drag a local draft; the label gains an
`· unsaved` suffix and the observed circle stays visible until readback arrives.
While that suffix is showing, the chip places a discard X and an accept
checkmark after the character name and before `unsaved`. Those controls call
the same Reset and Apply actions as the side editor.
Apply sends only the dirty parts, center before radius, through the existing
audited commands and reports each step's durable result. Reset discards the draft.
Hiding the layer removes circles, handles and the editor.

The map context menu and selected-point bar now offer **Set training position for
N characters** next to navigation. It reuses the shared per-character fan-out,
freezes each target's explicit region/X/Y with its current Z (or 0), and changes only
the center, so each target keeps its own radius. Navigate and training items have
independent eligibility and exception text. The Review actions preference shows the
existing preview before any admission; Cancel leaves no command.

Keyboard: labels are focusable buttons and Enter/Space select them. Map-container
Enter now ignores keys whose target is a marker, so Enter on any marker no longer
also selects the map center. When one menu item is disabled, ArrowUp/ArrowDown
keep focus on the enabled item.

Selection follow-up (2026-10-01): a left-click or tap inside a circle selects that
area and still picks the map point for the selected-point bar. Overlaps resolve to
the smallest containing radius, then the closest center, then name order, and
circles paint largest first so the visible fill matches. A second click on the
selected area, or a left-click outside every circle, deselects it like the
side-list toggle. Right-click, keyboard map-center Enter and Move center keep the
current selection. Name chips select their own character even over another circle.
Chips for areas with nearly shared centers spread around the circumference
(2: NNW/NNE, 3: N/SE/SW, 4+: even steps), and colliding chips rotate to a free
slot; layout reruns on zoom. Fixture browser checks with four overlapping areas
covered nested, half-overlap, lens, outside, chip, keyboard, right-click and
zoom-out cases, with no chip overlap or horizontal overflow at 1440×1000,
1280×800 and 390×844.

Evidence (**simulator fixtures only**, isolated stack and database; production
plugin worker transport with fake phBot adapters): three fixture characters
rendered three circles; editor Apply on one fixture produced fake
`set_training_position` then `set_training_radius` calls, two completed commands
and an updated readback. Handle drags produced the expected draft center/radius
and Reset cleared it. Context-menu and mobile selected-point submissions to two
targets each produced one completed `training.area.set` per target with its prior
radius retained; the non-target character received nothing. With review enabled,
zero commands were admitted until submit. 1440×1000, 1280×800 and 390×844 had no
page-level horizontal overflow; the editor stays within the side panel on mobile.
No real phBot character received a training command, and the reference demo's
training-area interaction was not reinspected, so runtime and reference-visual
parity remain open.

## Live party map layer — 2026-09-30

Issue #23 adds a current-state **Party members** layer to the existing Map screen.
It uses the locally served Silkroad minimap asset
`/game-assets/interface/minimap/mm_sign_party.png`. The map payload is assembled
server-side from the canonical party resource observations rather than adding
per-character browser subscriptions. Spawned members require `player_id > 0`,
usable X/Y and a current observer region/Z scope. Outdoor and cave placement reuse
the existing coordinate transform; no region-tile fallback or guessed cave floor
is allowed.

Multiple current observers collapse to one logical party member, preferring the
freshest valid party observation. When the Characters and Party layers are both
enabled, a currently rendered managed-character marker suppresses the matching
party marker; disabling Characters allows that party marker to render. Popups
show only supplied name/guild/level/HP/MP values. Empty or unavailable party state,
disconnects and session replacement remove the old live contribution through the
existing live-map invalidation path.

This entry records implementation/test semantics, not a live phBot screenshot.

## Live NPC and teleporter map layer — 2026-10-01

Issue #24 adds a current-state **NPCs** layer to the Map screen. It uses
`/game-assets/interface/minimap/mm_sign_npc.png` for both NPCs and teleporters.
Names are always drawn under the marker. A row is a teleporter when its server
name matches `GATE_<name>`; every other row is an NPC. The popup shows name, role,
server name, model, region, coordinates, and the observing characters. Navigate
here opens the existing navigation confirmation at the marker and does not submit
a command by itself.

The payload is the union of connected characters' latest `get_npcs()` snapshots.
Different sessions collapse when server, region, server name, model, and position
within 8 world units match. Same-session rows stay separate. Cave placement uses
the observer's Z and omits a row when the floor cannot be proven. Disconnect,
session replacement, and a 35-second TTL remove a snapshot. There is no NPC
history, shop listing, or teleport execution.

This entry records implementation and fixture semantics. A live phBot `get_npcs()`
observation still requires plugin 1.7.0 on the operator's runtime.
A same-viewport runtime check with a spawned party member, duplicate observers and
a cave floor remains open.

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
- Agent management lives under Settings → Agents after the dashboard redesign. It
  lists all active credentials, including never-connected identities, with real
  connected state, stable agent ID, plugin/phBot/protocol versions, connection age
  and last-seen time. Operators can create a new one-time ID/token pair and revoke an
  offline credential; connected or stale-state removals are blocked. Revocation keeps
  historical agent references intact while preventing future authentication. The
  plaintext token remains visible only in the current provisioning panel and is not
  recoverable later. No fabricated monitoring data is used.

### Evidence and deliberate deferrals

Backend evidence is the authenticated /agent protocol, PostgreSQL agent metadata and
generation-fenced active registry. Plugin evidence is PhMon.py plus the shared
simulator contract. UI evidence is the Nuxt shell, Settings → Agents panel and same-origin agent action routes.

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
- At this Slice 2 snapshot, botting/training was unknown because official docs expose
  start/stop mutations but no read-only getter. Issue #35 later added the narrow
  botting readback described above; the operator-authorized 2026-10-01 live check
  observed boolean botting state on four active sessions. This does not verify the
  meaning of `stopped` or `None` from optional `get_status()`.

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
humanized server name, and shows its level and explicitly labeled type. The installed
phBot docs for
`get_monsters()` do not promise a level property; plugin 1.5.2 forwards it only when
the runtime supplies a bounded integer. The UI clearly says “level unavailable” when
that evidence is absent. Map marker labels contain name and level; numeric model/type
codes remain separate from both and are never presented as monster levels.

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
The nearby list retains its working monster Type field with an explicit label;
observed General and Party General types display by name, while unsupported code 27
is shown as `Unknown (27)` under Type. Map marker labels still show only name and
level.

The operator supplied an offline-character map crop showing muted grayscale portrait
pins at each character's last location. The map now includes offline characters when
their saved location and observation timestamp exist, renders their portraits in
grayscale with a gray outline, and labels their row/popup as an offline last-known
position. Their last-known location can be viewed or jumped to, while current-position
freshness remains required for live actions. The focused frontend suite passed all 31
tests and the production web image was deployed; `/map`, `/api/health` and server
`/readyz` returned HTTP 200 with PostgreSQL healthy. The inspected Greatest scope had
four online and zero offline characters, so a live offline-marker screenshot and
viewport comparison remain open until an offline character is present.

The operator reported that characters and nearby monsters disappeared and open map
popups closed during refresh. The relative event window advances every 30 seconds;
that changed the map subscription and its reset callback discarded the full snapshot.
Map feed refresh now retains existing data while server/area/floor/region remain the
same and marks it syncing until the replacement arrives. Switching spatial scope
still drops old-scope data; an observed empty monster snapshot still clears monsters.
The focused frontend suite passed (33 tests), changed-file ESLint passed and the Nuxt
production build passed. Deployment to the authorized host returned HTTP 200 for
`/map`, `/api/health` and server `/readyz`; the authenticated map still displayed
characters and nearby monsters after 35 seconds. Nuxt typecheck still reports errors
in unchanged `map.vue` and `mapMarkerPresentation.ts` code. Open-popup retention has
not yet been directly exercised in the browser.

The operator reported that the raster disappears below 100% zoom. Browser inspection
reproduced the blank map at 84% while marker layers remained; Leaflet's GridLayer was
pruning tiles below its default minimum zoom of 0. The map's configured zoom limits
now apply to both the map and raster GridLayer. After rebuilding and deploying web,
browser inspection at the 50% minimum showed the map imagery, markers and 16 raster
tile canvases with no failed tiles. `/map`, `/api/health` and server `/readyz` all
returned HTTP 200. The mapZoom regression test, all 34 frontend unit tests, ESLint on
changed files and local Nuxt production build passed.

### Faster current monster snapshots — 2026-09-29

The operator reported that a ten-second nearby-monster polling interval was too slow
for combat and confirmed that 0.1 seconds works on the active setup. Plugin
current-snapshot polling is now 0.1 seconds; durable historical sampling remains once
per minute per observer cell. Moving character and monster markers now interpolate
between snapshots over 120 ms. Follow-up inspection found that sub-0.5-pixel changes
were being snapped directly; the threshold is now 0.01 pixels and every existing
marker kind interpolates position changes. Added small-delta coverage. All 86 plugin
tests and all 37 frontend unit tests pass, and the Nuxt production build succeeds. The web service was
rebuilt and restarted on the authorized test host; `/map`, `/api/health` and
`/readyz` return successfully while PostgreSQL and Go remain running. The updated
plugin source is staged at `/var/www/phmon/plugin/PhMon.py`; this does not replace the
copy already loaded by phBot. Runtime load and visible animation need live movement
after the browser reloads and the plugin is reloaded.

### Teleport-following map correction — 2026-09-29

The reference showed nuker1 in Hotan while PhMon omitted it. PhMon's four-region
allowlist excluded Hotan region 23687, which encodes root tile `(135,92)`.
The Greatest map now decodes outdoor region IDs as its primary tile source and
places X/Y within that tile using 192 units per tile. A fresh Donwhang observation
in region 26520 resolves to `(152,103)`. A selected character's subscription spans
outdoor regions, and an old region filter clears after teleport. Stale positions
remain displayed with a stale treatment. The plugin preserves joined-game sampling
on a repeated `connected()` callback. Side-by-side browser comparison and reload of
the running plugin remain open, as do cave floors and full Slice 7–8 acceptance.

After deployment, a 2026-09-29 browser comparison showed nuker1 at Hotan
`(77.5,9.0)` on both PhMon and phMonitor, on the same central plaza feature.
PhMon rendered four character markers, including nuker1, with region 23687 and
tile `(135,92)` in its readout. This verifies the current Hotan placement at the
observed browser state; a subsequent teleport and plugin reload remain unobserved.

### Slice 9 historical heatmap surface — 2026-09-29

The existing Map screen now owns historical analytics rather than introducing a
second map application. Current characters, current nearby monsters and recent
death/drop markers retain their Slice 7 live/current semantics. A separate historical
section can independently enable observer-local mob averages, mob-type sightings,
deaths, drops, unique sightings and player movement, using the same raster coordinate
adapter and rendering below interactive live markers.

Historical controls include 1h, 24h, 7d and 30d relative windows plus a validated
custom range, region scope, historical character scope and observed mob-type facets.
Relative windows advance on the existing 30-second map clock. In-flight historical
requests are cancellable and sequence guarded; a same-scope refresh keeps the last
valid overlay visible while the replacement loads. Server/area/floor/region changes
clear stale historical results.

Heatmap API `source_rows` reports the total unsuppressed canonical source population
contributing to an aggregation before the bounded point limit is applied. It is not
the sum of only the returned cells.

True spatial mob density remains visibly unavailable because observation coverage has
not been verified. The observer-local layer is labelled as limited and explicitly
states that it is not spatial mob density. Cave/special-area queries fail closed while
their imagery/transforms remain unvalidated.

Heatmap reset records a server-owned suppression projection with the exact active
layer/server/area/floor/time and optional region/character/mob filters. It does not
delete canonical activity events, movement samples or mob observations. A reset
with neither a region nor a character filter is treated as broad and requires an
additional explicit confirmation enforced by the backend as well as the UI.

### Cave floors and 2D point Z handling — 2026-09-29

The operator-authorized phMonitor v0.5.0 executable inspection found these
floor definitions and 192-unit map anchors: Tomb B1–B6 use distinct regions
-32761..-32766; Donwhang 1F–4F use region -32767 or 32767 and Z bands -50..70,
71..210, 211..350, 351..490; Job Temple's single region -32752 identifies 1F,
while its 2F and Annex 1–5 share that region and cannot be inferred from it.
GreatestSRO `Media.pk2` has tiles for all 17 floors. The offline exporter now
publishes local PNGs in `game-assets/minimap_d/`; Leaflet switches each floor's
own grid and markers.

The reference map point is two dimensional. phMonitor converts it to X/Y and a
region, then reuses the selected character's current Z even when the chosen map
floor differs; it falls back to Z=0 when unavailable. PhMon follows this rule,
without terrain-height lookup or per-pixel Z calibration. Donwhang actions need
one of the observed region IDs; if the current character does not provide that
choice, actions stay disabled. Job Temple higher-floor actions use the explicit
shared region after a manual floor choice.

Live reference sample: after the operator teleported nuker1 to Donwhang Stone
Cave, phBot v20.1.2 showed X=-24272.5, Y=-93.5. phMonitor v0.5.0 put the
character near (-24273.0,-93.5), Z=0, on its 1F map; selecting 2F changed the
raster and hid the 1F marker. In the separate PhMon view, nuker1 still showed
its prior Hotan position observed at 2026-09-29 15:45:25Z. That entry was stale
and is not presented as Donwhang validation. No live bot command was used.

Implementation evidence: cave tile inventory/export, 17-floor profile tests,
coordinate and Z fallback tests, authenticated signed-region command flow, and
deterministic phBot adapter coverage. Local browser inspection showed distinct
Donwhang 1F/2F imagery, floor switching, nine loaded tiles, and the 3F layout at
the 1280 desktop setting. Desktop layout had no horizontal overflow. At the mobile
setting, controls stack and the floor bar remains reachable by vertical scrolling;
the browser tool constrained the requested 390px width to a 500px page viewport,
so exact 390px visual parity still needs a browser that honors that width. The
fixture had no live characters and did not send commands. Real-runtime command
execution remains open.

### Cave map test deployment — 2026-09-29

Deployed `codex/cave-maps-reference-z` to the authorized test host
`node@192.168.10.25:/var/www/phmon`. The source transfer contained the cave PNGs
and asset index; it excluded `.env`. A rollback archive of the pre-deploy source
files is at `/var/www/.deploy-cave-maps-reference-z-20260929/source-before.tar.gz`.
The Go server and Nuxt web images built successfully, and only those two services
were recreated. PostgreSQL kept container ID
`96e300a6b9864d6d426fa21dc1a92f150e41e882169be9b038f3601308e8e20d`; the separate
Silkroad containers were not changed.

Post-deploy checks: all Compose services report healthy; server `/readyz` and web
`/api/health` return `status=ok,database=ok`; `/map` returns HTTP 200; the public
asset index reports dataset `gamedata-17f8847c77edd7c7fadd`; all 1,891 cave PNGs
are present in the built web image. Donwhang 1F/2F, Job Temple 1F and Tomb B1
representative tile URLs return HTTP 200 `image/png`. The map-profile API requires
operator authentication, so its unauthenticated probe returned 401 as expected.
No database migration, simulator data or phBot command was applied. The plugin
source is on the test host but was not installed into a running phBot client.
Real-runtime navigation and authenticated browser verification remain open.

Follow-up after the first deployment: the server's packaged
`server/game-data/servers.json` still pointed Greatest at the previous dataset ID,
so the profile correctly marked the new tile set unavailable. Generated the new
compact server metadata from the matching exporter bundle and public asset index,
updated the Greatest dataset mapping, and added a regression that loads the
packaged metadata and checks it enables all 17 cave floors. `go test ./...` passed.
Rebuilt and recreated only the Go server. The running container now reports the
new dataset mapping; `/readyz`, web `/api/health`, `/map`, and the representative
Donwhang cave tile all pass. PostgreSQL retained the same container ID.

### Signed region and character jump diagnosis — 2026-09-29

The operator's `plugin/phMonitorAdapter.py` `normalize_position()` reads
`get_position()`, preserves `int(pos.get('region', 0) or 0)`, and returns it
alongside X/Y/Z. It does not reject negative cave region IDs. The live read-only
nuker1 snapshot on the deployed map instead had fresh X/Y/Z (`-24294, -91, 0`)
and no region; selecting nuker1 left **Jump to character** disabled. The PhMon
collector had filtered negative regions in both `get_character_data()` and
`get_position()` paths, and the Go wire-state validator also rejected negative
values. This dropped the region needed by the existing cave classifier and jump
handler.

Updated plugin collection and event location validation to preserve nonzero signed
regions in `-32768..65535`. Updated Go state/event validation and cave map region
filters for the same signed range; bumped the plugin to 1.5.3. No coordinate
inference fallback was added.
Plugin tests (91), Go tests (`go test ./...`), Nuxt unit tests (43), typecheck and
production build pass. Rebuilt/recreated only the Go server; database and web
service were left running, health/readiness pass, and agents reconnected. The
updated plugin source is on the test host but was **not** installed in phBot; the
live nuker1 snapshot still lacks the region, so the deployed Jump action remains
disabled until the updated plugin is installed/reloaded and reports a fresh
signed region. No bot command was sent.

### Cave monster snapshot visibility — 2026-09-29

After nuker1 loaded plugin 1.5.3, its live Donwhang position retained region
`-32767`, but the map reported no current monster snapshot. The collector still
required a positive observer region, and the server's live-snapshot validator
rejected signed regions. The cave live filter also discarded empty snapshots and
could not assign a monster without Z to the observer's floor. The current live
sample after the fix reports the monsters in region `-32767`; the cave profile also
retains the separately observed `32767` Donwhang region ID as a supported alias.

Plugin 1.5.4 now accepts the signed cave region, preserves the observed Donwhang
region alias, and sends the observer's current Z with live snapshots. Go validates
the signed region and alias, classifies cave monsters using monster Z or the
snapshot observer Z, and preserves a current empty snapshot scoped to the observer's
floor. Cave marker projection uses that same observer-Z fallback. A migration
widens signed mob-region storage constraints for future durable samples. Plugin
tests (93), `go test ./...`, Nuxt unit tests (43) and Nuxt typecheck pass. Live
verification after deployment shows nine current nuker1 monsters in Donwhang 1F,
with visible map markers and nine entries in the nearby-monsters panel. Their
reported region is `-32767`; the current observer position carries Z `-9`. The
active client plugin version was not surfaced in the map view. No character
command was sent during this check.

### Region names — 2026-09-29

The public phBot Game Data API documents get_zone_name(region) and its example
returns Jangan. PhMon carries an optional zone on canonical events and a separate
optional training_zone for training-area readback. Both values come from the
runtime for the specific observed region; numeric region IDs remain available for
map and command behavior.

Events and remote command controls display zone names. Historical events without
a captured name show Unknown zone when coordinates exist; there is no historic
backfill. Plugin fixtures, Go validation and persistence coverage, frontend
location tests and Nuxt typecheck cover the flow. Database integration and live
phBot runtime checks remain open.

### Issue #30 — multi-character command orchestration — 2026-09-30

Added a reusable frontend fan-out contract with typed catalog command names,
deduplicated ordered targets, current capability/session/scope checks, frozen
per-target arguments and request bodies, independent submissions started concurrently,
per-child idempotency keys, uncertain-outcome retry using the original request,
and session-matched exact-result merging. The reusable action/result components show
individual skipped, rejected, accepted and execution outcomes, separate submission
and execution counts, verification details, and stale-feed status. They are not yet
wired into broad map, training-area or phBot-tool flows; those remain in their owning
child issues.

The browser live protocol v1 adds bounded `controls.character_ids` and
`commands.idempotency_keys` projections. Batch controls return target-specific rows
from set-based character/session/control reads; exact result reads are scoped to the
same `operator` identity as command admission and bypass the normal history limit.
`useLiveData()` owns two optional slots per fan-out owner and chunks requests at 100;
it keeps batch cache freshness separate from base live-stream health and retains
observed results across chunk rotation and temporary outages. Agent/plugin protocol
versions and backend execution, rate-limit, expiry and one-in-flight rules are
unchanged.

The `Review actions before submitting` preference is stored in a browser-local
cookie and defaults to off. With it off, the explicit action click prepares and
submits without a prompt. With it on, multi-character actions show the selected
and eligible counts, planned command count, arguments and skip reasons; the action
refreshes its controls projection before submission and requires another review
click if the plan changed. A target whose session changes remains skipped under
its original frozen identity. The setting also governs the existing single-character
Return Scroll, Disconnect, Clientless, map navigation and training-area prompts.
Required schema intent fields remain `true` for return, disconnect and clientless
commands; preparation itself never posts a command. Chat's separate global-message
confirmation and record-deletion confirms are outside the character-command flow.

Pure utility tests cover selection deduplication, unsupported and unavailable
targets, argument-mode checks, all-ineligible zero submissions, session replacement,
more-than-four-request concurrency, partial rejection/uncertain outcomes, original-body retry,
HTTP-response loss after an authoritative result, and session-safe result merging.
`scripts/command_smoke.py` now starts three disposable simulator workers and exercises
the multi-character controls projection, independent session-fenced command rows,
and exact-key recovery with execution evidence. It verifies independent successful
and failed child outcomes. Browser acceptance used the isolated consumer at
1440×1000, 1280×800 and 390×844. Settings showed the preference off by default
and persisted its browser-local cookie when enabled. Keyboard cancellation of the
preview made no `/api/commands` request; a reviewed submit produced three accepted,
completed simulator rows with API verification evidence. The consumer and worker
data are fixture-only and are not evidence of real phBot runtime validation.

### Issue #29 — independent map action targets — 2026-09-30

The map Characters section now separates focus from action-target selection.
Character-row buttons still focus the character; adjacent checkboxes edit a
session-local, deduplicated target set. All/None and existing group selectors
operate on applicable characters in the active map server/area/floor/zone scope.
Groups display unchecked, mixed, or checked state from their applicable member
IDs. The character list remains available when character markers are hidden.
Spatial scope changes clear targets; stale refreshes retain them, and a confirmed
current snapshot prunes IDs that have left the same scope. Current map commands
continue using the focused character and their existing single-character session
fencing.

The shared `useLiveData().groups` stream is now server-neutral so a map route
override can resolve saved groups for its selected server. Sidebar and Stats
continue filtering group members through the global server scope.

Validation passed: 64 Nuxt unit tests, Nuxt typecheck, lint (0 errors; 41 style
warnings), Prettier check, production build, and `git diff --check`. The local
browser verified focus/target independence, All/None, and character-list
availability with map markers hidden. At 1440×1000, 1280×800, and 390×844 the
document had no horizontal overflow. No saved groups were present in the browser
fixture, so live group checkbox rendering was covered by the pure selection tests
for tri-state, overlap, and membership changes. No command was submitted. Screens
contain live operator character data and were kept outside the repository.

The follow-up layout keeps Characters in the right map panel, at its top, and
gives its list a taller scroll area. Historical heatmaps now use a keyboard
accessible disclosure button and start collapsed. Browser checks confirmed the
right-side placement, 340 px character list at 1440×1000, 272 px at 1280×800,
and 304 px at 390×844; the heatmap control started collapsed and toggled open
with Space. The document had no horizontal overflow at those viewports. The
production Nuxt container build passed after this layout change.

### Map and mobile drawer scroll reachability — 2026-09-30

Browser testing found that an expanded Historical heatmaps section created a
second full-height scroll region inside the right map panel, in addition to
document scrolling. Removed the panel height cap and its vertical scrollbar so
the page scroll reaches Layers, heatmaps, monsters, and events in one flow. The
Characters list stays independently bounded and is taller: 460 px at
1440×1000, 368 px at 1280×800, and 371 px at 390×844. On 1280×800, scrolling
over the map canvas brought the expanded heatmap range and reset controls into
the viewport; the map panel itself had no scroll range.

The 390×844 drawer also clipped lower navigation links behind a separately
scrolling link list. The open drawer is now the single scroll surface, with the
link list flowing naturally inside it. Browser navigation reached Settings
after scrolling the drawer. Map was tested at 1440×1000, 1280×800 and 390×844;
there was no horizontal overflow, the heatmap disclosure started collapsed,
targets started empty on remount, and the whole right panel flowed with the
document. Browser selection checks covered independent focus and targets,
All/None, group mixed/checked states, keyboard Space, and keeping the Characters
section available with character markers hidden. No character command was sent.

A user-supplied Characters-sidebar reference on 2026-09-30 further establishes
the compact per-character treatment: character name with level, right-aligned
presence state, red HP and blue MP bars, then server/location beneath them. The
map implementation now renders that presentation through the reusable
`MapCharacterStatusRow` component while preserving independent action targeting
and focused-character behavior. Browser screenshot verification for this styling
remains pending.

Focused character pins default to green borders (`#58bd8a`) and use blue
(`#4db9ff`) for the focused `selectedCharacterID`. Browser checks showed one
blue pin following the focus button, other pins remaining green, and changing
action targets leaving the focused pin unchanged.

Dashboard, Stats, Events, Chat, Alchemy and Guild Storage were also checked at
1440×1000 and 390×844 with no document horizontal overflow. Settings was opened
at mobile width. A repeated full-page browser-navigation sweep caused the Nuxt
dev websocket to stop; the dev server was restarted, health returned HTTP 200,
and subsequent in-app navigation checks remained stable. The LAN dev URL is
`http://192.168.10.25:3006`.

Party map pins display each party member's name below the icon, positioned close
to the marker and falling back to `Party member <player id>` when no name is
reported. The Recent deaths and Recent drops map layers both start disabled;
operators can enable either layer from the Layers controls when wanted.

### Map multi-selection pin feedback — 2026-09-30

Character pins now use the blue selected border for either the focused character
or membership in the map action-target set. Individual checkboxes, saved groups,
and All/None update existing pins immediately. Focus remains independent from
action targets; clearing targets preserves the focused pin's blue border.
This supersedes the earlier focus-only pin styling above.
Clicking the focused character's row button again clears focus. Clicking a
different row switches focus; checkbox and group targets remain independent.

Authenticated local dev browser verification covered four managed characters and
the existing nukers group: individual multi-selection/deselection, group
selection/clearing, All/None, and independent focus/target changes all passed.
Repeated row clicks cleared focus and restored green when no target was checked;
clearing focus also preserved independently checked targets.
Computed borders matched blue for selected pins and green for others; target
changes did not remount the map. No browser errors or character commands occurred.
All 64 frontend unit tests, Nuxt typecheck, focused ESLint (0 errors; 19 existing
style warnings), Prettier, production build, and `git diff --check` passed using
the project's supported Node 24 runtime.

### Issue #27 — multi-character navigation and remaining routes — 2026-09-30

The Map page now opens Vue-rendered navigation actions from right-click, selected
point, touch, and keyboard-center actions. The menu resolves each ordered,
deduplicated action target independently from its current session, fresh position,
capability and active map profile. It shows eligibility and skip reasons before
submission; existing optional action review remains in force. Prepared requests
freeze target, session, destination and idempotency key. A final per-child admission
guard refreshes map-feed freshness, session/capability match, profile version, and
coordinate conversion immediately before each POST. The existing single-character
training-area action remains explicitly focused-character only.

The plugin now negotiates protocol v8 and version 1.6.0 parses generated scripts
once, executes the exact bounded validated text, and publishes normalized route
snapshots without script source or teleporter IDs. The Go service validates the
route against the exact authenticated, completed durable command and stores route
state only in memory. Fresh accepted positions advance its monotonic reducer. Waits,
teleports, ambiguous scopes, unsupported transforms and unsafe crossings cannot
create a connector or cross-block polyline. Arrival remains a position observation,
not a command result. The browser renders each remaining route independently and
keeps route-list selection separate from character focus/action targets.

Issue #27 is implemented on top of the merged #29/#30 targeting and fan-out
foundations. The public issue confirms that map actions use ordinary per-character
`character.navigate` commands and that remaining route geometry is a transient,
session-scoped map overlay. See [Issue #27](https://github.com/brantje/phmon/issues/27).

The Vue context menu opens from map right-click and the selected-point action for
touch/keyboard use. Local browser inspection showed selected targets, per-character
offline skips and reasons, and a disabled Navigate action when no target was
eligible. Enter selected the map center. At 390×844 the selected-point action opened
the menu within the viewport. Active route overlays showed an independently labeled
character route and waiting/stale status at all three target viewports. A later
operator request removed the separate destination marker; the remaining path and
waypoint dots stay.
The browser document had no horizontal overflow at 1440×1000, 1280×800 or 390×844;
there were no page errors. No command was submitted from the browser UI because its
fixture targets were offline.

Screenshots were kept outside the repository: `/tmp/phmon-issue27-1440-menu.png`,
`/tmp/phmon-issue27-route-active.png`, `/tmp/phmon-issue27-route-1280.png`,
`/tmp/phmon-issue27-route-390.png` and `/tmp/phmon-issue27-390-menu.png`. These
screenshots use deterministic fixture characters and local exported map assets.

The authenticated protocol-v8 navigation smoke used the production plugin worker
with a fake phBot API adapter. It verified admission → exact validated script start
→ transient map route → a fresh post-invocation position marking arrival, and
confirmed that route frames do not create position-history rows. A second run held
the fixture beyond the 35-second presentation freshness window while continuing
live heartbeat handling, then verified stale-to-fresh recovery and observed arrival.
Fixture results do not establish Windows/phBot runtime behavior.

Validation passed under Node 24.20.0 with a disposable PostgreSQL database:
`bash scripts/check.sh` (race-enabled Go suites, 103 plugin tests, 90 frontend unit
tests, format/lint/typecheck/build and Compose configuration), plus the authenticated
navigation smoke. ESLint reported zero errors and 42 style warnings. Focused Go
navigation/commands/mapprofile/httpapi suites also passed. The public demo browser
navigation timed out in Chromium; the existing 2026-09-26 reference captures and
the issue description supplied the reference baseline. No production deployment
or real-character command was performed.

### Issue #27 operator navigation follow-up — 2026-09-30

The operator rejected the large right-click dialog and supplied a dashed connected
route example. The ordinary menu is now one compact navigation action naming its
frozen character or target count, with a short exception message when needed.
Coordinates, badges, target cards and redundant dismiss buttons are removed;
optional review and detailed independent results are below the map. The same
selected-point action remains available to touch/keyboard users.

The remaining-path renderer uses a 3 px cyan dashed polyline with waypoint dots.
Validated outdoor region seams remain connected through snapshot, progress and
browser conversion; genuine transitions and unavailable/filtered portions remain
separate. All eligible browser admissions start concurrently. The shared Go
scheduler dispatches on four independent workers, preserving session fencing,
per-character admission constraints and exact retries.

Automated and live evidence is recorded in
[the navigation runtime ledger](reference/issue27-navigation-runtime.md). Browser
checks cover the requested desktop/mobile sizes and stable map mounting. Live
Hotan navigation verifies connected outdoor seams, decreasing remaining paths,
simultaneous independent routes and fresh arrival after command completion.
Independent `path_not_found` failures are retained in the ledger; their cause is
unverified. The operator confirmed the plugin upload and later reported working
behavior. Fixture arrival remains explicitly separate from Windows/phBot evidence.

### Monster rank icons on the name label — 2026-10-01

Live monster markers stay the red HP bubble. General, Champion, and Giant rank
icons, plus the shared party badge for party ranks, render in front of the
opt-in name label. They do not replace or cover the bubble. Names remain off
until **Show nearby monsters names** is enabled, so the icons appear only with
that label.
