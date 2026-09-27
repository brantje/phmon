# PhMon engineering guide

This is the canonical architecture and implementation plan. Read it before changes;
update it whenever decisions or slice status change. `prompt.md` is the historical
original brief; the execution contract below is the current scope and authority.

## Execution contract: read this and build

When the user asks an agent to implement this guide (for example, “read AGENTS.md
and let it rip”), that request authorizes implementation of the **entire remaining
roadmap**, not just one slice. Start from the earliest incomplete dependency and
continue through Slice 15 and the final acceptance gates. Do not stop after making
a plan, building a visual mockup, finishing one slice, or reaching the MVP milestone.

The 2026-09-26 instruction establishing demo feature/layout/style parity supersedes
the original prompt's “Slice 0 only” and “ask before Slice 1” limits for subsequent
implementation runs. `prompt.md` is historical context. This guide is the current
project contract. A request merely to edit this document does not itself start an
implementation run. Explicit limits in the current user request always take priority.

Working loop:

1. Inspect Git status, this file, the current implementation and the relevant tests.
   Verify the completion ledger against code; do not rebuild completed work.
2. Pick the earliest incomplete slice; define a small end-to-end increment and its
   behavior/visual acceptance checks. Use the feature matrix below to avoid omissions.
3. Verify the relevant phBot API/data capability from official documentation and,
   where available, the installed runtime before choosing a collection or command
   mechanism. Record the source, version, limitations and evidence in
   `docs/phbot-capabilities.md` when Slice 1 begins. Never invent function signatures.
4. Implement real plugin -> Go -> PostgreSQL -> UI behavior as needed. Add migrations
   when introducing durable data. Add UI progressively inside the reference shell.
5. Run focused tests, existing regression checks and relevant browser flows. Fix
   failures. Record commands and results, reference comparisons and remaining gaps.
6. Update the slice ledger and feature matrix evidence, then **continue immediately**
   to the next slice. Routine package choices, small refactors, schema design,
   test fixtures, local builds and local restarts do not require a permission checkpoint.
7. If a dependency is unavailable, document a precise blocker, finish independent
   work, and return to it when possible. Ask only for missing input/access that cannot
   be resolved from the repository, docs or environment, or for genuinely destructive
   or externally publishing actions not already authorized. Do not ask the user to
   choose ordinary implementation details.

Do not use lack of a real phBot process as a reason to stop all development. Build a
separate deterministic protocol simulator/test harness when connectivity exists, use
it for development and automated end-to-end tests, and clearly label fixture data.
It must exercise the same contracts as the plugin, never become a production data
source, and never be shipped as fake monitoring. Track actual Windows/phBot runtime
validation separately; simulator results are not proof of real integration.

A blocked capability does not disappear from scope. Record the missing source/API,
what was tried, the affected feature and the condition needed to finish it. Continue
unblocked work, but do not claim full parity with unresolved required capabilities.
Do not silently turn an unsupported requirement into a permanently decorative control.

Keep concise progress updates during work. At session/context boundaries, leave a
resume entry here containing the active slice, completed increment, files affected,
validation, blockers and exact next action. On resume, continue from that entry.
Do not merge, publish, send notifications to real third parties, or operate real bot
characters merely to demonstrate a feature unless that action is authorized. Test
commands and outbound integrations against local test recipients/adapters first.

## Target product: phMonitor demo parity

**End goal: PhMon has the same applicable feature set, information layout,
interaction structure and visual style as https://phmonitor.com/demo, implemented
independently on this project's self-hosted architecture.** A generic monitoring
app with roughly similar functionality is insufficient. Both functional parity and
visual fidelity are requirements. Heatmaps and the rest of the original roadmap
remain required even when the demo shows empty data.

### Reference baseline and inspection rules

The public demo was inspected on **2026-09-26**, displaying **v0.5.0**, at a
1440 × 1000 desktop viewport. Dashboard, Stats, Chat, Economy, Alchemy, Academy,
Map, Item Search, Skill Builder and Settings were navigable. Most data
was empty because its connection was unavailable. Some advanced navigation and
subtabs appeared in the page markup but were hidden in the active easy mode;
those labels establish intended areas, not proof that their workflows executed.

Repository reference screenshots (inspection evidence only, not application assets):
[dashboard](docs/reference/phmonitor-dashboard.png),
[stats](docs/reference/phmonitor-stats.png),
[chat](docs/reference/phmonitor-chat.png),
[map](docs/reference/phmonitor-map.png), and
[mobile chat](docs/reference/phmonitor-chat-mobile.png).
Desktop captures are 1440 × 1000; mobile is 390 × 844. The recurring connection-error
modal/backdrop were hidden only in the inspection browser. Captures retain the
reference's branding/ads for comparison; the explicit adaptations below govern
what PhMon actually implements.

- Open the demo in a real browser before implementing each major screen. Inspect
  easy/advanced modes, navigation, tabs, filters, drawers/dialogs and responsive
  behavior when accessible. Record any newly verified details in
  `docs/reference-parity.md` once implementation starts.
- **Ignore the demo's connection-lost popup.** Dismiss it; if it repeatedly reappears,
  hide only the connection-error overlay in the inspection browser. Do not spend
  time fixing its backend or treating the popup as a blocker or target UI feature.
  Do not bypass entitlements or inspect private endpoints/client protocols.
- Empty demo tables do not mean those features are optional. Derive table structure,
  controls and empty-state layout from the public screen; verify data semantics
  independently against phBot. Document inaccessible details instead of guessing
  that they were observed. Preserve this baseline if the demo later changes or is down.
- Capture reference and local screenshots at the same viewport, mode and screen.
  Keep a concise comparison ledger in `docs/reference-parity.md`, with links to
  local evidence artifacts where practical. Avoid relying solely on memory or a URL.
- Match the product's design with PhMon branding and independently authored
  components. Use locally served original, licensed, operator-supplied or
  operator-imported game-client artwork/map tiles/item icons where their local use is
  permitted. Record asset provenance; do not commit or redistribute extracted
  copyrighted client assets by default. An explicit operator instruction may
  authorize versioning the generated public asset tree; never include source
  archives or exporter-only provenance audits. Do not hotlink phMonitor assets, embed its
  application, copy its client bundle or connect to its services.

### Explicit adaptations to the demo

These preserve the original project boundaries; they are not implementation gaps:

- No client video, screen streaming or stream-quality settings. Live means state,
  positions, observations and map layers.
- No premium checkout, license/entitlement system, artificial paid feature limits
  or phMonitor infrastructure. Advanced features are available to the self-hosted
  operator. Do not reproduce a Premium purchase link or third-party advertisements.
- No separate local executable/broker and no backend filesystem access to the
  operator's Windows machine. Replace “copy plugin into local phBot folders” with
  plugin download/configuration and clear installation instructions. Report the
  plugin's connection/version status through the normal connection model.
- Keep the demo's server-information/dashboard-card layout where useful, but use
  operator-managed server information, optional own artwork and local metadata.
  Third-party/public server-directory functionality is outside this project's scope;
  it is not part of monitoring/control parity and must not be reproduced or scraped.
- Community links and promotional slots are optional operator configuration, with
  sensible empty/hidden states. Use PhMon's own identity and instance URL in
  the mobile QR/copy-link panel; never expose credentials in that URL.

All other observed functional areas are in scope subject to verified phBot source
capabilities. Do not interpret “where supported” as permission to skip investigation.

### Layout and visual contract

Use Nuxt UI primitives, themed to the reference; their default light/green styling
is not the target. The existing Slice 0 status page is temporary development UI.
Replace it with the application shell as part of Slice 1, retain health as an
operational diagnostic, and add real panels as their data slices become available.

- Desktop: a full-height left navigation rail approximately **228 px** wide, a slim
  top strip approximately **34 px**, and a main workspace filling the remaining
  viewport. Sidebar collapse lives at its upper boundary; the easy/advanced toggle
  sits at the upper right. Keep the sidebar navigable when content scrolls.
- Sidebar: brand/icon/version and connection indicator, server scope selector,
  primary navigation, collapsible tools/settings groups, then instance QR/link
  utilities. Preserve the demo's grouping and ordering as listed in the matrix.
  Show active navigation with a subdued blue highlight, thin border and small icon.
- Main header: small thematic icon, pale gold page title, muted explanatory line.
  Use modest workspace gutters and compact controls; preserve the dashboard's
  asymmetric panel composition instead of making every screen an equal-card grid.
- Palette observed in the demo settings: primary **#FEF6C3**, panel/background tint
  **#0D131D**, text **#EAF1FF**. Use near-black/navy translucent surfaces, blue-gray
  borders, subdued secondary text and restrained status colors. Start with these
  as semantic CSS tokens; validate actual contrast and screenshots.
- Typography: **Segoe UI, Tahoma, Arial, sans-serif**, roughly **14 px** base text,
  **25 px** page titles, compact **13–14 px** menu text. Observed menu rows are about
  **32 px** tall with **4 px** corners and thin borders. Headings are gold/cream;
  labels and dense tables remain readable. Avoid oversized landing-page headings,
  excessive whitespace, large pill controls or bright SaaS-style gradients.
- Backdrop: an atmospheric game-world landscape with a strong dark overlay, using
  approved local artwork. Panels must remain legible independently of the image.
  A temporary neutral backdrop is acceptable during development, not an excuse to
  declare visual completion before the intended treatment is in place.
- Dashboard: fleet summary counters and total gold; deaths panel; server-information
  artwork/card; a wider event panel; stacked recent-drop/chat cards; offers panel.
  Preserve relative prominence and links into detail views. Empty panels retain
  their structure. Populate cards from the corresponding real backend data.
- Lists use compact filter bars, count badges, date ranges, dense tables, pagination
  and item/character imagery where available. Chat uses channel tabs, a contact
  column, conversation pane and bottom composer. Map uses a large canvas with
  character/destination selectors and adjacent/stacked layer controls.
- Easy mode simplifies visible tools/filters; advanced mode exposes full controls.
  This is a persisted presentation preference, never an authorization bypass.
- Responsive: reproduce the reference's collapsible/off-canvas navigation and
  reflow panels on narrow screens. Verify at 1440 × 1000, 1280 × 800 and 390 × 844.
  Tables may scroll within their region; the whole page must not overflow. Preserve
  keyboard access, focus visibility, labels and touch usability.
- Show loading, empty, stale/disconnected, error and recovered states intentionally.
  Our connection failures use a calm persistent status and retry behavior; do not
  reproduce the demo's recurring modal. Keep last known data clearly marked stale.

### Required feature-to-slice matrix

This matrix supplements the original roadmap. Original “initial” or “potential”
items do not narrow the final parity target. Implement each area in its owning
slice; Slice 15 closes remaining presentation/tool gaps and verifies the whole app.
For each row, record backend/plugin/UI evidence and any capability blocker in
`docs/reference-parity.md`. A navigation label or empty placeholder is not completion.

