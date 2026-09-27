# Slice 2.5 screenshot and asset coverage ledger

Reference evidence, 2026-09-26. All 32 supplied screenshots and all five baseline
reference images were visually reviewed. The screenshot proves a presentation
need, not its source filename, runtime semantics or availability in GreatestSRO.

Implementation outcomes are recorded below by asset family and dataset. Source
entry/hash provenance stays in exporter-only audit files, never in app-facing bundle
data. Screenshots remain reference material; never crop their assets into the application. The
`phmonitor_screenshots/` directory was already untracked and remains untouched.

## Confirmed exporter boundary (2026-09-27)

Use GreatestSRO archives for all client assets, including maps. Export finished
browser-ready files plus normalized JSON for copying to PhMon. No application-side
PK2 support, database importer or profile-management UI is required. The preview is
standalone and reads only exported files. `Map.pk2` was selected; its identical-size,
different-hash copy is excluded as backup. Direct minimap raster tiles are read from
GreatestSRO's `Media.pk2`; no terrain renderer is needed. Do not substitute phBot
minimap tiles. Operator excluded sounds and interface controls; non-control symbols
remain in scope as candidate assets. Unverified mappings must remain unresolved.

## All supplied screenshots

All filenames below are under `../phmonitor_screenshots/`. Shared shell requirements apply to every row: page/sidebar symbols, atmospheric background and top-strip resource imagery. PhMon supplies its own identity and generated instance QR; advertisements, Premium badges, server-directory graphics and streaming are excluded. Non-control symbol art is prepared as candidates; interface controls and control artwork are outside Slice 2.5 scope by operator instruction.

