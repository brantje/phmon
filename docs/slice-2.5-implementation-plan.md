# Slice 2.5 — standalone game-data exporter

Corrected plan and implementation record for Slice 2.5, 2026-09-27.
Branch: `codex/slice-2.5-assets-plan`.
This replaces the previous importer plan and prompt entirely. Implementation is
limited to the standalone exporter; later slices are not started.

## Confirmed scope

The operator confirmed a standalone exporter producing a folder of browser-ready
assets plus JSON catalogs to copy to the application. All client inputs, including
maps, come from the GreatestSRO archives. Abandoned importer/backend files are removed.

Scope decisions confirmed during implementation: `Map.pk2` is the selected map
archive and `Map - copia.pk2` is its excluded backup; direct raster minimap tiles
are decoded from GreatestSRO `Media.pk2`. No terrain-mesh renderer is needed for
the minimap preview. Sounds and interface-control artwork are excluded. Non-control
UI symbols remain included as clearly labeled candidates. Unverified source fields
are omitted or marked unresolved rather than inferred.

```text
GreatestSRO archives (read-only)
    -> standalone offline exporter
    -> finished folder: JSON catalogs + browser-ready assets
    -> PhMon consumes finished files in later slices
```

Only the exporter knows PK2 formats, client paths, archive keys, source table layouts
and texture decoding. PhMon knows only the exported JSON schema, dataset identity,
semantic asset keys and output-relative paths. The exporter runs without PhMon,
PostgreSQL, Docker or phBot. It does not call application APIs, modify databases,
configure server bindings or modify the application.

Do not add backend importers, migrations, upload flows, profile-management screens,
PhMon asset browsers, PK2 packages under `server/`, or import commands in `phmonctl`.
No production Go/Nuxt, plugin transport or Compose changes are required. Build a
standalone output preview for verification, not an application feature.

## Inputs and evidence

Read-only input: `C:\Users\sander\Documents\Silkroad Online\GreatestSRO`.
Previously observed files (recheck during implementation):

| Archive | Bytes |
| --- | ---: |
| Media.pk2 | 3,009,126,400 |
| Data.pk2 | 4,358,639,616 |
| Map.pk2 | 1,092,251,648 |
| Map - copia.pk2 | 1,092,251,648 |
| Music.pk2 | 146,100,224 |
| Particles.pk2 | 296,435,712 |

Equal map lengths do not prove identical contents. Hash and select explicit source
roles, never directory-order precedence. Stream reads and support offsets above
4 GiB. Do not execute the supplied EXEs or load client DLLs.

All 32 supplied screenshots and five baseline captures were visually inspected in
the planning conversation. See [the coverage ledger](slice-2.5-asset-coverage.md).
Supplied images are 2560 × 1315; baseline desktop is 1440 × 1000 and mobile 390 × 844.
Review every image when implementing. They demonstrate asset needs, not source
filenames, format semantics or proof of availability in GreatestSRO.

Verify real archive support independently; abandoned code is not accepted evidence.
Record primary format references and dependency versions/licenses before reuse.
Unknown table columns must not be guessed or exposed as opaque rows in place of
usable catalog metadata. No dependence on phMonitor assets, services or bundles.

## Output contract

Configurable, ignored output, for example `exports/greatestsro/<dataset-id>/`:

```text
<dataset-id>/
  bundle/                       # copy only this folder to the application
    manifest.json
    catalogs/
      items.json
      entities.json
      skills.json
      masteries.json
      skillGroups.json
      regions.json
      teleports.json
      maps.json
      backgrounds.json
      portraits.json
      pets.json
      interfaceSymbols.json
      interfaceControls.json
      localization.json
      taxonomy.json
      ui.json
      sounds.json
    assets/
      images/<content-hash>.png
      maps/<content-hash>.png
  audit/                        # exporter/operator only; not app input
    sources.json
    coverage.json
    unresolved.json
    preview/

# Optional browser alias tree (default target; ignored by Git)
web/public/game-assets/
  asset-index.json              # semantic asset key -> stable public URL
  icon/skill/china/bow_area_a.png
  icon/item/china/...
  interface/character/char_ch_man1.png # portrait candidate; entity join unresolved
  icon/cos/...                     # source-associated entity icons, not role mappings
  minimap/...
```

