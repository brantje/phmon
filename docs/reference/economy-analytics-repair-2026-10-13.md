# Economy analytics repair — 2026-10-13

The operator reported Economy missing multiple characters despite the navbar
showing over 200B gold. Authenticated read-only inspection reproduced three faults:

- Direct Analytics entry used the Stats-only `characters` feed for its selector.
  That feed was unsubscribed, so the selected Odin UUID had no visible option.
- The character economy query performed bank lookups on every inventory sample
  and scanned history separately for counts, buckets and rankings. The fleet's
  roughly 479,000 samples exceeded the three-second query deadline and returned
  `subscription.unavailable`.
- Gold bars subtracted the smallest balance as a numeric origin. A lone character
  or guild therefore became a zero-height bar with a misleading offset axis.

Analytics now populates its selector from the always-subscribed fleet feed.
The backend selects latest inventory samples per character/calendar bucket first,
then joins bank observations once per selected sample, reusing those balances for
scope counts and rankings. Daily net change groups comparable intervals before
counting character-days. Bank time fencing, inventory-only fallback, signed net
change, exact string balances and the 20-series limit retain their semantics.
Gold bars use zero as their origin; lines retain integer offsets for precision.
The page explains that the navbar combines character and guild balances while
Economy displays the selected balance scope separately.

At acceptance, 12 characters plus personal banks held 79,745,727,103 gold and the
observed iBot guild bank held 163,583,368,773 gold. Their sum is 243,329,095,876,
matching the navbar's rounded **243.3B**. The supplied URL explicitly selected
Odin; clearing Character now returns all 12 chart series. Historical gold samples
were preserved, including the existing October 9 balance discontinuity. This
repair does not infer its cause or rewrite earlier observations.

## Validation

- All `internal/analytics` PostgreSQL race tests passed on a separate disposable
  PostgreSQL 18.6 instance. Added checks cover bank timing, inventory-only fallback,
  combined rankings, count/leader preservation beyond the 20-series limit, and a
  480,000-sample / 636-bank-observation fleet query (1.94 s).
- Read-only live database query returned 12 characters / 72 daily points in 2.05 s.
- 254 frontend unit tests, Nuxt typecheck, changed-file ESLint (zero errors; four
  existing HTML void-element style warnings), Prettier and diff checks passed.
  Go vet/build and both production Docker builds passed.
- Authenticated production browser flow at 1440×1000, 1280×800 and 390×844: direct
  Odin entry, populated character picker, clearing the UUID filter, all 12 balance
  and time series, switching to guild storage, and no page overflow. No browser
  errors or unavailable analytics frames occurred. Screenshots were inspected to
  confirm visible bars for both single-character and single-guild balances.
- Recreated only the local Go/web containers. PostgreSQL remained running and
  `/readyz` and `/api/health` report healthy. No bot command or integration message
  was sent. The web build preserved the previously deployed navigation precision
  fix and excluded pending Players changes from its staged build context.

## Local evidence

- [Fleet desktop](economy-analytics-2026-10-13/fleet-1440.png)
- [Odin desktop](economy-analytics-2026-10-13/odin-1440.png)
- [Guild desktop](economy-analytics-2026-10-13/guild-1440.png)
- [1280×800](economy-analytics-2026-10-13/fleet-1280.png)
- [390×844](economy-analytics-2026-10-13/fleet-390.png)
- [Browser results](economy-analytics-2026-10-13/results.json)

This was a focused repair to an existing screen. No new demo comparison or full
roadmap acceptance is claimed. Live balances can change after these captures.

## Follow-up: recurring unavailable responses

The operator immediately reported the unavailable banner after the first repair.
A 25-second production observation reproduced two unavailable frames interleaved
with successful snapshots. PostgreSQL cancelled the signed-gold-summary query;
a concurrent `pg_stat_activity` snapshot showed all five ingestion connections
waiting on `WalSync` / `WALWrite` commits for approximately 5–7.5 seconds. The
original short viewport checks missed this recurring failure. The first repair
was insufficient under the observed database contention.

The running backend now reserves one additional connection exclusively for
analytics reads, with `default_transaction_read_only=on`. Ingestion, retention
and rate-reset writes retain their original five-connection pool. Analytics still
has one concurrent build slot and its original three-second request / 2.5-second
statement deadlines. JIT compilation is disabled inside analytics transactions.

Economy snapshots are reused for at most five seconds from their original `AsOf`
with a 16-entry bound keyed by the full normalized filter. Concurrent requests for
one scope share a read. Failed reads are not cached; a waiting client can cancel
independently. Snapshot timestamps are never relabelled as fresh. Other analytics
views retain their existing query semantics. Future database failures still report
unavailability; cached observations are not a substitute for persistent data.

The host filesystem was 96% full during diagnosis. Removing only unused Docker
build cache older than 24 hours reclaimed 4.945 GB; it was 92% full afterward.
No database volume, application record or running unrelated container was removed.

Validation: all analytics and HTTP API PostgreSQL race tests pass. New tests cover
an occupied ingestion pool, five-second expiry from observation time, scope
isolation, bounded entries, failed-read recovery, concurrent sharing and waiting
client cancellation. Go vet/build and the production server Docker build pass.
Only the Go container was recreated; PostgreSQL and web remained running.

Production acceptance ran two fleet Economy pages, an Odin-only page and a live
Map page concurrently for 121 seconds, with eight manual-refresh rounds. The
Economy pages delivered 79 / 76 / 72 snapshots with 22 distinct database `AsOf`
timestamps each; the map delivered 183 snapshots. All selected-scope totals were
correct. There were zero unavailable responses and zero browser errors, and
readiness remained healthy. This checks normal concurrent monitoring, not an
artificial disk-failure scenario. Evidence:
[concurrent results](economy-analytics-2026-10-13/concurrent-results.json) and
[fleet screenshot](economy-analytics-2026-10-13/concurrent-fleet.png).
