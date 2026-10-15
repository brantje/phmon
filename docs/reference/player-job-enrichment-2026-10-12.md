# Normal-player ph-proxy v2 job verification — 2026-10-12

Story: an authenticated v2 upload containing numeric job affiliation for a normal
player reaches enrichment storage and the Players list/detail, with native-first
precedence and original confidence, independently of wearing a job suit.

## Sender evidence

Inspected upstream main at `406fc04e5c1703e2f8880e47db9ba191a8d3209f` through the
authenticated GitHub API (public browser fetch returned 404). No local ph-proxy
checkout was available. Relevant primary sources:

- [v2 contract](https://github.com/brantje/ph-proxy/blob/406fc04e5c1703e2f8880e47db9ba191a8d3209f/docs/phmon-enrichment-v2.md),
  blob `22a3f01ed3a41fe262c29df3b6f6f964de999fde`.
- [sender wire types](https://github.com/brantje/ph-proxy/blob/406fc04e5c1703e2f8880e47db9ba191a8d3209f/internal/phmon/upload.go):
  `Job *uint8` / `JobLevel *uint8`, JSON `job` / `job_level`; zero pointers survive
  `omitempty`. There is no `job_type` field in the enrichment contract.
- [field copy/validation](https://github.com/brantje/ph-proxy/blob/406fc04e5c1703e2f8880e47db9ba191a8d3209f/internal/phmon/player.go):
  per-field values/evidence, direct or association-bearing alias provenance,
  `verified` and `unverified_reference_mapping` are accepted for spawn job data.
- [spawn reader](https://github.com/brantje/ph-proxy/blob/406fc04e5c1703e2f8880e47db9ba191a8d3209f/internal/silkroad/spawn.go):
  job/job level are captured for normal-name and job-alias observations; classified
  worn job items are separate. Empty job-item list does not erase affiliation.

The code labels follow PhMon's existing native vocabulary / numeric mapping in
`plugin/PhMon.py` (`_JOB_TYPE_CANDIDATES`): 0 none, 1 trader, 2 thief, 3 hunter.
Unfamiliar codes stay numeric, and upstream confidence stays explicitly unverified
where supplied. This is receiver/presentation evidence, not independent proof of
server-specific packet semantics in a Windows/phBot runtime.

## Trace and corrections

| Boundary | Result |
| --- | --- |
| Request validation | `validateSubmitV2` accepts numeric uint8 job/job level with matching per-field evidence; `validateEvidenceV2` already accepts `unverified_reference_mapping`. No validation loosening needed. |
| Storage | Original numeric values/evidence were retained in `extended`/`field_evidence`; `syncProxyFallbacks` intentionally left `proxy_job` empty. It now maps direct evidence to native names or numeric strings. Alias-derived job/level cannot populate normal fallbacks. |
| Existing enrichment | Migration `000035_ph_proxy_job_fallback.sql` backfills attached and staged direct jobs without rewriting tracking/history/presence, evidence or observation times. |
| API | List and detail preserve native fields and expose fallbacks only for null native values, with `job_evidence` / `job_level_evidence`. Detail refresh after promotion includes newly attached fallbacks. |
| UI | Both list and detail use native-first job/job-level fallbacks and display source/confidence. Unknown codes render numerically. Details separately show raw proxy job/level and visibly inferred alias source, plus known empty worn-item state. |
| Query semantics | Job/job-level filters, sorts and counts use the same native-first values before pagination. Numeric unfamiliar codes are filterable through the API and visible list options. |

No production deployment, proxy reconfiguration, tracking cleanup, character
commands or outbound notifications were performed.

## Local validation

- PostgreSQL integration tests in `server/internal/phproxy/job_flow_integration_test.go`:
  normal Trader/Thief/Hunter without suits, numeric zero, unknown 255, native None
  and native Hunter precedence, native job level zero, alias exclusion/evidence,
  confidence retention, complete tracking/history preservation, staged promotion,
  attached/staged backfill, filtered counts and sorting before pagination.
- HTTP integration test `TestPhProxyIdentityHTTPFlow`: authenticated v2 direct
  numeric zero reaches list/detail with original unverified confidence; later
  alias-attributed values remain raw enrichment and never become fallbacks.
- `go test -race ./internal/phproxy ./internal/players ./internal/httpapi`,
  `go vet` for these packages and server build passed against isolated PostgreSQL.
- Node 24.20.0 frontend: 249 unit tests, lint (zero errors; existing warnings) and
  Nuxt typecheck and production build passed.
- All `scripts/check.sh` gates passed on a fresh, ready PostgreSQL **18.6** database
  with Node **24.20.0** and `GOFLAGS=-p=1` (serial test packages, race detector still
  enabled). Includes Go vet/race/build, **266** plugin tests, transport audit,
  frontend format/lint/typecheck/build and **249** unit tests. The script's final
  Compose configuration gate initially stopped because `.env.example` intentionally
  leaves its operator secret blank; the same final gate passed separately with
  `OPERATOR_ACCESS_SECRET=phmon-isolated-job-browser-test-secret-2026`.
  No real credential was printed or changed.

  Earlier broad-check attempts used PostgreSQL 17 (an unrelated map-index plan
  assertion and command-admission failure), or started PostgreSQL 18 tests before
  readiness. Reusing that interrupted database then hit fixed-fixture duplicate
  keys in map tests. A fresh ready 18.6 database with serial packages passed every
  Go suite; no unrelated production code or tests were changed to force a pass.
  Logs: `/tmp/phmon-job-final-check.log`, `/tmp/phmon-job-web-build.log`.

- `scripts/player_job_browser_check.mjs`: accepted synthetic HTTP v2 payloads
  → isolated PostgreSQL → running Go → Nuxt → Chrome list/detail. Verified all
  requested precedence/attribution/confidence rules at 1440×1000, 1280×800 and
  390×844, with no browser errors or whole-page horizontal overflow.

Evidence: [browser results](local/player-job-browser-results.json),
[desktop list](local/player-job-list-1440.png),
[normal-player detail](local/player-job-detail-1440.png),
[alias detail on mobile](local/player-job-alias-mobile.png).

## Live evidence and rollout status

Read-only production PostgreSQL and authenticated GET inspection on 2026-10-12
confirmed the data already arrives. One snapshot showed 226 direct normal-name
job enrichments, all retaining `unverified_reference_mapping`: 164 code 0, 50 code
1, 10 code 2 and 2 code 3. Their fallback column was empty. A later snapshot showed
227 direct jobs; 217 had a known empty classified job-item observation, 225 had
null native job and 60 had null native job with nonzero job affiliation. Counts
changed with ongoing live traffic. Successful-receipt timestamps were present;
a sampled latest receipt was `2026-10-12T09:46:42.524109Z` (11:46:42 CEST).

Before the requested restart, an authenticated live `GET /api/players/{id}` for
a direct normal-name Trader returned HTTP 200, native job absent, `enrichment.job=1`,
reason `unverified_reference_mapping`, identity kind `character_name`, and no job
fallback.

The operator then requested that the Compose containers be rebuilt and restarted.
The server and web containers came up healthy, PostgreSQL stayed running, and the
server applied migration 35 (ledger version 35). A post-migration database snapshot
contained 233 direct normal-name job records and 233 corresponding fallback values:
169 code 0 → `none`, 52 code 1 → `trader`, 10 code 2 → `thief`, and 2 code 3 →
`hunter`. Three rows with known native values retain precedence in the API.

Authenticated GETs through the restarted local web/API stack verified list and
detail for a live stored native-null code-1 player: `display_fallback.job=trader`,
`enrichment.job=1`, and the original `unverified_reference_mapping` evidence.
This confirms live receipt, storage, migration, API projection and displayed value
for the existing local traffic. No private player identity or token is recorded.
No new payload was sent and no character was operated during this check.

Windows sender settings and raw outgoing requests were not independently inspected;
the operator supplied that uploads are enabled and v2 is selected. Native Windows/
phBot packet/runtime behavior and a new naturally occurring post-restart upload
remain unverified. Existing tracking IDs/history were not rewritten by migration 35;
the enrichment code has no live-presence write path.
