# Issue #90 — Slice 13 acceptance audit (PR #91)

Review scope: `feat/90-economy-item-analytics`, draft PR #91. This is a
test/evidence ledger, **not** a declaration of completion or release readiness.
The current re-review includes the fixes at commits `7bb50ee`, `deb6970`,
`86882ba`, `3845433`, `b24632a` and `5455418`.
Source-backed claims below are from inspected branch code/tests. A test's
presence is not an executed pass; check the current PR SHA's Actions logs.

## A — Duplicate Events and world-drop semantics

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Rare and Normal Drop defaults only include callback world drops | Implemented | `web/app/pages/events.vue`, `server/internal/events/store.go` | `TestDeathEventsAreDurableIdempotentAndScoped`; source/kind filters | Real representative callbacks and browser tab inspection |
| Owned Gains and Transfers separate from world drops | Implemented | `web/app/pages/events.vue`, `server/internal/events/store.go`, `server/internal/analytics/query.go` | `TestDeathEventsAreDurableIdempotentAndScoped`; `TestAnalyticsQueriesUseCanonicalEventsAndStableScope` | Reconcile representative live source observations |
| Historical linked callback/gain rows preserve both canonical events and exact IDs | Implemented | `server/internal/events/store.go`, `web/app/pages/events.vue` | PostgreSQL history-link count/cursor and exact-ID assertions in `TestDeathEventsAreDurableIdempotentAndScoped` | Shared verified pickup identity absent; historical heuristic links cannot be safely collapsed |
| Dashboard, map and notifications don't reinterpret owned gains as world drops | Evidence-limited | Existing downstream consumers; `server/internal/analytics/query.go` | Canonical analytics integration assertions | Full disposable browser and downstream-source regression audit |
| Source and time filters with consistent totals/cursors | Implemented | `server/internal/httpapi/event.go`, `server/internal/events/store.go` | Multi-page, source, kind, callback exclusion regression in `TestDeathEventsAreDurableIdempotentAndScoped` | CI must pass with PostgreSQL enabled |

## B — Storage/pet movements and reconciliation

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Guild storage observed only with scoped guild identity | Implemented | `plugin/PhMon.py`, `server/internal/events/store.go` | `ResourceEventDerivationTests` guild withdrawal/deposit/switch, `TestGuildStorageContainerReferences` | Real guild observer evidence |
| Same-sample and bounded delayed personal/guild/pet transfers | Implemented | `plugin/PhMon.py` | Python resource derivation transfer, partial and delayed fixtures | phBot snapshot scheduling evidence and packet cause confirmation |
| Mixed movements never become invented drops | Implemented | `plugin/PhMon.py`, `web/app/pages/events.vue` | Python ambiguous and mixed-container tests | Physical source combinations not exhaustively known |
| Pending gain/loss expiry, pressure, unavailable source and sampling gap | Implemented | `plugin/PhMon.py` | Python expiry, pressure, storage-unavailable, guild-unavailable, missing-sample, sampling-gap fixtures | Network/queue durability and live timing remain unverified |
| Reconnect, rejected session, leave, identity and guild switch | Implemented | `plugin/PhMon.py` | Python old-session and rejection tests plus guild-switch fixtures | End-to-end reconnection under Windows/phBot still open |
| Cross-observer guild resource identity | Evidence-limited | `server/internal/resources/search.go`, `plugin/PhMon.py` | `TestResourceSnapshotsPersistFencingAndFreshness` | Concurrent live observer revisions and timing |

## C — Provenance, deduplication and history

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Durable idempotent canonical event IDs/replay protection | Implemented | `server/internal/events/store.go` | `TestDeathEventsAreDurableIdempotentAndScoped` | Verify deployed version and ingestion against actual phBot |
| Callback does not prove pickup; heuristic `drop_event_id` remains unverified | Implemented | `plugin/PhMon.py`, `server/internal/events/store.go` | Legacy link, unrelated same-model activity, exact source IDs tests | Physical pickup proof remains unavailable |
| Historical item attributes only from corresponding event evidence | Implemented | `server/internal/events/store.go`, `docs/item-instance-evidence.md` | Linked/unlinked callback payload tests | Historical non-persisted attributes cannot be reconstructed |
| Packet pet receipt plus state-diff represents one physical acquisition | Evidence-limited | `server/internal/analytics/query.go`, `plugin/PhMon.py` | `TestAnalyticsQueriesUseCanonicalEventsAndStableScope` verifies limited count population | No verified common physical acquisition ID; never claim unique acquisitions |
| Count events separately from transferred/acquired quantity | Evidence-limited | `server/internal/analytics/query.go`, `docs/analytics-calculations.md` | Existing owned-gain quantity vs event count integration fixture | Dedicated canonical transfer count and transferred-quantity analytics still incomplete |

## D — Unified Item Search

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Current canonical containers, ownership, freshness and item detail | Implemented; screenshot correction verified locally | Canonical resources API; grouped `/items`; shared popup and character resource links | `web/tests/itemSearch.test.ts`; `scripts/item_search_browser_check.mjs`; [screenshot audit](reference/item-search-screenshot-parity-2026-10-15.md) | Latest remote CI / remaining real-runtime gates |
| Single chosen guild observer snapshot, no mixed revisions | Implemented | `server/internal/resources/search.go` | PostgreSQL resource and guild snapshot fixture | Concurrent competing guild observer acceptance |
| Repeatable-read count/page and canonical server-key cursor | Implemented | `server/internal/resources/search.go` | Display-name/server-key discrepancy fixture and two-page search | Cross-request resource update stability not proved |
| Item taxonomy/degree only when a grounded server profile is present | Implemented for mapped metadata / explicit exported namespaces; coverage limited | Per-row server-resolved metadata; dependent dropdowns in `/items` | Cross-profile model collision, unknown taxonomy and real category/subcategory/degree browser checks | Unmapped custom item families / profiles remain unknown |
| Historical item-instance copy details | Evidence-limited | `server/internal/events/store.go` | Event-specific payload assertions | No identity for unobserved historical copies |

