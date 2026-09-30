# Monster type icons — 2026-09-30

The requested offline PK2 inspection found four relevant textures in the
operator-supplied GreatestSRO `Media.pk2`. The existing read-only `PK2Archive`
reader and `ddj_to_png` converter extracted metadata-free 16 × 16 PNGs, preserving
source alpha: rank icons are transparent and the party badge is opaque. Numeric
prefixes use PhMon's existing monster type mapping in
`web/app/utils/mapMarkerPresentation.ts`, rather than archive entry IDs.

| Type ID | Output filename | Source texture | Role |
| --- | --- | --- | --- |
| 0 | `0_general.png` | `interface/targetwindow/tw_icon_normal.ddj` | General rank |
| 1 | `1_champion.png` | `interface/targetwindow/tw_icon_champion.ddj` | Champion rank |
| 4 | `4_giant.png` | `interface/targetwindow/tw_icon_giant.ddj` | Giant rank |
| 16 | `16_party_general.png` | `icon/etc/europe_partymob.ddj` | Shared party badge |
| 17 | `17_party_champion.png` | `icon/etc/europe_partymob.ddj` | Shared party badge |
| 20 | `20_party_giant.png` | `icon/etc/europe_partymob.ddj` | Shared party badge |

The three party files are byte-identical aliases, supplied for convenient type-ID
lookup. The badge is separate from rank artwork; use it alongside the appropriate
general, champion or giant icon. No distinct party rank textures were found by
inventory searches for party, monster, champion, giant, elite, titan and target
names. Target-window resource text and textures were inspected locally. No client
runtime rendering or composition was verified; this is asset discovery, not proof
of the client's rendering rules. Elite, titan, unique, level-relative gems and
party-member map symbols were reviewed but are not relabeled as these mob types.

The generic exporter includes these icons automatically from version 0.4.4.
Run the regular export command using the exporter environment:

```sh
PYTHONPATH=tools/game-data-exporter/src python -m phmon_game_exporter.cli export \
  --source /home/node/GreatestSRO --output exports/game-data \
  --asset-output exports/game-assets
```

The regular bundle contains `catalogs/monsterTypes.json` with dataset-scoped type
identities, semantic asset keys, rank relationships and explicit missing-texture
states. Images retain the bundle's content-addressed filenames and deduplication.
The public asset destination contains the six ID-prefixed aliases under
`monster-types/`, with checksums and semantic keys in `asset-index.json`. Archive
provenance stays in the separate exporter audit directory. The initial discovery
artifacts remain local under `exports/monster-icons/`; the standalone extraction
script was removed after integration. Source archives remain read-only. No
monitoring UI, production deployment or game character operation changed.

Validation: decoded all four textures; visually inspected the extracted rank
textures and party badge; checked all six PNGs for 16 × 16 dimensions, alpha
transparency, manifest hashes and ID prefixes; confirmed identical party aliases;
repeated the export and compared file hashes for deterministic output.

Generic exporter regression checks cover automatic catalog and public alias
generation, byte-identical party aliases, repeat exports, copied bundle/public-tree
validation without source archives, missing party textures and unsafe alias
rejection while preserving earlier valid output. All 52 exporter tests pass.

A full real-archive run through the regular CLI produced dataset
`gamedata-0cfdb5ba711c7363570c` under
`exports/monster-export-integration/datasets/` and a public tree under
`exports/monster-export-integration/game-assets/`. The existing export validation
passed for 20 catalogs, 10,039 unique bundle images and 12,068 public aliases.
`monsterTypeIconCount` is 6, the monster catalog is `parsed`, and no monster
textures are missing. All six aliases were compared byte-for-byte with the initial
extraction and their public index checksums and 16 × 16 dimensions verified.
The full dataset remains `incomplete` for nine other unresolved asset families;
this change does not claim those gaps are closed.
