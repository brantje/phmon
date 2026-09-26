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
  No fabricated monitoring data is used.

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
