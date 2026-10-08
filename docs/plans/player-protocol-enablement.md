# Player packet decoding and automatic transition linking

Status: planned, 2026-10-08. This document follows the
[player registry implementation](player-registry-job-identities.md) and the
[opcode investigation](../player-observation-protocol.md), under the canonical
[engineering guide](../../AGENTS.md). The current request authorizes this
documentation plan; it does not start another implementation or runtime run.

## Outcome

Implement independently authored player decoders and connect verified evidence to
the existing registry, equipment history and lifecycle matcher. Enable capabilities
only for an exact target profile whose evidence supports them. Missing, ambiguous,
unsupported or revoked evidence keeps that capability disabled. A working spawn
decoder must not implicitly enable equipment deltas, identity comparison or
automatic links.

Current baseline: plugin 1.9.30 / protocol 18 has local diagnostic capture only;
`AssessTransition()` always returns `Automatic: false`. No production player entity
decoder, acknowledged `player.observations` feed or target-profile transition policy
is enabled. Existing `map.players`, LiveStore, confirmed alias reuse and manual
review continue independently. The phBot 20.1.2 / Greatest evidence without `items`
does not satisfy an equipment gate. All work below remains pending.

## Capability and evidence model

Use an operator-managed, versioned profile manifest and a backend-owned capability
resolver. Identify each profile by normalized server key, exact client family/build,
phBot version, exported dataset version/hash and decoder/comparison-rule revision.
Record how the authenticated observer session matches that tuple; an unknown tuple
must not fall back to a similar server or a source implementation's client branch.

Keep separate gates for each supported opcode/layout, job-name classification,
slot/attribute coverage, lifecycle continuity, world/cave coordinate scope,
identity comparison and automatic transition rule. Automatic rules are scoped to
profile, job role and direction: normal→Trader/Hunter/Thief and each reversal need
their own validation. Cross-profile automatic association is disabled until a
separate rule proves comparability; identical hashes do not establish it.

| State | Behavior |
| --- | --- |
| Disabled / evidence missing | Existing getter observations only; capture can remain opt-in; no entity decode, verified identity hash or automatic link |
| Implemented / test-only | Pure parser exercised against explicitly synthetic fixtures; no production feed |
| Runtime verified | Exact-profile sanitized fixtures and independently observed facts validate the named fields/layout; reviewed manifest permits that decoder capability |
| Review only | Verified lifecycle/classification/comparison gates allow persisted explainable candidates; `automatic=false` |
| Automatic enabled | All rule dependencies and runtime acceptance pass, and that exact rule is explicitly enabled in audited operator configuration |
| Revoked | Stop new use immediately, reset affected lifecycle state and mark affected evidence/policies unavailable; retain prior observations and decisions |

Manifest entries must contain evidence IDs/content hashes, sanitized fixture paths,
capture metadata, pinned primary-source references, exact verified fields and
limitations, reviewer/date and the implementation revision tested. Capture metadata
includes server/client/phBot/dataset versions, sequence/session boundaries and the
independent facts used as ground truth. Runtime and synthetic fixtures have
different labels and directories. Raw exports remain local until sanitized; no
executable, extracted client source or credentials enter the repository.

Verification is owned by the backend manifest. Agent-provided `verified`,
`identity_verified`, profile IDs or matching scores cannot authorize capabilities.
Missing dependency, manifest mismatch or invalid revision produces a stable reason
code rather than a global force-enable bypass. A config switch alone cannot turn
missing evidence into verified support.

## Implementation sequence

### 1. Build the disabled capability foundation

- Add manifest validation and a profile/capability resolver in
  `server/internal/players`; connect it to authenticated agent session admission.
  Keep all target capabilities disabled in the initial manifest.
- Extend profile status through an authenticated read endpoint, proposed
  `GET /api/players/capabilities?server=...`, and its Nuxt proxy. Expose enabled
  capabilities, verified coverage and reason codes such as `unknown_profile`,
  `evidence_missing`, `unsupported_layout`, `evidence_revoked` and `policy_disabled`.