The exporter emits this layout. Operator-excluded or unresolved families have explicit
empty/status catalogs; `sounds.json` and `interfaceControls.json` contain no extracted
assets. Converted imagery is lossless PNG. Do not rename undecoded bytes to fake
conversion.

Manifest fields: schema version, dataset ID, exporter version, supported locales/
capabilities, completion status, catalog paths/counts/checksums and asset entries.
Asset entries contain stable semantic key, safe relative path, content checksum,
media type, size and dimensions/duration. Multiple keys may share the same bytes.

No source archive names, Windows paths, PK2 entry paths, keys, raw tables, source
column ordinals or decoding instructions belong in `bundle/`. Keep archive hashes,
roles, source-entry/row references, decoder details and provenance in `audit/`, joined
to output IDs. Never record secret archive keys. PhMon never needs those audit files.

When `--asset-output` is supplied, verified assets receive stable browser aliases
with `.png` extensions (for example `game-assets/icon/skill/china/bow_area_a.png`).
The public tree contains only image files and an index mapping normalized semantic
asset keys to relative public URLs. These public aliases do not include archive names,
raw entry metadata, table paths or conversion instructions; the audit retains exact
source-entry provenance. Keep unresolved character/monster/unique roles unresolved
even when an associated icon has a stable URL. `web/public/game-assets/` is the npm
command's ignored default target. The root and URL prefix can be changed by the
operator's `--asset-output` argument.

Derive dataset identity deterministically from selected source content and export
schema/settings. Keep timestamps in audit files outside deterministic bundle identity.
Unchanged inputs/configuration/tool versions produce identical bundle bytes; changed
inputs produce a separate version. Scope records by dataset and kind; preserve
distinct model/ref/code namespaces rather than assuming they are interchangeable.

Use normalized named fields, documented units and explicit unknowns. Export static
identity, names/descriptions, taxonomy, requirements, reference properties and asset
keys. Never fabricate live plus, quantity, blues, current durability, HP, positions,
learned skills, ownership or history. Later consumers keep observed facts separate.

## Sequential implementation plan

### 1. Standalone tool and source inspection

Create `tools/game-data-exporter/` with its own CLI, pinned dependencies, README and
tests. Prefer Python for the offline tool unless verified constraints justify another
language. All source parsing/conversion stays within the tool. Provide inspect,
export and validate commands; document actual syntax.

Inventory source hashes, archive families, encodings/table lists, image/audio formats
and map data. Verify structure/key requirements from evidence. Save local audit
output and concise format documentation. Ask if unknown keys or conflicting sources
require operator knowledge; never substitute another dataset silently.

Gate: safely list real entries and read representative data/image payloads; prove
large-offset support with authored fixtures; reject cycles, truncation and invalid
offsets. The tool works with all PhMon services stopped.

### 2. Schema, deterministic exports and safe publication

Define the JSON contract and authored fixtures before bulk conversion. Resolve
verified localization joins and constituent table lists; handle supported encodings,
duplicate/disabled records and numeric ranges deliberately. Validate field types,
IDs, asset references, units, paths and checksums.

Write to bounded staging output, validate, then publish a completed immutable version
using same-filesystem atomic rename. Preserve prior valid output on failure or
cancellation. Prevent competing writes to the same destination. Diagnostic partial
output must be explicitly incomplete, never a ready bundle. Do not modify client files.

Gate: identical repeated export, changed-input versioning, deterministic ordering,
reference integrity, failure/recovery, dataset-scoped ID isolation and no source
format/path leakage into app-facing JSON.

### 3. Complete static catalogs and converted assets

Export all supported records in each required family, not a few screenshot examples:

- Items: both races' weapons, armor/garments/shields, accessories, consumables,
  ammunition, scrolls, alchemy materials/elixirs/stones, avatars/cosmetics, pet items
  and other supported classes. Include verified taxonomy/degree, names/descriptions,
  requirements, static properties and magic-option labels/units.
- Entities: character portraits, Attack/Fellow/Pick/Transport imagery, monsters,
  NPCs and unique art at icon and larger card sizes. Summon-item icons, portraits
  and full-body art are distinct roles; do not silently interchange them.
