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

**Git workspace rule:** Never create, use, or switch to a Git worktree unless the user explicitly asks for a worktree. Work in the existing checkout/branch by default.

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

Operator exception (2026-09-27): the user explicitly authorized inspecting and
decompiling the local `phMonitor-v0.5.0.exe` to understand Slice 4 item statistics
and blue options. This overrides the client-inspection restriction for that bounded
investigation. Keep the implementation independent and retain the restrictions on
service dependencies, entitlement bypass and operating characters without approval.
Evidence: [item tooltip investigation](docs/reference/item-tooltip-investigation.md).

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

### Authorized phMonitor executable analysis

The operator explicitly authorizes static inspection and decompilation of
`%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe` to understand phMonitor's behavior,
data handling and calculations and inform PhMon's independent implementation.
Use this username-free path when documenting the executable's location.

This permission supersedes any earlier blanket prohibition or historical resume
entry that says phMonitor's client cannot be reverse-engineered. Findings may be
documented and used to guide independently authored code; do not commit or
redistribute the executable, extracted client bundles or decompiled source.
The restrictions on bypassing paid-access controls, accessing private services
and depending on phMonitor infrastructure remain in force.

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
  column, conversation pane and bottom composer. Map uses a large viewport with
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

| Area / navigation           | Required behavior and layout                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | Owning slices   |
| --------------------------- | ------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------------- | --------------- |
| Shell and instance access   | Reference sidebar/header, server scope, connection/version state, responsive navigation, easy/advanced mode, instance URL copy and mobile QR panel. Persist preferences; scope data consistently.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                         | 1, 2, 15        |
| Dashboard                   | Fleet online/offline/alive/dead counts, gold total, recent deaths/events/rare drops/chat/trade offers, server-information card and working drill-down links.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 2, 5, 6, 13, 15 |
| Stats and character details | Search characters/guild/server/zone, create/edit groups, live stats and progress, current status, and a dedicated character detail surface. Detail views include inventory/equipment, supported pet classes (Attack/Fellow/Pick/Transport) with applicable state/inventory, party membership/setup and verified actions. Preserve character identity and group membership across restarts.                                                                                                                                                                                                                                                                                                                                | 2–4, 12         |
| Events                      | Unified timeline plus level-up/custom/death/rare-drop/normal-drop/unique and item-acquisition/transfer filters; character/item/date filtering, counts, pagination and map links. Keep world drops distinct from owned-item gains; preserve acquisition destination/container and only attach party/pet/pickup provenance when verified. Rare-drop presentation preserves observed rarity/seal/color/detail metadata; normal-drop detail preserves observed blues/attributes where the source exposes them. Persist occurrences with reliable ordering without inventing missing item properties or acquisition causes.                                                                                                    | 4 (death increment only), 5, 7, 13 |
| Chat                        | General/private/party/guild/union/global tabs; sender character selector, private contacts/new conversation, recipient field, history and jump-to-latest, message composer and results. Add emoji/item references where supported; confirm costly/global sends.                                                                                                                                                                                                                                                                                                                                                                                                                                                           | 6, 13, 15       |
| Economy                     | Global buy/sell/trade offers and stall views; text/character/item-type/subcategory/degree filters, reset controls, stall transactions/chat and source attribution. Derive history only from observable data.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                              | 6, 13           |
| Alchemy                     | Current attempt log, historical item sessions, highest plus and success/failure/attempt counts; character/item/type/degree filters; statistics over recorded attempts. Do not fabricate probabilities.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | 5, 12           |
| Academy                     | Owned/joined academy tabs, membership/state, join/leave/graduation activity, unread log and mark-read action; map member layer and historical metrics where supported.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | 4, 5, 7, 12     |
| Guild Storage               | Guild-scoped item listing/detail, search integration, freshness/observer attribution and explicit confirmed removal of stored records.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                    | 4, 13, 15       |
| phBot tools                 | Client/bot controls explicitly cover start/stop bot or training, set training area, set training radius, walk, disconnect, return scroll and go clientless where the verified phBot API supports each action. Party Setup must reproduce the verified reference control surface and round-trip current configuration/state. Scripts must be discoverable/listable, manageable where supported and executable for explicit character targets; Quest exposes verified information and supported actions. Investigate each tool's real controls and argument semantics before implementation. Route every mutation through authenticated, capability-aware, audited commands; never arbitrary remote Python/shell execution. | 3, 4, 15        |
| Analytics                   | Character/session rates, deaths, rare/normal items, economy and academy analyses; time/server/character filters, charts and documented calculations backed by durable data.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                               | 12, 13          |
| Map                         | Pan/zoom, region/quick destination selection, character picker/jump-to-character, coordinates/tile/zoom display; cave-specific floor bar, floor imagery and Back to world map; right-click plus touch/keyboard point actions for verified generated-script navigation and active training-area positioning; characters and academy members, recent deaths/drops with time ranges, live nearby-monster markers, mob-density/types and other historical layers. Use the server's versioned exported dataset for region/map reference data and local assets where available. Validate dedicated map/coordinate handling for Jangan Cave / Tomb of Qin-Shi, Donwhang Cave / Donwhang Stone Cave and Job Temple / Temple instead of assuming PK2 presence proves the outdoor transform applies. Safe confirmation and explicit server/region/layer scope for heatmap reset.                                               | 2.5, 7–9        |
| Item Search                 | Search inventory/equipment/character sets, storage, guild storage, applicable pet inventories and job pouch where verified; text/server/type/subcategory/degree filters, reset, item details and owner/source navigation. Reuse shared SRO artwork and static item definitions by stable game item code across server scopes; use the active profile to map numeric model IDs and apply version-specific overrides. Preserve live/historical instance facts and exact container provenance from their observed source.                                                                                                                                                           | 2.5, 4, 13      |
| Skill Builder               | Chinese/European builds, game-version/cap selection (demo exposes 110/120/140), mastery/skill prerequisites and level adjustment, bulk increment/decrement shortcuts, reset, SP totals and comparison with a live character. Prefer versioned skill/reference data from the server's exported game-data profile where present; verify rules per supported version and distinguish planning from execution.                                                                                                                                                                                                                                                                                                                | 2.5, 15         |
| Automations                 | Conditions and schedules tabs, add/edit/enable/disable/delete, target selection, backend evaluation/execution, expiry/missed-run handling and auditable results. Condition/action content supports the verified phMonitor-style placeholders/variables through a bounded server-side template context with deterministic missing-variable behavior; templates never execute arbitrary code. No paid rule-count limits.                                                                                                                                                                                                                                                                                                    | 10, 11          |
| Settings                    | Language selection with working translations for offered locales; easy/advanced mode; primary/background/text colors; icon sizes (45/60/75 px) and text sizes (11/14/18 px); persisted chat/notification preferences; plugin install/config guidance.                                                                                                                                                                                                                                                                                                                                                                                                                                                                     | 1, 6, 15        |
| Notifications               | Per-event sound/browser notification preferences for messages, deaths, rare drops, alchemy thresholds, uniques, academy changes, offline state, sales and level-ups; local WAV library upload/preview/assignment. Browser permissions are explicit. Discord webhook CRUD/test/delivery with redacted secrets, bounded retries and observable results.                                                                                                                                                                                                                                                                                                                                                                     | 5, 6, 10, 15    |
| Record management           | Character and guild-record deletion with typed-name confirmation, scope/retention explanation and server-side authorization. No accidental bulk removal.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                  | 14, 15          |
| Operations                  | Usable setup, auth/agent token management, compatibility reporting, backups/restore/migrations, retention and deployment/upgrade instructions.                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                                            | 1, 14           |

Advanced phBot/analytics/automation screens and hidden subtabs still require focused
reference inspection when accessible. Their labels were visible in public markup;
only the visible easy-mode flows were exercised during the initial inspection.

### Map cave evidence and 2D point decision — 2026-09-29

The approved phMonitor v0.5.0 static investigation and GreatestSRO `Media.pk2`
inspection established 17 cave floor tile sets. The exporter publishes their
converted PNGs locally and the Greatest versioned map profile supplies each
floor's tile bounds, 192-unit X/Y anchor, region IDs, and observed Donwhang Z
bands. Tomb regions -32761..-32766 identify B1..B6. Donwhang regions
-32767/32767 use Z bands -50..70, 71..210, 211..350, 351..490. Job Temple's
region -32752 identifies 1F only; upper and annex images require manual floor
selection and are not inferred from that region.

