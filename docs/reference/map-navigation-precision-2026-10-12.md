# Outdoor map navigation precision — 2026-10-12

The operator reported right-click → Navigate to failing and explicitly authorized
tests with LordLurks. Testing used the existing checkout and local instance,
LordLurks on Greatest, plugin **1.9.36**, protocol **19**, phBot **20.1.3**. No
commands were sent to other characters.

## Observed failure boundary

The right-click menu prepared the correct target/session/destination, and the
authenticated Go admission/dispatcher delivered the command. The first command
was acknowledged in approximately **24 ms**; it failed inside plugin path
generation before `start_script`. Fresh character/map observations continued.

| Command suffix | Destination X, Y, Z | Result |
| --- | --- | --- |
| `cf155ae6-784a-47ad-9b39-27d745a591f5` | 6461.3999, 1097.4000, -32 | `command_expired` at the existing ten-second deadline |
| `ed6c85d1-425b-4f54-bbbd-0c095ab2ffb7` | 6432.0555, 1078.2, -32 | `path_not_found`; operator log records generation returning after 865 ms |
| `dcd02ba4-d62e-40d0-aff7-dc7d719bc464` | 6435, 1078, -32 | `api_return=true`; three route steps; arrival observed |
| `7696ca8a-c089-4038-9d25-5a04ddf146de` | 6435.0563, 1093.8973, 0 | Right-click command expired during generation |
| `05ef3243-7664-42d5-8239-f1e8e3e8d455` | 6435, 1094, 0 | `api_return=true`; three route steps; arrival observed |
| `e458eaf1-8ab2-4028-8792-af99410bd0b4` | 6435.05, 1078.1, 0 | `path_not_found`, approximately 9.26 s after admission |
| `7a7521f7-9424-4277-a7ed-73726bdd2ea0` | 6435, 1078, 0 | Patched right-click UI: accepted in approximately 833 ms; **Arrived** in live tray |
| `c533e172-6a99-4986-9261-c9cc2293062c` | 6436, 1094, 0 | Deployed UI: `path_not_found` after approximately 1.48 s |
| `ebe240fa-3247-4500-a41e-8c9b43a02400` | 6435, 1094, 0 | Deployed right-click UI: native True in approximately 1.15 s; exact-command **Arrived** observed |

Full IDs start with `cmd_`. The paired decimal/whole-unit attempts support a
precision workaround on this installed runtime. They do not establish phBot's
internal pathfinder cause or promise a path to arbitrary obstructed destinations.
No path fallback, native restart or automatic command retry was used.
The deployed `(6436, 1094)` failure also confirms that whole-unit normalization
does not guarantee native path availability, even one unit from a successful
endpoint. PhMon does not identify a destination as walkable without native path
evidence, and this investigation does not diagnose the remaining native refusals.

The official [Paths API](https://plugins.phbot.org/phbot-api/paths), checked on
2026-10-12, documents `generate_script(region, x, y, z)`, its None/False/list
results, and the five-second generation rate limit. Its example uses whole
coordinates, but it does **not** explicitly prohibit decimals. The official
[Script API](https://plugins.phbot.org/phbot-api/script) documents starting script
text in the background. Signatures and collection/mutation mechanisms are unchanged.

## Independent fix and checks

Outdoor map navigation resolves the clicked point, rounds X/Y to whole game
units, and clamps edge rounding to the original encoded region. This happens
before immutable request preparation and final admission, so the request, its
summary and its exact retries agree. Z retains its existing per-character frozen
value. Cave transforms, training centers, map displays and direct movement keep
their existing precision. No plugin/backend/protocol change is required.

Focused tests pass **44/44**, covering decimal preparation/admission, positive
and negative tile-edge fencing, cave/training precision, existing exact retries,
sessions and fan-out. All frontend unit tests pass **251/251** on Node 24.20.0;
Nuxt typecheck, changed-file ESLint, Prettier and `git diff --check` pass.

The actual right-click test against the patched development frontend produced
command `cmd_7a7521f7-9424-4277-a7ed-73726bdd2ea0`, rounded `(6435, 1078)`, a
confirmed native script result and a fresh-position **Arrived** observation.
Screenshot: `/tmp/phmon-navigation-fixed.png` (local inspection evidence).
An initial development browser run failed during Vite dependency preparation
before the menu mounted and sent no command; the subsequent run completed.

The local production web rebuild uses an isolated source context that excludes
pre-existing Players working-tree edits. The backend/database/plugin are not
restarted. The existing approved Map layout remains the visual baseline; this
targeted bug fix does not claim a new demo comparison or full roadmap completion.

Rollout completed: only Compose `web` was recreated and is healthy. The deployed
browser test asserted HTTP 202, whole-unit arguments, zero browser errors, and
an `arrived` map frame for the exact admitted command
`cmd_ebe240fa-3247-4500-a41e-8c9b43a02400`, then captured
`/tmp/phmon-navigation-production-fixed.png`. LordLurks returned to the initial
Jangan neighborhood `(6435, 1094)`. The disposable development server was stopped.