| Area / navigation | Required behavior and layout | Owning slices |
| --- | --- | --- |
| Shell and instance access | Reference sidebar/header, server scope, connection/version state, responsive navigation, easy/advanced mode, instance URL copy and mobile QR panel. Persist preferences; scope data consistently. | 1, 2, 15 |
| Dashboard | Fleet online/offline/alive/dead counts, gold total, recent deaths/events/rare drops/chat/trade offers, server-information card and working drill-down links. | 2, 5, 6, 13, 15 |
| Stats and character details | Search characters/guild/server/zone, create/edit groups, live stats and progress, current status, and a dedicated character detail surface. Detail views include inventory/equipment, supported pet classes (Attack/Fellow/Pick/Transport) with applicable state/inventory, party membership/setup and verified actions. Preserve character identity and group membership across restarts. | 2–4, 12 |
| Events | Unified timeline plus level-up/custom/death/rare-drop/normal-drop/unique filters; character/item/date filtering, counts, pagination and map links. Rare-drop presentation preserves observed rarity/seal/color/detail metadata; normal-drop detail preserves observed blues/attributes where the source exposes them. Persist occurrences with reliable ordering without inventing missing item properties. | 5, 7, 13 |
| Chat | General/private/party/guild/union/global tabs; sender character selector, private contacts/new conversation, recipient field, history and jump-to-latest, message composer and results. Add emoji/item references where supported; confirm costly/global sends. | 6, 13, 15 |
| Economy | Global buy/sell/trade offers and stall views; text/character/item-type/subcategory/degree filters, reset controls, stall transactions/chat and source attribution. Derive history only from observable data. | 6, 13 |
| Alchemy | Current attempt log, historical item sessions, highest plus and success/failure/attempt counts; character/item/type/degree filters; statistics over recorded attempts. Do not fabricate probabilities. | 5, 12 |
| Academy | Owned/joined academy tabs, membership/state, join/leave/graduation activity, unread log and mark-read action; map member layer and historical metrics where supported. | 4, 5, 7, 12 |
| Guild Storage | Guild-scoped item listing/detail, search integration, freshness/observer attribution and explicit confirmed removal of stored records. | 4, 13, 15 |
| phBot tools | Client/bot controls explicitly cover start/stop bot or training, set training area, set training radius, walk, disconnect, return scroll and go clientless where the verified phBot API supports each action. Party Setup must reproduce the verified reference control surface and round-trip current configuration/state. Scripts must be discoverable/listable, manageable where supported and executable for explicit character targets; Quest exposes verified information and supported actions. Investigate each tool's real controls and argument semantics before implementation. Route every mutation through authenticated, capability-aware, audited commands; never arbitrary remote Python/shell execution. | 3, 4, 15 |
| Analytics | Character/session rates, deaths, rare/normal items, economy and academy analyses; time/server/character filters, charts and documented calculations backed by durable data. | 12, 13 |
| Map | Pan/zoom, region/quick destination selection, character picker/jump-to-character, coordinates/tile/zoom display; characters and academy members, recent deaths/drops with time ranges, live nearby-monster markers, mob-density/types and other historical layers. Use the server's versioned exported dataset for region/map reference data and local assets where available. Validate dedicated map/coordinate handling for Jangan Cave / Tomb of Qin-Shi, Donwhang Cave / Donwhang Stone Cave and Job Temple / Temple instead of assuming PK2 presence proves the outdoor transform applies. Safe confirmation and explicit server/region/layer scope for heatmap reset. | 2.5, 7–9 |
| Item Search | Search inventory/equipment/character sets, storage and guild storage; text/server/type/subcategory/degree filters, reset, item details and owner/source navigation. Resolve static taxonomy/names/icons through the server's game-data profile while preserving live/historical instance facts from their observed source. | 2.5, 4, 13 |
| Skill Builder | Chinese/European builds, game-version/cap selection (demo exposes 110/120/140), mastery/skill prerequisites and level adjustment, bulk increment/decrement shortcuts, reset, SP totals and comparison with a live character. Prefer versioned skill/reference data from the server's exported game-data profile where present; verify rules per supported version and distinguish planning from execution. | 2.5, 15 |
| Automations | Conditions and schedules tabs, add/edit/enable/disable/delete, target selection, backend evaluation/execution, expiry/missed-run handling and auditable results. Condition/action content supports the verified phMonitor-style placeholders/variables through a bounded server-side template context with deterministic missing-variable behavior; templates never execute arbitrary code. No paid rule-count limits. | 10, 11 |
| Settings | Language selection with working translations for offered locales; easy/advanced mode; primary/background/text colors; icon sizes (45/60/75 px) and text sizes (11/14/18 px); persisted chat/notification preferences; plugin install/config guidance. | 1, 6, 15 |
| Notifications | Per-event sound/browser notification preferences for messages, deaths, rare drops, alchemy thresholds, uniques, academy changes, offline state, sales and level-ups; local WAV library upload/preview/assignment. Browser permissions are explicit. Discord webhook CRUD/test/delivery with redacted secrets, bounded retries and observable results. | 5, 6, 10, 15 |
| Record management | Character and guild-record deletion with typed-name confirmation, scope/retention explanation and server-side authorization. No accidental bulk removal. | 14, 15 |
| Operations | Usable setup, auth/agent token management, compatibility reporting, backups/restore/migrations, retention and deployment/upgrade instructions. | 1, 14 |


Advanced phBot/analytics/automation screens and hidden subtabs still require focused
reference inspection when accessible. Their labels were visible in public markup;
only the visible easy-mode flows were exercised during the initial inspection.

### Final definition of done

All of the following must hold before reporting the end goal complete:

- Every applicable feature-matrix row works end to end with the self-hosted backend;
  every original slice and Slice 15 meets its acceptance criteria. Explicit demo
  adaptations above are documented; no required area is silently omitted.
- A new operator can start the stack, install/configure the plugin, connect multiple
  agents, observe characters, use correctly targeted commands, inspect durable
  history, chat, maps/heatmaps, rules, schedules and analytics using documented steps.
- Real phBot integration is validated on a recorded supported version. If no runtime
  is available, report that gate as blocked, describe simulator coverage precisely,
  and leave the real-integration gate open.
- UI matches the reference's hierarchy, proportions, navigation, density, palette,
  typography and key interactions at the specified desktop/mobile viewports, using
  appropriate local assets. Screenshots are reviewed side by side. A default Nuxt
  theme, a health page or a loosely inspired redesign does not meet this gate.
- Data survives appropriate restarts. Connection loss/reconnect restores current
  state without wrong-agent commands or duplicate historical events. Expired
  commands stay expired; schedules and rules have documented deterministic behavior.
- Authentication, authorization, input limits, bounded queues, retention and secret
  handling protect the supported deployment. Minimum protection is implemented
  when a feature needs it; Slice 14 hardens it, not postpones all security until last.
- Automated checks, browser workflows and production/container builds pass. No
  unexplained runtime errors, inaccessible critical controls or external phMonitor
  service/asset requests remain. Integration limitations are explicit.
- Documentation and the completion ledger match the code. Setup, local development,
  upgrades, backup/restore and operational limitations are reproducible.

## Product and architecture

Build a completely self-hosted alternative to phMonitor, using phMonitor only as a
feature, layout and visual-style reference. Never depend on its backend, protocol, client, premium entitlement
system or infrastructure; never reverse engineer or bypass paid-access controls.

```text
phBot -> custom Python plugin -> outbound HTTPS / WebSocket -> Go backend -> PostgreSQL
                                                              ^
                                                              |
                                                       Nuxt + Nuxt UI
```

Each active phBot instance connects directly over TLS, eventually at `/agent`; there
is no local broker or Go/Windows agent. Live means character state and a game map,
not game-client video or live-stream capture (explicitly excluded).

**The plugin reports facts and executes commands; the backend owns business logic.**
The plugin collects, normalizes, publishes, receives commands, executes them through
controlled dispatch, and reports results. It is authoritative only about its own
current process/game session. The backend owns policy, authorization, command
lifecycle, history, persistence, aggregation, analytics, conditions and scheduling.
It is the durable authority for agents, characters, commands, events, historical
state, map observations, heatmap aggregates, rules and schedules. Do not place
analytics, heatmaps, historical calculations, rules, schedules, authorization policy
or persistent business state in the plugin for convenience.

## Boundaries and protocol principles

- Never block latency-sensitive phBot callbacks on network I/O. Callbacks collect
  and enqueue in memory, then return; a worker handles networking. Incoming commands
  use controlled dispatch, never arbitrary execution in the network thread.
- Tolerate complete backend outages: connected -> disconnected -> retry with backoff
  -> connected -> hello -> current/full state where required -> normal events.
  Transient high-frequency observations may eventually drop while disconnected.
  No durable local SQLite/event store without a demonstrated need.
- Commands eventually have stable IDs (`cmd_...`), command names (`bot.start`), expiry
  (`expires_at`), and result messages (`command.result`, matching ID and status).
  Distinguish queued, sent, acknowledged, completed, failed and expired. A successful
  socket write is never successful execution. Unsafe delayed actions expire.
  Implement this lifecycle in Slice 3, not in the foundation.
- Snapshots describe current state: HP/MP, XP/SP, gold, position, botting, inventory,
  party, pets and training. Events describe occurrences: death, drop, unique spawn,
  teleport, level-up, chat, alchemy and disconnect/reconnect. Use deltas/events where
  sufficient; avoid persisting enormous repeated full snapshots.
- Heatmaps are first-class, server-computed analytics of plugin observations. Track
  samples as well as observed mobs; density is conceptually observations / samples.
  Standing still must not create arbitrarily hot cells merely as time passes.
  Future layers: live characters, nearby monsters, drops, deaths, mob density/types,
  unique sightings, movement, XP gain, gold and items gathered.
- `docs/protocol.md` records the unimplemented protocol boundary. Do not invent an
  agent wire protocol or install a WebSocket dependency before Slice 1.

## Repository, dependencies and conventions

Initial repository inspection found only `prompt.md` and an empty Git repository.
The current Slice 0 structure is below; evolve it with implemented slices:

```text
AGENTS.md                 canonical guide and full roadmap
README.md                 setup, configuration and validation
plugin/README.md          reserved integration boundary (no Python plugin yet)
server/cmd/server/        Go executable
server/internal/config/   environment validation
server/internal/httpapi/  health/readiness handlers and boundary tests
web/app/                  Nuxt 4 application with Nuxt UI and TypeScript
web/server/api/           thin same-origin health transport to Go
docs/protocol.md          protocol status and future compatibility principles
docs/reference/           public-demo screenshots for visual comparison only
scripts/                  local validation
.github/workflows/        CI
docker-compose.yml        local PostgreSQL, Go and Nuxt services
.env.example              documented local-only configuration
```

- Backend: Go standard library HTTP and pgx PostgreSQL pool. Frontend: Nuxt 4,
  Nuxt UI 4, TypeScript, npm lockfile. Runtime: Node 24.20.0 LTS, Go 1.27.1,
  PostgreSQL 18.6. Nuxt 4.5.2, Nuxt UI 4.11.2, pgx 5.11.0. Pin dependencies and commit lockfiles. No ORM is needed in Slice 0.
- H3 1.15.11 is an explicit frontend dependency, matching Nuxt/Nitro 2 and
  preventing dev-tool H3 2 prereleases from determining generated server types.
