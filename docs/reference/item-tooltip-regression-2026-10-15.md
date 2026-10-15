# Normal / Rare Drops tooltip regression — 2026-10-15

Scope: operator report against PR #91, branch `feat/90-economy-item-analytics`.

## Cause and reproduced evidence

The Slice 13 correction removed the old temporal item join (`095a1b5`) and later
stopped promoting even stored heuristic `drop_event_id` associations into the
callback's item (`7bb50ee`). New plugin gains no longer claim such links either.
Default Drop tabs correctly exclude independent gains, so neither route supplying
observed instance details remained in these tabs. The renderer still worked.
The existing frontend test supplied an already enriched `item_details` fixture,
which did not exercise the database/API path that had been removed.

Authenticated, read-only inspection of the running instance reproduced this:

- Normal callback `2efd5bfb-fa96-436a-8f3e-62eff55e7378` has only `payload.model`.
  Inventory event `25157e6c-71b1-454c-a710-f21598f39750`, recorded one second later,
  retains its own typed API item observation and five resolved stats.
- Rare callback `d1f11535-94ab-453c-b9c1-ca407ccd26f8` also has only a model.
  Inventory event `e546770f-cc90-4595-bb2a-45bfda2538ca`, one second later, retains
  the Valor Glove observation. These are distinct canonical source events.

The [official callback API](https://plugins.phbot.org/phbot-api/events) supplies
only an equippable model for these callbacks. It supplies no item-instance or
recipient identity. Temporal proximity cannot establish pickup causality.

## Repair

The read-only Events response exposes `related_item_observation` separately from
`payload.item` and `item_details`. It retains the source event ID, timestamp,
container, source and that event's own historical item snapshot. Existing typed
item presentation resolves this snapshot independently of the callback.

An eligible stored historical association is labeled
`stored_unverified_drop_event_id`. Otherwise, a single nearby same-model gain may
be offered as `nearby_same_model_observation`: same agent, character, session,
server and nearby coordinates, within three seconds, quantity one, bag/pet
container, no ambiguous instance flag, no other stored link and no competing
same-model callback. Multiple eligible gains or competing callbacks suppress that
suggestion. These bounds restrict a suggestion; they do not prove identity.

The shared tooltip shows a separate **Inventory observation** or **Pet inventory
observation** section with the actual stats/blues and source event ID. It states
that relation to the drop is unverified and the values belong to the inventory
observation. The callback's own unknown stats remain unknown. Direct item evidence
on an event still uses the ordinary tooltip path. No current slot lookup, guessed
rolls, event collapse, canonical history rewrite or counting change is introduced.

No plugin or agent protocol change is required. Full pickup identity remains an
open capability gate, separate from access to captured item details.

## Regression gates

- PostgreSQL Events tests cover historical association detail availability,
  nearby observation/source retention, and preservation of the callback payload.
- `TestRelatedItemObservationsFailClosed` covers bag/pet suggestions, ambiguous
  copies, multiple gains, late/missing/different positions, different models and
  a gain associated with another callback.
- HTTP presentation tests resolve the checked-in real phBot Python Casque white
  rolls/blues independently for both normal and rare callbacks.
- Frontend tests verify stats, durability, enhancement and blues remain in the
  separate observation; callback values never inherit them.
- `scripts/item_detail_browser_check.mjs` reads retained normal/rare observations,
  compares details to the exact source-event response and checks the rendered
  tooltip, focus/Escape, viewport bounds and overflow at all three required sizes.

Validation and rollout results are recorded in the accompanying AGENTS.md resume
entry after completion. This repair is not Slice 13 acceptance.

Browser acceptance additionally found that an Events `event_id` subscription was
rejected by the existing live-filter validator, leaving direct source-event pages
on “Connecting live data”. The exact-ID store/API path already existed; the live
validator now accepts a valid UUID and still rejects malformed IDs. The browser
check exercises this actual source-navigation route instead of bypassing live data.
Tooltips also reposition when related details arrive while they are open.

## Follow-up — Item Search failed to load

The operator's screenshot exposed a separate missing browser transport. The new
pages called `/api/items` and `/api/economy/offers`, and Go registered both, but
neither route existed in `web/server/api`. Both returned Nuxt 404 responses. Store
integration and frontend query-helper tests could not catch the missing middle.

Both routes now use the existing authenticated same-origin proxy conventions:
operator headers, query forwarding, `Cache-Control: no-store`, bounded timeout,
no automatic retries and preserved upstream validation/error status. A new
read-only `scripts/search_proxy_smoke.py` runs in the existing Stack CI job,
checking both real proxy routes, auth, limits and validation status even with an
empty database. `scripts/item_search_browser_check.mjs` verifies actual results,
search/filter/reset, cursors, tooltips, local browser error/recovery, responsive
layout and Economy page loading. It never writes monitoring fixtures or operates
a character.

Mobile Drop names also navigate to item filters; a separate touch details button
now opens the tooltip without losing it during navigation. Keyboard focus and
Escape retain their existing behavior.

## Executed acceptance

- PostgreSQL race suites: `go test -race ./internal/events ./internal/httpapi
  ./internal/resources` passed against a separate temporary database. The database
  was removed after testing. Go vet passed for the same packages.
- All 261 frontend unit tests passed; final typecheck, production Docker builds,
  changed-file format/lint and whitespace checks passed. Lint reports two existing
  void-element style warnings and no errors.
- Both authenticated proxy routes passed the new read-only CI smoke against the
  running instance, with 401/400 preservation, forwarded limits and no-store.
- Browser checks passed at 1440×1000, 1280×800 and 390×844: Item Search loaded 769
  retained item slots; Global Economy loaded 5,410 retained messages. Search,
  reset, container filter, cursor, tooltip and local transport-failure recovery
  passed with no page overflow or browser exceptions.
- Normal Drop `9295d4ed-cbbc-4ab9-b781-ccb08603620e` displayed two source stats and
  one blue from observation `48855b74-2616-4301-af4d-f3903b4f5eef`.
  Rare Drop `d1f11535-94ab-453c-b9c1-ca407ccd26f8` displayed five source stats and
  one blue from `e546770f-cc90-4595-bb2a-45bfda2538ca`. All three viewports matched
  those values to the exact source-event API response, preserved callback payloads
  and passed keyboard/touch/Escape and tooltip-bounds checks.
- Local server/web rollout is healthy. PostgreSQL was not restarted, no source
  history was rewritten and no character command was sent. No remote commit,
  push, merge or publication was performed. Full Slice 13 acceptance is separate.

Local screenshots and machine-readable results are in
`/tmp/phmon-item-details-20261015` (`drop.item-*`, `drop.rare-*`, `items-*`,
`economy-*`, `results.json`, `search-results.json`). These are diagnostic artifacts,
not product assets.
