# PhMon game-data exporter

This is an independent offline Python tool. It reads GreatestSRO archives and emits
a browser-ready `bundle/` plus exporter/operator provenance under `audit/`. Copy only
the `bundle/` directory to PhMon. PhMon needs no PK2, Windows client path, client
table schema or conversion logic.

It does not call PhMon or any application API and does not require PostgreSQL,
Docker, Nuxt, Go, phBot, Three.js, Blender, a browser or a GPU. Monster artwork is
rendered offline with NumPy and Pillow. The source archive folder is opened read-only. Sound and
interface-control artwork are excluded; a curated set of non-control symbols is
included as unmapped candidates. `Map.pk2` is the selected map archive;
`Map - copia.pk2` is excluded as its backup. Direct minimap images come from the
GreatestSRO `Media.pk2`. Root minimap tile indices link to region `gridX/gridZ` on
exact pairs; that index join does not claim an in-tile world-coordinate transform.
Adjacent root-tile edge continuity supports X increasing right and Y increasing
up; the standalone overview uses supported per-set directions. No terrain renderer
or character-coordinate transform is included.

## Install

```powershell
Set-Location C:\Users\sander\Documents\phmon
py -3.12 -m venv tools/game-data-exporter/.venv
tools/game-data-exporter/.venv/Scripts/python.exe -m pip install -e tools/game-data-exporter
tools/game-data-exporter/.venv/Scripts/python.exe -m pip install -r tools/game-data-exporter/requirements.txt
```

`SRO_PK2_KEY` can provide the local archive key. It is hidden from CLI help and is
never copied into bundle/audit files. Do not pass it in shell history if command-line
exposure is a concern.

## Commands

```powershell
$python = "tools/game-data-exporter/.venv/Scripts/python.exe"
$source = "C:\Users\sander\Documents\Silkroad Online\GreatestSRO"
& $python -m phmon_game_exporter.cli inspect --source $source --audit exports/gamedata-inspection/audit
& $python -m phmon_game_exporter.cli export --source $source --output exports/greatestsro
```

The export command prints the immutable dataset ID and exact bundle/audit paths.
The bundle contains normalized JSON catalogs, deterministic asset names, checksums
and explicit incomplete/not-in-scope family states. The audit contains source hashes,
archive paths, source rows/entries and conversion details. Keep audit local to the
exporter/operator.

To also publish stable web paths for Nuxt's static `public/` directory, set the
destination on export:

```powershell
& $python -m phmon_game_exporter.cli export `
  --source $source `
  --output exports/greatestsro `
  --asset-output web/public/game-assets
```

The public tree contains PNG aliases such as
`game-assets/icon/skill/china/bow_area_a.png` and an `asset-index.json` mapping
normalized semantic asset keys to public URLs. It contains no archive paths or
source-table details; those stay under the separate exporter `audit/` directory.
The default Nuxt destination is tracked with the PhMon repository. Source archives,
audit files, and temporary exporter outputs remain outside the Nuxt public tree and
are ignored under `exports/`. Choose a path outside the checkout for temporary outputs.

From `web/`, the npm convenience command uses that Nuxt destination by default.
Provide the read-only source with `--source` or the `GREATESTSRO_SOURCE` environment
variable; output locations can be overridden with `--output` and `--asset-output`:
Relative `--source`, `--output`, and `--asset-output` values resolve from `web/`.
Relative `GREATESTSRO_SOURCE` and `PHMON_GAME_ASSETS_OUTPUT` values also resolve
from `web/`.

```powershell
$env:GREATESTSRO_SOURCE = "C:\Users\sander\Documents\Silkroad Online\GreatestSRO"
npm run export:assets
# or: npm run export:assets -- --source "C:\path\to\GreatestSRO" --asset-output public/game-assets
```

Validate the exported folder without opening archives:

```powershell
& $python -m phmon_game_exporter.cli validate --bundle "exports/greatestsro/<dataset-id>/bundle"
```

When the public asset tree was generated, validate that copy without access to the
source archives as well:

```powershell
& $python -m phmon_game_exporter.cli validate `
  --bundle "exports/greatestsro/<dataset-id>/bundle" `
  --public-assets web/public/game-assets
```

Start the standalone local preview. It serves only the finished bundle plus a small
embedded preview page and generates minimap overview sheets from copied bundle PNGs:

```powershell
& $python -m phmon_game_exporter.cli preview --bundle "exports/greatestsro/<dataset-id>/bundle"
```

The default server binds to `127.0.0.1:8765`. It has catalog search, item/entity/
skill/mastery/group/teleport/symbol/portrait thumbnails, and a tile-set overview.
It is an exporter review surface, not a PhMon screen implementation. The map
preview shows client tile indices only.

Copy and validate just the app-facing folder:

```powershell
Copy-Item -Recurse -LiteralPath "exports/greatestsro/<dataset-id>/bundle" -Destination "C:\PhMonAssets\game-data"
& $python -m phmon_game_exporter.cli validate --bundle "C:\PhMonAssets\game-data"
```

The second validation can be run after disconnecting/removing the source archive
directory and without retaining `audit/`.

## Output contract

Every normal `export` includes `catalogs/monsterTypes.json` and the available
monster rank/party textures in the validated bundle. With `--asset-output`, the
same command also writes ID-prefixed aliases under `game-assets/monster-types/`
and indexes them in `asset-index.json`. No additional script is needed. The files
are `0_general.png`, `1_champion.png`, `4_giant.png`, `16_party_general.png`,
`17_party_champion.png` and `20_party_giant.png`. Party IDs 16, 17 and 20 share
one badge; the catalog records the corresponding rank IDs separately. Missing
textures remain explicit in catalog status and coverage; no substitute is generated.
See [monster icon findings](../../docs/reference/monster-icon-investigation.md).

The six PNGs are also committed under `web/public/game-assets/monster-types/` and
registered in the public asset index, so they can be used immediately without
running the exporter. Their browser URLs start with `/game-assets/monster-types/`.

### Monster model pictures

Exporter **0.6.1** reads monster model resources from `Data.pk2` when that archive
is present. Normal export renders all exact monster resource joins from the indexed
character tables. Use a repeatable `--monster-model` option to limit a trial:

```powershell
& $python -m phmon_game_exporter.cli export `
  --source $source `
  --output exports/greatestsro `
  --asset-output web/public/game-assets `
  --monster-model tigerwoman