- Go owns readiness; Nuxt's server route only forwards health and sanitizes transport
  failures. Backend URLs stay server-private; browsers use same-origin `/api/health`.
- Use DRY and YAGNI: smallest maintainable design for the current slice. No Redis,
  Kafka, RabbitMQ, NATS, Kubernetes, microservices, CQRS or event sourcing without a
  demonstrated requirement. Avoid abstractions until there is a concrete use.
- No schema, migrations, product tables or mock monitoring data in Slice 0. Introduce
  migrations with the first durable schema, then harden upgrade procedures later.
- Use environment configuration; never hardcode production credentials or log tokens
  or database URLs. Bind local service ports to loopback, validate external input,
  and use TLS for real deployment. Authenticate agents in Slice 1 and prevent one
  agent impersonating another. No elaborate IAM in Slice 0.
- Keep Go formatted with gofmt; use go vet and behavior-oriented Go tests. Use Nuxt
  ESLint, Prettier and TypeScript checks for the frontend. Prefer explicit types at
  boundaries; handle loading, unavailable and recovery states in UI.
- For each slice: inspect architecture; define minimal domain/API changes; implement
  backend, plugin and frontend as needed; add focused tests; update docs; run
  relevant validation and report exactly what passed and what remains.
- Tests cover behavior and boundaries: HTTP/API, meaningful domain logic,
  repository/database integration where valuable, meaningful frontend behavior and
  protocol tests when it exists. No tests duplicating implementation details, no
  large framework just for coverage. Validate the real browser/API/database path.
- Before edits inspect status, conventions and relevant files. Preserve unrelated
  work. Scope changes to the active slice, never merge branches/PRs without explicit
  instruction. Make grouped commits only if the environment expects commits.

## Scope and implementation status

**Slice 0: complete. Slice 1: implementation and automated validation complete;
operator has manually verified basic real phBot → PhMon connectivity. Slice 2: in
progress. Slices 3–15: not started.** Slice 1's detailed runtime/profile/API and
same-viewport visual gates remain open; basic connectivity must not be described as
blocked or as proof of all runtime APIs. See `docs/phbot-capabilities.md`.

**Current turn (2026-09-27):** the operator explicitly requested that the generated
Nuxt assets under `web/public/game-assets/` be tracked in Git. Removed that directory
from `.gitignore`; the prepared public tree contains 10,172 files (10,171 asset URLs
plus `asset-index.json`), 487,358,758 bytes total. Source PK2 archives and the
exporter-only audit remain excluded. This supersedes earlier notes that the public
tree was ignored; those entries record the state when their checks ran. Exact
minimap marker placement and other coverage gaps remain unresolved.

**PR review follow-up (2026-09-27):** verified and fixed all seven actionable
CodeRabbit findings: unique entity-shard audit names even for empty shards; raw
output-path symlink checks before resolution; bounded PK2 directory traversal; X-axis
map-sheet orientation; palette-PNG transparency; portable sparse-file fixtures; and
web-relative resolution for npm `--source`, `--output` and `--asset-output` paths.
Validation passed: 26 exporter fixture tests, Python compileall, Node syntax check,
ESLint for the wrapper, Prettier for the changed text files, and `git diff --check`.
The symlink-parent test simulates `Path.is_symlink()` because this Windows account
cannot create filesystem symlinks (WinError 1314); a physical symlink test remains
unverified here. No new real archive export was run for these code-only fixes.

**Current turn (2026-09-27): Slice 2.5 exporter follow-up adds the Nuxt public asset
destination and npm command; asset readiness remains incomplete.** Exporter `0.4.0`
adds configurable `--asset-output`, stable PNG URL aliases and a source-independent
`asset-index.json`; `web/package.json` adds `npm run export:assets`, defaulting to
ignored `web/public/game-assets/`. The GreatestSRO source path remains a runtime CLI
argument or `GREATESTSRO_SOURCE` environment variable, not hardcoded in PhMon. The
public index maps all 30,276 verified semantic keys across 10,171 PNG aliases; raw
asset-entry provenance stays in the exporter-only audit. A real `0.4.0` export took
245.409 s. Copy-only validation passed for 17 catalogs, 8,921 content-addressed
bundle assets and 10,171 public files without source/audit inputs. Nuxt served
`/game-assets/icon/skill/china/bow_area_a.png` as `image/png` with its indexed SHA-256.
The public tree is 487,358,758 bytes including `asset-index.json`; all extracted
assets remain Git-ignored. Compileall and 18 fixture tests passed; targeted ESLint and
Prettier checks passed. The copied 0.4.0 bundle preview returned HTTP 200. No
database, backend, Docker or phBot process was needed. See the Slice 2.5 plan and
coverage ledger addenda. Do not claim all-assets-ready.

The npm export was rerun independently in 246.084 s with byte-identical bundle reuse.
The Nuxt tree validated all 10,171 aliases and 30,276 keys. A fresh copy containing
only `bundle/` (no audit or PK2 files) validated all catalogs/assets and served the
standalone preview page, manifest, map catalog, a PNG and a tile sheet with HTTP 200;
all 18 exporter tests passed. No dedicated mob-icon family was found, so entity-role
gaps stay unresolved.

**Active minimap verification follow-up (2026-09-27):** `docs/minimap-verification.md`
records region/tile checks against GreatestSRO exports and the phBot reference. It
confirms `Map\97\168` aligns with `gridX=168, gridZ=97` / `168x97`, identifies
Donwhang as `Town_Dunhwang` in the source, and resolves the current live characters'
region 25735 to root tile 135x100. Exporter 0.4.2/schema 1.2.1 links `mapAssetKeys`
only for exact root minimap/grid pairs: the real catalog has 2,449 of 2,471 region
records linked; 22 have no exact root tile. A full edge pass over 5,118 root tiles
supports X increasing right (4,806 pairs, mean RGB edge error 17.010 vs 36.426;
4,132 preferred) and Y/Z increasing upward (4,721 pairs, 16.576 vs 36.364; 4,100
preferred). The separate 185-tile `arabia` set independently supports the same
directions but remains geographically unjoined. Exporter 0.4.2 records per-set
measurements and its standalone overview uses supported directions. Exact in-tile
world coordinates, marker anchor and special-area registration remain unvalidated;
the PhMon app has no Map screen, so no production marker transform was added.

The 0.4.2 real export took 242.804 s and an identical rerun took 254.988 s and
reused the byte-identical bundle. The copied bundle validates 17 catalogs, 8,921
assets, 30,276 keys and zero dangling references; the Nuxt public tree validates
10,171 files and 30,276 mappings. Bundle-only copied preview requests for the page,
manifest, maps catalog, live-region PNG and both grid sheets returned HTTP 200; the
root and `arabia` sheets were visually checked. Twenty fixture tests passed and
compileall passed. A fresh read-only GET at 03:27:56 UTC returned four online
characters in Region 25735 (X=98.1–101.5, Y=1551.1–1560.2; names omitted), which maps
to `(135,100)` only. The screenshot is not identity/time linked to those rows. The
confirmed Slice 2.5 exporter scope has no terrain renderer, so exact in-tile world
coordinates and marker anchor carry forward unvalidated. Before a later map screen
places markers, validate them from a trusted reference or identity/time-linked map
observation. Slice 2.5 stops here; do not start Slice 3.

**Current goal continuation (2026-09-27):** a full comparison with the installed
phBot reference directory found 5,117 shared root tile names, 677 reference-only
names, one GreatestSRO-only name, and 30 shared tiles whose raw GreatestSRO DDJ
payloads are opaque black despite visible terrain in the phBot comparison. Other
non-backup GreatestSRO archives contain no alternate raster entries; no phBot image
was substituted. The selected-character label in the map screenshot matches one
current API row, and all four online rows map by region 25735 to root tile
`(135,100)`. This verifies tile selection, not exact placement: the local crop
candidate favors increasing Y down while broader grid evidence favors increasing
grid Y up; the screenshot and API row are not time-linked. Full evidence and the
unresolved source/transform requests are in `docs/minimap-verification.md` and the
coverage ledger. No production map screen or coordinate transform was added. Exact
next action: use any operator-supplied authorized archive or synchronized map
capture/trusted transform to resolve the respective gaps; otherwise retain them as
unresolved. Do not substitute phBot source assets or start Slice 3.

**Current Slice 2.5 follow-up (2026-09-27):** exporter `0.4.3` / schema `1.2.2`
adds explicit opaque-black minimap raster status and a hatched preview treatment.
The `web` npm command was run with `--asset-output public/game-assets` and again
without an override; the first real run wrote dataset
`gamedata-47c969ded0613d4c2a22` in 258.879 s (bundle 465,803,751 bytes; separate
audit 24,243,119 bytes). A second run without `--asset-output` took 268.352 s,
reused the byte-identical bundle and published to default `web/public/game-assets`.
Current validation reports 17 catalogs, 8,921 bundle PNGs,
30,276 semantic keys, 10,171 public files, zero dangling references and 207
uniformly black map tiles (193 root and 14 secondary-set). `icon/skill/china/bow_area_a.png`
exists; 3,183 `icon/item` aliases exist; no `icon/mob/` alias or verified entity
portrait/pet/unique role was found. Sounds and interface controls remain excluded
per operator instruction; 159 non-control symbol candidates remain in scope. A
bundle-only copy validated without audit/source files, and its standalone browser
preview rendered a 2,724 × 1,104 minimap overview with black source tiles hatched.
All extracted files are Git-ignored. Asset coverage remains incomplete: 22 regions
have no exact root tile; 677 phBot comparison-only tiles and 30 shared visible
terrain mismatches are not substituted; in-tile coordinate/marker transforms remain
unvalidated. Keep Slice 2.5 in progress for unresolved coverage, answer pending
map-source/time-linked-capture questions when available, and do not start Slice 3.

**Minimap goal continuation (2026-09-27 05:38 UTC):** re-inventoried the current
GreatestSRO Map/Media archives and the phBot reference files. Verified again that
`Map/97/168.o2` exists while `Map/168/97.o2` does not; `Media/minimap/168x97.ddj`
and reference `168x97.jpg` are the corresponding X=168/Y=97 raster. All six
Jangan and four Donwhang tiles exist on both sides at 256 × 256 (group RGB MAE
5.8430 and 5.0927); live tile 133x95 has MAE 5.1911. A 05:36:29 UTC API snapshot
of four rows mapped exactly through region grid indices: three to tile 135x100,
one to 133x95, both nonblack. A later selected-character row mapped to 135x100.
The API rows are current, but the saved Map screenshot is timestamped the prior
day and no production Map component exists, so exact marker-pixel placement cannot
be verified. Official phBot `get_position()` docs report region and X/Y/Z only;
the Map guide cautions its map is not fully accurate, and neither provides the
needed GreatestSRO pixel transform. Next action: obtain the previously requested
synchronized character/map capture or trusted transform, then validate against a
real marker render. Do not guess offsets, implement a production map UI in Slice
2.5, or start Slice 3.

