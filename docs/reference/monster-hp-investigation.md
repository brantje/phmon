# Monster HP collection and map rendering

Date: 2026-10-02. The operator authorized static inspection of
`%USERPROFILE%\Downloads\phMonitor-v0.5.0.exe`, then requested its behavior for
PhMon's mob HP display. The executable was inspected without running it or
connecting to phMonitor services. No executable, extracted bundle or decompiled
source is included in this repository.

Artifact: 77,774,336 bytes, SHA-256
`6768f32f4615e9503449e24b147236a83b8ed97b3febb7f0c7e05b263f9415ae`.
This matches the artifact used by the earlier item tooltip investigation.

## Reference evidence

The embedded plugin reads `hp` and `max_hp` directly from `get_monsters()`.
The [official monster API](https://plugins.phbot.org/phbot-api/monsters) documents
those fields and identifies dictionary keys as monster IDs. The reference does
not estimate HP from a static catalog, multiply it by rank, or parse additional
HP packets in this collection path. Both HP fields participate in its snapshot
fingerprint, so changes can trigger a new observation. Collection uses the
configurable diff interval (default two seconds); its frontend also buffers
monster updates at a two-second interval.

Reproducible file offsets in this exact executable:

| Routine / declaration           | Byte offset | Observation                                                    |
| ------------------------------- | ----------: | -------------------------------------------------------------- |
| `normalize_monsters_snapshot`   |    35983504 | Normalizes IDs and raw HP fields from the phBot getter.        |
| `monsters_fingerprint`          |    35986025 | Includes HP, maximum HP, coordinates and attacking state.      |
| `visibleMapMonsters`            |    32780039 | Deduplicates by normalized server and positive monster ID.     |
| `renderMapMarkers`              |    32821967 | Computes the HP ring and stable marker key.                    |
| Monster tooltip HP calculation  |    32853251 | Shows raw current/max HP and a horizontal percentage bar.      |
| `MAP_MONSTER_APPLY_INTERVAL_MS` |    28353963 | Two-second frontend application interval.                      |
| Monster bubble CSS              |    41492082 | Red center with a conic HP ring; party and attacking variants. |

Native Go symbols provide additional evidence:
`main.mapMonsterKey` at `0x140593c00` normalizes the server and uses the positive
monster ID, falling back to name/model and half-unit position buckets when no
usable numeric ID exists. `main.(*Server).replaceObserverMapMonsters` at
`0x140596720` replaces each observer's snapshot. Aggregation in
`main.(*Server).currentBroadcastMapMonstersLocked` at `0x140595880` assigns
whole rows into the shared keyed map. Overlapping rows are overwritten during
map iteration; this is not a verified newest-HP or authoritative-observer policy.

The normal ring is `clamp(hp / max_hp, 0, 1)`. The red center stays red; the ring
shows remaining HP. If maximum HP is zero or absent, positive current HP produces
a full ring and nonpositive HP an empty ring. The tooltip bar instead stays empty
when maximum HP is nonpositive. HP-only changes update the existing bubble's CSS
property without replacing it. The inspected collection/backend/UI paths contain
no minimum-HP cache or rule that HP may only decrease.

## Independently implemented PhMon behavior

`mapMarkerPresentation.ts` now groups current sightings by normalized server and
monster ID, and the Map page uses that same identity for its Leaflet marker.
Different IDs never merge because their model/name/position happen to match;
one ID remains one marker even when observers report positions over eight units
apart. Numeric ID strings are normalized without losing integer precision.
Existing opaque protocol IDs retain their identity. The no-ID fallback uses
region and half-unit position buckets, preserving cave scope without requiring
an outdoor tile transform.

PhMon retains its timestamp-based selection of a complete source row rather than
copying the reference backend's unordered overwrite behavior. Equal observation
times use a stable session tie-break. HP and maximum HP always come from the same
chosen row. A newer higher HP value is shown as observed; no minimum is held
across polls or after disappearance. Source snapshots remain unchanged.

`MapCanvas.vue` updates the ring's CSS property in place for HP-only changes.
The popup continues to refresh raw HP and its bar. The reference's zero-maximum
ring/bar behavior is reproduced, while genuinely missing/invalid HP retains
PhMon's explicit unavailable state. Rank/label/availability/maximum-HP changes
can still rebuild icon content while preserving the Leaflet marker.

The existing authenticated plugin/Go transport, 0.1-second collection cadence,
500 ms live-snapshot coalescing, stale handling, cave filtering and historical
observation data require no contract or schema changes for this correction.
The earlier eight-unit matching rule and claim that monster IDs are necessarily
process-local are superseded by this investigation.

## Validation

- All 166 frontend unit tests pass under Node 24.20.0, including different nearby
  IDs, one moving ID across observers, server isolation, stable marker keys,
  deterministic timestamp ties, source immutability, observed HP increases and
  zero-maximum/clamped/unavailable values.
- Frontend lint passes with zero errors and 60 existing warnings; Nuxt typecheck,
  production build and focused formatting checks pass.
- A disposable browser fixture compiles the actual production `MapCanvas.vue`
  and matching helpers. At 1440 × 1000, 1280 × 800 and 390 × 844, 21 behavioral
  checks pass per viewport: damage/healing/death/respawn, independent nearby IDs,
  observer-switch marker and bubble DOM preservation, open-popup updates,
  zero-maximum values, clicks after rank changes, no horizontal overflow and
  no JavaScript errors. The fixture uses local exported map/artwork and does not
  enter a production data source or send character commands.
- Local screenshots: `/tmp/phmon-mob-hp-1440.png`,
  `/tmp/phmon-mob-hp-1280.png`, `/tmp/phmon-mob-hp-390.png`.

This verifies implementation behavior with deterministic observations. Actual
Windows/phBot reports from multiple observers, including conflicting HP values
and instanced-area identity, remain a separate runtime validation gate. The
reference's keyed aggregation does not prove that it resolves every stale HP
report correctly.
