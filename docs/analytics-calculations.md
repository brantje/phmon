# Analytics data and calculation contract

Version 1 describes Slice 12 calculations. It uses accepted backend facts, not
predictions about undocumented callbacks. A returned metric includes its value,
unit, UTC window, timezone when grouped by calendar, calculation version, source
coverage and a status. `available`, `limited`, `insufficient_history`, `empty`,
and `unsupported` are distinct. A null value with a reason is not a zero.

## Sources and boundaries

| Fact | Source of truth | Coverage limitation |
| --- | --- | --- |
| Character EXP, SP, gold, level, region, dead, botting | Session-fenced `character_metric_samples`, written in the same accepted character-state transaction | History starts with the first accepted sample after deployment; no old balance is recoverable from the current row. |
| Death, rare/normal world-drop, level-up, teleport, alchemy, item and academy membership occurrences | Canonical `activity_events` rows and their source metadata | Only observed occurrences are counted. A teleport is not proof of a return scroll. Drop classification can be unknown or profile-versioned. |
| Guild-storage balance | Accepted guild-resource observations with server, normalized guild, observer and resource revision | Current contents do not represent older values. Several observers can see the same balance. |
| Item type/degree/seal/plus/blues | Persisted event instance details and active versioned item metadata | Unknown fields remain unknown; static catalog data cannot recreate historical instance facts. |
| Academy membership | Canonical observer-sourced join/leave events; new plugin payloads add an academy ID when the accepted academy snapshot exposes one | Academy/member identity lifetime and role-code meanings remain unverified. Graduation/ban cause and exact join timestamps are not sourced. |
| Stall sale or transaction | None until a verified source is identified | Do not infer sales from chat, a gold delta, or item loss. |

The initial sampling interval is 10 seconds. Sample once at session start, then at
most once per interval, and promptly when level, region, dead, or known botting state
changes. Insert only after session ownership and state validation and in the same
transaction as the accepted state update. Sampling receipt time is server time; do
not trust a client timestamp for ordering. Keep unknown fields null. A new session
starts a new baseline. Do not sample high-frequency map position frames.

Migration 25 stores these observations in `character_metric_samples` and guild
balance facts in `guild_gold_samples`; `character_rate_resets` records the scoped
operator baseline and idempotency key. The server retains these samples for 90 days
by default (`ANALYTICS_RETENTION_DAYS`) and prunes in bounded batches. Existing
current character and resource rows are never backfilled. Guild gold is sampled
only from an accepted, observed resource payload with a valid nonnegative integer
`gold`, inside the same resource transaction and with observer/session/revision
provenance.

Two adjacent character samples are comparable only when they belong to the same
character/session, both metric values are present, the second time is later, and
their gap is at most 30 seconds. There is no rate over longer gaps. Each interval
contributes its exact elapsed seconds to that metric's coverage. A metric with less
than 60 seconds of eligible coverage is `insufficient_history`.

## Rates and intervals

For an eligible balance metric, `net units/hour = sum(endpoint_delta) * 3600 /
sum(eligible_interval_seconds)`. Keep negative SP/gold changes. They are net balance
changes, not proof of spend type, earned SP, farming income, or sale revenue. A
pooled fleet rate divides summed deltas by summed character-seconds and is labeled
per observed character-hour. It is not a fleet total per wall-clock hour. Calendar
buckets use an explicit IANA timezone and `[from,to)` UTC interval; calendar windows
use timezone boundaries, so daylight-saving days may contain 23 or 25 hours.

Same-level EXP uses the signed difference between adjacent EXP balances. A single
level rollover may add `previous_max_exp - previous_current_exp + next_current_exp`
only if the profile verifies those fields and no intervening loss is plausible.
Multi-level jumps require a verified profile-specific XP requirement table;
otherwise that interval is unavailable. Level decreases, changed profiles and
implausible rollovers do not yield fabricated positive XP.

XP percent/hour is eligible same-level EXP/hour divided by that level's verified
requirement. Projected level-up time uses fresh `max_exp-current_exp` divided by
positive comparable recent EXP/second, with at least 60 seconds of coverage. A
missing requirement, cap, stale state, nonpositive pace or incompatible rollover
returns null and a reason.

Known botting true/false time is accumulated only over short intervals where both
endpoints have the same known state. Unknown or changing intervals remain unknown;
command history is never used as an observed state. Session span, observed
character-connected coverage and known botting duration are separate measures.