The map point itself is 2D. A command uses converted X/Y and explicit region,
then reuses the selected character's current Z or zero if unavailable. Do not
add per-pixel terrain height or require Z calibration. Ambiguous cave region
selection keeps point actions unavailable. Generated navigation is capability
gated on documented `generate_script` and `start_script`; it validates bounded
route lines and reports phBot's result without claiming arrival. Reference,
phBot API, and live/stale sample distinctions are recorded in
`docs/phbot-capabilities.md` and `docs/reference-parity.md`.

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
system or infrastructure.
Local executable inspection and decompilation are permitted under
"Authorized phMonitor executable analysis" above.

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
- Use Nuxt file-based routing in `web/app/pages/` for every implemented screen,
  including dynamic routes such as `characters/[id].vue`. Follow the
  [Nuxt pages convention](https://nuxt.com/docs/4.x/directory-structure/app/pages).
  Keep `web/app/app.vue` limited to application providers and the
  `<NuxtLayout><NuxtPage /></NuxtLayout>` outlet; never select whole screens with
  pathname checks or accumulate feature markup/state there.
- Put the persistent application shell in `web/app/layouts/` and reusable UI in
  `web/app/components/`. Pages compose focused feature components; components own
  their local forms, dialogs and interaction state. Extract shared reactive logic
  into focused `web/app/composables/` and pure formatting into `web/app/utils/`.
  Do not move a monolithic app into a single oversized layout or composable.
  Keep one shared live-data transport, use `NuxtLink` for internal navigation,
  and clean up page-specific subscriptions, watchers and timers on navigation.
  Verify direct route loads, client navigation, back/forward navigation and mobile
  shell behavior whenever this structure changes.
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

**Slice 4 implementation checkpoint (2026-09-27):** the local worktree now has
protocol-v4 resource snapshots/deltas, bounded chunk assembly, revision/session
fencing, PostgreSQL resource/item persistence, documented phBot API collectors,
visible-card resource subscriptions, grouped Stats cards, four-column/eight-row bag
pages, a shared accessible item preview, equipment/storage/pet/party/academy views,
the existing character-targeted Actions component, a server/guild-scoped Guild
Storage page with search/observer freshness, and a persisted server selector applied
to fleet summaries, Stats and Guild Storage. Final local Go tests/vet, 39 plugin
tests, Nuxt typecheck, lint, format check and production build pass; lint retains 13
existing void-element style warnings. The PostgreSQL integration test is compiled but
skips without `TEST_DATABASE_URL`; this host has no local PostgreSQL or Docker, and
race coverage therefore remains pending. The operator-supplied LAN reference was
opened and its Stats Overview, Info (character set), Progress, Inventory, Storage and
Pet tabs inspected. The local browser still reaches the operator sign-in gate, so
authenticated local screenshots remain unreviewed. The Stats card now follows the
reference Info/Progress split and defaults Storage to personal storage. API
capacity/equipment-slot mapping and current phBot runtime collection remain
unverified; item packet parsing is disabled because the published
vSRO 1.188 references available here do not provide a verified item layout or packet
fixture. Static item catalog-to-server binding and Party Setup write/reload/readback
also remain open, as does explicit guild-record deletion. Do not claim Slice 4
acceptance until these gates are resolved or recorded as explicit acceptance
blockers. The user explicitly scoped this run to Slice 4; stop before Slice 5. Exact
next action: receive the updated plugin from the user and validate its live API/config
observations; then review authenticated local screenshots and obtain version-specific
item packet fixtures before closing the remaining data gates.

**Slice 4 audit update (2026-09-28):** verified the repository's Party Setup gap
against official Party, Config and Misc API documentation. Membership remains a
separate live observation; Party Setup is now a distinct read-only panel with editing
disabled because no supported field/writer/reload/readback contract exists. The
0–12 equipment split remains `adapter_lead_runtime_unverified`; the flat inventory
API does not establish indices or capacity semantics, and the interface warns that
equipment labeling/capacity may be wrong. Do not promote this mapping without
independent runtime/API evidence. Implemented authenticated Guild Storage record
removal with exact guild-name confirmation, exact server/guild scoping, atomic
database deletion, per-scope serialization against concurrent agent snapshots,
deleted counts and explicit future-observation retention behavior. It only removes
saved PhMon observations and does not mutate the game. Added focused handler/auth
tests and a PostgreSQL integration assertion, but the integration test skipped here
because `TEST_DATABASE_URL` is unset. Item API fields and live screenshots were
re-audited; packet movement retention, Greatest packet fixtures, full family/blue
semantics, live invalidation, and matching fresh-data screenshots remain open. Go
tests/vet, 54 plugin tests, frontend unit tests, typecheck, lint, formatting and
production build pass; lint has 14 existing void-element warnings. No deployment,
merge, push, or character operation was performed. Slice 4 remains incomplete.
Exact next action: obtain an authorized installed-phBot Party Setup write/reload/
readback contract and a documented or independent equipment-slot mapping; meanwhile
verify guild purge in a PostgreSQL-backed environment and capture authenticated
1440×1000, 1280×800 and 390×844 UI evidence when the local session is fresh. Then
continue Slice 4 evidence gates only; do not start Slice 5.

**Slice 3 acceptance follow-up (2026-09-27, active):** PR #9 base sync commit
`481df89` is mergeable and both duplicate CI runs passed: PostgreSQL integration and
Go race coverage, production-worker command smoke with fake adapters, authenticated
reconnect, outage/recovery and responsive `/api/live` browser checks. CodeRabbit's
review on the preceding `14e23f8a` head posted ten actionable findings; each was
independently verified and fixed in the current worktree, including the example secret
and outdated setup/protocol text, auth failure throttling, known-unsent command states,
offline controls, pre-limit history filtering, logout failure state and rejected
WebSocket handshake cleanup. The README warning outside the diff was also corrected.
The dotenv key-order suggestions were a non-actionable style preference and were
identified as such in the CodeRabbit replies. Local Go tests/vet, all 32 plugin tests, live
transport audit, Nuxt format/lint/typecheck and production build pass (lint has 13
existing self-closing warnings). PostgreSQL/race evidence for these latest code fixes
must come from CI because this Windows host has no local Docker/PostgreSQL and CGO is
disabled. No bot command was sent; Walk remains excluded by instruction, Clientless
remains unsupported on the live 20.1.1/plugin 1.1.0 runtime, and Execute Script is
outside the bounded catalog. On `fb76e09`, all tests and builds passed but both
validation CI runs failed only at the final Compose config check because that job did
not supply the now-required operator secret after `.env.example` was hardened. Commit
`d30e4a0` supplies a disposable CI-only value to validation; both validation and stack
jobs passed in runs `36332718330` and `36332721628`. This includes PostgreSQL integration
and Go race tests, production-worker fake-adapter command flow, reconnect/outage recovery
and browser `/api/live` checks. The latest-head CodeRabbit request was rate limited and
Copilot returned the account quota limit; neither posted new finding text. Exact next
action: request both automated reviews again when their quotas permit and inspect any
new comments. No command was sent to a bot character; Walk remains excluded,
Clientless remains unsupported on the live 20.1.1/plugin 1.1.0 runtime, and Execute
Script remains outside the bounded catalog. Keep PR draft and Slice 3
incomplete while real Walk, safe per-session Clientless and broad real-runtime
mutation evidence remain open; stop before Slice 4.
**Slice 3 implementation P0–P6 checkpoint (2026-09-27):** P0–P6 implementation is
present: separate operator-cookie auth; durable idempotent command admission/audit;
session/generation-targeted v3 dispatch; per-socket capability checks; callback-only
allowlisted plugin adapters with bounded queues; expiry/no-replay/unknown outcomes;
and command/control live snapshots through the existing `/api/live` connection.
Added the reference-shaped per-character Actions grid, session-selected Client view,
authenticated Nuxt command proxy and a fake-adapter end-to-end stack smoke wired into
CI. Clientless remains explicitly unsupported; Execute Script remains disabled because
the documented `start_script(str)` has no safe trusted catalog/list contract and raw
script bodies are outside this slice's command boundary. P7 local checks: `go vet ./...`,
`go test ./...`, Go builds, 30 plugin tests, Python compile checks, live transport
audit, Nuxt typecheck, lint (0 errors; 13 `vue/html-self-closing` warnings) and
production Nuxt build pass. `npm run format:check` passes repository-wide after
normalizing six previously unformatted files without behavior changes.
Local PostgreSQL integration tests need `TEST_DATABASE_URL`; it is absent in this
Windows worktree. Local Compose validation/container build cannot run here because
Docker is not installed, and `go test -race ./...` cannot run because CGO is disabled
and there is no C compiler. Separate disposable Compose worker/fake-adapter smoke was
previously recorded as passing. A later operator-authorized real same-value training
radius command on nuker1 completed and read back successfully; see the latest LAN
checkpoint below. Thus a single runtime path has evidence, not broad runtime parity.
Required responsive screenshots/browser matrix remains open. Keep Clientless,
Execute Script and untested mutations gated; stop before Slice 4.

**Slice 3 LAN deployment checkpoint (2026-09-27):** deployed to
`node@192.168.10.25:/var/www/phmon`; original `0.0.0.0` server bind and Compose's
all-interface web bind are preserved, PostgreSQL remains bound to loopback, and the
existing `.env` backup and named database volume were preserved. All three Compose
services report healthy; Go `/readyz`, Nuxt `/api/health`, and LAN-origin login
admission (invalid test secret returned expected 401 rather than origin rejection)
were checked. Because there is no HTTPS endpoint on this trusted LAN, added the
explicit `OPERATOR_ALLOW_INSECURE_HTTP` opt-in and exact `http://192.168.10.25:3005`
allowlist entry; this makes the session cookie unencrypted on the LAN. Focused Go
auth/config/HTTP API tests pass. Next: operator opens `http://192.168.10.25:3005`,
installs the plugin against `ws://192.168.10.25:8081/agent`, then run the simulator
smoke against a disposable database and separately record real phBot evidence only
when the operator has installed the plugin and authorized the named test character.
No bot character was operated during deployment.

**LAN dashboard live-path follow-up (2026-09-27):** browser verification exposed a
proxy bug: `coder/websocket` rejected the allowed browser Origin because its Host
was the internal `server:8081`. Go now skips only that duplicate library check after
the existing operator-session and exact-origin middleware succeeds. Added a live
regression test proving proxy-host/origin mismatch works while an unlisted origin
still receives 403. `go test ./internal/httpapi ./internal/auth ./internal/config`
passes. Deployed and verified through the authenticated browser: dashboard shows 2
online v3/v2 agents and 3 online characters; the agent table is populated. No
credentials were printed by diagnostics and no bot commands were sent.

**Slice 3 timestamp, persistence and acceptance follow-up (2026-09-27):** the LAN
plugin reconnect loop was traced to `hello.ack.server_time` using RFC3339Nano while
the embedded worker only accepted whole-second UTC RFC3339. The server now sends the
documented-compatible whole-second form. The disposable PostgreSQL/Compose command
smoke then exposed PostgreSQL 18 rejecting the untyped generation parameter in the
queued audit's `jsonb_build_object`; it is now explicitly cast to `bigint`. The
production Go error response was also leaking the Go 201 credential-creation status
as Nuxt 200; the proxy now preserves the upstream status. Durable command deadlines
retain nanosecond precision; Go emits whole-second UTC RFC3339 on the wire for
compatibility with deployed v3 plugins, while plugin 1.1.1 also parses fractional
timestamps without `datetime.fromisoformat`. A
disposable LAN Compose stack passed authenticated `/api/live` command smoke through
durable admission/audit, exactly one production-worker fake callback and an
`api_confirmed` result. Its temporary project, environment file, database and volume
were removed. No smoke touched the production database or a real bot action.

Deployed the server and web fixes to `node@192.168.10.25:/var/www/phmon`, rebuilding
only server/web; PostgreSQL container and named volume were not restarted or removed.
All three services are healthy, two protocol-v3 agents reconnect, four character
sessions are live, and the LAN browser shows current data plus enabled supported
actions. A live training-area readback is present. The running profiles still report
plugin 1.1.0. The current server now emits whole-second wire expiry timestamps so
these v3 profiles can parse command frames without extending their durable server
deadline; no real command was sent.

Checks on this follow-up: `go test ./...`, `go vet ./...`, Go server/phmonctl builds,
30 Python plugin tests, Python compile, live transport audit, Prettier, Nuxt
typecheck/lint/build and the disposable Compose command smoke passed. Lint has 13
existing `vue/html-self-closing` warnings and zero errors. Slice 3 is **not complete**:
no named real-character mutation/effect tests are authorized; the official clientless
mutation remains unsupported; Execute
Script remains outside the bounded command contract; PostgreSQL concurrency/race
tests, responsive reference screenshots at all required viewports and final
direct/browser regression flows remain open. Exact next action: finish remaining
authorized fixture/browser checks and document named-character runtime effects only
if explicit authorization is provided. Do not submit a production command or
continue into Slice 4.

**Slice 3 implementation P2 (2026-09-27):** command admission now has a
typed server-owned catalog, migration `000004_commands.sql`, durable command/audit
records, idempotency hashing, one-in-flight-per-character enforcement, fixed-window
bounded admission limits, current character/session fencing, same-region walk and
training-area region validation, disruptive-action confirmation and an authenticated
`POST /api/commands` returning HTTP 202 only for durable acceptance. Later P3–P7
status and exact remaining gates are recorded in the latest checkpoint above.

**Slice 3 implementation P1 (2026-09-27):** operator authentication is
implemented on the Slice 3 branch. Go owns bounded hashed opaque sessions, an
eight-hour absolute expiry, login throttling, strict named cookies, configured
Origin validation and live-socket revocation. Production router wiring protects
browser monitoring, credential creation and existing mutations while leaving
`/agent` on its separate bearer-token boundary. Nuxt gates feature pages behind
sign-in and forwards only the named operator cookie plus Origin to private Go
routes and the live WebSocket. Loopback HTTP is explicit development-only; HTTPS
cookies remain Secure. Next package: P2 command domain/schema/HTTP admission.

**Slice 3 implementation P0 (2026-09-27):** implementation is now authorized on
`codex/slice-3-plan`. The Slice 3 command/auth contract is frozen in
`docs/protocol.md` and `docs/phbot-capabilities.md`: protocol v3 commands with v2
monitoring compatibility, explicit character+session targets, per-socket capability
ownership, one in-flight command per character, durable no-replay lifecycle,
same-region walking, bounded typed arguments, separate operator-cookie authentication,
and honest API-result versus observed-state semantics. Public phBot docs were
rechecked; core bot/trace/training/walk/return/disconnect primitives remain
documented. No safe public clientless mutation is verified, so that capability
remains blocked without blocking independent Slice 3 work. Next package: P1 operator
authentication.

**Slice 3 planning handoff (2026-09-27):** the operator requested a detailed plan
for GPT-6 Luna, not implementation. Added
[`docs/slice-3-implementation-plan.md`](docs/slice-3-implementation-plan.md), with
sequential work packages, operator authentication, session-targeted command
lifecycle, capability/API evidence, persistence, callback dispatch, WebSocket UI,
tests and a scoped implementation prompt. Reviewed `phmonitor_screenshots/`
Actions/Client and adjacent Stats captures; the plan uses the current Nuxt
pages/components structure. Official docs confirm core primitives; a safe
clientless mutation remains unverified. Validation in this planning turn is
document/source/layout inspection only; no implementation tests or real bot
actions were run. Slice 3 remains not started. Next action, when implementation
is requested: execute P0 of the plan, then continue its Slice 3 packages and
acceptance gates without starting later slices.

**PR #7 CodeRabbit follow-up (2026-09-27):** verified and fixed three functional
findings. Credential creation disables dismissal and client route navigation until
the one-time response settles; dismissal then clears the token. Empty stale or
reconnecting agent lists display an unavailable state with an enabled refresh
button. Group creation trims names and ignores blank input (the backend already
rejects blank names). Typecheck, production build, formatting and transport audit
passed; lint retains its three existing input warnings. Isolated browser fixtures
verified pending/success/failure credential states, navigation blocking, dismissal,
stale empty agents with retry, and blank/trimmed group request behavior. No real
credentials or bot actions were used. Next action: push fixes, reply/resolve the
three review threads, request another review, and wait for CI and CodeRabbit.

**Nuxt structure refactor (2026-09-27):** completed on
`codex/nuxt-pages-components` within the operator's explicit refactor-only scope.
`web/app/app.vue` is now the provider/layout/page outlet. The persistent shell lives
in `layouts/default.vue`; Dashboard, Stats and character detail use `pages/index.vue`,
`pages/stats.vue` and `pages/characters/[id].vue`. Focused components own navigation,
top summary, mobile access, dashboard panels, character/group controls, agents,
one-time credentials, operations and page headers. Shared fleet calculations,
health monitoring and pure formatters have dedicated composables/utilities.
Internal links use Nuxt routing; page list/detail subscriptions are removed on
unmount while the shell keeps the shared WebSocket alive. Health polling is owned
by the layout and its lifecycle hooks are registered synchronously.

Validation: frontend typecheck, production build, formatting and live-transport
source audit passed; lint passed with the same three pre-existing self-closing-input
warnings. An isolated WebSocket fixture exercised direct Dashboard/Stats/detail
loads, client navigation, browser back/forward, search and subscription disposal;
the navigation sequence retained one socket and the same shell DOM. Browser checks
also covered mobile menu closing on navigation, credential panel opening and QR
dialog Escape/focus restoration. No page errors were reported. Screenshots at
1440×1000, 1280×800 and 390×844 are under ignored `exports/nuxt-refactor/`; the mobile
document width stayed 390 px. This is fixture-based refactor evidence, not new
phBot/backend integration validation or closure of the reference-parity gates.
No roadmap slice was advanced. Next action: review this branch; resume outstanding
Slice 2/2.5 work only under its existing scope. Do not start Slice 3.

**Slice 0: complete. Slice 1: implementation and automated validation complete;
operator has manually verified basic real phBot → PhMon connectivity. Slice 2: in
progress. Slices 3–15: not started.** Slice 1's detailed runtime/profile/API and
same-viewport visual gates remain open; basic connectivity must not be described as
blocked or as proof of all runtime APIs. See `docs/phbot-capabilities.md`.

**Live-data WebSocket transport enforcement (2026-09-27): complete.** The
mandatory Live-data transport contract was recorded above before implementation. Go
`/api/live`, the same-origin Nitro WebSocket relay, versioned subscriptions/revisions,
the shared typed browser connection, stale/reconnect behavior, bounded backpressure and
database recovery synchronization are implemented without changing the plugin protocol
or database schema. Existing HTTP read endpoints remain diagnostic compatibility only;
HTTP mutations do not refresh live state.

Validation completed on code head `9c729c57679b5c38621044cff68308ef50f5e577`
(GitHub Actions run `36278699555`): both `validate` and `stack` passed. Evidence
includes PostgreSQL-backed Go race/integration tests, plugin tests, the source transport
audit, frontend format/lint/typecheck/production build, production and Nuxt-development
`/api/live` smoke coverage, filtered/detail/revision/cross-client group scenarios,
simulator character switching and multiple-socket lifecycle coverage, malformed-frame/
slow-consumer/cross-origin tests, backend restart/reconnect, PostgreSQL outage/recovery
on the same browser-facing WebSocket, and a browser network audit showing zero HTTP/SSE
live-data reads through startup, filtering, manual refresh, mutations and recovery.
Responsive checks at 390×844 and 1440×1000 also passed. Future map positions/live
layers remain outside this change and are bound by the same WebSocket-only contract.

**Previous turn (2026-09-26): Slice 2 Dashboard/header verification.** Scope is the
user-requested visual follow-up; no agent protocol, schema or phBot behavior changed.
The Dashboard and Stats routes are now distinct, the top strip displays observed
character counts/vitals/gold, the dashboard follows the reference's asymmetric card
layout, and unimplemented dashboard/sidebar areas are labeled `LATER`. The sidebar
shows an instance QR and copy-link control. Local Nuxt typecheck/build passed, lint
passed with three existing input warnings, and formatting passed for changed files.
The local browser at 1264 × 710 verified Dashboard/Stats navigation; its backend was
unavailable, and the 1440 × 1000 populated-data comparison remains open. The LAN
browser's older build could not be rebuilt because Docker is unavailable here. See
`docs/reference-parity.md`. Do not start Slice 3.

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
and had no horizontal overflow at CSS viewports 1440 x 1000, 1280 x 800 and 390 x 844. Screenshot/preview outputs are ignored under `exports/`; extracted Nuxt assets
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

Named `set_training_area(name)` selects an existing area; it does not create a new
one on the tested phBot 20.1.2 runtime. An unknown unique name returned `False` with
unchanged area readback on Greatest/nuker1 (2026-09-28). phBot's documented Add
action is in its own UI; do not present PhMon's named selection as area creation or
edit phBot configuration files as a substitute for an unverified creation API.
See `docs/phbot-capabilities.md` for the bounded live test.

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

This slice is also the first major visual reconstruction of the phMonitor Stats
experience. Treat the current Slice 2 `/stats` composition, `CharacterPanel` and
character-detail presentation as development scaffolding rather than a permanent
page structure. Preserve their working data flows, character identity, grouping,
search and targeting behavior, but do not preserve their generic panel/table layout
when it conflicts with the supplied reference.

By the end of Slice 4, the populated Stats experience must be recognizably derived
from `phmonitor_screenshots/02-stats-01.png` through `02-stats-04.png` for all
functionality whose owning slices have been implemented. Do not merely append new
inventory, pet and party panels underneath the existing Slice 2 UI.

**Reference behavior and visual contract (phMonitor v0.5.0):**

The five supplied Stats screenshots are one connected character-management
experience whose functionality spans several slices. Slice 4 owns the first major
page-level integration pass and must use those captures as its visual baseline:

- `02-stats-01.png` establishes the grouped-character summary hierarchy, compact
  character statistics and location/minimap placement. Reuse the real Slice 2
  character/group state now. Actual map-coordinate rendering remains owned by
  Slice 7; do not fake a correctly positioned minimap before its transform is
  validated.
- `02-stats-02.png` establishes the expanded character-centric presentation:
  character identity/art, live statistics, pet information, location panel and
  compact resource/status cards. Slice 4 must implement the pet-related and
  character-layout portions that are supported by real data. Slice 7 later makes
  the map portion authoritative.
- `02-stats-03.png` is directly owned by this slice: equipment/character-set items
  arranged as a visual equipment surface around the character presentation, with
  item icons, empty equipment positions and observed plus/quantity overlays.
  A generic equipment table does not satisfy this reference.
- `02-stats-04.png` establishes the visual treatment for character progress and
  training/status information. Existing Slice 2 live values such as level, XP, SP,
  HP/MP and gold should be integrated into this character-centric composition now
  where available. Derived historical/rate metrics whose source does not yet exist
  remain owned by Slice 12 and must stay explicitly unavailable rather than being
  fabricated.
- `02-stats-05.png` primarily belongs to Slice 3. When Slice 3 is present, Slice 4
  must integrate its existing capability-aware command controls into the same
  character-targeted Stats/detail experience instead of creating a second command
  implementation or leaving the commands isolated in a generic operations panel.

Slice ownership controls data/functionality, not whether the page may already adopt
the reference composition. Build the reference layout progressively using everything
that is genuinely available from completed slices, and leave only genuinely
future-owned data unavailable.

Do not wait for Slice 15 to introduce the Stats visual direction. Slice 15 is the
cross-product parity and acceptance pass; it may reconcile spacing, responsive
behavior, remaining controls and incomplete reference details, but Slice 4 must
already establish the character-centric Stats structure.

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
resolved by stable item code from the shared SRO catalog; the active Slice 2.5 profile
maps numeric model IDs and can provide version-specific overrides. Show the source-
provided stack/quantity value when applicable. Preserve actual empty slots. A plus
value or mutable status may affect the compact label/accent only when observed;
static rarity/seal may come from the verified item definition. Never derive item
quality from icon color alone. Missing icons use one deliberate placeholder while
retaining the item's name and identity.

Each occupied item slot exposes one shared **Silkroad-style item detail card** used by
bag inventory, equipped/character-set items, personal storage, guild storage and pet
inventory. Show it on hover or keyboard focus; do not add click-to-pin or click-to-open
behavior. Keyboard users must be able to focus slots and inspect the detail surface.
Long item details scroll inside the popover/drawer/dialog instead of overflowing the
page.

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
  and base/reference properties may be enriched from the shared SRO item definition
  by stable item code; the active Slice 2.5 profile maps numeric model IDs and
  supplies version-specific overrides when needed. Item-instance state such as plus,
  quantity, observed blues, current durability and other mutable values remains
  sourced from the live/historical observation
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
`Storage`, `Guild Storage`, the applicable pet, or `Job Pouch` where verified),
observer and freshness/last observed state around the collection/detail surface. Do not
inject those PhMon operational fields into the middle of the Silkroad stat block.
Last-known storage, guild-storage, pet or job-pouch data must be clearly marked
stale/last observed rather than visually indistinguishable from currently opened/live
state.

Treat these as **canonical item containers**, not unrelated ad-hoc payloads. Each
container observation must carry enough stable source/container/slot/item identity to
support Slice 5 acquisition/transfer reasoning without confusing a slot move with a new
item. At minimum investigate and model, where the verified phBot API exposes them:

- character bag inventory
- equipped/character-set items
- personal storage
- guild storage
- each applicable pet inventory, including Pick/Grab pets
- job pouch
- any additional verified item-bearing container discovered during capability review

A container becoming observable after plugin load, pet summon, storage open, reconnect
or refresh establishes current/last-known state; it does **not** by itself mean every
visible item was newly acquired.

**Implement:**

Stats/detail visual integration:

- restructure `/stats` and `/characters/{character_id}` as necessary around the
  supplied Stats reference instead of treating the current Slice 2 component
  composition as fixed architecture
- retain the existing stable `character_id`, search, group membership, live state
  and command-targeting behavior while changing presentation
- provide a clear grouped-character -> selected/expanded-character drill-down model
  matching the reference information hierarchy
- integrate already-implemented Slice 2 live statistics into the new character
  presentation rather than duplicating or replacing their backend sources
- integrate Slice 3 controls through its existing command/capability/result lifecycle
  when available; do not introduce frontend-owned command semantics
- use Slice 2.5 game assets only when their semantic mapping is verified. Item icons
  and validated static reference data may be used directly; unresolved character
  portraits, pet roles or other uncertain mappings must use a deliberate fallback
  rather than guessed associations
