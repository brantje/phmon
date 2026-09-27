from __future__ import annotations

import argparse
import json
import os
import sys
from pathlib import Path

from PIL import Image

from . import __version__
from .pk2 import PK2Archive, PK2Error, sha256_file

ARCHIVES = (
    ("data", "Data.pk2", "game_data"),
    ("media", "Media.pk2", "catalogs_and_ui"),
    ("maps", "Map.pk2", "maps"),
    ("music", "Music.pk2", "music_candidates_excluded_by_operator"),
    ("particles", "Particles.pk2", "visual_effects"),
)


def _json_write(path: Path, value: object) -> None:
    path.parent.mkdir(parents=True, exist_ok=True)
    text = json.dumps(value, ensure_ascii=False, sort_keys=True, indent=2) + "\n"
    path.write_text(text, encoding="utf-8", newline="\n")


def inspect_source(source: Path, audit: Path, key: str) -> dict:
    if not source.is_dir():
        raise ValueError(f"source directory does not exist: {source}")
    source_resolved = source.resolve(strict=True)
    audits = []
    missing = []
    for role, filename, description in ARCHIVES:
        path = source_resolved / filename
        if not path.is_file():
            missing.append({"role": role, "expectedFile": filename, "reason": "missing"})
            continue
        before = path.stat()
        digest = sha256_file(path)
        with PK2Archive(path, key=key) as archive:
            inventory = archive.inventory()
            after = path.stat()
            if (before.st_size, before.st_mtime_ns) != (after.st_size, after.st_mtime_ns):
                raise ValueError(f"source archive changed during inspection: {filename}")
            rows = [
                {
                    "path": entry.path,
                    "kind": "directory" if entry.kind == 1 else "file",
                    "offset": entry.position,
                    "size": entry.size,
                    "nextBlockOffset": entry.next_block,
                }
                for entry in inventory.entries
            ]
        _json_write(audit / "inventory" / f"{role}.json", rows)
        audits.append(
            {
                "role": role,
                "description": description,
                "sourcePath": str(path),
                "sizeBytes": before.st_size,
                "sha256": digest,
                "pk2Encrypted": inventory.encrypted,
                "headerChecksumMatched": inventory.checksum_valid,
                "entryCount": inventory.entry_count,
                "fileCount": inventory.file_count,
                "directoryCount": inventory.directory_count,
                "extensionCounts": inventory.extension_counts,
                "inventoryFile": f"inventory/{role}.json",
            }
        )
    backup = source_resolved / "Map - copia.pk2"
    if backup.is_file():
        audits.append(
            {
                "role": "maps_backup_excluded",
                "sourcePath": str(backup),
                "sizeBytes": backup.stat().st_size,
                "sha256": sha256_file(backup),
                "selectionReason": "operator selected Map.pk2; this same-size, different-hash copy is excluded",
            }
        )
    sources = {
        "auditVersion": 1,
        "exporterVersion": __version__,
        "sourceRoot": str(source_resolved),
        "selectedArchives": audits,
        "missingArchives": missing,
        "keyMaterialRecorded": False,
    }
    _json_write(audit / "sources.json", sources)
    _json_write(
        audit / "coverage.json",
        {
            "reportVersion": 1,
            "sourceInspection": "complete" if not missing else "incomplete",
            "archivesInspected": sum(row["role"] != "maps_backup_excluded" for row in audits),
            "archivesMissing": missing,
            "archiveRoles": [
                {"role": role, "expectedFile": filename, "intendedContents": description}
                for role, filename, description in ARCHIVES
            ],
            "unresolved": [],
        },
    )
    return sources