| Screenshot | Visually observed | Asset/catalog preparation for later slices |
| --- | --- | --- |
| 02-dashboard.png | Populated counters, skull death entries, unique illustrations, server artwork, event/drop/chat/offer cards | Gold/stat/event/category icons; unique art; optional operator server-card image; backdrop |
| 02-stats-01.png | Group summary with Hotan-area minimap thumbnail | Map tiles available to small previews as well as main map; stat icons |
| 02-stats-02.png | Character portraits, individual minimaps, Ironclad Horse image, gold/SP cards | Character-model portraits; pet role mapping; map imagery; gold/SP and skill navigation icons |
| 02-stats-03.png | Equipment columns around character portrait, empty slots, plus overlays | Weapon/armor/accessory icons, portrait, slot treatment; plus values remain runtime data |
| 02-stats-04.png | Progress/rate panels and training bar | Shared portrait/icons; bars and charts are authored UI, not extracted images |
| 02-stats-05.png | Start/stop training/trace, area/radius, return, walk, disconnect, script action buttons | Controls are excluded; do not infer API support from their appearance |
| 03-phbot-tools.png | Client, Party, Scripts, Quest icons and clientless control | Category symbols only; controls are excluded and API support is established elsewhere |
| 05-alchemy.png | Statistics and weekday chart, Sessions tab | Alchemy page icon; prepare all alchemy item/material icons from roadmap, although sessions are not shown |
| 05-deaths.png | Character portrait per death, map-link icon | Portraits, death and location icons; region names |
| 05-history-01.png | Skull events and Tiger Girl, Captain Ivy, Uruchi, Ghost Unique images | Event symbols and unique entity mappings; source evidence required for custom Ghost Unique |
| 05-history-02.png | Empty level-up timeline | Level-up/custom event symbols needed from roadmap; empty state shows no individual event asset |
| 05-normal-drops.png | Vulpecula Pearl Ring icon/name | Accessory icon/name/taxonomy; all normal-item families, not just ring example |
| 05-rare-drops.png | Empty rare-drop table, item and map columns | Rare-item icons and semantic detail support; no observed seal/blues can be inferred from this empty table |
| 05-uniques.png | Large cutout-style Tiger Girl, Captain Ivy, Uruchi; different Ghost Unique image | Higher-resolution unique art role separate from tiny icons; names/models/types; HP/spawn facts remain live |
| 06-chat.png | Settings → Chat notification options, not conversation UI | Shared chat icon; channel roles; use baseline chat screenshots for actual composer evidence |
| 06-sounds.png | Event sound selectors, Preview, custom WAV section | Reviewed; audio assets are excluded by operator instruction |
| 07-map.png | Populated desert tiles, portrait markers, character picker, layer controls | Full map/minimap families and region/destination metadata; the screenshot shows one coordinate/marker point, but exact tile placement and marker anchor remain unvalidated |
| 10-conditions-01.png | Empty Conditions page | Automation icon and shared shell; no unique content bitmap needed |
| 10-conditions-02.png | Academy join trigger and party-list action editor | Reuse academy/party/character assets; dropdown semantics belong to later automation implementation |
| 10-discord.png | Empty webhook setup page | Shared settings icon; optional locally bundled licensed integration icon; no remote branding dependency |
| 11-schedules-01.png | Empty schedules page | Shared automation icon; no separate game-image family |
| 11-schedules-02.png | Repeat schedule, weekdays, target selector and action editor | Shared target/action imagery; calendar controls authored in UI |
| 12-analytics-01.png | Death charts, skull/location/character summary icons | Death icon, portrait, map/location and summary symbols |
| 12-analytics-02.png | Empty rare-drop charts and summary cards | Rare/item-type/character symbols; complete item taxonomy for future chart grouping |
| 12-analytics-03.png | Normal-drop chart with Accessories grouping | Accessory taxonomy and item/category icons; chart is generated from later history |
| 12-analytics-04.png | Gold chart and character/guild/stall summary | Gold, character, guild and stall-sale icons; shared item taxonomy |
| 12-analytics-05.png | Academy charts and graduation/leave/academy summary | Academy, graduation and leave/kick symbols; do not assume a separate Academy page was inspected |
| 13-global-offers.png | WTB/WTS/WTT and text/type/subcategory/degree filters | Complete item taxonomy/localized names; offer/stall icons; prepare item imagery although results are empty |
| 13-guild-storage.png | Populated slot grid, diverse equipment/materials, plus overlay and gold | Reuse complete item icons, empty slots and gold symbol; no separate guild-storage copies |
| 15-settings.png | Portrait size previews 45/60/75, text sizes 11/14/18 and notification toggles | Portrait candidate art at native resolution; scalable semantic image rendering; local background art. No sound assets are exported. |
| 2.5-item-search.png | Inventory and guild grouping; potions, scrolls, arrows, gear, elixirs, stones, pet items | All item classes and exact taxonomy joins; fallback visible for Wings of Fire reinforces missing-icon handling; quantity/plus are overlays |
| 2.5-skill-builder.png | Chinese/European choice, caps 110/120/140, current/future skill panels, frames, tabs, gold category symbols, empty/locked cells and arrows | Skills/masteries/groups and verified labels/icons; cap/tree/cost/prerequisite rules unresolved; interface controls/frames excluded |

## Existing baseline images also reviewed

| File under docs/reference/ | Contribution |
| --- | --- |
| phmonitor-dashboard.png | Empty easy-mode dashboard; shell/category imagery, backdrop and optional artwork slot |
| phmonitor-stats.png | Empty group state; shared shell assets |
| phmonitor-chat.png | Actual desktop conversation tabs, contacts, plus/item-reference and emoji composer controls |
| phmonitor-chat-mobile.png | Chat header icon and responsive reflow; no new asset family |
| phmonitor-map.png | Empty map, navigation and layer panel; no evidence of actual tile transforms |

## Slice 2.5 execution record - 2026-09-27

**Status:** the standalone exporter, real archive export, copied-bundle validation and
preview are complete in the confirmed scope. Asset readiness is **incomplete**. The
real manifest deliberately says `incomplete`; passing exporter validation proves
bundle integrity, not that all required client semantics and artwork were found.

