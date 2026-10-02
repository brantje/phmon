# Offline monster picture export — 2026-10-02

The operator requested a one-unique Three.js experiment, then requested that it
become part of the exporter without Three.js and use model names rather than IDs.
Exporter 0.5.0 implemented that request with a NumPy/Pillow CPU renderer.

Tiger Girl's original local model is now rendered to
[`web/public/game-assets/monsters/tigerwoman.png`](../web/public/game-assets/monsters/tigerwoman.png).
It is a 512 × 512 transparent RGBA PNG, 156,243 bytes, SHA-256
`233bba6c895633837a9b5a05dcfc70b4001e670557620c5502b148b54c84f317`.
The file was visually checked for correct texture placement, full figure framing,
idle pose and hair cutouts. Its texture/silhouette remain those of the local
low-poly client model; lighting is approximate.

Normal export reads exact model-resource references from the existing indexed
character tables. `--monster-model tigerwoman` limits rendering and public
publication to that resource name. The `monsters` catalogue maps direct model IDs
1954 and 50918 to the one named PNG. Full export attempts all exact joins.
That initial run established single-model coverage. Exporter 0.6.0 extends it
with a unique batch selection, exact base-resource joins, transformation lists,
base palettes, CPD character assembly and audited custom-file/rest-pose handling.
It does not close the full uniques UI parity requirement.

Validation:

- `PYTHONPATH=tools/game-data-exporter/src /tmp/phmon-monster-render-venv/bin/pytest -q tools/game-data-exporter/tests`: **66 passed**.
- Real `export --source /home/node/GreatestSRO --monster-model tigerwoman`:
  384.932 s, 21 validated catalogues, 10,040 content-addressed assets, one monster
  picture and zero unsupported selected monster resources. The overall dataset
  remains incomplete due to existing independent reference-data gaps.
- Final renderer output matched the real export PNG byte-for-byte.
- Scoped publication to the requested `web/public/game-assets` target preserved
  all **12,069** existing indexed file records, their bytes and original dataset
  identity; it added only `monsters/tigerwoman.png` and its index metadata.
  Public validation reports 12,070 files and 32,175 semantic keys.
- Tests exercise depth ordering, transparent occlusion, UV orientation,
  deterministic PNG output, parser/path bounds, an original binary triangle
  fixture, complete export/publication, source-free copied-bundle validation,
  unknown model selection, source hash/selection identity, and scoped publication
  retaining unrelated assets and retaining last-known artwork on unsupported input.

Private native evidence is under the ignored
`exports/monster-render-export/gamedata-dd32ecb1f7a45358af54/` bundle/audit,
`exports/monster-render-export-result.json`, and
`exports/monster-render-publication.json`. Source archives remain read-only.
No source binary, mesh, skeleton, animation or exporter audit is added to the public
asset tree. The earlier browser experiment lives only in ignored local scratch
output; no Three.js dependency remains in the exporter or web application.

See the [exporter instructions](../tools/game-data-exporter/README.md) and
[format/source investigation](../tools/game-data-exporter/format-notes.md).

## All-uniques batch — exporter 0.6.0

The follow-up request was to export every unique from the local client into the
same folder. `--unique-monsters` selects enabled monster rows with rarity 3 or 8,
resolves exact base codes and expands transformation resource lists. The local
client has **801** enabled unique rows resolving to **106** resource definitions.

Native command:

```bash
tools/game-data-exporter/.venv/bin/python -m phmon_game_exporter.cli export \
  --source /home/node/GreatestSRO \
  --output /var/www/phmon/exports/uniques-render/export \
  --asset-output /var/www/phmon/web/public/game-assets \
  --unique-monsters
```

Result: **105 named PNGs** in `web/public/game-assets/monsters/`, covering standard,
dungeon, event, transformation and custom unique models. Every published PNG was
checked for a valid 512×512 RGBA image, transparent background and indexed digest.
Duplicate basenames use distinguishing resource-folder prefixes. Source archives
were opened read-only. Two CPD characters (`shinmu`, `volkoff`) render with their
referenced armor, weapon and accessory models.

The remaining resource, **`volkoft`** (MOB_MEGATITAN_03, reference 44500), is
unsupported: its equipment meshes request `heavy_13_*` materials, while the
referenced material set defines `heavy_15_*`. No replacement texture was guessed
and no placeholder PNG was published. Repairing or explicitly mapping those
source material definitions is needed to render that final model.

Five rendered resources use audited rest geometry because their rigs cannot be
applied: `haroeris_t2` exceeds the current 256-bone skeleton limit (257 bones);
`horned_demon` and `zed` have missing mesh-to-skeleton bone joins; `spirit` and
`venefica` contain non-finite bind positions. `valkyrie` has no idle animation and
also uses its real rest geometry. Cloth, particles and exact client shaders remain
outside this approximate still-image renderer.

Validation: **71 exporter tests pass**. Dataset
`gamedata-93183f2f3dec5300d4fc` completed in **506.582 s**; source-free bundle
validation checks 21 catalogues, 10,142 assets and zero dangling asset references.
Public validation checks **12,174** files and **32,279** semantic keys. All
**12,069** unrelated existing public file records and their bytes remain intact,
including their base dataset identity `gamedata-17f8847c77edd7c7fadd`. The overall
reference-data bundle remains incomplete due to existing independent gaps plus
the one unsupported monster definition. The uniques UI itself was not changed.

Ignored native evidence: `exports/uniques-render/export-result.json`,
`exports/uniques-render/validation.json`, and the immutable bundle/audit under
`exports/uniques-render/export/gamedata-93183f2f3dec5300d4fc/`. A contact sheet of
the final published PNGs is `exports/uniques-render/published-preview.jpg`.

## PR #62 review corrections — exporter 0.6.1

Malformed resource paths or basenames no longer stop unrelated valid models from
rendering. Each rejected join retains its reference ID, code, original resource
string and reason as an `invalid` private audit entry. Invalid joins have no public
alias, a separate coverage/result count, and keep the monster family
partial/unresolved. Regression tests cover full and selected exports, a mixed
comma-separated join, unsafe paths and an all-invalid batch.

The original `mad_general_2.png` contained only **470** visible pixels. Its actual
idle frame puts a spare weapon at Y −1,342…−1,313, far below the body at Y 0…65.
Camera fitting now excludes meshes with a gap greater than four times the largest
mesh's span, without modifying the recorded pose or source geometry. All meshes
still undergo rasterization, and excluded mesh paths remain in the private audit.
The corrected picture has **49,029** visible pixels, a **455×322** visible bounding
box and **135,382** bytes. It shows the posed body and equipped sword. Fixtures
verify stable fitting regardless of mesh order and preservation of nearby parts.
Its SHA-256 is `09936c502578e4ba0dacf510d81caf761260568e986043b01a80bdeea4dadaf6`.

All **79** exporter tests pass. The corrected native rerun uses immutable dataset
`gamedata-48ad548c98056b6c87b3`; its monster audit reports **105 rendered**, one
unsupported and **zero invalid** joins. Only `mad_general_2` excludes a mesh from
camera fitting. Native rerun evidence is under ignored
`exports/uniques-render/review-export/` and `review-export-result.json`.
The rerun completed in **505.259 s**. Source-free validation checks 21 catalogues,
10,142 bundle assets and zero dangling references; public validation checks 12,174
files and 32,279 semantic keys. All 12,069 unrelated records and their bytes are
preserved. Only `mad_general_2.png` changes; the other 104 monster PNGs match their
previous bytes. The public index records the new monster dataset and matching PNG
checksums/sizes. Full verification evidence is
`exports/uniques-render/review-validation.json`.