- Skills/masteries: both races, race/category symbols, full supported icons,
  names/descriptions, level/tree ordering, costs, requirements and prerequisites.
  Screenshot cap buttons 110/120/140 do not prove those versions are supported.
- Interface: non-control chat, event/stat/category, guild/party, status and map
  symbols plus candidate portraits. Interface controls/atlases, state variants and
  Skill Builder frames/tabs/locked-cell art are excluded by operator request.
- Suitable client backdrop/artwork, quest/reference labels/icons, academy/guild/
  party symbols and emoji assets where present. PhMon identity, advertisements,
  Premium and streaming graphics are not extraction targets.
- Sound assets are excluded by operator request. The output declares the family
  not-in-scope; music is not repurposed as an alert.

Bound decoded dimensions/bytes and conversion time. Preserve alpha, aspect ratio,
native resolution, color channels and orientation. Verify actual codec support;
report unsupported formats. Atlas crops require verified rectangles.

Gate: correct semantic joins across representative families, converted files for
all supported references, and precise unresolved entries. Unknown remains unknown.

### 4. Maps from GreatestSRO archives

Export the direct raster minimap tile family and region/teleport metadata available
inside GreatestSRO. Do not use phBot's minimap folder, remote tiles or screenshot
crops. Do not add a terrain-mesh renderer for this minimap export.

Determine whether sources contain decodable 2D tiles, atlases or data requiring
rendering. Raw terrain/3D files are not browser-ready map output. If required imagery
needs an unplanned rendering pipeline, document findings and ask before expanding
scope or changing sources. Continue independent families while awaiting answers.

Gate: preview tile sheets/seams and record dimensions/orientation, region/floor
relationships and missing areas. Mark transforms unvalidated unless independently
proven. Slice 7 owns live coordinate validation; do not guess outdoor transforms
for special areas or treat file presence as proof of correct positioning.

### 5. Standalone preview and coverage audit

Generate a local output preview with catalog search, family thumbnails, item/entity/
skill/mastery/group samples, non-control symbol candidates, portrait candidates,
map sheets and backdrop samples. No audio playback or interface-control crops.
It reads finished bundle files only, optionally through a simple static HTTP server.
It requires neither PhMon nor PostgreSQL.

Review transparency/quality, 45/60/75-pixel icons/portraits, large unique artwork and
layouts at 1440 × 1000, 1280 × 800 and 390 × 844. Compare references without claiming
the preview implements the screens. No external asset requests.

Audit every screenshot row and roadmap family: discovered, parsed, converted,
mapped, validated and visually/playback-reviewed counts, exceptions and evidence.
Do not skip needs absent from populated screenshots. Ask before replacing missing
required art with another source or generated content. Decorative placeholders
cannot replace missing semantic catalog/map data.

### 6. Real export and independent consumption

Run against GreatestSRO and record real counts, duration, peak resources, output
size, format support and blockers. Copy only `bundle/` into a clean folder and
validate/preview it with archives and audit files inaccessible. This is the key
acceptance test: the output works without client-media knowledge or dependencies.

Document exact PowerShell export/validate/preview/copy commands, paths with spaces,
schema/versioning, reruns, replacement and tests. Include a small standalone consumer
example resolving a model ID through JSON to an asset path. Do not integrate it into
PhMon or create database tables; later slices consume the finished JSON.

Add ignore rules before extraction for archives, generated bundles, reports and
previews. Never commit proprietary bytes or upload them as CI artifacts. CI uses
authored synthetic fixtures. Preserve the complete operator output locally.

## Validation and completion

Test malformed headers/entries, large offsets, cycles, traversal with both slash
styles, absolute/drive/UNC names, case collisions, symlink/reparse escape, invalid
text/columns/numbers, missing references, unsupported codecs/keys, oversized decoded
images/output, disk failures, cancellation and source changes during export.
Malformed input must not publish ready output or access arbitrary host paths.

Test determinism, semantic joins, locale fallback, scoped IDs, checksums, decoding
of all referenced assets, missing-family reporting and copied-folder usage. Run the
exporter suite and real export validation. Do not introduce database/backend test
infrastructure for this tool.