All 32 supplied screenshots and all five `docs/reference/` captures listed above were
reviewed. No screenshot/reference image was modified or copied into the export.

### Public asset aliases follow-up

The separate optional Nuxt public tree publishes browser-friendly aliases such as
`game-assets/icon/skill/china/bow_area_a.png`, `game-assets/icon/item/...`,
`game-assets/icon/cos/...`, `game-assets/interface/character/...`, and
`game-assets/minimap/...`. Its JSON index maps verified semantic asset keys to each
public URL without raw source-entry fields; exact provenance remains in
`audit/assets.json`. Character files remain portrait candidates, and entity icon
paths remain associated-icon joins; they do not establish portrait, pet, monster or
unique roles. The default `web/public/game-assets/` target is ignored by Git.

The real exporter `0.4.0` wrote 10,171 unique public paths covering 30,276 semantic
keys. The copied `game-assets/` folder validates with no audit or archive access, and
Nuxt served the sample skill URL as HTTP 200 `image/png` with the indexed checksum.
All previous family counts/blockers remain unchanged; the new aliases do not assert
additional portrait, pet, monster or unique roles.

### Real output and coverage

- Dataset `gamedata-66e9e3ee636c5a2a3f8f`; exporter `0.3.1`; schema `1.1.0`.
- Bundle: `exports/greatestsro/gamedata-66e9e3ee636c5a2a3f8f/bundle/` - 17
  catalogs, 8,921 unique converted PNG files, 30,276 semantic asset keys,
  465,088,731 bytes. The ignored sibling `audit/` is 24,200,372 bytes and is not
  needed by consumers.
- Archive selection is recorded only in ignored `audit/sources.json`: `Media.pk2`
  hash `dad25eddec0b61054fa2e55cba738ca308873441da4cc6960b18616612a6b4f8`;
  `Map.pk2` hash `a819141950fed83d2a293eed6ebb96e22ec6f8eceb407eb677fbc06656562183`.
  `Map - copia.pk2` is the operator-designated backup and was excluded. The 5,303
  direct 256 x 256 minimap images are in GreatestSRO `Media.pk2`: tile set 001 has
  5,118 records and set 002 has 185. `Map.pk2` was inventoried and its `tile2d.ifo`
  entry noted, but no terrain renderer was built and no transform was claimed.

