"""Publish browser-friendly asset aliases without exporting source provenance."""

from __future__ import annotations

import hashlib
import json
import os
import shutil
import time
from pathlib import Path, PurePosixPath
from typing import Any

from PIL import Image


INDEX_NAME = "asset-index.json"
INDEX_FORMAT = "phmon-game-assets-index"
GENERATOR = "phmon-game-data-exporter"
_SOURCE_ONLY_KEYS = {
    "sourceroot",
    "sourcepath",
    "sourceentry",
    "sourceentrypath",
    "archivepath",
    "pk2path",
    "sourcecolumn",
    "sourcecolumns",
    "sourcetable",
    "decodinginstructions",
}


def _digest(path: Path) -> str:
    hasher = hashlib.sha256()
    with path.open("rb") as file:
        for chunk in iter(lambda: file.read(1024 * 1024), b""):
            hasher.update(chunk)
    return hasher.hexdigest()


def _safe_relative(value: object, *, label: str) -> PurePosixPath:
    if not isinstance(value, str) or not value or "\\" in value or ":" in value:
        raise ValueError(f"{label} is not a safe relative POSIX path")
    parts = value.split("/")
    path = PurePosixPath(value)
    if path.is_absolute() or any(part in ("", ".", "..") for part in parts):
        raise ValueError(f"{label} is not a safe relative POSIX path")
    return path


def _contains_source_knowledge(value: object) -> bool:
    if isinstance(value, dict):
        return any(
            key.casefold() in _SOURCE_ONLY_KEYS or _contains_source_knowledge(child)
            for key, child in value.items()
        )
    if isinstance(value, list):
        return any(_contains_source_knowledge(child) for child in value)
    return False


def _no_symlink_components(path: Path) -> None:
    current = path
    missing: list[Path] = []
    while not current.exists() and current != current.parent:
        missing.append(current)
        current = current.parent
    if current.is_symlink():
        raise ValueError(f"asset output path contains a symlink: {current}")
    for component in reversed(missing):
        if component.exists() and component.is_symlink():
            raise ValueError(f"asset output path contains a symlink: {component}")


def _existing_generated_files(destination: Path) -> set[str] | None:
    if not destination.exists():
        return None
    if destination.is_symlink() or not destination.is_dir():
        raise ValueError("asset output must be a real directory")
    index_path = destination / INDEX_NAME
    if not index_path.exists():
        if any(destination.iterdir()):
            raise ValueError("asset output exists but is not owned by this exporter")
        return set()
    if index_path.is_symlink() or not index_path.is_file():
        raise ValueError("existing asset output index is not a regular file")
    try:
        old_index = json.loads(index_path.read_text(encoding="utf-8"))
    except (OSError, json.JSONDecodeError) as exc:
        raise ValueError("existing asset output index cannot be read") from exc
    if old_index.get("format") != INDEX_FORMAT or old_index.get("generator") != GENERATOR:
        raise ValueError("asset output exists but is not owned by this exporter")
    old_files = old_index.get("files")
    if not isinstance(old_files, list):
        raise ValueError("existing asset output index has an invalid files list")
    expected = {INDEX_NAME}
    for row in old_files:
        if not isinstance(row, dict):
            raise ValueError("existing asset output index has an invalid file entry")
        expected.add(_safe_relative(row.get("path"), label="existing public asset path").as_posix())
    actual: set[str] = set()
    for path in destination.rglob("*"):
        if path.is_symlink():
            raise ValueError("existing asset output contains a symlink")
        if path.is_file():
            actual.add(path.relative_to(destination).as_posix())
    if actual != expected:
        raise ValueError("asset output contains files outside its exporter index")
    expected_dirs = {"/".join(PurePosixPath(name).parts[:index]) for name in expected for index in range(1, len(PurePosixPath(name).parts))}
    actual_dirs = {path.relative_to(destination).as_posix() for path in destination.rglob("*") if path.is_dir()}
    if actual_dirs != expected_dirs:
        raise ValueError("asset output contains directories outside its exporter index")
    return expected


def ensure_public_asset_destination(destination: Path) -> None:
    """Reject unsafe or unowned output directories before archive processing."""
    _no_symlink_components(destination)
    _existing_generated_files(destination.resolve())