All-assets-ready requires all required families to have usable output and review
evidence. Report unresolved required gaps as blockers, not completion. Update the
coverage ledger and AGENTS.md with actual results/next actions. Stop before Slice 3.
No publishing, merging or operating real characters.

## Execution record — 2026-09-27

**Slice status:** exporter implementation, real export, copied-bundle validation and
standalone preview are complete within the operator-confirmed scope. **Asset readiness
is incomplete**: required taxonomy, localization, role mappings and map transforms
remain unresolved, so the bundle manifest correctly says `incomplete`. Do not treat
this as an all-assets-ready dataset.

The inspected source selection is GreatestSRO `Media.pk2` for client catalogs, art
and direct minimap tiles, plus `Map.pk2` for map-source inventory. `Map - copia.pk2`
is excluded as the operator-designated backup. Exact archive hashes and Windows paths
are in ignored exporter-only `audit/sources.json`; no provenance is in the copied
bundle. The exporter does not read the phBot minimap directory, Data/Music/Particles
archives or client executables.

Real export dataset: `gamedata-66e9e3ee636c5a2a3f8f`, exporter `0.3.1`, schema
`1.1.0`, 17 catalogs, 8,921 unique PNGs, 30,276 semantic asset keys and
465,088,731 bundle bytes. It parsed 14,238 items, 18,699 entities, 29,664 listed
skills, 18 masteries, 140 skill groups, 2,471 regions, 222 teleports with 294
endpoint links, 5,303 minimap tiles in two sets, 93 backgrounds, 52 portrait
candidates and 159 non-control symbol candidates. Sounds and interface-control
artwork are explicitly not-in-scope. Per-family resolution and missing counts are
recorded in `audit/coverage.json` and summarized in the asset coverage ledger.

The first real export safely stopped before publication because a skill identity field
assumption failed the cross-shard uniqueness check. After checking all seven indexed
shards (29,664 rows), the verified ID field was corrected and the export completed.
An identical rerun reused the same dataset and exact bundle bytes (`identicalBundleReused`);
the first and repeat runs took 178.797 s and 189.36 s. Five working-set samples
observed at most 165,826,560 bytes (~158 MiB); this is a sampled maximum, not a
guaranteed process peak. Bundle and audit sizes were 465,088,731 and 24,200,372 bytes.

Only the `bundle/` directory was copied to `exports/standalone-copy-0.3.1/`; that
clean copy contains only `assets/`, `catalogs/` and `manifest.json`. Running
`validate --bundle` on that path, without a source or audit argument, validated all
17 catalogs, all 8,921 PNGs and 30,276 semantic keys, found zero dangling references,
validated normalized relations and reported `sourceKnowledgeRequired: false`. The
copied folder was served by the preview, whose request surface resolves only bundle
paths. The exporter fixture test also removes its synthetic source tree before
validating the copied bundle.

Standalone preview review covered items, entities, skills, skill art, masteries,
skill groups, teleports, portrait candidates, unresolved pet roles, non-control
symbols, backgrounds and both map tile sets. The 159 symbol cards and 600 sampled
skill cards had zero broken loaded images; no external requests were observed. CSS
viewport measurements at 1440 × 1000, 1280 × 800 and 390 × 844 found no horizontal
overflow (the measurements used fixed-size same-origin frames because the browser's
minimum outer window is 500 px). Map tile-sheet previews were 2,724 × 1,104 pixels
for tile set 001 and 204 × 288 for tile set 002. They show tile-index layouts with
gaps; axis orientation, region/floor joins, outdoor transforms and the named Jangan
Cave / Donwhang Cave / Job Temple special-area transforms remain unvalidated. Ignored
visual evidence is under `exports/standalone-preview-*.png`.

Final commands run after creating the documented ignored local virtualenv:

```powershell
$python = "tools/game-data-exporter/.venv/Scripts/python.exe"
& $python -m compileall -q tools/game-data-exporter/src tools/game-data-exporter/tests
& $python -m pytest tools/game-data-exporter/tests -q
& $python -m phmon_game_exporter.cli --version
& $python -m phmon_game_exporter.cli validate --bundle "exports/standalone-copy-0.3.1"
```

