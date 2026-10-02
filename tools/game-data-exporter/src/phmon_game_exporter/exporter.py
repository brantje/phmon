"""Deterministic, offline export of supported GreatestSRO game-data families."""

from __future__ import annotations

import hashlib
import json
import os
import re
import shutil
import time
from collections import Counter, defaultdict
from contextlib import ExitStack
from datetime import UTC, datetime
from pathlib import Path, PurePosixPath
from typing import Any

from PIL import Image

from . import __version__
from .cli import ARCHIVES, _json_write
from .mapgrid import infer_tile_grid_orientation
from .monster_icons import MONSTER_ICONS
from .monster_export import MODEL_NAME, collect_monster_targets, export_monsters
from .item_metadata import item_metadata, magic_option_definitions
from .portrait_mapping import PORTRAIT_MODEL_RANGES, portrait_for_model, portrait_source_path
from .pk2 import Entry, PK2Archive, PK2Error, sha256_file
from .textures import TextureError, ddj_to_png

SCHEMA_VERSION = "1.2.2"
TEXT_ROOT = "server_dep/silkroad/textdata/"
ITEM_COMMON_FIELDS = 55
ENTITY_COMMON_FIELDS = 55
ITEM_SHARD = re.compile(r"ItemData_(\d+)\.txt\Z", re.IGNORECASE)
ENTITY_SHARD = re.compile(r"CharacterData_(\d+)\.txt\Z", re.IGNORECASE)
SKILL_SHARD = re.compile(r"SkillData_(\d+)\.txt\Z", re.IGNORECASE)
TILE_FILE = re.compile(r"(?P<x>\d+)x(?P<y>\d+)\.ddj\Z", re.IGNORECASE)
CAVE_TILE_FILE = re.compile(r"(?P<prefix>(?:dh_a01_floor0[1-4]|qt_a01_floor0[1-6]|rn_sd_egypt1_0[12]|rn_sd_egypt01_0[2-6]))_(?P<x>\d+)x(?P<y>\d+)\.ddj\Z", re.IGNORECASE)
EXCLUDED_BY_OPERATOR = {
    "sounds": "operator excluded sound export",
    "interfaceControls": "operator excluded interface controls and control artwork",
}
INTERFACE_SYMBOL_PATTERNS = (
    ("minimapMarker", re.compile(r"interface/minimap/mm_sign_[^/]+\.ddj\Z", re.IGNORECASE)),
    ("chatChannel", re.compile(r"interface/chattingwnd/chat_lamp_[^/]+\.ddj\Z", re.IGNORECASE)),
    ("cityMarker", re.compile(r"interface/worldmap/map/city_[a-z]+\.ddj\Z", re.IGNORECASE)),
    ("mapSymbol", re.compile(r"interface/worldmap/map/(?:map_arrow|map_icon_tel\d+|map_war_icon_[^/]+|xy_(?:guild|market_?\d*|speciality|ch_spear))\.ddj\Z", re.IGNORECASE)),
    ("mapMarker", re.compile(r"interface/worldmap/(?:wmap_sign_(?:apprenticeship|party|quest_a|questnpc|unionparty)(?:_02)?|wmap_guild_sign_location)\.ddj\Z", re.IGNORECASE)),
    ("statusEffect", re.compile(r"icon/stateodd/s_[^/]+_icon\.ddj\Z", re.IGNORECASE)),
    ("eventCategory", re.compile(r"interface/achievements/achiev_icon_(?:event|mini_event(?:_gray)?|mini_quest(?:_gray)?|mini_unique(?:_gray)?|quest|unique)\.ddj\Z", re.IGNORECASE)),
    ("timelineCategory", re.compile(r"interface/title/title_icon_(?:event|mini_event|mini_quest|mini_unique|quest|unique)\.ddj\Z", re.IGNORECASE)),
    ("statSymbol", re.compile(r"interface/ifcommon/com_(?:honor_level_[1-5]|icon_level_[1-6]|itemsign_rare|level_arrowt)\.ddj\Z", re.IGNORECASE)),
    ("resourceSymbol", re.compile(r"interface/(?:character/chr_(?:hp|mp|point_hp|point_mp|stat_point)|messagebox/msgbox_gold_icon|pet/pet_icon_(?:hp|sp)|playerminiinfo/pmi_(?:hp|mp|pet_hp)|targetwindow/tw_(?:hp|hp_npc|mp|icon_unique))\.ddj\Z", re.IGNORECASE)),
    ("eventStatSymbol", re.compile(r"interface/ifcommon/gopro/(?:event_statistics|map_dead_unique_location|party_join_guide_0)\.ddj\Z", re.IGNORECASE)),
    ("questSymbol", re.compile(r"interface/worldmap/wmap_quest_icon\.ddj\Z", re.IGNORECASE)),
    ("economySymbol", re.compile(r"icon/etc/icon/mini_gold_icon\.ddj\Z", re.IGNORECASE)),
    ("categorySymbol", re.compile(r"interface/underbar/ub_new_icon_[^/]+\.ddj\Z", re.IGNORECASE)),
    ("guildSymbol", re.compile(r"interface/guild/gil_(?:arena_dragon_team|arena_tiger_team|blue_flag|blue_tiger|honor_medals|icon0[1-3]|mark01|red_dregon|red_flag)\.ddj\Z", re.IGNORECASE)),
    ("partySymbol", re.compile(r"interface/party/pt_(?:association|flag|no_face|no_party|tabicon_uni_nomal|union_master)\.ddj\Z", re.IGNORECASE)),
)
PORTRAIT_CANDIDATE = re.compile(r"interface/character/char_(?P<race>ch|eu)_(?P<gender>man|woman)(?P<ordinal>\d+)\.ddj\Z", re.IGNORECASE)


class ExportError(ValueError):
    """The dataset could not be safely converted to a validated bundle."""


def _archive_index(entries: tuple[Entry, ...]) -> dict[str, Entry]:
    result: dict[str, Entry] = {}
    for entry in entries:
        if entry.kind != 2:
            continue
        key = entry.path.casefold()
        if key in result:
            raise ExportError(f"case-colliding source entries prevent safe lookup: {entry.path}")
        result[key] = entry
    return result


def _extract_textdata(media, index: dict[str, Entry], destination: Path) -> dict[str, Any]:
    """Copy every textdata file without decoding or filtering its contents."""
    destination.mkdir(parents=True)
    records = []
    for source_path, entry in sorted(index.items()):
        if not source_path.startswith(TEXT_ROOT):
            continue
        relative = entry.path[len(TEXT_ROOT):]
        parts = relative.split("/")
        if (
            not relative
            or any(part in ("", ".", "..") for part in parts)
            or any(char in relative for char in ("\\", ":"))
            or any(ord(char) < 32 for char in relative)
        ):
            raise ExportError(f"unsafe textdata output path: {entry.path}")
        target = destination.joinpath(*parts)
        target.parent.mkdir(parents=True, exist_ok=True)
        payload = media.read_payload(entry)
        target.write_bytes(payload)
        records.append({
            "sourceEntry": entry.path,
            "path": relative,
            "sizeBytes": len(payload),
            "sha256": hashlib.sha256(payload).hexdigest(),
        })
    return {
        "sourceArchive": "Media.pk2",
        "sourceDirectory": TEXT_ROOT.rstrip("/"),
        "fileCount": len(records),
        "totalBytes": sum(row["sizeBytes"] for row in records),
        "files": records,
    }


def _source_path(value: str) -> str | None:
    """Turn an associated client icon name into a contained Media entry."""
    if not value or value.casefold() == "xxx":
        return None
    normalized = value.replace("\\", "/")
    parts = PurePosixPath(normalized).parts
    if (
        PurePosixPath(normalized).is_absolute()
        or not parts
        or any(part in ("", ".", "..") for part in parts)
        or ":" in normalized
    ):
        raise ExportError("unsafe associated asset reference in source table")
    # AssocFileIcon paths are relative to the client media icon root. The exact
    # prefix is checked against the real archive index before a file is read.
    joined = "/".join(parts)
    return (joined if joined.casefold().startswith("icon/") else f"icon/{joined}").casefold()


def _decode_table(payload: bytes, source_label: str) -> tuple[str, str]:
    for encoding in ("utf-16", "cp949", "utf-8-sig"):
        try:
            return payload.decode(encoding, errors="strict"), encoding
        except UnicodeDecodeError:
            continue
    raise ExportError(f"table encoding is unsupported: {source_label}")


def _int_field(value: str, label: str, *, nullable: bool = False) -> int | None:
    if nullable and value.upper() in ("", "NULL", "XXX"):
        return None
    try:
        return int(value, 10)
    except ValueError as exc:
        raise ExportError(f"invalid integer in {label}") from exc


def _lines(payload: bytes, source_label: str) -> tuple[list[str], str]:
    text, encoding = _decode_table(payload, source_label)
    return [line for line in text.splitlines() if line and not line.startswith("//")], encoding


def _catalog(dataset_id: str, family: str, status: str, records: list[dict[str, Any]], **extra: Any) -> dict[str, Any]:
    return {
        "catalogVersion": SCHEMA_VERSION,
        "datasetId": dataset_id,
        "family": family,
        "status": status,
        "records": records,
        **extra,
    }


def _no_symlink_components(path: Path) -> None:
    absolute = path if path.is_absolute() else Path.cwd() / path
    current = Path(absolute.anchor)
    for part in absolute.parts[1:]:
        if part == "..":
            current = current.parent
            continue
        if part in ("", "."):
            continue
        current /= part
        if current.is_symlink():
            raise ExportError(f"symlink output path is not allowed: {current}")


def _item_shards(media, index: dict[str, Entry]) -> list[tuple[str, Entry]]:
    list_path = f"{TEXT_ROOT}itemdata.txt"
    entry = index.get(list_path.casefold())
    if entry is None:
        raise ExportError("item data shard index is missing")
    lines, _ = _lines(media.read_payload(entry), list_path)
    shards: list[tuple[str, Entry]] = []
    for line in lines:
        name = line.strip()
        if not ITEM_SHARD.fullmatch(name):
            raise ExportError("item shard index contains an unsupported filename")
        relative = f"{TEXT_ROOT}{name}"
        shard = index.get(relative.casefold())
        if shard is None:
            raise ExportError(f"item data shard referenced by the source is missing: {name}")
        shards.append((relative, shard))
    return shards