- Include profile/policy revision in stored evidence and decisions. If an additional
  migration is needed, use the next unused additive migration; do not rewrite
  000026 after it has been applied outside disposable tests.
- Show concise unavailable/review-only status in Player profiles and review UI.
  Retain the current list/map behavior and manual decision controls.

Acceptance: unknown/mismatched/revoked profiles and agent verification claims cannot
enable any decoder, fingerprint comparison or automatic link. Changing one profile
does not change another profile, opcode or job-direction rule. This increment can
be implemented and tested before runtime captures are available.

### 2. Acquire and catalogue exact-profile runtime evidence

Use the existing opt-in bounded capture on an operator-authorized target session.
No agent operates a real character merely to produce evidence. Catalogue captures
and independent screen/API observations for:

- Single spawn/despawn (`0x3015`/`0x3016`) and complete mixed-entity multipart groups
  (`0x3017`/`0x3019`/`0x3018`), including multiple segments and entity counts.
- Equip/unequip (`0x3038`/`0x3039`), slot/model/plus semantics, verified empty slots
  and available instance fields. Missing jewelry or omitted slots remain unknown.
- Normal/job names, job types and character model/level facts; job level must not
  be used as character level. Record unavailable fields explicitly.
- Reconnect, teleport, repeated spawn, runtime-ID reuse, incomplete groups and
  capture overflow/reset; record observer and target coordinates separately.
- All three normal↔job transitions, reversals, same gear on different players,
  nearby competing transitions and simultaneous presence. Cover each intended
  world/cave/floor coordinate scope.

Corroborate layouts with pinned primary implementations. The unresolved
`0x3038`/`0x3039` layouts need both an applicable field-order source and exact target
fixtures; an opcode label alone is insufficient. `0x3013`/`0x3040` own-character
data must not become an other-player equipment source.

Acceptance: every claimed capability has an inspectable evidence manifest; holes
remain recorded as disabled gates. Do not wait for every profile before continuing
work on a profile with sufficient evidence, and do not invent runtime fixtures.

### 3. Implement pure, bounded profile decoders

- Add pure Python parsing modules/tests behind disabled profile dispatch. Preserve
  callback byte-copy-only admission; decoding and assembly run on the worker.
- Implement verified single spawn/despawn and complete group assembly first. Add
  equip/unequip only when their separate layout gates pass. Exact byte consumption,
  bounded counts and explicit entity discrimination are mandatory.
- Preserve current capture bounds as the initial maximums: 64 KiB packet, 128
  records / 2 MiB payload, 256 KiB assembled group, 128 entities and 30 s capture.
  Runtime assembly also needs a bounded timeout and reset behavior. Any changed
  limits require measured memory/latency evidence.
- Unknown models/variants, trailing or missing bytes, count disagreement and
  malformed sequences reject the affected packet/group and invalidate continuity.
  Partial groups never publish partial entities or inferred despawns.
- Emit only verified fields, source/decoder/profile versions and observation times.
  Map slots to unknown/empty/occupied using explicit evidence. Keep uint64 instance
  values as decimal strings; catalog data remains reference metadata.

Acceptance: synthetic malformed/overflow/reset tests and separately labeled sanitized
runtime fixtures pass. Replaying a capture in test mode invokes no phBot command,
socket injection, production observation write or automatic decision. Only exact
verified layouts can leave test-only mode; unsupported variants remain disabled.

### 4. Negotiate durable supplemental observations

- Specify `player.observations` and commit acknowledgements in `docs/protocol.md`;
  choose the next available protocol revision during implementation and update
  `scripts/plugin_protocol_contract.py`. Preserve older agents and `map.players`.
- Negotiate exact supported profile/capability revisions with the authenticated
  backend. Additional packet facts supplement getter sightings and retain their
  own source IDs; one sighting must not create duplicate players.
- Start with at most 128 observations and 256 KiB serialized bytes per batch,
  always below the existing transport limit. Document bounded pending count/bytes,
  disk replay retention and overflow behavior. No unbounded packet spool.
