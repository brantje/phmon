# Slice 12 — Analytics implementation plan for Luna

Prepared **2026-10-07** against `main` at `b13b036` in `/var/www/phmon`.
Status: **in progress**. Implementation increments through P8 are in the checkout.
A one-million-event, 50-character fixture meets the measured first-request 90-day
and warm 30/7/1-day targets, and the isolated production-worker analytics smoke
passes. P8 remains open for a physical cache-cold run and concurrent chart/map
delivery measurement. P9 remains open for public-demo/browser viewport comparison,
Windows/phBot runtime evidence and remaining source facts. Unchecked packages are
not acceptance evidence.

## 1. Outcome and execution scope

Implement useful, durable performance history inside PhMon's existing application:

- Analytics → Deaths, Rare Drops, Normal Drops, Economy and Academy, with the
  reference's controls, two-chart workspace and summary column.
- Actual character Progress metrics, session history, observed bot uptime,
  daily/weekly summaries and farming-location comparison where the facts support it.
- Alchemy → Statistics, plus the historical item-attempt grouping needed to make
  those statistics reliable.
- Authenticated, bounded Go/PostgreSQL calculations and revision-fenced browser
  delivery, with documented formulas and reproducible fixture results.

The later implementation request should be **scoped to Slice 12 only**. Complete
every unblocked package below and its acceptance checks; stop before implementing
Slices 13–15. Slices 10/11 are absent in this checkout, but rules/schedules are not
dependencies of read-only historical analytics. Do not implement them as a detour.
Reuse and repair an earlier source contract only when Slice 12 needs that repair.

Economy transactions and some Academy facts overlap other slices. Investigate those
sources, implement the supported analytics now, and preserve precise open gates for
missing facts. A decorative chart or an unexplained zero is not completion.

Work in the existing checkout/branch. Do not create or switch to a worktree. Keep
application aggregation in Go, never in the plugin or by downloading all events to
the browser. Do not run real character commands, send external messages, merge or
publish as part of analytics testing. This planning request does not start Luna.

## 2. Verified starting point

Recheck these facts when implementation starts; historical resume entries describe
older environments and must not override the current code.

| Existing implementation | Reuse | Gap Slice 12 must address |
| --- | --- | --- |
| `server/internal/characters/store.go`; migrations `000002`/`000003` | Stable server/character identity, active-session ownership, session start/end and current numeric state | State updates overwrite XP/SP/gold/botting. There is no general performance sample history. Session timestamps alone do not establish uninterrupted observation or bot uptime. |
| `server/internal/events/store.go`; migrations `000006`/`000007` and follow-ups | Canonical, replay-safe death/drop/level/teleport/alchemy/item/membership events; occurrence timestamps, provenance and ordering | No non-spatial analytics query domain. Existing listing totals are not a substitute for chart aggregation. |
| `server/internal/resources/store.go` and `types.go` | Accepted, session/revision-fenced academy and guild-storage observations; observed item facts and server profile resolution | Observations are overwritten. Academy joins/leaves do not carry an academy ID in their current emitted payload. Guild gold needs history and observer reconciliation. |
| `server/internal/mapanalytics/` | Existing SQL/query/performance testing patterns, time bounds, coverage limitations and signed-region handling | This domain serves spatial heatmaps; it is not an XP/gold or general analytics collector. Movement sampling deliberately suppresses stationary samples. |
| `web/app/composables/useLiveData.ts`, `web/shared/types/live.ts`, `server/internal/httpapi/live.go` | One `/api/live` socket, subscription revisions, stale/recovery handling and bounded builds | No analytics stream. Keep heavy historical work from exhausting live snapshot capacity. |
| `web/app/components/CharacterCard.vue` | Current Progress surface and shared character resources | Progress explicitly says historical rates are unavailable. Replace that statement with supported metrics and individual missing-data reasons. |
| `web/app/pages/alchemy.vue` | Existing attempt feed, filters, canonical item popup and count summary | The page lists attempts and has aggregate counts; it does not implement the reference Statistics workflow or robust item sessions. |
| `web/app/components/AppSidebar.vue` | Existing shell and navigation order | Analytics, standalone Academy and Economy are disabled labels. Add the Analytics route and subtabs; do not pretend the other two full products are implemented. |
| `scripts/agent_simulator.py`, smoke scripts, CI | Production plugin worker with fake adapters, authenticated test stack, PostgreSQL/race and browser audit conventions | Add a deterministic analytics scenario and end-to-end assertions; fixtures must remain isolated from production. |

At the planning baseline the plugin uses **protocol 17**. The latest checked-in
migration is **`000024_thief_sightings.sql`**. Choose the next free migration number
at implementation time rather than assuming this remains unchanged.

`web/package.json` has no chart library. Node is pinned by `.nvmrc` to 24.20.0;
the planning shell defaults to Node 20.18.2. Use the pinned toolchain for validation.
Do not copy historical claims that Docker/PostgreSQL are unavailable without
checking the current host. Inspect availability without printing secrets.

## 3. Reference screen contract

All seven supplied captures below were visually reviewed while preparing this
plan. They are **2560 × 1315** operator screenshots, distinct from the earlier
1440 × 1000 public-demo baseline. Use both resolutions for comparison.