Results: compilation passed; 17 tests passed; the CLI reported `phmon-game-data
0.3.1`; copied-bundle validation passed with zero dangling references. The venv,
real export, audit and preview artifacts are ignored and outside Git. No PhMon,
backend, database, Docker or phBot process was used. Remaining asset blockers are
not closed by these passing exporter checks; see the coverage ledger for exact counts.

**Stop point:** Slice 2.5 only. Do not start Slice 3 in this task. Next action is to
use the incomplete bundle only where verified fields suffice and resolve the recorded
catalog/mapping gaps before any feature depends on them.

### Follow-up — Nuxt public asset destination

The operator requested a Nuxt public asset destination and an `npm run export:assets`
entry point. Exporter `0.4.0` adds configurable `--asset-output`, which writes
source-independent PNG aliases and `asset-index.json`; the exact converted archive
entry path stays only in the separate exporter audit. It refuses to replace an
unowned output directory. `web/package.json` now provides `export:assets`, defaulting
to ignored `web/public/game-assets/` and `exports/greatestsro/`, with source and path
overrides via flags or `GREATESTSRO_SOURCE` / `PHMON_GAME_ASSETS_OUTPUT`.

The real `0.4.0` npm export took 245.409 s and produced the same 17 catalogs, 8,921
content-addressed bundle assets, 30,276 semantic keys and incomplete status as the
prior dataset, under new exporter-versioned ID `gamedata-0ddf424b486dd8f6ff84`.
It wrote 10,171 public PNG aliases (482,109,338 image bytes) plus a 5,249,420-byte
index. `icon/skill/china/bow_area_a.png` maps to its verified skill asset key and
served through Nuxt as HTTP 200 `image/png` with the indexed SHA-256. A clean copy
containing only `bundle/` and `game-assets/` validated without source/audit inputs:
17 catalogs, 8,921 bundle assets, 30,276 bundle keys, 10,171 public files and 30,276
public key mappings, with no dangling references and source-knowledge flags false.
The copied bundle preview returned HTTP 200 for its page and manifest. Compileall,
18 exporter fixture tests, ESLint, Prettier and npm-wrapper help checks passed. The
manifest and family coverage remain incomplete; no later slice is started.

Command used from `web/`: `npm run export:assets -- --source
"C:\Users\sander\Documents\Silkroad Online\GreatestSRO"`. The optional target is
overridable with `--asset-output`; no database, backend, Docker or phBot process was
started.

Independent rerun on 2026-09-27 completed in 246.084 s. The exporter reused the
byte-identical dataset and republished the default Nuxt destination. Bundle
validation passed for 17 catalogs, 8,921 images, 30,276 keys, zero dangling
references and normalized relations; public-asset validation passed for all
10,171 aliases and 30,276 key mappings. A fresh `exports/standalone-copy-npm-run/`
contains only the copied `bundle/` (8,939 files; no audit folder or PK2 archive) and
passed the same bundle-only validation. Its standalone preview returned HTTP 200 for
the page, manifest, map catalog, a PNG tile and a generated tile-set sheet. The
fixture suite passed all 18 tests in 0.66 s. The ignored Nuxt tree totals 487,358,758
bytes including its index; it has 3,183 `icon/item` aliases and 57 portrait
candidates under `interface/character`. No verified `icon/mob` folder exists, so
monster-to-art roles remain unresolved. Grid/source comparison findings are recorded
in [minimap verification](minimap-verification.md); that note does not validate an
in-tile transform.

### Follow-up — verified minimap region indices

The 2026-09-27 minimap audit found the direct join between root `Media.pk2`
minimap filenames and `refregion` indices: `(gridX, gridZ)` matches tile filename
`(x, y)` for 2,449 of 2,471 region records. The selected Map.pk2 inventory has 15
`map/<Y>/<X>.o2` files; all 15 coordinate pairs match region grid pairs in that
order. The phBot reference directory's 5,794 root JPG names use `<X>x<Y>`. Jangan
(`25000`, X=168/Z=97) and Donwhang (`Town_Dunhwang`, four tiles X=152–153/Z=102–103)
have matching direct Media minimap PNGs and phBot JPGs. All 11 compared city/live
tiles are 256 × 256 and have mean RGB error 5.843 across Jangan, 5.093 across
Donwhang and 2.996 for the live tile after JPG recompression.