| Family | Parsed/output counts | Verified links or rendered art | Outstanding coverage |
| --- | --- | --- | --- |
| Items | 14,238 records; 9,308 names | 14,227 associated icons converted | 4,930 names unresolved; all 14,238 descriptions unresolved; 11 missing icons; taxonomy, degree, requirements and static properties unverified |
| Entities | 18,699 records; 15,905 names | 9,302 associated icons converted | 2,794 names and 294 icons unresolved; no portrait, pet-body, full-body or unique-art joins |
| Skills | 29,664 rows from the seven `skilldata.txt`-indexed shards; 5,432 names and 5,409 descriptions | 6,026 rows have associated icons; 601 unique referenced icon images | 4 referenced icons missing; 24,232 names and 24,255 descriptions unresolved; ranks, costs, prerequisites and tree/cap rules omitted; 366 unreferenced art candidates |
| Masteries | 18 rows; 6 names/descriptions | 14 rows with header-verified icon roles | 12 names/descriptions unresolved; 5 art candidates unmapped; caps and requirements omitted |
| Skill groups | 140 rows; 117 labels | All 140 join to exported masteries; no missing referenced icons | 23 labels unresolved; ordering is per verified group sequence only |
| Regions | 2,471 records | 2,449 exact root-minimap links by `(gridX, gridZ) == (x, y)`; each matched record has a `mapAssetKeys` reference | 22 region records have no exact root tile (17 at `(0,0)`, five nonzero pairs); in-tile transforms and special-area registration remain unvalidated |
| Teleports | 222 rows; 183 names | 193 region joins; 294 endpoint links with no unresolved endpoints | 39 names and 29 region joins unresolved; coordinates, fees and eligibility omitted |
| Minimap tiles | 5,303 raster tiles in two sets | All emitted as 256 x 256 PNG; 2,449 root-set tiles linked to region rows by exact grid indices; both tile sets independently support X increasing right and Y increasing up | `arabia` remains geographically unjoined to regions; in-tile world transform, marker anchor, outdoor coordinates and Jangan Cave / Donwhang Cave / Job Temple registration remain unvalidated; overview sheets contain gaps |
| Backgrounds | 93 records | Converted and visually sampled | Candidate background use is not a semantic screen-role mapping |
| Portraits | 52 candidates | Converted and visibly reviewed | Zero entity/character joins; candidates are not a completed portrait catalog |
| Pets | 0 verified role records | Explicit unresolved catalog | No body art or Attack/Fellow/Pick/Transport mapping verified |
| Non-control symbols | 159 candidates across 16 audit groups | Converted and visually reviewed | Bounded path-pattern candidate set; source filenames do not establish PhMon semantics |
| Sounds / interface controls | 0 records and no extracted art | Explicit `not-in-scope` catalogs per operator request | Excluded by operator, not a failure |
| Taxonomy / localization / UI support catalogs | 0 standalone records | Localized strings are embedded only where exact joins were verified | Item taxonomy and unmatched text remain unresolved; `ui.json` points to the symbol candidate catalog |

The exporter parsed only the seven shards named by `skilldata.txt`. Twelve unlisted
skill-table candidates (including the 40,000-level shard and encrypted/alternate
files) were recorded in audit and excluded because they are not part of the verified
source list. No rank/requirement semantics were inferred from them.

The audit records eight base unresolved families (taxonomy, entities, skills,
masteries, skill groups, portraits, pets, teleports) and additional individual item
and entity name/icon gaps. The manifest coverage records those counts. These source
gaps, plus unvalidated map placement and absent portrait/pet/unique/quest joins, are
blockers to claiming all-assets-ready. Sounds and controls are omitted by the operator
and are not blockers.

### Repeatability and standalone validation

First real export completed in 178.797 seconds. A same-input rerun took 189.36 seconds,
returned the same dataset ID and reused byte-identical bundle output. Five working-set
samples observed a maximum of 165,826,560 bytes (~158 MiB); sampling does not prove a
true peak. One earlier run safely aborted before publication when the skill ID field
failed a uniqueness check; the field was changed only after cross-checking uniqueness
across the seven indexed shards.

Only `bundle/` was copied to `exports/standalone-copy-0.3.1/`; the copy contains only
`assets/`, `catalogs/` and `manifest.json`, with no archives or `audit/`. The copied
bundle was validated and served by the preview using only its bundle path, with no
source/audit path provided. Validator result: 17 catalogs, 8,921 assets, 30,276
semantic keys, 0 dangling asset references, normalized relations validated,
`sourceKnowledgeRequired: false`. The source archives remained separate and read-only;
no source path was passed to the validator or preview. The synthetic fixture test
also deletes its source directory before validating its copied bundle.

Commands used (PowerShell):

```powershell
$python = "tools/game-data-exporter/.venv/Scripts/python.exe"
$source = "C:\Users\sander\Documents\Silkroad Online\GreatestSRO"
& $python -m phmon_game_exporter.cli export --source $source --output exports/greatestsro
Copy-Item -Recurse -LiteralPath "exports/greatestsro/gamedata-66e9e3ee636c5a2a3f8f/bundle" -Destination "exports/standalone-copy-0.3.1"
& $python -m phmon_game_exporter.cli validate --bundle "exports/standalone-copy-0.3.1"
& $python -m phmon_game_exporter.cli preview --bundle "exports/standalone-copy-0.3.1"
```

