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

Current-level XP percent/hour is eligible same-level EXP/hour divided by the latest
sample's positive observed `max_exp`; every included interval must have the same
level and the same requirement at both endpoints. Projected level-up time uses
fresh `max_exp-current_exp` divided by positive comparable current-level EXP/second,
with at least 60 seconds of coverage. A missing requirement, cap, stale state,
nonpositive pace or incompatible rollover returns null and a reason. Multi-level
and rollover-derived EXP remain unavailable without a verified, versioned
requirement source.

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

Gold balances use the latest fresh observation within each bucket. Character economy
balances combine inventory gold from `character_metric_samples` with personal storage
gold from `character_storage_gold_samples` observed at or before each inventory
sample time. Before the first bank observation, charts and rankings use inventory gold
only. Guild balances have a separate selector. For a guild bucket, one latest sample is
selected across observers with observer ID as a deterministic tie-break; observers
are never summed, and conflicting values are not described as reconciled. Guild net
change remains unsupported until a continuous observer policy is verified.
Character signed balance deltas remain inventory-only net change. No interpolation
bridges stale or missing samples. Latest inventory samples are selected before bank
lookups; bucket balances also supply scope counts and rankings. The navbar combines
character/personal-bank balances and distinct guild balances, while Economy shows
those scopes separately. Gold bars use absolute balances from a zero baseline;
line charts offset exact integer values before numeric conversion to preserve small
changes, with original balances retained in labels, tooltips and accessible data.
Economy snapshots may be shared for at most five seconds from their original AsOf,
keyed by the full normalized filter and bounded to 16 entries; timestamps are not
rewritten and failed reads are never cached. Analytics has a separate one-connection
read-only pool so ingestion commit stalls do not consume its acquisition budget.
Retention and rate resets continue on the ingestion pool.

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

## Review repair additions — 2026-10-07

The Rare/Normal source selector is either **world-drop observations** or
**owned-item gains**. World mode counts canonical `drop.rare` / `drop.item` rows.
Owned mode counts classified `item.acquired` and `item.quantity_increased` rows
only from accepted state-diff **inventory** destinations or the validated pet
inventory receipt source. Pet state-diff positives are retained as canonical
Events but excluded from this analytics population: there is no common
acquisition ID to prove whether a pet inventory delta and a receipt describe
the same pickup. It never includes `item.transferred`. The headline is a
limited **gain-evidence event count**, and quantity is the sum of eligible
`quantity_delta` payloads. Neither number establishes distinct physical
acquisitions, and unreceipted pet gains are deliberately not included. Evidence
rows retain their canonical event kind and destination details.

Progress includes daily and ISO-week summaries over the seven days ending at the
request's `to` time, in its selected IANA timezone. EXP gains include only
same-level intervals with the same observed `max_exp` requirement and nondecreasing
current EXP. SP and gold remain signed net balance changes. Each metric returns
its own eligible seconds; intervals crossing a local day/week boundary are not
assigned to that bucket. The summary also counts canonical deaths and world drops
by occurrence time. A zero event count means no matching canonical records were
stored, not proven inactivity. Large balance deltas are decimal strings.

The seven-day location comparison attributes sample deltas only when both interval
endpoints have the same recorded region and zone, are in the same session, and
remain within the 30-second interval bound. EXP/SP/gold denominators are reported
separately. Death/drop counts use their own canonical event region/zone; they do
not establish that the character was farming at that location. Results are limited
to the 20 locations with the most eligible observed coverage.

Alchemy Statistics counts unknown outcomes separately and labels each weekday's
empirical success percentage with its known-outcome denominator. The Sessions view
may show bounded candidate attempt segments from the current page when character,
session, slot, stable item traits, and a gap of at most one minute match. These
segments are explicitly ambiguous and page-local. Slot/model/fingerprint similarity
does not prove continued physical item identity; they are not used to calculate
chance-to-target or attempts-to-target.

The reusable Chart.js view fills missing occurrence buckets with zero counts and
keeps missing economy observations as null gaps. Local-hour bucket generation
follows timezone transitions: skipped spring hours stay absent and repeated
fall-back wall hours coalesce just as the SQL grouping does. Count axes use integer
ticks. The keyboard/touch range input and expandable numerical table expose exact
returned strings; gold plot coordinates are offset from the exact lowest balance
before conversion to chart coordinates so small changes around large balances are
not erased by JavaScript's integer precision limit.

## Slice 13 event and economy semantics — issue #90

**World drops:** An observed `drop.item` or `drop.rare` **callback event** is
one world-drop observation. Counts are event counts, not item quantities or
successfully collected items. A callback and its linked state-diff inventory
observation count as **one world drop**, because the latter has another kind.
Uncollected drops still count as observed world drops.

