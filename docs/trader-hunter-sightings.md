# Trader / Hunter event history

The **Events → Traders / Hunters** tab reads the existing `job.hunter_trader_seen` canonical activity events. No new packet decoder, polling loop, ingestion table, or plugin callback is required.

## Source and semantics

The phBot `EVENT_HUNTER_SPAWN` callback includes both traders and hunters. PhMon saves the callback's spotted name under `payload.value`, alongside the reporting character, server, occurrence time, and the **reporter's** position. The callback cannot conclusively identify the spotted character's exact job or location. The history therefore uses a combined tab, and any map link explicitly points to the **reporter location**. Earlier uncollected or rejected callback occurrences cannot be reconstructed.

Every accepted event remains an individual occurrence, including repeated sightings of the same name. The existing canonical event-ID/replay protections remain unchanged. This feed does not include the independent AdvancedAutoTrade trade reports or Trade Nexus thief sightings.

## Filters

The new tab keeps the existing server scope, name, from/to date range, 10/25/50-page size, cursor pagination and live WebSocket event history. The default date range is the same as other non-drop Events tabs (the last seven calendar dates).

- Name searches the **spotted name or reporting character** only in the trader/hunter event kind. Other event kinds retain their existing reporter-name search.
- **Hide own characters** is enabled by default for this tab. The URL omits `hide_own` when checked and uses `hide_own=0` when unchecked. Reset restores the checked default.
- The hide rule also excludes a spotted name equal to the job alias an own character reported through plugin `character_info.job_name` (plugin 1.9.39+); characters whose plugin has not sent it are covered only by the name/observed-pair rules.
- Exclusion uses the **spotted name**, not the reporter, against registered PhMon character identities on the **same server**. Known directly observed player-name/job-alias pairs may also exclude own aliases. Unverified identity guesses are not sufficient.
- The Go REST and live filters implement `hide_own=true` only for `kind=job.hunter_trader_seen`. Omitting it from API requests keeps old behavior; other kinds cannot request the exclusion.
- Name, time, server and own filters apply before both total counts and cursor paging; there is no frontend-only filtering or duplicate suppression.

## Validation

The Go event integration test `TestTraderHunterHistorySearchHideOwnAndPagination` uses `TEST_DATABASE_URL` (PostgreSQL) and checks spotted-name matches, reporter matches, repeated events, own-character exclusion, server scoping, total counts and cursor navigation. The HTTP and live protocol tests reject malformed or mis-scoped `hide_own` requests. Frontend smoke checks should confirm: tab navigation, checked default, unchecking, URL reload/back behavior, Reset, a new live sighting, and truthful observer-location labels. Verify with an actual phBot callback separately; database fixtures are not runtime proof.

This feature should not change Drops, Thieves, Trades, or the general Events feed.