Exporter `0.4.1` / schema `1.2.0` now emits the verified exact root-set index links
in `regions.mapAssetKeys` and a per-record `mapTileIndexStatus`; it does not join
the separate `arabia` set. It leaves pixel/world transforms `unvalidated`. Dataset
`gamedata-00cdb146f754911f1c80` has 2,449 linked records and 22 without an exact
root tile (17 at `(0,0)`, five nonzero grid pairs). In the real bundle, Region 25000
links to `(168,97)` and live Region 25735 to `(135,100)`.

Validation after this change: compileall passed; 18 fixture tests passed; exporter
CLI reported `0.4.1`. Two real `npm run export:assets` runs took 236.349 s and
247.297 s. The second returned `identicalBundleReused: true`. Copy-only bundle and
public-asset validation passed with 17 catalogs, 8,921 bundle PNGs, 30,276 bundle
keys, 10,171 public files, 30,276 public keys and zero dangling references. A
standalone preview from that copy returned HTTP 200 for the page, manifest, map
catalog, generated tile sheet and tile 135x100; the preview server was stopped.
The latest read-only character snapshot returned four online characters in Region
25735, X=98.2–100.0, Y=1555.3–1559.0. No marker was positioned by guesswork: the
known-coordinate screenshot mosaic fit does not uniquely validate pixel origin,
axes or marker anchor. The PhMon app still has only a Map navigation label. Exact
marker placement remains open; do not start Slice 3 under the Slice 2.5 scope.

An additional read-only snapshot at 03:27:56 UTC again placed four online
characters in Region 25735 (X=98.1–101.5, Y=1551.1–1560.2; names omitted). The
reference screenshot is not tied to those API identities or timestamp. Community
format notes suggest a 1,920-unit region, but are not an authoritative transform
contract for this source. The screenshot fit still does not validate marker pixels.
The region-to-root-tile lookup and root tile-grid row direction are confirmed,
while in-tile mapping remains unresolved.

### Follow-up — full-grid tile orientation and copied preview (2026-09-27)

Exporter `0.4.2` / schema `1.2.1` now measures adjacent 8-pixel RGB edge bands for
each minimap tile set. The root set has 5,118 tiles, 4,806 horizontal pairs and
4,721 vertical pairs. Lower-error pair counts and mean RGB edge errors support
increasing X right (4,132 wins, 17.010 vs 36.426) and increasing Y/Z up (4,100
wins, 16.576 vs 36.364). The separate 185-tile `arabia` set supports the same
directions from 163 horizontal and 165 vertical pairs but remains unjoined to
regions. The standalone tile overview now uses the per-set direction only when it
passes the documented pair-count/share/error thresholds; local world coordinates
and marker anchors remain unresolved.

The fixture suite passed 20 tests, including synthetic orientation and north-up
preview checks; compileall passed and the CLI reports `0.4.2`. Real dataset
`gamedata-21789415d0f1593162dd` exported through `npm run export:assets` in 242.804
s. A second run took 254.988 s and reused the byte-identical bundle. Output remained
incomplete: 17 catalogs, 8,921 unique PNGs, 30,276 semantic keys, 5,303 minimap
tiles, and 9 unresolved coverage entries. There are 2,449 exact root-region links
and 22 regions without an exact root tile. The bundle is 465,397,123 bytes and the
separate audit is 24,204,249 bytes.

Combined validation passed for the bundle and Nuxt public assets: 17 catalogs,
8,921 images, 30,276 keys, 10,171 public files, 30,276 public mappings, zero
dangling references and no source knowledge required. A separate `bundle/`-only
copy with no audit/PK2 files validated from its own directory. Its standalone
preview served the page, manifest, maps catalog, tile 135x100 and both tile-set
sheets with HTTP 200; the root and `arabia` sheets were visually checked. All
derived sheets and extracted output remain Git-ignored.

The 03:27:56 UTC read-only character snapshot still maps only to Region 25735 / tile
135x100. The screenshot is not identity/time-linked to those live rows. Tile index
and root grid directions are verified, but exact character pixel placement remains
open because the in-tile world transform and marker anchor are not established. This
is outside the confirmed Slice 2.5 asset/catalog scope. Validate it before a later
map screen places markers; stop after Slice 2.5 and do not start Slice 3.