def _magic_option_table(index: dict[str, Entry]) -> tuple[str | None, Entry | None]:
    candidates = [
        (path, entry)
        for path, entry in index.items()
        if path.startswith("server_dep/silkroad/")
        and path.rsplit("/", 1)[-1] == "magicoption.txt"
    ]
    if len(candidates) != 1:
        return None, None
    return candidates[0]


def _skill_shards(media, index: dict[str, Entry]) -> list[tuple[str, Entry]]:
    list_path = f"{TEXT_ROOT}skilldata.txt"
    entry = index.get(list_path.casefold())
    if entry is None:
        raise ExportError("skill data shard index is missing")
    lines, _ = _lines(media.read_payload(entry), list_path)
    shards: list[tuple[str, Entry]] = []
    for line in lines:
        name = line.strip()
        if not SKILL_SHARD.fullmatch(name):
            raise ExportError("skill shard index contains an unsupported filename")
        relative = f"{TEXT_ROOT}{name}"
        shard = index.get(relative.casefold())
        if shard is None:
            raise ExportError(f"skill data shard referenced by the source is missing: {name}")
        shards.append((relative, shard))
    return shards


def _resolved_text(table: dict[str, list[str]], key: str) -> str | None:
    values = sorted({value.strip() for value in table.get(key, []) if value.strip()})
    return values[0] if len(values) == 1 else None


def _candidate_identity(dataset_id: str, family: str, entry: Entry) -> tuple[str, str]:
    content_identity = hashlib.sha256(entry.path.casefold().encode("utf-8")).hexdigest()[:16]
    return (
        f"{family}-candidate:{dataset_id}:{content_identity}",
        f"{family}-art:{dataset_id}:{content_identity}",
    )


def _interface_symbol_group(path: str) -> str | None:
    for group, pattern in INTERFACE_SYMBOL_PATTERNS:
        if pattern.fullmatch(path):
            return group
    return None


def _portrait_details(path: str) -> tuple[str, str, int] | None:
    match = PORTRAIT_CANDIDATE.fullmatch(path)
    if match is None:
        return None
    race = "Chinese" if match.group("race").casefold() == "ch" else "European"
    gender = "male" if match.group("gender").casefold() == "man" else "female"
    return race, gender, int(match.group("ordinal"))


def _localized_rows(
    media,
    index: dict[str, Entry],
    relative: str,
    *,
    value_columns: tuple[int, ...] = (8,),
) -> tuple[dict[str, list[str]], dict[str, Any]]:
    entry = index.get(relative.casefold())
    if entry is None:
        return {}, {"status": "missing", "rowCount": 0, "encoding": None}
    lines, encoding = _lines(media.read_payload(entry), relative)
    result: dict[str, list[str]] = defaultdict(list)
    malformed = 0
    for line in lines:
        fields = line.split("\t")
        if len(fields) <= max(value_columns):
            malformed += 1
            continue
        key = fields[1]
        # Candidate display columns are selected only after exact key joins and
        # values that disagree remain unresolved instead of being guessed.
        if key:
            for column in value_columns:
                value = fields[column].strip()
                if value:
                    result[key].append(value)
    conflicts = sum(1 for values in result.values() if len(set(values)) > 1)
    return dict(result), {
        "status": "parsed" if not malformed and not conflicts else "partial",
        "rowCount": len(lines),
        "encoding": encoding,
        "valueColumnsChecked": [column + 1 for column in value_columns],
        "malformedRows": malformed,
        "conflictingKeys": conflicts,
    }


def _write_asset(
    *,
    staging_bundle: Path,
    category: str,
    entry: Entry,
    archive,
    semantic_key: str,
    assets_by_hash: dict[str, dict[str, Any]],
    converted_by_source: dict[str, tuple[bytes, int, int, str, str, str]],
    source_audit: list[dict[str, Any]],
    public_alias: str | None = None,
) -> tuple[str, int, int]:
    if len(entry.path) > 512:
        raise ExportError("source asset path is too long")
    use_source_cache = category != "maps" and (entry.path.casefold() in converted_by_source or len(converted_by_source) < 1024)
    converted = converted_by_source.get(entry.path.casefold()) if use_source_cache else None
    if converted is None:
        try:
            image = ddj_to_png(archive.read_payload(entry))
        except TextureError as exc:
            source_audit.append({"assetKey": semantic_key, "sourceEntry": entry.path, "status": "unsupported", "reason": str(exc)})
            raise ExportError(f"referenced image cannot be converted ({entry.path}): {exc}") from exc
        converted = (image.data, image.width, image.height, image.wrapper_field_status, image.wrapper_field_hex, image.embedded_format)
        if use_source_cache:
            converted_by_source[entry.path.casefold()] = converted
    png_data, width, height, wrapper_field_status, wrapper_field_hex, embedded_format = converted
    digest = hashlib.sha256(png_data).hexdigest()
    extension = "maps" if category == "maps" else "images"
    relative = f"assets/{extension}/{digest}.png"
    path = staging_bundle / relative
    if digest not in assets_by_hash:
        path.parent.mkdir(parents=True, exist_ok=True)
        path.write_bytes(png_data)
        assets_by_hash[digest] = {
            "path": relative,
            "sha256": digest,
            "mediaType": "image/png",
            "sizeBytes": len(png_data),
            "width": width,
            "height": height,
            "semanticKeys": [],
        }
    assets_by_hash[digest]["semanticKeys"].append(semantic_key)
    source_audit.append({
        "assetKey": semantic_key,
        "sourceEntry": entry.path,
        "status": "converted",
        "sha256": digest,
        "wrapperFieldStatus": wrapper_field_status,
        "wrapperFieldHex": wrapper_field_hex,
        "embeddedFormat": embedded_format,
        **({"publicAlias": public_alias} if public_alias else {}),
    })
    return digest, width, height


def _map_raster_content_status(path: Path) -> str:
    """Report an exact opaque-black raster without guessing its intended geography."""
    with Image.open(path) as image:
        if image.format != "PNG":
            raise ExportError("exported map raster is not a PNG")
        extrema = image.convert("RGBA").getextrema()
    if extrema == ((0, 0), (0, 0), (0, 0), (255, 255)):
        return "uniform-opaque-black"
    return "has-nonblack-pixels"


def _publish(staging: Path, destination: Path, lock: Path) -> bool:
    reused = False
    try:
        if destination.exists():
            if not destination.is_dir() or destination.is_symlink():
                raise ExportError("dataset output path already exists and is not a safe directory")
            for directory in ("bundle", "textdata"):
                prior_tree = destination / directory
                new_tree = staging / directory
                _no_symlink_components(prior_tree)
                if not prior_tree.is_dir():
                    raise ExportError(f"dataset identity matches existing output but {directory} is missing")
                prior_paths = list(prior_tree.rglob("*"))
                if any(path.is_symlink() for path in prior_paths):
                    raise ExportError(f"symlink in existing {directory} output")
                prior_files = sorted(path.relative_to(prior_tree).as_posix() for path in prior_paths if path.is_file())
                new_files = sorted(path.relative_to(new_tree).as_posix() for path in new_tree.rglob("*") if path.is_file())
                if prior_files != new_files or any(
                    sha256_file(prior_tree / relative) != sha256_file(new_tree / relative)
                    for relative in prior_files
                ):
                    raise ExportError(f"dataset identity matches existing output but {directory} bytes differ")
            shutil.rmtree(staging)
            reused = True
            return reused
        last_error = None
        for attempt in range(6):
            try:
                os.rename(staging, destination)
                return reused
            except PermissionError as exc:
                last_error = exc
                if destination.exists():
                    raise ExportError("dataset destination appeared during atomic publication") from exc
                time.sleep(0.2 * (attempt + 1))
        assert last_error is not None
        raise last_error
    finally:
        lock.unlink(missing_ok=True)