```

Or from `web/`: `npm run export:assets -- --source C:\path\to\GreatestSRO --monster-model tigerwoman`.
The output is **`web/public/game-assets/monsters/tigerwoman.png`**, named for the
model resource, rather than its numeric ID. PNGs are 512 × 512 with transparent
backgrounds. `catalogs/monsters.json` maps each resource name to its numeric model
IDs, codes, semantic asset key and public alias. Multiple IDs referencing the same
resource share the picture. The public asset index includes its checksum and URL.
To export all enabled standard and dungeon uniques, use:

```powershell
& $python -m phmon_game_exporter.cli export `
  --source $source `
  --output exports/greatestsro `
  --asset-output web/public/game-assets `
  --unique-monsters
```

From `web/`: `npm run export:assets -- --source C:\path\to\GreatestSRO --unique-monsters`.
This selects monster rarity 3 and 8, resolves explicit base references, and expands
comma-separated transformation resources. Event variants share one model picture.

When `--monster-model` or `--unique-monsters` is supplied, public publication updates only successful
selected monster pictures and their index entries, preserving existing other
assets and unselected monster pictures. The bundle still exports its normal
catalogue families. Without either option, public publication uses the normal full
asset-tree export. A wholly unsupported selected render leaves public assets
untouched; its failure remains in the returned counts and private audit.

Rendering applies a recorded idle pose, two-bone skinning, textured triangles,
depth occlusion, alpha cutouts, modest lighting and antialiasing in software.
The camera fits the main mesh cluster; props animated more than four times the
largest mesh's span away cannot shrink the portrait. Those meshes still undergo
rasterization, and their paths are listed privately as `cameraFitExcludedMeshes`.
Artwork uses the resource's base material palette (ID 0), rather than each
entity's TextureType appearance. BSR and assembled CPD character models are
supported, including equipment skinning and bone attachments. A model without an
idle animation uses its stored rest geometry. An invalid or unsupported custom skeleton also
uses rest geometry, with an explicit pose warning in the private audit. Unsupported
formats or missing geometry/material dependencies produce an unsupported record
and no placeholder PNG. Missing `Data.pk2` leaves artwork unresolved while the
other catalogue/icon/map export continues.

Names are lowercase safe resource basenames. When different resource paths reuse
a basename, the shortest distinguishing resource-folder names are prefixed; no
numeric ID is used. Rows with `xxx` resources follow their exact OrgObjCodeName128
reference. No localized-name or filename guessing is used.
Unsafe resource paths or basenames are isolated per comma-separated resource and
retained as `invalid` private audit entries with the original reference ID, code,
resource and reason. Coverage and the export result report a separate invalid
count; valid models still render, including when `--monster-model` is selected.
Invalid joins have no public alias and keep monster coverage partial/unresolved.

The selected names and the `Data.pk2` hash participate in the immutable dataset
identity. The bundle includes the resulting PNGs, never the mesh, skeleton,
animation or texture source files. Dependency paths, hashes and detailed failures
remain under `audit/tables/monster-renders.json`. Rendering is approximate; cloth,
particles and the original client's material shaders are outside this renderer.
The native unique batch is documented in [export evidence](../../docs/monster-art-export.md).
Read each run's audit for coverage and rest-pose limitations. See [format notes](format-notes.md).

`manifest.json` fixes the dataset ID, schema/exporter versions, family status,
supported display locale and safe catalog/asset paths. Each catalog carries its own
version, family, dataset ID, explicit status and normalized records. Asset paths are
content-addressed PNGs under `assets/images/` or `assets/maps/`; the manifest maps
semantic asset keys to their checksum, media type, dimensions and relative path.
All record and asset keys are scoped to the dataset ID. No source paths, archive
names, PK2 offsets, raw table rows, field ordinals or decoding instructions are in
the bundle.

Output is intentionally marked incomplete when names/descriptions/icons, taxonomy,
skill rank/requirement semantics, pet/entity/unique roles, teleport-region joins or
map transforms remain unresolved. Skill/mastery/group/teleport catalogs contain
only verified identity and exact-join fields; unverified rules stay out of the JSON.
See `format-notes.md` and the generated `audit/coverage.json` /
`audit/unresolved.json` for the actual run counts.

Unchanged archive content, schema and exporter version reuse the same immutable
dataset folder only if the newly generated bundle bytes match exactly. Changed
inputs create a different dataset ID. Failed conversion does not publish a bundle.

## Tests

```powershell
& $python -m pytest tools/game-data-exporter/tests
```

Tests use authored in-memory/synthetic fixtures and never bundle GreatestSRO assets.
The proprietary archive output remains ignored under `exports/`.