def materialize_public_assets(bundle: Path, audit: Path, destination: Path) -> dict[str, Any]:
    """Serialize writes to the public target and publish the finished tree."""
    _no_symlink_components(destination)
    destination = destination.resolve()
    destination.parent.mkdir(parents=True, exist_ok=True)
    lock = destination.parent / f".{destination.name}.asset-export.lock"
    try:
        descriptor = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    except FileExistsError as exc:
        raise ValueError("another export is writing this public asset destination") from exc
    os.close(descriptor)
    try:
        return _materialize_public_assets_locked(bundle, audit, destination)
    finally:
        lock.unlink(missing_ok=True)


def _materialize_public_assets_locked(bundle: Path, audit: Path, destination: Path) -> dict[str, Any]:
    """Create a replaceable public tree from a validated bundle and private audit."""
    bundle = bundle.resolve(strict=True)
    audit = audit.resolve(strict=True)
    ensure_public_asset_destination(destination)
    destination = destination.resolve()
    if destination == bundle or bundle in destination.parents or destination in bundle.parents:
        raise ValueError("asset output must be separate from the bundle")
    if destination == audit or audit in destination.parents or destination in audit.parents:
        raise ValueError("asset output must be separate from exporter audit files")
    manifest = json.loads((bundle / "manifest.json").read_text(encoding="utf-8"))
    audit_rows = json.loads((audit / "assets.json").read_text(encoding="utf-8"))
    if not isinstance(audit_rows, list):
        raise ValueError("exporter asset audit has an invalid format")

    by_key: dict[str, dict[str, Any]] = {}
    for row in manifest.get("assets", []):
        for key in row.get("semanticKeys", []):
            if key in by_key:
                raise ValueError("bundle manifest maps an asset key more than once")
            by_key[key] = row

    aliases: dict[str, dict[str, Any]] = {}
    key_paths: dict[str, str] = {}
    for audit_row in audit_rows:
        if audit_row.get("status") != "converted":
            continue
        key = audit_row.get("assetKey")
        asset = by_key.get(key)
        if asset is None:
            raise ValueError("exporter audit references an asset missing from the bundle")
        source_path = _safe_relative(audit_row.get("sourceEntry"), label="audited source entry")
        if source_path.suffix.casefold() != ".ddj":
            raise ValueError("converted source entry does not have a DDJ extension")
        public_path = source_path.with_suffix(".png")
        public_relative = PurePosixPath(*(part.casefold() for part in public_path.parts)).as_posix()
        prior_path = key_paths.get(key)
        if prior_path is not None and prior_path != public_relative:
            raise ValueError("one semantic asset key maps to multiple public paths")
        key_paths[key] = public_relative
        asset_digest = asset.get("sha256")
        existing = aliases.get(public_relative)
        if existing and existing["sha256"] != asset_digest:
            raise ValueError(f"different assets map to the same public path: {public_relative}")
        if existing is None:
            aliases[public_relative] = {
                "path": public_relative,
                "sha256": asset_digest,
                "mediaType": asset.get("mediaType"),
                "sizeBytes": asset.get("sizeBytes"),
                "width": asset.get("width"),
                "height": asset.get("height"),
                "assetKeys": set(),
                "bundlePath": asset.get("path"),
            }
        aliases[public_relative]["assetKeys"].add(key)

    if not aliases:
        raise ValueError("exporter audit contains no converted assets")

    stage = destination.parent / f".{destination.name}.staging-{os.getpid()}-{time.time_ns()}"
    backup = destination.parent / f".{destination.name}.backup-{os.getpid()}-{time.time_ns()}"
    stage.mkdir(parents=True)
    try:
        public_rows = []
        for relative in sorted(aliases):
            row = aliases[relative]
            bundle_relative = _safe_relative(row["bundlePath"], label="bundle asset path")
            source_file = bundle.joinpath(*bundle_relative.parts)
            if source_file.is_symlink() or not source_file.is_file() or _digest(source_file) != row["sha256"]:
                raise ValueError(f"bundle asset is missing or failed checksum validation: {bundle_relative}")
            output_file = stage.joinpath(*PurePosixPath(relative).parts)
            output_file.parent.mkdir(parents=True, exist_ok=True)
            shutil.copyfile(source_file, output_file)
            if output_file.stat().st_size != row["sizeBytes"] or _digest(output_file) != row["sha256"]:
                raise ValueError(f"public asset failed checksum validation: {relative}")
            public_rows.append({
                "path": relative,
                "url": f"{destination.name}/{relative}",
                "assetKeys": sorted(row["assetKeys"]),
                "sha256": row["sha256"],
                "mediaType": row["mediaType"],
                "sizeBytes": row["sizeBytes"],
                "width": row["width"],
                "height": row["height"],
            })
        index = {
            "format": INDEX_FORMAT,
            "formatVersion": 1,
            "generator": GENERATOR,
            "datasetId": manifest.get("datasetId"),
            "urlPrefix": destination.name,
            "fileCount": len(public_rows),
            "assetKeyCount": sum(len(row["assetKeys"]) for row in public_rows),
            "files": public_rows,
        }
        index_path = stage / INDEX_NAME
        index_path.write_text(json.dumps(index, ensure_ascii=False, sort_keys=True, indent=2) + "\n", encoding="utf-8", newline="\n")

        if destination.exists():
            os.rename(destination, backup)
        try:
            os.rename(stage, destination)
        except Exception:
            if backup.exists() and not destination.exists():
                os.rename(backup, destination)
            raise
        if backup.exists():
            shutil.rmtree(backup)
    except Exception:
        if stage.exists():
            shutil.rmtree(stage, ignore_errors=True)
        raise

    return validate_public_assets(destination)