def validate_bundle(bundle: Path) -> dict:
    bundle = bundle.resolve(strict=True)
    if not bundle.is_dir() or bundle.is_symlink():
        raise ValueError("bundle path must be a real directory")
    manifest_path = bundle / "manifest.json"
    if not manifest_path.is_file():
        raise ValueError(f"bundle manifest is missing: {manifest_path}")
    manifest = json.loads(manifest_path.read_text(encoding="utf-8"))
    if _contains_source_knowledge(manifest):
        raise ValueError("bundle manifest contains exporter-only source knowledge")
    if manifest.get("completionStatus") not in ("complete", "incomplete"):
        raise ValueError("manifest has invalid completionStatus")
    catalog_rows = manifest.get("catalogs")
    if not isinstance(catalog_rows, list):
        raise ValueError("manifest catalogs must be a list")
    checked_catalogs = 0
    checked_assets = 0
    asset_keys: set[str] = set()
    referenced_asset_keys: set[str] = set()
    catalogs_by_family: dict[str, dict] = {}
    for row in catalog_rows:
        relative = row.get("path")
        if not isinstance(relative, str) or Path(relative).is_absolute() or ".." in Path(relative).parts or "\\" in relative or ":" in relative:
            raise ValueError("manifest contains an unsafe catalog path")
        path = bundle / relative
        if path.is_symlink() or not path.resolve(strict=False).is_relative_to(bundle):
            raise ValueError("manifest catalog escapes through a symlink")
        if not path.is_file() or sha256_file(path) != row.get("sha256"):
            raise ValueError(f"catalog is missing or has the wrong checksum: {relative}")
        catalog = json.loads(path.read_text(encoding="utf-8"))
        if catalog.get("datasetId") != manifest.get("datasetId"):
            raise ValueError(f"catalog dataset ID mismatch: {relative}")
        if _contains_source_knowledge(catalog):
            raise ValueError(f"catalog leaks exporter-only source knowledge: {relative}")
        if catalog.get("catalogVersion") != manifest.get("schemaVersion"):
            raise ValueError(f"catalog schema version mismatch: {relative}")
        family = catalog.get("family")
        records = catalog.get("records")
        if not isinstance(family, str) or not isinstance(records, list):
            raise ValueError(f"catalog family or records are invalid: {relative}")
        if family in catalogs_by_family:
            raise ValueError(f"duplicate catalog family: {family}")
        record_ids = [record.get("id") for record in records if isinstance(record, dict)]
        if len(record_ids) != len(records) or any(not isinstance(record_id, str) or not record_id for record_id in record_ids) or len(set(record_ids)) != len(record_ids):
            raise ValueError(f"catalog record identities are missing or duplicated: {relative}")
        catalogs_by_family[family] = catalog
        referenced_asset_keys.update(_asset_references(catalog))
        checked_catalogs += 1
    maps_catalog = catalogs_by_family.get("maps")
    map_tile_asset_keys = {
        record.get("assetKey")
        for record in maps_catalog.get("records", [])
        if isinstance(record, dict) and isinstance(record.get("assetKey"), str)
    } if maps_catalog else set()
    map_raster_content_by_key: dict[str, str] = {}
    for row in manifest.get("assets", []):
        relative = row.get("path")
        if not isinstance(relative, str) or Path(relative).is_absolute() or ".." in Path(relative).parts or "\\" in relative or ":" in relative:
            raise ValueError("manifest contains an unsafe asset path")
        if not relative.startswith("assets/") or not relative.endswith(".png"):
            raise ValueError("asset path is outside the browser-ready asset folders")
        path = bundle / relative
        if path.is_symlink() or not path.resolve(strict=False).is_relative_to(bundle):
            raise ValueError("manifest asset escapes through a symlink")
        if not path.is_file() or path.stat().st_size != row.get("sizeBytes"):
            raise ValueError(f"asset is missing or has the wrong size: {relative}")
        if sha256_file(path) != row.get("sha256"):
            raise ValueError(f"asset checksum mismatch: {relative}")
        if row.get("mediaType") != "image/png":
            raise ValueError(f"unsupported exported media type: {relative}")
        with Image.open(path) as image:
            if image.format != "PNG" or image.size != (row.get("width"), row.get("height")):
                raise ValueError(f"asset format or dimensions mismatch: {relative}")
            image.verify()
        keys = row.get("semanticKeys")
        if not isinstance(keys, list) or not keys:
            raise ValueError(f"asset has no semantic keys: {relative}")
        for key in keys:
            if not isinstance(key, str) or not key or key in asset_keys:
                raise ValueError("manifest contains a duplicate or invalid semantic asset key")
            asset_keys.add(key)
        map_keys = [key for key in keys if key in map_tile_asset_keys]
        if map_keys:
            with Image.open(path) as image:
                extrema = image.convert("RGBA").getextrema()
            content_status = (
                "uniform-opaque-black"
                if extrema == ((0, 0), (0, 0), (0, 0), (255, 255))
                else "has-nonblack-pixels"
            )
            for key in map_keys:
                map_raster_content_by_key[key] = content_status
        checked_assets += 1
    dangling = sorted(referenced_asset_keys - asset_keys)
    if dangling:
        raise ValueError(f"catalog references missing semantic assets: {dangling[:5]}")

    mastery_ids = {record["id"] for record in catalogs_by_family.get("masteries", {}).get("records", [])}
    for group in catalogs_by_family.get("skillGroups", {}).get("records", []):
        if group.get("masteryId") is not None and group["masteryId"] not in mastery_ids:
            raise ValueError("skill group references a missing mastery record")
    region_ids = {record["id"] for record in catalogs_by_family.get("regions", {}).get("records", [])}
    for teleport in catalogs_by_family.get("teleports", {}).get("records", []):
        if teleport.get("regionId") is not None and teleport["regionId"] not in region_ids:
            raise ValueError("teleport references a missing region record")
    teleport_reference_ids = {record.get("referenceId") for record in catalogs_by_family.get("teleports", {}).get("records", [])}
    for link in catalogs_by_family.get("teleports", {}).get("links", []):
        if link.get("fromTeleportId") not in teleport_reference_ids or link.get("toTeleportId") not in teleport_reference_ids:
            raise ValueError("teleport link references a missing endpoint")
    if maps_catalog is not None:
        tile_set_ids = {record.get("tileSetId") for record in maps_catalog["records"]}
        orientation_rows = maps_catalog.get("tileSetOrientations")
        if not isinstance(orientation_rows, list):
            raise ValueError("map tile-set orientation data is invalid")
        seen_orientation_ids: set[str] = set()
        for row in orientation_rows:
            if not isinstance(row, dict) or row.get("tileSetId") not in tile_set_ids or row["tileSetId"] in seen_orientation_ids:
                raise ValueError("map tile-set orientation has a missing or duplicate tile set")
            seen_orientation_ids.add(row["tileSetId"])
            if row.get("status") not in ("edge-continuity-supported", "partial-edge-continuity-support", "insufficient-adjacencies", "inconclusive"):
                raise ValueError("map tile-set orientation status is invalid")
            if row.get("xIncreasingDirection") not in (None, "right", "left") or row.get("yIncreasingDirection") not in (None, "up", "down"):
                raise ValueError("map tile-set orientation direction is invalid")
            if row["status"] == "edge-continuity-supported" and (row["xIncreasingDirection"] is None or row["yIncreasingDirection"] is None):
                raise ValueError("fully supported map orientation is missing an axis direction")
        if manifest.get("schemaVersion") == "1.2.2":
            black_count = 0
            for tile in maps_catalog["records"]:
                declared = tile.get("rasterContentStatus")
                actual = map_raster_content_by_key.get(tile.get("assetKey"))
                if declared not in ("uniform-opaque-black", "has-nonblack-pixels") or actual is None:
                    raise ValueError("map tile raster content status is missing or invalid")
                if declared != actual:
                    raise ValueError("map tile raster content status disagrees with its PNG pixels")
                black_count += declared == "uniform-opaque-black"
            if maps_catalog.get("uniformOpaqueBlackTileCount") != black_count:
                raise ValueError("map catalog uniform-black tile count is inconsistent")
            manifest_map_coverage = manifest.get("coverage", {}).get("maps", {})
            if manifest_map_coverage.get("uniformOpaqueBlackTiles") != black_count:
                raise ValueError("manifest uniform-black map tile count is inconsistent")
            if black_count and maps_catalog.get("status") != "partial":
                raise ValueError("map catalog with uniform-black tiles must be partial")
    regions_catalog = catalogs_by_family.get("regions")
    if regions_catalog is not None and manifest.get("schemaVersion") == "1.2.2":
        for region in regions_catalog["records"]:
            status = region.get("mapTileContentStatus")
            if status not in (None, "uniform-opaque-black", "has-nonblack-pixels"):
                raise ValueError("region map tile content status is invalid")
    return {
        "datasetId": manifest["datasetId"],
        "completionStatus": manifest["completionStatus"],
        "catalogsValidated": checked_catalogs,
        "assetsValidated": checked_assets,
        "sourceKnowledgeRequired": False,
        "semanticAssetKeysValidated": len(asset_keys),
        "danglingAssetReferences": 0,
        "normalizedRelationsValidated": True,
        "uniformOpaqueBlackMapTiles": sum(status == "uniform-opaque-black" for status in map_raster_content_by_key.values()),
    }