- remove `LATER`/temporary scaffolding for functionality that is actually available
  by the end of Slice 4. Keep future-slice gaps explicit only where the underlying
  capability genuinely remains unavailable
- do not create duplicate desktop-only and mobile-only information models; responsive
  layouts must expose the same character/item/pet/party state

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

**Opcode boundary and deferral:** Slice 4 uses documented `get_inventory()`,
`get_storage()`, `get_guild_storage()`, `get_job_pouch()`, `get_pets()`, `get_party()`
and `get_academy()` observations first. Passive item-instance enrichment from
`handle_joymax` / `handle_silkroad` is permitted only for a version-verified generic
vSRO 1.188 structure, with bounded decoding and exact session/container/slot binding.
It must never inject packets or infer fields from arbitrary byte offsets. The generic
packet callbacks and `inject_joymax` / `inject_silkroad` functions do not by themselves
prove item, pet or party semantics. Keep enrichment disabled until published field
layouts, captured fixtures and stale/mismatch handling are verified and recorded in
`docs/phbot-capabilities.md`. Broader pet/party packet decoding or packet-based
actions remain deferred beyond Slice 15; any later support must record the exact
target version, direction, semantics, runtime evidence and limitations, route
mutations through audited commands and verify effects. Unresolved required fields
remain explicit acceptance gaps.

delta/change handling keyed by stable source/container + slot/item identity where
appropriate; do not churn/re-render the entire collection for one changed stack or
slot if the protocol can safely communicate a bounded update. Preserve enough
pre/post-state for Slice 5 to distinguish quantity gain, quantity loss, slot movement,
stack split/merge and cross-container transfer without treating all changes as new
acquisitions

Nuxt inventory/equipment/storage views with slot-preserving icon collections,
source/freshness indicators and responsive item detail surfaces

Nuxt pet view grouped by supported pet category, with applicable pet inventory

Nuxt party view with current-membership and Party Setup sections

**Acceptance criteria:**

at the end of Slice 4, `/stats` is no longer primarily the generic Slice 2
`CharacterPanel` + `AgentPanel` + `OperationsPanels` development composition; its
main character experience is recognizably based on the supplied phMonitor Stats
captures

`02-stats-02.png` and `02-stats-03.png` are used as explicit same-viewport comparison
targets for the character/equipment/pet portions implemented in this slice, with
differences documented rather than silently deferred

the reference-style character hierarchy incorporates existing Slice 2 identity,
online state, level, HP/MP, XP/SP, gold and location information wherever actually
observed, without creating a second source of truth

when Slice 3 is already implemented, its supported character actions appear naturally
inside the character-focused Stats/detail flow and retain exactly the same audited
backend command lifecycle

missing Slice 7 map transforms or Slice 12 historical/rate data do not block the
rest of the reference layout from being implemented; those specific areas remain
honestly unavailable without forcing the entire Stats page to remain scaffolding

verified Slice 2.5 assets are used where appropriate, while unresolved portrait,
pet-role, map-transform or item semantics are never guessed merely to make the
screenshot look populated

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

current inventory, equipment/character set, applicable pet inventories, job pouch and
available storage sources are visible without conflating ownership/source;
stale/last-known storage/pet/job-pouch observations are visibly distinct from
current/live observations

Attack/Fellow/Pick/Transport pets are represented when observed, and applicable pet
state/inventory remains associated with the correct pet across updates

party membership is visible independently from Party Setup configuration

Party Setup loads the plugin's current supported values, applies verified edits through
an audited command, reports pending/success/failure, then refreshes to prove the
effective runtime configuration

desktop hover and keyboard focus can inspect the same item information. Keep the
mobile item layout responsive and the items accessible without adding click-to-pin or
click-to-open behavior; do not claim an unverified tap-preview interaction. Verify
keyboard/focus handling and no page-level overflow at the project's target viewports

updates do not require blindly resending excessive full state when unnecessary

### Slice 5 — Event pipeline

**Objective:**

Build the canonical, extensible activity pipeline for every discrete occurrence observed
directly by phBot, derived reliably from monitored state, or decoded from a verified
Silkroad packet. Persist those occurrences durably so Timeline, Chat, Conditions,
Notifications, Analytics, Map and Economy can reuse the same source of truth instead
of creating parallel ingestion models.

Slice 5 owns **ingestion, normalization, delivery, durability and generic event
querying**. Feature-specific projections and workflows may live in later slices, but
they must consume this canonical pipeline when the underlying fact is an event.

**Event model:**

Define a versioned event envelope with, at minimum:

- stable `event_id`
- `schema_version`
- canonical `kind` and broader `category`
- `agent_id`, `character_id`, `server_id` and connection/session identity where known
- per-session monotonic `sequence` where the source can provide it
- `occurred_at` and backend `received_at`
- explicit provenance via `source` plus bounded `source_ref`/decoder metadata
- optional normalized position/region context where it was observed at event time
- bounded typed payload
- optional deterministic `dedupe_key` when the source cannot carry the same event ID
  across retries

Supported provenance values should distinguish at least:

- direct phBot event/callback
- phBot chat callback
- phBot alchemy callback
- reliable state-diff derivation
- verified Joymax/Silkroad packet decoder
- backend Condition/custom event
- backend/system event

Do not flatten every event into an unstructured JSON blob. Keep common searchable
identity/time/source fields first-class while using bounded typed payloads for
event-family detail. Unknown source fields remain absent/null; never synthesize facts
from display text or presentation.

**Direct phBot events/callbacks to ingest where verified:**

Cover the complete useful documented `handle_event` catalog, not only the subset
currently visible in the phMonitor marketing page. At minimum investigate, document
and implement supported events for:

- character death
- normal item drop
- **rare item drop as its own canonical event**, preserving phBot's separate rare-drop
  signal instead of trying to infer rarity from a normal-drop row
- unique spawn
- hunter/trader spawn
- thief spawn
- transport death
- another player attacking the character
- GM nearby/spawned
- character level-up
- alchemy completion/result
- other useful documented `handle_event` values discovered during implementation
- connection, disconnection, joined-game, reconnect/recovery and teleport callbacks

Verify exact callback/event IDs, argument meanings and runtime behavior against the
official phBot plugin documentation and the installed runtime before coding them.
Document the verified mapping in `docs/phbot-capabilities.md`; do not copy guessed
constants from third-party snippets.

Also ingest dedicated callbacks where they carry richer semantics than
`handle_event`, especially alchemy callbacks/results and chat. Correlate duplicate
signals deterministically rather than storing two independent copies of the same
occurrence.

**Canonical event families should include where supported:**

- `session.connected`
- `session.disconnected`
- `session.joined_game`
- `session.teleported`
- `character.died`
- `character.level_up`
- `character.attacked`
- `drop.item`
- `drop.rare`
- `world.unique_spawned`
- `world.gm_spawned`
- `job.hunter_trader_seen`
- `job.thief_seen`
- `pet.transport_died`
- `alchemy.finished`
- `chat.message_received`
- reliable party/academy/quest/pet lifecycle events derived from bounded state diffs
  where no direct callback exists
- bounded `custom.*` events emitted intentionally by later Conditions/backend logic

Names may be refined while implementing, but keep one stable canonical naming scheme
and migration/version rules. Do not create separate timeline-only names for the same
fact.

**Chat boundary:**

Inbound chat is an event source and therefore enters through Slice 5. Normalize every
supported incoming chat message into the canonical pipeline with its verified channel,
sender/recipient context, character/server scope, timestamp and bounded original
message content.

Slice 6 owns the chat-specific persistence/query projection if needed, conversation
model, unread/navigation behavior, composer and outbound sending. It must consume the
Slice 5 event instead of inventing a second plugin -> backend ingestion path.

**Reliable derived events:**

A state transition may become a canonical event when phBot exposes trustworthy current
state but no direct callback. Candidate examples include:

- party member joined/left
- academy member joined/left/graduated or other verified membership/state changes
- quest accepted/completed/removed where the available API can distinguish them
  reliably
- pet summoned/dismissed or other lifecycle changes where identity is stable enough

Use bounded, identity-aware diffs over authoritative snapshots. Startup/reconnect state
must not be misreported as a burst of historical joins/leaves. Record provenance as
derived state and test reconnect/reload behavior. If the source cannot distinguish an
actual transition from missing/stale data, do not emit the event.

**Verified packet-derived events:**

phBot exposes raw Joymax/Silkroad packet hooks. Use them only as an extension mechanism
for valuable events that cannot be represented correctly from documented callbacks or
state APIs.

- maintain an explicit opcode/decoder allowlist
- bind each decoder to the verified game/server/protocol assumptions it supports
- unit-test decoders with captured/fixture packets whose provenance is documented
- emit a normal canonical event after decoding; downstream code must not depend on raw
  packet layout
- retain only bounded decoder/source metadata needed for debugging
- do not build an indiscriminate packet logger or persist all raw traffic
- unknown/unverified opcodes remain unsupported instead of being guessed

Prefer direct phBot callbacks over packet parsing whenever both provide the same fact.

**Item/drop event snapshots:**

Item events must retain every actually observed display/detail field needed by later
UI and analytics without re-querying mutable inventory state:

- canonical item identity/model/code where available
- display name
- plus value
- quantity/stack where relevant
- rarity/seal metadata actually observed
- degree/category/type taxonomy when verified
- observed item color/grade
- observed blues/attributes
- ground-drop identity and coordinates when the event can be reliably correlated with
  a current phBot drop observation

Fields absent from the source remain absent. Never infer a seal, blue, rarity,
probability or item property from presentation alone. A rare-drop callback is evidence
that the occurrence was a rare drop; it is not permission to invent missing item
instance metadata.

**Item acquisition, transfer and container-delta events:**

A world drop and an owned-item acquisition are different facts. `drop.item` /
`drop.rare` answer "what appeared as a drop"; they do not prove that this character,
party member or pet received the item. Model actual possession changes separately.

Add canonical item event kinds where the verified source supports them, including:

- `item.acquired` when an observed owned container gains quantity that was not already
  present in another known container for the same owner/session
- `item.transferred` for a reliably correlated movement between known containers,
  such as Pick-pet -> character bag or bag -> storage
- `item.quantity_increased` / `item.quantity_decreased` when stack deltas are useful
  and cannot yet be classified more specifically
- optional more specific acquisition methods such as `party_distribution`,
  `pet_pickup`, `ground_pickup`, `quest_reward`, `purchase`, `alchemy_output`
  or similar **only when the callback/packet/state source actually proves that cause**

Every acquisition/transfer payload should retain, where known:

- canonical item identity and observed item snapshot
- quantity delta
- destination container type/identity and slot
- source container type/identity and slot for transfers
- owner/character/server/session
- acquisition/transfer method and provenance only when proven
- correlation IDs to related `drop.*`, packet or command events when reliable

The key rule is: **inventory appearance proves possession, not provenance**. A bag or
pet inventory delta may prove that an item was gained, while the reason remains
`unknown`. Do not label a gain as party distribution, Pick-pet pickup, monster drop,
purchase or another cause merely because it is plausible.

Party item distribution needs special care. Current party state/configuration is not
proof of who received a specific drop. If a verified Silkroad/phBot callback or packet
identifies the allocation recipient, preserve that as acquisition provenance and
correlate it with the receiving container delta. Otherwise emit the reliable
`item.acquired` fact with unknown acquisition method.

Pick/Grab pets and other item-bearing pets are first-class owned containers. An item
newly observed in a pet inventory may produce `item.acquired`; a later move from that
pet into the character bag is `item.transferred`, not a second acquisition. Apply the
same principle to job pouch and other verified containers.

Container-delta reconciliation must explicitly avoid false acquisitions:

- initial inventory/pet/job-pouch/storage snapshots after startup/reconnect/open/summon
  establish baseline state and do not emit acquisition events for existing contents
- inventory sorting or slot reordering does not create acquisition/transfer events
- stack split/merge does not change total owned quantity and is not an acquisition
- quantity increase emits only the positive delta, not the whole resulting stack
- storage becoming newly observable is not acquisition
- pet summon/dismiss visibility changes are not acquisitions
- a cross-container move must not be counted as both a loss and a new acquisition when
  it can be correlated reliably
- a rare/normal `drop.*` event and a later `item.acquired` event remain two distinct
  facts and may be correlated; neither should be collapsed into the other

Use bounded correlation windows and stable item/container identity. When correlation is
ambiguous, preserve the separate observed facts rather than inventing a transfer or
cause.

**Delivery and durability:**

phBot callbacks must stay fast and nonblocking. Event publishing therefore uses a
bounded asynchronous queue plus a bounded crash/reconnect-resistant local spool for
events not yet durably acknowledged by the backend.

Implement at-least-once transport with idempotent backend persistence:

1. assign stable event identity and sequence before enqueue
2. enqueue/spool without waiting on backend I/O in the phBot callback
3. send ordered batches through the existing authenticated agent connection
4. acknowledge only after durable backend persistence
5. replay unacknowledged events after reconnect/plugin/backend restart
6. enforce backend uniqueness/idempotency so retries do not duplicate history

Define spool bounds and overflow behavior explicitly. High-volume/noncritical event
families may use tighter retention or batching, but rare drops, deaths, alchemy
results and other important discrete events must not silently disappear merely because
the backend was briefly unavailable. Slice 14 may harden tuning/retention further; the
basic reliable contract belongs here.

**Backend/query/UI work:**

Implement:

- durable PostgreSQL event storage and indexes for server/character/kind/time queries
- idempotent batch ingestion and acknowledgements
- generic cursor-based event querying with deterministic ordering
- server/character/date/event-family/event-kind filtering
- item-aware filters where canonical item identity is present
- bounded pagination/count behavior suitable for long-running self-hosted instances
- Nuxt activity timeline matching the reference Events/History direction
- specialized filters/views for deaths, normal drops, rare drops, uniques, level-ups,
  alchemy and custom events where applicable
- map/detail links when an event has validated coordinates or canonical item context

Chat events may be hidden from the generic activity timeline by default to prevent
noise, but they remain part of the same ingestion/durability architecture and are
queryable for Slice 6.

**Acceptance criteria:**

- direct phBot event mapping is documented and covered by focused tests/fixtures
- rare drops and normal drops remain distinct canonical event kinds
- inbound chat reaches durable backend storage through the same Slice 5 ingestion
  contract consumed by Slice 6
- derived events do not create false transitions during startup/reconnect or stale
  snapshots
- any packet-derived event has an explicit verified decoder/version boundary and tests;
  there is no generic raw-packet persistence path
- events survive page reload, plugin reconnect and backend restart within the
  documented spool/retention contract
- retry/replay is idempotent: one real occurrence renders as one durable event
- timeline ordering remains deterministic across reconnects and batched delivery
- rare/normal drop rows and detail links render stored observed item semantics without
  re-querying mutable current inventory state
- acquisition tests cover character bag, Pick/Grab-pet inventory and job pouch where
  supported, including positive stack deltas
- startup/reconnect/storage-open/pet-summon baselines, slot sorting and stack split/merge
  do not generate false acquisitions
- a pet -> bag or other reliably correlated cross-container move is stored as a
  transfer rather than a second acquisition
- party-distribution provenance is attached only when a verified callback/packet/source
  proves the recipient; otherwise the acquisition method remains unknown
- a related `drop.*` and `item.acquired` can coexist and be correlated without being
  deduplicated into one semantic event
- event ingestion never blocks normal phBot behavior
- later slices can subscribe/query the canonical stream without creating a second event
  transport or history table for the same occurrence

### Slice 6 — Chat

**Objective:**

Provide phMonitor-style remote chat visibility and sending on top of the canonical
Slice 5 event pipeline.

Slice 6 owns chat-specific history/projections, conversation semantics, navigation,
notifications/preferences and outbound messaging. It does **not** introduce a second
plugin-to-backend inbound transport: incoming messages originate from the normalized
`chat.message_received` events produced by Slice 5.

**Inbound chat:**

Support every verified phBot chat channel that maps cleanly to a user-visible
conversation. At minimum investigate and implement where supported:

- general/local
- private
- party
- guild
- union
- global
- other useful documented channels/types, with their raw verified chat type retained
  as bounded source metadata when necessary

Normalize each inbound message with:

- stable message/event identity
- server and observed character
- canonical channel/type
- sender name/identity as exposed by phBot
- recipient/private peer when the source provides it
- original bounded message text
- occurred/received timestamps
- inbound direction
- optional canonical item/entity references only when they can be resolved reliably
  without rewriting the original message

Preserve enough original channel/source metadata for later Economy parsing or other
projections, but do not classify arbitrary chat as a trade offer in this slice.

**Chat history and projections:**

Build the chat read model from canonical events rather than mutating event history.

Implement:

- persistent history with cursor pagination in both directions
- server-scoped General/Global history for the selected server; General read state
  applies to every character on that server, and Global has no unread counter
- per-character scope for Private, Party, Guild and Union conversations
- channel tabs matching the reference: General, Private, Party, Guild, Union and Global
  where supported
- stable private-conversation identity/contact list
- unread/read state and jump-to-latest behavior where useful
- deterministic ordering when inbound events arrive late after reconnect
- retention behavior that is explicit and does not silently diverge from the canonical
  source event
- deduplication/correlation for outbound messages that are subsequently observed again
  through phBot, so one sent message does not render twice

The event record remains the durable occurrence. A specialized chat table/index is
allowed as a projection for efficient conversation queries, but it must be rebuildable
or traceable to canonical event/message identity rather than becoming competing truth.

**Outbound chat:**

Implement remote sending only through the authenticated, capability-aware command
lifecycle established in Slice 3.

For each supported outbound channel:

- validate sender character/server target
- validate channel-specific recipient/arguments
- enforce message length/encoding limits verified from phBot/runtime behavior
- return explicit pending/sent/rejected/failed/timeout state
- correlate successful sends with any later observed chat callback
- require explicit confirmation for costly/global sends when applicable
- never expose arbitrary Python, packet injection or unrestricted opcode sending as a
  chat feature

If a channel is visible inbound but cannot be sent through a verified supported phBot
API, keep it read-only and report that capability honestly.

**Reference-style Nuxt UI:**

Reproduce the verified phMonitor chat structure rather than a generic log viewer:

- sender/character selector
- channel tabs
- private contacts/new-conversation flow
- recipient field where needed
- conversation/history pane
- jump-to-latest affordance
- bottom composer with send state/error feedback
- responsive contact/conversation navigation for narrow screens
- loading, empty, disconnected/stale and recovered states
- persisted chat preferences from Settings where applicable

Support emoji and canonical item references/details where the verified source and
reference behavior allow them. Do not fabricate rich item links from unverified text
parsing. Item references that become canonical should reuse the shared item-detail
semantics from Slices 4/5/13.

**Notifications/integration boundary:**

Message sound/browser/Discord preferences may subscribe to canonical chat events, but
notification delivery/configuration follows the shared notification/Condition
architecture. Do not bury notification side effects directly inside the phBot chat
callback.

Slice 13 may derive Economy/global-offer records from preserved chat data when the
format/source can be verified. Slice 6 must preserve the source material needed for
that work without claiming every trade-looking message is structured economy data.

**Acceptance criteria:**

- supported inbound messages appear in the web UI after passing through Slice 5 and
  remain available after reload/backend restart according to retention
- messages are attributed to the correct server, character, channel, sender and private
  peer where those fields are available
- General/Private/Party/Guild/Union/Global navigation matches supported source
  capabilities and does not show writable controls for unsupported outbound channels
- private conversations have stable identity and paginated history
- reconnect/replayed events do not duplicate visible messages
- supported outbound chat can be sent remotely through the normal command lifecycle
  with visible success/failure state
- outbound messages that are echoed back by phBot are correlated instead of duplicated
- costly/global sends require the documented confirmation behavior where applicable
- the chat UI remains usable at the project's desktop and mobile target viewports
- Slice 6 does not maintain a second inbound transport or contradictory source of truth

### Slice 7 — Live map

**Objective:**

Show actual live game-world state without video capture.

**Map implementation decision (2026-09-28):** Use Leaflet inside a client-only Nuxt
component with `CRS.Simple` for the locally served, versioned Silkroad raster map.
Nuxt owns selectors, dialogs, status and command feedback; Leaflet owns tile loading,
pan/zoom, markers and pointer interaction. Adapt the exported tile grid through a
local tile/grid layer rather than assuming a geographic XYZ service. Use Leaflet's
Canvas renderer or a custom canvas/grid layer for dense observations and heatmaps;
do not build a separate canvas pan/zoom engine for the base map.

Keep game-coordinate conversion in a tested, dataset-versioned adapter independent
of Leaflet. A tile match alone does not validate the position within that tile.
Calibrate outdoor and each required special-area map separately, including axis
direction, tile origin/scale, region boundaries and the Z value needed by commands.
Where conversion or Z is unverified, show the map for inspection but disable
coordinate-based actions instead of sending guessed destinations.

**Cave-floor interaction contract (reference rechecked 2026-09-28):** Quick
navigation to a cave opens a dedicated map view with a compact floor bar anchored
inside the bottom of the map viewport: Back to world map, the cave name, and
floor-specific buttons with the active floor highlighted. Donwhang Stone Cave has
1F–4F; Tomb of Qin-Shi has B1–B6; Job Temple has 1F, 2F and Annex 1–5. Choosing
another floor replaces that floor's map imagery and resets to its floor-specific
view; Back to world map removes the bar and restores the outdoor view. Model caves,
floors, labels, tiles and view presets in the versioned map profile instead of
hardcoding one shared floor list or using the outdoor tile transform for interiors.
Use the same floor-navigation component for other caves when their profile supplies
validated floor metadata. Keep markers, event overlays and density layers scoped to
the selected floor.
The reference displays X/Y, tile and zoom while viewing a cave; update these with
the active view and floor, preserving signed X/Y values rather than clamping cave
coordinates to the outdoor range. Its observed cave selection X/Y values are
recorded in `docs/reference-parity.md`; they are viewport readouts, not proof of a phBot
region/X/Y/Z conversion. Confirm the selected floor and validated region/coordinates
before enabling a map-issued walk or training-area command.

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

floor-aware cave navigation, active-floor selection and Back to world map

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

Right-click on a validated map point opens actions to navigate the selected character
there or set that point as its active training-area center. Provide an equivalent
selected-point menu for touch and keyboard users; show the resolved server, region
and coordinates before confirmation. Draw a preview marker and the observed training
radius when available. Map navigation uses a distinct typed `character.navigate`
destination command with authentication, capability checks, session scope and audit.
The documented phBot `generate_script(region,x,y,z)` returns script lines that may include
walking, waits and teleports; `start_script(str)` accepts multiline script text.
For map navigation, investigate and validate this generated-script route rather than
replaying `generate_path(x,y)` waypoints as the general navigation mechanism. The
plugin must generate the script locally for the explicit target, bound and validate
its returned lines before execution, and never accept arbitrary browser-provided
script text. Verify supported-runtime behavior, interactions with an existing script,
cancellation and arrival evidence before enabling the action. Starting a background
script is not proof of arrival; report unknown when completion cannot be verified.
Keep the existing same-region `character.walk` command distinct during migration;
it currently uses `generate_path` and has different limits. Training-position changes
use the existing `training.area.set` command and require an active area and explicit
region. Never derive Z from a two-dimensional click without validated area data, and
never call phBot directly from the browser.

**Active-route display (operator visual target, 2026-09-28):** While a map-issued
navigation command is active, draw its remaining walk path in the frontend map as a
cyan line with visible waypoint dots, the current character marker and destination.
The operator-supplied image is a visual target for this overlay, not evidence of a
phMonitor API or routing algorithm. Keep route geometry transient and scoped to the
command, character, session, server, region and cave floor. The plugin may send a
bounded, validated sequence of walk coordinates through the existing Go live channel
to make this possible; do not expose raw generated script text to the browser or
persist the route as character-position history.

On each fresh observed position, trim passed waypoints and segments and redraw from
the character's current position toward the next remaining waypoint. Never leave a
trail of past positions on the active-route layer. Replace the overlay on reroute and
clear it on confirmed completion, cancellation, failure or session supersession.
During a temporary stale connection, freeze the last route with a stale indication
rather than implying movement. Draw only the segment for the selected region and
floor; break the line at teleports and waits, and never invent a straight segment
across missing or unparseable route instructions. If a generated script cannot yield
safe walk geometry, show navigation status without a speculative path.

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

Donwhang Stone Cave 1F–4F, Tomb of Qin-Shi B1–B6 and Job Temple 1F/2F/Annex 1–5
render distinct floor imagery with an active floor indicator and correct cave-scoped
coordinates/layers; switching floors and returning outdoors update the view cleanly

recent-death/drop time filtering and Academy-member visibility work independently

map architecture supports future heatmap layers

right-click, touch and keyboard point actions offer only commands supported by the
selected character/session; generated-script navigation is enabled only after real
runtime validation and bounded script execution, with confirmation and honest
arrival/result state; training-area readback is visible, and unvalidated coordinates
cannot be submitted

the active navigation overlay shows only remaining validated walk segments, shrinks
with fresh observed movement, respects character/session/region/floor changes and
clears on completion or interruption; no past-position trail or raw script appears
in the browser

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

This ratio is only a conceptual starting point. Monster positions returned in a
snapshot do not establish which spatial cells the observer could see. Until a
coverage footprint is supported by verified data, label the readback as an
observer-local average count and do not claim spatial mob density.

Exact spatial model may evolve during implementation.

**Acceptance criteria:**

observations from multiple characters can contribute

standing still for a long period does not incorrectly create arbitrarily hot cells merely because time passed

backend can query spatial mob density by area/time range only after an eligible
observation-coverage model has been verified; otherwise expose the limited
observer-local metric explicitly

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

item acquisition, transfer and drop history with links back to canonical Slice 5
events; keep world drops distinct from items actually acquired by this character/pet

valuable/rare drop tracking preserving observed rarity/color/seal metadata

acquisition analytics by destination/source container and proven method where available,
including character bag, Pick/Grab-pet inventory, job pouch and party-distribution
provenance when a verified source identifies the recipient. Unknown acquisition cause
must remain unknown rather than being inferred from party/pet state.

one Item Search query surface across character inventory, equipped/character sets,
character storage, guild storage, applicable pet inventories and job pouch where the
verified source exposes them; retain source/container/owner, server,
observer/freshness and navigation back to the owning character/guild/pet

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
inventory, equipment/character set, character storage, guild storage, an applicable
pet inventory or job pouch, while still showing its exact
source/container/owner/freshness.

Item acquisition history distinguishes actual owned-item gains from world drops and
cross-container transfers; moving an item from Pick/Grab pet to the character bag does
not inflate acquisition counts.

Party-distributed items show that provenance only when it was actually observed from a
verified callback/packet/source; inventory-only evidence is displayed as an acquisition
with unknown method.

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

### Post-Slice-15 candidate

- **Trace target picker:** When the operator clicks Start Trace, show all player
  characters currently available within trace range and let the operator select the
  target. Verify the official phBot API/runtime source for how nearby players and
  range are exposed before implementing; keep selection session-targeted and report
  unavailable/stale results honestly. This is a later-phase backlog item, not part of
  Slice 3's current `trace.start` name argument or a reason to infer botting state.

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

**Slice 3 LAN recovery checkpoint (2026-09-27 12:59 UTC):** after the server-only rebuild, PostgreSQL and web stayed up and both plugin 1.1.0 v3 profiles reconnected; the authenticated dashboard reports 4 online characters and fresh snapshots. The browser's former operator session had been lost at Go server restart because sessions are process-memory only; signing in again restored the existing `/api/live` stream. On `nuker1` detail, Actions buttons are enabled for the current session and Start Trace opens its session-locked form; it was cancelled without submitting a command. `/phbot/client` only exposes disabled Clientless because no safe documented per-session primitive exists. To support these still-deployed whole-second-only parsers, Go now sends whole-second command expiry timestamps while PostgreSQL/Go deadlines retain full precision; truncation only shortens validity and `ttl_ms` is still an upper bound. Added a dispatcher timestamp regression test; `go test ./internal/commands ./internal/httpapi` passed. The server-only Compose rebuild is healthy and the browser again shows both agents Online with 4 characters. No migration, DB restart, or real command occurred. Next: run final Slice 3 non-destructive acceptance checks; keep real mutation/effect validation open unless explicitly authorized, report clientless/runtime and reference-screen gates without claiming full completion; stop before Slice 4.

**Slice 3 LAN command recovery and runtime checkpoint (2026-09-27):** on the
LAN's plain HTTP origin, `crypto.randomUUID()` was unavailable because browsers
restrict it to secure contexts. The UI swallowed that pre-request exception into
“Command could not be accepted,” so Go/PostgreSQL correctly had no command row. Both
command forms now use a shared `crypto.getRandomValues()` idempotency-key helper,
available on this HTTP LAN origin; web was rebuilt and restarted. The first earlier
web build failed for lack of disk space; the bounded Buildx cache prune retained
containers, images and the PostgreSQL volume. A follow-up Compose command restarted
Go and PostgreSQL unintentionally; the named data volume remained, PostgreSQL
recovered, and 4 characters / 16 agent records were present. No database volume was
deleted. The restart invalidated the in-memory operator session; services recovered,
agents reconnected and the operator reauthenticated.

The operator explicitly authorized testing with `nuker1`. From its live detail page,
with session `84bb2a1f-36bf-4993-8b5a-7b0f60e83750`, observed radius 20, submitted
only `Set Training Radius = 20`. UI reported durable acceptance and then showed
`completed` / `observed` in command history over the existing `/api/live` browser
connection; fresh training-area readback remained region 25735, position 100/1559/0,
radius 20. Command id: `cmd_404e7465-956a-4802-8f6d-59e590fe59bd`. This validates one
low-impact real-runtime round trip and readback for that runtime, not the rest of the
catalog or botting effects. No other command was submitted.

Latest targeted checks after the HTTP fix passed locally: Go tests and vet, 30 Python
plugin tests, Python compile, Nuxt format check/typecheck/build and lint (13 existing
`vue/html-self-closing` warnings, zero errors). Remaining P7 work: run current
race/database/browser/container acceptance checks and capture the required
1440×1000, 1280×800, 390×844 and 2560×1315 comparisons; these were not completed by
the single real command check. Clientless still lacks a verified safe per-session
API; scripts remain outside Slice 3's bounded command catalog. Do not claim Slice 3
complete and stop before Slice 4. Exact next action: execute the remaining P7 matrix
from `docs/slice-3-implementation-plan.md`, record real runtime radius evidence
separately from simulator coverage, and report each unresolved gate.

