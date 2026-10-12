# PhMon enrichment v2 and player identity presentation

The receiver implements the operator's [ph-proxy v2 contract](https://github.com/brantje/ph-proxy/blob/main/docs/phmon-enrichment-v2.md).
Synthetic contract examples are retained in `server/internal/phproxy/testdata/upstream-v2-examples.json`
from upstream blob `d567b1b7453d20efe6b0e5373de4020187e7704a`. These are synthetic
examples, not captured player traffic or executable/source extracts.

## Transport and evidence

- `POST /api/integrations/ph-proxy/v2/enrichments`: existing agent bearer token and
  `schema_version: 2`. V1 stays independently strict; endpoint and schema must agree.
- `GET /api/integrations/ph-proxy/v2/status?server=`: operator-authenticated latest
  gateway metadata, coverage and successful receipt. It does not reveal the
  sender's live queue/state or identify the cause of an earlier rejected upload.
- Profile `silkroad-equipment-v1`, field/opcode/reason allowlists, 256 KiB bodies,
  128 players, 64 entries per list, 24-hour idempotency/staging and the existing
  10,000 pending-name limit per agent remain enforced.
- Name and explicit level require direct normal-name attribution. Eligible model,
  gear, avatar, job, job level, race, guild, position and job-item observations may
  carry `job_alias` attribution with a bounded self-contained inferred association.
  Links are optional in subsequent association-bearing deltas.
- Field evidence and associations remain part of equal-time comparisons and are
  exposed on details. Older/conflicting equal-time observations preserve accepted
  data; omitted/null fields preserve data, while complete empty lists replace it.
- Invalid v2 fields return HTTP 400 with fixed `code`, allowlisted `field` and
  zero-based `player_index` where available. Credentials, player names, raw
  payloads and decoder error strings are excluded. Unsupported evidence properties
  are rejected. Other existing HTTP statuses and retry rules remain unchanged.
- Identity field updates are serialized across concurrent uploads and pending
  promotion. Promotion copies all supported v2 fields and their original evidence.
  Alias-attributed observations never become unqualified normal-name fallbacks.
- Migration 33 permits unsigned 32-bit model references and 128-byte staged names.
  Ordinary phBot tracking names remain bounded by its 64-byte protocol contract;
  v1 input bounds are unchanged. No proxy-only name becomes a tracking record.

## Display identity API

List and detail responses add `display_identity` without rewriting native fields:

```json
{
  "canonical_id": "33333333-3333-4333-8333-333333333333",
  "player_name": "Magik",
  "job_alias": "xDont",
  "aliases": ["xDont"],
  "related_record_ids": ["33333333-3333-4333-8333-333333333333", "44444444-4444-4444-8444-444444444444"],
  "source": "ph-proxy",
  "status": "inferred",
  "first_seen_at": "2026-10-12T00:00:00Z",
  "last_seen_at": "2026-10-12T01:00:00Z"
}
```

This example is synthetic. Missing names/aliases are JSON null and display as
**Unknown**. Other status values are `observed`, `unknown` and `conflict`.
Related IDs include the canonical record and original alias records.

Names are classified from explicit native names, known jobbing state, thief
sightings, own-character identity, or accepted normal-name proxy evidence.
Unclassified observed names are not guessed to be normal names. Job affiliation
alone does not classify a job alias. The original observed name stays on details.

Mappings use exact normalized names within the same server. Only a unique,
non-conflicting accepted mapping groups records. A tracked normal-name record is
the canonical list row; no player, observation, level snapshot or live-map row is
merged or deleted. Conflicting normal identities and multiply claimed aliases
stay separate, with an unresolved-mapping indicator. An observed alias remains
known even when its normal-name association is ambiguous; it is not promoted to
a known player name. Proxy-only names remain
staged until ordinary tracking establishes the normal-name record.

The primary alias is the compatible explicit native alias, or the latest accepted
proxy alias by `last_matched` with normalized alias as the tie-breaker. Details
show all aliases and links to each original sighting record. Display sighting times
span the group; native timestamps retain their original meaning. Other native
values remain those of the primary record.