def export_dataset(source: Path, output: Path, key: str, asset_output: Path | None = None, *, monster_models: list[str] | None = None, unique_monsters: bool = False) -> dict[str, Any]:
    if monster_models is not None:
        monster_models = sorted(set(name.casefold() for name in monster_models))
        if not monster_models or any(not MODEL_NAME.fullmatch(name) for name in monster_models):
            raise ExportError("monster model selection contains an unsafe or empty model name")
    source = source.resolve(strict=True)
    if not source.is_dir():
        raise ExportError(f"source directory does not exist: {source}")
    _no_symlink_components(output)
    output = output.resolve()
    if output == source or source in output.parents:
        raise ExportError("export output must be outside the source archives directory")
    if asset_output is not None:
        from .public_assets import ensure_public_asset_destination

        _no_symlink_components(asset_output)
        asset_output = asset_output.resolve()
        if any(
            left == right or left in right.parents or right in left.parents
            for left, right in ((asset_output, source), (asset_output, output))
        ):
            raise ExportError("public asset output must be separate from source archives and exporter output")
        ensure_public_asset_destination(asset_output)
    output.mkdir(parents=True, exist_ok=True)

    started = time.perf_counter()
    source_paths = {role: source / filename for role, filename, _ in ARCHIVES}
    for role in ("media", "maps"):
        if not source_paths[role].is_file():
            raise ExportError(f"required source archive is missing: {source_paths[role].name}")

    # Catalogues/UI/minimap rasters come from Media.pk2. Monster resources come
    # from Data.pk2 when present. Map.pk2 is identity-checked; terrain is not rendered.
    media_path = source_paths["media"]
    maps_path = source_paths["maps"]
    source_roles = ["media", "maps"] + (["data"] if source_paths["data"].is_file() else [])
    initial_stats = {role: (source_paths[role].stat().st_size, source_paths[role].stat().st_mtime_ns) for role in source_roles}
    source_hashes = {role: sha256_file(source_paths[role]) for role in source_roles}
    dataset_hash = hashlib.sha256(
        json.dumps(
            {"schema": SCHEMA_VERSION, "exporter": __version__, "sourceHashes": source_hashes, "monsterModels": monster_models, "uniqueMonsters": unique_monsters},
            sort_keys=True,
            separators=(",", ":"),
        ).encode("ascii")
    ).hexdigest()
    dataset_id = f"gamedata-{dataset_hash[:20]}"
    dataset_directory = output / dataset_id
    lock = output / f".{dataset_id}.lock"
    try:
        descriptor = os.open(lock, os.O_CREAT | os.O_EXCL | os.O_WRONLY, 0o600)
    except FileExistsError as exc:
        raise ExportError("another export is already writing this dataset") from exc
    os.close(descriptor)
    staging = output / f".{dataset_id}.staging-{os.getpid()}-{time.time_ns()}"
    bundle = staging / "bundle"
    audit = staging / "audit"
    try:
        bundle.mkdir(parents=True)
        audit.mkdir(parents=True)
        audit_sources: dict[str, Any] = {
            "auditVersion": 1,
            "exporterVersion": __version__,
            "createdAtUtc": datetime.now(UTC).isoformat(),
            "sourceRoot": str(source),
            "selectedArchives": [],
            "excludedArchives": [
                {"file": "Map - copia.pk2", "reason": "operator designated Map.pk2 as source and this copy as backup"},
                {"file": "Music.pk2", "reason": EXCLUDED_BY_OPERATOR["sounds"]},
                {"file": "Particles.pk2", "reason": "visual effects are outside the bounded asset families selected for this export"},
            ],
            "keyMaterialRecorded": False,
            "datasetId": dataset_id,
        }
        for role, filename, description in ARCHIVES:
            if role in source_roles:
                audit_sources["selectedArchives"].append({
                    "role": role,
                    "description": description,
                    "sourcePath": str(source_paths[role]),
                    "sizeBytes": source_paths[role].stat().st_size,
                    "sha256": source_hashes[role],
                })

        with ExitStack() as source_stack:
            media = source_stack.enter_context(PK2Archive(media_path, key=key))
            map_archive = source_stack.enter_context(PK2Archive(maps_path, key=key))
            data_archive = source_stack.enter_context(PK2Archive(source_paths["data"], key=key)) if "data" in source_roles else None
            media_info = media.inventory()
            map_info = map_archive.inventory()
            media_index = _archive_index(media_info.entries)
            textdata_report = _extract_textdata(media, media_index, staging / "textdata")
            _json_write(audit / "textdata.json", textdata_report)
            portrait_entries = {
                entry.path.casefold(): entry
                for entry in media_info.entries
                if entry.kind == 2 and _portrait_details(entry.path)
            }
            asset_refs: list[dict[str, Any]] = []
            assets_by_hash: dict[str, dict[str, Any]] = {}
            converted_by_source: dict[str, tuple[bytes, int, int, str, str, str]] = {}
            unresolved: list[dict[str, Any]] = []

            item_text, item_text_audit = _localized_rows(media, media_index, f"{TEXT_ROOT}textdata_equip&skill.txt")
            skill_text, skill_text_audit = _localized_rows(media, media_index, f"{TEXT_ROOT}textdata_equip&skill.txt", value_columns=(8, 9))
            object_text, object_text_audit = _localized_rows(media, media_index, f"{TEXT_ROOT}textdata_object.txt")
            item_records: list[dict[str, Any]] = []
            item_audit: list[dict[str, Any]] = []
            missing_item_names = missing_item_descriptions = missing_item_icons = 0
            seen_item_ids: set[int] = set()
            for shard_ordinal, (table_path, shard) in enumerate(_item_shards(media, media_index), start=1):
                lines, encoding = _lines(media.read_payload(shard), table_path)
                shard_audit: dict[str, Any] = {"sourceTable": table_path, "encoding": encoding, "rows": len(lines), "normalized": 0}
                for line_number, line in enumerate(lines, start=1):
                    fields = line.split("\t")
                    if len(fields) < ITEM_COMMON_FIELDS:
                        raise ExportError(f"item row has too few fields at {table_path}:{line_number}")
                    ref_id = _int_field(fields[1], "item reference ID")
                    assert ref_id is not None
                    identity = ref_id
                    if identity in seen_item_ids:
                        raise ExportError(f"duplicate item reference ID {identity}")
                    seen_item_ids.add(identity)
                    code_name = fields[2]
                    name_key = fields[5]
                    desc_key = fields[6]
                    candidates = item_text.get(name_key, [])
                    names = sorted(set(candidates))
                    display_name = names[0] if len(names) == 1 else None
                    display_description = _resolved_text(item_text, desc_key)
                    if display_name is None:
                        missing_item_names += 1
                    if display_description is None:
                        missing_item_descriptions += 1
                    record_id = f"item:{dataset_id}:{ref_id}"
                    asset_key = None
                    associated = _source_path(fields[54])
                    if associated is not None:
                        source_entry = media_index.get(associated)
                        if source_entry is None:
                            missing_item_icons += 1
                            item_audit.append({"recordId": record_id, "table": table_path, "row": line_number, "assetSourcePath": associated, "assetStatus": "missing"})
                        else:
                            asset_key = f"item-icon:{dataset_id}:{ref_id}"
                            _write_asset(
                                staging_bundle=bundle,
                                category="images",
                                entry=source_entry,
                                archive=media,
                                semantic_key=asset_key,
                                assets_by_hash=assets_by_hash,
                                converted_by_source=converted_by_source,
                                source_audit=asset_refs,
                            )
                            item_audit.append({"recordId": record_id, "table": table_path, "row": line_number, "assetSourcePath": associated, "assetStatus": "converted"})
                    item_records.append({
                        "id": record_id,
                        "referenceId": ref_id,
                        "code": code_name,
                        "name": {"en": display_name} if display_name is not None else {},
                        "description": {"en": display_description} if display_description else {},
                        "assetKey": asset_key,
                        "presentation": item_metadata(fields),
                    })
                    shard_audit["normalized"] += 1
                shard_audit["sourceRows"] = len(lines)
                shard_audit.pop("sourceTable")
                shard_audit["sourcePath"] = table_path
                _json_write(audit / "tables" / f"items-shard-{shard_ordinal:04d}.json", shard_audit)

            item_records.sort(key=lambda row: row["referenceId"])
            _json_write(bundle / "catalogs" / "items.json", _catalog(dataset_id, "items", "partial", item_records, recordCount=len(item_records), locales=["en"]))
            magic_path, magic_entry = _magic_option_table(media_index)
            if magic_entry is None:
                magic_records = []
                magic_audit = {
                    "status": "missing_or_ambiguous_source",
                    "rowCount": 0,
                    "recordCount": 0,
                    "labelsResolved": 0,
                    "valueScaleStatus": "unresolved",
                }
                magic_encoding = None
            else:
                magic_lines, magic_encoding = _lines(media.read_payload(magic_entry), magic_path or "magicoption.txt")
                magic_records, magic_audit = magic_option_definitions(magic_lines, object_text)
            _json_write(
                bundle / "catalogs" / "magicOptions.json",
                _catalog(
                    dataset_id,
                    "magicOptions",
                    "partial" if magic_records else "unresolved",
                    magic_records,
                    recordCount=len(magic_records),
                    locales=["en"],
                    valueScaleStatus="unresolved",
                ),
            )
            magic_audit["encoding"] = magic_encoding
            _json_write(audit / "tables" / "magic-options.json", magic_audit)
            _json_write(bundle / "catalogs" / "localization.json", _catalog(dataset_id, "localization", "partial", [], locales=["en"], storage="resolved display strings are embedded in family catalogs"))
            _json_write(bundle / "catalogs" / "taxonomy.json", _catalog(dataset_id, "taxonomy", "unresolved", [], reason="client category and subcategory semantics are not verified"))
            unresolved.append({"family": "taxonomy", "reason": "item type/category and subcategory mappings have not been independently verified"})

            # Character data provides entity identities and some associated client
            # icons. It does not establish that those icons are portrait/card art.
            entity_records: list[dict[str, Any]] = []
            monster_source_rows: list[list[str]] = []
            entity_audit: list[dict[str, Any]] = []
            missing_entity_names = missing_entity_icons = 0
            verified_portrait_joins = 0
            portrait_models_by_path: dict[str, list[int]] = defaultdict(list)
            entity_list_entry = media_index.get(f"{TEXT_ROOT}characterdata.txt".casefold())
            entity_shards: list[tuple[str, Entry]] = []
            if entity_list_entry is None:
                unresolved.append({"family": "entities", "reason": "character data shard index is missing"})
            else:
                shard_lines, _ = _lines(media.read_payload(entity_list_entry), f"{TEXT_ROOT}characterdata.txt")
                for name in shard_lines:
                    leaf = name.strip()
                    if not ENTITY_SHARD.fullmatch(leaf):
                        raise ExportError("character data shard index contains an unsupported filename")
                    rel = f"{TEXT_ROOT}{leaf}"
                    if rel.casefold() not in media_index:
                        unresolved.append({"family": "entities", "sourceShard": rel, "reason": "listed character data shard is missing"})
                        continue
                    entity_shards.append((rel, media_index[rel.casefold()]))
                entity_ids: set[int] = set()
                for shard_ordinal, (table_path, shard) in enumerate(entity_shards, start=1):
                    lines, encoding = _lines(media.read_payload(shard), table_path)
                    table_audit = {"sourcePath": table_path, "encoding": encoding, "sourceRows": len(lines), "normalized": 0}
                    for line_number, line in enumerate(lines, start=1):
                        fields = line.split("\t")
                        if len(fields) < ENTITY_COMMON_FIELDS:
                            raise ExportError(f"entity row has too few fields at {table_path}:{line_number}")
                        ref_id = _int_field(fields[1], "entity reference ID")
                        assert ref_id is not None
                        identity = ref_id
                        if identity in entity_ids:
                            raise ExportError(f"duplicate entity reference ID {identity}")
                        entity_ids.add(identity)
                        if fields[2].startswith("MOB_"):
                            monster_source_rows.append(fields)
                        token = fields[5]
                        names = sorted(set(object_text.get(token, [])))
                        display_name = names[0] if len(names) == 1 else None
                        if display_name is None:
                            missing_entity_names += 1
                        record_id = f"entity:{dataset_id}:{ref_id}"
                        associated = _source_path(fields[54])
                        portrait_asset_key = None
                        portrait_mapping_status = "unmapped"
                        portrait = portrait_for_model(ref_id)
                        if portrait is not None:
                            race, gender = portrait[:2]
                            race_code = "CH" if race == "ch" else "EU"
                            gender_code = "MAN" if gender == "man" else "WOMAN"
                            expected_code_prefix = f"CHAR_{race_code}_{gender_code}_"
                            portrait_path = portrait_source_path(ref_id)
                            assert portrait_path is not None
                            portrait_path = portrait_path.casefold()
                            portrait_entry = portrait_entries.get(portrait_path)
                            if fields[2].startswith(expected_code_prefix) and portrait_entry is not None:
                                _, portrait_asset_key = _candidate_identity(dataset_id, "portrait", portrait_entry)
                                portrait_mapping_status = "verified-phmonitor-model-v050"
                                portrait_models_by_path[portrait_path].append(ref_id)
                                verified_portrait_joins += 1
                        asset_key = None
                        if associated:
                            source_entry = media_index.get(associated)
                            if source_entry:
                                asset_key = f"entity-associated-icon:{dataset_id}:{ref_id}"
                                _write_asset(
                                    staging_bundle=bundle,
                                    category="images",
                                    entry=source_entry,
                                    archive=media,
                                    semantic_key=asset_key,
                                    assets_by_hash=assets_by_hash,
                                    converted_by_source=converted_by_source,
                                    source_audit=asset_refs,
                                )
                                entity_audit.append({"recordId": record_id, "table": table_path, "row": line_number, "assetStatus": "converted", "assetSourcePath": associated, "portraitMappingStatus": portrait_mapping_status, "portraitAssetKey": portrait_asset_key})
                            else:
                                missing_entity_icons += 1
                                entity_audit.append({"recordId": record_id, "table": table_path, "row": line_number, "assetStatus": "missing", "assetSourcePath": associated, "portraitMappingStatus": portrait_mapping_status, "portraitAssetKey": portrait_asset_key})
                        entity_records.append({
                            "id": record_id,
                            "referenceId": ref_id,
                            "code": fields[2],
                            "name": {"en": display_name} if display_name else {},
                            "description": None,
                            "associatedIconAssetKey": asset_key,
                            "portraitAssetKey": portrait_asset_key,
                            "portraitMappingStatus": portrait_mapping_status,
                            "classification": "unmapped",
                        })
                        table_audit["normalized"] += 1
                    _json_write(audit / "tables" / f"entities-{shard_ordinal:06d}.json", table_audit)
            unresolved.append({"family": "entities", "reason": "pet class and full-body artwork roles remain unresolved; character portraits use only the explicit phMonitor v0.5.0 model mapping"})
            entity_records.sort(key=lambda row: row["referenceId"])
            _json_write(bundle / "catalogs" / "entities.json", _catalog(dataset_id, "entities", "partial", entity_records, recordCount=len(entity_records), locales=["en"]))
            monster_collection_errors = []
            monster_targets = collect_monster_targets(monster_source_rows, uniques_only=unique_monsters,
                                                      invalid=monster_collection_errors)
            data_index = _archive_index(data_archive.inventory().entries) if data_archive is not None and monster_targets else {}
            monster_records, monster_audit, monster_coverage = export_monsters(
                archive=data_archive, index=data_index, targets=monster_targets,
                selected=monster_models, dataset_id=dataset_id, bundle=bundle,
                assets=assets_by_hash, asset_audit=asset_refs,
                collection_errors=monster_collection_errors,
            )
            monster_coverage["uniqueOnly"] = unique_monsters
            monster_failures = monster_coverage["unsupported"] + monster_coverage["invalid"]
            monster_status = "parsed" if monster_records and not monster_failures else "partial" if monster_coverage["rendered"] else "unresolved"
            _json_write(bundle / "catalogs" / "monsters.json", _catalog(dataset_id, "monsters", monster_status, monster_records, recordCount=len(monster_records), coverage=monster_coverage))
            _json_write(audit / "tables" / "monster-renders.json", monster_audit)
            if data_archive is None or monster_failures:
                unresolved.append({"family": "monsters", "reason": "Data.pk2 unavailable" if data_archive is None else f'{monster_coverage["unsupported"]} unsupported models and {monster_coverage["invalid"]} invalid resource joins; see private monster-renders audit'})

            # Skill shards are selected only through the client's plaintext index.
            # The exporter deliberately omits rank rules, costs and prerequisites:
            # those fields have no verified semantics in the inspected schema.
            skill_records: list[dict[str, Any]] = []
            skill_audit: list[dict[str, Any]] = []
            skill_icon_sources: set[str] = set()
            skill_icon_asset_keys: dict[str, str] = {}
            missing_skill_names = missing_skill_descriptions = missing_skill_icons = 0
            skill_index_entry = media_index.get(f"{TEXT_ROOT}skilldata.txt".casefold())
            if skill_index_entry is None:
                _json_write(bundle / "catalogs" / "skills.json", _catalog(dataset_id, "skills", "unresolved", [], recordCount=0))
                _json_write(audit / "tables" / "skill-source-selection.json", {"status": "missing-index", "indexedShards": [], "excludedCandidates": []})
                unresolved.append({"family": "skills", "reason": "the source-linked skill shard index is missing"})
            else:
                skill_ids: set[int] = set()
                skill_codes: set[str] = set()
                indexed_skill_shards = _skill_shards(media, media_index)
                indexed_skill_paths = {path.casefold() for path, _ in indexed_skill_shards}
                unlisted_skill_candidates = sorted(
                    entry.path
                    for entry in media_info.entries
                    if entry.kind == 2
                    and "skilldata" in entry.name.casefold()
                    and entry.name.casefold().endswith(".txt")
                    and entry.path.casefold() not in indexed_skill_paths
                    and entry.path.casefold() != f"{TEXT_ROOT}skilldata.txt".casefold()
                )
                _json_write(audit / "tables" / "skill-source-selection.json", {
                    "indexSourcePath": f"{TEXT_ROOT}skilldata.txt",
                    "indexedShards": [path for path, _ in indexed_skill_shards],
                    "excludedCandidates": [{"sourcePath": path, "reason": "not listed by the plaintext skilldata index; omitted without interpreting encrypted or alternate-table payloads"} for path in unlisted_skill_candidates],
                })
                for shard_ordinal, (table_path, shard) in enumerate(indexed_skill_shards, start=1):
                    lines, encoding = _lines(media.read_payload(shard), table_path)
                    shard_audit: dict[str, Any] = {"sourcePath": table_path, "encoding": encoding, "sourceRows": len(lines), "normalized": 0}
                    for line_number, line in enumerate(lines, start=1):
                        fields = line.split("\t")
                        if len(fields) != 118:
                            raise ExportError(f"skill row has an unexpected field count at {table_path}:{line_number}")
                        ref_id = _int_field(fields[1], "skill reference ID")
                        assert ref_id is not None
                        code = fields[3].strip()
                        if not code or ref_id in skill_ids or code.casefold() in skill_codes:
                            raise ExportError(f"duplicate or empty skill identity at {table_path}:{line_number}")
                        skill_ids.add(ref_id)
                        skill_codes.add(code.casefold())
                        name = _resolved_text(skill_text, fields[62])
                        description = _resolved_text(skill_text, fields[64])
                        missing_skill_names += name is None
                        missing_skill_descriptions += description is None
                        icon_source = _source_path(fields[61])
                        icon_asset_key = None
                        icon_status = "not-referenced"
                        if icon_source:
                            skill_icon_sources.add(icon_source)
                            icon_entry = media_index.get(icon_source)
                            if icon_entry is None:
                                missing_skill_icons += 1
                                icon_status = "missing-source-asset"
                            else:
                                icon_asset_key = skill_icon_asset_keys.get(icon_source)
                                if icon_asset_key is None:
                                    content_identity = hashlib.sha256(icon_source.encode("utf-8")).hexdigest()[:16]
                                    icon_asset_key = f"skill-icon:{dataset_id}:{content_identity}"
                                    _write_asset(staging_bundle=bundle, category="images", entry=icon_entry, archive=media, semantic_key=icon_asset_key, assets_by_hash=assets_by_hash, converted_by_source=converted_by_source, source_audit=asset_refs)
                                    skill_icon_asset_keys[icon_source] = icon_asset_key
                                icon_status = "verified-reference"
                        record_id = f"skill:{dataset_id}:{ref_id}"
                        skill_records.append({
                            "id": record_id,
                            "referenceId": ref_id,
                            "code": code,
                            "name": {"en": name} if name else {},
                            "description": {"en": description} if description else {},
                            "iconAssetKey": icon_asset_key,
                            "iconReferenceStatus": icon_status,
                        })
                        skill_audit.append({"recordId": record_id, "table": table_path, "row": line_number, "iconSourcePath": icon_source, "iconStatus": icon_status})
                        shard_audit["normalized"] += 1
                    _json_write(audit / "tables" / f"skills-shard-{shard_ordinal:04d}.json", shard_audit)
                skill_records.sort(key=lambda row: row["referenceId"])

                skill_art_candidates = []
                all_skill_art = sorted(
                    (entry for entry in media_info.entries if entry.kind == 2 and entry.path.casefold().startswith("icon/skill/") and entry.path.casefold().endswith(".ddj")),
                    key=lambda entry: (entry.path.casefold(), entry.path),
                )
                for entry in all_skill_art:
                    if entry.path.casefold() in skill_icon_sources:
                        continue
                    candidate_id, asset_key = _candidate_identity(dataset_id, "skill", entry)
                    _, width, height = _write_asset(
                        staging_bundle=bundle,
                        category="images",
                        entry=entry,
                        archive=media,
                        semantic_key=asset_key,
                        assets_by_hash=assets_by_hash,
                        converted_by_source=converted_by_source,
                        source_audit=asset_refs,
                    )
                    skill_art_candidates.append({"id": candidate_id, "displayName": f"Unmapped skill art {len(skill_art_candidates) + 1}", "assetKey": asset_key, "width": width, "height": height, "mappingStatus": "unmapped"})
                skill_status = "partial" if missing_skill_names or missing_skill_descriptions or missing_skill_icons else "parsed"
                _json_write(bundle / "catalogs" / "skills.json", _catalog(dataset_id, "skills", skill_status, skill_records, recordCount=len(skill_records), locales=["en"], unmappedArtCandidates=skill_art_candidates, artCandidateCount=len(all_skill_art)))
                if missing_skill_names or missing_skill_descriptions or missing_skill_icons:
                    unresolved.append({"family": "skills", "reason": f"{missing_skill_names} names, {missing_skill_descriptions} descriptions and {missing_skill_icons} referenced icons remain unresolved; ranks, requirements, costs and prerequisites are not normalized"})
                else:
                    unresolved.append({"family": "skills", "reason": "verified identities, localized fields and exact icon joins are exported; rank/requirement/cost/prerequisite semantics remain unresolved"})

            # The mastery table has a plaintext header explicitly naming both icon
            # columns. Other numeric fields remain opaque and are not exported.
            mastery_records: list[dict[str, Any]] = []
            mastery_icon_sources: set[str] = set()
            mastery_path = f"{TEXT_ROOT}skillmasterydata.txt"
            mastery_entry = media_index.get(mastery_path.casefold())
            mastery_data_available = mastery_entry is not None
            missing_mastery_names = missing_mastery_descriptions = missing_mastery_icons = 0
            if mastery_entry is not None:
                mastery_lines, mastery_encoding = _lines(media.read_payload(mastery_entry), mastery_path)
                header_ok = any("Mastery Icon" in row and "Mastery Focus Icon" in row for row in mastery_lines)
                if not header_ok:
                    unresolved.append({"family": "masteries", "reason": "the source header naming mastery icon roles is missing"})
                else:
                    mastery_ids: set[int] = set()
                    for line_number, line in enumerate(mastery_lines, start=1):
                        fields = line.split("\t")
                        if len(fields) != 13 or not fields[0].strip().isdigit():
                            continue
                        ref_id = _int_field(fields[0], "mastery reference ID")
                        assert ref_id is not None
                        if ref_id in mastery_ids:
                            raise ExportError(f"duplicate mastery reference ID {ref_id}")
                        mastery_ids.add(ref_id)
                        name = _resolved_text(skill_text, fields[2].strip())
                        description = _resolved_text(skill_text, fields[4].strip())
                        missing_mastery_names += name is None
                        missing_mastery_descriptions += description is None
                        record_id = f"mastery:{dataset_id}:{ref_id}"
                        icon_keys: dict[str, str | None] = {"iconAssetKey": None, "focusIconAssetKey": None}
                        for column, key_name in ((11, "iconAssetKey"), (12, "focusIconAssetKey")):
                            source_path = _source_path(fields[column])
                            if source_path is None:
                                continue
                            mastery_icon_sources.add(source_path)
                            icon_entry = media_index.get(source_path)
                            if icon_entry is None:
                                missing_mastery_icons += 1
                                continue
                            asset_key = f"mastery-{key_name.removesuffix('AssetKey')}:{dataset_id}:{ref_id}"
                            _write_asset(
                                staging_bundle=bundle,
                                category="images",
                                entry=icon_entry,
                                archive=media,
                                semantic_key=asset_key,
                                assets_by_hash=assets_by_hash,
                                converted_by_source=converted_by_source,
                                source_audit=asset_refs,
                            )
                            icon_keys[key_name] = asset_key
                        mastery_records.append({
                            "id": record_id,
                            "referenceId": ref_id,
                            "name": {"en": name} if name else {},
                            "description": {"en": description} if description else {},
                            **icon_keys,
                            "mappingStatus": "partial" if not name or not any(icon_keys.values()) else "verified-fields",
                        })
                    mastery_records.sort(key=lambda row: row["referenceId"])
                    _json_write(audit / "tables" / "masteries.json", {"sourcePath": mastery_path, "encoding": mastery_encoding, "parsedRecords": len(mastery_records), "namedRecords": len(mastery_records) - missing_mastery_names})
            else:
                unresolved.append({"family": "masteries", "reason": "the mastery data table is missing"})
                _json_write(audit / "tables" / "masteries.json", {"sourcePath": mastery_path, "status": "missing"})

            all_mastery_art = sorted(
                (entry for entry in media_info.entries if entry.kind == 2 and entry.path.casefold().startswith("icon/skillmastery/") and entry.path.casefold().endswith(".ddj")),
                key=lambda entry: (entry.path.casefold(), entry.path),
            )
            mastery_art_candidates = []
            for entry in all_mastery_art:
                if entry.path.casefold() in mastery_icon_sources:
                    continue
                candidate_id, asset_key = _candidate_identity(dataset_id, "mastery", entry)
                _, width, height = _write_asset(
                    staging_bundle=bundle,
                    category="images",
                    entry=entry,
                    archive=media,
                    semantic_key=asset_key,
                    assets_by_hash=assets_by_hash,
                    converted_by_source=converted_by_source,
                    source_audit=asset_refs,
                )
                mastery_art_candidates.append({"id": candidate_id, "displayName": f"Unmapped mastery art {len(mastery_art_candidates) + 1}", "assetKey": asset_key, "width": width, "height": height, "mappingStatus": "unmapped"})
            mastery_status = "unresolved" if not mastery_data_available else ("partial" if missing_mastery_names or missing_mastery_descriptions or missing_mastery_icons else "parsed")
            _json_write(bundle / "catalogs" / "masteries.json", _catalog(dataset_id, "masteries", mastery_status, mastery_records, recordCount=len(mastery_records), locales=["en"], unmappedArtCandidates=mastery_art_candidates, artCandidateCount=len(all_mastery_art)))
            if mastery_data_available:
                unresolved.append({"family": "masteries", "reason": f"{missing_mastery_names} names, {missing_mastery_descriptions} descriptions and {missing_mastery_icons} referenced icons remain unresolved; mastery cap/requirement rules are not normalized"})

            # Skill-group rows join to mastery identities and expose only verified
            # references, group ordering, resolved label, and exact icon reference.
            group_path = f"{TEXT_ROOT}skillgroup.txt"
            group_entry = media_index.get(group_path.casefold())
            group_records: list[dict[str, Any]] = []
            group_icon_missing = group_names_missing = group_mastery_unmatched = 0
            if group_entry is None:
                _json_write(bundle / "catalogs" / "skillGroups.json", _catalog(dataset_id, "skillGroups", "unresolved", [], recordCount=0))
                unresolved.append({"family": "skillGroups", "reason": "the skill-group table is missing"})
            else:
                mastery_ids = {row["referenceId"] for row in mastery_records}
                group_lines, group_encoding = _lines(media.read_payload(group_entry), group_path)
                group_keys: set[tuple[int, int]] = set()
                for line_number, line in enumerate(group_lines, start=1):
                    fields = line.split("\t")
                    if len(fields) != 7:
                        raise ExportError(f"skill-group row has an unexpected field count at row {line_number}")
                    mastery_ref = _int_field(fields[2], "skill-group mastery reference")
                    order = _int_field(fields[4], "skill-group order")
                    assert mastery_ref is not None and order is not None
                    group_identity = (mastery_ref, order)
                    if group_identity in group_keys:
                        raise ExportError(f"duplicate skill-group identity at row {line_number}")
                    group_keys.add(group_identity)
                    name = _resolved_text(skill_text, fields[5].strip())
                    group_names_missing += name is None
                    source_path = _source_path(fields[6])
                    asset_key = None
                    if source_path:
                        group_icon_entry = media_index.get(source_path)
                        if group_icon_entry is None:
                            group_icon_missing += 1
                        else:
                            asset_key = f"skill-group-icon:{dataset_id}:{mastery_ref}:{order}"
                            _write_asset(staging_bundle=bundle, category="images", entry=group_icon_entry, archive=media, semantic_key=asset_key, assets_by_hash=assets_by_hash, converted_by_source=converted_by_source, source_audit=asset_refs)
                    mastery_joined = mastery_ref in mastery_ids
                    group_mastery_unmatched += not mastery_joined
                    group_records.append({
                        "id": f"skill-group:{dataset_id}:{mastery_ref}:{order}",
                        "masteryId": f"mastery:{dataset_id}:{mastery_ref}" if mastery_joined else None,
                        "masteryReferenceId": mastery_ref if not mastery_joined else None,
                        "sortOrder": order,
                        "name": {"en": name} if name else {},
                        "assetKey": asset_key,
                        "masteryJoinStatus": "verified" if mastery_joined else "unresolved",
                    })
                group_records.sort(key=lambda row: ((row["masteryId"] or ""), row["sortOrder"]))
                _json_write(audit / "tables" / "skill-groups.json", {"sourcePath": group_path, "encoding": group_encoding, "sourceRows": len(group_lines), "parsedRecords": len(group_records), "localizedNames": len(group_records) - group_names_missing, "missingIcons": group_icon_missing, "unmatchedMasteryReferences": group_mastery_unmatched})
                group_status = "partial" if group_names_missing or group_icon_missing or group_mastery_unmatched else "parsed"
                _json_write(bundle / "catalogs" / "skillGroups.json", _catalog(dataset_id, "skillGroups", group_status, group_records, recordCount=len(group_records), locales=["en"]))
                if group_names_missing or group_icon_missing or group_mastery_unmatched:
                    unresolved.append({"family": "skillGroups", "reason": f"{group_names_missing} labels, {group_icon_missing} icons and {group_mastery_unmatched} mastery joins remain unresolved"})

            # Portrait model ranges are verified against phMonitor v0.5.0's
            # charPortraitAssetByModel implementation; unrelated entities and pet
            # roles remain unmapped.
            portrait_records = []
            for entry in sorted(portrait_entries.values(), key=lambda entry: (entry.path.casefold(), entry.path)):
                race, gender, ordinal = _portrait_details(entry.path) or ("", "", 0)
                candidate_id, asset_key = _candidate_identity(dataset_id, "portrait", entry)
                _, width, height = _write_asset(staging_bundle=bundle, category="images", entry=entry, archive=media, semantic_key=asset_key, assets_by_hash=assets_by_hash, converted_by_source=converted_by_source, source_audit=asset_refs)
                model_ids = sorted(portrait_models_by_path.get(entry.path.casefold(), []))
                portrait_records.append({"id": candidate_id, "displayName": f"{race} {gender} portrait candidate {ordinal}", "raceLabel": race, "genderLabel": gender, "candidateNumber": ordinal, "assetKey": asset_key, "width": width, "height": height, "mappingStatus": "verified-character-model" if model_ids else "unmapped-candidate", "mappedModelIds": model_ids})
            _json_write(bundle / "catalogs" / "portraits.json", _catalog(dataset_id, "portraits", "partial" if portrait_records else "unresolved", portrait_records, recordCount=len(portrait_records), entityRoleMappingStatus="verified-character-models" if verified_portrait_joins else "unresolved", mappedModelCount=verified_portrait_joins))
            if not verified_portrait_joins:
                unresolved.append({"family": "portraits", "reason": "no character entity rows matched the explicit phMonitor v0.5.0 model mapping"})
            _json_write(audit / "tables" / "portrait-model-joins.json", {
                "mappingSource": "phMonitor v0.5.0 charPortraitAssetByModel static analysis",
                "mappingStatus": "verified-character-models" if verified_portrait_joins else "unresolved",
                "modelRanges": [
                    {"race": race, "gender": gender, "minimum": minimum, "maximum": maximum}
                    for race, gender, minimum, maximum in PORTRAIT_MODEL_RANGES
                ],
                "joinedModels": sorted({model for models in portrait_models_by_path.values() for model in models}),
                "joinCount": verified_portrait_joins,
            })

            # Monster type icons are included on every normal export. Party
            # variants share the observed badge rather than invented rank art.
            monster_icon_records = []
            missing_monster_icons = []
            for type_code, name, source_path, role, rank_code in MONSTER_ICONS:
                entry = media_index.get(source_path)
                asset_key = None
                width = height = None
                public_alias = f"monster-types/{type_code}_{name}.png"
                if entry is None:
                    missing_monster_icons.append(type_code)
                else:
                    asset_key = f"monster-type-icon:{dataset_id}:{type_code}"
                    _, width, height = _write_asset(
                        staging_bundle=bundle, category="images", entry=entry,
                        archive=media, semantic_key=asset_key,
                        assets_by_hash=assets_by_hash,
                        converted_by_source=converted_by_source,
                        source_audit=asset_refs, public_alias=public_alias,
                    )
                monster_icon_records.append({
                    "id": f"monster-type:{dataset_id}:{type_code}",
                    "typeCode": type_code, "name": name, "role": role,
                    "rankTypeCode": rank_code, "assetKey": asset_key,
                    "publicAlias": public_alias if asset_key else None,
                    "width": width, "height": height,
                    "assetReferenceStatus": "verified-reference" if asset_key else "missing-source-asset",
                })
            monster_icon_count = sum(row["assetKey"] is not None for row in monster_icon_records)
            monster_icon_status = "parsed" if not missing_monster_icons else ("partial" if monster_icon_count else "unresolved")
            _json_write(bundle / "catalogs" / "monsterTypes.json", _catalog(
                dataset_id, "monsterTypes", monster_icon_status, monster_icon_records,
                recordCount=len(monster_icon_records), iconCount=monster_icon_count,
                sharedPartyBadgeTypeCodes=[16, 17, 20],
            ))
            if missing_monster_icons:
                unresolved.append({"family": "monsterTypes", "reason": "required monster textures are missing", "typeCodes": missing_monster_icons})

            # Small, curated non-control symbol families. Pressed/focused/button
            # states and complete control atlases are excluded by filename allowlists.
            interface_records = []
            for entry in sorted((entry for entry in media_info.entries if entry.kind == 2 and entry.path.casefold().endswith(".ddj") and _interface_symbol_group(entry.path)), key=lambda entry: (entry.path.casefold(), entry.path)):
                symbol_group = _interface_symbol_group(entry.path)
                assert symbol_group is not None
                code = PurePosixPath(entry.path).stem
                candidate_id, asset_key = _candidate_identity(dataset_id, "ui-symbol", entry)
                _, width, height = _write_asset(staging_bundle=bundle, category="images", entry=entry, archive=media, semantic_key=asset_key, assets_by_hash=assets_by_hash, converted_by_source=converted_by_source, source_audit=asset_refs)
                interface_records.append({"id": candidate_id, "code": code, "symbolGroup": symbol_group, "assetKey": asset_key, "width": width, "height": height, "mappingStatus": "candidate"})
            _json_write(bundle / "catalogs" / "interfaceSymbols.json", _catalog(dataset_id, "interfaceSymbols", "partial" if interface_records else "unresolved", interface_records, recordCount=len(interface_records), buttonStatesIncluded=False))
            _json_write(bundle / "catalogs" / "ui.json", _catalog(dataset_id, "ui", "partial" if interface_records else "unresolved", [], interfaceSymbolCatalogPath="catalogs/interfaceSymbols.json", symbolCandidateCount=len(interface_records), controlsIncluded=False))

            # Attack/Fellow/Pick/Transport and pet-body assignments have no proven
            # reference joins in this source; pet item art remains in item catalogs.
            _json_write(bundle / "catalogs" / "pets.json", _catalog(dataset_id, "pets", "unresolved", [], recordCount=0, roleNames=["Attack", "Fellow", "Pick", "Transport"], roleMappingStatus="unresolved", petBodyArtStatus="unresolved"))
            unresolved.append({"family": "pets", "reason": "the inspected image families contain pet items/skills, but no verified pet-body and Attack/Fellow/Pick/Transport role mapping"})

            # All direct minimap tiles are in Media.pk2. Their coordinates are the
            # tile filenames only; world transforms and dungeon relationships stay unknown.
            tile_entries: list[tuple[str, int, int, Entry]] = []
            for entry in media_info.entries:
                if entry.kind != 2 or not entry.path.casefold().startswith("minimap/") or not entry.path.casefold().endswith(".ddj"):
                    continue
                match = TILE_FILE.fullmatch(entry.name)
                if match is None:
                    unresolved.append({"family": "maps", "sourceEntry": entry.path, "reason": "non-grid minimap image name could not be assigned a grid coordinate"})
                    continue
                prefix = entry.path[: -len(entry.name)].strip("/")
                relative_group = prefix[len("minimap/") :].strip("/")
                tile_set_source = relative_group.casefold()
                tile_entries.append((tile_set_source, int(match.group("x")), int(match.group("y")), entry))
            group_names = sorted({group for group, _, _, _ in tile_entries})
            group_ids = {group: f"tile-set-{index:03d}" for index, group in enumerate(group_names, start=1)}
            tile_records: list[dict[str, Any]] = []
            map_raster_status_by_digest: dict[str, str] = {}
            for group, x, y, entry in sorted(tile_entries, key=lambda r: (group_ids[r[0]], r[2], r[1], r[3].path.casefold())):
                tile_set_id = group_ids[group]
                asset_key = f"map-tile:{dataset_id}:{tile_set_id}:{x}:{y}"
                digest, width, height = _write_asset(
                    staging_bundle=bundle,
                    category="maps",
                    entry=entry,
                    archive=media,
                    semantic_key=asset_key,
                    assets_by_hash=assets_by_hash,
                    converted_by_source=converted_by_source,
                    source_audit=asset_refs,
                )
                if digest not in map_raster_status_by_digest:
                    asset_path = bundle / assets_by_hash[digest]["path"]
                    map_raster_status_by_digest[digest] = _map_raster_content_status(asset_path)
                tile_records.append({
                    "id": f"map-tile-record:{dataset_id}:{tile_set_id}:{x}:{y}",
                    "tileSetId": tile_set_id,
                    "x": x,
                    "y": y,
                    "assetKey": asset_key,
                    "width": width,
                    "height": height,
                    "rasterContentStatus": map_raster_status_by_digest[digest],
                })
            uniform_black_tiles = [row for row in tile_records if row["rasterContentStatus"] == "uniform-opaque-black"]
            if uniform_black_tiles:
                unresolved.append({
                    "family": "maps",
                    "reason": "minimap raster is uniform opaque black; terrain content is unavailable from this raster",
                    "tileRecords": [
                        {"tileSetId": row["tileSetId"], "x": row["x"], "y": row["y"]}
                        for row in uniform_black_tiles
                    ],
                    "recordCount": len(uniform_black_tiles),
                })
            root_tile_set_id = group_ids.get("")
            root_tile_assets: dict[tuple[int, int], str] = {}
            root_tile_raster_status: dict[tuple[int, int], str] = {}
            for tile in tile_records:
                if tile["tileSetId"] != root_tile_set_id:
                    continue
                coordinate = (tile["x"], tile["y"])
                if coordinate in root_tile_assets:
                    raise ExportError(f"duplicate root minimap tile coordinate: {coordinate}")
                root_tile_assets[coordinate] = tile["assetKey"]
                root_tile_raster_status[coordinate] = tile["rasterContentStatus"]
            tile_asset_paths_by_key = {
                semantic_key: bundle / asset["path"]
                for asset in assets_by_hash.values()
                for semantic_key in asset["semanticKeys"]
                if semantic_key.startswith(f"map-tile:{dataset_id}:")
            }
            tile_set_orientations = []
            for tile_set_id in sorted(group_ids.values()):
                set_records = [tile for tile in tile_records if tile["tileSetId"] == tile_set_id]
                tile_set_orientations.append({
                    "tileSetId": tile_set_id,
                    **infer_tile_grid_orientation(set_records, tile_asset_paths_by_key),
                })
            _json_write(bundle / "catalogs" / "maps.json", _catalog(
                dataset_id,
                "maps",
                "partial" if any(row.get("family") == "maps" for row in unresolved) else "parsed",
                tile_records,
                tileSetCount=len(group_ids),
                tileSetOrientations=tile_set_orientations,
                uniformOpaqueBlackTileCount=len(uniform_black_tiles),
                coordinateSemantics="root filename indices join refregion gridX/gridZ on exact pairs only; each tile set reports edge-continuity-supported grid direction when evidence meets its threshold; in-tile world coordinates, marker anchors, offsets, and cross-set registration remain unvalidated",
            ))

            # Named minimap_d grids are separate floor rasters. Retain their
            # exact filename coordinates; the server profile supplies the
            # observed 2D anchors and region/floor rules independently.
            cave_records: list[dict[str, Any]] = []
            for entry in media_info.entries:
                if entry.kind != 2 or not entry.path.casefold().startswith("minimap_d/"):
                    continue
                match = CAVE_TILE_FILE.fullmatch(entry.name)
                if match is None:
                    continue
                prefix = match.group("prefix").casefold()
                x, y = int(match.group("x")), int(match.group("y"))
                asset_key = f"cave-tile:{dataset_id}:{prefix}:{x}:{y}"
                _, width, height = _write_asset(
                    staging_bundle=bundle, category="maps", entry=entry,
                    archive=media, semantic_key=asset_key,
                    assets_by_hash=assets_by_hash,
                    converted_by_source=converted_by_source,
                    source_audit=asset_refs,
                )
                cave_records.append({"id": asset_key, "floorPrefix": prefix,
                                     "x": x, "y": y, "assetKey": asset_key,
                                     "width": width, "height": height})
            _json_write(bundle / "catalogs" / "caveMaps.json", _catalog(
                dataset_id, "caveMaps", "parsed" if cave_records else "unresolved",
                sorted(cave_records, key=lambda row: (row["floorPrefix"], row["y"], row["x"])),
                floorCount=len({row["floorPrefix"] for row in cave_records}),
                coordinateSemantics="named floor tile indices only; 2D anchors and region/Z floor rules are supplied by the versioned server map profile",
            ))

            # Regions have an independently documented 21-column text schema.
            region_path = f"{TEXT_ROOT}refregion.txt"
            region_entry = media_index.get(region_path.casefold())
            region_records: list[dict[str, Any]] = []
            region_encoding = None
            if region_entry is None:
                unresolved.append({"family": "regions", "reason": "refregion source table is missing"})
            else:
                lines, region_encoding = _lines(media.read_payload(region_entry), region_path)
                region_ids: set[int] = set()
                for line_number, line in enumerate(lines, start=1):
                    fields = line.split("\t")
                    if len(fields) != 21:
                        raise ExportError(f"region row has an unexpected number of fields at row {line_number}")
                    region_id = _int_field(fields[0], "region ID")
                    x = _int_field(fields[1], "region grid X")
                    z = _int_field(fields[2], "region grid Z")
                    assert region_id is not None and x is not None and z is not None
                    if region_id in region_ids:
                        raise ExportError(f"duplicate region ID {region_id}")
                    region_ids.add(region_id)
                    links = [_int_field(value, "region linked ID", nullable=True) for value in fields[11:21]]
                    map_asset_key = root_tile_assets.get((x, z))
                    region_records.append({
                        "id": f"region:{dataset_id}:{region_id}",
                        "referenceId": region_id,
                        "gridX": x,
                        "gridZ": z,
                        "continentCode": fields[3],
                        "areaName": fields[4] if fields[4] not in ("", "NULL", "xxx") else None,
                        "battlefieldCode": _int_field(fields[5], "region battlefield code"),
                        "climateCode": _int_field(fields[6], "region climate code"),
                        "linkedRegionIds": links,
                        "mapAssetKeys": [map_asset_key] if map_asset_key is not None else [],
                        "mapTileIndexStatus": "exact-grid-match" if map_asset_key is not None else "no-exact-root-tile",
                        "mapTileContentStatus": root_tile_raster_status.get((x, z)),
                        "worldTransformStatus": "unvalidated",
                    })
            region_records.sort(key=lambda row: row["referenceId"])
            matched_region_tile_count = sum(bool(row["mapAssetKeys"]) for row in region_records)
            missing_region_tile_rows = [row for row in region_records if not row["mapAssetKeys"]]
            if missing_region_tile_rows:
                unresolved.append({
                    "family": "maps",
                    "reason": "region records without an exact root minimap grid tile",
                    "regionIds": [row["referenceId"] for row in missing_region_tile_rows],
                    "recordCount": len(missing_region_tile_rows),
                })
            _json_write(bundle / "catalogs" / "regions.json", _catalog(
                dataset_id,
                "regions",
                "partial",
                region_records,
                recordCount=len(region_records),
                sourceLocale="unknown",
                rootMinimapTileSetId=root_tile_set_id,
                rootMinimapGridJoin={
                    "status": "partial" if missing_region_tile_rows else "complete",
                    "matchedRegionRecords": matched_region_tile_count,
                    "regionRecordsWithoutExactTile": len(missing_region_tile_rows),
                    "matchRule": "exact gridX/gridZ to root minimap x/y only",
                    "worldTransformStatus": "unvalidated",
                },
            ))

            # Teleport IDs/codes/name keys and the region reference are joined only
            # where the numeric region target exists. Connection rows export the
            # two verified endpoint IDs; their other fields are intentionally omitted.
            teleport_path = f"{TEXT_ROOT}teleportdata.txt"
            teleport_entry = media_index.get(teleport_path.casefold())
            teleport_records: list[dict[str, Any]] = []
            teleport_ids: set[int] = set()
            missing_teleport_names = missing_teleport_regions = 0
            teleport_lines: list[str] = []
            teleport_encoding = None
            if teleport_entry is None:
                unresolved.append({"family": "teleports", "reason": "the teleport data table is missing"})
            else:
                teleport_lines, teleport_encoding = _lines(media.read_payload(teleport_entry), teleport_path)
                known_region_ids = {row["referenceId"] for row in region_records}
                for line_number, line in enumerate(teleport_lines, start=1):
                    fields = line.split("\t")
                    if len(fields) != 14:
                        raise ExportError(f"teleport row has an unexpected field count at row {line_number}")
                    ref_id = _int_field(fields[1], "teleport reference ID")
                    assert ref_id is not None
                    code = fields[2].strip()
                    if ref_id in teleport_ids or not code:
                        raise ExportError(f"duplicate or empty teleport identity at row {line_number}")
                    teleport_ids.add(ref_id)
                    name = _resolved_text(object_text, fields[4].strip())
                    region_ref = _int_field(fields[5], "teleport region reference")
                    region_joined = region_ref in known_region_ids
                    missing_teleport_names += name is None
                    missing_teleport_regions += not region_joined
                    teleport_records.append({
                        "id": f"teleport:{dataset_id}:{ref_id}",
                        "referenceId": ref_id,
                        "code": code,
                        "name": {"en": name} if name else {},
                        "regionId": f"region:{dataset_id}:{region_ref}" if region_joined else None,
                        "regionJoinStatus": "verified" if region_joined else "unresolved",
                    })
            teleport_records.sort(key=lambda row: row["referenceId"])
            teleport_links: list[dict[str, int]] = []
            teleport_links_path = f"{TEXT_ROOT}teleportlink.txt"
            teleport_links_entry = media_index.get(teleport_links_path.casefold())
            bad_teleport_links = 0
            if teleport_links_entry is None:
                unresolved.append({"family": "teleports", "reason": "the teleport-link table is missing"})
            else:
                link_lines, link_encoding = _lines(media.read_payload(teleport_links_entry), teleport_links_path)
                for line_number, line in enumerate(link_lines, start=1):
                    fields = line.split("\t")
                    if len(fields) != 23:
                        raise ExportError(f"teleport-link row has an unexpected field count at row {line_number}")
                    source_id = _int_field(fields[1], "teleport-link source ID")
                    target_id = _int_field(fields[2], "teleport-link target ID")
                    assert source_id is not None and target_id is not None
                    if source_id not in teleport_ids or target_id not in teleport_ids:
                        bad_teleport_links += 1
                        continue
                    teleport_links.append({"fromTeleportId": source_id, "toTeleportId": target_id})
            _json_write(audit / "tables" / "teleports.json", {
                "teleportDataSourcePath": teleport_path,
                "teleportDataEncoding": teleport_encoding,
                "sourceRows": len(teleport_lines),
                "parsedRecords": len(teleport_records),
                "localizedNames": len(teleport_records) - missing_teleport_names,
                "resolvedRegionReferences": len(teleport_records) - missing_teleport_regions,
                "unresolvedRegionReferences": missing_teleport_regions,
                "teleportLinkSourcePath": teleport_links_path,
                "teleportLinkEncoding": link_encoding if teleport_links_entry is not None else None,
                "parsedLinks": len(teleport_links),
                "unresolvedLinks": bad_teleport_links,
            })
            teleport_status = "partial" if missing_teleport_names or missing_teleport_regions or bad_teleport_links else ("parsed" if teleport_entry else "unresolved")
            _json_write(bundle / "catalogs" / "teleports.json", _catalog(
                dataset_id,
                "teleports",
                teleport_status,
                teleport_records,
                recordCount=len(teleport_records),
                links=teleport_links,
                linkSemantics="verified endpoint relationships only; additional source fields are unresolved",
                locales=["en"],
            ))
            if teleport_entry is not None:
                unresolved.append({"family": "teleports", "reason": f"{missing_teleport_names} names, {missing_teleport_regions} region joins and {bad_teleport_links} link endpoint rows unresolved; coordinates, fees, eligibility and location transforms are omitted"})

            # Curated-in-family: backgrounds are non-control artwork; action/control
            # atlases and sounds are operator-excluded.
            background_entries = sorted(
                (entry for entry in media_info.entries if entry.kind == 2 and entry.path.casefold().startswith("interface/loading/") and entry.path.casefold().endswith(".ddj")),
                key=lambda entry: (entry.path.casefold(), entry.path),
            )
            background_records = []
            for ordinal, entry in enumerate(background_entries, start=1):
                asset_key = f"client-background:{dataset_id}:{ordinal:03d}"
                _, width, height = _write_asset(
                    staging_bundle=bundle,
                    category="images",
                    entry=entry,
                    archive=media,
                    semantic_key=asset_key,
                    assets_by_hash=assets_by_hash,
                    converted_by_source=converted_by_source,
                    source_audit=asset_refs,
                )
                background_records.append({"id": f"background:{dataset_id}:{ordinal:03d}", "assetKey": asset_key, "width": width, "height": height})
            _json_write(bundle / "catalogs" / "backgrounds.json", _catalog(dataset_id, "backgrounds", "parsed", background_records, recordCount=len(background_records)))
            for family, reason in EXCLUDED_BY_OPERATOR.items():
                _json_write(bundle / "catalogs" / f"{family}.json", _catalog(dataset_id, family, "not-in-scope", [], reason=reason, recordCount=0))

            # Static region parsing also keeps the selected Map.pk2 manifest visible
            # in audit. No 3D meshes are copied or presented as ready map imagery.
            map_metadata_count = sum(1 for entry in map_info.entries if entry.kind == 2 and entry.name.casefold() == "tile2d.ifo")

            catalog_paths = sorted(path for path in (bundle / "catalogs").glob("*.json"))
            catalog_rows = []
            for path in catalog_paths:
                relative = path.relative_to(bundle).as_posix()
                catalog = json.loads(path.read_text(encoding="utf-8"))
                row = {"path": relative, "sha256": sha256_file(path), "recordCount": len(catalog["records"])}
                if isinstance(catalog.get("links"), list):
                    row["relationCount"] = len(catalog["links"])
                catalog_rows.append(row)
            asset_rows = sorted(assets_by_hash.values(), key=lambda row: row["path"])
            for item in asset_rows:
                item["semanticKeys"] = sorted(set(item["semanticKeys"]))
            completion_status = "incomplete" if unresolved or missing_item_names or missing_item_icons or missing_entity_names or missing_entity_icons or missing_item_descriptions else "complete"
            manifest = {
                "schemaVersion": SCHEMA_VERSION,
                "datasetId": dataset_id,
                "exporterVersion": __version__,
                "supportedLocales": ["en"],
                "completionStatus": completion_status,
                "sourceKnowledgeRequired": False,
                "catalogs": catalog_rows,
                "assets": asset_rows,
                "coverage": {
                    "items": {"records": len(item_records), "namesResolved": len(item_records) - missing_item_names, "descriptionsMissing": missing_item_descriptions, "iconsMissing": missing_item_icons},
                    "entities": {"records": len(entity_records), "namesResolved": len(entity_records) - missing_entity_names, "iconsMissing": missing_entity_icons, "portraitJoins": verified_portrait_joins},
                    "skills": {"records": len(skill_records), "namesResolved": len(skill_records) - missing_skill_names, "descriptionsResolved": len(skill_records) - missing_skill_descriptions, "recordsWithIcons": sum(row["iconAssetKey"] is not None for row in skill_records), "uniqueReferencedIconAssets": len(skill_icon_asset_keys), "missingReferencedIcons": missing_skill_icons, "unmappedArtCandidates": len(skill_art_candidates) if skill_index_entry else 0},
                    "masteries": {"records": len(mastery_records), "namesResolved": len(mastery_records) - missing_mastery_names, "descriptionsResolved": len(mastery_records) - missing_mastery_descriptions, "recordsWithIcons": sum(row["iconAssetKey"] is not None for row in mastery_records), "missingReferencedIcons": missing_mastery_icons, "unmappedArtCandidates": len(mastery_art_candidates)},
                    "skillGroups": {"records": len(group_records), "namesResolved": len(group_records) - group_names_missing, "missingIcons": group_icon_missing, "unmatchedMasteryReferences": group_mastery_unmatched},
                    "portraits": {"candidateImages": len(portrait_records), "entityMappings": verified_portrait_joins},
                    "pets": {"verifiedRoleMappings": 0, "status": "unresolved"},
                    "interfaceSymbols": {"candidateImages": len(interface_records), "buttonStatesIncluded": False},
                    "monsterTypes": {"icons": monster_icon_count, "missingTypeCodes": missing_monster_icons, "sharedPartyBadgeTypeCodes": [16, 17, 20]},
                    "monsters": monster_coverage,
                    "maps": {"tiles": len(tile_records), "tileSets": len(group_ids), "uniformOpaqueBlackTiles": len(uniform_black_tiles), "gridOrientations": {row["tileSetId"]: row["status"] for row in tile_set_orientations}, "worldTransformsValidated": False},
                    "regions": {"records": len(region_records)},
                    "teleports": {"records": len(teleport_records), "namesResolved": len(teleport_records) - missing_teleport_names, "regionJoinsResolved": len(teleport_records) - missing_teleport_regions, "links": len(teleport_links), "unresolvedLinks": bad_teleport_links},
                    "backgrounds": {"records": len(background_records)},
                    "sounds": {"status": "not-in-scope"},
                    "interfaceControls": {"status": "not-in-scope"},
                },
            }
            _json_write(bundle / "manifest.json", manifest)

            _json_write(audit / "sources.json", audit_sources)
            _json_write(audit / "assets.json", asset_refs)
            _json_write(audit / "records.json", {"items": item_audit, "entities": entity_audit, "skills": skill_audit})
            _json_write(audit / "coverage.json", {
                "reportVersion": 1,
                "datasetId": dataset_id,
                "completionStatus": completion_status,
                "families": {
                    "items": {"discovered": len(item_records), "parsed": len(item_records), "named": len(item_records) - missing_item_names, "convertedIconReferences": sum(r["assetStatus"] == "converted" for r in item_audit), "missingIcons": missing_item_icons, "missingDescriptions": missing_item_descriptions, "nameTextAudit": item_text_audit},
                    "entities": {"discovered": len(entity_records), "parsed": len(entity_records), "named": len(entity_records) - missing_entity_names, "convertedAssociatedIcons": sum(r["assetStatus"] == "converted" for r in entity_audit), "missingIcons": missing_entity_icons, "portraitJoins": verified_portrait_joins, "nameTextAudit": object_text_audit},
                    "skills": {"discovered": len(skill_records), "parsedRecords": len(skill_records), "named": len(skill_records) - missing_skill_names, "descriptionsResolved": len(skill_records) - missing_skill_descriptions, "recordsWithIcons": sum(row["iconAssetKey"] is not None for row in skill_records), "uniqueReferencedIconAssets": len(skill_icon_asset_keys), "missingIconReferences": missing_skill_icons, "unmappedImageCandidates": len(skill_art_candidates) if skill_index_entry else 0, "localizationAudit": skill_text_audit},
                    "masteries": {"discovered": len(mastery_records), "parsedRecords": len(mastery_records), "named": len(mastery_records) - missing_mastery_names, "descriptionsResolved": len(mastery_records) - missing_mastery_descriptions, "recordsWithIcons": sum(row["iconAssetKey"] is not None for row in mastery_records), "missingIconReferences": missing_mastery_icons, "unmappedImageCandidates": len(mastery_art_candidates)},
                    "skillGroups": {"discovered": len(group_records), "parsedRecords": len(group_records), "named": len(group_records) - group_names_missing, "missingIcons": group_icon_missing, "unmatchedMasteryReferences": group_mastery_unmatched},
                    "portraits": {"candidateImagesConverted": len(portrait_records), "entityMappings": verified_portrait_joins},
                    "pets": {"verifiedRoleMappings": 0, "status": "unresolved"},
                    "interfaceSymbols": {"candidateImagesConverted": len(interface_records), "groups": dict(sorted(Counter(row["symbolGroup"] for row in interface_records).items()))},
                    "monsterTypes": {"iconsConverted": monster_icon_count, "missingTypeCodes": missing_monster_icons},
                    "monsters": monster_coverage,
                    "maps": {"minimapTiles": len(tile_records), "tileSets": len(group_ids), "uniformOpaqueBlackTiles": len(uniform_black_tiles), "tileSetOrientations": tile_set_orientations, "MapPk2Tile2dMetadataEntries": map_metadata_count, "rootTileSetId": root_tile_set_id, "regionGridMatchedRecords": matched_region_tile_count, "regionRecordsWithoutExactRootTile": len(missing_region_tile_rows), "worldCoordinateTransforms": "unvalidated"},
                    "regions": {"parsed": len(region_records), "encoding": region_encoding},
                    "teleports": {"parsed": len(teleport_records), "namesResolved": len(teleport_records) - missing_teleport_names, "regionReferencesResolved": len(teleport_records) - missing_teleport_regions, "linksParsed": len(teleport_links), "linksUnresolved": bad_teleport_links, "status": teleport_status},
                    "backgrounds": {"parsedAndConverted": len(background_records)},
                    "sounds": {"status": "not-in-scope by operator instruction"},
                    "interfaceControls": {"status": "not-in-scope by operator instruction"},
                },
                "unresolved": unresolved + ([{"family": "items", "reason": f"{missing_item_names} item names and {missing_item_descriptions} descriptions could not be joined from available localization rows"}] if missing_item_names or missing_item_descriptions else []) + ([{"family": "items", "reason": f"{missing_item_icons} associated item icons are missing"}] if missing_item_icons else []) + ([{"family": "entities", "reason": f"{missing_entity_names} names and {missing_entity_icons} associated icons unresolved; pet/full-body artwork roles remain unmapped"}] if missing_entity_names or missing_entity_icons else []),
                "excluded": [{"family": family, "reason": reason} for family, reason in EXCLUDED_BY_OPERATOR.items()],
                "mapSourceDecision": "Map.pk2 selected; Map - copia.pk2 is backup and excluded",
            })
            _json_write(audit / "unresolved.json", unresolved)

        for role in source_roles:
            after = source_paths[role].stat()
            if (after.st_size, after.st_mtime_ns) != initial_stats[role] or sha256_file(source_paths[role]) != source_hashes[role]:
                raise ExportError(f"source archive changed during export: {source_paths[role].name}")

        from .cli import validate_bundle

        validation = validate_bundle(bundle)
        bundle_reused = _publish(staging, dataset_directory, lock)
        public_asset_validation = None
        if asset_output is not None:
            from .public_assets import materialize_public_assets

            public_asset_validation = materialize_public_assets(
                dataset_directory / "bundle",
                dataset_directory / "audit",
                asset_output,
                namespace="monsters" if monster_models is not None or unique_monsters else None,
                textdata=dataset_directory / "textdata",
            )
        bundle_bytes = sum(path.stat().st_size for path in (dataset_directory / "bundle").rglob("*") if path.is_file())
        audit_bytes = sum(path.stat().st_size for path in (dataset_directory / "audit").rglob("*") if path.is_file())
        result = {
            "datasetId": dataset_id,
            "completionStatus": completion_status,
            "datasetPath": str(dataset_directory),
            "bundlePath": str(dataset_directory / "bundle"),
            "auditPath": str(dataset_directory / "audit"),
            "textdataPath": str(dataset_directory / "textdata"),
            "textdataFileCount": textdata_report["fileCount"],
            "textdataBytes": textdata_report["totalBytes"],
            "elapsedSeconds": round(time.perf_counter() - started, 3),
            "bundleBytes": bundle_bytes,
            "auditBytes": audit_bytes,
            "catalogCount": validation["catalogsValidated"],
            "uniqueAssetCount": validation["assetsValidated"],
            "itemCount": len(item_records),
            "entityCount": len(entity_records),
            "skillCount": len(skill_records),
            "masteryCount": len(mastery_records),
            "skillGroupCount": len(group_records),
            "mapTileCount": len(tile_records),
            "teleportCount": len(teleport_records),
            "teleportLinkCount": len(teleport_links),
            "backgroundCount": len(background_records),
            "portraitCandidateCount": len(portrait_records),
            "portraitModelJoinCount": verified_portrait_joins,
            "interfaceSymbolCount": len(interface_records),
            "monsterTypeIconCount": monster_icon_count,
            "monsterRenderCount": monster_coverage["rendered"],
            "monsterRenderUnsupportedCount": monster_coverage["unsupported"],
            "monsterRenderInvalidCount": monster_coverage["invalid"],
            "unresolvedFamilyCount": len(unresolved),
            "sourceKnowledgeRequired": False,
            "identicalBundleReused": bundle_reused,
        }
        if public_asset_validation is not None:
            result["publicAssetOutputPath"] = str(asset_output)
            result["publicAssetCount"] = public_asset_validation["publicFilesValidated"]
            result["publicAssetKeyCount"] = public_asset_validation["semanticAssetKeysValidated"]
            result["publicTextdataPath"] = str(asset_output / "textdata")
            result["publicTextdataFileCount"] = public_asset_validation.get("textdataFilesValidated", 0)
        return result
    except Exception:
        shutil.rmtree(staging, ignore_errors=True)
        lock.unlink(missing_ok=True)
        raise
