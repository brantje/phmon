# Plan: Map → Teleport → Designate Recall Point

Date: 2026-10-04. The operator subsequently authorized implementation on branch
`codex/designate-recall-point`. The command contract, guarded review UI, and a
read-only capture probe are implemented. Two successful manual Auren/Hotan
captures established the fixed request for phBot 20.1.3 on Greatest. The plugin
now submits that request under these exact conditions. Automated result
confirmation and a PhMon-issued runtime check remain open. See
[the investigation](../reference/recall-point-investigation.md).

## Intended behavior

From a map right-click, touch point action, or keyboard context menu, the operator can select a currently observed teleporter and designate it as the recall point for one or more checked characters. Each character gets an independent, audited result. The clicked map coordinate is only the menu anchor; the selected live gate determines the recall point.

With one observed gate, put **Designate Recall Point** in that gate's Teleport flyout alongside destination actions. With multiple gates, put it under each gate's submenu. Keep Reverse return separate because it uses a scroll and does not set a recall point. Show the action only for a selected gate; explain per-character unavailability in the review when the capability or fresh gate observation is missing. Preserve current desktop, touch, and keyboard flyout behavior.

## Evidence gate: determine the real phBot operation first

1. Inspect the current installed phBot runtime and [official map guide](https://guide.phbot.org/phbot/map), [plugin API index](https://plugins.phbot.org/phbot-api), [NPC API](https://plugins.phbot.org/phbot-api/npc), and [packet injection API](https://plugins.phbot.org/phbot-api/packet-injection) for a dedicated, supported recall-point function. Record the build, server/locale, API names, and results in `docs/phbot-capabilities.md`.
2. Existing [Issue #32 evidence](../reference/issue32-teleporter-investigation.md) found no recall-point API. Its community `0x7059` payload is a hypothesis, not an implementation contract. The documented script command `recall` recalls a pick pet, so it cannot serve this action.
3. If no dedicated API exists, obtain an operator-authorized capture of one manual **Designate Recall Point** action on a test character at a known gate. Verify the emitted opcode and payload, whether NPC selection or range checks precede it, and whether a server response or phBot readback confirms the saved point. Check a second gate/server/build as needed before broad enablement. Static inspection of the operator-authorized phMonitor executable may inform behavior, but does not replace phBot runtime evidence.
4. Record exact supported builds and conditions. If the capture does not establish a safe mechanism, keep the command unsupported with a precise reason and retain this plan as an open capability gap. Do not expose a control that implies the point was saved.

## Implementation path

1. **Command contract.** Add a fixed `character.recall_point.designate` command to the Go allowlist and plugin capability frame. Its arguments identify the selected gate by server name, region, position, and model when available; the existing command envelope carries the character and expected session. Validate finite coordinates, a `GATE_*` server name, confirmation, and the command's narrow payload. No opcode or arbitrary packet bytes may come from the browser. Update protocol/version compatibility documentation if the new capability requires a version gate.
2. **Fresh, per-character gate resolution.** Reuse `map.npcs` observers for UI eligibility, but resolve the gate again through that character's own `get_npcs()` at plugin execution time. Match the requested gate's server name and location/model within the map's documented tolerance; reject missing, stale, or ambiguous matches. Never reuse a merged map marker's runtime NPC ID for a different character. Recheck session/generation immediately before the action.
3. **Bounded execution and result.** Implement only the verified phBot primitive or verified fixed packet sequence in the plugin callback path. Report the character's gate, effective action, and a reason code through the existing audited command lifecycle. Distinguish packet/API submission from confirmed designation. If no response/readback proves the saved point, display **sent; outcome unverified** and do not persist a claimed current recall point. Do not add a new database table unless verified readback introduces durable state; the existing command audit is sufficient for attempts.
4. **Map interaction.** Add the action to `web/app/pages/map.vue` and a focused intent/composable beside `useMapTeleportAction.ts`. Use the shared fan-out preview/results UI with mandatory explicit confirmation, checked-target count, eligible/skipped counts and reasons, concurrent admissions, exact retries, and scope/session invalidation. The confirmation names the gate and explains that it changes the character's recall destination. A newly stale feed, changed gate, or changed target selection cancels a prepared review.
5. **Compatibility and documentation.** Advertise support only for verified runtime/server conditions. Older agents and unsupported builds receive a clear capability reason. Update `docs/protocol.md`, `docs/phbot-capabilities.md`, `docs/reference-parity.md`, plugin install guidance, and the AGENTS.md ledger with source, test, and remaining runtime limits.

## Acceptance checks

- Plugin tests: absent API, missing/ambiguous gate, wrong session, changed gate, unsupported build, one verified execution, result classification, and no arbitrary opcode/packet input.
- Go tests: strict argument/confirmation validation, capability gating, authenticated admission, per-character audit, idempotency, expiry, and independent four-worker fan-out delivery.
- Frontend tests: zero/one/multiple gates, mixed eligible targets, preview cancellation, per-character results, touch and keyboard flyouts, and no POST before confirmation.
- Browser checks: 1440 × 1000, 1280 × 800, and 390 × 844; compare the map menu with the reference and record screenshots in `docs/reference-parity.md`.
- Simulator end-to-end run using a deterministic fake phBot adapter for the verified contract. Separately record the operator-authorized Windows/phBot test and observed server response; simulator success alone does not establish live support.

Current status: Go validation/capability projection, plugin gate resolution and
fixed packet submission, map intent/review, and the read-only probe are
implemented and unit-tested. The operator's manual Windows/phBot designation
succeeded twice. Server-response status-byte classification, the
verified-contract simulator, browser viewport screenshots, and a PhMon-issued
live designation remain pending. The manual test proves the request bytes; it
does not prove the PhMon remote command path or a later saved outcome.

## Current repo entry points

- `web/app/pages/map.vue`: map context menu, gate and destination flyouts.
- `web/app/composables/useMapTeleportAction.ts`, `web/app/utils/mapTeleportAction.ts`: selected targets, gate/session eligibility, and shared fan-out.
- `server/internal/commands/{catalog,service}.go`, `server/internal/httpapi/agent.go`: allowlist, validation, capability projection and admission.
- `plugin/PhMon.py`: capability frame, per-session `get_npcs()` gate lookup, and command execution.
- `web/tests/mapTeleportAction.test.ts`, `plugin/test_phmon.py`, `server/internal/commands/*_test.go`: focused regression coverage.

The supplied screenshot path was unavailable on this host, so menu placement above follows the current PhMon implementation and the official phBot map guide. Recheck the visual detail against the screenshot if it is reattached.
