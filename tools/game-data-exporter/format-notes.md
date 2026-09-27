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
