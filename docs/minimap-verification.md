# Minimap grid and live-position verification — 2026-09-27

This note records read-only comparison evidence for the active minimap task. The
phBot installation is used only as a visual/reference comparison; exported map
images come from GreatestSRO. No bot actions were sent.

## Grid indexing and source comparison

The selected GreatestSRO `Media.pk2` export provides 5,303 direct 256 × 256
minimap PNGs. `Map.pk2` is the selected map archive; its same-sized, different-hash
copy `Map - copia.pk2` is excluded as backup. The local phBot reference folder has
5,794 root JPGs named `<X>x<Y>.jpg` plus separate special-map files under `d/`;
the installed files are not arranged as `Map/<Y>/<X>`.

The archive indices agree with the region table. Region 25000 is hex `0x61A8`,
with `gridX=168`, `gridZ=97`, and `Town_Jangan`; GreatestSRO and the reference both
contain `168x97`, while neither has the reversed `97x168`. Jangan's catalog rows
cover X=167–169, Z=97–98. The source spells Donwhang as `Town_Dunhwang`; its four
rows are (152,102), (153,102), (152,103), and (153,103). Every tile in both named
neighborhoods exists in GreatestSRO and the phBot reference. Contact sheets showing
both candidate vertical orders and both sources are in ignored
`exports/minimap-grid-check-2026-09-27/`.

Archive inventory adds one path-level cross-check: selected `Map.pk2` contains
`map/97/168.o2`, while selected `Media.pk2` contains `minimap/168x97.ddj`. Together
with Region 25000's `gridZ=97` / `gridX=168` and the converted `168x97.png`, this
supports the path convention `Map/<Y>/<X>` and image filename `<X>x<Y>`. The same
Map.pk2 inventory contains `map/102/152.o2` and `map/102/153.o2`, but not
`map/103/152.o2`, `map/103/153.o2`, or the live-region path `map/100/135.o2`.
Those direct minimap images do exist in Media.pk2. This exporter therefore takes
browser minimap rasters from the GreatestSRO Media archive and does not claim that
Map.pk2 has complete raster coverage or silently fall back to phBot files.

The current source tables support a useful exact index join without a world
transform: 2,449 of 2,471 `refregion` records match a tile in the root minimap set
by `(gridX, gridZ) == (x, y)`. All 15 indexed Map.pk2 `map/<Y>/<X>.o2` files also
match region grid pairs in that order. The 22 region records without a root tile
include 17 records at grid `(0,0)` and five nonzero grids: (211,125), (211,126),
(215,126), (219,126), and (220,126). Exporter 0.4.2 writes only exact root-set
matches to `regions.mapAssetKeys`, sets `mapTileIndexStatus` per record and leaves
`worldTransformStatus` unvalidated. The second minimap set is not joined; matching
numeric indices in a separate set do not prove common geography.

## Root raster grid orientation

The raw GreatestSRO root minimap PNGs provide broad seam evidence for how to lay
the tiles out. Across 4,806 horizontally adjacent pairs, comparing 8-pixel edge
bands gave mean RGB error 17.010 when increasing X is placed to the right, versus
36.426 when increasing X is placed to the left. The rightward order had lower
error on 4,132 pairs (394 favored left; 280 tied). Across 4,721 vertically
adjacent pairs, placing increasing Y/Z upward gave mean RGB error 16.576, versus
36.364 for increasing Y/Z downward. The upward order had lower error on 4,100
pairs (336 favored down; 285 tied). These comparisons used the exported GreatestSRO
root tiles only; the phBot JPG directory was not an input.

The exporter records this as `edge-continuity-supported` when an axis has at least
eight adjacent pairs, the preferred direction has lower error on at least 80% of
all comparable pairs (ties count as non-wins), and its mean error is at most 75%
of the alternative. This supports compositing the root grid with X increasing
right and Y/Z increasing upward. The standalone tile overview follows the recorded
direction. It does not prove the local world-coordinate-to-pixel transform inside
a tile, a character marker anchor, or the registration of another tile set to the
root geography. The separate 185-tile `arabia` set independently meets the same
thresholds: 163 horizontal pairs support X increasing right (mean error 17.230
versus 35.814), and 165 vertical pairs support Y increasing up (15.745 versus
39.400). It remains unjoined to `refregion`.