Before UI implementation, reopen [the public demo](https://phmonitor.com/demo) in a
real browser, inspect easy/advanced modes, each control, mobile reflow and chart
interaction, and dismiss only its connection-error overlay. The planning pass
checked its public page text, which still exposes the five Analytics tabs and
Alchemy Sessions/Statistics. It did **not** verify current browser interactions,
chart tooltips, mobile behavior or reset dialogs. Do not turn screenshot labels
into claims that hidden workflows were exercised.

### Analytics workspace

Preserve the existing navy/gold shell. The supplied desktop workspace contains
`Analytics | <area>`, a five-tab row, compact From/To date inputs, a short section
description, two similarly sized charts and a narrower summary panel on the right.
The summary contains stacked inset cards rather than a new full-width KPI grid.
At narrow widths, reflow these regions vertically and keep chart/table overflow
inside each region. Keep required filters usable in both presentation modes.

| Reference | Left region | Middle region | Summary / required interactions |
| --- | --- | --- | --- |
| [Deaths](../phmonitor_screenshots/12-analytics-01.png) | Death counts with Per Char / Per Group / Per Location selectors | Deaths by time, with Per Group selector | Total deaths, average deaths per character per day, location with most deaths, character with most deaths; links to scoped Events/character/map evidence |
| [Rare Drops](../phmonitor_screenshots/12-analytics-02.png) | Drops by time with Per Group / Per Location | Drops with Item Type / Item Degree selectors | Total rare drops, character with most drops, known item-type shares; retain the empty chart panels |
| [Normal Drops](../phmonitor_screenshots/12-analytics-03.png) | Same time/group/location controls | Same type/degree controls | Total normal drops, leading character, known type shares; populated counts and chart totals must reconcile |
| [Economy](../phmonitor_screenshots/12-analytics-04.png) | Gold balance; separate Character and Guild Storage selectors | Stall sales; Character and Item Type selectors | Average gold change per day, character with most gold, guild with most gold, average stall sales per day; gold balance and recorded transactions remain distinct |
| [Academy](../phmonitor_screenshots/12-analytics-05.png) | Total graduations | Average time per graduate | Graduations, departures/bans and academies; show unavailable classifications where no verified source distinguishes them |

The screenshot sidebar omits Normal Drops as a sublink while the main tab row
includes it. Keep all five main tabs and provide consistent, working navigation;
record any deliberate sidebar adaptation. Use `/analytics?view=deaths` and the
other view values below as shareable routes rather than pathname-switching in
`app.vue`. Preserve filters when switching views where their meaning is shared.

### Progress and Alchemy

- [Character Progress](../phmonitor_screenshots/02-stats-04.png): compact two-column
  tiles for XP/h **as a percentage**, SP/h, Level up in, Gold/h, Normal Drops (24h),
  Rare Drops (24h), Deaths (24h), Time between returns; a last-24h Training Behavior
  bar and Reset Rates. Expose units, rate period and observation coverage without
  replacing the character-card composition. Label estimates and net balance rates.
- [Alchemy Statistics](../phmonitor_screenshots/05-alchemy.png): Sessions/Statistics
  tabs, an inset summary strip for elixirs/attempts, chance to reach a plus target,
  average elixirs to reach it and latest try, then a wide weekday success chart.
  Match that hierarchy. Use measured outcomes with denominators; no invented
  probability, unobserved material spend or slot-based persistent item identity.

Screenshot values are layout evidence, not test data or correct reference formulas.
In particular, do not reproduce an unexplained chart total, fractional death tick,
extra future date bucket or gold-axis value solely because it appears in a capture.

## 4. Source capability and missing-data contract

The official sources below were checked on 2026-10-07. They establish collection
primitives, not full semantics for every rate. Extend `docs/phbot-capabilities.md`
with version/runtime evidence before enabling any newly interpreted field.

| Fact | Evidence / safe implementation | Limits to preserve |
| --- | --- | --- |
| Character numeric facts | [Character API](https://plugins.phbot.org/phbot-api/character), existing `_sample_character` collector | Current EXP is a balance within a level; SP/gold are balances, not lifetime earnings. Missing fields stay null. |
| Death/drop/level/teleport events | [Events API](https://plugins.phbot.org/phbot-api/events), existing canonical pipeline | Drop callbacks cover equippable items. Teleported does not identify a return-scroll trip. Received time differs from occurrence time. |
| Alchemy attempt | [Alchemy API](https://plugins.phbot.org/phbot-api/alchemy), existing `alchemy_update(slot, success, plus)` | Outcome and plus can be unknown in preserved records. A slot or model does not prove the same item persisted across attempts. |
| Academy membership | [Academy API](https://plugins.phbot.org/phbot-api/academy), `_normalize_academy` and resource diffs | Member `type` codes do not have verified role meanings. The API does not establish graduation, ban or exact join timestamps. |
| Botting state | Existing `_read_botting_state`; capability evidence for Issue #35; [Botting API](https://plugins.phbot.org/phbot-api/botting) | Optional `get_status()` interpretation is runtime-qualified. `stopped`, missing values and `None` remain unknown under current evidence. Commands are not observed uptime. |
| Item/region metadata | [Game Data API](https://plugins.phbot.org/phbot-api/game-data), `resources/metadata.go` and active exported profile | Static definitions may enrich taxonomy, not past instance blues, gold prices or acquisition causes. |
| Guild-storage gold | Existing accepted `guild_storage` payload and documented runtime evidence | Current contents must not be backdated. Multiple character observers can report the same guild; never sum their balances. |
| Stall sale/price facts | No durable transaction domain or accepted sale event exists in this checkout | Investigate official documentation, installed runtime and authorized executable evidence. Chat offers, inventory losses and gold increases do not prove sales. |

Use statuses such as `available`, `limited`, `insufficient_history` and
`unsupported`, with a stable reason and coverage metadata. They are different from
`empty` (a supported source has no recorded occurrences in the selected window).
Zero recorded events never proves zero activity while the observer was absent.

New plugin work is only justified for missing **facts**: for example, including
academy ID and observation context in membership events. Initial XP/SP/gold history
can use already accepted state frames in Go without a protocol bump. Do not add
rate calculations, storage, timers or a speculative wire protocol to Python.

## 5. Calculation contract: settle this before chart code

Implement pure calculation functions with explicit units and a calculation version.
Document final choices in `docs/analytics-calculations.md`. Use one contract for
cards, charts, summary tables and drill-downs.

### Time, identity and scope

1. Every request carries bounded UTC instants `[from, to)` and an explicit IANA
   timezone for calendar grouping. Default the UI to its browser timezone; the
   operator's current timezone is Europe/Amsterdam. Include all of the chosen To
   date by using the next local midnight, not by adding a fixed 86,400 seconds.
   Test 23/25-hour DST days. Store UTC; local calendar days and weeks are display
   groupings, while per-hour denominators use real elapsed seconds.
2. Scope by normalized server and stable character IDs. A wrong-server character
   selection must yield an explicit validation result, not silently broaden scope.
   UUIDs do not authorize data access; use the existing operator session and origin
   policy. Preserve signed cave regions and server/dataset/floor identity.
3. Current groups can overlap and have no membership history. Label Per Group as
   **current group membership**. Do not double-count the headline total when a
   character belongs to two groups. Include Ungrouped and deterministic ties.
   Historical group membership is unavailable until it is actually recorded.
4. All summaries in a snapshot use the same normalized filter, calculation version
   and database read snapshot/as-of time. Cursor pagination must not change totals.
   Late canonical events belong to occurrence-time buckets and invalidate them.

### Observed intervals and rates

- Save sparse accepted numeric-state observations. Proposed initial cadence:
  first sample per session, at most one ordinary sample per **10 seconds**, plus
  immediate level/region/botting/death-state transitions. Keep unchanged heartbeats
  at the regular cadence so stationary farming retains observation coverage.
  Do not feed this from protocol-16+ five-Hz realtime position frames.
- Adjacent samples form an eligible interval only within the same character
  session, with positive duration and no gap greater than **30 seconds**. Clip
  coverage to the requested window. Include predecessor/successor context in SQL
  without importing out-of-window event counts. Never bridge reconnect, backend
  restart, resource unavailability or an expired observation.
- Allocate an observed balance delta to its ending observation's bucket; mark the
  bucket as net observed change rather than assuming uniform earnings. Include a
  delta only when both endpoints lie within the requested rate window. Keep any
  boundary coverage excluded from the delta denominator out of that rate's
  denominator too. Distinguish general coverage from metric-specific coverage.
- For a metric with eligible deltas, `net_rate_per_hour = sum(delta) * 3600 /
  eligible_seconds`. A pooled rate per observed character-hour adds numerators
  and denominators rather than averaging character rates; label that unit. A
  fleet total per wall-clock hour needs its own common covered window and must
  not be confused with the pooled rate. No denominator or less than **60 seconds**
  of eligible rate history yields null with `insufficient_history`, not zero/infinity.
- SP/h and Gold/h initially mean **net observed balance change**. Keep negative
  changes, including spending and losses. Positive changes cannot be relabeled
  farming income, sale revenue or earned SP without a verified source.
- Same-level EXP delta is `current_exp_after - current_exp_before`, preserving
  negative changes. For exactly one observed level increase, use
  `previous_max_exp - previous_current_exp + next_current_exp` only after verifying
  those field meanings and that no intervening rollover/loss is hidden. Multi-level
  jumps need a versioned, verified level-EXP table or stay unavailable. The exported
  `textdata/leveldata.txt` is an investigation input, not a proven normalized table.
  Level decrease, changed profile/EXP rules and implausible rollover break EXP
  comparability; they do not become zero or a huge positive gain.
- Progress XP%/h uses eligible **same-level** EXP change divided by the matching
  level requirement and elapsed hours. Across rollover, show absolute EXP/h where
  supported and a separately labeled current-level pace. A percentage assembled
  from incompatible level requirements is unavailable.
- Level-up ETA is a labeled projection `(max_exp-current_exp)/positive_current_level
  EXP_per_second`, using fresh current state and at least 60 seconds of comparable
  recent history. Nonpositive pace, cap, stale state or missing requirement → null.
- Recorded deaths/drops per hour need a matching covered observation denominator;
  raw event totals can still be shown with limited coverage. 24h tiles always use
  the rolling 24h event window; Reset Rates never changes these totals.
- A monitoring-session span is bounded by start, last observed activity and end.
  Known online time uses covered observations, not an `ended_at=now()` restart
  reconciliation across an unobserved outage. Expose session span and observed
  duration separately. Bot uptime accrues only in intervals with consistent known
  botting state; mixed/unknown intervals remain unknown. Return true/false/unknown
  durations and coverage instead of calling all connected time botting.

### Events, grouping and balances

| Metric | Definition |
| --- | --- |
| Death total | Count distinct canonical `character.died` event IDs matching the filter. Preserve reason provenance; inferred PvP is not a verified killer. |
| Average deaths / character / day | Death total divided by the selected eligible character population and selected local calendar-day count. Return both denominators and limited coverage. Characters with zero deaths remain in the denominator; document the historical-population limitation. |
| Leading character/location | Maximum recorded count, deterministic tie order and scoped links. Unknown locations are an explicit bucket; never use current position as past location. |
| Rare/normal drop observations | Separate source population for canonical world-drop observations using stored kind/classification/version. Unknown rarity remains unknown. Cross-character observations are not guaranteed unique world items. |
| Owned gains | Separate population of recorded owned-item gains with exact destination and verified provenance. Exclude transfers from acquisition counts. Show occurrences and quantity as distinct measures. Linked world-drop/gain rows must not be added into a single invented total. |
| Type/degree shares | Known taxonomy count divided by the known classified population, accompanied by known/unknown counts. Reuse shared profile-aware taxonomy. Unknown degree is not degree 0. |
| Gold balance | Last valid observed value per character or server/guild at each bucket, with bounded freshness. Keep unavailable entities separate; do not interpolate across long gaps or add repeated guild observers. Opening values may provide labeled context, not fabricated history. |
| Daily gold change | Signed comparable observed deltas; do not derive income from highest minus lowest balance. Character/guild transfers may net out in a combined view but remain unattributed unless captured. |
| Farming comparison | Per server/profile/region/floor or verified named training area, compare eligible net gains, event totals and denominators. Attribute an interval only if its endpoints share the location; transitions are unknown. Coordinates alone do not establish In Field or In Town. |

Default drop charts to world observations and label that source. Offer a separate
owned-gains view using the existing event provenance; do not silently reuse the
Events page's combined `include_owned_gains` total as a unique-drop total. Any future
combined presentation needs an explicit, tested occurrence-linking rule.

### Alchemy and Academy

- Alchemy attempts count canonical `alchemy.attempt` IDs. Success/failure require
  a boolean source value; unknown outcomes remain a third count. Empirical success
  share is `successes/(successes+failures)` with the denominator visible. Weekday
  grouping uses the chosen timezone. Highest plus uses observed values only.
- An attempt's inventory slot is not persistent item identity. Derive bounded
  attempt runs using explicit captured item identity where present, otherwise a
  conservative character/session/slot/item-fingerprint segment. Break on observed
  replacement, inventory loss, reconnect and unavailable identity. Label ambiguous
  runs and omit item-session-only calculations rather than merge identical models.
  `alchemy.finished` does not itself count as an attempt or prove a plus target.
- Reach-target share and average attempts-to-target require complete, comparable
  runs with a known starting plus, target and terminal outcome. Incomplete runs
  cannot be counted as failures or omitted without exposing cohort exclusions.
  If the source cannot establish those facts, retain the summary positions with
  clear unavailable reasons and deliver the supported attempt/outcome statistics.
- Count academy joins/leaves as **observed membership changes**. A level threshold,
  disappearing member or numeric member type never proves graduation or ban.
  Graduate duration requires a proven graduation and compatible observed join,
  academy identity, member identity and observation coverage. A member present at
  first snapshot has an unknown join time. Multiple controlled observers must not
  inflate a claimed unique academy-wide total; show per-observer changes until a
  source establishes unique occurrence identity.

Reset Rates is a backend-persisted per-character baseline with audit metadata,
affecting the card's rate window only. Confirm the named target and explain the
effect. It must not delete events, sessions, samples, heatmaps or other characters'
metrics. Time between returns and Training Behavior require verified return/location
semantics; investigate them and keep the missing portions explicit.

## 6. Storage and transport design

These are proposed PhMon contracts, not existing phBot signatures. Finalize names
in P0/P1 and keep implementation, tests and protocol docs consistent.

### Durable facts

Add a narrow migration for:

1. `character_metric_samples`: sample ID, character/session/agent ownership,
   server/profile identity, backend accepted timestamp, nullable level/current EXP/
   max EXP/SP/gold/botting/dead, region/zone/floor where verified, source and sample
   schema version. Index character/session/time and server/time. Keep signed region
   constraints aligned with existing migrations. Numeric balances remain integers.
2. `guild_gold_samples`: normalized server/guild, observer character/session,
   accepted resource revision/check timestamp, nullable gold, availability and
   provenance. Preserve conflicting observers; reconcile one guild value per bucket
   deterministically without summing them. A repeated equal balance is a fresh
   observation only if the incoming resource actually checked that source.
3. `character_rate_resets`: scoped character, baseline timestamp, actor, request
   idempotency key and audit time. Keep reset mutation separate from source deletion.
4. Academy context / alchemy-run projection only if required after source inspection.
   Any derived projection keeps canonical event IDs, calculation version and a
   bounded idempotent rebuild path. Prefer queries over creating duplicate truth.

Insert character samples inside the existing accepted character-state transaction,
after ownership validation and under its session lock. Extract a small transaction
writer helper to avoid copying the SQL between `SnapshotSession`/`UpdateSession`.
Use the same approach in `resources.Store.Apply` for validated guild gold. Rejected
state, stale resource revision or failed transaction must not write analytics facts.
Identical duplicate frames cannot create gain/outcome counts; sampling cadence and
transition comparison prevent write storms. Invalidation occurs only after commit.

No historical numeric backfill exists in current overwritten rows. Seed a labeled
first observation from genuinely current accepted data and expose history starting
there. Canonical old events/sessions remain queryable. An event payload projection
can be rebuilt; a missing past balance cannot.

Initial sample retention: configurable **90 days**, with bounded batched cleanup
and exposed oldest available time. Query windows may extend to 366 days for events,
but numeric results outside retained coverage remain limited. Do not delete existing
canonical sources under a new analytics retention setting. Measure storage costs
(10-second sampling is up to 8,640 ordinary rows/character/day); choose a different
cadence only with documented coverage/error and load evidence. Add daily rollups
only if measurements justify them, with late-event correction and rebuild tests.

Keep the existing confirmed guild-record purge coherent: describe whether the
new guild-gold history is removed. Prefer including it in that same exact-scope,
typed-name, advisory-lock transaction and returning its deleted count. Analytics
cache/projections must not resurrect deleted history. Do not introduce a new bulk
deletion UI as part of this slice.

### Read contract and bounded delivery

- Introduce `server/internal/analytics/` with typed filters/results, pure
  calculations and PostgreSQL query methods; keep spatial `mapanalytics` intact.
- Add an `analytics` live stream for one selected area and `character_analytics`
  for visible Progress cards/details, over the existing `/api/live` connection.
  Reuse revision validation, auth, unsubscribe, stale-data rules and reconnect.
  Batch visible character IDs to avoid exhausting the 32-subscription limit.
- Shared filter: server, character/group scope, `view` (`deaths`, `rare_drops`,
  `normal_drops`, `economy`, `academy`, `alchemy`, `performance`), from/to, timezone,
  bucket (`hour`, `day`, `week`), grouping, optional item type/degree/source and
  guild scope, page cursor/size. Reject unknown combinations and bound every input.
- Result: normalized filter, calculation version, as-of/source coverage, per-metric
  status/reason, headline summaries, chart series, bounded table rows, total and
  cursor. Balances/large totals that exceed JavaScript's safe integer range use
  decimal strings. Rates may use finite rounded numbers with explicit units.
- Suggested bounds: 366-day maximum range, 500 points/series, 20 displayed groups
  plus Other, 50 visible character IDs, page size 25/max 100, 3-second statement
  timeout. Totals include all eligible rows before chart truncation; disclose
  truncation and preserve Other so bars can reconcile. Null gaps differ from zero
  event buckets. Choose a coarser explicit bucket when the requested range would
  exceed the point bound, and return that choice in the normalized filter. Reject
  oversized frames rather than silently clipping evidence.
- Use SQL aggregation, filtered session/sample window queries and appropriate
  indexes. Do not load a million raw events into Go. Keep a bounded analytics build
  budget separate from the existing two live build slots if measurements show
  historical work delays live data. Coalesce per-scope invalidations and cancel
  superseded requests; never build 90-day charts on every incoming state frame.
- Browser read/refresh/filter changes use the live subscription. Historical REST
  handlers are optional for authenticated tooling and can share the query domain;
  do not add browser polling, SSE or a second WebSocket. The existing Map heatmap
  HTTP implementation does not authorize a new polling pattern here.
- `POST /api/analytics/rate-resets` is a proposed authenticated same-origin,
  idempotent mutation with a thin Nitro relay. Success invalidates Progress data;
  it does not optimistically rewrite observations. No bot command is involved.

## 7. Sequential implementation packages

Complete these in order. Each package leaves working behavior and tests, records
evidence, and proceeds to the next. Do not stop after scaffolding or a screenshot.

### P0 — Freeze sources, contracts and reference evidence

**Read:** `AGENTS.md` Slice 12 and matrix; this plan; capabilities/parity/protocol
docs; character/events/resources/mapanalytics stores; live hub/composable;
CharacterCard, Alchemy, sidebar; simulator and CI.

**Work:** Recheck Git status/versions, current migrations and prerequisite reality.
Inspect reference controls in a browser. Inventory available runtime facts without
operating characters. Verify level EXP, botting, return/town classifications,
academy identity/roles/graduation, stall transactions and alchemy identity. Check
the authorized executable analysis notes and username-free executable location if
needed; do not commit executable/decompiled source or inspect private services.
Write calculation/source tables, chosen filters and exact blockers in the docs.

**Exit:** Every promised metric has a source, units, denominator, missing-data rule,
scope and test scenario. No claimed reference interaction is merely guessed.

### P1 — Persist accepted performance and gold facts

**Files:** next migration(s); new analytics sample writer/types; character/resource
stores and integration tests; config/main startup for retention if needed.

**Work:** Implement transactionally fenced sampling and cadence; first sample and
transition behavior; guild observer provenance and availability; indexes, bounded
retention and sample-start coverage. Preserve state ingestion and realtime map
latency. Verify fresh database creation and upgrade from migration 24.

**Exit:** Production plugin worker → Go → PostgreSQL stores real accepted facts;
restart preserves them; stale ownership/revision and failed transactions write
nothing. Stationary, unchanged characters retain bounded regular coverage.

### P2 — First vertical increment: Deaths analytics

**Files:** `server/internal/analytics/{types,filter,events,store}.go` (proposed),
query/integration tests; live hub and shared analytics types/composable; new
`web/app/pages/analytics.vue`, focused chart/summary/filter components; sidebar.

**Work:** Add filtered death totals, per-character/current-group/location counts,
time series and four summaries. Implement one live analytics subscription, empty/
error/stale states and accessible chart/table evidence. Wire the actual route/tab
and drill-downs. Use the established shell and auth.

**Exit:** A deterministic death emitted through the production worker appears once
in the selected chart, summary, table and Events drill-down; replay/restart does not
inflate it. Two browsers agree. Scope/date changes reject old revisions.

### P3 — Character rates, sessions and Progress integration

**Files:** pure interval/rate functions and tests; sample/session SQL; rate-reset
store/handler/relay; `CharacterCard.vue`; focused Progress component/composable;
batched `character_analytics` stream; calculation documentation.

**Work:** Implement EXP/SP/net gold, known/unknown bot duration, coverage, ETA,
rolling 24h death/drop tiles, session table, calendar summaries and location
comparison. Use shared calculations; integrate card/detail surfaces. Implement
Reset Rates with durable scope/audit/idempotency. Investigate Training Behavior and
time-between-return source limits; show available portions and exact reasons.

**Exit:** A hand-computed fixture matches the browser; rollovers, losses, outages,
stale sessions and null fields behave as documented. Reset survives reload and
changes only that character's rate baseline. Multiple visible cards stay within
subscription limits and unsubscribe when hidden/unmounted.

### P4 — Rare/Normal Drop analytics and canonical details

**Files:** event population/aggregation helpers and SQL tests; profile-aware item
taxonomy integration; Analytics drop components; shared item-popup utilities only
where extraction is necessary.

**Work:** Time/group/location and type/degree charts, totals/top-character/type
shares, source selector and bounded evidence table. Preserve stored rarity/seal/
observed blues; reuse `ItemDetailPopup` and `itemRecordFromActivityEvent`. Factor
common population semantics with Events where useful, without changing its existing
combined-feed behavior silently. Add unknown-taxonomy and classification-version
coverage. Keep world observations, owned gains and transfers distinct.

**Exit:** Chart sums, headline counts and table/drill-down populations reconcile.
Pet → bag transfer adds zero acquisitions. An associated drop and gain do not
become two unique claimed drops. Unknown degree/rarity stays visible as unknown.

### P5 — Economy analytics over actual balance history

**Files:** character/guild balance query functions/tests; Economy analytics region
components; existing guild purge handler/store/tests if the deletion scope grows.

**Work:** Gold balance series with Character/Guild Storage filters, signed daily
changes, historical leading balances and coverage. Reconcile multiple guild
observers and stale/conflicting values. Implement stall-sale aggregates only if a
verified canonical sale source exists or P0 establishes a bounded source extension.
Keep stall capability/reason visible and leave the acceptance gate open otherwise.

**Exit:** Two observers of one guild do not double its gold; a current balance is
never backdated; spending appears as negative net change. Deletion is exact-scope
and invalidates history correctly. No inferred price/revenue appears from chat or
inventory delta evidence. Do not build Slice 13's full offers/search/stall product.

### P6 — Academy analytics with proven lifecycle context

**Files:** academy source context in plugin/event validation only if needed;
analytics membership queries/projection/tests; Academy analytics components;
protocol/capability docs and plugin-version guard artifacts when changed.

**Work:** Add server/academy/member/observer context to new records after verifying
the identity contract. Keep legacy records usable but context-limited. Deliver
recorded joins/departures, observed academy counts and member timelines. Implement
graduate counts/duration or bans only with proven events. Test observer overlap,
ID reuse, unavailable resource gaps, academy switching and reconnect baselines.

**Exit:** No fabricated graduation/ban appears. First-seen members have unknown
join duration. Unique totals are deduplicated only by a proven identity; otherwise
label observer-specific counts and document the unresolved reference summaries.

### P7 — Alchemy Statistics and item-run history

**Files:** alchemy analytics query/calculation modules/tests; `pages/alchemy.vue`;
focused sessions/statistics components; shared live filters and sidebar subtabs.

**Work:** Preserve existing attempts while adding Sessions/Statistics route state,
historical attempt runs, character/item/type/subcategory/degree/date filters,
reset/paging, highest-plus and outcome counts, weekday chart and latest attempt.
Expose empirical outcome denominators and complete/ambiguous run coverage. Fill
target-plus summary controls only where cohort facts support the calculation.

**Exit:** Slot reuse/model duplicates/reconnect do not merge unrelated items;
unknown outcomes do not count as failures; identical replay contributes one
attempt. Item detail matches the stored canonical event. No theoretical chance
or invented elixir/material count is displayed.

### P8 — Resource bounds, recovery and historical load

**Files:** analytics performance tests; live invalidation/query cancellation tests;
retention/cache/rebuild code only as required; map/live regression tests.

**Work:** Measure 1/7/30/90-day windows over at least one million canonical events
and a representative sample fleet in a disposable database. Examine SQL plans and
pool/build saturation. Verify bounded result/frame sizes, null-gap series, Other
groups, late-event correction, failed-build recovery and cache invalidation on
source purge/reset. Finish necessary indexes or bounded build scheduling.

**Exit:** On a documented test host, warm common 30-day charts target under 1s,
cold/large bounded queries complete within the 3s query deadline, and simultaneous
chart queries do not delay map/character live delivery beyond its established
coalescing behavior. If those targets fail, fix query work or document measured
remaining limits; do not mark performance complete based on ten fixture rows.

### P9 — Complete end-to-end, visual and documentation gates

**Files:** `scripts/analytics_smoke.py` and `scripts/analytics_browser_smoke.mjs`
(proposed); simulator analytics scenario; CI; calculation/protocol/capabilities/
parity/README docs; `AGENTS.md` completion and resume ledger.

**Work:** Run the scenario and browser workflows below on an isolated test stack;
run focused and full checks; compare screenshots; fix findings. Record separate
implementation/simulator/reference/Windows-runtime/source-capability status.
Update every affected matrix row and documentation, and summarize unresolved
required facts with the exact condition needed to close each gate.

**Exit:** All implemented paths satisfy the acceptance matrix. Slice 12 remains
incomplete if required source/runtime/visual/performance gates are still open.
Finish independent work and report those gates honestly; stop at Slice 12.

## 8. Required test matrix and deterministic oracle

Use an injected clock in pure/query tests. Integration tests require a disposable
`TEST_DATABASE_URL`; a skipped PostgreSQL test is not a passing acceptance gate.

| Boundary | Required cases |
| --- | --- |
| Source/session | Multiple agents and sibling sockets; stale owner/generation; old session after reconnect; null/missing fields; out-of-order state; rejected resource revision; transition sampling; no five-Hz sample storm |
| Durability | Upgrade/restart; sampling transaction rollback; DB outage during ingestion/query; regular heartbeat recovery; no fabricated pre-install history; retention coverage; exact guild purge |
| Time/scope | `[from,to)` edges; same timestamp IDs; occurrence vs receipt; late replay; DST/month/year/ISO-week boundaries; wrong server/character/group; overlapping groups; signed cave regions and mixed profiles |
| Rates | Zero/short denominator; stationary state; negative EXP/SP/gold; single/multiple rollover; changed EXP requirement; profile switch; capped level; stale ETA; session/gap boundaries; reset races/idempotency |
| Events/items | Duplicate event IDs; mixed world drops/owned gains; linked drop/gain; pet→bag transfer; quantities; unknown rarity/type/degree; static-profile change; preserved plus/seal/blues; canonical evidence links |
| Economy | Two guild observers, conflicts, stale/no-gold source, spending/transfer, unavailable stall receipt; no sale inferred from inventory/chat/gold |
| Academy | Member first seen; join/leave/source unavailable; academy switch; ID reuse; observer overlap; legacy missing context; unverified role/graduate/ban; complete versus censored duration |
| Alchemy | Success/failure/null; replay; slot replacement; same-model items; reconnect; completion without attempt; unknown starting plus/target; weekday timezone and cohort exclusions |
| Transport/UI | Unauthenticated/cross-origin; filter bounds; oversized payload; stale revisions; reconnect recovery; page disposal; batched visible cards; cross-browser reset; loading/empty/limited/unsupported/error/recovered states |
| Load | Million-event queries; bounded series/table/Other; cancellation; slow browser; concurrent heavy charts and realtime map; retention jobs; no source ingestion starvation |

Minimum numeric oracle, independent of production implementation:

- One same-level character observed for 600 eligible seconds gains 1,000 EXP
  (requirement 10,000), loses 20 SP and gains 6,000 gold: **6,000 EXP/h**, **60%/h**,
  **−120 net SP/h**, **36,000 net gold/h**. Current EXP 2,000 leaves 8,000 to level:
  projected ETA **4,800 seconds** at that pace. Use adequate samples throughout,
  not two endpoints separated by an invalid ten-minute gap.
- First character has two deaths and second one death over a complete two-local-day
  request containing two eligible characters: total **3**, average **0.75 per
  character/day**. Changing the population to only characters with deaths is an
  invalid denominator shortcut.
- One world drop, one associated owned gain, then pet→bag transfer: world-drop
  population **1**, owned-gain population **1**, additional gains from transfer **0**.
- Two observers each report one guild's 100,000 gold: guild total **100,000**.
- Four alchemy attempts (two true, one false, one null): attempts **4**, successes
  **2**, failures **1**, unknown **1**, empirical known-outcome share **2/3**.
- An observed academy disappearance with no graduation/ban evidence contributes
  an observed departure, **zero proven graduations**, and an unavailable ban cause.

Production-worker scenario must authenticate, register fixture characters, emit
state and canonical callback events, repeat a batch, switch session, lose/recover
the connection and demonstrate persistence after backend restart. Long history
can be seeded directly into a **disposable** database for query/load tests; keep
that evidence separate from actual wire/callback end-to-end tests. Do not add
production options for backdating accepted character state.

## 9. Validation and acceptance evidence

Run focused package checks after each increment. Once integrated, run
`bash scripts/check.sh` with the pinned Node/Go toolchains and a disposable
PostgreSQL database. Its current gates include Go formatting/vet/race/build,
plugin unit tests, live-transport audit, frontend formatting/unit/lint/typecheck/
production build and Compose config validation. Run the production container and
analytics smoke/browser flow on an isolated Compose project with unique ports and
database; do not restart an operator's running monitoring stack for convenience.

Extend the source and browser transport audits to cover analytics reads; their
current endpoint checks alone will not detect a new `/api/analytics` polling path.
Keep existing agent, command, map/realtime-position, outage and recovery checks.
If a plugin change is necessary, update the version/protocol contract and run
`plugin/test_protocol_contract.py` plus the repository's version guard conventions.

Browser acceptance must exercise all five views, both Alchemy tabs and Progress:

1. Direct route load, client navigation, back/forward, tab/filter restoration,
   server scope and current group behavior.
2. Valid/invalid dates, DST range, character/guild/item selectors, chart grouping,
   paging, Other/unknown buckets and canonical evidence links.
3. Tooltip/value inspection through keyboard/touch as well as mouse; provide an
   accessible numerical table for every meaningful chart. Use integer count axes
   and unsmoothed count series; smoothing must not imply impossible negative counts.
4. Reset Rates confirmation/cancel/idempotency, cross-browser update and persistence,
   with unchanged event/24h totals and unrelated character data.
5. Empty data, history beginning now, incomplete coverage, unknown bot state,
   unsupported stalls/graduations, source errors, disconnect and recovered data.
6. Multiple visible Progress cards, rapid scope switches, filter revisions,
   unmount/remount cleanup and continued realtime Map responsiveness.

Capture same-screen/mode reference and local evidence at **1440×1000**,
**1280×800**, **390×844**, plus **2560×1315** for the supplied screenshot match.
Store local captures under `docs/evidence/slice12/`; compare chart/summary widths,
workspace gutters, control size, density, typography, palette and mobile overflow.
Record comparisons and remaining differences in `docs/reference-parity.md`.

For charts, use one small reusable client-rendered chart wrapper, with a numerical
table and stable sizing. Select and pin a maintained library after checking its
official docs/license, Vue/Nuxt integration, SSR behavior and accessibility; do not
hand-build an entire chart framework or adopt a second dashboard UI system.

Document real-runtime validation separately: phBot/plugin/protocol/Python versions,
observed balance cadence/rollover and state meaning, source callbacks and academy/
guild/alchemy context. Read-only observation can validate calculations; no bot
mutation or paid elixir consumption is authorized merely to generate test data.
Missing runtime evidence keeps the relevant gate open while simulator work finishes.

## 10. Initial blocker register

These are known evidence gaps, not permission requests or instructions to stop.

| Gap at planning time | Affected result | Condition to close |
| --- | --- | --- |
| No historical numeric sample table | Retrospective XP/SP/gold/botting before installation | Begin accepted durable sampling; older numeric history remains unavailable unless an authoritative existing source is found |
| No verified multi-level EXP mapping | Rate over skipped level transitions | Verify active-profile level requirements / exported table semantics and test against runtime observations |
| Partial bot-state semantics | Complete uptime and Training Behavior | Verify false/unknown meanings and explicit location/state facts; retain unknown coverage meanwhile |
| Teleport lacks return cause | Time between returns | Proven source identifying completed return trips, not only command intent or generic teleport |
| No stall transaction source/model | Revenue, transaction/price charts | Verified sale receipts/callbacks or versioned runtime fixtures and canonical durable ingestion; coordinate with Slice 13 |
| Academy roles/graduations/bans unverified; diffs lack academy ID | Graduate duration, owned/joined classification, unique lifecycle totals | Verified identity/role/lifecycle source with context and overlap semantics; never infer a graduation from level or disappearance |
| Alchemy identity/start/target/terminal context incomplete | Reliable item sessions and target-plus cohort metrics | Captured item continuity and complete cohort facts, or a documented limited empirical presentation |
| Current groups have no historical membership | Historical Per Group attribution | Use labeled current membership now; actual historical membership recording is needed for retrospective claims |
| Browser and Windows acceptance not performed by this planning pass | Visual/runtime completion | Run the listed comparisons and separately record runtime results |

At completion distinguish **implemented and verified**, **implemented with limited
source coverage**, and **blocked required capability**. Do not mark the five-tab
navigation alone, an empty unsupported chart, or skipped tests as Slice 12 complete.

## 11. Ready-to-use Luna implementation prompt

```text
Implement Slice 12 — Analytics in /var/www/phmon, following AGENTS.md and
docs/slice-12-implementation-plan.md. This request is scoped to Slice 12 only:
complete all unblocked packages P0–P9 and acceptance checks, then stop; do not
continue to Slices 13–15 or implement Automations as a detour.

Inspect Git status and recheck the plan's baseline first. Use the existing
checkout/branch; never create or switch to a worktree. Reuse durable character
sessions, canonical events/items, resource observations, map/profile metadata and
the single /api/live transport. Do not rebuild completed features. Work in small
sequential increments and keep concise progress updates and a current AGENTS.md
resume entry. Do not delegate unless the user separately requests it.

Deliver the reference-shaped five Analytics tabs, real Progress/session/rate
surfaces and Alchemy Statistics. Inspect the public demo in a browser before UI
work and compare the supplied analytics/Progress/Alchemy screenshots at the
documented viewports. Treat screenshots as layout evidence, not formulas.

Persist accepted numeric facts with session fencing; keep calculations in Go.
Use explicit time/server/character scope, timezone/DST rules, documented units,
metric-specific denominators, coverage and missing-data reasons. Never invent
income from balances, graduation from disappearance, sales from chat/inventory,
bot uptime from commands or item identity from slot/model. Preserve world drops,
owned gains, transfers and observed item details as distinct canonical facts.

Investigate unsupported required facts against official phBot documentation and
available runtime/authorized executable evidence; record precise blockers and
continue independent work. Do not make placeholders permanent or claim full
acceptance while required capabilities remain unresolved. Source records must
survive restarts; Reset Rates changes only its scoped baseline.

Use the production plugin worker with fake adapters and a disposable test stack
for end-to-end tests. Run PostgreSQL integration/race, frontend, protocol,
recovery, load and browser checks; a skip is an open gate. Separate simulator,
visual and real Windows/phBot evidence. Do not operate real characters, spend
elixirs, send external messages, merge, publish or restart the operator's live
stack just to demonstrate analytics. Update calculation/protocol/capability/
parity/setup docs and report exact verification and remaining gates.
```

## 12. Execution ledger

- [x] P0 — source/formula/reference contract frozen from official sources and supplied captures; interactive demo controls are a P9 gate
- [x] P1 — accepted durable samples and guild-gold history
- [x] P2 — Deaths end-to-end vertical increment
- [x] P3 — Progress, rates, sessions, summaries and baseline reset
- [x] P4 — Rare/Normal Drop analytics and shared details
- [x] P5 — Economy balance analytics and transaction capability evidence
- [x] P6 — Academy context and supported lifecycle analytics
- [x] P7 — Alchemy Statistics and defensible historical runs
- [ ] P8 — physical cache-cold and concurrent chart/map load acceptance (large/warm query targets passed)
- [ ] P9 — full tests, browser comparisons, docs and honest final gate status

Latest implementation checkpoint — 2026-10-07: migration 25 persists accepted
session-fenced character samples, guild-storage gold observations and rate resets.
P1–P8 have implementation and automated evidence: five analytics views, selectable
hour/day/week buckets, character Progress and reset, profile-aware drop taxonomy
with shared item details, character/guild balance history, academy-scoped
membership changes, empirical Alchemy Statistics, bounded response data, 90-day
retention and separate analytics build capacity.

Files include `server/internal/analytics`, migration 25, character/resource/event
ingestion and tests, authenticated live analytics/reset handlers, plugin 1.9.27,
the analytics/Progress/Alchemy UI, `scripts/analytics_smoke.py`, and updated CI and
capability/reference/calculation docs. `bash scripts/check.sh` passed against a
disposable PostgreSQL database with race tests, Go vet/build, all 222 plugin tests,
transport audit, 218 frontend unit tests, typecheck, lint (zero errors, 74 warnings),
format, production build and Compose validation. The isolated Compose worker smoke
passed a production plugin death callback through authenticated Analytics to chart,
summary, occurrence and drill-down evidence; a second live client returned the same
event ID. It also verified Progress rates and durable rate reset. The opt-in load
run seeded one million canonical events across 50 characters/locations and analyzed
the fixture before query planning: first 90-day query 1.636 s (3 s target), warm
30-day 676 ms, 7-day 166 ms and 1-day 47 ms (1 s targets). The local Compose stack
was left untouched.

Open evidence and source gates: no interactive public-demo session or same-viewport
Analytics/Progress/Alchemy screenshot comparison was available; no actual Windows/
phBot runtime was available. Concurrent chart/map delivery under the million-event
fixture has not been measured. Stall sale receipts, continuous/reconciled guild net
change, graduation/ban facts, return cause, a verified level XP-requirement table
and alchemy item-run identity remain unsupported. The available local game-data
profiles contain item/magic metadata and character portraits, not XP requirements.
Exact next action: inspect the public demo with an interactive browser and capture
Analytics/Progress/Alchemy at 1440×1000, 1280×800 and 390×844; measure analytics
queries alongside map/character delivery; validate sources on Windows/phBot; and
continue the remaining capability investigations without inferring missing facts.
Stop at Slice 12; do not start Slice 13.