**Minimap goal continuation (2026-09-27 05:52 UTC):** a fresh GET returned four
online character rows with state timestamps within one second of the response. All
four are in Region 25735 and resolve through the exported catalog to exact,
nonblack root tile `(135,100)`. The source-independent `standalone-copy-0.4.3/bundle`
preview served its page, manifest, maps catalog, resolved 135x100 PNG and both
tile-set sheets with HTTP 200 and correct content types. Exact world-to-pixel
mapping and the screenshot marker anchor remain unresolved; the app has no Map
surface. Next action: obtain the previously requested synchronized capture or a
trusted transform before validating any marker placement. Do not guess offsets,
modify production map UI in Slice 2.5, or start Slice 3.

At 05:58 UTC the current checkout's Nuxt dev server on port 3006 served the public
135x100 map PNG, China bow skill PNG alias and asset index with expected MIME types;
all three response hashes matched `web/public/game-assets`. The temporary server
was stopped. Requests to `192.168.10.25:3005` returned HTML for asset paths, but
there was no local listener on that port, so it is treated as a separate remote
host rather than evidence about this checkout.

At 06:05 UTC, re-hashed source archives and rechecked the selected Map inventory:
`Map.pk2` is 1,092,251,648 bytes, SHA-256
`a819141950fed83d2a293eed6ebb96e22ec6f8eceb407eb677fbc06656562183`, matching the
selected-source audit. `Map - copia.pk2` has the same size but a different hash and
remains excluded. `map/97/168.o2` and `map/102/152.o2` exist; reversed `map/168/97.o2`
and `map/103/152.o2` do not. Recompared 12 exported GreatestSRO raster PNGs with
phBot JPGs: all are 256 × 256; Jangan mean RGB MAE 5.8430, Donwhang 5.0927, live
tiles 135x100 2.9955 and 133x95 5.1911. A 06:05:19 UTC API read had four rows
updated within a second, all region 25735 -> exact nonblack tile (135,100), with
X=85.800–98.157 and Y=1557.047–1562.200. The 07-map screenshot predates that read
by over nine hours and remains insufficient to prove exact live marker pixels.
Next action: acquire a synchronized position/map capture or trusted transform;
keep exact placement unresolved and do not start Slice 3.

An independent seam audit of the installed phBot reference's 5,794 flat root JPGs
also supports the grid orientation: 5,407 horizontal pairs prefer X increasing
right (mean edge RGB MAE 17.193 vs 35.320, 4,656 lower-error pairs) and 5,298
vertical pairs prefer Y increasing up (16.555 vs 35.401, 4,647 lower-error pairs).
This corroborates the GreatestSRO source-grid result, but does not prove the local
world-to-pixel transform or current marker pixels.

On goal resumption, a 07:32:48 UTC API GET returned four recently updated rows in
two regions: three in Region 25735 -> `(135,100)`, and one in Region 23941 ->
`(133,93)`. Both exact-grid tiles are nonblack and present in the phBot reference;
the 133x93 PNG/JPG RGB MAE is 5.5813. This advances live tile-selection coverage,
but the snapshot is over ten hours after the saved map screenshot and still cannot
verify in-tile marker pixels. Keep the goal active during this fresh blocked audit;
next needed evidence is a synchronized map/position capture or trusted transform.

**Previous current turn (2026-09-27): Slice 2.5 standalone exporter implemented
within the operator-confirmed scope; asset readiness remains incomplete.** Added
`tools/game-data-exporter/` with inspect/export/validate/preview commands, bounded
read-only PK2/DDJ conversion, normalized deterministic JSON/PNG bundle output,
exporter-only audit and authored fixtures. Selected GreatestSRO `Media.pk2` for
catalogs/art/minimap rasters; `Map.pk2` is inventoried and selected while
`Map - copia.pk2` is excluded as backup. Sounds and interface controls are excluded;
non-control symbols remain in scope. No PhMon/backend/plugin/database/API/UI changes.

Real dataset `gamedata-66e9e3ee636c5a2a3f8f` (exporter 0.3.1, schema 1.1.0): 17
catalogs, 8,921 PNG assets, 30,276 semantic keys and a 465,088,731-byte bundle. The
repeat export reused byte-identical output. The copied bundle validates without
source/audit paths: 0 dangling asset references and normalized relations validated.
The manifest remains `incomplete`; item taxonomy/descriptions/names/icons, entity and
pet/portrait/unique roles, skill/mastery rules, teleport metadata and map transforms
still have explicit unresolved coverage. See the Slice 2.5 plan and asset ledger.

Final environment checks: Python 3.12.10 virtualenv install and CLI version passed;
compileall passed; 17 fixture tests passed; copied-bundle validate passed. The real
export took 178.797 s, identical repeat 189.36 s; five working-set samples peaked at
165,826,560 bytes (sampled, not guaranteed peak). The standalone preview rendered
from the copied bundle, reviewed all in-scope families, observed no external requests,
and had no horizontal overflow at CSS viewports 1440 x 1000, 1280 x 800 and 390 x
844. Screenshot/preview outputs are ignored under `exports/`; extracted Nuxt assets
are ignored under `web/public/game-assets/`. Source was
kept read-only; no PhMon services, PostgreSQL, Docker or phBot were used. Exact next
action: stop after Slice 2.5. Do not start Slice 3 in this task; carry unresolved
coverage forward without inventing semantics or substituting sources.

**Previous turn (2026-09-26): Slice 2 PR #3 correctness follow-up.** Scope remained
Slice 2 only. This follow-up adds periodic dead-generation session reconciliation
after database recovery, nonfatal per-character rejection for stale writes, plugin
suppression of a rejected observation, full-current-observation semantics for state
updates, and an atomic logical-agent disconnect fence. The multi-socket Registry is
intentional: one agent ID/token may have several independent active socket generations;
one socket may own several explicit character sessions; closing a generation or
rejecting one of its stale character operations must leave sibling generations and
characters alone. Main's newer Slice 2.5 game-data catalog requirements remain in the
roadmap below. Do not start Slice 3.

For each subsequent slice keep a completion entry with: status (`not started`,
`in progress`, `blocked`, `complete`), implemented behavior/files, tests actually run,
visual comparison evidence, real-runtime versus simulator validation, limitations,
and the exact next action. Never overwrite the historical Slice 0 evidence.

### Slice 0 completion record

- Created Go API, Nuxt + Nuxt UI application, PostgreSQL Compose service, isolated
  application images, environment examples, validation scripts and GitHub Actions.
- Go owns bounded database readiness (`/readyz`); liveness (`/healthz`) stays up
  during a database outage. Nuxt forwards health through `/api/health` with a bounded
  timeout and sanitized failures. Browser refresh handles failure and recovery.
- Node 24.20.0, Go 1.27.1, PostgreSQL 18.6, Nuxt 4.5.2, Nuxt UI 4.11.2; pgx pool,
  no ORM. Dependency lockfiles included. Explicit H3 1 matches Nitro
  2; icons bundle locally with external fallback disabled; fonts use the system.
- Validation passed: `bash scripts/check.sh` with `TEST_DATABASE_URL` (gofmt,
  go vet, race tests including real PostgreSQL, Go build, Prettier, ESLint, Nuxt
  typecheck/build, Compose config), Docker image builds and healthy startup,
  `scripts/smoke.py` for healthy/outage/recovery states, and browser checks of the
  built stack and Nuxt dev server. The browser retry button recovered from a real
  database outage without reloading. No initial browser errors or external resource
  requests. `npm audit` reported zero vulnerabilities.
- CI includes validation and complete Docker startup plus database outage/recovery
  smoke tests. Hosted GitHub Actions has not run in this environment.
- Deviations: `plugin/README.md` reserves the plugin boundary instead of an empty
  Python implementation; source development uses host processes with PostgreSQL in
  Compose, while the full Compose stack runs built images. No infrastructure beyond
  the slice is needed. Ports 3005/8081/5435 avoid existing local services.
- Limitations: local HTTP only, no user/agent auth, no domain database schema or
  migrations, no monitoring/control behavior, no automatic frontend polling. The
  single status page uses a manual check button. These are deliberate scope limits.
- Deferred: all product features, starting with **Slice 1 — Agent registration and
  connectivity**. No executable plugin, WebSocket or `/agent` route exists.
- See README for reproducible commands and configuration. Never describe planned
  roadmap behavior as implemented.

### Slice 1 progress record

- Added protocol v1 over bearer-authenticated outbound WebSocket: hello/ack,
  application heartbeat, bounded message size, explicit compatibility rejection and
  automatic reconnect semantics. Per-agent tokens are stored only as SHA-256 hashes
  and are bound to stable UUID agent IDs.
- Added embedded transactional migrations, durable agent metadata, a generation-fenced
  in-memory active registry and a safe read-only agent presentation API. Each
  authenticated socket has its own generation; agent presence stays online while any
  socket remains active, and stale cleanup cannot mark a concurrent socket offline.
- Added one-time credential provisioning through both `phmonctl agent create` and
  the web dashboard. Both call the same server-side generator/store path. The web POST
  returns only the newly generated plaintext token once with no-store semantics;
  existing tokens/hashes are never listable or retrievable. Because user auth is not
  implemented yet, web provisioning is explicitly a trusted-network capability.
- Plugin configuration now uses phBot's native QtBind GUI instead of requiring
  operators to create PhMon.json. URL, agent ID and token are persisted in PhMon-owned
  settings keyed by both phBot's active get_config_path() and get_profile(), so
  multiple accounts/characters and multiple named profiles for one character remain
  isolated. PhMon never modifies phBot's player JSON. The GUI clears the token field
  after load/save and only reuses the stored token when URL and agent ID still match.
- Added plugin/PhMon.py using only Python standard-library networking. Public phBot
  documentation verifies socket support, while actual embedded-runtime availability
  of ssl, select, threading, hashlib, base64, struct and urllib.parse still requires
  the real phBot validation gate. A dedicated worker owns network I/O; phBot callbacks
  never block on the backend. Added scripts/agent_simulator.py using the exact same
  transport/worker contract.
- Replaced the temporary Slice 0 page with the first reference-style PhMon shell:
  compact sidebar/header, persisted easy/advanced and collapse preferences, live
  agent list, one-time web credential creation, loading/stale/error/recovery states,
  responsive agent cards and credential-free instance copy/QR access. Health remains
  an operational diagnostic.
- Extended CI to provision a real test credential through the Nuxt web endpoint and
  exercise
  connect -> Go restart -> automatic reconnect -> disconnect through
  plugin transport -> Go -> PostgreSQL -> Nuxt, in addition to database
  outage/recovery checks.
- Validation actually observed in this completion pass: the focused stdlib plugin
  protocol/config/backoff suite now contains 16 tests, including per-profile settings
  isolation, saved-config round-trip and hidden-token reuse rules. GitHub Actions run
  36256686626 passed both jobs on profile-GUI head 3c28b41: validate ran
  bash scripts/check.sh with TEST_DATABASE_URL against PostgreSQL, and stack exercised
  credential creation -> production PhMon.py transport/simulator connect -> Nuxt API
  visibility -> backend restart -> automatic reconnect -> disconnect, plus database
  outage/recovery. Existing PR review threads remain resolved.