## E — Item movement/history analytics

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Search → history → filtered canonical Events/source link | Source navigation implemented; exact-item history entry pending | Character resource links; canonical Events routes and `itemHistoryTarget` retained | Frontend history-link tests, real source-tab browser checks | Restore a reference-compatible history action; old per-row diagnostics removed per operator screenshot |
| Drop observations vs owned gains and observed quantity | Implemented, evidence-limited | `server/internal/analytics/query.go`, `web/app/pages/analytics.vue` | `TestAnalyticsQueriesUseCanonicalEventsAndStableScope` | Unique verified acquisitions cannot be inferred |
| Method/container-filtered item-history breakdown and timeline | Implemented, pending verification | `server/internal/analytics/query.go`, `web/app/pages/analytics.vue`, canonical Events store | `TestAnalyticsQueriesUseCanonicalEventsAndStableScope` asserts world/gain/transfer counts, moved quantities and source/destination/method filtering | PostgreSQL and UI/browser acceptance, real pet receipt overlap |
| Historical rarity, seal, plus and blues not borrowed across copies | Evidence-limited | `server/internal/events/store.go`, item presentation helpers | History event payload tests | Some historical source observations lack instance fields |
| Independent callback and gain must not double-count drops | Implemented | `server/internal/analytics/query.go` | Analytics integration world/owned fixture | More real pet and transfer combinations needed |

## F — Global Economy

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Classified WTB/WTS/WTT from retained Global chat with raw source links | Implemented | `server/internal/chat/offers.go`, `server/internal/httpapi/economy.go`, `web/app/pages/economy.vue` | `TestClassifyGlobalOfferRequiresExplicitPrefix`; chat integration tests | Real multilingual/ambiguous ad examples |
| Source-backed text, sender, type, server and bounded date filters | Implemented | Same files | `TestOfferFiltersFailClosed`, `web/tests/searchPage.test.ts` | Browser/stale and filter interaction verification |
| Structured item/seal/type/subcategory/degree filters | Evidence-limited / incomplete | Free text is retained without invented item IDs | `TestClassifyGlobalOfferRequiresExplicitPrefix` | Verified attributable item identity/parser for chat text |
| Cross-observer dedup of identical actual broadcast while retaining repeats | Evidence-limited | `server/internal/chat/offers.go` | Repeated-observer fixtures | Verified shared broadcast identity unavailable; count observer-message occurrences |
| Confirmed sales and realized prices | **Blocked** | No capability-backed source writer | None (intentionally unsupported) | Verified sale transaction/outcome and paid amount evidence |

## G — Stall/price history

| Requirement | Status | Implementation | Regression / evidence | Remaining gate |
| --- | --- | --- | --- | --- |
| Stall listing/visit/sell and realized gold, quantity and item | **Blocked** | `docs/phbot-capabilities.md`; no speculative pipeline | Not applicable without source evidence | Captured authenticated supported phBot/runtime or packet transactions |
| Asking price vs actual sale distinction | Implemented as limitation | `server/internal/chat/offers.go`, `web/app/pages/economy.vue` | Offer classifier + API tests | Cannot surface realized price history |
| Pricing graphs, completed-sale ledger | **Blocked** | Deliberately not implemented | None | Prove transaction source before adding schema/calculations |

## End-to-end and CI gates

- `bash scripts/check.sh` includes Go gofmt/vet/race tests/build, Python plugin
  discovery, transport audit, frontend formatting/unit/lint/typecheck/build, and
  Docker Compose parse. Inspect the Validation result at **latest** SHA; don't
  use passing checks from an earlier SHA.
- PostgreSQL integration tests require `TEST_DATABASE_URL`. A CI workflow
  that provides PostgreSQL may count as successful integration only if tests
  actually execute without skipping.
- **Browser parity is pending** at 1440×1000, 1280×800, and 390×844. No
  screenshots or interaction measurements were generated for this re-review.
- Real Windows/phBot evidence is pending for guild storage timing,
  repeated multi-container movements, pet pickup receipts, chat common
  broadcast IDs, and verified stall transaction/cash settlement.
- **Recommendation: NOT READY** until the actual latest CI result is green
  and the E, browser, and relevant real-source gates are completed. Keep draft,
  do not merge, deploy or operate live characters.

## 2026-10-15 targeted operator regression verification

The Item Search and Economy pages were missing their Nuxt API proxies; store and
query-helper tests had not covered that transport. Both proxies now exist and the
Stack CI job includes `scripts/search_proxy_smoke.py`. Authenticated live browser
verification passed at all three required sizes for both pages, with search,
filter/reset/cursor, no-store/auth/error status, empty/error/recovery and item
popup checks. Normal/Rare Drop popups expose source-labeled separate historical
item observations and preserve callback facts/counts. Exact-ID live navigation
and mobile tooltip opening were also repaired. See the [regression report](reference/item-tooltip-regression-2026-10-15.md).
These passes do not resolve the remaining acquisition identity, runtime or other
issue #90 capability/acceptance gates above.
