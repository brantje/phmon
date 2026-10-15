# Item Search screenshot correction — 2026-10-15

The operator's supplied populated Item Search screenshot is the visual baseline.
The previous implementation incorrectly exposed the resource observation model
as the page design: two panels, free-text taxonomy fields and large per-item
provenance rows. Passing the API/loading checks did not establish visual parity.

## Result

`/items` now has one panel with compact Search, Item Type, dependent Subcategory
and Degree dropdowns, Include char sets and Reset Filters. Character sets are
excluded initially, as in the supplied capture. A small refresh action supports
reloading without another filter row. The header's internal observation wording
and per-item observer/revision/event diagnostics are removed.

Results are grouped by server/character, then container. Guild ownership stays
separate; distinct pet IDs remain distinct containers. Owner and container match
badges count the displayed slots. Each container shows Owner, Viewed by and Last
synced once, followed by compact icon/name tiles with page/slot labels. Source
buttons select Inventory, Info/equipment, personal/guild storage, job pouch or
Pet on the corresponding character. The shared item popup still renders the
actual item instance, including whites and blues where supplied. Exact unsigned
stack counts are retained in the tile overlay and accessible name.

Tile measurements: 64 px high, 45 px icon frame, 13 px item names, 12 px slot text.
The grid reflows within the workspace and uses one column on narrow screens.

## Data and boundary checks

The existing authenticated Nuxt → Go → PostgreSQL query remains the source of
current searchable slots. No collection, canonical history writer, bot command,
DB migration or model mapping was added. The UI traverses existing bounded
100-row cursor pages before grouping, instead of labeling a partial API page as
a whole character. It verifies totals and container revisions across pages and
retries once if records change; continued replacement fails visibly rather than
combining revisions. New server scope aborts and fences older requests. This is
per-container consistency, not an assertion of a global transaction across HTTP
pages. Closed/unavailable containers retain the existing API semantics.

Text search matches supplied names, stable codes and models. Dropdowns use each
row's server-resolved static metadata, without interpreting one server's numeric
model on another server. Equipment categories use exported `sort_type`; alchemy
and mall categories use explicit `ITEM_ETC_ARCHEMY_` / `ITEM_MALL_` namespaces.
Unknown items remain in Any type. Subcategories and degrees are offered only
where present; a numeric suffix in a consumable's code is not invented as degree.

The old per-item canonical-event diagnostic link is intentionally absent from
this reference layout. Canonical Events/history routes and their query helper
remain; a reference-compatible exact-item history entry is a separate open
Slice 13 presentation gate, not claimed complete by this correction.

## Verification evidence

Public demo navigation was inspected again in Chrome at 1440×1000. Item Search
became selected in navigation, but its content did not render within the bounded
wait. No private endpoint or bundle was inspected. The operator's populated
screenshot, rather than that blank public view, governs this change.

The browser acceptance script checks the actual authenticated transport and
retained rows: complete owner/container counts, text search/reset, sets,
category/subcategory/degree behavior, single-item popup, keyboard/touch/Escape,
correct source tabs, local transport failure/recovery and page bounds. Required
captures/results are in `/tmp/phmon-items-parity-20261015`. Its simulated 503 is
confined to the test browser and never becomes monitoring data.

New unit coverage exercises colliding numeric models across server profiles,
unknown definitions, guild/pet separation, bag slot offsets, cursor traversal,
changed revisions and broken/incomplete pagination. Existing drop acceptance is
rerun because item tiles reuse the shared tooltip component. Final execution
results are recorded in the accompanying AGENTS resume entry.

Final local validation: 274 unit tests, typecheck, formatting, changed-file lint
(zero errors; four void-element style warnings), production build and diff checks
pass. The healthy running web image `871b1868d9cb` passed the complete search flow
again at all three required sizes, against 777 current slots / 5,446 messages.
All three Drop/dashboard captures passed their shared-tooltip regressions; the
pet source gap remains unchanged. No whole-Slice completion is claimed.
