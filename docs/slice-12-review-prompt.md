# Slice 12 review and Luna repair prompt

Review date: 2026-10-07. Reviewed PR [#80](https://github.com/brantje/phmon/pull/80)
at `b7542adc6d21316ea923f1bbb84f0c029a3a2255`, against
[`slice-12-implementation-plan.md`](/var/www/phmon/docs/slice-12-implementation-plan.md).
The instructions below are the ready-to-use repair prompt. Final recheck: PR #80
and the checkout advanced to `3fca132` on `codex/slice12-review-fixes`. A concurrent
task addressed all five CodeRabbit comments in that commit; it also includes
`e47f737` for the existing TradeNexus work. Recheck these commits before starting:
items 2 and 17 and the empty-wake portion of item 3 are already addressed and are
verification notes, not instructions to duplicate those fixes.

Work in `/var/www/phmon`. Read `AGENTS.md`, the Slice 12 plan, and this review.
Fix the still-valid findings below and finish the omitted, unblocked Slice 12
work. Scope this request to Slice 12; do not begin Slices 10/11 or 13–15.
Recheck the current branch and code before changing anything. Use the existing
checkout; do not create a worktree or delegate. Preserve the pre-existing
TradeNexus changes and other unrelated edits. Do not merge, publish, operate real
characters, send external messages, or restart the operator's running stack.

The review reran PostgreSQL-backed race tests for analytics, characters,
resources, events and HTTP API against an isolated PostgreSQL 18.6 container.
Those tests passed, as did all 218 frontend unit tests and 222 plugin tests.
Eleven additional Go regression probes initially failed, and a plugin probe
demonstrated four invented academy membership changes. After `3fca132`, the
retention and empty-wake probes pass; the other nine Go probes still fail. The
analytics and HTTP API PostgreSQL-backed race suites were also rerun and pass at
`3fca132`. These probes used temporary Go overlays rather than editing application
code. Their files and output are under
`/tmp/phmon-slice12-review/`; the reproducible cases below remain the repair
contract if those temporary files are unavailable. Browser/Windows execution
and physical cache-cold/concurrent load acceptance were not verified by this
review.

Address correctness and transport failures first:

1. **[P1] Validate dates before changing a live subscription.**
   [analytics.vue:193](/var/www/phmon/web/app/pages/analytics.vue:193) checks only
   `from > to`. Clearing either date sends a missing required bound; ranges over
   366 days also get sent. The backend rejects these filters and closes the shared
   socket in `liveClient.subscribe`. Reconnect resends the invalid subscription,
   affecting the entire app. The Alchemy Statistics watcher also lacks the invalid
   range guard. Validate complete dates, ordering and the actual normalized UTC
   span before subscribing, show a specific inline error, and retain/recover the
   valid connection. Test clearing/typing dates, excessive ranges, inverted Alchemy
   dates and 23/25-hour DST boundaries without a socket failure.

2. **[Fixed locally; verify load] Make retention keep pace with ingestion.**
   At the reviewed PR head,
   [store.go:150](/var/www/phmon/server/internal/analytics/store.go:150) stopped when
   the combined deletion count is below 2,000. With no expired guild rows it stopped
   after deleting only 1,000 character rows. The initial probe left 1,500 of 2,500
   expired rows behind. Even without that early exit, ten batches every six hours
   could not remove a 50-character fleet's approximately 432,000 ordinary
   samples/day. Commit `3fca132` now continues while either table returns a full
   batch, uses a bounded cancellable time budget and schedules every 15 minutes.
   The regression probe now passes. Preserve this fix and verify cleanup indexes,
   sustained backlog progress and impact on ingestion at the planned fleet load.
   This incorporates CodeRabbit's retention finding.

3. **[P1] Preserve pending map snapshots across selective analytics passes.**
   [live.go:861](/var/www/phmon/server/internal/httpapi/live.go:861) clears
   `positionSnapshots` before determining which stream class this pass serves.
   An analytics-only pass discards a pending position subscribe/refresh; the
   following standard pass then skips it. This can occur when a position request
   arrives between flag capture and subscription capture. The regression probe
   reproduced the lost request. Consume only requests actually served in the pass.
   CodeRabbit's empty-wake fix is already in `3fca132`: zero invalidation flags
   remain zero and an empty wake skips builds. Its regression probe now passes;
   preserve the fix. The pending-position probe still fails. Test the race,
   explicit position refresh, empty wakes and revision changes.

4. **[P2] Complete analytics invalidation and isolate heavy work.**
   [agent.go:1089](/var/www/phmon/server/internal/httpapi/agent.go:1089) calls only
   standard invalidation after resources commit, although that transaction can
   insert guild gold history. Group mutations and guild-history deletion likewise
   do not invalidate analytics. Open charts can retain removed data or old group
   labels until an unrelated analytics event or manual refresh. Notify affected
   scopes after successful commits, including retention/purge, and preserve
   rejection/rollback fencing. Test another browser receiving the change without
   manually refreshing. Separate global build slots still feed one serial
   per-client snapshot loop: waiting/building analytics delays that client's
   standard snapshots. Cancel obsolete work and provide bounded scheduling that
   lets ordinary live delivery progress. Coalesce by affected scope so staggered
   samples from a fleet do not continuously rebuild every 90-day chart.

5. **[P2] Compute leaders from real entities before adding Other.**
   [query.go:532](/var/www/phmon/server/internal/analytics/query.go:532) sorts the
   aggregate Other bucket together with actual entities. `leaderMetric` then takes
   its first row. Fifty characters with one death each return “Other” as the
   character with most deaths; locations and drop leaders have the same problem.
   Query leaders independently or retain identity-bearing ranked leaders before
   presentation truncation. Use deterministic identity tie-breaks and test a large
   Other bucket plus same-name characters on different servers.

6. **[P2] Sort guild balances numerically.**
   [economy.go:139](/var/www/phmon/server/internal/analytics/economy.go:139) selects
   `gold::text` and orders by the output alias `gold`. A 900-gold guild precedes a
   10,000-gold guild and becomes the summary leader. Order by the underlying numeric
   balance; serialize to text afterward. Preserve server/guild identity and
   deterministic observer ties. Add the 900-versus-10,000 regression.

7. **[P2] Bound series without discarding the end of the selected range.**
   [economy.go:28](/var/www/phmon/server/internal/analytics/economy.go:28) and the
   guild equivalent limit all series together to 501 rows ordered oldest first.
   Query-level truncation then keeps 500. Ten characters over 60 days lose the most
   recent ten days despite each series having only 60 points. Bound series and
   per-series buckets explicitly, coarsen when necessary, and disclose omitted
   entities without dropping newer dates. Keep large integer values exact and
   totals independent of chart truncation. Also test a non-aligned exactly
   500-hour request: elapsed-duration checks alone can permit 501 calendar buckets.

8. **[P2] Include unknown taxonomy in degree chart reconciliation.**
   [query.go:243](/var/www/phmon/server/internal/analytics/query.go:243) uses a degree
   breakdown that includes unknown degrees for known types but drops rows whose
   entire taxonomy is unknown. One known D10 drop plus one unknown model returns
   total 2 and a displayed degree breakdown totaling 1. Keep distinguishable unknown
   buckets that account for the complete selected population. Do not turn missing
   degree into zero; reconcile chart, summary, evidence table and source counts.

9. **[P2] Do not invent academy changes when ID context disappears.**
   [PhMon.py:3525](/var/www/phmon/plugin/PhMon.py:3525) treats transitions between a
   numeric ID and `None` as an academy switch. An unchanged member list with IDs
   `7 -> missing -> 7` produces four join/leave events. Treat missing/invalid ID
   context as unavailable continuity and rebaseline conservatively; emit a full
   academy switch only when both different IDs are known. Test missing/invalid ID,
   source gaps, recovery, reconnect and a genuine `7 -> 8` switch. Keep observer
   attribution and server-qualified academy identity explicit.

10. **[P2] Calculate event tiles independently of numeric rate history.**
    [performance.go:78](/var/www/phmon/server/internal/analytics/performance.go:78)
    returns before querying canonical events when there are no metric samples.
    A character with one recorded death then returns `deaths_24h=0`. This affects
    pre-install history, outages and characters without fresh numeric observations.
    Preserve identity and rolling event counts while returning null/unavailable
    rates. Test supported recorded events with absent/expired samples and after
    Reset Rates; insufficient rate history must never manufacture zero activity.

11. **[P2] Keep live Progress windows current across browsers and resets.**
    [CharacterCard.vue:120](/var/www/phmon/web/app/components/CharacterCard.vue:120)
    freezes `to` when subscribing and moves it once a minute. New accepted samples
    trigger builds but remain outside that window. After another browser resets
    rates, its reset can be later than this client's `to`; the reset condition in
    [performance.go:109](/var/www/phmon/server/internal/analytics/performance.go:109)
    ignores it and returns the old rate as current. The initiating browser alone
    advances its window immediately. Establish a bounded live rolling-window
    contract or refresh affected clients' effective windows after reset. Preserve
    the semantics of genuinely historical requests. Verify immediate cross-browser
    reset, post-mount samples, reconnect and unchanged rolling event totals.
    Implement the planned batched visible-character performance delivery rather
    than one analytics subscription/timer/query per card. Test the planned
    50-character batch and cleanup against the 32-subscription limit; current Stats
    pagination at 24 cards is not evidence that the required batched contract exists.

12. **[P2] Fence current-level pace to compatible XP requirements.**
    [performance.go:225](/var/www/phmon/server/internal/analytics/performance.go:225)
    checks the requirement only within each adjacent pair, then combines intervals
    from different requirements. XP% and ETA divide that pooled pace by the latest
    requirement. The same-level probe changes requirement 1,000 to 10,000 and pace
    1 to 10 EXP/s; the returned current pace is 198%/h instead of 360%/h. Restrict
    current-level projections to the latest compatible requirement/profile history
    with enough eligible coverage. Persist/fence source profile/version when needed.
    Reconcile capability documentation, which currently says XP% is unavailable,
    with the implementation that enables it from observed `max_exp`. Verify and
    document current-level field semantics separately from unsupported rollovers.

13. **[P2] Preserve integer precision before calculating rates.**
    [calculate.go:96](/var/www/phmon/server/internal/analytics/calculate.go:96)
    converts each balance to float before subtracting. A one-gold gain from
    `9007199254740992` over 60 eligible seconds becomes delta 0 and rate 0 instead
    of delta 1 and 60 gold/h. Subtract/accumulate exactly before converting a bounded
    rate to float. Audit current/max EXP and other integral fields serialized as
    JSON numbers; use decimal strings where required by the calculation contract.

14. **[P2] Restore scope, pagination and canonical evidence links.**
    The Analytics page has no character/group or guild selection despite backend
    fields, and it does not restore dates/grouping from shareable route state.
    [analytics.vue:260](/var/www/phmon/web/app/pages/analytics.vue:260) fails to reset
    occurrence pagination when item type/degree change. Filter changes can keep a
    cursor from another population. Centralize filter validation, route restoration
    and cursor reset, including browser back/forward and balance scope.
    [analytics.vue:281](/var/www/phmon/web/app/pages/analytics.vue:281) links to Events
    with only kind/name: the destination defaults to seven days and drop tabs also
    include owned gains, so a 90-day world-drop chart drills into a different
    population. Preserve server, stable character ID, exact time bounds and source
    flags, and offer exact canonical event evidence. Map links need server,
    character and verified region/floor context through existing map helpers,
    rather than an event ID on the default server. Test old events, duplicate names
    across servers and signed cave locations. Coverage queries must use the same
    character/group/source scope instead of another character's oldest sample;
    return promised denominators and retention/coverage metadata explicitly.

Finish the plan's missing deliverables without inventing unavailable facts:

15. **[P2] Complete the supported P3/P4/P6/P7 workflows and reference composition.**
    Hour/day/week bucket selection is not a performance summary, location duration
    alone is not farming-location comparison, and the Sessions tab is still a flat
    alchemy attempt feed. Implement daily/weekly performance summaries and location
    comparisons from eligible gains, observed denominators and canonical counts.
    Add the separate owned-gains population/source selector required by P4 while
    excluding transfers. Implement conservative alchemy attempt segments using
    captured character/session/slot/item fingerprints where defensible, mark
    ambiguity, and break on replacement/loss/reconnect/unavailable continuity.
    Do not claim persistent physical item identity or complete target cohorts.
    Expose known/unknown outcome counts, denominator, latest attempt and required
    character/item/taxonomy/date controls on Statistics itself. Currently its
    date controls and error/stale UI live only in the other tab; failures can look
    like supported empty data. Academy grouping is supported by Go but absent from
    the UI; expose supported observer/member evidence without claiming graduation.
    Restore the reference's independent chart controls and arrangement: drop time
    chart on the left, type/degree chart on the right, known type shares in summary;
    Economy's balance/sales areas; Alchemy's inset summary strip and wide weekday
    chart. Preserve unavailable reference positions with precise reasons where a
    source is blocked. Extract focused components rather than expanding the large
    page/card files further. Reopen P3/P4/P7 ledger claims until their exit criteria
    have actual evidence.

16. **[P2] Use the planned accessible chart wrapper and finish P8/P9 gates.**
    [AnalyticsChart.vue:50](/var/www/phmon/web/app/components/AnalyticsChart.vue:50)
    creates fractional count ticks (`3 / 2 = 1.5`), draws every series as one flat
    sequence, spaces omitted dates as if contiguous, and exposes bar values only
    through non-focusable SVG titles plus a hidden table. It has no keyboard/touch
    value interaction and cannot represent null gaps. Follow the plan's maintained,
    pinned client chart-library requirement, keep integer count axes, explicit
    percentage/gold units, stable time coordinates, real series/legends, zero
    occurrence buckets and null missing-balance gaps. Expose an accessible numerical
    table and mouse/keyboard/touch inspection. Finish the same-screen viewport
    comparisons at 1440×1000, 1280×800, 390×844 and 2560×1315. Extend both transport
    audits to detect analytics HTTP reads while permitting the reset POST; current
    audits still cover only agents/characters/groups. Add the omitted browser
    workflows and meaningful replay/session/restart/outage/reset regressions to
    the smoke. Run the million-event and representative numeric-sample fleet load
    tests, actual cache-cold evidence, and simultaneous analytics/map/character
    delivery, including one socket carrying both stream classes and obsolete-query
    cancellation. Keep Windows/source gates separate and open when access is absent.

Preserve and validate the other CodeRabbit fixes already committed concurrently:

17. **[Fixed locally] Correct protocol and accessibility references.**
    In [phbot-capabilities.md:27](/var/www/phmon/docs/phbot-capabilities.md:27), plugin
    1.9.27 uses protocol **18**, not 17. In
    [protocol.md:1516](/var/www/phmon/docs/protocol.md:1516), canonical kinds are
    `academy.member_joined` and `academy.member_left`, not `academy.joined/left`.
    Do not rewrite accurate historical protocol-17 planning baselines. In
    [CharacterCard.vue:637](/var/www/phmon/web/app/components/CharacterCard.vue:637),
    training/session heading IDs and both `aria-labelledby` attributes need a
    per-card `useId()` value. All three fixes are already present in `3fca132`.
    Check them in the final combined state and verify multiple Progress cards;
    do not reapply the original patches.

All five CodeRabbit inline findings were valid against the originally reviewed
head and have now been addressed by the concurrent `3fca132` commit. Retention
is consolidated into item 2, invalidation into item 3, and its other three
findings into item 17. Original findings:
[protocol number](https://github.com/brantje/phmon/pull/80#discussion_r4211231232),
[event kinds](https://github.com/brantje/phmon/pull/80#discussion_r4211231254),
[retention](https://github.com/brantje/phmon/pull/80#discussion_r4211231267),
[empty invalidation](https://github.com/brantje/phmon/pull/80#discussion_r4211231277),
[heading IDs](https://github.com/brantje/phmon/pull/80#discussion_r4211231281).

Add regression tests for the repaired behavior, use an isolated database/stack,
and run the appropriate focused checks followed by `bash scripts/check.sh` with
the pinned Node 24.20.0/Go toolchains and real PostgreSQL integration tests. Keep
queries bounded and authenticated, commit facts before invalidation, preserve the
single `/api/live` socket, and maintain session/generation/revision fencing.
Update calculations, capabilities, protocol, parity evidence and the slice ledger
to match the final implementation. Report fixes and measured validation separately
from unresolved runtime/source/visual gates. Unavailable sales, graduation/ban,
return-cause, rollover or complete alchemy-cohort facts must remain explicit;
their absence does not justify omitting supported workflows or marking Slice 12
complete.
