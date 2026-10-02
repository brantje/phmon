# Source format and conversion notes

This file is exporter documentation. Its contents and all provenance stay outside
the app-facing `bundle/` directory.

## PK2 container

The exporter implements the observed JoyMax PK2 header, 20-entry directory blocks,
64-bit block/data offsets, 81-byte EUC-KR names, and the archive's salted Blowfish
directory/checksum convention. The parser was authored for this tool; it does not
vendor another implementation. Format cross-checks:

- [Veykril `pk2` architecture notes](https://github.com/Veykril/pk2/blob/main/ARCHITECTURE.md)
- [Veykril PK2 entry layout](https://raw.githubusercontent.com/Veykril/pk2/main/src/format/entry.rs)
- [Veykril PK2 header layout](https://raw.githubusercontent.com/Veykril/pk2/main/src/format/header.rs)
- [Veykril PK2 Blowfish handling](https://raw.githubusercontent.com/Veykril/pk2/main/src/blowfish.rs)

The archive key is only a command/runtime input. It is neither written to audit nor
the bundle. Source handles are opened read-only. Directory traversal, absolute and
drive paths, cycles, truncated/out-of-range offsets, excessive depths and excessive
payload reads are rejected.

## DDJ images

Representative GreatestSRO payloads begin with the 20-byte `JMXVDDJ 1000` wrapper;
the four-byte field at byte 12 matches the full file size minus 12 in either
little-endian or big-endian order for some files. Other actual files have a different
field value; the exporter records those as `unmatched-advisory-field` in audit and
does not use the field to bound reads. Byte 16 is `3`; a fully embedded DDS or PNG
starts at byte 20, and the PK2 entry's actual length bounds the read. Pillow must
load the complete embedded image, and pixel/edge bounds apply before deterministic
metadata-free PNG output. Output pixels retain source dimensions and alpha where
present. Bytes are never relabelled as PNG without a decode.

The public [Silkroad development file-format index](https://github.com/JellyBitz/srodevs-docs/blob/master/file-formats/README.md)
also identifies the `JMXVDDJ` wrapper. Actual header/size checks were performed
against files read from this GreatestSRO `Media.pk2`.

## Minimap and region data

The selected GreatestSRO `Media.pk2` contains 5,303 `minimap/<x>x<y>.ddj` images,
including named groups such as `arabia`. They are already 256 × 256 2D raster tiles;
they are decoded directly. Tile coordinates are preserved as filename grid indices.
For the root minimap set, region rows link to a tile only when `refregion.gridX`
and `gridZ` exactly equal the tile's filename X and Y. The nested `arabia` set is
not linked by this rule. An eight-pixel edge-band comparison tests both horizontal
and vertical neighbor directions per tile set; an axis direction is reported only
with at least eight pairs, lower error on at least 80% of all comparable pairs
(ties count as non-wins), and at least a 25% lower mean edge error. Both the
GreatestSRO root set and the separate `arabia` set support increasing X to the
right and increasing Y upward. The preview composes each passing set north-up;
`arabia` still has no region/geography join. This grid orientation does not
establish an in-tile world-coordinate transform, marker anchor, or special-area
registration. The exporter does not render terrain meshes.

`Map.pk2` was operator-selected; the same-sized `Map - copia.pk2` is recorded only as
the excluded backup. Map terrain/`tile2d.ifo` is inventoried for audit but is not
needed to render the direct minimap tiles. No phBot minimap folder or screenshot crop
is read.

`refregion.txt` has 21 tab-separated fields in this archive. The named interpretation
used for reference ID, grid X/Z, continent, area, battlefield/climate codes and ten
links is cross-checked against the [Ikarus map editor's SRO format notes](https://github.com/y-xLeo/ikarus-map-editor/blob/main/SRO_MAP_FORMAT_NOTES.md#refregiontxt)
and the source row shape. Negative special IDs are retained. `areaName` text is
preserved as present; the source does not identify a locale reliably. Exact grid
links are reported per region; missing root tiles remain unlinked. No live
coordinate-to-pixel transform is claimed.

## Item identity, localized strings and icon joins

The client item shards are UTF-16 tab-separated records listed by `itemdata.txt`.
This export uses only fields whose joins can be reproduced against the same archive:
numeric record ID, code string, localized name key, description key and associated
icon path. The text lookup key matches field 1 of `textdata_equip&skill.txt`; its
English display text at zero-based field 8 resolves some item names. The icon
reference is converted to an `icon/` archive lookup. Unmatched names/descriptions
and missing icons stay explicit in the bundle/audit rather than being guessed.

The source does not provide verified item category/subcategory/degree semantics in
the material available for this run, nor a matching description-text table. Therefore
the bundle does not guess taxonomy, item requirements, static combat properties,
descriptions or per-record magic-option meaning. Missing names remain explicit;
the export is marked incomplete. A later authoritative schema can expand this
catalog without changing the item IDs already exported.

## Skill, mastery, group and teleport fields

`skilldata.txt` explicitly lists the skill shards used by the exporter. Only its
seven listed shards are parsed; alternate, encrypted, decoded and unlisted table
files are recorded in audit as omitted. The 29,664 selected rows each have a unique
numeric reference ID and unique code across the selected set. The normalized skill
catalog exports those two identities, exact localized name/description joins when
unambiguous, and exact associated icon references when present. It does not expose
rank levels, SP costs, tree ordering, requirements or prerequisites: the table has
no trusted field schema for those semantics. Unreferenced art files remain labeled
as candidates.

The mastery table has a tabular header naming `Mastery Icon` and `Mastery Focus
Icon`. Numeric records export reference ID, exact localized name/description joins,
and those two header-labeled asset roles. Other numeric fields and mastery cap or
requirement rules are omitted. `skillgroup.txt` exports the observed mastery
reference join, per-mastery order, exact localized group label and associated icon
reference; rows whose mastery target is absent remain unresolved. The skill-group
icon family is separate from skill art.

Teleport rows export reference ID, code, an exact localized display name when
unambiguous, and a region reference only when it joins to an exported region ID.
Teleport-link rows export only their two endpoint IDs after both endpoints join to
exported teleports. Coordinates, fees, eligibility, buildings and region/world
transforms are omitted. This is not an outdoor map-transform validation.

Entity names and associated icons are exported where joins resolve, but associated
art is not mislabeled as full-body art. The 52 character-selection images are
portrait *candidates* with no entity mapping. Pet item/skill icons remain with their
own records; no Attack/Fellow/Pick/Transport or pet-body join is established.
Unique classification and higher-resolution unique art also remain unresolved.

## Interface candidates and excluded families

The interface symbol catalog contains a curated, non-control set: chat channels,
event/stat/status categories, map/city markers, guild/party symbols, underbar
category glyphs and portrait candidates. Each is labeled a candidate; the source
path does not establish PhMon-specific behavior. Button states, action controls,
control atlases, frames and Skill Builder cell art are excluded by operator request.
Loading artwork is a separate background catalog. Sounds are excluded by operator
request; music and particle archives are not substituted.

## Dependencies

- Python 3.12 (standard library for PK2, tables, JSON, CLI and preview server).
- [Pillow 12.3.0](https://pypi.org/project/Pillow/12.3.0/) — MIT-CMU license; DDS
  decoding and PNG output.
- [PyCryptodome 3.23.0](https://pypi.org/project/pycryptodome/3.23.0/) — BSD license;
  Blowfish for PK2 directory blocks.
- [setuptools 80.9.0](https://pypi.org/project/setuptools/80.9.0/) — MIT license;
  pinned build backend only.
- pytest 8.4.2, MIT license, for authored fixture tests.

No game archive parser/decoder binary, PhMon process, PostgreSQL, Docker, phBot,
external tile service or network download is needed when running the exporter.


## Slice 4 static item presentation (2026-09-27)

Column facts independently implemented from RSBot's reference loader at commit
`1723fed61b7c75cdb7db58cf04cd5a19dc560fc5`:
https://github.com/SDClowen/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Client/ReferenceObjects/RefObjItem.cs
and sibling RefObjCommon/ObjectCountry/ObjectGender/ObjectRarity sources.
Common columns 9–12 type IDs, 14 country, 15 rarity, 32–39 requirement pairs;
item columns 58 gender, 59 strength, 60 intelligence, 61 item class. Equipment
class determines degree and, for rarity 2 and legacy class 1–30, Star/Moon/Sun.
No name-pattern classification or unverified rolled-stat formula is used.
The local source definitions for the four operator screenshots agree with their
visible static fields. Base stats are deliberately not displayed as instance stats.

The exporter now carries item-table reference ranges separately as
decimal strings under `presentation.reference_stats` for armor, shields, weapons and
accessories. The column mapping follows the pinned `RefObjItem.Load` field indexes in
the RSBot source; those values are reference inputs only and are not used to calculate
rolled display stats until the vSRO 1.188 formula and rounding order are independently
verified. The backend validates their shape, bounds and min/max ordering.

When an unambiguous `server_dep/silkroad/**/magicoption.txt` table exists, the
exporter writes a separate `magicOptions` catalog from RefMagicOpt row ID, group,
level and packed raw ranges. It adds an English label only when the group exactly
joins to one unique key in `textdata_object.txt`. It does not derive names from group
codes or infer units/scaling. The compact server catalog retains the raw definitions;
the backend omits a blue from display until both a validated label and value scale are
available. The GreatestSRO bundle in this worktree has not been rebuilt from its source
archive with that catalog, so no verified option definitions are packaged here.

`build_item_presentation.py BUNDLE ASSET_INDEX OUTPUT [MEDIA_PK2]` produces a
compact backend catalog. The optional local archive upgrades old bundles only
when every ID/code matches. New exporter item records carry `presentation`.
Generated output has no source archive paths; it references existing local PNGs.

## Offline monster artwork — exporter 0.6.1

The normal export can produce transparent model pictures using a CPU rasterizer
implemented with NumPy and Pillow. It requires no browser, GPU or external
renderer. Geometry is read from the operator's local `Data.pk2`; archive access
remains bounded and read-only. No binary client resources are copied to the bundle.

The exact `CharacterData` numeric ID/code and zero-based `AssocFileObj128` field
52 were checked against the local Tiger Girl row: model ID 1954,
`MOB_CH_TIGERWOMAN`, `mob\china\tigerwoman.bsr`. The event variant ID 50918
references that same resource. The public basename is `tigerwoman`, and both direct
references map to `monsters/tigerwoman.png`. Variant rows with `xxx` resources
follow their explicit OrgObjCodeName128 (column 4) reference. Comma-separated
resource lists expand into individual transformations. Resource path joins are
validated against the archive; basename collisions use distinguishing folder
names, preserving model names instead of numeric IDs.

Independent format parsers were checked against the actual headers and offsets
and the format author's research:

- [Compound CPD, `JMXVCPD 0101`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/JMXVCPD)
- [Resource BSR, `JMXVRES 0109`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/JMXVRES)
- [Mesh BMS, `JMXVBMS 0110`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/JMXVBMS)
- [Material BMT, `JMXVBMT 0102`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/JMXVBMT)
- [Skeleton BSK, `JMXVBSK 0101`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/JMXVBSK)
- [Animation BAN, `JMXVBAN 0102`](https://github.com/DummkopfOfHachtenduden/SilkroadDoc/wiki/JMXVBAN)

These are community research descriptions, not Joymax official specifications.
Unsupported versions and flags fail closed for that model. Named model artwork
uses the explicit base palette ID 0; alternate entity textures are not separate
model PNGs. Tiger Girl has seven meshes, 1,937 triangles and
69 bones. The renderer calculates bind and posed world matrices, normalizes the
observed two-bone weights, uses the recorded idle keyframe nearest 20% of the
animation's duration, and converts the left-handed frame to its camera frame.
Diffuse UVs retain their DirectX top-left origin. A depth buffer resolves visible
triangles independently of draw order; explicitly named `_2side` materials use
cutout alpha. Other diffuse alpha is treated as sheen. Lighting is an approximation,
and no cloth/particle simulation or exact client-shader matching is claimed.

Output is deterministic within the pinned renderer environment: NumPy 2.5.3,
Pillow 12.3.0 and renderer `phmon-software-v1`. A 2× supersampled render is reduced
to a 512 × 512 RGBA PNG. No renderer library runs in the web app. Private audit
records include every read dependency's SHA-256, pose and triangle/bone counts;
the public index contains PNG metadata and semantic keys only. Missing or
unsupported artwork remains missing, without a decorative generated substitute.

Portrait camera fitting excludes meshes separated from the largest mesh's bounds
by more than four times that mesh's span. The actual local `mad_general_stand02`
frame moves `mad_general_weapon_02.bms` to Y −1,342…−1,313 while the body occupies
Y 0…65. Including that spare weapon in the camera bounds shrank the body to a
few pixels. The fit now follows the main geometry, without changing the recorded
pose or replacing source geometry. All meshes still participate in rasterization;
`cameraFitExcludedMeshes` retains excluded dependency paths in the private audit.
Regression fixtures verify that distant props do not shrink the portrait, mesh
ordering does not change the fit, and nearby detached parts remain visible.

Malformed exact resource joins are retained separately as `invalid` private audit
entries, with their original reference IDs, codes, resource strings and reasons.
They have no valid model alias and are excluded from the catalogue's `models` and
`unsupported` counts. A separate `invalid` coverage count and
`monsterRenderInvalidCount` result preserve the diagnostics, including for a
selected export. Any invalid joins keep the family partial/unresolved. The
exporter patch version participates in dataset identity so corrected bundles
cannot reuse the earlier immutable outputs.

The `--unique-monsters` selection uses enabled MOB_ rows and rarity **3/8**,
verified against the primary RSBot
[ObjectRarity source](https://github.com/SDClowen/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Client/ReferenceObjects/ObjectRarity.cs)
and [MonsterRarity source](https://github.com/SDClowen/RSBot/blob/1723fed61b7c75cdb7db58cf04cd5a19dc560fc5/Library/RSBot.Core/Objects/MonsterRarity.cs).
Rarity 6/7 are elites, not uniques. The local source has 801 enabled unique rows,
resolving to 106 resource definitions including transformation models and custom
pet-shaped uniques.

Custom mesh files sometimes renamed skin bones without updating subsequent BMS
section offsets. The parser reads faces at the actual end of the documented skin
block, bounds the discrepancy to 4 KiB, validates every triangle, and records the
header discrepancy privately. CPD equipment paths similarly permit a maximum
16-byte discrepancy between the declared length and their explicit `.bsr` end.
No source data is repaired or overwritten. CPD equipment shares the root skeleton;
independent accessories use their BSR attachment bone. Invalid or unsupported bind/animation
data falls back to the resource's unchanged rest geometry with a pose warning.
Ignored skeleton origin/local transforms are skipped rather than rejecting their
unused NaNs. Missing named material joins remain unsupported.