def validate_public_assets(destination: Path) -> dict[str, Any]:
    _no_symlink_components(destination)
    if destination.is_symlink():
        raise ValueError("public asset path must not be a symlink")
    destination = destination.resolve(strict=True)
    if destination.is_symlink() or not destination.is_dir():
        raise ValueError("public asset path must be a real directory")
    index_path = destination / INDEX_NAME
    if index_path.is_symlink() or not index_path.is_file():
        raise ValueError("public asset index is missing")
    index = json.loads(index_path.read_text(encoding="utf-8"))
    if _contains_source_knowledge(index):
        raise ValueError("public asset index contains exporter-only source knowledge")
    if index.get("format") != INDEX_FORMAT or index.get("formatVersion") != 1 or index.get("generator") != GENERATOR:
        raise ValueError("public asset index format is unsupported")
    rows = index.get("files")
    if not isinstance(rows, list) or index.get("fileCount") != len(rows):
        raise ValueError("public asset index has an invalid file list")
    url_prefix = _safe_relative(index.get("urlPrefix"), label="public asset URL prefix")
    if len(url_prefix.parts) != 1:
        raise ValueError("public asset URL prefix must be one path segment")
    seen_paths: set[str] = set()
    seen_keys: set[str] = set()
    for row in rows:
        relative = _safe_relative(row.get("path"), label="public asset path")
        name = relative.as_posix()
        if name in seen_paths or row.get("url") != f"{url_prefix.as_posix()}/{name}":
            raise ValueError("public asset index contains a duplicate path or invalid URL")
        seen_paths.add(name)
        keys = row.get("assetKeys")
        if not isinstance(keys, list) or not keys or any(not isinstance(key, str) or not key for key in keys):
            raise ValueError("public asset index has invalid semantic asset keys")
        if any(key in seen_keys for key in keys):
            raise ValueError("one semantic asset key maps to multiple public paths")
        seen_keys.update(keys)
        path = destination.joinpath(*relative.parts)
        if path.is_symlink() or not path.is_file() or path.stat().st_size != row.get("sizeBytes") or _digest(path) != row.get("sha256"):
            raise ValueError(f"public asset is missing or has the wrong checksum: {name}")
        with Image.open(path) as image:
            if image.format != "PNG" or image.size != (row.get("width"), row.get("height")):
                raise ValueError(f"public asset is not the indexed browser-ready PNG: {name}")
            image.verify()
    expected = seen_paths | {INDEX_NAME}
    actual: set[str] = set()
    for path in destination.rglob("*"):
        if path.is_symlink():
            raise ValueError("public asset tree contains a symlink")
        if path.is_file():
            actual.add(path.relative_to(destination).as_posix())
    if actual != expected:
        raise ValueError("public asset tree contains files that are missing from its index or not indexed")
    if index.get("assetKeyCount") != len(seen_keys):
        raise ValueError("public asset index semantic key count is inconsistent")
    return {
        "datasetId": index.get("datasetId"),
        "publicFilesValidated": len(rows),
        "semanticAssetKeysValidated": index["assetKeyCount"],
        "sourceKnowledgeRequired": False,
    }