def _contains_source_knowledge(value: object) -> bool:
    forbidden = {
        "sourcepath", "sourceroot", "sourcearchive", "sourceentrypath", "entrypath",
        "archivepath", "pk2path", "offset", "nextblockoffset", "sourcecolumn",
        "sourcecolumns", "sourcetable", "decodinginstructions", "archiverole",
        "sourceencoding",
    }
    if isinstance(value, dict):
        return any(key.casefold() in forbidden or _contains_source_knowledge(child) for key, child in value.items())
    if isinstance(value, list):
        return any(_contains_source_knowledge(child) for child in value)
    return False


def _asset_references(value: object) -> set[str]:
    found: set[str] = set()
    if isinstance(value, dict):
        for key, child in value.items():
            if key.casefold().endswith("assetkey") and isinstance(child, str):
                found.add(child)
            found.update(_asset_references(child))
    elif isinstance(value, list):
        for child in value:
            found.update(_asset_references(child))
    return found


def _parser() -> argparse.ArgumentParser:
    parser = argparse.ArgumentParser(prog="phmon-game-data")
    parser.add_argument("--version", action="version", version=f"%(prog)s {__version__}")
    commands = parser.add_subparsers(dest="command", required=True)
    inspect = commands.add_parser("inspect", help="inventory selected read-only client archives")
    inspect.add_argument("--source", type=Path, required=True)
    inspect.add_argument("--audit", type=Path, required=True)
    inspect.add_argument("--key", default=os.environ.get("SRO_PK2_KEY", "169841"), help=argparse.SUPPRESS)
    export = commands.add_parser("export", help="build a staged, validated browser-ready bundle")
    export.add_argument("--source", type=Path, required=True)
    export.add_argument("--output", type=Path, required=True)
    export.add_argument("--asset-output", type=Path, help="also publish browser paths and an asset index to this directory")
    export.add_argument("--key", default=os.environ.get("SRO_PK2_KEY", "169841"), help=argparse.SUPPRESS)
    validate = commands.add_parser("validate", help="validate an exported bundle without sources")
    validate.add_argument("--bundle", type=Path, required=True)
    validate.add_argument("--public-assets", type=Path, help="also validate a copied public asset tree without source archives")
    preview = commands.add_parser("preview", help="serve the copied bundle in a standalone browser preview")
    preview.add_argument("--bundle", type=Path, required=True)
    preview.add_argument("--host", default="127.0.0.1")
    preview.add_argument("--port", type=int, default=8765)
    return parser


def main() -> int:
    args = _parser().parse_args()
    try:
        if args.command == "inspect":
            report = inspect_source(args.source, args.audit, args.key)
            inspected = sum(row["role"] != "maps_backup_excluded" for row in report["selectedArchives"])
            print(json.dumps({"inspected": inspected, "audit": str(args.audit.resolve())}, indent=2))
            return 0 if not report["missingArchives"] else 2
        if args.command == "validate":
            result = validate_bundle(args.bundle)
            if args.public_assets:
                from .public_assets import validate_public_assets

                result["publicAssets"] = validate_public_assets(args.public_assets)
            print(json.dumps(result, indent=2))
            return 0
        if args.command == "export":
            from .exporter import export_dataset

            result = export_dataset(args.source, args.output, args.key, args.asset_output)
            print(json.dumps(result, indent=2))
            return 0
        if args.command == "preview":
            from .preview import serve_preview

            serve_preview(args.bundle, args.host, args.port)
            return 0
        raise AssertionError(args.command)
    except (OSError, ValueError, PK2Error, json.JSONDecodeError) as exc:
        print(f"error: {exc}", file=sys.stderr)
        return 2


if __name__ == "__main__":
    raise SystemExit(main())
