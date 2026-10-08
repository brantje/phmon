# Persistent player registry, equipment and job identities

This focused feature implements the 2026-10-07/08 operator request under the
canonical [engineering guide](../../AGENTS.md). Work stays in the existing `main`
checkout, initially `721a486`. No merge, push, deployment, worktree or real-character
operation was performed. The remaining project roadmap is outside this focused run.

## Completion ledger

All independent implementation and local acceptance work is complete. **Full
runtime completion remains open:** no sanitized target-server other-player
packet/equipment/job-transition evidence was available. Disabled decoders and
synthetic acceptance do not satisfy that gate.

The follow-up [packet decoding and transition enablement plan](player-protocol-enablement.md)
now defines the pending capability-manifest, decoder, transport and automatic-rule
increments. The completion statement above applies to the original registry run;
it does not claim those follow-up increments implemented.

| Slice | Implemented and verified | Remaining target-runtime gate |
| --- | --- | --- |
| 1. Registry and Player pages | Migration 000026, five durable tables, field timestamps, alias-safe admission, bounded persistence, historical server scopes, Player after Stats, filters/sorting/cursors/URL history, basic profiles | None for existing verified `map.players` facts |
| 2. Research and capture | Opcode-by-opcode pinned primary-source research; local opt-in allowlisted bounded capture; synthetic envelope/reset/overflow tests | Sanitized exact-version single/group/multipart/equip/unequip fixtures; entity layouts and exact consumption |
| 3. Equipment | Normalized coverage/unknown/empty/occupied slots, enrichment/tooltips, decimal uint64 values, versioned fingerprints, partial merges, chronological and delayed A→B→A history | Actual target other-player slot/model/plus/instance-stat coverage; verified cross-mode comparison profile |
| 4. Job correlation | Session/epoch/incarnation lifecycle foundation; explainable review-only assessments; retained alias conflicts; durable thief import; serialized reversible associations | Actual normal→Trader/Hunter/Thief and reversals; stable identifier or validated transition rule; production transition candidate admission awaits the verified decoder |
| 5. Profiles and review | Canonical/source profiles, equipment/history, aliases/jobs, sightings/evidence, confirmed operator corrections/confirm/reject/link/unlink, map profile resolution | Actual live equipment/job presentation depends on the evidence above |
| 6. Hardening/docs | Production collector/worker simulator, disposable PostgreSQL integration, races/outages/replay/concurrency/retention, frontend/browser/regression/build checks, responsive artifacts | Separately recorded real phBot/client acceptance; no real character was operated |

- [x] All six slices' independent code, tests and documentation.
- [x] Existing collector, `map.players` wire semantics, protocol 18 and LiveStore TTL preserved.
- [x] Authenticated browser → proxy → Go → PostgreSQL flow and plugin simulator transport.
- [ ] Target-server player entity decoding and equipment/job evidence verified.
- [ ] Target-profile identity comparison and transition automatic linking enabled after evidence.

## Architecture and identity decisions

Existing `players.LiveStore` remains the live-map authority with its **35-second
TTL**. Persistence is submitted only after existing validation/session/generation
admission and LiveStore update. PostgreSQL failures cannot prevent live-map updates.
New code extends `server/internal/players`; there is no replacement collector or
separate broker. Migration **000026** adds `players`, `player_aliases`,
`player_observations`, `player_equipment_history` and `player_identity_links` plus a
canonical projection view. Server keys use lowercase trimmed existing-store
normalization. Confirmed active normal names are unique within their server;
equipment hashes are never unique.

An unclassified name is an observed alias. Its normal name and role remain unknown;
the table displays **Unknown** beside the recognizable alias. Unique server-scoped
aliases may be reused provisionally across observers. Contradictory model,
classification or simultaneous-presence evidence creates independent unresolved
records and a retained `alias-conflict-v1` candidate. Ambiguous aliases are never
resolved by arbitrary name selection. Runtime IDs remain scoped to observer
session/epoch/incarnation, not permanent identity keys.