### Follow-up — explicit map-raster gaps and Nuxt default (2026-09-27)

Exporter `0.4.3` / schema `1.2.2` adds `rasterContentStatus` to map-tile rows and
region links, plus an explicit `uniformOpaqueBlackTileCount` and unresolved map
audit entry. It counts exact opaque-black PNGs from their decoded pixels; it does
not infer a replacement source. The preview hatches these tiles and reports the
family partial. The aggregate 207 includes 193 root-set tiles and 14 `arabia`
tiles. The earlier phBot comparison's 30 outliers are the shared filenames where
the black GreatestSRO source conflicts with visible terrain in that comparison;
phBot imagery remains comparison-only. The map audit still records 22 regions
without an exact root-tile join, for 229 unresolved map records total. Map
orientation evidence does not close the in-tile coordinate transform.

The real command with an explicit output parameter,
`npm run export:assets -- --source "C:\Users\sander\Documents\Silkroad Online\GreatestSRO" --asset-output public/game-assets`,
completed in 258.879 s. It wrote dataset `gamedata-47c969ded0613d4c2a22`, marked
`incomplete`: 17 catalogs, 8,921 unique bundle PNGs, 30,276 semantic keys, 5,303
map tiles, 14,238 items, 18,699 entities, 29,664 skills, 18 masteries, 140 skill
groups, 222 teleports, 294 teleport links, 93 backgrounds, 52 portrait candidates
and 159 non-control symbols. Bundle size is 465,803,751 bytes; exporter-only audit
size is 24,243,119 bytes. The Nuxt tree has 10,171 PNG aliases and 30,276 semantic
keys. The public index includes `icon/skill/china/bow_area_a.png` and 3,183
`icon/item` aliases; it has no `icon/mob/` path. Entity icon associations and
character portrait candidates remain candidates, not verified monster/unique or
portrait roles. Sounds and interface controls are excluded by operator request.

Combined source-independent validation passed for the bundle and Nuxt public tree:
17 catalogs, 8,921 assets, 30,276 keys, zero dangling references, normalized
relations valid, 10,171 public files and 30,276 public mappings; both source-
knowledge checks are false. A bundle-only copy with 8,939 files (manifest, 17
catalogs and assets; no audit or archives) also validated with zero dangling
references. Its standalone preview returned HTTP 200; the browser displayed the
copied dataset, item thumbnails and generated map sheet. The rendered root sheet
is 2,724 × 1,104 and visibly hatches black-source gaps. The browser emitted no
warnings or errors. A second npm run without `--asset-output` completed in 268.352
s, set `identicalBundleReused: true`, and published to default
`web/public/game-assets`. This verifies both the explicit target parameter and the
Nuxt default.

Compileall passed, all 21 fixture tests passed, the CLI reported `0.4.3`, and
`git diff --check` passed (only Git LF-to-CRLF advisories). The copied bundle,
audit and Nuxt assets are ignored under `exports/` and `web/public/game-assets/`.
No PhMon service, PostgreSQL, Docker or phBot process was used.

## Prompt for GPT-6 Luna

> Implement Slice 2.5 only on codex/slice-2.5-assets-plan. Read AGENTS.md,
> docs/slice-2.5-implementation-plan.md and docs/slice-2.5-asset-coverage.md.
> Build a standalone offline exporter reading
> C:\Users\sander\Documents\Silkroad Online\GreatestSRO, including its archives
> for maps. Produce browser-ready assets and normalized JSON catalogs in a folder
> I can copy to PhMon. PhMon must have no PK2/client-media knowledge. Do not add
> backend importers, migrations, APIs/UI, database dependencies or phmonctl import
> commands. Review all screenshots and cover complete required later-slice asset
> families. Keep source provenance in exporter-only audit files. Verify deterministic
> real exports, standalone previews, coverage and copied-bundle use without source
> files. Ask whenever required source availability, semantics or scope is uncertain.
> Preserve unrelated work, keep extracted assets ignored, record actual results and
> blockers, and stop before Slice 3.