- Real-runtime limitation: no compatible Windows/phBot process is available in this
  environment. Record the actual phBot version, embedded Python version and imported
  stdlib-module behavior in docs/phbot-capabilities.md when that gate is exercised.
  Simulator results are not a substitute.
- Visual limitation: same-viewport screenshots at 1440x1000, 1280x800 and 390x844
  could not be captured from a runnable local Nuxt stack in this environment and
  remain BLOCKED. The sidebar also has no canonical PhMon app version/build metadata
  to display; docs/reference-parity.md records that gap rather than treating the
  existing "slice 1" development label as a version.
- Original Slice 1 next action at the time of that validation: exercise PhMon.py in a
  real supported phBot runtime and compare screenshots at all three viewports. The
  operator has since confirmed basic real connectivity; API/profile-specific runtime
  validation and screenshot comparison remain open. Slice 2 implementation is now
  authorized independently and does not claim those gates are closed.

### Slice 2 progress record (2026-09-26)

- Status: in progress pending PR review-fix validation. Real phBot data API validation
  remains partial and botting state remains unknown. Scope is Slice 2; no Slice 3
  command path was added.
- Added migrations for server-scoped characters/current state, generation-fenced
  character sessions, persisted groups/membership, and startup stale-session
  reconciliation. Stable identity uses normalized `(server_name, character_name)`;
  the server-scoped name uniqueness assumption and lack of a verified global game ID
  are documented.
- Protocol v2 adds authenticated `character.identify` → `character.registered`,
  explicit-ID `character.snapshot`, `character.state`, and `character.left`. Every
  post-resolution message is checked against character UUID, agent ID and active
  connection generation. Reconnect requires full state after resolving identity.
- Plugin sampling uses documented `get_character_data()`, `get_position()` and
  `get_zone_name(region)` on the callback thread, coalesces changed state in a
  bounded queue, and leaves backend networking to the existing worker. The Botting
  API has no documented read-only state getter; botting remains unknown, not guessed.
- Follow-up fix after observing a connected agent with no character registration:
  plugin v1.1.0 can recover when loaded after `joined_game()` by detecting the
  populated server/name returned from `get_character_data()`. It still refuses to
  sample after an observed disconnect. The live phBot instance initially reported
  v1.0.0; after reload, it reports v1.1.0 and its character state is visible.
- Runtime churn follow-up: multiple live characters appeared under one authenticated
  agent while a single-socket-per-agent registry repeatedly canceled the prior
  socket. The registry now tracks concurrent sockets independently, closes only the
  character sessions owned by a closing generation, and marks the agent offline only
  after its last socket closes. `/api/agents` reports `active_connections`.
- Review correctness model: `character.identify` explicitly claims that character's
  current session for the socket generation. `character.snapshot` and
  `character.state` require that current session and cannot open or reclaim it. A
  later explicit identify can hand off one character without invalidating other
  characters owned by the same or another socket. Identify makes the claimed character
  online and clears old current stats until its full snapshot arrives. Snapshot and
  plugin state messages both replace current observed fields, so unavailable values
  become unknown rather than appearing freshly observed from stale retained data.
  Ownership rejection returns a nonfatal `character.rejected` response, scoped to the
  explicit character ID; the plugin suppresses repeated writes for that identity
  until another identity is observed or the socket reconnects. A healthy-server
  reconciler compares persisted live sessions with exact registry generations every
  three seconds and ends dead-generation sessions after database recovery without
  affecting live sibling sockets. Logical-agent disconnect writes use the final
  cohort's atomically captured connection timestamp, so a newer socket cannot be
  marked disconnected by delayed cleanup.
  An observed empty guild clears guild metadata; omitted guild means unavailable.
- Added searchable character/detail and persisted group APIs, same-origin Nuxt
  proxies, backend-backed debounced search, a compact character overview with group membership controls, and a
  `/characters/{character_id}` detail surface. Later inventory/pet/party/map/action
  panels are explicitly marked unavailable for their owning slices.
- Added PostgreSQL integration coverage for server-scoped identity reuse, state,
  switch/offline handling, generation fencing, search and group membership; protocol
  API tests and simulator fixture checks pass. The same-origin overview/search/group
  and character detail were built and smoke checked against the full Compose stack.
- Validation: baseline and final `bash scripts/check.sh` passed with
  `TEST_DATABASE_URL` enabled (Go race tests/PostgreSQL, 24 plugin tests, Prettier,
  ESLint with three self-closing-input warnings, Nuxt typecheck/build and
  Compose config). `docker compose up --build -d --wait --wait-timeout 180` and
  `python3 scripts/smoke.py` passed. Simulator E2E passed through plugin transport,
  Go, PostgreSQL and Nuxt. Browser screenshots were captured at 1440 × 1000,
  1280 × 800 and 390 × 844; the 390 px viewport has no document-level overflow.
  The follow-up collector regression test and complete PostgreSQL-enabled check
  passed again. Two simulator-created fixture characters and their sessions/agent
  were removed from the local development database.
- Real runtime evidence: phBot 20.1.1 agents running plugin 1.1.0 registered four
  online characters. `/api/agents` showed three active sockets for one agent ID and
  one socket for another; `/api/characters` showed three distinct live character
  IDs/sessions under the first agent and another under the second, with advancing
  current-state timestamps. Server, name, zone, level, HP/MP, XP/SP, gold, region,
  and position were visible. This manually verifies multiple concurrent character
  sessions behind one agent and isolation across agents. It does not establish
  multiple phBot profiles in one process. Basic real phBot → PhMon connectivity was
  operator-confirmed earlier. Simulator coverage remains separately labelled and
  was not used for this runtime claim.
- The real evidence includes multiple concurrent sockets sharing one agent ID/token
  and several simultaneous character sessions. Simulator tests separately cover both
  physical socket close orders and backend restart; these are not claimed as manually
  tested runtime scenarios.
- Exact remaining Slice 2 work before claiming full completion: record embedded
  Python version, manually validate character switch/teleport/reconnect snapshot and
  repeated disconnect behavior, and determine whether botting state has a supported
  read-only source. Official botting docs show start/stop operations but no state
  getter; botting remains unknown. Do not start Slice 3.

### PR #3 correctness review follow-up (2026-09-26)

- Status: requested review fixes are implemented and validated. Slice 2 still has the
  real-runtime checks listed above. Commits pushed to the existing PR branch:
  `91e2d46` (authority/snapshot, leave ordering, multi-socket
  metadata and guild semantics) and `2c2bba6` (concurrent identity discovery test).
- Backend/UI/smoke changes in this follow-up include backend-backed
  debounced search with request cancellation; connected-agent and active-socket counts
  shown separately; same-logical-agent credential wording; upstream status/body
  preservation for character/group proxies; 4 KiB pre-buffer group request bound; and
  outage smoke expectations with a guaranteed CI recovery step.
- Validation: PostgreSQL-backed `go test -race` for
  `internal/agents`, `internal/characters`, and `internal/httpapi`; all 25 plugin unit
  tests. The full `bash scripts/check.sh` passed under Node 24.20.0 with PostgreSQL
  enabled; ESLint reports only the existing three self-closing-input warnings.
- Isolated Compose stack on ports 5536/8181/3505 passed production build, normal
  `scripts/smoke.py`, agent/character simulator scenarios, backend restart followed
  by automatic full-snapshot recovery, database outage (`/healthz` 200, readiness,
  health and data APIs 503), and recovery to normal 200/404/201/204 behavior. The
  separate local stack with four real phBot sockets remained untouched.
- Real-runtime evidence remains the already recorded phBot 20.1.1/plugin 1.1.0 run,
  including multiple concurrent sockets on one agent ID/token and several live
  characters. The review-specific authority race and outage cases are simulator and
  automated-test evidence, not new phBot validation.
- This UI/proxy/smoke increment was committed and pushed as `725dd3d`; subsequent
  recovery fixes and main integration are recorded in the following entry.

### PR #3 recovery and multiplexing correctness follow-up (2026-09-26)

- Status: implementation and automated validation passed. Commit `4d95b5e` adds
  dead-generation session reconciliation, explicit nonfatal ownership rejection,
  fresh current-state semantics for updates/new claims, a disconnect metadata race
  fence, and simulator/CI coverage.
- `RunSessionReconciler` pings PostgreSQL and repeatedly runs an idempotent bounded
  reconciliation while available. It closes only sessions whose exact
  `(agent_id, connection_generation)` is absent from the in-memory Registry; the
  Registry remains authoritative for live connectivity. Backend startup reconciliation
  continues to end all pre-restart sessions.
- Protocol v2 responds to valid stale snapshot/state/left messages with
  `character.rejected` and keeps the socket open. PostgreSQL-backed WebSocket tests
  prove one socket can retain B while a second socket takes A, and stale A operations
  cannot alter A or close either socket. Plugin unit coverage proves repeated writes
  for a rejected identity are suppressed until reconnect or a new identity.
- State and snapshot both replace current observations. Identify remains the session
  publication point for Slice 2; a newly claimed session clears prior stats, so the
  UI may show online identity with unknown state until the authoritative snapshot.
- CI's outage scenario uses two production-plugin simulator workers with one token,
  stops PostgreSQL, closes one socket during the outage, restores PostgreSQL without
  restarting Go, and checks that only the dead character is taken offline while the
  sibling character and logical agent stay connected. This is simulator/Compose
  evidence, not real phBot runtime evidence.
- Focused validation passed on isolated PostgreSQL at port 5536:
  `go test -race ./internal/agents ./internal/characters ./internal/httpapi` and
  `python3 -m unittest plugin.test_phmon` (26 tests). Run the canonical full check and
  `bash scripts/check.sh` with PostgreSQL enabled under Node 24.20.0. ESLint reports
  only the three existing input self-closing warnings. Production Compose build and
  normal `scripts/smoke.py` passed on ports 5536/8181/3505. The character lifecycle
  simulator passed before and after Go server restart, followed by disconnect
  verification. `scripts/outage_session_smoke.py` stopped PostgreSQL while two
  production-plugin simulator sockets shared one credential, closed A during the
  outage, restored the database without restarting Go, and verified A offline, B
  online, and the logical agent connected with one active socket. Outage health/data
  APIs and post-recovery normal smoke checks passed. GitHub Actions runs `36271335065`
  and `36271338139` both passed `validate` and `stack`, including the new outage
  scenario.
- `origin/main` was integrated in merge commit `ca30d62`. Its newer Slice 2.5
  game-data catalog and PK2 requirements were preserved in the feature matrix and
  roadmap; Slice 2 progress and review records were retained and updated.
- Real phBot evidence remains only the already recorded phBot 20.1.1/plugin 1.1.0
  multiple-socket/multiple-character observation. The stale-operation and database
  outage/recovery cases remain simulator plus automated backend evidence. Remaining
  manual Slice 2 checks are recording embedded Python and observing character
  switch/teleport/reconnect/repeated-disconnect behavior; botting remains unknown
  because official docs expose mutations but no read-only getter. Do not merge PR #3
  or start Slice 3.

## Canonical slice roadmap

