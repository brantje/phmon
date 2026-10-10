# Map position rendering investigation — 2026-10-10

Scope: the operator reported slow movement for `nuker4` on Greatest. Inspected
the existing LAN deployment with an authenticated, headless Chromium browser at
1440 × 1000. No bot command was sent. Measurements are short live samples, not
a controlled game-speed benchmark.

## Cause and repair

`MapCanvas` rebuilt every closed non-character popup on each marker refresh.
The CPU profile identified `markerPopup`, coordinate `toLocaleString` formatting,
DOM construction and Vue's recursive watcher traversal among the hottest paths.
Every position batch rebuilt the marker array, including nested observation data;
the deep marker watcher traversed the entire tree. Unrelated refreshes also
cancelled and restarted an in-progress marker animation with the same target.

Popups now use a lazy Leaflet content function and a bounded-by-markers current
input registry. Only an open popup is refreshed immediately. Removed markers and
unmount clear the registry. The marker watcher observes array replacement, and
map/position wire snapshots use shallow refs because their producers replace the
root value. An unchanged target leaves its current animation running; changed
targets still interpolate from the currently displayed position. Session,
sequence, scope, visibility, and stale-position rules are unchanged.

## Live comparison

Production builds used the same backend and actual character movement. An
independent Node WebSocket consumer measured delivery without map rendering.
Each browser CPU sample covered approximately 12 seconds after initial load.

| Measurement | Original build on port 3005 | Repaired build on local port 3006 |
| --- | ---: | ---: |
| Direct position interval, median | 309 ms | 304 ms |
| Browser position interval, median | 361 ms | 319 ms |
| Browser position interval, p95 | 599 ms | 584 ms |
| CPU profiler idle sample share | 3.0% | 34.0% |
| JavaScript page errors | 0 | 0 |

An earlier LAN baseline had a 536 ms median browser interval against a 337 ms
direct interval and only 1.4% idle samples. A separate repaired sample had a
302 ms browser median against a 320 ms direct median and 39.3% idle samples.
The robust finding is removal of the expensive hidden-popup work and improved
ability to process incoming positions; live timing varies with workload.

After replacing only the local web container, the browser replay passed on the
LAN address. A fresh live sample had a 313 ms median browser interval against a
312 ms direct interval, 615 ms browser p95, 43.9% idle CPU samples and no page errors. The web service
health check passed; the Go backend and connected plugins were not restarted.

The upstream stream still delivered approximately three updates per second in
these observations. Its 200 ms publication limit is an upper-rate cap, not a
guaranteed source cadence. phBot's normal callback is 500 ms; the movement packet
wake supplements it. This change does not increase sampling or change plugin
versions, and does not establish a separate backend/database latency cause.
Wire observation timestamps have whole-second precision, so they cannot prove
subsecond end-to-end latency.

## Regression verification

`scripts/map_position_browser_check.mjs` captures live contracts in memory,
then replays position and unchanged map frames only inside the test browser.
It blocks application HTTP mutations and sends no agent commands. Fixture guild
labels are confined to the browser. Run with Playwright installed, an authenticated
test/development instance, and an online character with a world-map position and
at least one rendered other-player pin:

```sh
# Supply OPERATOR_ACCESS_SECRET through the process environment; do not log it.
SMOKE_WEB_URL=http://127.0.0.1:3006 \
MAP_SERVER=Greatest MAP_CHARACTER_ID=<character-uuid> \
PHMON_PLAYWRIGHT_MODULE=<absolute-path-to-playwright/index.mjs> \
node scripts/map_position_browser_check.mjs
```

The original build fails the regression: repeated unchanged targets leave the
marker still moving after the refresh burst. The repaired build must settle
without restarting, build zero closed popups during the burst, refresh popup
data while closed/open/reopened, preserve selection/follow, and fit desktop and
mobile viewport widths. Replay results prove browser rendering behavior; actual
incoming movement is separately established by the live measurements above.

Validation: the full `scripts/check.sh` passed: Go vet/race/build, 247 plugin tests,
transport audit, 238 frontend tests, formatting, lint (87 existing warnings, zero
errors), typecheck, production build and Docker config. The script requires
nonsecret operator placeholders in its environment because `.env.example` leaves
the required operator values blank. Docker web build and the deployed LAN browser
regression also passed. PostgreSQL integration tests
requiring `TEST_DATABASE_URL` were not enabled; there are no backend/schema changes.