The first real export and repeat were run before the documented local virtualenv was
created; the final reproducibility setup used Python 3.12.10 with the pinned Pillow,
PyCryptodome, setuptools and pytest dependencies from the tool files.

### Preview and visual evidence

Previewed items/entities/skills/masteries/groups/teleports/portrait candidates/pet
coverage/non-control symbols/backgrounds and both map sets from the copied bundle.
All 159 symbol candidates and 600 sampled skill cards loaded with zero broken images;
no external requests were observed. Exact CSS viewport checks at 1440 x 1000,
1280 x 800 and 390 x 844 found no horizontal overflow. The browser outer window has a
500 px minimum, so the viewport checks used fixed-size same-origin frames; 390 px was
the CSS viewport under test. Screenshots and map-sheet previews are ignored under
`exports/standalone-preview-*.png`; they show exporter review UI only, not PhMon page
parity. Portrait art was visually reviewed but remains unmapped, and pet roles are
shown as unresolved in the preview. The map sheets show source tile-index arrangement
only, not an asserted world map.

### Final validation

```powershell
$python = "tools/game-data-exporter/.venv/Scripts/python.exe"
& $python -m compileall -q tools/game-data-exporter/src tools/game-data-exporter/tests
& $python -m pytest tools/game-data-exporter/tests -q
& $python -m phmon_game_exporter.cli --version
& $python -m phmon_game_exporter.cli validate --bundle "exports/standalone-copy-0.3.1"
```

Results: compilation passed, 17 fixture tests passed, CLI version was
`phmon-game-data 0.3.1`, and copied-bundle validation passed as summarized above.
`git check-ignore` confirmed the copied bundle and preview screenshots are ignored;
no extracted assets/audits are tracked. No PhMon service, PostgreSQL, Docker or
phBot process was required or used.

**Stop after Slice 2.5.** This task did not start Slice 3. Use only verified exported
fields in later work; revisit the recorded gaps before treating affected catalogs as
complete.

## Required families beyond visible populated examples

- Full supported item set: both races, equipment/avatars, accessories, stackables,
  consumables, alchemy, pet-related items, storage/guild/pet inventory reuse.
- All supported character portraits and pet/entity types; party and Academy member
  markers, NPC and mob symbols, unique artwork at card and event-list sizes.
- Both races' full supported skill/mastery icon sets, descriptions, tree metadata,
  costs and requirements; race/mastery symbols and Skill Builder frame/state parts.
- Outdoor maps, town previews, every available dungeon/floor family, especially
  Jangan Cave/Tomb of Qin-Shi, Donwhang Cave/Donwhang Stone Cave, Job Temple/Temple.
  Maintain a separate inventory and transform-validation status for each.
- Quest names/icons/reference records where present, guild/party/academy symbols,
  item-description labels and magic-option names/units. These are roadmap needs;
  the screenshots do not prove their exact data representation.
- App shell/background and tool/event/stat/category symbols; action controls are excluded.
  Source missing decorative art from original/licensed/operator material.
- Sound-role assets were considered from the screenshots but are excluded by the
  operator. No sound is required or substituted.
- Interface controls, action buttons, frames, locked-cell art and state variants are
  excluded by the operator. The scoped interface family contains non-control symbols.
- Emoji/item-reference imagery if the verified client supports it; use normal
  local/system emoji where appropriate, not a copied phMonitor emoji bundle.

## Region/minimap index follow-up — 2026-09-27

Exporter `0.4.1` / schema `1.2.0` adds `regions.mapAssetKeys` links only for exact
root-minimap coordinate pairs (`gridX/gridZ` to tile `x/y`). Real dataset
`gamedata-00cdb146f754911f1c80` linked 2,449 of 2,471 region records; 22 remain
without a root tile, including the 17 special/placeholders at `(0,0)` and five
nonzero grid pairs. The `arabia` tile set remains separate and unjoined. The world
coordinate transform remains explicitly unvalidated.