- Assign stable evidence IDs with server/session/epoch/incarnation and source
  references; retries resend the exact payload. Fence generations/sequences,
  reject conflicting reuse and acknowledge only after database commit.
- Adapt accepted facts into existing per-field merge/history/enrichment code. Feed
  lifecycle events once after durable admission; replay must not create fresh
  despawns, incarnations, intervals or candidates. Reset lifecycle on loss of
  continuity; a missing snapshot or dropped batch cannot establish absence.

Acceptance: plugin→authenticated transport→PostgreSQL→API→browser passes restart,
reconnect, lost acknowledgement, delayed data, replay, multiple observers and
database outage tests. Live map remains independent of persistence failures.

### 5. Admit verified candidates before automatic associations

- Wire committed verified spawn/despawn evidence to `LifecycleTracker` and
  `AddTransitionCandidate`. Persist original source ownership and explainable
  assessments, including competing candidates, without fabricated probabilities.
- Enable comparison only for the verified normal-slot projection and profile
  revision. Exclude job outfits/avatar/transient fields and require at least four
  comparable normal slots. Treat unknown coverage and incompatible model/level
  or coordinate scope as insufficient evidence.
- Keep the current 15 s / 30 world units / 60 s defaults as proposed values to
  validate, not proof of a correct automatic rule. Reject conflicting gear and
  simultaneous presence; external thief reports cannot authorize transitions.
- Run each profile/job-direction rule in review-only observation mode against
  independently labeled transitions and negative cases. Record reviewed outcomes,
  ambiguous/false matches and the exact tested rule revision.
- Add an internal automatic-decision path only after that rule passes acceptance.
  Reuse the manual path's deterministic transactional locks, server checks,
  revision fencing, conflict/cycle rejection and reversible canonical projection.
  Audit actor as the system policy, retain evidence IDs and policy revision, and
  re-evaluate current evidence/gates inside the decision transaction.

Acceptance: distinct players with identical gear, competing matches, contradictory
presence, stale policy/review revisions and concurrent manual/automatic decisions
cannot produce unsafe links. Matching-window expiry retains existing candidates.
Passing one role/direction does not enable unverified ones. Manual confirm/reject,
correction and unlink remain available.

### 6. Verify activation, revocation and presentation

- Run plugin/protocol checks, Go tests/races, disposable PostgreSQL integration,
  frontend tests, formatting/lint/typecheck and the production build with the
  repository-required toolchain. Measure high-change writes and bounded queues.
- Verify profile/equipment/tooltips/candidate flows and map regressions at
  1440×1000, 1280×800 and 390×844, including partial/unavailable/recovered status.
- Record separately authorized actual target-runtime evidence for the exact
  enabled decoders and job-direction rules. Simulator success cannot replace it.
- Test immediate rule/decoder revocation, profile change and restart. Stop new
  admissions/decisions, discard uncommitted lifecycle continuity and expose why.
  Keep source evidence and prior link audit history; revocation must not silently
  unlink confirmed players or delete observations. Flag affected associations
  for operator review where their evidence has been invalidated.
- Update the focused feature ledger, `AGENTS.md`, capability/protocol docs and
  reference evidence per increment. Existing unresolved profiles remain visible
  as disabled; no global completion claim while required gates are open.

## Completion checklist and next action

- [ ] Disabled capability resolver, backend-owned manifests and visible reasons.
- [ ] Exact target fixtures catalogued per opcode/field/profile; missing gates retained.
- [ ] Pure bounded decoders pass synthetic and actual-fixture checks separately.
- [ ] Negotiated supplemental feed has post-commit acknowledgement and exact replay.
- [ ] Verified lifecycle produces retained review candidates without automatic links.
- [ ] Each automatic role/direction rule passes real positive and negative cases.
- [ ] Transactional activation/revocation, retained audits and full regressions pass.

Start with increment 1 using a manifest that enables nothing. Then request only the
specific missing capture/access needed for increment 2, while continuing independent
test/transport preparation. This plan does not authorize deployment, packet
injection, private-service access, a worktree or real-character operation.
