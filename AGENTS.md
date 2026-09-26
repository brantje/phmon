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
Map, Item Search, Skill Builder, Settings and Server List were navigable. Most data
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
  components. Use locally served original, licensed or operator-supplied artwork,
  map tiles and item icons. Record asset provenance. Do not hotlink phMonitor assets,
  embed its application, copy its client bundle or connect to its services.

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
  The public server directory must be self-hosted/operator-managed or explicitly
  sourced from an allowed feed, never scraped from a private phMonitor backend.
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
| Stats and character details | Search characters/guild/server/zone, create/edit groups, live stats and progress, current status, character details with inventory/equipment/pets/party and available actions. Preserve character identity and group membership across restarts. | 2–4, 12 |
| Events | Unified timeline plus level-up/custom/death/rare-drop/normal-drop/unique filters; character/item/date filtering, counts, pagination and map links. Persist occurrences with reliable ordering. | 5, 7 |
| Chat | General/private/party/guild/union/global tabs; sender character selector, private contacts/new conversation, recipient field, history and jump-to-latest, message composer and results. Add emoji/item references where supported; confirm costly/global sends. | 6, 13, 15 |
| Economy | Global buy/sell/trade offers and stall views; text/character/item-type/subcategory/degree filters, reset controls, stall transactions/chat and source attribution. Derive history only from observable data. | 6, 13 |
| Alchemy | Current attempt log, historical item sessions, highest plus and success/failure/attempt counts; character/item/type/degree filters; statistics over recorded attempts. Do not fabricate probabilities. | 5, 12 |
| Academy | Owned/joined academy tabs, membership/state, join/leave/graduation activity, unread log and mark-read action; map member layer and historical metrics where supported. | 4, 5, 7, 12 |
| Guild Storage | Guild-scoped item listing/detail, search integration, freshness/observer attribution and explicit confirmed removal of stored records. | 4, 13, 15 |
| phBot tools | Client/bot controls, party management, script management and quest information/actions where the public phBot API permits them. Investigate each tool's real controls; route all mutations through authenticated audited commands, never arbitrary remote Python/shell execution. | 3, 4, 15 |
| Analytics | Character/session rates, deaths, rare/normal items, economy and academy analyses; time/server/character filters, charts and documented calculations backed by durable data. | 12, 13 |
| Map | Pan/zoom, region/quick destination selection, character picker/jump-to-character, coordinates/tile/zoom display; characters and academy members, recent deaths/drops with time ranges, mob-density/types. Safe confirmation and scoped deletion for heatmap reset. | 7–9 |
| Item Search | Search inventory/equipment/character sets, storage and guild storage; text/server/type/subcategory/degree filters, reset, item details and owner/source navigation. | 4, 13 |
| Skill Builder | Chinese/European builds, game-version/cap selection (demo exposes 110/120/140), mastery/skill prerequisites and level adjustment, bulk increment/decrement shortcuts, reset, SP totals and comparison with a live character. Verify skill datasets and rules per supported version; distinguish planning from execution. | 15 |
| Automations | Conditions and schedules tabs, add/edit/enable/disable/delete, target selection, backend evaluation/execution, expiry/missed-run handling and auditable results. No paid rule-count limits. | 10, 11 |
| Settings | Language selection with working translations for offered locales; easy/advanced mode; primary/background/text colors; icon sizes (45/60/75 px) and text sizes (11/14/18 px); persisted chat/notification preferences; plugin install/config guidance. | 1, 6, 15 |
| Notifications | Per-event sound/browser notification preferences for messages, deaths, rare drops, alchemy thresholds, uniques, academy changes, offline state, sales and level-ups; local WAV library upload/preview/assignment. Browser permissions are explicit. Discord webhook CRUD/test/delivery with redacted secrets, bounded retries and observable results. | 5, 6, 10, 15 |
| Record management | Character and guild-record deletion with typed-name confirmation, scope/retention explanation and server-side authorization. No accidental bulk removal. | 14, 15 |
| Server List | Self-hosted directory with version/cap/model filters, server detail/rates/concept metadata, optional artwork and pagination. No invented online/player-count claims or dependency on the reference's directory. | 15 |
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

**Slice 0: complete and locally validated (2026-09-26). Slice 1: in progress
(implementation and automated validation complete; runtime/browser gates pending).
Slices 2–15: not started.**
The active work is **Slice 1 — Agent registration and connectivity**. Its production
path and simulator-backed automated coverage are implemented and hosted CI is green.
Do not begin Slice 2 until the remaining Slice 1 validation gates are closed or
explicitly accepted as blocked.

**Current turn (2026-09-26): Slice 1 completion/validation.** The authenticated
connection contract, durable agent identity, plugin, simulator, backend registry/API,
reference shell and stack lifecycle tests are implemented. The WebSocket client now
polls for readiness before consuming a frame and treats a stall after frame decoding
begins as a broken connection, preventing partial-frame stream corruption. Real
Windows/phBot runtime validation and same-viewport browser screenshot comparison
remain explicit open gates rather than being inferred from simulator/source results.

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
  in-memory active registry and a safe read-only agent presentation API. New valid
  sessions supersede old sockets without stale cleanup marking the replacement
  offline.
- Added phmonctl agent create for one-time credential provisioning. No browser route
  exposes tokens or hashes.
- Added plugin/PhMon.py using only Python standard-library networking. Public phBot
  documentation verifies socket support, while actual embedded-runtime availability
  of ssl, select, threading, hashlib, base64, struct and urllib.parse still requires
  the real phBot validation gate. A dedicated worker owns network I/O; phBot callbacks
  never block on the backend. Added scripts/agent_simulator.py using the exact same
  transport/worker contract.