The exporter fixture suite passed 18 tests. Two real npm exports from the same
GreatestSRO source completed in 236.349 s and 247.297 s; the second reused the
byte-identical bundle. A clean copy containing only `bundle/` plus a separate
`game-assets/` copy validated 17 catalogs, 8,921 bundle PNGs, 30,276 keys, 10,171
public files, 30,276 public mappings and zero dangling references, with
`sourceKnowledgeRequired: false`. The preview served its copied page, manifest,
map catalog, tile sheet and live-region PNG with HTTP 200. Both copies and all
extracted outputs remain ignored by Git.

A read-only GET at 03:06 UTC returned four online characters in region 25735;
`regions.json` links that region to tile-set-001 `(135,100)`. This verifies region
index selection only. The populated reference screenshot does not uniquely
establish the live markers' pixel transform; see [the minimap verification](minimap-verification.md).

A follow-up GET at 03:27:56 UTC again found four online characters in region 25735
(X=98.1–101.5, Y=1551.1–1560.2; names omitted). This confirms the same tile index
selection only. The screenshot is not linked to those API identities or timestamp,
so it cannot validate their in-tile pixels.

An archive-only edge-continuity pass over all 5,118 root PNG tiles found 4,806
horizontal and 4,721 vertical neighboring pairs. X-increasing-right had mean 8-pixel
edge error 17.010 versus 36.426 for left, winning 4,132 pairs (394 left, 280 ties).
Y-increasing-up had mean error 16.576 versus 36.364 for down, winning 4,100 pairs
(336 down, 285 ties). The separate `arabia` set also supports X right/Y up from 163
horizontal and 165 vertical pairs but remains unjoined to regions. Exporter 0.4.2
records per-set evidence and the standalone overview lays out sets only on axes
meeting its explicit pair-count, win-share and mean-error thresholds. This improves
tile-grid placement; it does not validate in-tile world coordinates or the live
character marker anchor.

The 0.4.2/schema 1.2.1 real export produced dataset
`gamedata-21789415d0f1593162dd` in 242.804 s: incomplete status, 17 catalogs,
8,921 unique images, 5,303 minimap tiles, 30,276 semantic keys, 2,449/2,471 exact
region/tile links, and nine unresolved coverage entries. A second real npm
export took 254.988 s and reported `identicalBundleReused: true`. Bundle size was
465,397,123 bytes; exporter-only audit size was 24,204,249 bytes. The default Nuxt
tree validated 10,171 aliases and all 30,276 public keys. Combined bundle/public
validation reported zero dangling references and `sourceKnowledgeRequired: false`.

A copy containing only `bundle/` (8,939 files: assets, catalogs and manifest; no
audit or archive) validated 17 catalogs, 8,921 assets and 30,276 keys from its own
directory. Its standalone preview returned HTTP 200 for the page, manifest, map
catalog, tile 135x100 and both tile-set sheets. Root and `arabia` sheets were
visually reviewed in the ignored `exports/minimap-orientation-check-0.4.2/` folder.
The exporter fixture suite passed 20 tests; compileall and CLI version checks
passed. World-coordinate-to-pixel mapping remains explicitly unresolved; the
current app still has no map surface and no marker-placement implementation was
added.

## Follow-up — full phBot reference and live-row audit (2026-09-27)

The phBot reference folder contains 5,794 root `XxY.jpg` tiles and 2,224 files in
`d/`. The exported GreatestSRO root aliases contain 5,118 names: 5,117 overlap,
677 exist only in the phBot comparison folder, and `110x103` exists only in the
GreatestSRO export. All 5,117 overlapping files are 256 × 256; their RGB MAE is
4.9552 on average (median 4.7016, p95 6.4051). Thirty outliers (MAE > 10) are
concentrated at X=217–218, Y=84–99 except 91. Those GreatestSRO DDJ sources decode
to uniform opaque black, while the corresponding phBot JPGs contain terrain. The
raw `217x96.ddj` payload confirms the black pixels are in the archive source, not
introduced by conversion. No alternate entries were found in GreatestSRO
`Data.pk2`, `Particles.pk2` or `Music.pk2`; the selected `Map.pk2` does not provide
replacement raster tiles. These 30 tiles and the 677 phBot-only references stay
unresolved; phBot files remain comparison-only.