The same SQL projection drives list/detail identity. Grouping, name/all-alias
search, sorting, own-character filtering, counts and pagination precede returning
rows. Scoped reads restrict the base relation to the requested server. Counts and
pages use one read-only repeatable-read snapshot. Existing sort key `job_name`
remains compatible while its UI heading is **Job alias**. Own characters remain
hidden by default and can be included with the existing filter.

An original alias detail URL retains its native history but reads latest enrichment
from the canonical normal-name record. Gear still exposes its original identity,
confidence and observation time. Unknown item/model IDs stay numeric. Enrichment
never establishes current equipment inventory, rolled properties or live presence.

## Verification and boundaries

Automated tests cover all three upstream v2 examples, malformed associations,
strict v1, safe HTTP diagnostics, known empty lists, evidence-only equal-time
conflicts, unsigned model IDs, staged names, v2 promotion, concurrent freshness,
cross-server isolation, ambiguity, classification, search, counts and pagination.
The synthetic Magik/xDont case retains two original records and histories while
showing one player with seven visible gear entries. Both detail URLs expose gear.

Browser acceptance uses a separate loopback Go/Nuxt instance and isolated
PostgreSQL data, never an operator's real bot characters. Screenshots and results
are linked in `docs/reference-parity.md`. The browser harness is
`scripts/player_identity_browser_check.mjs`; it requires fixture-server and local
operator configuration and never seeds production monitoring or sends bot commands.

These results establish receiver/presentation behavior with fixtures. Earlier
read-only production database inspection found Magik's seven stored items and
inferred xDont link; a historical HTTP 400 lacked player/field attribution.
Neither observation identifies the cause of that rejected request. This increment
does not deploy, reconfigure ph-proxy, or claim new Windows/phBot live validation.

## Normal-player job affiliation — 2026-10-12

The upstream sender uses numeric `job` / `job_level`, including known zero, and
can attach both directly to normal character names without a worn job item.
PhMon's native registry calls the displayed job type `job`. The receiver already
accepted these v2 fields and `unverified_reference_mapping`; the missing path was
`syncProxyFallbacks`, which deliberately omitted job, plus detail UI native-only
reads. Both are corrected.

Direct normal-name job evidence populates `proxy_job` and, only when native job
is null, `display_fallback.job`: 0 = `none`, 1 = `trader`, 2 = `thief`, 3 = `hunter`.
Other uint8 codes remain numeric strings and render as **Unknown (code)**. The
original numeric code and evidence remain in `enrichment`. Native `none`,
`unknown` and job level zero are known and keep precedence. No job-item or
`is_jobbing` condition gates affiliation. Alias-derived jobs remain excluded from
fallbacks and appear separately on details with an **Inferred from alias** label.
Both list and detail expose original job/job-level evidence on fallbacks and show
its confidence; `unverified_reference_mapping` is never upgraded to verified.

Migration 35 recovers fallbacks from previously accepted direct evidence in both
attached and staged enrichment. It changes no player, tracking/history, agent,
live-map state or original enrichment evidence/time. Job and job-level filtering,
sorting and counts use native-first COALESCE before pagination; exact unfamiliar
job codes can be filtered numerically. Known empty job-item observations render
**None observed**, without changing native jobbing state.

Sender inspection, local tests, screenshots and live receipt/display evidence are in
[the focused verification report](reference/player-job-enrichment-2026-10-12.md).
The requested local Compose server/web rebuild applied migration 35 and verified
normal-player fallbacks against the existing database. Windows phBot sender
runtime validation remains separate.

## Nearby-player admission — 2026-10-12

A generic nearby name does not classify player name versus job alias. The receiver
now requires identity evidence before `ApplyLive` creates registry/history rows.
Non-expired accepted pending direct-name evidence permits admission and promotion
when ordinary tracking observes the exact normalized name in that server. Accepted
global alias links can then admit related alias records; a self-contained field
association alone is not a global alias mapping. Proxy-only names remain staged.
Live-map sightings stay independent and complete. See the
[source investigation and validation](reference/player-admission-2026-10-12.md).
The rule is implemented and running in the local Compose server after the
operator-requested container rebuild. Other instances require their own rollout.
