"""A local static preview that reads only a finished bundle."""

from __future__ import annotations

import json
import mimetypes
import re
from functools import partial
from http.server import BaseHTTPRequestHandler, ThreadingHTTPServer
from pathlib import Path, PurePosixPath
from urllib.parse import parse_qs, unquote, urlsplit

from PIL import Image, ImageDraw

from .cli import validate_bundle

_PAGE = r"""<!doctype html>
<html lang="en"><head><meta charset="utf-8"><meta name="viewport" content="width=device-width, initial-scale=1">
<title>Offline game-data bundle preview</title>
<style>
:root{color-scheme:dark;font:14px/1.45 "Segoe UI",Tahoma,Arial,sans-serif;background:#0d131d;color:#eaf1ff}*{box-sizing:border-box}body{margin:0;min-height:100vh;background:radial-gradient(ellipse at 75% -25%,#263648 0,transparent 40%),#0d131d}header{padding:20px clamp(16px,3vw,42px);border-bottom:1px solid #293648;background:#111a27}h1{font-size:24px;color:#fef6c3;margin:0 0 4px}p{color:#a9b7ca;margin:4px 0}.status{display:inline-block;padding:3px 8px;border:1px solid #46546a;border-radius:4px;color:#f4d77e;font-size:12px}.layout{display:grid;grid-template-columns:220px minmax(0,1fr);min-height:calc(100vh - 96px)}aside{padding:16px;border-right:1px solid #293648;background:#101824}aside label{display:block;font-size:12px;color:#a9b7ca;margin-bottom:6px}select,input{background:#111a27;color:#eaf1ff;border:1px solid #3b4a60;border-radius:4px;min-height:36px;padding:6px 9px;max-width:100%}aside select{width:100%}main{padding:20px clamp(12px,2.5vw,36px);min-width:0}.toolbar{display:flex;gap:8px;align-items:center;flex-wrap:wrap;margin:12px 0}.toolbar input{flex:1;min-width:180px}.grid{display:grid;grid-template-columns:repeat(auto-fill,minmax(150px,1fr));gap:10px}.card{min-width:0;min-height:150px;background:#111a27;border:1px solid #293648;border-radius:4px;padding:10px;overflow:hidden}.thumb{height:104px;display:grid;place-items:center;background:#0b111a;border:1px solid #253348;margin-bottom:8px}.thumb img{max-width:100%;max-height:100%;object-fit:contain;image-rendering:auto}.thumb.pixels img{image-rendering:pixelated;max-width:75px;max-height:75px}.name{font-size:13px;overflow-wrap:anywhere}.muted{font-size:11px;color:#91a0b5;overflow-wrap:anywhere}.empty{padding:20px;border:1px dashed #48566a;color:#b1bed0;border-radius:4px}.map-sheet{display:block;max-width:100%;height:auto;image-rendering:pixelated;border:1px solid #394a61;background:#0b111a}.map-wrap{overflow:auto;background:#0b111a;padding:8px;max-height:72vh}.summary{display:flex;gap:8px;flex-wrap:wrap}.summary span{border:1px solid #293648;background:#111a27;padding:5px 8px;border-radius:4px;color:#c1cde0;font-size:12px}.hidden{display:none}@media(max-width:760px){.layout{grid-template-columns:1fr}aside{border-right:0;border-bottom:1px solid #293648;padding:10px 14px;position:sticky;top:0;z-index:2}.layout{min-height:calc(100vh - 110px)}main{padding:14px}.grid{grid-template-columns:repeat(auto-fill,minmax(118px,1fr));gap:7px}.card{padding:7px;min-height:126px}.thumb{height:88px}.map-wrap{max-height:60vh}}@media(max-width:390px){h1{font-size:20px}.layout{display:block}.grid{grid-template-columns:repeat(2,minmax(0,1fr))}.toolbar input{min-width:0;width:100%;flex-basis:100%}}
</style></head><body><header><h1>Offline game-data bundle preview</h1><p>This is an exporter verification surface. It reads the selected bundle only; it does not reproduce PhMon screens.</p><span id="dataset" class="status">Loading bundle…</span></header>
<div class="layout"><aside><label for="family">Asset family</label><select id="family"><option value="items">Items</option><option value="entities">Entities</option><option value="skills">Skills</option><option value="masteries">Masteries</option><option value="skillGroups">Skill groups</option><option value="teleports">Teleports</option><option value="portraits">Portrait candidates</option><option value="pets">Pet role coverage</option><option value="interfaceSymbols">Non-control symbols</option><option value="backgrounds">Background artwork</option><option value="maps">Minimap tiles</option><option value="regions">Regions</option></select><p id="familyStatus" class="muted"></p></aside><main><div class="summary" id="summary"></div><div class="toolbar" id="toolbar"><input id="search" placeholder="Search names or codes" aria-label="Search catalog"></div><div id="mapControls" class="toolbar hidden"><label for="tileSet">Tile set</label><select id="tileSet"></select><button id="renderMap" type="button">Render tile overview</button><span id="mapNote" class="muted"></span></div><div id="notice"></div><div id="results" class="grid"></div><div id="mapOutput" class="map-wrap hidden"></div></main></div>
<script>
const data={manifest:null,catalogs:{},assets:new Map()};
const $=s=>document.querySelector(s);
function element(tag,cls,text){const e=document.createElement(tag);if(cls)e.className=cls;if(text!==undefined)e.textContent=text;return e}
function addStat(text){$('#summary').append(element('span','',text))}
function assetFor(key){if(!key)return null;return data.assets.get(key)||null}
function showCard(title,detail,key,pixel=false){const card=element('article','card');const thumb=element('div','thumb'+(pixel?' pixels':''));const asset=assetFor(key);if(asset){const img=document.createElement('img');img.loading='lazy';img.alt=title||'Game asset';img.src='./'+asset.path;thumb.append(img)}else thumb.append(element('span','muted','No linked image'));card.append(thumb);card.append(element('div','name',title||'(name unresolved)'));if(detail)card.append(element('div','muted',detail));$('#results').append(card)}
function displayName(value){if(typeof value==='string')return value.trim();if(value&&typeof value==='object'&&!Array.isArray(value)){for(const locale of Object.keys(value).sort()){if(typeof value[locale]==='string'&&value[locale].trim())return value[locale].trim()}}return ''}
function reset(){ $('#results').replaceChildren();$('#summary').replaceChildren();$('#notice').replaceChildren();$('#mapOutput').replaceChildren();$('#mapOutput').classList.add('hidden'); }
function renderMapNote(){const cat=data.catalogs.maps||{};const id=$('#tileSet').value;const orientation=(cat.tileSetOrientations||[]).find(row=>row.tileSetId===id);const axes=orientation?`${id}: X increases ${orientation.xIncreasingDirection||'unresolved'}; Y increases ${orientation.yIncreasingDirection||'unresolved'} (${orientation.status}).`:'';const blackCount=(cat.records||[]).filter(row=>row.tileSetId===id&&row.rasterContentStatus==='uniform-opaque-black').length;const contentNote=blackCount?`${blackCount} uniform-black source tiles are shown with warning hatches; map coverage is unresolved.`:'';$('#mapNote').textContent=[cat.coordinateSemantics,axes,contentNote].filter(Boolean).join(' ')}
function render(){if(!data.manifest)return;reset();const family=$('#family').value;const cat=data.catalogs[family]||{status:'unresolved',records:[]};$('#familyStatus').textContent='Catalog status: '+cat.status;$('#toolbar').classList.toggle('hidden',family==='maps');$('#mapControls').classList.toggle('hidden',family!=='maps');$('#mapOutput').classList.add('hidden');
 if(family==='maps'){const sets=[...new Set(cat.records.map(r=>r.tileSetId))];const sel=$('#tileSet');sel.replaceChildren(...sets.map(s=>{const o=document.createElement('option');o.value=s;o.textContent=s+' ('+cat.records.filter(r=>r.tileSetId===s).length+' tiles)';return o}));renderMapNote();addStat(cat.records.length+' tile records');addStat(sets.length+' tile sets');return}
 let rows=cat.records||[];if(family==='skills')rows=[...rows.filter(row=>row.iconAssetKey),...(cat.unmappedArtCandidates||[])];if(family==='masteries')rows=[...rows.filter(row=>row.iconAssetKey||row.focusIconAssetKey),...(cat.unmappedArtCandidates||[])];
 addStat((cat.records||[]).length+' records');if(cat.artCandidateCount!==undefined)addStat(cat.artCandidateCount+' art files in source family');if(cat.links)addStat(cat.links.length+' endpoint links');if(family==='interfaceSymbols')addStat(rows.length+' symbol candidates');if(family==='portraits')addStat('Entity-to-portrait joins unresolved');if(family==='pets')addStat('Pet body art / role mapping unresolved');
 const q=$('#search').value.trim().toLocaleLowerCase();let shown=0;for(const row of rows){const name=displayName(row.name)||row.displayName||'';const code=row.code||'';if(q&&!String(name+' '+code+' '+row.id).toLocaleLowerCase().includes(q))continue;const key=row.assetKey||row.iconAssetKey||row.associatedIconAssetKey;const identifier=code||(row.referenceId!==undefined?'Reference '+row.referenceId:row.id?.split(':').at(-1)||'');const details=row.name&&!name?('English name unresolved'+(identifier?' · '+identifier:'')):(row.symbolGroup||row.mappingStatus||identifier);showCard(name||code||(row.referenceId!==undefined?'Record '+row.referenceId:row.id),details,key,['skills','masteries','items'].includes(family));if(++shown>=600)break}if(!shown)$('#results').append(element('div','empty','No matching records in this bundle.'));
 if(cat.status==='unresolved')$('#notice').append(element('div','empty','This family has no verified normalized records. Any displayed image candidate is not assigned to a game role.'));
}
async function load(){const response=await fetch('./manifest.json');data.manifest=await response.json();for(const row of data.manifest.catalogs){data.catalogs[row.path.split('/').at(-1).replace('.json','')]=await (await fetch('./'+row.path)).json()}for(const asset of data.manifest.assets)for(const key of asset.semanticKeys||[])data.assets.set(key,asset);$('#dataset').textContent=data.manifest.datasetId+' · '+data.manifest.completionStatus+' · '+data.manifest.assets.length+' unique images';render()}
$('#family').addEventListener('change',()=>{$('#search').value='';render()});$('#tileSet').addEventListener('change',renderMapNote);$('#search').addEventListener('input',render);
$('#renderMap').addEventListener('click',async()=>{const set=$('#tileSet').value;if(!set)return;const box=$('#mapOutput');box.replaceChildren();const image=document.createElement('img');image.className='map-sheet';image.alt='Minimap tile coordinate overview; game-world transform is not validated';image.src='./__map-sheet?setId='+encodeURIComponent(set);box.append(image);box.classList.remove('hidden')});
load().catch(error=>{$('#dataset').textContent='Bundle preview failed';$('#notice').append(element('div','empty',String(error))) });
</script></body></html>"""


