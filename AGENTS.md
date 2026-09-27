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
| Events | Unified timeline plus level-up/custom/death/rare-drop/normal-drop/unique and item-acquisition/transfer filters; character/item/date filtering, counts, pagination and map links. Keep world drops distinct from owned-item gains; preserve acquisition destination/container and only attach party/pet/pickup provenance when verified. Rare-drop presentation preserves observed rarity/seal/color/detail metadata; normal-drop detail preserves observed blues/attributes where the source exposes them. Persist occurrences with reliable ordering without inventing missing item properties or acquisition causes. | 5, 7, 13 |
| Chat | General/private/party/guild/union/global tabs; sender character selector, private contacts/new conversation, recipient field, history and jump-to-latest, message composer and results. Add emoji/item references where supported; confirm costly/global sends. | 6, 13, 15 |
| Economy | Global buy/sell/trade offers and stall views; text/character/item-type/subcategory/degree filters, reset controls, stall transactions/chat and source attribution. Derive history only from observable data. | 6, 13 |
| Alchemy | Current attempt log, historical item sessions, highest plus and success/failure/attempt counts; character/item/type/degree filters; statistics over recorded attempts. Do not fabricate probabilities. | 5, 12 |
| Academy | Owned/joined academy tabs, membership/state, join/leave/graduation activity, unread log and mark-read action; map member layer and historical metrics where supported. | 4, 5, 7, 12 |
| Guild Storage | Guild-scoped item listing/detail, search integration, freshness/observer attribution and explicit confirmed removal of stored records. | 4, 13, 15 |
| phBot tools | Client/bot controls explicitly cover start/stop bot or training, set training area, set training radius, walk, disconnect, return scroll and go clientless where the verified phBot API supports each action. Party Setup must reproduce the verified reference control surface and round-trip current configuration/state. Scripts must be discoverable/listable, manageable where supported and executable for explicit character targets; Quest exposes verified information and supported actions. Investigate each tool's real controls and argument semantics before implementation. Route every mutation through authenticated, capability-aware, audited commands; never arbitrary remote Python/shell execution. | 3, 4, 15 |
| Analytics | Character/session rates, deaths, rare/normal items, economy and academy analyses; time/server/character filters, charts and documented calculations backed by durable data. | 12, 13 |
| Map | Pan/zoom, region/quick destination selection, character picker/jump-to-character, coordinates/tile/zoom display; characters and academy members, recent deaths/drops with time ranges, live nearby-monster markers, mob-density/types and other historical layers. Use the server's versioned exported dataset for region/map reference data and local assets where available. Validate dedicated map/coordinate handling for Jangan Cave / Tomb of Qin-Shi, Donwhang Cave / Donwhang Stone Cave and Job Temple / Temple instead of assuming PK2 presence proves the outdoor transform applies. Safe confirmation and explicit server/region/layer scope for heatmap reset. | 2.5, 7–9 |
| Item Search | Search inventory/equipment/character sets, storage, guild storage, applicable pet inventories and job pouch where verified; text/server/type/subcategory/degree filters, reset, item details and owner/source navigation. Resolve static taxonomy/names/icons through the server's game-data profile while preserving live/historical instance facts and exact container provenance from their observed source. | 2.5, 4, 13 |
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

**Slice 3 acceptance follow-up (2026-09-27, active):** current PR head
`14e23f8a` passed CI run `36330451436`: PostgreSQL-backed integration and Go race
coverage; production plugin worker command smoke with fake adapters; authenticated
agent reconnect; database-outage/recovery probes; and browser `/api/live` plus
responsive assertions at 390×844 and 1440×1000. The database recovery probe found
and fixed a typed interval issue; the integration assertion now verifies expired
state and durable audit insertion. The mobile browser assertion verifies visible
character rows inside the bounded table scroller. Manual LAN review also covered
1440×1000, 1280×800, 390×844 and 2560×1315; no post-fix screenshot artifact was
saved. This Windows host has no Docker/PostgreSQL and CGO is disabled, so those
integration/race checks are evidenced by CI rather than local execution. The live
nuker1 runtime reports plugin 1.1.0 and
`client.clientless.supported=false` / `unsupported_runtime_primitive`; Clientless
stayed disabled and no command was submitted. Walk was not tested per the operator's
instruction; the installed plugin also lacks the 1.1.2 pathfinding capability.
Execute Script remains outside the bounded command catalog. GitHub currently reports
a merge conflict against `main`; the fetched base change is merged locally while
preserving both Slice 4 opcode-boundary and item-provenance guidance. CodeRabbit's
review for `14e23f8a` was still processing before the base merge. Copilot has
repeatedly responded that the requesting account reached its quota limit. Exact next
action: commit and push the base merge plus acceptance-record updates, rerun CI,
request both reviewers for the resulting head, and continue triage. Keep the PR draft; do not claim Slice 3
complete or continue to Slice 4 while real Walk, safe Clientless and broad real-runtime
mutation evidence remain open.
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

**Opcode boundary and deferral:** Slice 4 uses the documented `get_pets()` and
`get_party()` APIs for pet and party state, and implements only Party Setup fields and
actions whose semantics are verified through a supported phBot API/runtime contract.
The generic `handle_joymax` / `handle_silkroad` callbacks and `inject_joymax` /
`inject_silkroad` functions are packet-level primitives, not dedicated pet/party API
support. Do not make raw opcode decoding or packet injection a Slice 4/15 dependency.
Additional opcode-based pet/party decoding or actions may be considered as follow-up
work after Slice 15. Before that work is treated as supported, record the target
server/client version, opcode direction, payload semantics, runtime evidence and
limitations in `docs/phbot-capabilities.md`; route mutations through the audited
command lifecycle and verify their effects. This sequencing defers implementation,
not the evidence or parity requirement: document any unresolved required capability
as an explicit gap, and do not claim full parity while it remains unresolved.

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

desktop hover/focus and click behavior plus mobile tap behavior can inspect the same
item information, with keyboard/focus handling and no page-level overflow at the
project's target viewports

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
- per-server/per-character scope
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