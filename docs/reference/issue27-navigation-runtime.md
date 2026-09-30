# Issue #27 follow-up verification — 2026-09-30

All live commands below were explicitly authorized by the operator. Fixture
checks ran in a separate Compose project and database. Production database and
credentials were preserved. No secrets or raw generated scripts are recorded.

## Interface and concurrency

At 1440×1000, 1280×800 and 390×844, the ordinary context menu contains one
navigation action naming the frozen character/count. Empty selection shows
“Select characters first.” Opening, checking, dismissing or cancelling review
sends no command. Review closes the menu, retains the prepared operation and
focuses the existing panel below the map. Enter selects the map center; touch
retains the selected-point action. The mobile menu clamps to X=182..382 in a
390 px viewport. Escape restores map focus. Target, menu and route updates keep
the same map element. No horizontal page overflow or browser errors were observed.

All four group POSTs began before the first response. In the final attempt their
start span was 1.6 ms; durable send timestamps span 5.9 ms. A seven-child deferred
unit test proves requests beyond four start before any response. A blocked-sender
Go race test verifies other workers deliver and reconciliation continues.

## Live runtime evidence

After the operator uploaded the plugin, all four fresh sessions reported protocol
8 and navigation support; the connected agent reported PhMon 1.6.0 and phBot
20.1.2. Version reporting establishes compatibility; it is not a file checksum.

| Attempt | Result | Geometry and arrival evidence |
| --- | --- | --- |
| nuker4, Hotan westward, 19:23:55 UTC | `start_script=true`; 12 validated steps | One remaining block crossed regions 23687/23686. Observed remaining counts 11→10→9→7→0. Durable completion 19:24:03.831955; fresh observed arrival 19:24:09. |
| Four nukers, Hotan paved road, 19:26:37 UTC | Four independent HTTP 202 admissions; nuker2 completed, three `path_not_found` results | nuker2 counts 4→3→2→1→0; fresh arrival 19:26:48 after completion 19:26:43.974051. Failures did not block this route. |
| Four nukers, second nearby road point, 19:29:36 UTC | Four independent HTTP 202 admissions; nuker4/nuker2 completed, nuker1/nuker3 `path_not_found` | Simultaneous independent routes; nuker2 counts 5→4→3→2→1→0. nuker4 arrived 19:29:47, nuker2 19:29:53, after completion 19:29:46.197693. |

[Sanitized command and route evidence](issue27-navigation-live-evidence.json)
contains exact command IDs, destinations, timings and reduced remaining-path
snapshots. The initial North Taklamakan attempts failed with `start_script=false`.
The first tests preceded confirmation of the deployed plugin. Later tests also
overlapped operator movement; the operator confirmed initiating the teleport to
Hotan. Those attempts do not isolate a code fault. The operator supplied an
`event_loop` ten-second warning and later reported that navigation seemed to be
working. API invocation latency and intermittent `path_not_found` remain observed
limitations with an unverified root cause. Do not attribute them solely to phBot
or claim four successful arrivals for an agent-issued batch.

Local browser artifacts, retained outside Git: `/tmp/phmon-followup-menu-single.png`,
`/tmp/phmon-followup-menu-group.png`, `/tmp/phmon-followup-review.png`,
`/tmp/phmon-followup-mobile-menu.png`, `/tmp/phmon-followup-1280.png`,
`/tmp/phmon-followup-fixture-seam.png`, and `/tmp/phmon-live-group-09.png`.
The last two show the dashed connected route treatment; the fixture capture is
clearly identified by its Issue27 fixture name and stale status.

## Automated evidence and review

Full `scripts/check.sh` passed under Node 24.20.0 with a fresh disposable PostgreSQL
database: Go race tests, 104 plugin tests, 100 frontend tests, formatting, lint,
typecheck and production build. Command/live/navigation fixture smoke tests passed.
The additional outdoor-seam smoke uses the production plugin worker, observes a
skipped-sample prefix, checks a single remaining block/connector across the seam,
then emits a separate arrival observation. Route publication adds no history rows.
Both navigation smoke modes are now in CI.

CodeRabbit's payload-budget ordering finding is fixed by sorting session views;
its duplicate-report database overhead is fixed before all ownership reads while
retaining final lifecycle/sequence fences. The requested all-target frontend
concurrency and five-second route recovery publication are preserved. The browser
recovery audit now waits for a controller readiness signal before logging into the
restarted backend, removing an old-backend-cookie race in the smoke harness.

## Callback watchdog investigation — 2026-09-30

The operator explicitly requested investigation of the ten-second `event_loop`
warning. Navigation currently invokes both path generation and script start on
the callback thread; accepted-command timings do not separate those APIs from
sampling or queue delay. Plugin 1.6.1 logs each navigation stage before/after its
call and reports the total/four slowest callback stages whenever a callback takes
at least 500 ms. Tests cover success, False/None returns, exceptions, redaction,
fast-callback silence and timing-report execution after an exception. Logs contain
no script text, API arguments or exception details.

The official [Events API](https://plugins.phbot.org/phbot-api/events) says the
callback runs every 500 ms. The [script command documentation](https://plugins.phbot.org/handling-script-commands)
explains interpreter locking and why sleeping inside callbacks blocks other
callbacks. The Paths/Script contracts do not document thread safety for
`generate_script`/`start_script`. PhMon therefore retains callback invocation while
collecting runtime evidence; moving those calls to a thread would require further
verification. The warning is not resolved merely by these diagnostics. A fresh
operator-installed 1.6.1 log is required to isolate the stage before a corrective
change can be verified.

### Confirmed callback stall and bounded generation — 2026-09-30

Operator-supplied 1.6.1 timing logs isolate `generate_script` on nuker1/nuker2
at 8077/8124 ms. Validation and source readback took 0 ms; `start_script` took
3/2 ms. Total callback times were 8086/8131 ms. nuker4 generation took 577 ms,
script start 4 ms and callback total 589 ms. Thus synchronous path generation in
our callback dispatch is the confirmed blocking stage. The generation latency
itself remains native API behavior, not a diagnosed remote-service failure.

Plugin 1.6.2 makes a narrow exception to the older callback-only API plan: only
`generate_script` runs on a dedicated bounded daemon thread. One generation slot
is shared across profile workers in a plugin instance. Transport stays separate,
and all validation, position reads and script mutations remain callback-owned.
Expiry, current identity/profile, session and generation epoch are checked again
before invocation. Teleport, disconnect, revocation and stop discard late results;
no callback joins or waits on a generator. Tests exercise a deliberately blocked
generator, continued sampling/result flushes, API thread identity, duplicates,
invalid results, lifecycle rejection and slot bounds across worker replacement.

Official docs do not promise native generation thread safety or GIL behavior.
The installed-phBot gate is therefore explicit: load 1.6.2, verify generation
completes while fresh position sampling continues, verify script start/arrival,
and check that no ten-second callback warning returns. Do not claim this runtime
gate passed based only on Python fixture threads. Exact next action: push 1.6.2
for operator installation and inspect its callback/position evidence, then finish
final-head CI and CodeRabbit without merging PR #50.