The installed phBot folder itself is flat and names images `XxY`; the GreatestSRO
archive path convention was verified independently: `Map\97\168.o2` exists,
`Map\168\97.o2` does not, and Media contains `minimap\168x97.ddj`. All six named
Jangan tiles and all four Donwhang tiles exist in both sources and have mean RGB
MAE 5.8430 and 5.0927. The live tile 135x100 has MAE 2.9955.

A 04:26 UTC read-only API GET returned four online characters in region 25735, all
linked to root tile `(135,100)`; their X range was 94.926–102.840 and Y range was
1556.002–1562.600. One row matches the screenshot's selected-character label and
is near its displayed coordinates, but its state is more than seven hours later
than the screenshot file timestamp. A single-tile crop weakly favors candidate
increasing-Y-down placement, while the broad seam/mosaic evidence favors
increasing grid-Y-up; pan, world transform and marker anchor remain unvalidated.
See [minimap verification](minimap-verification.md). No source substitution or
coordinate projection was made.

### Live-region tile recheck (2026-09-27 05:36–05:38 UTC)

Four API rows had recent `state_updated_at` values. Three Region 25735 rows joined
to root tile `(135,100)`; one Region 24453 row joined to `(133,95)`. Both joins
were exact and both source tiles contained nonblack pixels. A later query for the
screenshot-selected character again returned Region 25735 and tile `(135,100)`.
The screenshot is roughly 8 hours 54 minutes older; matching tile selection and
nearby X/Y values do not prove exact marker placement. Tile 133x95 compared with
the phBot reference at RGB MAE 5.1911. The current app still has no Map component
or live marker layer, so production pixel placement cannot be reviewed in Slice
2.5. See the expanded evidence and limits in [minimap verification](minimap-verification.md).

## Follow-up — asset aliases and black-source map coverage (2026-09-27)

Exporter `0.4.3` / schema `1.2.2` now marks exact opaque-black minimap PNGs as
`uniform-opaque-black` in the map catalog and linked region rows. Of 5,303 tiles,
207 have that pixel content: 193 in root tile set 001 and 14 in secondary set 002.
The map family remains partial. The 30 shared-name phBot comparison outliers are
the subset with visible reference terrain; no comparison image was substituted.
The independent region join still has 22 records without exact root tiles. Together
the audit reports 229 unresolved map records (black rasters plus unmatched region
joins). `Map.pk2` remains the selected map archive and the `- copia` file remains
the designated backup; direct minimap raster assets are from GreatestSRO `Media.pk2`.

Real npm export produced dataset `gamedata-47c969ded0613d4c2a22`: 17 catalogs,
8,921 unique converted PNGs, 30,276 semantic keys, 5,303 map tiles, 14,238 item
rows, 18,699 entities, 29,664 skills, 18 masteries, 140 skill groups, 222
teleports, 93 backgrounds, 52 portrait candidates and 159 non-control symbols.
The separate audit remains exporter-only. The public Nuxt index covers 10,171
browser paths and all 30,276 verified keys. `icon/skill/china/bow_area_a.png` is
present; there are 3,183 `icon/item` aliases and no `icon/mob/` folder. The 9,302
entity-associated icon keys resolve through existing `icon/cos`, `icon/pet2` and
`icon/etc` paths, but no monster or unique art role is verified. There are 57
`interface/character` candidates and zero entity portrait joins. Keep these role
gaps explicit for later slices; the public folder structure does not invent them.