def _safe_bundle_file(bundle: Path, relative: str) -> Path:
    path = PurePosixPath(relative)
    if path.is_absolute() or not path.parts or any(part in (".", "..") for part in path.parts) or "\\" in relative:
        raise ValueError("unsafe request path")
    target = bundle.joinpath(*path.parts)
    if target.is_symlink() or not target.resolve(strict=False).is_relative_to(bundle):
        raise ValueError("requested path escapes bundle")
    return target


def _map_sheet(bundle: Path, tile_set_id: str) -> bytes:
    if not re.fullmatch(r"tile-set-\d{3}", tile_set_id):
        raise ValueError("invalid tile set")
    maps_catalog = json.loads((bundle / "catalogs" / "maps.json").read_text(encoding="utf-8"))
    manifest = json.loads((bundle / "manifest.json").read_text(encoding="utf-8"))
    assets = {key: row for row in manifest["assets"] for key in row.get("semanticKeys", [])}
    records = [row for row in maps_catalog["records"] if row["tileSetId"] == tile_set_id]
    if not records:
        raise ValueError("unknown tile set")
    orientations = {row["tileSetId"]: row for row in maps_catalog.get("tileSetOrientations", [])}
    orientation = orientations.get(tile_set_id, {})
    increasing_x_direction = orientation.get("xIncreasingDirection")
    increasing_y_direction = orientation.get("yIncreasingDirection")
    min_x = min(row["x"] for row in records)
    min_y = min(row["y"] for row in records)
    max_x = max(row["x"] for row in records)
    max_y = max(row["y"] for row in records)
    scale = 12
    columns = max_x - min_x + 1
    rows = max_y - min_y + 1
    if columns * rows > 1_000_000:
        raise ValueError("tile coordinate extent exceeds preview bounds")
    sheet = Image.new("RGBA", (columns * scale, rows * scale), (13, 19, 29, 255))
    for tile in records:
        destination = (
            (max_x - tile["x"] if increasing_x_direction == "left" else tile["x"] - min_x) * scale,
            (max_y - tile["y"] if increasing_y_direction == "up" else tile["y"] - min_y) * scale,
        )
        if tile.get("rasterContentStatus") == "uniform-opaque-black":
            placeholder = Image.new("RGBA", (scale, scale), (56, 27, 35, 255))
            draw = ImageDraw.Draw(placeholder)
            for start in range(-scale, scale * 2, 4):
                draw.line((start, 0, start + scale, scale), fill=(219, 118, 71, 255), width=1)
            sheet.alpha_composite(placeholder, destination)
            continue
        asset = assets.get(tile["assetKey"])
        if asset is None:
            continue
        path = _safe_bundle_file(bundle, asset["path"])
        with Image.open(path) as source:
            source.thumbnail((scale, scale), Image.Resampling.BOX)
            sheet.alpha_composite(source.convert("RGBA"), destination)
    from io import BytesIO

    output = BytesIO()
    sheet.save(output, format="PNG", optimize=False, compress_level=9)
    return output.getvalue()