Exporter `0.4.2` / schema `1.2.1` emits those per-set measurements in
`maps.tileSetOrientations`; preview sheets use a supported Y direction and display
the selected set's axes/status. Real dataset
`gamedata-21789415d0f1593162dd` records Region 25000 at `(168,97)` and live Region
25735 at `(135,100)`, both with `exact-grid-match` and
`worldTransformStatus: unvalidated`. The independent rerun reused the
byte-identical 465,397,123-byte bundle. A bundle-only copy validated and served its
two tile sheets with HTTP 200; the copied output contains no audit or PK2 files.

In real dataset `gamedata-00cdb146f754911f1c80`, Region 25000 links to root tile
168x97, and live Region 25735 links to root tile 135x100. Their records report
`exact-grid-match`; both still report `worldTransformStatus: unvalidated`.

All 11 neighborhood/live-position PNGs compare to the corresponding phBot JPGs at
256 × 256. Mean absolute RGB error (from the reference JPG's lossy recompression)
averages 5.843 across the six Jangan tiles, 5.093 across the four Donwhang tiles,
and 2.996 for live-position tile 135x100. The visual tile contents agree; the
phBot files are not inputs to the exporter.

An 8-pixel RGB edge-strip comparison measured Z-increasing-down/up mean errors of
26.89/17.26 for Donwhang and 29.34/31.87 for Jangan. The neighborhoods disagree
about which vertical order joins more closely, so those small samples alone do not
establish row orientation; the full-grid edge pass above supplies the stronger
root-set evidence.

## Full installed-reference directory comparison — 2026-09-27

The phBot reference directory is a flat set of 5,794 root JPGs named `<X>x<Y>.jpg`
plus 2,224 special-map files under `d/`; the Y-folder/X-filename convention belongs
to the GreatestSRO `Map.pk2` archive. A fresh read-only inventory of the selected
`Map.pk2` contained 16,852 entries (16,741 files). It includes `map/97/168.o2` and
does not include the reversed `map/168/97.o2`. The matching Media source entry is
`minimap/168x97.ddj`, and the phBot reference is `168x97.jpg`; `97x168.jpg` is absent.
This verifies Y=97 as the archive folder and X=168 as the filename coordinate.

Comparing every root phBot filename with the exported GreatestSRO root minimap
aliases found 5,118 exported PNG names, 5,117 shared pairs, 677 reference-only names
and one exporter-only name (`110x103`). There were no duplicate export names.
All 5,117 shared pairs are 256 × 256. Across the shared pairs, the RGB mean absolute
error after phBot JPG recompression is 4.9552 (median 4.7016; 95th percentile
6.4051). Thirty pairs exceed MAE 10: X=217–218 and Y=84–99 except Y=91. Every
GreatestSRO PNG in those pairs is opaque solid black while the corresponding phBot
JPG contains visible terrain. The raw `Media.pk2` payload for `217x96.ddj` is a
valid, uncompressed 32-bit DDS whose pixel bytes are `00 00 00 ff`; the exporter
decodes those bytes as opaque black, so this is not a DDJ decoder or axis-transpose
error. `Data.pk2`, `Particles.pk2` and `Music.pk2` have no alternate minimap entries;
the selected `Map.pk2` has only object-placement (`.o2`) entries at some regions, not
replacement minimap rasters. The `Map - copia.pk2` backup remains excluded. The 677
reference-only tiles and 30 black-source tiles remain unresolved; phBot images were
comparison-only and were not copied into the export.

The requested neighborhoods are present on both sides: all six Jangan tiles
(X=167–169, Y=97–98) and all four Donwhang tiles (X=152–153, Y=102–103) compare at
256 × 256, with mean RGB MAE 5.8430 and 5.0927 respectively. The live-region tile
135x100 compares at MAE 2.9955. These checks establish filename-index alignment and
visual agreement for those ten neighborhood tiles; they do not establish a
world-coordinate transform.

## Live position tile check

A read-only GET to
`http://192.168.10.25:3005/api/characters?q` was repeated on 2026-09-27 around
03:06 UTC. It returned four online characters, all in zone `Desert of Mysterious
death`, region 25735 (hex `0x6487`). The exported region row resolves that ID to
`gridX=135`, `gridZ=100`. Their positions range X=98.200–100.000,
Y=1555.300–1559.000 in this snapshot. The GreatestSRO Media archive contains
`135x100.ddj`, the browser alias `minimap/135x100.png` exists, and that PNG matches
the corresponding phBot reference JPG at 256 × 256. Map.pk2 does not contain
`map/100/135.o2`.

A fresh read-only GET at 2026-09-27 03:27:56 UTC again returned four online
characters in Region 25735, with X=98.1–101.5 and Y=1551.1–1560.2. Names were not
retained. This corroborates the region/tile selection at a later time, but the
reference screenshot is not identity- or time-linked to these API rows; it cannot
calibrate a marker's in-tile pixel position.

The supplied populated map screenshot displays `X 99.4, Y 1556.6`, with the portrait
marker near canvas pixel (666, 490), and terrain from the same desert tile family.
Under the unverified assumptions of 1,920 world units per region and the displayed
125% zoom (320 screen pixels per 256-pixel tile), that coordinate predicts the
135x100 tile's top-left near (649, 231). A crop there resembles the exported tile,
but a nearby alternate origin has almost the same image error.

A broader 6 × 6 tile-mosaic comparison favored increasing Z upward: its best
sampled MAE was 21.06 versus 36.58 for increasing Z downward. However, the best-fit
origin was 56 px left and 80 px above the position-derived origin, and placed the
marker over 135x99 instead of 135x100; the position-derived candidate scored 23.90.
The map is visually repetitive and the selected portrait/badge does not reveal its
coordinate anchor. Full-grid seam evidence now supports the root row order, but
the screenshot fit still does not validate the marker transform or establish that
the reference screenshot itself is correctly aligned.

The official [phBot character API reference](https://plugins.phbot.org/phbot-api/character)
documents `get_position()` as region plus X/Y/Z values, but does not define a
world-coordinate-to-tile-pixel transform. Root tile row order has separate
edge-continuity evidence; the exporter still marks world transforms unvalidated.
The current app also has no map screen: `Map` is a navigation label only. Therefore
this evidence does not claim exact in-tile marker placement.

The community-authored [Ikarus Silkroad map-format notes](https://github.com/y-xLeo/ikarus-map-editor/blob/main/SRO_MAP_FORMAT_NOTES.md)
describe the usual outdoor region as 1,920 world units and minimap tiles as
256 × 256 pixels, and show inverted grid-row placement for a north-up composite.
That is useful as a candidate convention, but it is not an authoritative
GreatestSRO/phBot transform specification and does not establish this screenshot's
pixel scale, local-axis direction, panning origin or portrait-marker anchor. It is
not used by the exporter to infer or emit pixel coordinates.

## Current live-row and screenshot cross-check — 2026-09-27

A read-only API GET during the 2026-09-27 04:26 UTC audit returned four online
characters, all in region 25735, with X=94.926–102.840 and Y=1556.002–1562.600.
The region catalog links 25735 to `(gridX, gridZ)=(135,100)` and asset
`tile-set-001:135:100`. The screenshot's selected-character label matches one API
row; that row was at X=98.6, Y=1559.3, state timestamp 04:21:17 UTC. The screenshot
file's filesystem timestamp is 2026-09-26 22:44:05 local time and its coordinate readout is
X=99.4, Y=1556.6, so it is not a same-time capture. The row identity and nearby
position corroborate the same region/tile but do not validate pixel placement.

An exploratory 320-pixel tile crop at 125% zoom, using the community-noted 1,920
world-unit region size and the screenshot marker at canvas `(666,490)`, gave tile
135x100 MAE 20.246 with local Y increasing down and 30.390 with local Y increasing
up. The broader 6 × 6 mosaic registration favored increasing grid Y up (MAE 21.06
versus 36.58) but fitted an origin 56 px left and 80 px above the coordinate-derived
candidate and placed the marker over tile 135x99. These results conflict and are
not an authoritative transform: the screenshot is not time-linked to the API,
pan/origin and marker anchor are unknown, and the community map notes are not a
GreatestSRO/phBot transform specification. Preserve `worldTransformStatus` as
`unvalidated` until a trusted transform or synchronized position/map capture resolves
the conflict.

## Remaining blocker and next action

The known-coordinate screenshot and the tile mosaic do not uniquely determine the
coordinate-to-pixel transform. Exact marker placement still needs a trusted
GreatestSRO/phBot scale, local-axis convention, origin/offset and marker anchor.
The small city seam comparisons remain inconsistent about Z direction, while the
full root-grid comparison strongly supports a north-up row order. The screenshot
registration result above is only exploratory. The independent community format
notes provide a candidate region size but do not close the local-transform gap. Do
not turn a plausible single-point fit or special-map tile presence into a claimed
transform. The confirmed exporter scope has no terrain renderer, and the active
cross-check goal does not make the saved screenshot/API pair time-linked. Exact
character pixels and the separate Jangan Cave / Donwhang Cave / Job Temple transforms
remain unvalidated. Before claiming exact placement, validate the scale, axis
directions, origin and marker anchor from a trusted reference or synchronized
position/map capture. The app still has no Map screen, so no production marker
placement has been implemented or claimed.

No production map UI or later slice was changed as part of this verification.

## Current live-row tile selection (2026-09-27 05:36–05:38 UTC)

Fresh read-only GETs to `/api/characters?q` returned four character rows with
`state_updated_at` within 2 seconds of the response. At 05:36:29 UTC,
three rows in Region 25735 joined through `regions.json` to `gridX=135`,
`gridZ=100`, then to the exact root minimap tile `(135,100)`. The fourth row in
Region 24453 joined to `(133,95)`. Both region rows report `exact-grid-match`,
and both raster records report `has-nonblack-pixels`. Tile 133x95 is present in
GreatestSRO Media as `minimap/133x95.ddj`; its 256 × 256 PNG has RGB MAE 5.1911
against the 256 × 256 phBot comparison JPG.

The map screenshot's selected-character label was checked against a later live
row at 05:37:55 UTC: that row was Region 25735, X=97.6, Y=1556.8, with
`state_updated_at` 05:37:54 UTC, and therefore selects tile `(135,100)`. The
screenshot displays X=99.4, Y=1556.6 and its filesystem time is 20:44:05 UTC on
2026-09-26, about 8 hours 54 minutes before that API row. The close coordinates
and matching tile are useful corroboration, but the captures are not synchronized
and cannot validate the marker's pixels.

Rechecked archive convention and named areas from the current source: Map.pk2
contains `map/97/168.o2` and not `map/168/97.o2`; Media.pk2 contains
`minimap/168x97.ddj` and not its reversed name. Thus the Map path's folder is Y=97
and filename is X=168, matching the raster filename `<X>x<Y>`. The six Jangan
tiles `(167..169,97..98)` are all present in both folders, are 256 × 256, and
have mean RGB MAE 5.8430. The four Donwhang tiles `(152..153,102..103)` are also
all present in both folders and are 256 × 256, with mean RGB MAE 5.0927. Map.pk2
contains the Donwhang `map/102/152.o2` and `map/102/153.o2` paths; the Y=103 O2
paths are absent, while its direct minimap rasters still exist in Media.pk2 and
match the phBot comparison. No minimap raster was taken from phBot.

**Result:** live region-to-tile selection is reverified with current rows and the
named city rasters still agree with the phBot reference within JPG recompression
error. Exact world-coordinate-to-pixel placement remains unverified. The web app
has no Map component/marker layer yet (`web/app` contains only the shared
`app.vue` and stylesheet, and its character detail labels map position as Slice
7), so there is no live PhMon marker to inspect. Do not infer an in-tile transform
from the unsynchronized screenshot or add a guessed overlay.

Official phBot documentation corroborates this limit: [`get_position()`](https://plugins.phbot.org/phbot-api/character)
documents only the region and X/Y/Z position fields, without a tile-to-pixel
transform. The [phBot Map guide](https://guide.phbot.org/phbot/map) says its map is
not 100% accurate. These sources support tile selection and approximate display;
they do not provide a trusted pixel calibration for this GreatestSRO map.

## Exporter raster-content status (2026-09-27)

The later exporter `0.4.3` / schema `1.2.2` pixel scan reports 207 opaque-black
source minimap rasters across both GreatestSRO tile sets: 193 root tiles and 14
`arabia` tiles. The 30 files previously identified by comparison with visible
phBot terrain are a subset, not the complete count of black client rasters. The
exported map catalog is partial and the standalone preview hatches all 193 root-set
tiles instead of displaying them as terrain. No phBot JPEG was copied to either
asset output. The 22 region rows lacking an exact root tile remain separately
unresolved; the full map audit accounts for 229 unresolved records. This corrects
the scope of the earlier 30-tile visual mismatch note without changing its
comparison results or claiming an in-tile coordinate transform.

## Fresh API and copied-preview check (2026-09-27 05:52 UTC)

A read-only GET to `http://192.168.10.25:3005/api/characters?q` returned HTTP 200
JSON and four rows. A follow-up at 05:52:24 UTC found every `state_updated_at`
within one second of the response; all four rows were in Region 25735. The real
exported region record has `(gridX,gridZ)=(135,100)`, `exact-grid-match`, and
`has-nonblack-pixels`, so all four rows select root tile `(135,100)` regardless of
their varying local X/Y values. This is fresh corroboration of the discrete
region-to-tile lookup only; it does not establish a character's pixel within that
tile.

Re-inspection of the real normalized records confirms this boundary in the data:
Region 25735 has `gridX=135`, `gridZ=100`, and `worldTransformStatus=unvalidated`;
the region/map records contain no world-unit scale, tile origin or projected pixel
coordinate. Map tile records provide only the raster's grid X/Y, dimensions and
asset key. This catalog cannot independently calculate marker pixels.

The source-independent copy `exports/standalone-copy-0.4.3/bundle` was served by
the standalone preview at `http://127.0.0.1:8765/`. Requests for the preview page,
manifest, maps catalog, the manifest-resolved PNG for map tile 135x100, and generated
tile sheets for `tile-set-001` and `tile-set-002` returned HTTP 200 with HTML, JSON,
or PNG content types as appropriate. These requests used the copied bundle, which
contains no GreatestSRO archive or exporter audit. The current PhMon web application
still has no Map screen or marker layer, so production pixel placement remains
unimplemented and unreviewable here.

## Nuxt public URL check (2026-09-27 05:58 UTC)

To check the generated Nuxt destination in this checkout, its dev server was run
temporarily at `127.0.0.1:3006` and then stopped. GETs for
`/game-assets/minimap/135x100.png`,
`/game-assets/icon/skill/china/bow_area_a.png`, and
`/game-assets/asset-index.json` returned HTTP 200 as `image/png`, `image/png`, and
`application/json`. The response sizes were 86,931, 1,790, and 5,249,420 bytes;
each response SHA-256 matched its file under `web/public/game-assets/`. The separate
remote PhMon origin `192.168.10.25:3005` returned the HTML app document for these
asset paths; no local listener was present on port 3005, so that host was not used
as evidence about this checkout's Nuxt asset serving.

## Fresh archive, tile and live-position recheck (2026-09-27 06:05 UTC)

Re-hashed the read-only source archives. Selected `Map.pk2` is 1,092,251,648 bytes
with SHA-256 `a819141950fed83d2a293eed6ebb96e22ec6f8eceb407eb677fbc06656562183`,
matching the selected-map audit entry. `Map - copia.pk2` has the same byte length
but SHA-256 `e15072a89e37069a8a0e98bd18f592432a944fa97f4b26d6ba49252710928dfe` and
remains excluded as the operator-designated backup. The selected Map inventory
contains `map/97/168.o2` (13,170 bytes) and `map/102/152.o2` (3,570 bytes); it has
neither reversed `map/168/97.o2` nor `map/103/152.o2`. Thus the archive folder is
Y and its filename is X, consistent with the Media raster and phBot reference name
`168x97`.

Recompared all 12 256 × 256 selected exported PNGs to the installed phBot JPGs:
the six Jangan tiles average RGB MAE 5.8430, the four Donwhang tiles 5.0927, live
tile `135x100` 2.9955 and tile `133x95` 5.1911; the combined mean is 5.3013.
This reconfirms tile-content alignment for the checked names with JPG recompression
accounted for. No phBot image entered the public export.

A new read-only GET at 06:05:19 UTC returned HTTP 200 JSON with four character
rows; `state_updated_at` values were 06:05:18–06:05:19 UTC. All four were in
Region 25735 (X=85.800–98.157, Y=1557.047–1562.200), which joins exactly to the
nonblack root raster `(135,100)`. This verifies the live tile choice for this
snapshot. The supplied `07-map.png` has filesystem timestamp 2026-09-26 20:44:05
UTC and shows selected coordinates X=99.4,Y=1556.6 with a marker near canvas
`(666,490)`; it predates this API read by over nine hours. Since there is no
synchronized map capture and no Map screen/marker layer in this checkout, these
rows cannot validate actual rendered marker pixels. Do not guess a marker transform
or claim the live characters were visually checked at current coordinates.

## Installed phBot reference-grid seam audit (2026-09-27)

Independently checked all 5,794 flat root JPGs in the installed phBot minimap
folder, comparing 8-pixel edge bands for every neighboring filename pair. Across
5,407 horizontal pairs, putting increasing X to the right gave mean RGB edge MAE
17.193 versus 35.320 for increasing X to the left; rightward was lower on 4,656
pairs (489 favored left, 262 tied). Across 5,298 vertical pairs, putting increasing
Y upward gave mean 16.555 versus 35.401 for increasing Y downward; upward was lower
on 4,647 pairs (397 favored down, 254 tied). This independent reference-only result
supports the exported root-sheet orientation and agrees with the selected archive's
`Map/<Y>/<X>` path and `<X>x<Y>` raster convention. The phBot JPGs remain comparison
inputs only, never exported sources. Seam continuity still does not define local
world X/Y pixel axes inside a tile, coordinate scale/origin, marker anchor or
special-area transforms.

## New live snapshot across two regions (2026-09-27 07:32 UTC)

A fresh read-only API GET at 07:32:48 UTC returned four rows whose state timestamps
were 07:32:46–07:32:47 UTC. Three rows in Region 25735 (X=92.700–100.000,
Y=1555.100–1559.000) joined exactly to nonblack root tile `(135,100)`. One row in
Region 23941 (X=-215.070, Y=231.064) joined exactly to nonblack root tile
`(133,93)`. Both region records have `worldTransformStatus=unvalidated`. The newly
observed 133x93 raster and installed phBot reference are both 256 × 256 and have
RGB MAE 5.5813; tile 135x100 remains at MAE 2.9955. Thus the live region-to-tile
join is verified across two distinct positions/tiles, rather than only the prior
135x100 cluster.

This response is still more than ten hours later than the saved `07-map.png`; no
current map renderer exists in `web/app`. It cannot prove where these characters
would render within either tile. The candidate 1920-unit transform remains
untrusted, and the `worldTransformStatus` stays unresolved.
