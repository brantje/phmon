# Issue #55 verification — 2026-10-01

These captures use disposable, explicitly named fixtures through the production
plugin protocol. They verify the UI and protocol integration; they do not establish
Windows/phBot runtime validation. The supplied HTML/screenshots are the layout
reference. PhMon's existing palette, resource colors and shell remain in use.

| Corrected panel | Final capture |
| --- | --- |
| Characters: underline tabs/counts, action grid, horizontal trace row, group chips, numeric HP/MP tracks, offline/stale states | [Panel](characters-panel.png), [workspace](map-characters.png) |
| Activity: aligned name/rank rows, grouped counts, unboxed events, inline range | [Panel](activity-panel.png), [workspace](map-activity.png) |
| Layers: right-aligned switches/counts, subordinate names, folded history | [Panel](layers-panel.png), [workspace](map-layers.png) |

Additional captures: [wide desktop](map-wide.png), [1280 × 800](map-1280.png),
[390 × 844 mobile](map-mobile.png), [cave controls](map-cave.png),
[legend](map-legend.png), [navigation](map-navigation.png),
[waiting for movement evidence](navigation-tray.png),
[reported arrival](navigation-tray-arrival.png).

## Checks

- `bash scripts/check.sh` passed after phases 1–7, panel corrections and final
  corrections. The final invocation used Node 24.20.0 and an isolated check-only
  `OPERATOR_ACCESS_SECRET` for Compose interpolation. Frontend lint has warnings,
  with zero errors; typecheck, unit tests and production build pass.
- `scripts/remote_controls_smoke.py` passed against isolated web/backend ports
  53055/58055: current capabilities, independent fan-out outcomes, execution-time
  positions, training readback, mandatory confirmation, void Disconnect results,
  and unsupported Clientless handling.
- Browser checks passed for keyboard character search/Enter/Escape/No matches,
  group/All targeting, inspector and overlapping portrait selection, point action
  menus, copy action reachability and tab state retention. A radius draft of 65
  survived Layers → Characters and live updates; Reset restored observed radius 50.
- Disconnect opened the existing consequential-action review for the selected
  fixtures. Cancel returned focus to Disconnect. Test commands went only to fixture
  adapters, never real characters.
- Historical controls expose all six layers, custom From/To, character and rank
  filters. Reset opens the existing scope dialog; broad reset confirmation is
  required. The verification canceled the reset without deleting data.
- Jangan Cave B1 → B6 updated the URL floor; Back to world restored world scope.
  Map legend, zoom controls and explicit point context actions remain reachable.
- Generated navigation used the existing plugin worker and validated script.
  Command completion showed “Waiting for movement evidence.” A controlled arrival
  observation then showed “Arrived”; Clear finished removed the exact tray record.
- No horizontal overflow at 2560 × 1287, 1440 × 1000, 1280 × 800 or 390 × 844.
  Desktop fills the viewport; mobile stacks the panel beneath the map. Browser
  error collection was empty.

Navigation Stop, numeric progress/ETA, observed trace state and nearby-player trace
refresh remain capability gaps tracked in [#57](https://github.com/brantje/phmon/issues/57).
Return Scroll uses command results without fabricated routes. The disposable test
stack was removed after verification; no production restart or deployment occurred.


## Operator correction: remove ordinary-click popup

The left-click coordinate/action chip and click-point marker were removed at the
operator's request. The selected-point renderer and selection state were deleted.
Background clicks close the inspector even inside a training circle. Character
markers and training labels still open their deliberate inspection controls;
Move center still consumes the next map click as an unsaved draft. Right-click,
touch context actions and Shift+F10/Menu-key actions keep navigation/training
reachable. No ordinary click opens those menus.

`bash scripts/check.sh` passed after this correction. Isolated browser verification
confirmed zero click-point markers and zero popups after a background click with
three targets selected, preserved marker inspection,
training Move center/Reset, and right-click/Shift+F10 menus with navigation, training
and Copy coordinates. Browser error collection was empty.
[Background click without a popup](map-click-no-popup.png).


## Operator correction: historical heatmaps

Range and Character use two equal-width framed dropdowns. Layer rows are 36 px
high with 26 × 14 px switches; Mob ranks uses the requested short label. Off/the
active count sits beside the header chevron. The unavailable-layer note and compact
Reset… button share the footer. Counts remain sourced from the backend.

`bash scripts/check.sh` passed. Browser verification covered the expanded layout,
custom dates, conditional rank filter and scoped reset dialog/Cancel, plus mobile
filter sizing without horizontal overflow. Browser error collection was empty.
[Updated section](historical-heatmaps-panel.png).


## PR #58 full review

CodeRabbit's requested full review of `a098ed4` completed with one accessibility
finding and three nitpicks. The fixes provide matching accessible resource labels
and bounded numeric values, require an assigned command ID for tray deduplication,
consolidate the monster HP-track CSS, and cache map action eligibility for rendering
while checking it again when clicked. All 132 frontend unit tests pass, including
missing/empty command-ID cases.

An isolated browser harness compiled the actual Vue components: eight resource
bars covered missing, valid, nonfinite and out-of-range data; all exposed matching
accessible text and consistent bounded numeric attributes. Partial Go-to searches
now close after Enter or mouse selection, reopen when typing, and close on Escape.
Browser error collection was empty. These checks use local fixtures only.
The full post-review `bash scripts/check.sh` passed.


## Action feedback live-update correction

Action feedback starts closed and toggles only through its button. Submission,
review and operation-array updates cannot reopen it or select another panel tab.
An isolated Vue browser harness using the actual page feedback markup/state and
pre-fix watchers reproduced opening after one update. After removal, nine
one-second updates plus new operations/submission/review changes preserved the
closed state and active tab. Click-open and click-close both retained the operator's
choice during updates. Browser errors were empty, and the full
`bash scripts/check.sh` passed with all 132 frontend tests.