def serve_preview(bundle: Path, host: str = "127.0.0.1", port: int = 8765) -> None:
    bundle = bundle.resolve(strict=True)
    validate_bundle(bundle)

    class Handler(BaseHTTPRequestHandler):
        def do_GET(self) -> None:  # noqa: N802 - stdlib handler spelling
            parsed = urlsplit(self.path)
            route = unquote(parsed.path)
            try:
                if route == "/" or route == "/index.html":
                    body = _PAGE.encode("utf-8")
                    content_type = "text/html; charset=utf-8"
                elif route == "/__map-sheet":
                    set_id = parse_qs(parsed.query).get("setId", [""])[0]
                    body = _map_sheet(bundle, set_id)
                    content_type = "image/png"
                else:
                    relative = route.lstrip("/")
                    target = _safe_bundle_file(bundle, relative)
                    if not target.is_file():
                        self.send_error(404)
                        return
                    body = target.read_bytes()
                    content_type = mimetypes.guess_type(target.name)[0] or "application/octet-stream"
                self.send_response(200)
                self.send_header("Content-Type", content_type)
                self.send_header("Content-Length", str(len(body)))
                self.send_header("X-Content-Type-Options", "nosniff")
                self.send_header("Cache-Control", "no-store")
                self.end_headers()
                self.wfile.write(body)
            except (OSError, ValueError, KeyError, json.JSONDecodeError) as exc:
                self.send_error(400, str(exc))

        def log_message(self, format: str, *args: object) -> None:
            print(f"preview: {self.address_string()} - {format % args}")

    server = ThreadingHTTPServer((host, port), Handler)
    print(f"Offline bundle preview: http://{host}:{server.server_port}/  (Ctrl+C to stop)")
    try:
        server.serve_forever()
    except KeyboardInterrupt:
        pass
    finally:
        server.server_close()
