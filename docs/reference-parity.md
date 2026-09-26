# Reference parity ledger

This ledger records implementation evidence against the public phMonitor demo
baseline captured in docs/reference on 2026-09-26. Reference screenshots are
inspection evidence only and are never shipped as PhMon application assets.

## Slice 1 — shell and instance access

Status: Slice 1 shell implementation and automated CI validation complete; required
same-viewport browser comparison remains BLOCKED in the current execution environment.

### Implemented behavior

- Desktop shell uses the reference proportions: approximately 228 px navigation rail
  and 34 px top strip, with compact rows and controls rather than a generic SaaS
  landing page.
- Semantic colors start from primary #FEF6C3, panel #0D131D and text #EAF1FF, with
  near-black/navy surfaces, blue-gray borders, subdued secondary text and restrained
  status colors.
- Segoe UI, Tahoma and Arial are the primary system font stack; the page title and
  dense navigation/table sizing follow the recorded baseline.
- Sidebar grouping exposes the known product areas without pretending later slices
  are implemented. Future screens are disabled and advanced-only tools appear only
  when advanced mode is enabled.
- Easy/advanced mode and desktop sidebar collapse persist as same-site preferences.
  They are presentation state only and grant no authorization.
- Mobile navigation becomes off-canvas; agent rows reflow into compact cards instead
  of forcing whole-page horizontal overflow.
- Instance access uses the current PhMon origin for copy/QR. The QR never contains an
  agent ID, token or other credential.
- Backend readiness remains a calm top-strip diagnostic. Agent API failures retain the
  last successful list and show stale/unavailable state instead of a recurring modal.
- The Slice 1 dashboard is backed by GET /api/agents and shows real connected state,
  stable agent ID, plugin/phBot/protocol versions, connection age and last-seen time.
  It can also create a new agent ID/token pair through a no-store same-origin POST;
  the plaintext token remains visible only in the current provisioning panel and is
  not recoverable later. No fabricated monitoring data is used.

### Evidence and deliberate deferrals

Backend evidence is the authenticated /agent protocol, PostgreSQL agent metadata and
generation-fenced active registry. Plugin evidence is PhMon.py plus the shared
simulator contract. UI evidence is the Nuxt shell and same-origin /api/agents route.

The neutral dark backdrop is deliberate Slice 1 development treatment; approved
game-world artwork is still required before final visual completion. Character/server
data, dashboard game panels and server scoping depend on later owning slices and are
not represented as working controls.

Hosted CI is green for the Slice 1 transport/test head, including the complete Docker
agent lifecycle and database outage/recovery path. That automated evidence does not
substitute for same-viewport visual comparison.

Required browser evidence remains open because this execution environment cannot run
the complete local Nuxt/Docker browser stack:

- 1440x1000: BLOCKED — no screenshot captured or side-by-side comparison observed.
- 1280x800: BLOCKED — no screenshot captured or side-by-side comparison observed.
- 390x844: BLOCKED — no screenshot captured or side-by-side comparison observed.

The sidebar contract calls for a PhMon version/build identifier. The current
"self-hosted · slice 1" text is a development/slice label, not an application version.
No canonical PhMon application version/build value currently exists in package
metadata or runtime configuration. A hardcoded invented version would be misleading,
so the version portion of the parity ledger remains explicitly open until canonical
release/build metadata exists.

Real Windows/phBot runtime validation is also still open. The simulator exercises the
same production wire contract but is not evidence that the embedded phBot runtime has
loaded and operated the plugin successfully.

## Slice 2 — character overview and detail

Status: implementation and automated checks complete for the implementable Slice 2
scope; final visual parity and real phBot data-API validation remain open.

### Reference evidence reviewed

- Reopened `docs/reference/phmonitor-dashboard.png` and
  `docs/reference/phmonitor-stats.png` (1440 × 1000). Dashboard uses an asymmetric
  panel layout with fleet character counters, last deaths, server information, a
  broad recent-events panel, rare-drop/chat stack and offers. Stats opens with a
  compact thematic header and a large grouped-character surface; its empty state
  offers a first-group prompt rather than implying characters have loaded.
- These saved captures are from the recorded 2026-09-26 public-demo inspection. The
  live demo and advanced character subtabs were not independently re-exercised in
  this slice; hidden details are not inferred from these screenshots.

### Implemented and evidenced behavior

- Go/PostgreSQL automatically resolves joined identities by normalized server plus
  character name, tracks generation-scoped live sessions/current stats and closes
  sessions on leave, replacement, disconnect and backend startup.
- Protocol v2 identifies a character before registration, then requires explicit
  `character_id` on each snapshot/state/left message. The plugin reports documented
  state fields and sends full snapshots after reconnect.
- Character overview supports server/name/guild/zone search, persisted group
  filtering and membership controls. Stable detail path is
  `/characters/{character_id}`. It shows known state and labels later inventory,
  pets, party, map and action areas as not yet implemented.
- Empty character state explains automatic phBot discovery. Group operations are
  persisted but remain organizational metadata.
- No external phMonitor assets or service calls were introduced. The neutral
  development backdrop remains; approved local artwork and final visual treatment
  remain open.

### Comparison and gaps

- A compact character table now appears above the existing agent/operations panels;
  the reference's deaths, server-information artwork, events, drops/chat and offers
  remain owned by later slices. This is the Slice 2 state surface, not complete
  dashboard parity.
- Local overview evidence: [1440 × 1000](evidence/slice2-overview-1440x1000.png),
  [1280 × 800](evidence/slice2-overview-1280x800.png) and
  [390 × 844](evidence/slice2-overview-390x844.png). The 1440 capture was reviewed
  against the saved dashboard/stats references. At 390 px, document/body scroll width
  equals the viewport width; the mobile navigation remains collapsed and content
  stacks without page-level horizontal overflow. [Character detail at 1440 × 1000](evidence/slice2-character-detail-1440x1000.png)
  has no direct reference capture; its hierarchy follows the shared shell while later
  panels remain explicit gaps.
- Botting/training remains unknown because official docs expose start/stop mutations
  but no read-only getter. No phBot data API has been manually validated yet,
  despite operator confirmation of basic plugin/backend connectivity. The plugin's
  v2 lifecycle was exercised through the deterministic simulator over the same
  production transport, not a phBot runtime.