Each field has its own observation timestamp. Missing values retain knowledge;
verified explicit emptiness may clear it. Late data can extend first-seen/history
without replacing newer facts. Item replacement invalidates previous instance
attributes. Partial equipment retains each missing slot/attribute's original time.
Latest unavailable attempts preserve earlier gear and expose its age separately.
Late attributes may fill earlier unknown fields only within the same observed
configuration; they cannot cross a known replacement/reversal boundary.
No catalog reference range is presented as an observed instance statistic.

Linking preserves every player UUID and its observation ownership. Confirmed
associations produce a canonical projection; `/api/players/{source-id}` resolves
to it, while `?source=1` keeps the original source inspectable. Source updates also
advance the canonical revision. Decisions acquire server transaction and ordered
player locks, validate revisions and reject cross-server, cyclic, conflicting or
simultaneously proven associations. Unlink appends an audit decision, revokes the
association and restores independent projections. Review requires a checkbox,
reason and authenticated operator. The existing single-operator authentication
records actor `operator`; it does not invent a multi-user identity system.

Routine sightings expire after **90 days**, configurable through
`PLAYER_OBSERVATION_RETENTION_DAYS` (1–3650). Retention deletes at most 5,000 rows
per batch, 12 batches per hourly run, with a one-minute run deadline. Player
records, aliases, decisions and meaningful equipment evidence survive. History
pins its original observations and copies its latest endpoint evidence so delayed
changes can split an interval after routine sightings expire. A→B→A retains three
intervals rather than globally deduplicating equal hashes. Thief reports are
idempotently copied by their durable IDs into pinned registry evidence before
original report retention may remove them. Observer fallback coordinates never
become thief coordinates; external reports cannot authorize automatic links.

## Persistence and capture bounds

First sightings and meaningful field/configuration changes are queued immediately.
Unchanged last-seen/location checkpoints occur at most once per 30 seconds per
observer incarnation. Consecutive unchanged pending snapshots coalesce; A→B→A
changes remain distinct. Source references deterministically deduplicate retries.
DB batches contain at most 128 observations. The worker flushes each second with
a three-second database deadline and retains bounded retries/status counters.

Queue limits are **4,096 records / 8 MiB**, with an **8,192-entry** recent checkpoint
cache. Overflow is counted, not silently described as durable. Legacy map frames
have no commit acknowledgement: pending memory is transient and may be lost on
backend restart. This limitation is explicit; no `player.observations` message was
introduced without a verified additional packet source. A future negotiated
message must provide stable IDs, bounded batches, post-commit acknowledgements,
session fencing and exact replay.

Plugin **1.9.29 / protocol 18** adds local Capture players (15 s) / Save player
capture controls. Capture is off by default, server-to-client and allowlisted.
The callback only admits copied bytes; worker processing/export performs no bot
command or injection. Bounds: 64 KiB per packet, 128 records / 2 MiB payload
(4 MiB hex plus bounded overhead), 30-second maximum duration, 256 KiB assembled
group / 128 entities. Session changes, malformed sequences and overflow invalidate
continuity. Missing/truncated captures never prove despawns. Exports are local,
explicitly **unsanitized**, not uploaded or committed runtime fixtures.

## Equipment and matching

`gear-v1` is a SHA-256 hash over deterministic sorted known slots and observed
persistent configuration. Only normalized model/plus/variance/magic fields enter it.
Durability/times/the enriched item presentation adapter are excluded, while
durability remains inspectable. Unknown, observed-empty and occupied are separate
states. Unsigned 64-bit variance/magic values remain decimal strings through JSON,
Go and TypeScript. Unknown item models retain observed facts without fabricated
metadata. Existing shared inventory defaults stay unchanged.