The following roadmap preserves the original Slice 0–14 work and adds Slice 15
for complete demo parity. The feature matrix supplies binding additions to each
slice. Later slices are plans, not existing functionality. Complete slices in
order and continue; milestones are checkpoints, not automatic stopping points.

### Slice 0 — Skeleton and local development

**Objective:**

Establish the repository and development foundation without implementing product behavior yet.

**Implement:**

repository/application structure

Go backend skeleton

Nuxt + Nuxt UI frontend skeleton

PostgreSQL local service

Docker Compose

environment/config handling

.env.example

backend health endpoint

frontend connectivity to backend health endpoint

basic CI

formatting/lint/test commands

protocol/version placeholder/documentation where useful

**Expected local stack:**

Go server
PostgreSQL
Nuxt frontend

**Acceptance criteria:**

entire development stack starts locally

backend can connect to PostgreSQL

backend exposes a health/readiness endpoint

Nuxt can successfully call the backend

CI runs basic backend/frontend validation

configuration is documented

no Slice 1 product functionality is implemented yet

This slice is complete; retain these criteria as regression checks.

### Slice 1 — Agent registration and connectivity

**Objective:**

Connect phBot instances reliably to the backend.

**Implement:**

initial phBot Python plugin

outbound WebSocket connection

agent authentication token

stable agent_id

hello handshake

heartbeat

reconnect/backoff

backend active-agent registry

connected/disconnected status

last-seen timestamp

plugin version

phBot version

Nuxt page showing agents live

**Acceptance criteria:**

starting a phBot instance causes it to appear in the UI

stopping/disconnecting it updates the UI

reconnect works without manual intervention

agent authentication is enforced

### Slice 2 — Character identity and core live stats

**Objective:**

Provide the first genuinely useful monitoring dashboard.

**Character routing contract:**

Agent identity and character identity are separate concerns. An agent identifies the
plugin/runtime connection; it must never implicitly define the target character.

Every character-scoped plugin -> backend and backend -> plugin protocol message MUST
include an explicit `character_id`. This applies to snapshots, live-state updates,
events, observations, chat, commands, acknowledgements/results and future
character-scoped message types. Never infer `character_id` solely from
`agent_id`, the WebSocket connection, or whichever character was most recently seen.

Agent-scoped messages that genuinely have no character target, such as agent hello,
heartbeat, capability/version reporting and connection lifecycle, do not require
`character_id`.

Design the protocol so one agent can represent or multiplex multiple characters
without changing message schemas. The initial plugin/runtime implementation may
still expose only one active character per phBot instance, but the backend and
wire contract must not bake in that limitation. Commands and other mutations must
validate both the authenticated agent and explicit `character_id` before routing.

Character discovery is automatic. There is no required manual “add character” flow:
when an authenticated agent observes a previously unknown joined character, the
backend creates/upserts its stable character record and establishes its
`character_id`. The character must then appear automatically in the UI. Previously
known characters retain their identity and history across disconnects, agent
reconnects, backend restarts and later use by the same or another authorized agent.

Track character presence separately from agent connectivity. An agent may be
connected while no character is online (for example at login/character selection).
A joined character is online only while the backend has a current live character
session for that explicit `character_id`. Track enough session/presence state to
deterministically derive online/offline, including the serving `agent_id`,
join/start time, last-seen/activity time and leave/end reason/time where available.
On normal leave, character switch, agent disconnect or heartbeat/session expiry,
close the affected live character session and mark that character offline. Switching
from character A to B must not leave A falsely online.

Do not derive character presence merely from the existence of a stored character
record or from agent connectivity. The model must remain capable of representing
multiple simultaneous character sessions behind one agent if a future runtime
supports that, without changing the character-scoped wire contract.

**Implement:**

joined-game detection

server identity

character identity

stable backend character record

automatic discovery/registration of previously unknown joined characters

durable per-character online/offline presence and live-session tracking

current character state

level

HP/MP

XP/SP

gold

current position

botting/training state

full snapshot after join/reconnect

searchable character overview across character name, guild, server and zone

persisted character groups with create/rename/delete and explicit membership management

dedicated Nuxt character detail route/surface with identity, live status, progress,
location and links into inventory/equipment, pets, party, map and verified actions;
later slices fill those linked panels without redesigning the detail route

Nuxt character overview/dashboard

**Acceptance criteria:**

previously unknown characters are registered automatically and appear in the UI without manual setup

character online/offline status reflects its live game session independently of agent connectivity

character switching closes the previous character session so stale online state is not retained

current statistics update live

character search filters and persisted groups survive reload/restart and never alter
character identity or command targeting

the character detail surface addresses one stable character_id and keeps links/actions
scoped to that character even when multiple agents/characters are connected

reconnect restores correct current state

character identity does not depend solely on an ephemeral socket connection

all character-scoped protocol messages carry explicit character_id and are never targeted by connection identity alone

one agent can address multiple character identities without a protocol/schema redesign

### Slice 2.5 — Standalone game-data and asset exporter

**Operator-confirmed boundary (2026-09-27):**

Build a standalone offline exporter that reads the GreatestSRO archives and produces
browser-ready assets plus normalized JSON catalogs in a folder the operator copies
to PhMon. This supersedes the earlier PK2-import/backend-catalog implementation plan.
The exporter uses GreatestSRO archives for maps too; do not substitute phBot tiles.

Only the exporter knows source media files, PK2 formats, paths, keys, table layouts
and conversion logic. PhMon consumes only finished JSON and assets. No backend
importer, migration, upload/catalog-management UI, database connection, API changes,
phmonctl import commands or runtime archive access are part of this slice.

Read [the corrected plan](docs/slice-2.5-implementation-plan.md) and
[all screenshot asset requirements](docs/slice-2.5-asset-coverage.md).
Input: `C:\Users\sander\Documents\Silkroad Online\GreatestSRO`, read-only.

**Implement:**

- A standalone tool under `tools/game-data-exporter/`, with inspect/export/validate
  commands, pinned dependencies, tests and reproducible operator instructions.
- A portable versioned output folder containing JSON catalogs, a manifest and
  browser-ready images/maps. Dataset identity and scoped record IDs support
  multiple client versions without collisions. PhMon need not understand their
  source archives. Future slices may associate servers with exported datasets.
- Static item catalogs and supported item/entity/monster/unique art; verified
  skill/mastery/group/localization/icon fields; regions/teleports/minimap tiles;
  candidate portraits; a bounded set of non-control UI symbols; and suitable
  backdrop artwork. Pet-role mapping, taxonomy and other unverified fields remain
  unresolved. Sounds and interface controls are excluded by operator instruction.
  Cover every screenshot plus later-slice requirements not shown in populated shots.
- Source provenance and format diagnostics in exporter-only audit files, outside
  the ready-to-copy app bundle. No source paths, raw client tables or archives in
  the bundle. Keep source archives, exporter-only source audits and temporary
  exports ignored and out of CI/build artifacts. The public alias tree under
  `web/public/game-assets/` is tracked when explicitly requested by the operator.
- Deterministic exports, bounded parsing/conversion, staged output and atomic
  publication. Preserve earlier valid output if a run fails or is cancelled.
- A standalone visual preview and per-family/per-screenshot coverage report.
  No PhMon feature implementation is needed to inspect or validate the output.

phBot remains authoritative for dynamic facts such as plus, quantity, blues,
current durability, positions, HP and learned skills. Exported reference metadata
must not overwrite or fabricate live/historical observations. Unknown source fields
remain unknown. Verify table semantics; opaque raw rows are not a finished catalog.

Export direct minimap imagery from GreatestSRO. `Media.pk2` contains grid-named
raster minimap tiles; `Map.pk2` is selected and hash-checked but its terrain data
is not rendered. Do not use phBot tiles. No region-to-tile or live coordinate
transforms are claimed; Slice 7 owns alignment. Dungeon/floor labels and outdoor
transforms remain unresolved when direct raster data does not establish them. Ask
about uncertain semantics or missing required assets instead of inventing mappings
or silently choosing replacements.

**Acceptance criteria:**

- Standalone exporter runs without PhMon, PostgreSQL, Docker or phBot.
- Authored fixtures and real GreatestSRO export resolve known items, entities,
  skills and maps through usable normalized JSON to decoded browser assets.
- All in-scope asset families have coverage and visual review evidence. Missing
  required families remain explicit blockers to an all-assets-ready claim.
- Repeating with the same inputs/settings/tool versions produces identical bundle
  bytes; changed inputs create separate versions. Scoped identities do not collide.
- Copying only the finished bundle to a clean directory is sufficient to validate,
  preview and resolve IDs to files with archives and source audits inaccessible.
- Malformed inputs fail safely, processing is bounded, no arbitrary host paths are
  accessed and no failed run publishes output as ready.
- All 32 supplied screenshots and five baseline captures are accounted for, plus
  later-slice needs absent from those images. No phMonitor assets are copied.
- Documentation records actual tests, real output counts, unresolved gaps and exact
  export/validate/preview/copy commands. Stop before Slice 3.

### Slice 3 — Remote commands

**Objective:**

Safely control core phBot actions remotely.

**Implement:**

server-to-plugin command protocol

command IDs

acknowledgements

command results

command expiry

audit/history persistence

initial actions, each with an explicit typed argument schema and capability check:

start bot / start training

stop bot / stop training

set training area using the exact verified phBot training-area representation; do not
silently substitute stale backend coordinates for a user-selected/current client area

set training radius with bounded numeric validation matching the verified runtime

walk using the verified phBot walk/destination arguments and current server/region
context; reject unsupported cross-region or malformed destinations rather than guessing

disconnect

return scroll

go clientless where the installed phBot version/API exposes a safe supported primitive

If phBot maps two reference controls to the same underlying primitive, document that
alias and keep one canonical backend command implementation rather than duplicating
side effects.

Expose per-agent/per-character capability reporting so unsupported controls are
disabled with a reason instead of being sent optimistically. Disruptive actions such
as disconnect/go-clientless require an intentional confirmation in the UI. Every
character-scoped command carries explicit character_id and validates that target
against the authenticated agent/session before dispatch.

Nuxt controls with pending/success/failure states and post-command state refresh

**Acceptance criteria:**

commands reach only the intended connected agent

frontend distinguishes sent from successfully executed

expired commands are not replayed unexpectedly

unsupported commands are rejected before dispatch and the UI explains the missing
runtime capability/version requirement

training-area/radius/walk arguments are validated server-side and results record the
effective arguments acknowledged by the plugin where available

command history is inspectable

### Slice 4 — Inventory, pets and party

**Objective:**

Expose important operational game state while reproducing phMonitor's compact,
icon-first inventory/item presentation rather than falling back to generic data tables.

**Reference behavior and visual contract (phMonitor v0.5.0):**

The reference Stats experience treats bag inventory, character set/equipment, storage
and applicable pet inventory as visual Silkroad item collections. Items are identified
primarily by their game icon and in-game presentation; numeric values are overlaid or
shown compactly with the item rather than replacing the collection with a spreadsheet.
Preserve the source slot order and empty slots wherever the source exposes slot
indices, so the UI still reads like the character's actual inventory/storage rather
than a sorted search result. Item Search in Slice 13 is the searchable/table-oriented
cross-source surface; Slice 4 is the per-character operational view.