Analytics event charts currently report occurrence counts by calendar bucket;
they do not claim per-observed-hour death/drop rates. The character Progress
death/drop tiles are rolling 24-hour counts and are unaffected by rate reset.
Reset Rates records a per-character backend baseline with operator/time/idempotency
metadata. It changes subsequent card-rate calculations for that character only; it
does not delete source facts or alter event totals.

## Canonical event aggregation

Events are counted by distinct canonical event ID and `occurred_at`; pages and
subscribers do not affect totals. Deaths per character per day divide by the eligible
selected character population, including characters with no death, times the local
calendar days in the request. That population is current until historical roster
facts exist and therefore reports limited coverage.

World-drop observations and owned item gains are separate metrics. A transfer is
never a new acquisition. A rare/normal type share uses only rows whose stored
classification and active profile taxonomy are known; degree shares separately
report known-type rows whose degree is unknown. The response reports unknown type
coverage and total rows. Degree/category 0 is not a fallback for unknown metadata.
Cross-character drop callbacks are observations, not proof of globally unique
physical drops. An associated owned gain is not added to the world-drop total.

Location means the event's recorded region and zone; current location is never used
to fill missing past location. Preserve server and dataset context. Current saved
groups are used only for explicitly labeled current-group breakdowns; members may
appear in multiple groups. Do not sum group totals into the fleet headline. Historical
group attribution is unavailable until group membership history exists.

Gold balances use the latest fresh observation within each bucket. Character and
guild balances have separate selectors. For a guild bucket, one latest sample is
selected across observers with observer ID as a deterministic tie-break; observers
are never summed, and conflicting values are not described as reconciled. Guild net
change remains unsupported until a continuous observer policy is verified.
Character signed balance deltas are shown as net change. No interpolation bridges
stale or missing samples.

Alchemy counts use unique canonical `alchemy.attempt` event IDs. `success=true` and
`success=false` count as successes/failures; absent/null outcomes count as unknown.
Empirical success percentage is successes divided by known outcomes and shows that
denominator. Highest plus is the maximum observed callback value. Weekday uses the
explicit timezone. Alchemy completion callbacks do not add attempts. Inventory
slots and equal item models do not establish continuing item identity. Item-session
and reach-target statistics require proven, complete continuity/start/target facts;
otherwise explain why those summaries are unavailable.

Academy join/leave counts mean observed membership changes. A disappearance or
level/type code does not establish graduation or a ban. First-seen members have
unknown join time. Plugin 1.9.27 / protocol 18 adds optional `academy_id` context
from the accepted academy snapshot; older events stay contextless. An academy ID
change emits a departure in the previous academy and a join in the new one. This
does not prove graduation, a ban, or lifetime uniqueness of member IDs. Graduate
duration and academy-wide unique totals need verified academy/member identity and
graduation evidence; until then report observed changes and limited/unsupported
status.

## Query bounds and response rules

All source reads are server-side and use authenticated operator and normalized
server/character scope. Requests are bounded to 366 days. Time series contain at
most 500 points using a returned hourly/daily/weekly bucket. Tables are paginated
(25 default, 100 maximum); a chart includes at most 20 groups plus Other. Totals are
computed before truncation. Unknown/null points remain gaps, not zero bars. Large
integral balances are serialized as decimal strings to avoid JavaScript precision
loss.

Progress reads at most 10,001 matching rows to detect truncation and returns the
latest 10,000, at most 100 session rows, and the leading 100 training locations plus
an `Other locations` sum. A result is marked limited when a bound is reached or the
latest accepted sample is stale. Analytics queries use a 2.5-second SQL statement
timeout and 3-second overall deadline. Their live builds use a separate single
concurrency slot from ordinary character/map subscriptions.

The opt-in million-event integration test ran on 2026-10-07 in a disposable
PostgreSQL 18.6 container. It seeded one million canonical death events over a
50-character, 50-location fleet, ran `ANALYZE`, then measured the first 90-day
request at 1.636 s and the warm 30/7/1-day requests at 676/166/47 ms. These meet
the 3 s large-query and 1 s warm-query targets. The first request was not a physical
cache-cold test: fixture insertion may leave table pages in the operating-system
cache. Concurrent chart/map delivery is a separate open measurement.

Every response shares one database snapshot and returns as-of time, normalized
filters, calculation version, per-source coverage/status, and truncation. Timeout,
unavailable source and insufficient history stay distinct from an empty supported
window. Out-of-window predecessor/successor samples can define rate coverage at a
boundary, but never add event counts to the requested window.