The explicit `--asset-output public/game-assets` run wrote the Nuxt tree; a second
`npm run export:assets` invocation without that parameter completed in 268.352 s,
reused the byte-identical dataset and wrote to the default Nuxt destination.
Source-independent validation passed on
the new bundle plus public tree (zero dangling keys and valid normalized
relationships) and on an 8,939-file bundle-only copy. A live preview served the
copied bundle; the 2,724 × 1,104 root sheet showed the opaque-black gap hatching.
All exported files remain ignored and outside Git. The family status stays
incomplete; do not promote mapped item/entity icons to unsupported monster,
portrait, pet or unique roles.

### Latest copied-preview and live tile lookup check (2026-09-27 05:52 UTC)

A fresh read-only character API response contained four recently updated online
rows, all in Region 25735. The exported region catalog links each by exact grid
match to tile `(135,100)`, whose GreatestSRO raster has nonblack pixels. The copied
bundle-only preview served its page, manifest, map catalog, the resolved tile PNG,
and both generated tile-set sheets with HTTP 200 and expected MIME types. The
world-coordinate pixel transform remains unvalidated; see
[minimap verification](minimap-verification.md).

The current checkout's Nuxt dev server also returned HTTP 200 for the public
minimap tile, `icon/skill/china/bow_area_a.png`, and asset index, with the expected
PNG/JSON content types and response hashes matching local public files. Exporter
preview and Nuxt static serving are verified separately.

The 2026-09-27 06:05 UTC recheck compared 12 selected GreatestSRO raster aliases
against the installed phBot reference: all were 256 × 256; mean RGB MAE was 5.8430
for the six Jangan tiles, 5.0927 for four Donwhang tiles, 2.9955 for `135x100`, and
5.1911 for `133x95` (12-tile mean 5.3013). Four fresh live rows all joined exactly
from Region 25735 to nonblack tile `(135,100)`. Their capture is not synchronized
with `07-map.png`, so only tile selection—not pixel placement—is verified; see
[minimap verification](minimap-verification.md).

The installed phBot reference folder's 5,794 root JPGs were also seam-checked
independently: 5,407 horizontal pairs support X increasing right (mean edge RGB MAE
17.193 versus 35.320; 4,656 wins) and 5,298 vertical pairs support Y increasing up
(16.555 versus 35.401; 4,647 wins). This supports the grid sheet orientation but
does not validate in-tile coordinates or live marker pixels.

A newer 07:32:48 UTC live API snapshot expands checked map coverage: three rows
joined from Region 25735 to `(135,100)` and one from Region 23941 to `(133,93)`;
both raster records are exact-grid, nonblack and present in the installed phBot
reference. The 133x93 image comparison is RGB MAE 5.5813. These checks verify
region-to-tile selection, not rendered positions inside a tile.

## Audit rules

For operator-excluded sounds and interface controls, record `not-in-scope`; empty
status catalogs are not missing work.

Each in-scope family needs **discovered → parsed → converted → mapped → output-validated → visually
reviewed** evidence. Record
counts at every step and investigate discrepancies. A directory listing or one
sample icon does not establish coverage of a family. Every supported referenced
asset must resolve; unsupported references remain individually reportable.

Separate asset readiness from later feature implementation. Keep screenshot-visible
requirements separate from roadmap-derived requirements and source availability.
No screenshot covers every pet inventory, item tooltip, quest or dungeon; these
must still be investigated. Do not omit unseen screens from the final audit.

## Operator-directed Git tracking update (2026-09-27)

After the export and coverage checks above, the operator requested that the Nuxt
public asset tree be committed with the application. The tracked tree contains
10,171 browser asset aliases plus `asset-index.json` (10,172 files; 487,358,758
bytes). This supersedes the earlier ignored-output notes above, which describe the
state at the time those checks ran. The source PK2 archives and exporter-only audit
remain outside Git. Coverage and semantic gaps are unchanged by tracking these
files.