- Replaced the temporary Slice 0 page with the first reference-style PhMon shell:
  compact sidebar/header, persisted easy/advanced and collapse preferences, live
  agent list, loading/stale/error/recovery states, responsive agent cards and
  credential-free instance copy/QR access. Health remains an operational diagnostic.
- Extended CI to provision a real test credential and exercise
  connect -> Go restart -> automatic reconnect -> disconnect through
  plugin transport -> Go -> PostgreSQL -> Nuxt, in addition to database
  outage/recovery checks.
- Validation actually observed in this completion pass: Python compile checks and
  14 focused stdlib plugin protocol/config/backoff tests pass locally. GitHub Actions
  run 36256023045 passed both jobs on transport/test head 677804e: validate ran
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
- Exact next action: validate PhMon.py inside a real supported phBot runtime and run
  side-by-side browser screenshot checks at all three required viewports. Do not
  start Slice 2 as part of this task.

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

**Implement:**

joined-game detection

server identity

character identity

stable backend character record

current character state

level

HP/MP

XP/SP

gold

current position

botting/training state

full snapshot after join/reconnect

Nuxt character overview/dashboard

**Acceptance criteria:**

character appears after joining the game

current statistics update live

reconnect restores correct current state

character identity does not depend solely on an ephemeral socket connection

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

initial actions:

start bot

stop bot

disconnect

return scroll

Nuxt controls with pending/success/failure states

**Acceptance criteria:**

commands reach only the intended connected agent

frontend distinguishes sent from successfully executed

expired commands are not replayed unexpectedly

command history is inspectable

### Slice 4 — Inventory, pets and party

**Objective:**

Expose important operational game state.

**Implement:**

character inventory

storage where cleanly available

pets

pet inventory

party members/state

delta/change handling where appropriate

Nuxt inventory view

Nuxt pet view

Nuxt party view

**Acceptance criteria:**

current inventory is visible

pet state/inventory is visible

party membership is visible

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

**Implement:**

event envelope/schema

plugin event publishing

nonblocking outbound queue

durable backend event storage

Nuxt activity timeline

basic event filtering

**Acceptance criteria:**

events survive page reload/backend querying

timeline ordering is reliable

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

map/region normalization

current character markers

optional nearby-monster overlay

optional nearby-drop overlay

Nuxt map component

legally usable/private map assets

**Acceptance criteria:**

live character position is visible on a map

movement updates correctly

region transitions are handled

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

**Useful filters:**

time range

region

mob type

character

server

Add backend pre-aggregation where justified for larger ranges.

**Acceptance criteria:**

heatmaps render from backend data

time filtering works

layers can be enabled/disabled

performance remains reasonable for accumulated historical data

This slice marks the target for the first complete phMonitor-replacement MVP.

### Slice 10 — Conditions and automation

**Objective:**

Build a generic server-owned rules engine.

**Concept:**

WHEN conditions
THEN actions

**Possible initial inputs:**

HP/MP thresholds

disconnected

inventory full

bot stopped

death

unique seen

item dropped

**Possible actions:**

notification

Discord webhook

phBot command

Keep rule evaluation on the backend.

Do not implement condition logic inside individual plugins.

**Acceptance criteria:**

rules are persisted

rules evaluate deterministically

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

Implement historical charts in Nuxt.

**Acceptance criteria:**

metrics are derived from durable backend data

time-range queries work

calculations are documented/tested

dashboard remains usable over meaningful historical ranges

### Slice 13 — Economy and item analytics

**Objective:**

Add item-centric historical/search functionality when the available phBot data supports it.

**Potential functionality:**

item acquisition history

valuable drop tracking

search across observed items

economy/stall information where available

price history where sufficiently reliable source data exists

Do not invent data that phBot cannot provide.

Do not overbuild this slice before confirming actual source capabilities.

Acceptance criteria should be defined based on the verified API/data available at implementation time.

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
- Skill Builder with legally usable/versioned skill datasets, race/cap/mastery and
  prerequisite rules, editable/saved plans, calculated SP costs, reset/bulk controls
  and live-character comparison. Show unsupported versions honestly. Remote skill
  execution, if supported, uses the existing command lifecycle and confirmation.
- Complete persistent appearance/mode/language/chat/notification settings, local
  sound library, safe record-management dialogs, instance QR/copy-link utilities,
  self-hosted server directory and operator-managed dashboard information cards.
- Complete backend/API validation for preferences, uploads, webhook destinations,
  local assets and deletion operations; avoid introducing privileged arbitrary file
  access or outbound requests to internal services through user-supplied URLs.
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
- Representative populated and empty screens, detail views, dialogs and failure/
  recovery flows pass browser checks at all target viewports. No reference outage
  popup, phMonitor branding, paid gating or dependency is carried into our product.
- The entire final definition of done above passes, or remaining hard blockers are
  explicitly reported as incomplete. Passing Slice 9 or 14 alone is not final parity.

## Milestones and dependency order

- Slices 0–4: first usable monitoring/control system.
- Slices 0–9: complete phMonitor-replacement MVP target.
- Slices 10–14: advanced self-hosted platform.
- Slice 15 and all final gates: complete applicable demo feature/layout/style parity.

Milestones organize progress. Respect dependencies, but do not stop at a milestone
when the full implementation contract is active.

```text
foundation -> connectivity -> character state -> commands -> events
           -> map observations -> heatmaps -> conditions / scheduling / analytics
```

Reuse earlier concrete abstractions where appropriate; do not create speculative
infrastructure for later slices. UI parity develops alongside functional slices;
Slice 15 consolidates it. Keep this guide aligned with repository reality.