**Slice 3 Walk path implementation checkpoint (2026-09-27):** after the operator
clarified Walk must navigate a path, verified the official
[phBot Paths API](https://plugins.phbot.org/phbot-api/paths) and Movement API. The
production `PhMon.py` worker now requires `generate_path`, `move_to_region` and
`get_position` on its exact socket capability report; it validates a route of at most
256 finite same-region waypoints and steps them from phBot's callback. Observed
arrival uses a documented-in-project 12-unit horizontal tolerance, the route times
out after five minutes, and path API `False`/`None`, invalid/cross-region routes,
target changes and missing position have explicit failure/unknown outcomes. Server
gives Walk a six-minute result grace around the plugin's five-minute route limit and
accepts plugin-reported `unknown` outcomes. No teleport or generated script execution.
UI and protocol/capability/plugin/setup/parity docs explain these semantics. Added
production-worker fake-adapter tests for traversal, cross-region rejection and session
supersession. Validation: 32 Python tests/compile, Go tests/vet, Nuxt
format/lint/typecheck/production build, and `git diff --check` pass (13 existing
self-closing lint warnings). Deployed server and web to `node@192.168.10.25` using
web-only restart for the final copy update; `/api/health` and all Compose health checks
are green, and the PostgreSQL container ID/start time stayed unchanged. Browser
verification through the existing `/api/live` connection shows 4 online characters;
the connected PhMon 1.1.0 runtime has Walk disabled with the explicit minimum-version
message. Plugin 1.1.2 is intentionally not deployed because the operator will install
it. No real walk/movement was issued; nuker1 authorization was only for the same-value
radius check. Exact next step: after operator installs plugin 1.1.2, verify its
capability report and route behavior with an explicitly authorized safe movement test;
meanwhile finish remaining P7 checks and keep the real movement/runtime and reference
capture gates open. Slice 3 remains incomplete; do not continue to Slice 4.

**Slice 3 non-Walk P7 verification (2026-09-27):** the LAN browser was reviewed at
1440×1000, 1280×800, 390×844 and 2560×1315. CI run `36330451436` passed the
PostgreSQL integration/race checks, production-worker command smoke with fake
adapters, authenticated reconnect, outage recovery and `/api/live` browser audits.
The mobile audit confirms visible character rows in the bounded scroller. Screenshots
were inspected in-session but not saved. The live nuker1 runtime reports
`client.clientless.supported=false` / `unsupported_runtime_primitive`; the control
stayed disabled and no command was sent. Walk traversal was excluded by instruction;
plugin 1.1.0 does not meet the 1.1.2 pathfinding requirement. Execute Script remains
outside the bounded catalog. Local base merge and status records are ready to commit;
next run is to commit/push, rerun CI, request fresh Copilot and CodeRabbit reviews,
and triage their results. Copilot has returned a quota-limit response, and CodeRabbit
was processing the pre-merge head. Preserve untracked operator files
`plugin/PhMon5.py`, `server/phmonctl.exe`, and `server/server.exe`. Keep PR draft and
Slice 3 incomplete while real Walk, safe Clientless and broad real-runtime mutation
gates remain open; stop before Slice 4.


### Resume — 2026-09-27 Slice 4 item correction

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


### Resume — 2026-09-27 rolled item detail implementation

Active scope: Slice 4 only. Plugin is now 1.2.2, and its loaded version remains
visible in QtBind. `plugin/PhMon.py` adds bounded passive 0x3040/0x3052 readers,
off-callback decoding, source-slot/model/session tracking, API reconciliation, and
fail-closed invalidation on 0xB034, malformed packets, unsupported protocol, queue
overflow, profile/session changes and mismatched API state. `server/internal/resources`
adds dataset-gated family roll-quality percentages, exact 64-bit parsing and optional
validated blue definitions. `web/app/components/ItemSlot.vue` renders quality,
unavailable states and resolved blues. Protocol and runtime evidence are recorded in
`docs/item-instance-evidence.md`, `docs/protocol.md`, `docs/phbot-capabilities.md`,
`docs/reference-parity.md` and `plugin/README.md`.

Validation completed: 48 plugin tests; 29 exporter tests; `go test ./...` and
`go vet ./...`; 3 frontend formatting tests; Nuxt typecheck/build. ESLint reports
only the existing self-closing-void warnings. `go test -race` could not run because
this Windows Go environment has CGO disabled. The rebuilt dataset contains raw
definitions for 615 magic options (298 with ranges), but no verified labels, units or
scales. No Greatest packet fixture is available, and the packet layouts are
corroborated by pinned RSBot code but not confirmed against the active runtime.
Absolute stat formulas, max durability, named blue values/scaling, storage snapshots,
full item moves/transfers and real tooltip values remain open.

The additive server/frontend release was deployed at
`node@192.168.10.25:/var/www/phmon`. Only `server` and `web` were rebuilt/recreated;
PostgreSQL stayed healthy on `phmon_postgres_data`. The operator has loaded a plugin
reporting 1.2.2: two agents connect, and current API-backed items continue arriving.
However, the persisted `item_enrichment` payload on all four characters still reports
`not_observed / unknown / passive_item_packet_decoder_not_enabled`, which is the disabled
fallback rather than the parser-enabled workspace shape. Zero of 239 item rows has an
`instance` object. Next: verify/reload the exact parser-enabled `plugin/PhMon.py` file,
then recheck protocol detection and naturally arriving item packets; do not request item
movement or other game actions to create traffic. Keep Slice 4 open until the real
packet, formula and tooltip gates are met; stop before Slice 5.

### Resume — 2026-09-27 persistent live WebSocket

Fixed route navigation reconnects by moving WebSocket lifecycle ownership from
component mount counts to the authenticated default layout in `web/app/layouts/default.vue`
and `web/app/composables/useLiveData.ts`. Starting is idempotent; the shared socket
remains alive while Dashboard and Stats subscriptions change. Reconnect waits for a
closing socket's close event. Frontend unit tests, lint, typecheck and production build
passed. The two changed files were copied to `node@192.168.10.25:/var/www/phmon`, the
web image was rebuilt and only the web container was recreated. Browser verification
showed no WebSocket event during Dashboard → Stats or Stats → Dashboard, with fleet
summary, character list and inventory still visible. Server and PostgreSQL remained
healthy. Next: resume the Slice 4 item-instance runtime gate; verify the loaded plugin
parser state and wait for naturally observed item details without operating characters.

### Resume — 2026-09-27 item parser reupload follow-up

At 21:03 UTC, two agents reported plugin 1.2.2 / phBot 20.1.1 / agent protocol v4;
four characters and 242 item rows were fresh, with zero `instance` objects. Do not
interpret stored `passive_item_packet_decoder_not_enabled` JSON as proof of the loaded
plugin: `resources.Store.Apply` kept old payload JSON for every non-observed resource.
The deployed checkout's `plugin/PhMon.py` was also the older pre-parser file, unlike
the workspace file. Implemented diagnostic release 1.2.3 with
`decoder_build=vsro_1188_passive_r1` in QtBind and enrichment telemetry, and updated
resource persistence to refresh only the `item_enrichment` diagnostic payload on
`not_observed` while preserving its observed timestamp. Added plugin and DB integration
tests. Next: validate, copy the updated server store and plugin release artifact to
`/var/www/phmon`, rebuild/restart only the backend, then have the operator load this
exact 1.2.3 file. Confirm the parser build/protocol before waiting for natural item
packets. Do not operate characters; rolled tooltips stay open until real packet,
layout and formula evidence arrives.

### Resume — 2026-09-27 item protocol selection correction

Live DB inspection at 21:24 UTC showed all four characters running plugin 1.2.3,
phBot 20.1.1 and agent protocol v4. The `vsro_1188_passive_r1` parser marker was
present, but each `item_enrichment` snapshot reported `protocol=unknown`; the 251 API
item rows contained no instances. A read-only check of the installed phBot config
identified the cause: `get_config_dir()` points to `Config`, but `vSRO.json` is its
parent's file and profiles are a root mapping (`GreatestSRO` -> `servers: [Greatest]`).
The installed flags select the 1.188 baseline; numeric `version=296` is ignored.
Updated plugin detection to support this path/shape, added a reason code, and bumped
the uniquely identifiable plugin to 1.2.4 / `vsro_1188_passive_r2`. Local detector
verification returned `('vsro-1.188', 'v1.188_selected_by_phbot_flags')`; all 50
plugin tests and `go test ./...` pass. The diagnostic persistence backend fix is
already deployed and all three services are healthy. Exact next action: operator
transfers [plugin/PhMon.py] and checks the QtBind label for 1.2.4, then recheck all
four live diagnostics for `protocol=vsro-1.188` and the `r2` marker. Do not operate
characters; actual item decoding, captured layouts, absolute formulas and blue values
remain open gates. I did not inspect `phMonitor-v0.5.0.exe`; the project guidance
explicitly forbids reverse-engineering that client, so the fix uses the installed
phBot config and public phBot API documentation instead.

### Resume — 2026-09-27 21:28 UTC plugin 1.2.4 verified live

Operator uploaded the corrected file. Read-only LAN DB inspection confirmed all four
active characters on plugin 1.2.4 / decoder r2, with `protocol=vsro-1.188` and
`protocol_reason=v1.188_selected_by_phbot_flags`. Heartbeats were fresh, 239 item rows
were present, and zero contained instances. `nuker1` reported
`inventory_operation_unclassified`; the other three reported `protocol_changed`.
Upload and protocol detection are now confirmed. Do not ask for another upload to
solve missing details: full inventory snapshots/movements and verified absolute
formulas/blue semantics remain implementation gaps. Next implementation work is the
remaining decoder/reference-data scope, with natural live item evidence required
before claiming rolled tooltip completion. No real game action was issued.

### Resume — 2026-09-27 authorized reference inspection and API evidence fix

The operator explicitly allowed local phMonitor executable inspection for item
semantics. Static inspection of embedded Python/JavaScript found an API-backed
tooltip path; see `docs/reference/item-tooltip-investigation.md` for the artifact
hash, relevant offsets, formula observations and unresolved discrepancies. The
earlier claim that richer fields necessarily require packets was too strong.
PhMon discarded integer dictionary keys, potentially losing blue/attribute IDs;
it also omitted several attribute aliases. Independently fixed these in plugin
1.2.5 with evidence schema 2, ordered typed mapping entries and bounded field-type
diagnostics. All 52 plugin tests pass. No third-party code/assets were copied into
the app, no external service contacted, and no character operated. Next: operator
loads 1.2.5, inspect live raw evidence/schema, then implement verified typed API
instance conversion and family calculations. Do not blindly adopt reference name
heuristics, accessory calculations or unexplained attack adjustments. Backend
already accepts additive JSON evidence; no service restart is needed for this fix.
The 1.2.5 single-file release is staged in the local plugin directory and
`node@192.168.10.25:/var/www/phmon/plugin/PhMon.py`; checksums match. The running
phBot processes were not changed. Only the plugin artifact and its README were
copied; the existing stack and volumes were left running.


### Resume — 2026-09-27 plugin 1.2.5 received; API tooltips deployed

Verified all four agents on 1.2.5/phBot20.1.1 with fresh heartbeats and 244
schema-2 items at 21:44:58 UTC. Captured 14 sanitized real API item observations in
server/internal/resources/testdata/phbot-20.1.1-api-items.json. Added independent
api_details.go resolver and tests: shape/count validation, family-specific white
maps, four dataset-matched blue meanings, explicit unknowns, unchanged raw evidence,
and no caching by slot. See docs/item-instance-evidence.md for exact supported scope.
Updated shared ItemSlot to accept API presentation without manufacturing a packet
instance. Deployed only server/web updates with compose build and up --no-deps;
postgres and volumes preserved. Both production images built successfully.

Validation: 53 Python tests, Go tests/vet, frontend unit tests/typecheck pass;
lint has 14 existing void-element warnings and no errors. Browser live verification
shows Python Casque rolls 12/22/19/3/32/9 with Int3/MP5 and Tiger Bone Coronet rolls
61/45/32/0/9/0 with Steady2/Parry5%, simultaneously visible on independent cards.
All four characters recovered online after deployment. PostgreSQL integration/race
and full viewport visual acceptance were not rerun for this increment. Existing
format-notes.md has CRCRLF whitespace problems outside this change.

Plugin 1.2.6 retains the exact named absolute-stat fields seen in live API schema;
it is staged locally and remotely with matching SHA256
7b380f5aad1b566d2257ed009ee7c9e3e733b078f2dee50840fa5c2dbf1ee960.
User transfers it. Exact next action: after upload, inspect actual phys_def/mag_def,
reinforcement/absorption/attack ranges and max_durability against matching reference
items, determine units/scaling and modifiers, then add verified absolute rendering.
Other white families and blue definitions remain open. Do not claim complete rolled
stats, formula validation or safe-invalidation runtime acceptance yet. Do not operate
real characters for fixtures. Dashboard layout and live socket architecture unchanged.

### Resume — 2026-09-28 plugin 1.2.6 absolute tooltip values

The operator transferred 1.2.6. Live phBot 20.1.1 items now contain 21 typed API
field names, including attack/defense, reinforcement and absorption ranges,
max durability, whites and blues. `server/internal/resources/api_details.go`
resolves matching dataset/model/code evidence into ordered named stats, percentages
and all observed blue entries; unknown option codes stay literal with raw values.
`web/app/components/ItemSlot.vue` renders these shared details. Browser inspection
after deployment confirmed Python Casque 54.8/73.3, durability 77/77, parry 23,
reinforcement 13.9%/18.2%, Int3/MP5 and Phoenix Horn Spear's observed attack
ranges, rate, critical, durability and reinforcement ranges. Missing white maps
do not hide independent scalar data. The backend/web were updated in the existing
stack with PostgreSQL and volumes untouched. Focused and full Go tests, vet,
53 plugin tests, frontend unit/typecheck/lint/format/build passed; lint retains
14 nonfatal void-element warnings. Evidence: `docs/item-instance-evidence.md`.

Current API data does not report blue roll-quality percentages, maximum magic
option capacity or Advanced elixir eligibility; these are omitted unless an
actual option says otherwise. Full passive packet coverage and live safe-
invalidation evidence remain open Slice 4 gates. Do not operate characters merely
to generate fixtures. Next: verify reference semantics for remaining families and
modifiers with naturally observed items, then complete packet and Party Setup
gates before marking Slice 4 complete. Do not proceed to Slice 5 under the
current explicit user scope.

### Resume — 2026-09-28 inventory tooltip interaction and rarity colors

Following browser feedback, item enhancement overlays are now white for normal
items and gold only when item metadata confirms rarity. The shared `ItemSlot.vue`
preview opens on hover/focus only; click-to-pin/open and Escape pin-dismissal were
removed. Updated the Slice 4 canonical interaction contract and reference-parity
ledger. Validation: frontend unit tests (3), Nuxt typecheck, production build,
focused ESLint (one existing void-element warning), and ItemSlot Prettier pass.
Pushed in `1522000`; deployed by copying only the changed frontend component/style,
building `web` and recreating only that service. Server/PostgreSQL container IDs
remained unchanged and health stayed green. Browser loaded four live characters and
their inventories; manual pointer-hover/color inspection remains unrecorded. The PR's
validate/stack checks pass; CodeRabbit is rate-limited after the prior review. Next:
record the manual hover/color check when available and keep Slice 4 open.

### Resume — 2026-09-28 Slice 4 audit continuation

Active scope remains Slice 4 on `feat/inventories`. Read and compared the current
branch against `docs/reference-parity.md`, `docs/phbot-capabilities.md`,
`docs/item-instance-evidence.md`, the official phBot Inventory/Party/Config/Misc
APIs, and read-only browser evidence. Implemented Party Setup as a distinct but
read-only view with precise blocker details; did not invent a writer or claim it
works. Added an authenticated typed-confirmation purge for saved guild-storage
records scoped by server and guild, with a serialized DB transaction and explicit
retention semantics. Updated equipment mapping warnings and the evidence docs. The
working tree retains untracked `plugin/phMonitorAdapter.py` untouched; it is not
branch implementation. Validation: `go test ./...`, `go vet ./...`, plugin
`python -m unittest` (54), frontend `npm run test:unit` (3), typecheck, lint (14
pre-existing void-element warnings), Prettier check and production build passed.
`TEST_DATABASE_URL` is unset, so PostgreSQL integration/race validation is not
confirmed. `git diff --check` passed. Visual comparison is documented but not a
same-viewport acceptance: local app was stale and could not take the reference tab's
1440×1000 viewport. No deployment, game traffic, merge or publish. Exact next action:
repeat the purge integration test with PostgreSQL, and close only Slice 4 runtime
gates when supported Party Setup and equipment mapping evidence, packet fixtures /
movement semantics, and fresh matched-size screenshots are available; then revise
the docs. Stop before Slice 5.

### Resume — 2026-09-28 live death status and death event rollout

Active work implements the user's requested death increment during Slice 4: protocol 5
nullable `dead` state, durable `character.died` event ingestion/acknowledgement, and
Dashboard, Stats, sidebar and Events → Deaths surfaces. This is the death-event
foundation brought forward from Slice 5; other event kinds and the remaining Slice 4
acceptance gates stay open. Updated plugin `1.3.0` was already transferred to phBot
clients by the operator; this rollout deployed only server and web files to
`node@192.168.10.25:/var/www/phmon`.

Production server/web images built and restarted successfully. `/readyz` and
`/api/health` returned `status=ok,database=ok`; migration `000006` is present.
PostgreSQL reports three agents on plugin `1.3.0` / protocol 5 and five online
characters with fresh Alive state; no online dead or unknown states were present.
The death event table is empty because no natural `EVENT_DIED` callback occurred.
No character was operated to create one. A PostgreSQL container restart was observed
during the rollout; it came back healthy with the existing database and event table
intact, but the restart cause is unconfirmed.

A production snapshot parameter mismatch was found in the first server build. State
writes were split into smaller updates inside the existing transaction, covered by
`TestCharacterIdentitySessionsSearchAndGroups`, then redeployed. The full Go suite
and `go vet ./...` pass against the isolated PostgreSQL fixture. Existing frontend
unit/typecheck/lint/format/build and 58 plugin tests passed before this server-only
correction. The production death callback remains an open runtime gate. Side-by-side
screenshots at 1440×1000, 1280×800 and 390×844 remain unrecorded; retain the existing
Slice 4 visual/runtime gates and complete the authorized screenshot comparison when
a signed-in review browser is available.

Exact next action: use the authenticated deployed UI to capture Dashboard, Stats,
sidebar and Events → Deaths at the three required viewports, then wait for a natural
phBot death callback to confirm one durable event and its dashboard/list appearance.
Do not operate a character solely for this check. Keep Slice 4 open and continue its
remaining Party Setup, inventory-slot, packet and visual gates.

### Resume — 2026-09-28 selected-server scope correction

The operator reported that selecting Servar still showed Greatest groups and
possibly related data. Audited all server-filtered API and live paths. Implemented
exact case-insensitive server scoping for character list, group list/member
snapshots, and character detail. Fixed the live reconnect path to preserve the
selected group scope. Character search/list and command target UI now send/filter by
scope; detail refuses an ID outside the selected server. Events and guild-storage
were already scoped; `/api/agents` remains intentionally global because one agent
can observe several game servers, and character resources/commands are addressed by
globally unique character/session IDs. Global groups can span servers; scoped group
responses only include matching members.

Validation passed: full Go tests against an isolated PostgreSQL 18.6 fixture, `go vet
./...`, frontend unit tests, Nuxt typecheck, lint (17 non-fatal HTML void-element
warnings), Prettier and production build. The fixture and SSH tunnel were temporary
and have been stopped. Production files were uploaded to `10.25`; only server/web
were rebuilt and restarted, and `/readyz` plus `/api/health` returned
`status=ok,database=ok`. PostgreSQL was not restarted. Direct Servar-vs-Greatest UI
confirmation remains open because the available production browser displayed
operator sign-in and no authenticated browser session was available. The untracked
`plugin/phMonitorAdapter.py` remains untouched.

The operator clarified that character inventory uses shared SRO assets and static
item definitions across servers. Metadata enrichment now resolves presentation by
the stable `servername` code, including rarity/seal, requirements, type data and
reference-stat ranges; conflicting fields across catalogs are omitted. Numeric
model IDs still require the mapped server profile. The updated metadata fix was
uploaded to `10.25` and the `server` service rebuilt/restarted; `/readyz`,
`/api/health` and the static game icon returned successfully, and PostgreSQL was
not restarted. `go test ./internal/resources` and `go build ./...` passed. The browser
still needs an authenticated session for visual inventory confirmation. The untracked
`plugin/phMonitorAdapter.py` remains untouched. Exact next action: report the
cross-server item-definition reuse and deployed status; do not restart PostgreSQL
or touch the untracked plugin file.

### Resume — 2026-09-28 Stats health badge during page sync

The operator reported a brief Unknown character-health/death badge when Stats loads
and asked to retain the last information. `CharacterCard` had treated any shared
WebSocket `syncing` period as stale, even when that character's own state sample was
fresh. It now keeps Alive/Dead from that sample during subscription sync; the
existing timestamp freshness threshold and actual stale state still produce Unknown.
There is no separate health query for this badge: it comes from the character
snapshot. The local `pnpm build` attempt moved the npm-installed modules and stopped
at pnpm's ignored-build-script check; no project lockfiles were changed. Production
Docker's `npm ci` and Nuxt build succeeded. Only web was rebuilt/restarted on 10.25;
`/api/health` returned `status=ok,database=ok`, `/stats` returned HTTP 200, and
PostgreSQL remained healthy without restart. Exact next action: let the operator
reload Stats and confirm the badge no longer flashes Unknown; the available browser
is not authenticated for a visual check. The untracked `plugin/phMonitorAdapter.py`
remains untouched.

### Resume — 2026-09-28 app-wide character death cache

The operator requested a global in-app cache so Dashboard and Stats can display
Alive/Dead directly from the last character state. `useLiveData.ts` now keeps a
reactive cache keyed by normalized server, character ID and session ID, and hydrates
fleet, filtered character, group-member and detail snapshots from the newest
same-session boolean sample. A new session discards the prior state. Cache fallback
retains the sample's original `state_updated_at`; the 30-second age threshold still
turns old values Unknown. The freshness clock continues while the socket reconnects.
Dashboard counts, Stats cards/table and sidebar group indicators use that cached,
time-bounded state. `npm run typecheck` passed. The production Docker npm/Nuxt build
succeeded; only web was rebuilt/restarted on `10.25`. `/api/health` reports
`status=ok,database=ok`, `/stats` returns HTTP 200 and PostgreSQL remains healthy
without restart. Authenticated visual confirmation remains unavailable in the
current browser. Exact next action: operator reloads Dashboard/Stats to observe the
cached status through navigation and reconnect. Keep
`plugin/phMonitorAdapter.py` untouched.

### Resume — 2026-09-28 Slice 5 canonical event pipeline

The user explicitly authorized Slice 5 implementation despite the earlier Slice 4-only
resume boundary. Slice 5 is **in progress**, not complete. Protocol v6 batches now
extend the retained v5 death path; migration `000007_event_pipeline.sql` adds
agent-level nullable context, per-session sequence, dedupe and item indexes. The Go
store validates typed event envelopes, applies authenticated session fencing and
returns per-event batch results after transaction commit. The plugin now queues
lifecycle, documented event IDs 0–10, raw-type chat, alchemy attempts/completion and
identity-aware party/academy/pet/container deltas through its bounded spool. The
Events page, dashboard recent-event/drop feeds and Alchemy Sessions page consume the
canonical query/live stream. Raw chat channel mapping remains `unknown`; drop model
IDs do not become instance details; no packet decoder was activated.

Files changed for this increment: `plugin/PhMon.py`, `plugin/test_phmon.py`,
`plugin/README.md`, `server/internal/database/migrations/000007_event_pipeline.sql`,
`server/internal/events/store.go`, `server/internal/events/validation_test.go`,
`server/internal/events/store_integration_test.go`, `server/internal/httpapi/agent.go`,
`server/internal/httpapi/event.go`, `server/internal/httpapi/live.go`,
`web/shared/types/live.ts`, `web/app/composables/useLiveData.ts`,
`web/app/components/AppSidebar.vue`, `web/app/components/AppTopBar.vue`,
`web/app/components/DashboardOverview.vue`, `web/app/layouts/default.vue`,
`web/app/pages/events.vue`, `web/app/pages/alchemy.vue`,
`web/app/assets/css/main.css`, `docs/phbot-capabilities.md`, `docs/protocol.md`, and
`docs/reference-parity.md` plus six local screenshot artifacts under
`docs/reference/local/`. The untracked `plugin/phMonitorAdapter.py` remains untouched.

Validation passed: `go test ./...`, `go vet ./...`, `go build ./...`,
`python -m py_compile plugin/PhMon.py`, `python -m unittest plugin.test_phmon`
(71 tests, including spool capacity and disk failure), `npm run test:unit` (4 tests),
`npm run typecheck`, `npm run lint`,
`npm run format:check` and `npm run build`. ESLint reports 22 non-fatal
`vue/html-self-closing` warnings. The PostgreSQL integration tests compile but skip:
there is no `TEST_DATABASE_URL`, Docker CLI or disposable PostgreSQL, so migration,
database batch/replay/filter checks and the complete protocol simulator path remain
unverified. Local authenticated Events/Alchemy smoke checks used an empty loopback
fixture backend and verify presentation only. Exact 1440×1000 and 1280×800 content
viewport screenshots are in `docs/reference/local/`. Firefox clamps its minimum
window width to 500 CSS pixels; narrow captures are 500×844, not the required
390×844. Document width matched the viewport at the captured sizes, with the dense
table scrolling inside its panel. Keyboard smoke check passed for the narrow menu:
Tab reaches the open-navigation control, Enter opens and focuses the first link, and
Escape closes the panel and restores focus. Full app-wide keyboard review remains
open. A phBot process was present but its native UI/callback values were inaccessible;
fixture evidence is not runtime validation.

Slice 4 remains incomplete: Party Setup mutation/readback, equipment-slot mapping,
packet fixtures and item family/blue/runtime evidence remain open. The user's plan
makes these dependency gates prerequisites to marking Slice 5 complete. Do not claim
Slice 4 or Slice 5 complete. The local PostgreSQL binaries/service and Docker CLI were
already checked and are unavailable. Exact next action when a disposable local
PostgreSQL is available: set `TEST_DATABASE_URL`, apply migration `000007`, and run
the gated store tests and protocol simulator end to end. Complete the 390×844 browser
comparison when a viewport below 500 CSS pixels is available. Keep the installed
phBot runtime gate open until callback values can be observed. Keep
`plugin/phMonitorAdapter.py` untouched.


### Resume — 2026-09-28 Slice 6 implementation

The user authorized implementation of the planned Slice 6 increment on
`codex/slice-6-chat-plan`, now rebased by merge commit `43181b2` onto `main` commit
`8771ce3`. Work and evidence
are recorded in `docs/slice-6-implementation-plan.md`, `docs/protocol.md`,
`docs/phbot-capabilities.md` and `docs/reference-parity.md`. Added migration
`000009_chat.sql` after main's `000008_character_portraits.sql`, transactionally
projected chat history, read/contact/preferences
APIs, a revision-fenced live stream, `chat.send` validation and session capability
modes, optional phBot chat adapter, responsive `/chat` UI and `/settings` notification
controls. Global sends require confirmation; phBot API acceptance does not establish
delivery. Operator testing on phBot 20.1.2 confirmed numeric callback types 1=General/All,
2=Private, 4=Party, 5=Guild and 6=Global. The plugin maps these values and preserves raw types;
migrations `000010_chat_numeric_channels.sql` and
`000011_chat_echo_reconciliation.sql` normalize existing canonical events/chat
projections, link unique historical outgoing echoes and fix the outbound echo-link
constraint. Migration `000012_chat_private_numeric_type.sql` backfills type-2 inbound
messages as Private. General and Global history/read state are server scoped across
characters; Global unread counts are disabled. Chat messages render as a flat
chronological log in every channel.
`plugin/phManager.py` was absent; the untracked
`plugin/phMonitorAdapter.py` remains untouched and unversioned.

Dashboard now renders the three latest canonical chat events in a recent-chat card
with conversation links. The currently open reference browser was inspected read-only:
it shows six channel tabs, an offline sender selector, and empty General history.

Local validation after this correction: Python plugin tests (79 passed), Nuxt unit
tests (8 passed), `npm run typecheck`,
`npm run format:check` and `npm run build` pass. The deployed server observes two
protocol-v6 agents reporting plugin 1.4.1 and phBot 20.1.2 with current sessions.
New messages arrive under their confirmed channel types. All 144 inbound records
present before the plugin update were reclassified; migration 10 carries the
correction to other databases. Migration 11 fixes the outbound echo-link constraint
and links two unique historical same-session echoes to audited outgoing commands.
Chat renders flat history rows in every channel.
Follow-up chat fix in progress: version the plugin as 1.4.2, map numeric type 2 to
Private and add migration 12 for historical unknown rows. Make General/Global history
server scoped, General read cursors/unread counts apply across characters, and suppress
Global unread counts and badges. Add a multi-character PostgreSQL integration test.
The deployed page currently shows one unclassified message, consistent with type 2
missing from the 1.4.1 callback map. Update this entry after validation/deployment.

`phBotChat` outbound methods remain unverified. PostgreSQL integration tests with
`TEST_DATABASE_URL`, authenticated browser comparison at 1440×1000, 1280×800 and
390×844, and outbound phBot API verification remain open. Keep Slice 6 in progress
until these gates are closed. Exact next action: capture the authenticated chat view
at the required viewports and verify `phBotChat` method availability on the recorded
runtime without sending unapproved test messages.

### Resume — 2026-09-28 character portraits

Implemented the requested portrait restoration in the isolated
`codex/character-portraits` worktree at
`C:\Users\sander\Documents\phmon-character-portraits`, created from `main`.
The original Slice 5 checkout and its preexisting working-tree changes were left
untouched. The plugin now samples the documented optional `get_character_data()`
`model` value; the backend validates, persists and returns `model_id`, and clears it
when a new character session is claimed while retaining it after disconnect. The
active Greatest profile has 52 verified exporter joins and 52 profile-scoped local
portrait URLs. Stats, character detail, Dashboard history and death/event views use
the shared responsive component with initials fallback. Mapping evidence is in
`docs/reference/character-portrait-investigation.md`; parity and asset ledgers were
updated.

Validation: 62 plugin tests, 50 exporter tests and full bundle validation passed;
Go tests and `go vet ./...` passed; the new Go integration test also reads the
persisted model through a fresh pool. That database integration test was skipped
because `TEST_DATABASE_URL` and a local PostgreSQL service are unavailable. Frontend
unit tests (4), typecheck, Prettier and production build passed; ESLint had 19
HTML void-element warnings and no errors. A local deterministic protocol fixture
rendered Stats, detail and Deaths at 1440×1000, 1280×800 and 390×844. The verified
model image loaded locally, unknown and failed images showed initials, all requests
stayed same-origin, and the pages had no horizontal overflow. Fixture screenshots
are ignored under `exports/portrait-fixture-screenshots/`.

The remaining portrait gates are the PostgreSQL-backed session/persistence test and
real phBot runtime confirmation. Exact next action when a disposable database is
available: set `$env:TEST_DATABASE_URL` and run `go test ./internal/characters
./internal/httpapi` from `server/`, then record a supported phBot version check when
that runtime is available. Keep overall Slice 2.5/4/5 status open for their other
requirements.

### Rollout follow-up — 2026-09-28

Prepared a deployment worktree at `C:\Users\sander\Documents\phmon-portrait-rollout`
on current `main` (`dd9219f`, Slice 5 PR #12), carrying the portrait implementation
from the isolated `codex/character-portraits` worktree. Reconciled the canonical
event query/UI with Slice 5 and kept migration `000008_character_portraits.sql`
after `000007_event_pipeline.sql`. The original checkout and portrait implementation
worktree remain unchanged.

The integrated worktree passes Go tests and vet, 77 plugin tests, 50 exporter tests,
Nuxt typecheck, 8 frontend unit tests, ESLint, Prettier and production build. Deployed
the server and web from snapshot `/var/www/phmon/.deploy-character-portraits-20260928`.
Migration `000008_character_portraits.sql` is recorded in the live PostgreSQL ledger;
both `/readyz` and `/api/health` return `status=ok,database=ok`. The PostgreSQL
container ID stayed unchanged. All 52 mapped local portrait URLs return HTTP 200 with
`image/png`; no character has reported the new model field until the updated plugin is
uploaded. Exact next action: operator uploads
`plugin/PhMon.py` from this rollout worktree (plugin version 1.4.0), then verify a
fresh character state carries `model_id` and the live UI renders the associated
portrait. Keep real phBot runtime validation open until that observation succeeds.

### Resume — 2026-09-28 Slice 6 chat follow-up

Continued the Slice 6 fix on `codex/slice-6-chat-plan`. Private contact changes now
keep the existing contact and unread snapshot while the matching conversation page
loads, and chat-only subscription refreshes no longer change the global live-data
status. A badge regression showed the chat filter must retain the selected sender ID
while viewing General/Global so private contacts remain sender scoped. General,
Party, Unknown and Global cursors/counts are server scoped; Private, Guild and Union
stay character scoped, with Private also scoped to its peer. This preserves unread
Guild/Union messages for characters in different groups. Shared server-wide channels
can mark read from another character's view, including when that character has no
local copy. The `/api/chat/read` response returns contacts and channel unread counts
queried after the durable cursor update; the page applies these values immediately,
then accepts the normal shared WebSocket snapshot. Updated
`docs/reference-parity.md` and added integration coverage for sender and group scopes.

Files changed: `server/internal/chat/store.go`,
`server/internal/chat/store_integration_test.go`, `server/internal/httpapi/chat.go`,
`web/app/composables/useLiveData.ts`, `web/app/pages/chat.vue`,
`docs/reference-parity.md` and this resume entry. Preserve the unrelated untracked
`plugin/phMonitorAdapter.py`.

Commit `47a815e` is pushed to PR #14 and deployed to
`node@192.168.10.25:/var/www/phmon`. General/Global inbound copies observed by
multiple characters are combined using server, channel, raw type, sender, text and a
two-second window. Repeated messages from one character remain distinct. Server-wide
unread totals use the same grouping. Added frontend/backend tests and expanded the
PostgreSQL integration scenario with duplicate General/Global observer rows.

Validation: `go test ./...`, Nuxt typecheck, 11 frontend unit tests, ESLint (22
existing HTML void-element warnings, no errors), Prettier and the Nuxt production
build passed. The database integration test remains gated by `TEST_DATABASE_URL`,
which is not configured locally. Latest CI and CodeRabbit review for `47a815e` are
still running. Server/web containers are healthy, `/readyz` reports database healthy
and `/chat` returns HTTP 200. Postgres was not restarted; container ID remains
`96e300a6b9864d6d426fa21dc1a92f150e41e882169be9b038f3601308e8e20d` and volume is
`phmon_postgres_data`. Exact next action: check PR #14 CI and have the operator
verify General/Global duplicates and unread badges on the live chat page. The
in-app browser has no tabs, so live authenticated verification is unavailable here.

### Resume — 2026-09-29 Slice 6 send capability gate

The operator reports phBot plugin 1.4.2 is installed and chat sending works. The
chat composer no longer shows “Waiting for this session to report chat
capabilities.” A missing frontend controls snapshot no longer disables Send; explicit
unsupported-mode reports still disable it, and the Go command service remains the
authoritative check for current-session and channel support. This keeps a delayed or
missing display snapshot from blocking a valid chat send. Updated Slice 6 evidence in
`docs/reference-parity.md`.

Files changed: `web/app/pages/chat.vue`, `docs/reference-parity.md`,
`.github/workflows/ci.yml` and this resume entry. Preserve the unrelated untracked
`plugin/phMonitorAdapter.py`.

Deployed the web-only change to `node@192.168.10.25:/var/www/phmon` after backing up
the source page to `/var/www/.phmon-chat-send-enable-20260929/chat.vue`. Local Nuxt
typecheck, 11 frontend unit tests, targeted ESLint, Prettier and production build
passed. The remote web image built and container became healthy; `/chat` returns
HTTP 200 and the old hint text is absent from the live client bundle. Go `/readyz`
returns `{"status":"ok","database":"ok"}`. The PostgreSQL container stayed at
`96e300a6b9864d6d426fa21dc1a92f150e41e882169be9b038f3601308e8e20d` with volume
`phmon_postgres_data`.

Commit `4bd4f88` is pushed to PR #14. Both validation jobs pass; of the duplicate
stack checks, one passed and one failed after the lifecycle simulator had already
exited and an unguarded cleanup `kill` failed under `set -e`. The workflow cleanup now
accepts that expected already-exited state. CodeRabbit is still processing the new
PR changes. Exact next action: commit and push the workflow fix, then check fresh CI
and CodeRabbit feedback. Live authenticated send verification remains for the
operator because the available browser session is unauthenticated.

### Resume — 2026-09-29 drop names and reference stats

The operator reported that Rare/Normal Drops on the live Events page show no
item names or stats and requested gold rare-item text. This focused correction
is separate from authorization to execute the whole roadmap. Local changes to
`server/internal/resources/metadata.go`, `server/internal/events/store.go`,
`server/internal/httpapi/character_portraits.go`, `web/shared/types/live.ts`,
`web/app/pages/events.vue`, `web/app/components/DashboardOverview.vue`, CSS,
tests and `docs/reference-parity.md` resolve static item presentation by
server-scoped model at query time, show reference ranges with an explicit
non-observed label and color rare-drop text gold. Full Go tests/build/vet,
Nuxt typecheck/build, lint and 13 frontend unit tests pass. The reference
Normal/Rare Drops were inspected read-only in the browser; exact value
tooltips there confirm the remaining collection gap. The live Events API requires auth
and this correction has not been deployed. Exact rolled stats/blues remain
blocked by the documented model-only phBot drop callback; a verified per-drop
source or packet correlation is needed. Next: build, review the diff against
the concurrent Slice 7/8 and level-up changes, integrate without overwriting
the live code, deploy the combined version when authorized, and verify an
authenticated drop row and screenshot.

### Resume — 2026-09-29 level-up callback investigation

The live Greatest Events page showed four level-up occurrences with payload
`{"level":71}` for nuker1–4, and all four character details currently show level
72. Read-only PostgreSQL inspection tied all four events to plugin 1.5.0,
protocol 7 and phBot 20.1.2. The deployed plugin forwards the callback integer
unchanged. Official phBot documentation says the callback contains the new level,
so this correction is grounded in the observed runtime behavior and retains the
raw callback value for future diagnosis.

This branch normalizes unmarked level-up callbacks for phBot 20.1.2 in
`server/internal/events/store.go`; `validation_test.go` covers normalization and
replay, and `000015_level_up_callback_correction.sql` corrects historical rows.
Migration 15 leaves 14 available for the ongoing Slice 7/8 mob-observation
migration, which is already present on the live deployment. Updated
`docs/phbot-capabilities.md` and `docs/reference-parity.md`. `go test ./...`
passed from `server/`; PostgreSQL accepted an `EXPLAIN` of the migration update
against the live schema without executing it. Database integration tests were not
run because no disposable `TEST_DATABASE_URL` is configured. The live server has
not been updated; do not deploy main over the live Slice 7/8 source. Exact next
action: integrate this patch with the current Slice 7/8 branch, run relevant tests
and migration against a disposable database, then deploy the combined server and
verify the four Events rows show level 72.

### Resume — 2026-09-29 Slices 7–8 map and mob observations

Implemented a substantial Slice 7 map surface and Slice 8 observation foundation in
the isolated worktree `C:\Users\sander\Documents\phmon-slice-7-8-luna` on
`codex/slice-7-8-luna`, based on `main`. The user's earlier
`codex/slice-7-8-map-density-plan` and the original checkout were left untouched.
The local map reference was corrected to
`http://192.168.10.105/?server=greatest&view=map&guild=ibot&x_from=2026-09-21&x_to=2026-09-28&e_sub=custom&an_from=2026-09-21&an_to=2026-09-28&u_page=8&c_tab=union&c_char=greatest%7Cgreatest%3Anuker1%3A1907`.
Reference inspection confirms the Stats group summary has a compact map beside its
summary and every character Overview card has its own position map; details and
captured reference filenames are recorded in `docs/reference-parity.md`.

Added `/map`, a shared Leaflet `CRS.Simple` tile renderer and a fail-closed coordinate
adapter; Stats map previews preserve server/area/floor and fresh character selection.
The profile endpoint describes the exported Greatest dataset, tile inventory, cave
families/floors and validation status. Existing authenticated live updates now carry
scoped map snapshots for character positions, current monster snapshots and recent
death/drop events. Agent protocol v7 accepts legacy versions, gathers bounded
`get_monsters()` snapshots, clears live monsters for empty/unavailable results, and
spools eligible complete samples for commit-acknowledged persistence. Migration
`000014_mob_observations.sql` adds idempotent durable samples/monster rows. The
bounded readback groups by observer cell; because monster coordinates do not
establish coverage, the API now identifies the ratio as an observer-local average
and makes no spatial density claim. Event map links and both required Stats preview
placements were added.
Navigation and training-area command controls remain disabled until transform and
runtime validation is available; historical density rendering/reset stay in Slice 9.

Files changed include `server/internal/{httpapi,mapprofile,mobs}` and migration 14,
`plugin/PhMon.py`, its tests/docs, `scripts/agent_simulator.py`, the map page/canvas,
authenticated Nuxt proxy routes for map profile/density, Stats cards/panel, events,
shared map/live types, frontend tests, the reference parity and capability documents,
and this resume ledger. No generated game assets were committed.

Validation: `python -m py_compile scripts/agent_simulator.py`; 83 plugin tests;
`go test ./...`; Nuxt typecheck; 18 frontend unit tests; ESLint (0 errors, 29 existing
and new HTML void-element warnings); Prettier check; and Nuxt production build all
pass. `git diff --check` passes. The local worktree has no `TEST_DATABASE_URL`, so the
database-backed Go suite was run inside a temporary remote Go container against a
fresh isolated PostgreSQL project; `go test ./...` passed all packages, including the
two-observer aggregation, sample replay and stationary-rate integration test. That
test project's database and environment files were removed afterward. Separately, I
ran `map-observations` against an isolated Compose project
with its own temporary PostgreSQL volume. The protocol v7 simulator passed movement,
current/empty/unavailable/truncated monster snapshots, death/drop events, reconnect
and idempotent replay. Authenticated density readback contained two cells with
numerator 1, denominator 2, and averages 1 and 0. The temporary project, database
volume, environment file and copied harness were removed. The regular deployment's
`/readyz` and `/api/health` are healthy; migration 14 is recorded, three agent records
remain with two connected, eight character records remain, and the PostgreSQL
container ID is unchanged. Authenticated HTTP checks passed for map profile, empty
density readback, `/map`, `/stats` and a map PNG tile. The browser still shows the
sign-in screen; no UI sign-in was automated, so screenshots at 1440×1000, 1280×800
and 390×844 remain outstanding.

Keep Slices 7 and 8 in progress. Root tile orientation evidence is documented, but
the in-tile coordinate transform, reverse conversion, region boundaries and command
Z are unvalidated. Qin-Shi Cave, Donwhang Stone Cave and Job Temple floor imagery and
transforms are unavailable. Academy member map positions and a live current ground
drop source are unavailable. Real phBot runtime validation and authorized navigation
testing remain open. Exact next action: capture the three required viewport
comparisons, then validate the selected server's outdoor and cave transforms against
synchronized in-game/map observations and a supported phBot runtime before enabling
map-issued commands or marking either slice complete.

### Test deployment — 2026-09-29

The operator authorized deploying this worktree to
`node@192.168.10.25:/var/www/phmon` and supplied the test operator secret. The secret
was placed in the remote `.env` with mode 0600 and is not recorded here. Before
overlaying source, captured the prior application and `.env` to the mode-0700
rollback snapshot `/var/www/.deploy-slice-7-8-luna-20260929`. Reused the already
deployed 5,118 local map tiles after verifying the count and representative hashes;
no additional game assets were transferred.

The server and web containers were rebuilt and restarted, and additive migration 14
applied successfully. The deployment is healthy, authenticated Map API proxies work,
and PostgreSQL remained on its prior container and volume. A second Compose project
bound to localhost used a separate temporary PostgreSQL volume for the protocol v7
simulator and density readback; all test state was removed afterward. The production
database received no simulator characters, events or mob samples. No phBot client
was changed or operated. The deployed UI was not visually inspected beyond HTTP route
responses because the browser is at the sign-in screen; visual and real-runtime gates
remain open.

### Resume — Stats map preview and navigation — 2026-09-29

Fixed the selected-group Stats view hiding its group map and the disabled Map item in
advanced Tools. `CharacterPanel.vue` now shows the selected group's summary preview;
`AppSidebar.vue` links Map to `/map`. Nuxt typecheck, 18 frontend unit tests, lint
(zero errors; 29 existing warnings) and production build pass. Deployed only the web
container to `node@192.168.10.25`; Go and PostgreSQL were not restarted. Health,
`/map`, the reported group Stats route and migration 14 were verified. Two v1.5.0 / v7
agents remain connected; the database contains 17 mob samples / 37 observations.
Exact next action: have the operator refresh the logged-in Stats page and confirm the
selected-group map preview and active Map link visually; keep transform/cave/runtime
acceptance gates open.

### Resume — 2026-09-29 six Slice 7–8 review fixes

Fixed only the six review findings in the isolated `codex/slice-7-8-luna` worktree.
`MapCanvas.vue` now uses the same max-Y tile transform for opening and reading a
preset, with a regression for tile (168, 97). Relative 1h/24h/7d map event windows
advance every 30 seconds. Map event snapshots query deaths and drops independently,
apply region and coordinate requirements in PostgreSQL before each 100-row bound,
then merge and cap; an exact server-scoped event-ID query preserves deep links outside
the rolling range. Events links carry the profile-mapped area/floor, and `/map`
focuses a resolvable event while explaining missing region transforms or floor art.
Profile responses are guarded by request sequence and selected server.

The observation readback now exposes `observer_local_average_count`, observer-cell
coordinates, monster-row and eligible-sample counts, and the average per sample. It
explicitly says coverage is unverified and the metric is not spatial mob density.
The PostgreSQL integration test now places a monster in another cell and checks that
its location does not create a density value there; it is written but not run here.

Validation passed: `go test ./...`; 23 Nuxt unit tests; Nuxt typecheck; ESLint (zero
errors, 29 existing void-element warnings); Nuxt production build; and 83 plugin
tests. The focused PostgreSQL integration tests were skipped because
`TEST_DATABASE_URL` is unset and Docker is unavailable. No browser viewport comparison
or real phBot session was run for these fixes. Slices 7 and 8 remain in progress: the
outdoor transforms, cave maps, coordinate-linked marker parity and real-runtime gates
are still open. Exact next action: rerun the event and mob PostgreSQL integration
tests with a disposable `TEST_DATABASE_URL`, then complete authenticated map browser
verification when that environment is available.

### Resume — 2026-09-29 Slice 7–8 review-fix deployment

Deployed the scoped six-finding fixes from this worktree to the previously authorized
`node@192.168.10.25:/var/www/phmon` test deployment. Uploaded only `server/` and
`web/` source (excluding local game assets and build artifacts); kept the remote root
`.env` unchanged. The remote pre-update source snapshot is
`/var/www/.deploy-slice-7-8-review-fixes-20260929/source-before.tar.gz`. Both images
built successfully; only server and web were restarted. PostgreSQL was not restarted,
its container ID stayed `96e300a6b9864d6d426fa21dc1a92f150e41e882169be9b038f3601308e8e20d`,
and schema migration 14 remains current. The deployed asset count and representative
map tile hash are unchanged.

Post-deploy evidence: `/readyz` reports database healthy; `/map`, the group Stats
route, `/api/health` and a local map tile return HTTP 200; deployed hashes for the
Go map event handler, mob observation store and affected map/events UI files match
this worktree. No production database fixtures or phBot commands were sent. The
PostgreSQL integration tests and authenticated visual checks remain open; slices 7–8
remain in progress. Exact next action: run the event and cross-cell mob integration
tests with a disposable `TEST_DATABASE_URL`, then complete authenticated browser
verification at 1440×1000, 1280×800 and 390×844. Keep transform, cave imagery and
real phBot runtime gates open.

### Resume — 2026-09-29 live map marker and popup parity

Scoped to the operator's three map screenshots and authorized static inspection of
the local phMonitor v0.5.0 executable. `MapCanvas.vue` now renders model portraits,
small name labels, monster type/HP bubbles, item/drop and death markers, and
character/monster/event detail popups. The character popup's Open Stats action
routes to the character detail page. Incremental marker updates retain one popup
through live snapshots. `map.vue` supplies group and event details; the shared
presentation helper tests type scales, HP math, deduplication and local asset URLs.

The map live response now applies the existing portrait catalog to characters and
events and resolves drop item names/icons in the selected server catalog. Plugin
1.5.1 includes bounded optional monster name, type code, HP/max HP and attack state;
Go validates and relays these fields while keeping older v7 agents compatible.
Files affected: `plugin/PhMon.py`, plugin test/README, Go mob validation/resources/
live portrait enrichment and focused tests, map UI/types/utils/tests, protocol and
capability/parity docs. No schema or command behavior changed in this increment.

Validation: 84 plugin tests, `go test ./...`, 29 Nuxt unit tests, Nuxt typecheck,
production build and lint (0 errors, 29 pre-existing void-element warnings) pass.
Deployed server and web to `node@192.168.10.25:/var/www/phmon`; the remote source
backup is `/var/www/.deploy-map-visuals-20260929/source-before.tar.gz`.
PostgreSQL stayed healthy with its original container ID. The authenticated browser
showed four loaded local model portraits and one character popup surviving an
11.5-second live refresh; Open Stats navigated to `/characters/<id>`.

Remaining blockers: connected 1.5.0 agents do not send monster name/HP, so those
fields remain unavailable until the operator installs/restarts 1.5.1 and real
phBot is observed. The reference drop badge image is absent from the approved
local export, so the current locally drawn badge differs. Cave imagery/transforms,
reverse command Z, real navigation, complete viewport comparison and the disposable
PostgreSQL integration gates remain open. Keep Slices 7–8 in progress.
Exact next action: install/verify plugin 1.5.1 on the real phBot agents and compare
map markers and popups at the three required viewports with the operator's reference
crops.

### Map zoom adjustment — 2026-09-29

Changed the full map zoom range to 50%–2000%, the default to 125%, and wheel/button
increments to 25%. At the 50% floor, the Leaflet grid composites four neighboring
exported 256 px map tiles into each rendered tile. The change is in
`web/app/components/MapCanvas.vue`; `docs/reference-parity.md` records the chosen
zoom behavior. No tests or builds were run for this small UI adjustment.
Exact next action: review the PR after CI completes, then continue the real-runtime
and required viewport parity checks while keeping Slices 7–8 in progress.

### Resume — 2026-09-29 nearby monster names and duplicate sightings

The current map side list displayed numeric monster data and deduped only on
process-local IDs. It now uses monster name, then a readable server name, then a
model fallback; it displays the level separately and explicitly reports unavailable
when no verified integer is present. Plugin version 1.5.2 passes an optional bounded
level only if the phBot runtime provides it; the documented `get_monsters()` response
does not promise that field. Map snapshots now collapse sightings only across
sessions when server, region, monster identity, rank and position (within 8 world
units) match, selecting the freshest observation. Same-session monsters remain
separate. Each row retains the freshest observer's character name.
Focused regression cases were added for display labels, cross-observer overlap,
non-merges and level validation. Files affected: `plugin/PhMon.py`, plugin README and
tests, Go live monster validation/tests, shared live types, map page/marker helper/
popup and its tests, and this parity/operations ledger.

The server and web production images built and the previously authorized test host
was updated; only server/web containers were restarted, while PostgreSQL remained
healthy. `/map`, `/api/health` and server `/readyz` returned HTTP 200. The authenticated
map showed seven current rows with names including Edimmu and Dimension pillar, and
clearly displayed unavailable levels. The remote plugin source hash matches local
1.5.2. No unit suites were run; regression cases were added for label fallback,
cross-session collapse, identity/position separation and invalid levels. No bot
command was sent.

The live `get_monsters()` level field remains runtime-dependent and is not verified;
older connected plugin versions may lack names. Slice 7–8 acceptance gates remain
open. Exact next action: record actual level availability from a connected 1.5.2
runtime (or add a verified level catalog source), then run the focused suites and the
required map/Stats viewport checks before merging PR #19. Live UI inspection also
showed nearby Shakram rows with distinct General and Party General ranks; the matcher
now keeps different rank codes separate and preserves the freshest observer label.
The rank-aware web build was deployed to the same test host. After reconnect, the
authenticated map showed four current monster rows by name and each row identified
the freshest observer; marker labels showed name and level, while the nearby list
also showed its working Type field. Live rows confirmed Party General and General
labels, and unknown code 27 remains explicitly identified as type. Level remained
unavailable. The list change was deployed; PostgreSQL stayed healthy and only the web
container was restarted in this follow-up deployment.

### Resume — 2026-09-29 offline character map treatment

The operator supplied an offline-character map example with desaturated portrait
pins and name badges. `/map` now includes offline characters with retained region,
X/Y and a valid `state_updated_at`; the portrait is grayscale with a gray border, and
the row/popup says offline last-known position. These coordinates are display-only:
fresh online state is still required for current-position actions. Added focused
eligibility coverage. The web production image built and was deployed to the
authorized test host; `/map`, `/api/health` and server `/readyz` returned HTTP 200,
and PostgreSQL reports healthy. The focused frontend suite passed (31 tests). The
current Greatest browser scope still has four online and zero offline characters,
so live visual comparison against an actual offline row remains unverified. Exact
next action: when a retained offline character is available, compare the deployed
marker at desktop and mobile sizes; keep Slices 7–8 in progress until all acceptance
gates, including real phBot and cave transform validation, are met.

### Resume — 2026-09-29 retain map data during refresh

The recurring relative event-window refresh called `setMapFeed`, whose resubscription
callback erased the entire cached map snapshot. This briefly removed character and
monster markers and caused Leaflet to close their popups. Map refresh now keeps the
rendered snapshot while the server, area, floor and region scope match; it still
clears the snapshot when that spatial scope changes, and a valid empty snapshot still
clears markers when received. Added focused scope-retention tests. The Nuxt production
build passed, ESLint on changed TypeScript files passed, and all 33 frontend unit tests
passed. Nuxt typecheck remains red on existing errors in `map.vue` lines 174–188 and
`mapMarkerPresentation.ts` line 111; no errors referenced the changed files. The web
image was deployed to the authorized test host; `/map`, `/api/health` and server
`/readyz` returned HTTP 200 with PostgreSQL healthy. After 35 seconds in the deployed
Greatest map, the authenticated UI still showed character positions and 10 nearby
monsters after the refresh interval. Exact next action: commit/push this fix to PR #19
and verify an open marker popup across a refresh with a connected browser session.

### Resume — 2026-09-29 raster visibility below 100% zoom

The map's Leaflet instance allowed zoom down to 50%, but its GridLayer retained the
Leaflet default `minZoom: 0`. Below 100%, GridLayer pruned all raster tiles while
markers remained visible. `MapCanvas.vue` now applies one shared min/max zoom range
to the map and raster layer. The focused test and all 34 frontend unit tests passed;
changed-file ESLint and local Nuxt production build passed. The test host web image
rebuilt successfully and `/map`, `/api/health` and server `/readyz` returned HTTP 200.
Browser verification at the 50% minimum showed the raster with 16 loaded tile canvases
and zero failed tiles, along with characters and monster markers. Exact next action:
commit/push this correction to PR #19; Slice 7–8 gates remain open.

### Resume — 2026-09-29 faster current monster snapshots

The operator confirmed that a 0.1-second monster poll works on the active setup and
asked for marker movement animation. Current plugin `get_monsters()` polling is 0.1
seconds; durable sampling remains once per minute per observer cell. Character and
marker positions interpolate between live snapshots over 120 ms. A 0.5-pixel snap
threshold caused some updates to jump; it is now 0.01 pixels and all existing marker
kinds interpolate. Added poll-boundary, interpolation and small-delta tests; all 86
plugin and 37 frontend unit tests pass.
The web build is deployed and healthy; PostgreSQL and Go remained running. The plugin
is staged at `/var/www/phmon/plugin/PhMon.py`; the active phBot copy has not been
replaced from this workspace. Runtime CPU impact and visible animation still need
verification with fresh page/plugin loads and moving markers. Exact next action:
reload the map page and staged plugin, then observe movement smoothness, update
freshness and phBot load; keep Slices 7–8 in progress.

### Resume — 2026-09-29 teleport-following outdoor map

Active Slices 7–8 remain in progress. The map now uses the encoded outdoor region
tile as primary placement (Hotan 23687→135/92; live Donwhang 26520→152/103), with
192 world units per tile for exact X/Y. Four explicit profile joins remain examples,
not an allowlist. An old region filter no longer drops a selected character after
teleport; it clears when the new region arrives. Stale online positions remain
visible and labelled. A repeated phBot `connected()` callback no longer resets the
joined-game sampling flag. Files changed: plugin callback/test, Go map profile/API
tests, web coordinate adapter, map feed/markers/UI/tests, and evidence docs. Python
88 tests, focused Go tests, Nuxt 40 unit tests, typecheck and production build pass.
Live phBot reload and side-by-side browser comparison are still required. Cave
imagery/transforms and remaining Slice 7–8 gates stay open. Exact next action:
deploy server/web and stage plugin on the authorized test host, then compare
nuker1 against the reference while teleports occur; confirm sampling after the
operator reloads the plugin.

Deployment and browser follow-up: the first web build exposed an older remote
`mapZoom.ts`; it was backed up and synchronized from this branch. Server and web
production images then built and were restarted without restarting PostgreSQL.
`/map`, `/api/health`, and `/readyz` returned HTTP 200; all three containers are
healthy. Browser inspection uncovered a separate array-filter callback bug that
hid every marker. A dedicated character-list filter and regression test fixed it;
the second web build/restart passed. The authenticated map now shows four markers,
including nuker1 on Hotan tile `(135,92)` at `(77.5,9.0)`. The reference shows
nuker1 at the same coordinates and plaza position. Final local checks: 88 Python
tests, focused Go tests, 41 Nuxt unit tests, Nuxt typecheck and production build
pass; lint has 0 errors and 30 existing Vue style warnings. Exact next action:
observe nuker1 through a fresh teleport after the operator loads the staged
plugin into phBot; keep cave and remaining Slice 7–8 gates open.

### Resume — 2026-09-29 Slice 9 historical heatmaps implementation

Implemented the Slice 9 historical-analytics architecture on
`codex/slice-9-heatmaps`, starting from merged Slice 7–8 main
`b98ace71cc5dcde9c68afea41462b5ee2a6e2123`. Migration
`000016_map_analytics.sql` adds bounded durable character-position samples and
heatmap-reset projection scopes. The existing authenticated character-state stream
now feeds server-side movement history without a plugin/protocol bump: samples are
session fenced, limited to at most one per two seconds, and stationary movement below
four horizontal game units is suppressed unless the region changes. Analytics write
failure does not invalidate an otherwise accepted canonical character-state update.

A new `server/internal/mapanalytics` query domain owns historical aggregation,
validation, bounds and reset semantics. Available sources are deaths, world drops,
unique-spawn callbacks, player movement and historical monster sightings. Mob-type
sightings use reported monster coordinates. The existing observer-cell denominator
is exposed separately as the limited `mob_observer_average` metric. True
`mob_density` deliberately returns `unsupported / observation_coverage_unverified`
because no verified phBot/runtime source establishes the spatial footprint observed
by a monster snapshot. Unvalidated cave/special-area transforms likewise fail closed
rather than falling back to outdoor coordinates.

Authenticated HTTP/Nuxt APIs now expose heatmap reads, observed mob facets and reset.
The reset operation records an exact suppression projection and never deletes
canonical activity events, movement samples or mob-observation rows. A scope with no
region and no character is considered broad and requires a second explicit
confirmation on both frontend and backend. PostgreSQL aggregation is spatially
bucketed and capped at 2,000 returned cells; time windows are capped at 31 days and
auto-select coarser buckets for longer ranges.

The existing `/map` screen renders historical layers under live marker layers using
Leaflet canvas vectors and the existing coordinate adapter. It adds independent
historical controls for 1h/24h/7d/30d/custom ranges, region, character and observed
mob type, plus per-layer loading/error/empty/limited/truncated states, legends and
the scoped reset dialog. Live nearby monsters remain current-state only. Same-scope
historical refresh keeps the previous result visible and cancels stale requests;
spatial scope changes clear old results.

Added PostgreSQL integration coverage for movement fencing/rate suppression, event
and mob layer filtering, monster-coordinate bucketing, the observer-local
denominator, unsupported true density/special maps, scoped reset preservation and a
100,000-row bounded movement aggregation. Added frontend coverage for historical
windows, coordinate reuse, normalization and unsupported layers; the existing map
simulator now includes delayed movement plus death/drop/unique history.

Initial branch CI exposed only Go formatting in the new server wiring/HTTP handler;
those formatting corrections were pushed afterward. Full current-head CI,
PostgreSQL performance timing, authenticated viewport inspection, and CodeRabbit
review are still pending at this resume point. Slices 7–8 remain in progress for
their pre-existing cave/runtime gates and are not marked complete by Slice 9.
Exact next action: require current-head CI green, fix only implementation-caused
failures, then open/review the Slice 9 PR and complete the CodeRabbit review loop
without merging.

### Resume — 2026-09-29 cave maps and reference-style Z handling

Active branch/worktree: `codex/cave-maps-reference-z`, created from `main` at
the merged Slice 7–8 commit. The original `codex/drop-item-display` checkout and
its untracked files were left untouched. Implemented the exporter cave catalog
and local tiles, profile mappings for all 17 floors, Leaflet floor rendering,
floor-scoped cave feeds/markers, signed point conversion, authenticated
session-targeted `character.navigate`, and cross-floor `training.area.set`.
Recorded executable findings and the live Donwhang 1F sample separately from
stale PhMon in the capability and parity ledgers. Validation passed: plugin
89 tests; Go mapprofile/commands/httpapi; Nuxt 43 unit tests; exporter 50 tests
with `PYTHONPATH=src`; Nuxt typecheck, lint (0 errors, 30 existing style
warnings), production build, and `git diff --check`. Local browser inspection
showed the distinct Donwhang floors, loaded 9-tile grids and no desktop horizontal
overflow. At the requested mobile check, Firefox constrained 390px to a 500px
page viewport; controls stack without horizontal overflow and the floor bar is
reachable by scrolling, but exact 390px parity remains unverified. Browser fixture
had no character data; commands were exercised only in the deterministic plugin
adapter tests. No live character command was sent. Exact next action: use a browser
that honors the 390px viewport and perform real phBot navigation validation when
an authorized runtime test window is available; keep that runtime gate open.

### Resume — 2026-09-29 cave map deployment

Deployed the branch to authorized test host `/var/www/phmon`. The server and Nuxt
web Docker images built; only these two services were recreated. PostgreSQL
retained container ID `96e300a6b9864d6d426fa21dc1a92f150e41e882169be9b038f3601308e8e20d`;
`.env` was preserved and no Silkroad container was changed. The pre-deployment
source snapshot is `/var/www/.deploy-cave-maps-reference-z-20260929/source-before.tar.gz`.
All 1,891 cave tiles exist in the built image; health/readiness, `/map`, the asset
index and representative tiles passed HTTP checks. No migration or live command
was run. Exact next action: verify the authenticated map at the deployed target;
keep the real phBot navigation and exact 390px viewport gates open.

### Resume — 2026-09-29 map dataset registration correction

The deployed map initially reported tiles unavailable because the packaged
server game-data mapping still selected the previous Greatest dataset. Generated
`server/game-data/gamedata-17f8847c77edd7c7fadd.json` from the matching exporter
bundle and local asset index, updated `servers.json`, and added a packaged-data
regression that verifies the map profile exposes all 17 cave floors. `go test
./...` passes. Rebuilt/restarted only the server; all services are healthy, the
running server reports the new dataset, `/map` and a cave image return HTTP 200,
and PostgreSQL is unchanged. Exact next action: refresh `/map` in the operator's
authenticated browser; real runtime navigation and 390px viewport gates remain.

### Resume — 2026-09-29 signed cave region and map jump

Inspected the operator-supplied `plugin/phMonitorAdapter.py` in the untouched
checkout: `normalize_position()` copies the integer `region` returned by
`get_position()` and does not clamp negative cave region IDs. Read-only deployed
test with nuker1 reproduced the map jump failure: the current live snapshot had
fresh X/Y/Z (`-24294, -91, 0`) but no region, so Jump was disabled. The PhMon
collector rejected negative values from both character data and `get_position()`;
the Go state validator rejected them too. Fixed the plugin filters, Go state and
event validation, signed cave map-region filters, and bumped the plugin to 1.5.3.
No X/Y fallback was added.
Validation: plugin 91 tests, `go test ./...`, Nuxt 43 unit tests, Nuxt typecheck
and production build passed. Deployed/restarted only the Go server; PostgreSQL and
web stayed up, health/readiness passed and agents reconnected. The updated plugin
source is staged on the test host but not installed into phBot, so live nuker1
still has no region and Jump remains disabled pending plugin install/reload and a
fresh state sample. No bot command was sent. Exact next action: install/reload the
updated PhMon plugin in the phBot runtime that owns nuker1, then retest its live
signed region and Jump UI; do not issue movement commands during this verification.

### Resume — 2026-09-29 cave monster visibility

Active branch/worktree: `codex/cave-maps-reference-z`. After plugin 1.5.3 was
loaded in the live nuker1 client, its current Donwhang position had region
`-32767` and Z `-9`, but the map showed no monster snapshot. Root causes were
the plugin's positive-only observer-region check, the server's signed-region
rejection, and cave-floor projection without monster Z. Plugin 1.5.4 accepts the signed region and
observed `-32767`/`32767` alias, sends observer Z, and the server scopes snapshots
to cave floors via monster Z or observer Z while preserving observed-empty status.
Added migration 000015 for signed durable mob regions and cave marker Z fallback.
Validation passed: plugin 93 tests, `go test ./...`, Nuxt 43 unit tests and Nuxt
typecheck and production build. Deployed server/web and applied migration 000015;
PostgreSQL container stayed up, readiness/health passed, and the browser verified
nine current nuker1 monsters and visible markers on Donwhang 1F. Plugin 1.5.4 is
staged on the test host; the active phBot plugin version was not surfaced in the
map view. No bot movement command was sent. Exact next action: no further work is
needed for the reported visibility issue unless the operator sees the feed drop
again; keep real command execution as a separate authorization/test gate.

### Resume — 2026-09-29 region zone names

Implemented on codex/zone-names in an isolated worktree. The plugin uses the
documented get_zone_name(region) API for canonical event positions and training
area readback. Events carry an optional zone; training state carries the separate
optional training_zone in migration 19. The event view and remote command controls
show names with explicit fallbacks, while retaining numeric region IDs for commands
and map behavior.

Validation before rebasing onto current main passed: 82 plugin tests, go test ./...,
16 frontend unit tests, Nuxt typecheck, Prettier checks and git diff --check.
Database integration tests were skipped because TEST_DATABASE_URL is unset; no
real phBot runtime was available. Re-run checks on the current-main integration.


### Resume — 2026-09-30 issue #23 live party map

Active branch: `feat/23-map-party-members`, created from main
`1f5a1d1038749d4c6594a8f84e09e99dba55b226`. Implemented the Issue #23
current-party map projection without a new poller, protocol version, WebSocket or
persistence table. Plugin normalization now bounds party X/Y like other live map
coordinates and has focused malformed/non-finite/empty/member-bound coverage.
The resource store exposes one server-scoped current-party query fenced by the
resource-state session and active character session; unavailable rows cannot leak
their retained prior payload. Server map projection uses observer region/Z only as
scope, fails closed on unproven cave floors, deduplicates current observations and
bounds the response.

The Nuxt map has a Party members toggle, exact-transform-only party marker helper,
managed-character precedence, the local `mm_sign_party.png` icon and conditional
name/guild/level/HP/MP popup fields. Focused server and frontend tests cover
spawn-state filtering, duplicate freshness, unavailable/empty/session replacement,
cave scope, transform rejection, managed overlap and the configured icon.

Commits so far:
- `df8037661238ebdda574bfe60f39172ed718d64e` — plugin normalization/tests.
- `15f7322e1bc64c0f32885afec803c5fccfce3741` — current-party backend projection/tests.
- `f238719a6715f20954b21d9173ec405f953d25bf` — party map UI/types/tests.

The execution environment cannot directly clone GitHub, so repository changes are
being committed through the connected GitHub API and the hosted validation workflow
is authoritative for the complete Go/Python/Nuxt/PostgreSQL suite. No real bot
command was sent. Exact next action: push this documentation commit, open the PR,
require CI green, then complete the requested CodeRabbit review/full-review loop
without merging.

### Resume — 2026-09-30 Settings agent management restoration

Active branch: `fix/settings-agent-management`, based on main after the dashboard
redesign. Restored agent provisioning and lifecycle management under
**Settings -> Agents** by reusing the existing one-time credential panel and live
agent stream. Active credentials are now listed before first connection, with
never-connected/offline/online states. Offline removal revokes the credential
instead of deleting the agent row, preserving historical foreign-key references;
connected or non-current/stale live state is not removable. Revocation is serialized
against live registry registration so a connect/remove race cannot leave a newly
registered socket using a revoked credential.

Backend changes add migration `000020_agent_revocation.sql`,
`DELETE /api/agents/{id}`, token revocation checks, live invalidation after
create/remove, and unit/integration coverage for never-connected listing, revocation,
connected-agent conflict and registry fencing. Nuxt adds the Settings section,
same-origin delete proxy, responsive remove actions and immediate live refresh.
README, plugin setup docs, protocol and parity ledgers now point operators to
Settings -> Agents. Draft PR #38 is open and must not be merged as part of this task.

Validation at this resume point: prior CI attempts were superseded by review fixes;
the final current-head Validation workflow still needs to complete. No real phBot
character action is required for this maintenance change. Exact next action: require
the final PR #38 head to pass both `validate` and `stack`; fix only failures caused
by this change, then report the draft PR ready for operator review without merging.

### Resume — 2026-09-30 Issue #29 map action targets

Implemented the Issue #29 map-only target-selection foundation in the current
checkout. Focus (`selectedCharacterID`) remains independent from the session-local
action-target ID set. The Characters sidebar now has individual target checkboxes,
All/None, applicable saved-group tri-state controls, and independent focus buttons.
Targets clear on server/area/floor/region changes and reconcile only against a
confirmed matching live snapshot. The map's character marker layer no longer hides
the target list. Shared group data is server-neutral; other screens retain their
existing server filtering. No API, protocol, migration or command behavior changed.

Changed `web/app/pages/map.vue`, `web/app/utils/mapActionTargets.ts`,
`web/tests/mapActionTargets.test.ts`, `web/app/composables/useLiveData.ts`,
`web/app/components/AppSidebar.vue`, `web/app/assets/css/main.css`, and this
reference ledger. The feature is in PR #40 on `codex/issue-29-map-action-targets`.
Its CI passed before the latest visual refinements. CodeRabbit's two findings were
fixed in `da24aac`; its subsequent review was rate limited. The browser fixture
had no saved groups, so group states were exercised in pure unit tests. No
character command was submitted.

Follow-up visual refinement keeps Characters on the right, above Layers, with a
taller scroll area. Historical heatmaps are keyboard accessible and collapsed by
default. Browser checks confirmed the right-side placement, a taller Characters
area at 1440×1000, 1280×800, and 390×844, collapsed default state, and Space-key
toggle. There was no horizontal overflow. The production Nuxt container build,
64 unit tests, and format check passed. Screenshots are in `/tmp` and not
committed because they contain operator data.

The current PR #40 head `845fe0d` passed both `validate` and `stack`; CodeRabbit
completed its review with no new comments on the visual refinement. Exact next
action before the scroll follow-up: operator review PR #40; do not merge without
authorization. Issue #30 owns batch command fan-out.

### Resume — 2026-09-30 map and drawer scroll reachability

Browser testing reproduced nested scrolling on the Map page and clipped lower
mobile navigation links. Updated `web/app/assets/css/main.css`: the map side
panel now follows document scrolling, Characters has a taller bounded list, and
the open mobile drawer is one scroll surface. Updated
`docs/reference-parity.md` with viewport and interaction evidence. The map right
panel has no independent scroll range; browser checks at 1440×1000, 1280×800 and
390×844 showed no horizontal overflow. Focus/target independence, group tri-state,
All/None, keyboard Space, marker-layer independence, and heatmap reachability
were exercised without submitting a character command. Dashboard, Stats, Events,
Chat, Alchemy and Guild Storage were also checked at desktop and mobile widths;
Settings was reached by scrolling the mobile drawer.

Nuxt unit tests (64), typecheck, lint (0 errors; 42 existing style warnings),
Prettier check and production build pass. Local dev health returned HTTP 200 at
`http://192.168.10.25:3006` with the existing backend. A repeated full-page route
sweep stopped the Nuxt dev websocket; it was restarted and subsequent in-app
navigation remained stable. Commit `5c6a607` is pushed to PR #40. Its `validate`
and `stack` checks passed. CodeRabbit's latest status is rate limited, with its
comment saying the next included review becomes available in 16 minutes; no
fresh CodeRabbit review has completed for this scroll fix. Exact next action:
after the review limit resets, request a CodeRabbit review for PR #40, fix any
actionable finding, then report CI and review status without merging.