**Owned gains:** `item.acquired` and `item.quantity_increased` describe
observed positive container deltas, with `acquisition_method: unknown` unless
separately verified. `item.transferred` is not a gain or world drop.
The existing `owned_gains` analytics view reports **recorded gain events**
from inventory state-diff observations and validated pet receipt sources, not
verified distinct physical item instances. Unlinked pet state-diff gains remain
visible in Events but not in this count; otherwise a single pet receipt and its
state delta would inflate it. If the pet receipt is absent the analytics metric
may undercount, and if the same physical acquisition is independently recorded
later in a bag with no proven transfer, it may overcount. Consequently exact
net physical-acquisition counts are **not established**. The metric status is
limited, not a verified acquisition count. `quantity_delta` is a separate item-count
field and must not silently replace the occurrence count.

**All Events logical projection:** A linked state-diff inventory
observation remains a distinct visible occurrence alongside its world-drop
callback. Even an explicit `drop_event_id` is retained for audit rather than
used to collapse rows or infer pickup causality. All Events totals and cursors
count the distinct canonical events matching the active filters. Exact event-ID
and event-kind searches also retain the original records. Temporal
co-occurrence or same-model similarity cannot justify fusion or copying
item-instance rolls.

**Transfers:** `item.transferred` requires observed source decrease and
matching destination increase (including guild-scoped shared storage), with
`quantity_delta` recording moved units, not sale units or net acquisitions.
Uncertain or unavailable container evidence must not create transfers or
world-drop events. A guild storage observer is not an owner of guild items.

**Global economy:** WTB/WTS/WTT messages are advertisements, not executed
trades. One classified retained Global chat message is one observed offer
occurrence. Independent character observations remain separate: sender, server,
raw chat type, text and a 500 ms window do not prove one server broadcast and
would conflate two legitimate repeated advertisements. A shared broadcast ID
is unavailable, so distinct physical advertisements cannot be reliably
deduplicated. The denominator is retained observer-message occurrences, not
globally unique advertisements or sales.
There is no verified realized-price or completed-sale denominator.

**Delayed reconciliation:** The six-second bounded pending window is for a
single compatible, opposite container delta. When a complete same-scope
observation expires without its counterpart, the independently measured net
increase/decrease is finalized with its original observation timestamp and
unknown cause; overflow also releases the oldest observation. No same-model
callback is now auto-claimed as a pickup. Sampling gaps, new/unavailable
containers and identity/session transitions cannot establish missing source
facts; resource snapshots remain the audit evidence when provenance is
incomplete.

**Historical policy:** Do not delete, merge or retroactively reclassify
ambiguous stored events. Query fixes apply read-only. An older event that lacks
explicit correlation remains independent, even if it resembles a drop.

## Legacy world-drop links and All Events totals

A pre-existing `drop_event_id` field in an inventory state-difference event is not verified pickup causality: the older producer used a heuristic. All Events lists the callback and inventory evidence as distinct occurrences rather than suppressing a potentially unrelated gain. Historical callback rows expose the stored unverified link for source auditing but never borrow the gain's historical item-instance attributes. World-drop analytics still count only callback occurrences; owned-gain analytics still count limited supported gain observations; transfers remain zero new acquisitions. An exact physical-acquisition count across sources is unsupported without an independently verified shared receipt identity.

## Canonical Item History (issue #90 section E)

The `item_history` view uses the same canonical `activity_events` rows as
Events and the existing read-only PostgreSQL analytics transaction. Search by
server, character, item name/code, local-date range, source/destination container
and recorded movement method, with count-by-character/item/location breakdowns,
occurrence timeline, cursor pagination and source-event links. Only source
payload properties are filterable: a missing container or method never becomes
an inferred value.

The summary keeps **world-drop callback occurrences**, **limited owned-gain
observations**, **unknown-origin gains**, **net quantity increases/decreases**,
**transfer event occurrences**, **transferred quantities**, **validated pet
receipt occurrences**, and **rare/normal classified occurrences** separate.
Transfer quantity means movement of already held items, **zero acquisitions**.
A validated receipt count cannot safely be added to state-diff gains as a
unique pickup count: no independently verified common physical acquisition ID
exists. The `distinct_acquisitions` metric is therefore explicitly unsupported.
The historical event population, daily buckets, metrics and paged occurrences
are evaluated under one repeatable-read transaction, subject to actual
monitoring coverage/retention. All Events keeps legacy heuristic callback/gain
rows separate rather than silently collapsing them.
