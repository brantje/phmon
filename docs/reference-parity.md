# Reference parity ledger

This ledger records implementation evidence against the public phMonitor demo
baseline captured in docs/reference on 2026-09-26. Reference screenshots are
inspection evidence only and are never shipped as PhMon application assets.

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
automated command smoke and a live LAN browser check pass. Real mutation and full
responsive screenshot gates remain open.

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
1440 × 1000, 1280 × 800, 390 × 844 and reference-native 2560 × 1315 remains open.
The saved evidence from Slice 2 is not presented as evidence for these new screens.