Do not guess an exact grid column count from an unavailable/empty reference state.
The final responsive slot geometry must be compared against the stored Stats reference
and a populated reference/runtime when available. What is fixed here is the
information hierarchy: compact icon/slot collection -> item preview/detail -> source
and freshness context.

Every occupied item slot uses the best legally usable local item icon available,
preferably resolved from the server's active Slice 2.5 game-data profile, and shows
the source-provided stack/quantity value when applicable. Preserve actual empty
slots. A plus value, rarity/seal or other status may affect the compact label/accent
only when that value is actually observed; never derive item quality from icon color
alone. Missing icons use one deliberate placeholder while retaining the item's name
and identity.

Selecting an item opens one shared **Silkroad-style item detail card** used by bag
inventory, equipped/character-set items, personal storage, guild storage and pet
inventory. Desktop may expose the same card on hover/focus for quick inspection, but
click/tap must pin/open it so touch users can inspect items. Keyboard users must be
able to focus slots and open/close the detail surface. Long item details scroll inside
the popover/drawer/dialog instead of overflowing the page.

The detail card must look like an in-game/phMonitor item description, **not** a generic
two-column key/value table:

- dark navy/blue compact panel, thin blue-gray border and dense line spacing
- item display name first in gold/yellow; append the observed plus value in the normal
  item-name treatment when present
- seal/rarity directly below the name when observed, for example `Seal of Star`
- item classification next, using the reference wording/presentation such as
  `Sort of item: Staff`; show degree/category/subcategory only when the source
  actually exposes them
- base stats as individual readable lines in the same logical order as Silkroad item
  information, with the label/value and any observed percentage/modifier kept on the
  same line
- requirement lines after the base stats; unmet/important requirements may use the
  reference red emphasis. Race such as `European` or `Chinese`, required level,
  gender or other restrictions are shown only when observed/known from verified game
  data
- magical options/blues form the final group and use the reference cyan/blue accent,
  one modifier per line
- static catalog fields such as canonical name, taxonomy, degree, race/requirements
  and base/reference properties may be enriched from the server's active Slice 2.5
  game-data profile when their semantics are static for that model/ref; item-instance
  state such as plus, quantity, observed blues, current durability and other mutable
  values remains sourced from the live/historical observation
- no invented blank rows such as `Critical: -`, no fabricated seals, blues,
  percentages, degree, gender, race or enhancement properties

The public reference item preview demonstrates the expected ordering and semantics for
a weapon: gold item name, seal line, `Sort of item`, attack/reinforce statistics,
critical, durability, attack distance/rate, required level/race, then blue magical
options. The implementation must support that shape without hard-coding it to one
weapon. Render only the fields meaningful to the observed item family:

- **Weapons:** physical/magical attack power, physical/magical reinforce,
  durability, critical, attack distance/range, attack rate/rating and other verified
  weapon stats.
- **Armor/garments/shields:** physical/magical defense or absorption/reinforce,
  durability, parry/blocking and other verified defensive stats.
- **Accessories:** physical/magical absorption and other verified accessory stats.
- **Consumables/materials/stackables:** stack/quantity plus the meaningful verified
  item description/properties; do not force weapon/armor rows.
- **Magic options/blues:** preserve each observed modifier as its own name/value line
  (for example STR/INT increases or blocking-rate modifiers). Preserve the original
  semantic value; do not flatten multiple blues into an opaque JSON string.

The shared item view needs a normalized presentation model, but it must retain the raw
observed/source fields required to improve rendering later. At minimum, where the
verified source provides them, retain:

- stable item/model identity and server/code name
- display name and local icon key/path
- source slot index and stack/quantity
- plus/enhancement value
- item family/type/subtype and degree/level
- seal/rarity/grade/color metadata when actually observed
- durability/current and maximum durability
- physical/magical attack values and reinforcement values
- physical/magical defense/absorption values and reinforcement values
- critical, parry/blocking, attack distance/range and attack rate/rating
- required level, race, gender and other explicit requirements
- all individually observed magical options/blues/attributes
- any other verified item-family-specific lines needed to reproduce the source item
  description without reducing it to a lossy summary

Exact field names and availability must be mapped from the connected phBot runtime/API
and recorded in `docs/phbot-capabilities.md`. Game-data lookups may enrich stable
catalog metadata such as name/type/level, but current mutable item-instance properties
must come from the observed item instance. Unknown means absent/unknown.

Container-level operational metadata stays visually separate from the in-game item
description. Show owner/character, server, source (`Inventory`, `Character Set`,
`Storage`, `Guild Storage`, or the applicable pet), observer and freshness/last
observed state around the collection/detail surface. Do not inject those PhMon
operational fields into the middle of the Silkroad stat block. Last-known storage or
guild-storage data must be clearly marked stale/last observed rather than visually
indistinguishable from currently opened/live state.

**Implement:**

character inventory as a slot-preserving icon collection with empty slots, quantities
and the shared item detail card above

equipped/character-set items as a distinct visual source from bag inventory, preserving
equipment-slot semantics and using the same item detail card

character storage and guild storage where cleanly available, preserving slot/source
identity and carrying observer/freshness metadata so last-known data is not presented
as live without context; use the same icon/detail treatment as character inventory

pet model covering the supported phMonitor categories Attack, Fellow, Pick and
Transport; retain stable pet identity/type and expose the state the verified phBot API
actually provides

pet inventory per applicable pet category using the same slot/icon/item-detail
presentation; do not synthesize inventories for pet types whose runtime source has none

party members/current party state

Party Setup as a separate read/edit surface from current party membership. First
inspect the connected phBot runtime/reference and record the exact available fields
and value semantics in docs/phbot-capabilities.md. Persist only configuration that is
meaningfully durable; apply changes through the Slice 3 command lifecycle, refresh
from the plugin after success, and never claim a saved setup was applied from a socket
write alone.

Verified direct party actions exposed by phBot (for example invite/leave/member
management only when actually supported) use explicit character targets and the same
audited command/result lifecycle. Do not invent controls merely because phBot has a
similarly named internal setting.

delta/change handling keyed by stable source + slot/item identity where appropriate;
do not churn/re-render the entire collection for one changed stack or slot if the
protocol can safely communicate a bounded update

Nuxt inventory/equipment/storage views with slot-preserving icon collections,
source/freshness indicators and responsive item detail surfaces

Nuxt pet view grouped by supported pet category, with applicable pet inventory

Nuxt party view with current-membership and Party Setup sections

**Acceptance criteria:**

current inventory is recognizable as the character's slot-based bag: item icons,
occupied and empty slots, source-provided quantities/stacks and source slot positions
are preserved instead of being rendered as a generic table

selecting representative weapon, armor/shield, accessory, stackable and blue/magic
items opens the shared dark item detail card with the correct per-family fields,
ordering, semantic accents and requirements; fields not supplied by the source are
absent rather than fabricated

a representative sealed/plussed item preserves its observed display name, plus,
seal/rarity and item-instance stats; a representative item with blues preserves each
observed blue as a separate cyan/blue modifier line

inventory, equipment/character set, personal storage, guild storage and applicable pet
inventory reuse the same item-detail semantics so the same item does not render
differently solely because its container changed

current inventory, equipment/character set and available storage sources are visible
without conflating their ownership/source; stale/last-known storage is visibly distinct
from current/live observations

Attack/Fellow/Pick/Transport pets are represented when observed, and applicable pet
state/inventory remains associated with the correct pet across updates

party membership is visible independently from Party Setup configuration

Party Setup loads the plugin's current supported values, applies verified edits through
an audited command, reports pending/success/failure, then refreshes to prove the
effective runtime configuration

desktop hover/focus and click behavior plus mobile tap behavior can inspect the same
item information, with keyboard/focus handling and no page-level overflow at the
project's target viewports

updates do not require blindly resending excessive full state when unnecessary

### Slice 5 — Event pipeline

**Objective:**

Move from current-state monitoring to durable activity history.

**Canonical events should include where available:**

death

item drop

unique spawn

teleport

level-up

disconnect

reconnect

alchemy result

custom event kind with a bounded payload so later Conditions can emit durable custom
events without inventing a second timeline model

**Implement:**

event envelope/schema

plugin event publishing

nonblocking outbound queue

durable backend event storage

item-event payloads/snapshots that retain every actually observed display/detail field
needed by later UI: canonical item identity/model/code, display name, plus value,
quantity/stack where relevant, rarity/seal metadata, degree/category, observed item
color/grade and observed blues/attributes. Fields absent from the source remain absent;
never infer a seal, blue, rarity or probability from presentation alone.

Nuxt activity timeline

basic event filtering

**Acceptance criteria:**

events survive page reload/backend querying

timeline ordering is reliable

rare/normal drop rows and detail links can render the stored observed item semantics
without re-querying mutable current inventory state

event ingestion does not block normal phBot behavior

### Slice 6 — Chat

**Objective:**

Provide remote chat visibility and sending.

Implement inbound chat where supported:

private

party

guild

union

general

other useful supported channels

Implement remote sending where supported.

**Add:**

persistent/appropriate history

per-character chat UI

command/result handling for outbound messages where necessary

**Acceptance criteria:**

incoming messages appear in the web UI

supported outbound chat can be sent remotely

messages are attributed to the correct character/channel

### Slice 7 — Live map

**Objective:**

Show actual live game-world state without video capture.

**Implement:**

canonical coordinate model

character current position

position history where useful

map/region normalization, including explicit transform/asset validation for:

- Jangan Cave / Tomb of Qin-Shi
- Donwhang Cave / Donwhang Stone Cave
- Job Temple / Temple

character picker plus jump-to-character

quick destination/region navigation

current cursor/viewport coordinates, tile identifier and zoom percentage

current character markers and Academy-member layer

live nearby-monster markers from current plugin observations, distinct from historical
mob-density/type analytics. Track enough identity/model/type/position plus freshness/TTL
to remove monsters that are no longer current instead of leaving stale markers.

recent death and drop overlays sourced from durable events with bounded time-range
filters and links back to their detail/event context

nearby-drop/current-world overlay only where a verified live phBot source exists; do
not present historical drop events as currently lying on the ground

Nuxt map component with pan/zoom and layer controls

map/region reference data and locally served assets from the active Slice 2.5
game-data profile where the exported bundle contains them, with explicit fallback
provenance for any additional operator-supplied/licensed assets

legally usable/private map assets for outdoor and required special-area maps; imported
PK2 imagery does not remove the requirement to validate coordinate transforms

**Acceptance criteria:**

live character position is visible on a map and jump-to-character centers the expected
character

movement updates correctly

nearby monsters appear/disappear according to current observations and freshness;
the live-monster layer never derives "current" monsters from historical heatmap data

region transitions are handled

the three verified special-area map families use validated assets/transforms and do
not silently fall back to incorrect outdoor coordinates

recent-death/drop time filtering and Academy-member visibility work independently

map architecture supports future heatmap layers

### Slice 8 — Mob observation and heatmap foundation

**Objective:**