`identity-v1` is unavailable without a backend-verified profile and compatible
character model and matching comparison-profile version. It excludes
job/avatar/transient values and requires at least
four known normal slots; comparisons use overlap and reject conflicting observed
configuration. Gear equality alone never links records. Transition assessment
uses defaults **15 s / 30 world units / 60 s / four comparable normal slots** and
requires verified lifecycle, compatible model/level and explicit comparable
coordinate scope. Competing matches remain reviewable. Assessments are
uncalibrated, not probabilities; expiration does not remove retained candidates.

No target-profile identity hash or transition automatic policy is enabled.
Confirmed aliases remain reusable. No proven cross-mode stable identifier was
found. The lifecycle/assessment foundation is exercised synthetically and will be
connected to production packet evidence only after the decoder gate is met.

## API and browser behavior

All routes reuse operator authentication, mutation-origin checks, parameterized
queries, bounded timeouts and Nuxt proxy conventions. List/history/candidate
limits are 25 by default and 100 maximum. Cursors validate their UUID/date/key
and bind to filter/sort context. Registry cursors support both directions with UUID
tie-breaking; default order is last-seen descending.

| Method | Route | Purpose |
| --- | --- | --- |
| GET | `/api/players` | Server/name/level/guild/job/last-seen/identity/equipment filters, whitelisted server sorting, cursors |
| GET | `/api/players/servers` | Durable historical server options |
| GET | `/api/players/{id}` | Canonical profile; `source=1` original |
| GET | `/api/players/{id}/equipment` | Latest knowledge and attempt availability |
| GET | `/api/players/{id}/equipment/history` | Paginated meaningful intervals and endpoint evidence |
| GET | `/api/players/{id}/aliases` | Paginated classification/job history |
| GET | `/api/players/{id}/observations` | Paginated original observer/source evidence |
| GET | `/api/players/match-candidates` | Paginated pending/confirmed/rejected/revoked reviews, optionally player-scoped |
| POST | `/api/players/{id}/aliases` | Explicit reviewed classification/correction |
| POST | `/api/players/links` | Confirm/reject/manual association |
| DELETE | `/api/players/links/{id}` | Reviewed audited revocation |

The URL owns filters, sorting and cursor. Text inputs debounce 250 ms; changes
reset pagination and abort obsolete requests. Reload/back/forward are supported.
Explicit URL server scope takes precedence over saved scope. Profiles carry a
validated internal return URL. Other-player map resolution is batched per server,
omits ambiguous aliases and preserves runtime marker IDs, deduplication,
coordinates and freshness. Unknown/partial/unavailable/loading/empty/error/retry
states remain explicit. Compact filters/tables and profile controls use the
existing dark/gold shell and remain keyboard accessible and responsive.

## Validation and reproducibility

Validated using **Go 1.27.1**, **Node 24.20.0**, **PostgreSQL 18.6** in a disposable
loopback Docker container, and **Playwright 1.58.2** installed outside the repository.
No production container, database, character or outbound recipient was used.
The test backend/frontend and disposable container/volume were removed afterward.
[Machine-readable local acceptance](../reference/player-registry/acceptance.json)
records the simulator/browser/restart results and final Go hardening checks.

- `bash scripts/check.sh` under Node 24: gofmt, vet, full Go race tests and builds;
  228 plugin tests; live transport audit; Prettier; 230 frontend unit tests;
  ESLint (zero errors, nonblocking warnings); typecheck; Nuxt production
  build; Docker Compose validation. `TEST_DATABASE_URL` enabled disposable DB
  integration instead of skipping it.
- PostgreSQL tests cover first/repeated/multiple-observer sightings, server/name
  conflicts, per-field late/missing updates, restart/new-store reads, replay,
  all filters/sort/cursor directions, partial/equipment reversals, late per-field
  attributes bounded by observed replacements, a committed A followed by queued
  B→A inside the checkpoint window, alias corrections,
  source preservation, retention, manual/unlink/stale/concurrent decisions and
  canonical map resolution. Network refusal then recovery tests keep live state
  independent and commit retained retries exactly once.