Collect statistically meaningful mob-density data.

Plugin publishes compact observation batches containing:

observer character

timestamp

region

character/observer coordinates

nearby monster identity/model/type

nearby monster coordinates

Reuse the canonical live-monster observation identity/coordinate model introduced in
Slice 7 where practical, but keep current presence and historical density semantics
separate. A monster disappearing from the live layer does not delete its valid sample,
and a historical sample must never resurrect a live marker.

Backend implements spatial aggregation.

Do not use naïve cumulative sightings alone.

Track enough information to derive values such as:

observation_samples
mob_observations
unique/identified mob observations where useful
mob types

Conceptual density:

density = mob observations / observation samples

Exact spatial model may evolve during implementation.

**Acceptance criteria:**

observations from multiple characters can contribute

standing still for a long period does not incorrectly create arbitrarily hot cells merely because time passed

backend can query spatial mob density by area/time range

### Slice 9 — Heatmaps

**Objective:**

Expose the accumulated spatial analytics in the UI.

Implement heatmap rendering and filters.

**Initial layers:**

mob density

mob types

deaths

drops

unique sightings

player movement

The live nearby-monster layer remains a current-state layer from Slice 7, not a
heatmap. It may be displayed alongside these historical layers but must retain
separate freshness semantics.

**Useful filters:**

time range

region

mob type

character

server

Add backend pre-aggregation where justified for larger ranges.

Heatmap reset must show exactly which server/region/layer/time-derived aggregate will
be removed. Default to the narrowest meaningful current scope; require explicit
confirmation for broader deletion. Clear/rebuild derived aggregates without silently
deleting unrelated canonical event history.

Named special-area maps from Slice 7 must support the historical layers whose
coordinates can be normalized correctly; unsupported combinations are shown as such
rather than plotted with a guessed transform.

**Acceptance criteria:**

heatmaps render from backend data

time filtering works

layers can be enabled/disabled

reset affects only the confirmed scope and leaves unrelated event/history sources
intact

heatmaps and event overlays align with validated coordinates on supported special maps

performance remains reasonable for accumulated historical data

This slice marks the target for the first complete phMonitor-replacement MVP.

### Slice 10 — Conditions and automation

**Objective:**

Build a generic server-owned rules engine.

**Concept:**

WHEN conditions
THEN actions

**Minimum initial inputs:**

HP/MP thresholds

disconnected

inventory full

bot stopped

death

unique seen

item dropped

**Minimum initial actions:**

notification

Discord webhook

phBot command

Model each rule with stable ID, enabled state, explicit character/server target scope,
trigger/input, predicates, ordered actions and audit metadata. Inspect the reference
Condition editor and verified phBot capabilities before expanding the trigger/operator/
action catalog; newly discovered supported controls belong in this slice rather than
being deferred to a separate automation system.

Support the reference-style placeholders/variables used inside condition-generated
messages/content. Define and document a bounded variable catalog sourced from the
trigger event plus current character/server context. Expand templates on the backend
at execution time with deterministic handling for missing/null values, explicit output
length limits and escaping appropriate to the destination. Provide preview/test
coverage using synthetic context. Templates are data only: no eval, Python, shell,
arbitrary expressions or unrestricted property traversal.

When a Condition intentionally creates a custom timeline event, emit the Slice 5
canonical custom event so filtering/order/retention remain consistent.

Keep rule evaluation on the backend.

Do not implement condition logic inside individual plugins.

**Acceptance criteria:**

rules are persisted

rules evaluate deterministically

placeholder expansion is deterministic, tested for missing values and bounded against
oversized/untrusted content

triggered actions are auditable

an unlimited number of self-hosted rules can be created subject only to practical resource limits

### Slice 11 — Scheduling

**Objective:**

Support server-owned scheduled actions.

**Initial actions may include:**

start bot

stop bot

return scroll

disconnect

**Implement:**

persisted schedules

scheduler execution

command integration

missed-run semantics

expiration behavior

Nuxt schedule/calendar editor

**Acceptance criteria:**

schedules survive backend restart

commands execute against the intended agent/character

missed/late commands follow documented semantics

### Slice 12 — Analytics

**Objective:**

Convert monitoring history into useful performance data.

At minimum reproduce the reference analytics areas for Deaths, Rare Drops, Normal
Drops, Economy and Academy with explicit time-range filtering and server/character
scope where the underlying records support it. Reuse canonical event/item/economy/
academy records rather than creating analytics-only duplicate truth.

**Potential metrics:**

XP/hour

SP/hour

gold/hour

deaths/hour

drops/hour

session duration

bot uptime

farming-area comparison

daily/weekly summaries

Implement historical charts/tables in Nuxt. Item analytics reuse the canonical item
presentation/detail semantics from Slices 5/13 so seal, rarity and observed blues do
not diverge between Events, Item Search and Analytics.

**Acceptance criteria:**

metrics are derived from durable backend data

time-range queries work

calculations are documented/tested

dashboard remains usable over meaningful historical ranges

### Slice 13 — Economy and item analytics

**Objective:**

Add item-centric historical/search functionality when the available phBot data supports it.

**Required functionality where the verified source data exists:**

item acquisition/drop history with links back to canonical events

valuable/rare drop tracking preserving observed rarity/color/seal metadata

one Item Search query surface across character inventory, equipped/character sets,
character storage and guild storage; retain source/owner, server, observer/freshness
and navigation back to the owning character/guild

text/server/item-type/subcategory/degree filtering, include-character-sets behavior
where it matches the reference, and deterministic reset controls

shared item detail presentation for current and historical records, including plus,
quantity, degree/category, seal/rarity/color and observed blues/attributes when the
source captured them; never synthesize absent item properties

global economy offers with WTB/WTS/WTT classification and searchable observed offer
text/item/seal/notes plus buyer/seller and item taxonomy filters

stall sale/transaction history and source attribution where available

price history only where sufficiently reliable observable source data exists

Do not invent data that phBot cannot provide.

Do not overbuild this slice before confirming actual source capabilities.

**Acceptance criteria:**

Item Search finds the same canonical item regardless of whether it currently lives in
inventory, equipment/character set, character storage or guild storage, while still
showing its exact source/owner/freshness.

Rare/normal historical item detail renders stored observed metadata consistently with
Events and Analytics.

Economy filters distinguish WTB/WTS/WTT and preserve source attribution.

Unsupported/unobservable price or item fields remain absent/unknown rather than being
estimated or copied from mutable current state.

### Slice 14 — Hardening

**Objective:**

Make the system safe and maintainable for long-running self-hosted use.

Implement as justified:

token rotation

token revocation

user authentication/authorization

command authorization

rate limiting

schema migrations

protocol version negotiation

plugin compatibility handling

queue bounds

backpressure

observability

retention rules

high-volume map observation cleanup/aggregation

deployment documentation

upgrade procedures

**Acceptance criteria:**

operational failures are observable

high-volume data has bounded storage behavior

agent credentials can be revoked/rotated

version incompatibilities fail clearly

system can be upgraded predictably

### Slice 15 — Demo parity completion and final acceptance

**Objective:** Finish the full applicable demo feature set and visual/interaction
parity, then verify the complete self-hosted product. This is not permission to defer
core screens or visual direction until the end; build them in their owning slices.

**Implement:**

- Close every open row in the feature matrix, including any incomplete advanced
  phBot tools. Reinspect accessible advanced screens and document verified behavior.
  Final phBot-tool parity includes the verified training/client command set, Party
  Setup round-trip, explicit-target script execution/management and supported Quest
  actions; generic placeholder buttons do not satisfy this gate.
- Reconcile character detail so live state, inventory/equipment/storage, the
  Attack/Fellow/Pick/Transport pet categories, party state/setup, map navigation and
  supported actions use one stable character target and consistent drill-down model.
- Reconcile map parity across outdoor and the verified special-area maps, including
  live nearby monsters with freshness, Academy members, recent event overlays and
  historical mob-density/type layers. Validate coordinates rather than accepting
  visually plausible but incorrect placement.
- Skill Builder using the active Slice 2.5 game-data profile for versioned skill
  identity/icons/reference metadata where available, plus verified race/cap/mastery
  and prerequisite rules, editable/saved plans, calculated SP costs, reset/bulk
  controls and live-character comparison. Show unsupported versions honestly. Remote skill
  execution, if supported, uses the existing command lifecycle and confirmation.
- Complete persistent appearance/mode/language/chat/notification settings, local
  sound library, safe record-management dialogs, instance QR/copy-link utilities
  and operator-managed dashboard information cards.
- Complete backend/API validation for preferences, uploads, webhook destinations,
  local assets and deletion operations; avoid introducing privileged arbitrary file
  access or outbound requests to internal services through user-supplied URLs.
- Finish shared item-detail/event rendering so rare-item color/seal metadata and
  observed normal-item blues/attributes agree across Events, character inventory,
  Item Search and Analytics without fabricating unavailable fields.
- Finish Conditions placeholder/variable behavior with documented bounded context,
  deterministic preview/execution and no executable template language.
- Reconcile the demo navigation, filters, detail surfaces and responsive layouts
  across every implemented area. Replace temporary scaffolding, broken controls,
  misleading mock data and unintended framework-default styling.
- Finish `docs/reference-parity.md` and `docs/phbot-capabilities.md` with behavior,
  visual evidence, source/version verification, deliberate adaptations and blockers.

**Acceptance criteria:**

- Every feature-matrix row has implementation and verification evidence. No hidden
  required gap is reclassified as optional merely to finish the roadmap.
- Skill planning calculations/prerequisites have focused tests with known reference
  data; saved plans and settings survive reload/restart as appropriate.
- Every supported phBot control has a capability-aware targeted command test; training
  area/radius/walk arguments and Party Setup prove the plugin's effective resulting
  state rather than treating dispatch as success.
- Live-monster expiry and special-map coordinate fixtures are tested, and representative
  rare/normal items verify seal/color/blues/detail consistency across surfaces.
- Conditions template variables have deterministic preview/execution tests, including
  missing values and output bounds.
- Representative populated and empty screens, detail views, dialogs and failure/
  recovery flows pass browser checks at all target viewports. No reference outage
  popup, phMonitor branding, paid gating or dependency is carried into our product.
- The entire final definition of done above passes, or remaining hard blockers are
  explicitly reported as incomplete. Passing Slice 9 or 14 alone is not final parity.

## Milestones and dependency order

- Slices 0–4, including Slice 2.5: first usable monitoring/control system.
- Slices 0–9, including Slice 2.5: complete phMonitor-replacement MVP target.
- Slices 10–14: advanced self-hosted platform.
- Slice 15 and all final gates: complete applicable demo feature/layout/style parity.

Milestones organize progress. Respect dependencies, but do not stop at a milestone
when the full implementation contract is active.

```text
foundation -> connectivity -> character state -> offline asset export -> commands
           -> inventory/events -> map observations -> heatmaps
           -> conditions / scheduling / analytics
```

Reuse earlier concrete abstractions where appropriate; do not create speculative
infrastructure for later slices. UI parity develops alongside functional slices;
Slice 15 consolidates it. Keep this guide aligned with repository reality.