- 1,000 unchanged moving snapshots coalesced into **one pending record (669 bytes)**.
  Repeated DB snapshots obey the 30-second checkpoint bound. An outage/high-change
  workload stays within 4,096 / 8 MiB; a 128-record first/change batch measured
  **443.69 ms** under local race instrumentation (a local measurement, not an SLA).
- `scripts/player_registry_smoke.py` imports the production plugin collector and
  worker, authenticates two fixture agents, verifies both observer attributions,
  disconnect and backend-restart persistence, reconnect/new session and runtime-ID reuse producing
  three independent records. It invokes no real phBot API or bot command.
- `server/testsupport/player-registry` seeds clearly labeled equipment/role fixtures
  only into a loopback `player_registry_*` database. It is not a production feed.
  `scripts/player_registry_browser_smoke.cjs` verifies all filter categories,
  bidirectional paging, five sorting controls, URL history, unknown versus empty,
  uint64 precision/tooltips, classify/link/unlink/reject, canonical routing,
  missing profiles, 503/retry recovery and map rendering at planned viewports.
- Local list/profile screenshots at **1440×1000, 1280×800, 390×844** and map at
  1280×800 are [comparison artifacts](../reference/player-registry/). No horizontal
  page overflow or browser page errors occurred. These are labeled fixture data,
  never proof of game integration.

Reproduce with a disposable database and locally started Go/Nuxt services:

```sh
# Set TEST_DATABASE_URL to a disposable PostgreSQL URL and a fixture-only
# OPERATOR_ACCESS_SECRET; Node 24 and the repository Go toolchain are required.
npx --yes --package=node@24.20.0 --call 'bash scripts/check.sh'

# Production collector/worker transport; loopback endpoints enforced.
SMOKE_WEB_URL=http://127.0.0.1:3016 \
PHMON_AGENT_URL=ws://127.0.0.1:8086/agent \
python3 scripts/player_registry_smoke.py

# Run in server/; fixture database name/loopback restriction is enforced.
PLAYER_REGISTRY_SIMULATOR_DATABASE_URL="$TEST_DATABASE_URL" \
go run ./testsupport/player-registry > /tmp/player-registry-fixture.json

# NODE_PATH points to an externally installed Playwright; no project dependency.
PLAYER_BROWSER_FIXTURE=/tmp/player-registry-fixture.json \
SMOKE_WEB_URL=http://127.0.0.1:3016 \
node scripts/player_registry_browser_smoke.cjs
```

A fresh fixture is required for a repeated browser run because review acceptance
intentionally changes its identity classifications. The disposable backend must
allow the local frontend origin. Fixture secrets are not production credentials.

Two existing validation defects were repaired: analytics integration fixtures now
stay within one UTC day even at midnight; the analytics date-range type follows
required Prettier formatting. Neither changes production analytics behavior.

## Capability blockers and exact next action

[Packet evidence and enablement requirements](../player-observation-protocol.md)
record pinned primary sources for every proposed opcode. Official Players API
examples include equipment, while **phBot 20.1.2 / Greatest / 2026-10-01** repository
runtime evidence has no `items`. No source proves the current target's complete
other-player instance stats, slot emptiness, job classification or stable cross-mode
identifier. `0x3038/0x3039` target layouts remain unresolved. Controlled-character
`0x3013/0x3040` cannot supply another player's instance facts.

Next runtime action requires separately authorized operator capture, not an
automatic bot operation: obtain sanitized complete callback sequences plus
independent screen/API facts for each exact client/server/data profile, explicit
spawn/despawn, malformed/partial cases, equip/unequip, reconnect/reused IDs,
normal↔Trader/Hunter/Thief, identical gear on distinct players and competing
transitions, including comparable world/cave scope. Then implement profile-gated
decoders with exact consumption tests; negotiate acknowledged observations; verify
actual equipment coverage and cross-mode matching before enabling the profile.
The next implementation sequence is detailed in
[the follow-up enablement plan](player-protocol-enablement.md), starting with a
disabled capability resolver before captures are available. The current follow-up
request changes documentation only.
